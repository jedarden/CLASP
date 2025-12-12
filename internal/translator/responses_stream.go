// Package translator handles protocol translation between Anthropic and OpenAI formats.
package translator

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/jedarden/clasp/internal/logging"
	"github.com/jedarden/clasp/pkg/models"
)

// ResponsesStreamProcessor handles the transformation of OpenAI Responses API SSE streams
// to Anthropic format.
type ResponsesStreamProcessor struct {
	mu sync.Mutex

	// State tracking
	state       StreamState
	messageID   string
	targetModel string
	responseID  string // Tracks the Responses API response ID

	// Thinking/reasoning tracking
	thinkingStarted    bool
	thinkingBlockIndex int

	// Content tracking
	textStarted    bool
	textBlockIndex int

	// Function call tracking
	funcCallIndex   int
	activeFuncCalls map[int]*funcCallState

	// Web search citation tracking
	// Citations are collected during streaming and appended to text at the end
	citations []models.ResponsesAnnotation

	// Deduplication tracking for text deltas
	// The Responses API may send the same text through multiple event types
	// (content_part.delta, output_text.delta, etc.)
	// We track seen sequence numbers to deduplicate events
	seenSequenceNumbers map[int]bool

	// Usage tracking
	usage         *models.ResponsesUsage
	usageCallback UsageCallback

	// Output
	writer io.Writer
}

type funcCallState struct {
	id         string
	name       string
	arguments  string
	blockIndex int
	started    bool
	closed     bool
}

// NewResponsesStreamProcessor creates a new stream processor for Responses API.
func NewResponsesStreamProcessor(writer io.Writer, messageID, targetModel string) *ResponsesStreamProcessor {
	return &ResponsesStreamProcessor{
		writer:              writer,
		messageID:           messageID,
		targetModel:         targetModel,
		state:               StateIdle,
		textBlockIndex:      0,
		funcCallIndex:       0,
		activeFuncCalls:     make(map[int]*funcCallState),
		seenSequenceNumbers: make(map[int]bool),
	}
}

// GetResponseID returns the captured response ID for conversation continuation.
func (sp *ResponsesStreamProcessor) GetResponseID() string {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	return sp.responseID
}

// SetUsageCallback sets the callback function for usage reporting.
func (sp *ResponsesStreamProcessor) SetUsageCallback(callback UsageCallback) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.usageCallback = callback
}

// GetUsage returns the final usage statistics from the stream.
func (sp *ResponsesStreamProcessor) GetUsage() (inputTokens, outputTokens int) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	if sp.usage != nil {
		return sp.usage.InputTokens, sp.usage.OutputTokens
	}
	return 0, 0
}

// ProcessStream reads an OpenAI Responses API SSE stream and writes Anthropic SSE events.
func (sp *ResponsesStreamProcessor) ProcessStream(reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	logging.LogDebugMessage("[STREAM] Starting Responses API SSE stream processing for model: %s", sp.targetModel)

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			if data == "[DONE]" {
				logging.LogDebugMessage("[STREAM] Received [DONE] signal (Responses API)")
				return sp.finalize()
			}

			// Debug log incoming Responses API SSE event
			logging.LogDebugSSE("INCOMING Responses API", "event", data)

			var event models.ResponsesStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				logging.LogDebugMessage("[STREAM] Error parsing Responses API event: %v", err)
				continue
			}

			if err := sp.processEvent(&event); err != nil {
				return fmt.Errorf("processing event: %w", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanning stream: %w", err)
	}

	return sp.finalize()
}

// processEvent handles a single Responses API stream event.
func (sp *ResponsesStreamProcessor) processEvent(event *models.ResponsesStreamEvent) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Deduplicate events using sequence number
	// The Responses API may send the same content through multiple event types
	// (e.g., content_part.delta and output_text.delta) with the same sequence number
	if event.SequenceNumber > 0 {
		if sp.seenSequenceNumbers[event.SequenceNumber] {
			// Already processed this sequence number, skip
			return nil
		}
		sp.seenSequenceNumbers[event.SequenceNumber] = true
	}

	switch event.Type {
	// Response lifecycle events
	case models.EventResponseCreated:
		return sp.handleResponseCreated(event)
	case models.EventResponseQueued, models.EventResponseInProgress:
		// Informational events, no action needed
		return nil
	case models.EventResponseCompleted:
		return sp.handleResponseCompleted(event)
	case models.EventResponseFailed:
		return sp.handleResponseFailed(event)
	case models.EventResponseIncomplete:
		return sp.handleResponseIncomplete(event)

	// Output item events
	case models.EventOutputItemAdded:
		return sp.handleOutputItemAdded(event)
	case models.EventOutputItemDone:
		return sp.handleOutputItemDone(event)

	// Content part events (legacy format)
	case models.EventContentPartDelta:
		return sp.handleContentPartDelta(event)
	case models.EventContentPartDone:
		return sp.handleContentPartDone(event)
	case models.EventContentPartAdded:
		// Content part initialization, handled by output_item.added
		return nil

	// Primary text streaming events (newer format)
	case models.EventOutputTextDelta:
		return sp.handleOutputTextDelta(event)
	case models.EventOutputTextDone:
		// Final text, but we've already streamed the deltas
		return nil
	case models.EventOutputTextAnnotationAdd:
		return sp.handleAnnotationAdded(event)

	// Refusal events
	case models.EventRefusalDelta:
		return sp.handleRefusalDelta(event)
	case models.EventRefusalDone:
		return nil

	// Reasoning events
	case models.EventReasoningTextDelta, models.EventReasoningSummaryTextDelta:
		return sp.handleReasoningDelta(event)
	case models.EventReasoningTextDone, models.EventReasoningSummaryTextDone:
		return nil
	case models.EventReasoningSummaryPartAdded, models.EventReasoningSummaryPartDone:
		return nil

	// Function call events
	case models.EventFunctionCallArgs:
		return sp.handleFunctionCallDelta(event)
	case models.EventFunctionCallDone:
		// Function call complete, handled by output_item.done
		return nil

	// Rate limit event
	case models.EventRateLimitsUpdated:
		// Informational, no action needed
		return nil
	}

	return nil
}

// handleResponseCreated handles the response.created event.
func (sp *ResponsesStreamProcessor) handleResponseCreated(event *models.ResponsesStreamEvent) error {
	if event.Response != nil {
		sp.responseID = event.Response.ID
	}

	// Emit message_start
	if sp.state == StateIdle {
		if err := sp.emitMessageStart(); err != nil {
			return err
		}
		sp.state = StateMessageStarted
	}

	return nil
}

// handleOutputItemAdded handles the response.output_item.added event.
func (sp *ResponsesStreamProcessor) handleOutputItemAdded(event *models.ResponsesStreamEvent) error {
	if event.Item == nil {
		return nil
	}

	// Emit message_start if not already done
	if sp.state == StateIdle {
		if err := sp.emitMessageStart(); err != nil {
			return err
		}
		sp.state = StateMessageStarted
	}

	switch event.Item.Type {
	case "function_call":
		// Start tracking the function call
		// Convert Responses API "fc_xxx" back to "call_xxx" for Anthropic format
		anthropicID := TranslateResponsesIDToAnthropic(event.Item.CallID)
		fcState := &funcCallState{
			id:         anthropicID,
			name:       event.Item.Name,
			blockIndex: sp.calculateNextBlockIndex(),
		}
		sp.activeFuncCalls[sp.funcCallIndex] = fcState
		sp.funcCallIndex++

		// Close text block if open, then emit content_block_start for tool_use
		if sp.textStarted && sp.state == StateTextContent {
			if err := sp.emitContentBlockStop(sp.textBlockIndex); err != nil {
				return err
			}
			sp.state = StateToolCall
		}

		// Emit content_block_start for tool_use
		if err := sp.emitContentBlockStart(fcState.blockIndex, "tool_use", fcState.id, fcState.name); err != nil {
			return err
		}
		fcState.started = true

	case "web_search_call":
		// OpenAI's web_search_preview tool was invoked
		// Convert to a WebSearch tool_use block for Claude Code compatibility
		// The ID format from web search is typically "ws_xxx"
		webSearchID := event.Item.ID
		if webSearchID == "" {
			webSearchID = fmt.Sprintf("call_ws_%d", sp.funcCallIndex)
		}

		fcState := &funcCallState{
			id:         webSearchID,
			name:       "WebSearch",
			blockIndex: sp.calculateNextBlockIndex(),
		}
		sp.activeFuncCalls[sp.funcCallIndex] = fcState
		sp.funcCallIndex++

		// Close text block if open
		if sp.textStarted && sp.state == StateTextContent {
			if err := sp.emitContentBlockStop(sp.textBlockIndex); err != nil {
				return err
			}
			sp.state = StateToolCall
		}

		// Emit content_block_start for WebSearch tool_use
		if err := sp.emitContentBlockStart(fcState.blockIndex, "tool_use", fcState.id, "WebSearch"); err != nil {
			return err
		}
		fcState.started = true

		// Emit the search query as the tool input
		// WebSearch expects {"query": "search terms"}
		// The Action field is now an object with Type and Query
		query := ""
		if event.Item.Action != nil {
			query = event.Item.Action.Query
		}
		searchInput := fmt.Sprintf(`{"query":%q}`, query)
		fcState.arguments = searchInput
		if err := sp.emitContentBlockDelta(fcState.blockIndex, "input_json_delta", "", searchInput); err != nil {
			return err
		}
	}

	return nil
}

// TranslateResponsesIDToAnthropic converts Responses API function call IDs back to Anthropic format.
// Responses API uses "fc_xxx" prefix, Anthropic/Chat Completions uses "call_xxx".
// This function is exported for use by the proxy handler.
func TranslateResponsesIDToAnthropic(id string) string {
	if id == "" {
		return id
	}

	// Convert Responses API "fc_xxx" → Anthropic "call_xxx"
	if strings.HasPrefix(id, "fc_") {
		return "call_" + strings.TrimPrefix(id, "fc_")
	}

	// Already in Anthropic/Chat Completions format
	if strings.HasPrefix(id, "call_") || strings.HasPrefix(id, "toolu_") {
		return id
	}

	// Add call_ prefix for any other format
	return "call_" + id
}

// handleContentPartDelta handles the response.content_part.delta event.
// NOTE: This may be sent alongside output_text.delta with the same content.
// The handleTextDelta function handles deduplication.
func (sp *ResponsesStreamProcessor) handleContentPartDelta(event *models.ResponsesStreamEvent) error {
	if event.Delta == nil {
		return nil
	}

	switch event.Delta.Type {
	case "text_delta":
		return sp.handleTextDelta(event.Delta.Text)
	case "refusal_delta":
		// Handle refusals as text
		return sp.handleTextDelta(event.Delta.Refusal)
	}

	return nil
}

// handleTextDelta handles text content from the stream.
func (sp *ResponsesStreamProcessor) handleTextDelta(text string) error {
	if text == "" {
		return nil
	}

	// Emit message_start if not already done
	if sp.state == StateIdle {
		if err := sp.emitMessageStart(); err != nil {
			return err
		}
		sp.state = StateMessageStarted
	}

	// Close thinking block if transitioning from thinking to text
	if sp.thinkingStarted && sp.state == StateThinkingContent {
		if err := sp.emitContentBlockStop(sp.thinkingBlockIndex); err != nil {
			return err
		}
		// Adjust text block index to follow thinking block
		sp.textBlockIndex = sp.thinkingBlockIndex + 1
	}

	// Start text block if not started
	if !sp.textStarted {
		if err := sp.emitContentBlockStart(sp.textBlockIndex, "text", "", ""); err != nil {
			return err
		}
		sp.textStarted = true
		sp.state = StateTextContent
	}

	// Emit text delta
	return sp.emitContentBlockDelta(sp.textBlockIndex, "text_delta", text, "")
}

// handleFunctionCallDelta handles function call argument deltas.
// Note: For function_call_arguments.delta events, the "delta" field is a string directly,
// not a nested object. The custom unmarshaller stores this in DeltaText.
func (sp *ResponsesStreamProcessor) handleFunctionCallDelta(event *models.ResponsesStreamEvent) error {
	// For function call arguments, delta is a string stored in DeltaText
	deltaStr := event.DeltaText
	if deltaStr == "" && event.Delta != nil {
		// Fallback to nested Delta.Delta for other formats
		deltaStr = event.Delta.Delta
	}

	if deltaStr == "" {
		return nil
	}

	// Find the matching function call state
	for _, fcState := range sp.activeFuncCalls {
		if !fcState.started || fcState.closed {
			continue
		}
		fcState.arguments += deltaStr
		if err := sp.emitContentBlockDelta(fcState.blockIndex, "input_json_delta", "", deltaStr); err != nil {
			return err
		}
		break
	}

	return nil
}

// handleContentPartDone handles completion of a content part.
func (sp *ResponsesStreamProcessor) handleContentPartDone(event *models.ResponsesStreamEvent) error {
	// Content part completion is handled by output_item.done
	return nil
}

// handleOutputItemDone handles completion of an output item.
func (sp *ResponsesStreamProcessor) handleOutputItemDone(event *models.ResponsesStreamEvent) error {
	if event.Item == nil {
		return nil
	}

	switch event.Item.Type {
	case "message":
		// Close text block if open
		if sp.textStarted {
			if err := sp.emitContentBlockStop(sp.textBlockIndex); err != nil {
				return err
			}
		}
	case "function_call":
		// Close function call block
		// Note: fcState.id is in Anthropic format (call_xxx), but event.Item.CallID is
		// in Responses API format (fc_xxx). We need to translate for comparison.
		translatedCallID := TranslateResponsesIDToAnthropic(event.Item.CallID)
		for _, fcState := range sp.activeFuncCalls {
			if !fcState.started || fcState.closed || fcState.id != translatedCallID {
				continue
			}
			if err := sp.emitContentBlockStop(fcState.blockIndex); err != nil {
				return err
			}
			fcState.closed = true
			break
		}
	case "web_search_call":
		// Close the WebSearch tool_use block
		// Find the fcState by matching the WebSearch name
		for _, fcState := range sp.activeFuncCalls {
			if !fcState.started || fcState.closed || fcState.name != "WebSearch" {
				continue
			}
			if err := sp.emitContentBlockStop(fcState.blockIndex); err != nil {
				return err
			}
			fcState.closed = true
			break
		}
	}

	return nil
}

// handleResponseCompleted handles the response.completed event.
func (sp *ResponsesStreamProcessor) handleResponseCompleted(event *models.ResponsesStreamEvent) error {
	if event.Response != nil && event.Response.Usage != nil {
		sp.usage = event.Response.Usage
	}

	// Close thinking block if still open
	if sp.thinkingStarted && sp.state == StateThinkingContent {
		if err := sp.emitContentBlockStop(sp.thinkingBlockIndex); err != nil {
			return err
		}
	}

	// If we have citations from web search, append them to the text
	if len(sp.citations) > 0 && sp.textStarted {
		sourcesText := sp.formatCitationsAsText()
		if err := sp.emitContentBlockDelta(sp.textBlockIndex, "text_delta", sourcesText, ""); err != nil {
			return err
		}
	}

	// Close text block if open
	if sp.textStarted && sp.state == StateTextContent {
		if err := sp.emitContentBlockStop(sp.textBlockIndex); err != nil {
			return err
		}
	}

	// Close any open function call blocks
	for _, fcState := range sp.activeFuncCalls {
		if !fcState.started || fcState.closed {
			continue
		}
		if err := sp.emitContentBlockStop(fcState.blockIndex); err != nil {
			return err
		}
		fcState.closed = true
	}

	// Determine stop reason
	stopReason := "end_turn"
	if len(sp.activeFuncCalls) > 0 {
		stopReason = "tool_use"
	}

	return sp.emitMessageDelta(stopReason)
}

// formatCitationsAsText formats collected URL citations as a "Sources:" section.
func (sp *ResponsesStreamProcessor) formatCitationsAsText() string {
	if len(sp.citations) == 0 {
		return ""
	}

	// Deduplicate citations by URL
	seen := make(map[string]bool)
	var unique []models.ResponsesAnnotation
	for _, c := range sp.citations {
		if !seen[c.URL] {
			seen[c.URL] = true
			unique = append(unique, c)
		}
	}

	var sb strings.Builder
	sb.WriteString("\n\nSources:\n")
	for _, c := range unique {
		if c.Title != "" {
			sb.WriteString(fmt.Sprintf("- [%s](%s)\n", c.Title, c.URL))
		} else {
			sb.WriteString(fmt.Sprintf("- %s\n", c.URL))
		}
	}
	return sb.String()
}

// handleResponseFailed handles the response.failed event.
func (sp *ResponsesStreamProcessor) handleResponseFailed(event *models.ResponsesStreamEvent) error {
	// Emit error as text if possible
	if event.Error != nil {
		errorText := fmt.Sprintf("Error: %s - %s", event.Error.Code, event.Error.Message)
		if err := sp.handleTextDelta(errorText); err != nil {
			return err
		}
	}

	return sp.emitMessageDelta("end_turn")
}

// handleResponseIncomplete handles the response.incomplete event.
// This occurs when the response is cut short (max tokens, content filter, etc.)
func (sp *ResponsesStreamProcessor) handleResponseIncomplete(event *models.ResponsesStreamEvent) error {
	// Close any open blocks before emitting the incomplete status
	if sp.textStarted && sp.state == StateTextContent {
		if err := sp.emitContentBlockStop(sp.textBlockIndex); err != nil {
			return err
		}
	}

	for _, fcState := range sp.activeFuncCalls {
		if !fcState.started || fcState.closed {
			continue
		}
		if err := sp.emitContentBlockStop(fcState.blockIndex); err != nil {
			return err
		}
		fcState.closed = true
	}

	// Use max_tokens as the stop reason for incomplete responses
	return sp.emitMessageDelta("max_tokens")
}

// handleOutputTextDelta handles response.output_text.delta events.
// This is the primary text streaming event in the Responses API.
func (sp *ResponsesStreamProcessor) handleOutputTextDelta(event *models.ResponsesStreamEvent) error {
	// The delta text can come from different fields depending on API version
	var text string
	if event.DeltaText != "" {
		text = event.DeltaText
	} else if event.Delta != nil && event.Delta.Text != "" {
		text = event.Delta.Text
	}

	if text == "" {
		return nil
	}

	return sp.handleTextDelta(text)
}

// handleRefusalDelta handles response.refusal.delta events.
// Refusals are streamed as text with a prefix indicator.
func (sp *ResponsesStreamProcessor) handleRefusalDelta(event *models.ResponsesStreamEvent) error {
	var refusalText string
	if event.DeltaText != "" {
		refusalText = event.DeltaText
	} else if event.Delta != nil && event.Delta.Refusal != "" {
		refusalText = event.Delta.Refusal
	}

	if refusalText == "" {
		return nil
	}

	// Prefix refusal text if this is the start
	if !sp.textStarted {
		refusalText = "[Refused] " + refusalText
	}

	return sp.handleTextDelta(refusalText)
}

// handleAnnotationAdded handles response.output_text.annotation.added events.
// Annotations are citations from web search results. We collect them and will
// append a "Sources:" section to the text output when the response completes.
func (sp *ResponsesStreamProcessor) handleAnnotationAdded(event *models.ResponsesStreamEvent) error {
	if event.Annotation == nil {
		return nil
	}

	// Collect URL citations for later output
	if event.Annotation.Type == "url_citation" {
		sp.citations = append(sp.citations, *event.Annotation)
	}

	return nil
}

// handleReasoningDelta handles reasoning_text.delta and reasoning_summary_text.delta events.
// These are emitted as thinking blocks in the Anthropic format.
func (sp *ResponsesStreamProcessor) handleReasoningDelta(event *models.ResponsesStreamEvent) error {
	var reasoningText string
	if event.DeltaText != "" {
		reasoningText = event.DeltaText
	} else if event.Delta != nil && event.Delta.Text != "" {
		reasoningText = event.Delta.Text
	}

	if reasoningText == "" {
		return nil
	}

	// Emit message_start if not already done
	if sp.state == StateIdle {
		if err := sp.emitMessageStart(); err != nil {
			return err
		}
		sp.state = StateMessageStarted
	}

	// Start thinking block if not started
	if !sp.thinkingStarted {
		if err := sp.emitThinkingBlockStart(); err != nil {
			return err
		}
		sp.thinkingStarted = true
		sp.state = StateThinkingContent
	}

	// Emit thinking delta
	return sp.emitThinkingBlockDelta(reasoningText)
}

// emitThinkingBlockStart emits a content_block_start event for thinking.
func (sp *ResponsesStreamProcessor) emitThinkingBlockStart() error {
	event := models.ContentBlockStartEvent{
		Type:  models.EventContentBlockStart,
		Index: sp.thinkingBlockIndex,
		ContentBlock: models.ContentBlockStartData{
			Type: "thinking",
		},
	}

	return sp.writeEvent(models.EventContentBlockStart, event)
}

// emitThinkingBlockDelta emits a content_block_delta event for thinking.
func (sp *ResponsesStreamProcessor) emitThinkingBlockDelta(text string) error {
	event := models.ContentBlockDeltaEvent{
		Type:  models.EventContentBlockDelta,
		Index: sp.thinkingBlockIndex,
		Delta: models.DeltaData{
			Type:     "thinking_delta",
			Thinking: text,
		},
	}

	return sp.writeEvent(models.EventContentBlockDelta, event)
}

// calculateNextBlockIndex calculates the next content block index.
// This accounts for thinking blocks at index 0, text blocks following, then tool calls.
func (sp *ResponsesStreamProcessor) calculateNextBlockIndex() int {
	baseIndex := 0
	if sp.thinkingStarted {
		baseIndex = 1 // Thinking block takes index 0
	}
	if sp.textStarted {
		baseIndex = sp.textBlockIndex + 1
	}
	return baseIndex + len(sp.activeFuncCalls)
}

// finalize completes the stream processing.
func (sp *ResponsesStreamProcessor) finalize() error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Call usage callback if set
	if sp.usageCallback != nil && sp.usage != nil {
		sp.usageCallback(sp.usage.InputTokens, sp.usage.OutputTokens)
	}

	// Emit message_stop
	if err := sp.emitMessageStop(); err != nil {
		return err
	}

	// Emit [DONE]
	return sp.writeSSE("", "[DONE]")
}

// emitMessageStart emits a message_start event.
func (sp *ResponsesStreamProcessor) emitMessageStart() error {
	event := models.MessageStartEvent{
		Type: models.EventMessageStart,
		Message: models.AnthropicResponse{
			ID:         sp.messageID,
			Type:       "message",
			Role:       "assistant",
			Content:    []models.AnthropicContentBlock{},
			Model:      sp.targetModel,
			StopReason: "",
			Usage: &models.AnthropicUsage{
				InputTokens:  100,
				OutputTokens: 1,
			},
		},
	}

	if err := sp.writeEvent(models.EventMessageStart, event); err != nil {
		return err
	}

	return sp.writeEvent(models.EventPing, models.PingEvent{Type: models.EventPing})
}

// emitContentBlockStart emits a content_block_start event.
func (sp *ResponsesStreamProcessor) emitContentBlockStart(index int, blockType, id, name string) error {
	event := models.ContentBlockStartEvent{
		Type:  models.EventContentBlockStart,
		Index: index,
		ContentBlock: models.ContentBlockStartData{
			Type: blockType,
		},
	}

	if blockType == "text" {
		event.ContentBlock.Text = ""
	} else if blockType == "tool_use" {
		event.ContentBlock.ID = id
		event.ContentBlock.Name = name
	}

	return sp.writeEvent(models.EventContentBlockStart, event)
}

// emitContentBlockDelta emits a content_block_delta event.
func (sp *ResponsesStreamProcessor) emitContentBlockDelta(index int, deltaType, text, partialJSON string) error {
	event := models.ContentBlockDeltaEvent{
		Type:  models.EventContentBlockDelta,
		Index: index,
		Delta: models.DeltaData{
			Type: deltaType,
		},
	}

	if deltaType == "text_delta" {
		event.Delta.Text = text
	} else if deltaType == "input_json_delta" {
		event.Delta.PartialJSON = partialJSON
	}

	return sp.writeEvent(models.EventContentBlockDelta, event)
}

// emitContentBlockStop emits a content_block_stop event.
func (sp *ResponsesStreamProcessor) emitContentBlockStop(index int) error {
	event := models.ContentBlockStopEvent{
		Type:  models.EventContentBlockStop,
		Index: index,
	}

	return sp.writeEvent(models.EventContentBlockStop, event)
}

// emitMessageDelta emits a message_delta event.
func (sp *ResponsesStreamProcessor) emitMessageDelta(stopReason string) error {
	outputTokens := 0
	if sp.usage != nil {
		outputTokens = sp.usage.OutputTokens
	}

	event := models.MessageDeltaEvent{
		Type: models.EventMessageDelta,
		Delta: models.MessageDeltaData{
			StopReason: stopReason,
		},
		Usage: &models.MessageDeltaUsage{
			OutputTokens: outputTokens,
		},
	}

	return sp.writeEvent(models.EventMessageDelta, event)
}

// emitMessageStop emits a message_stop event.
func (sp *ResponsesStreamProcessor) emitMessageStop() error {
	event := models.MessageStopEvent{
		Type: models.EventMessageStop,
	}

	return sp.writeEvent(models.EventMessageStop, event)
}

// writeEvent writes an SSE event to the output.
func (sp *ResponsesStreamProcessor) writeEvent(eventType string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling event data: %w", err)
	}

	// Debug log outgoing Anthropic SSE event
	logging.LogDebugSSE("OUTGOING Anthropic", eventType, string(jsonData))

	return sp.writeSSE(eventType, string(jsonData))
}

// writeSSE writes raw SSE data to the output.
func (sp *ResponsesStreamProcessor) writeSSE(event, data string) error {
	var output string
	if event != "" {
		output = fmt.Sprintf("event: %s\ndata: %s\n\n", event, data)
	} else {
		output = fmt.Sprintf("data: %s\n\n", data)
	}

	_, err := sp.writer.Write([]byte(output))
	return err
}

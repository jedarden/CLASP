// Package translator handles protocol translation between Anthropic and OpenAI formats.
package translator

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"

	"github.com/jedarden/clasp/pkg/models"
)

// StreamState tracks the state of SSE stream transformation.
type StreamState int

const (
	StateIdle StreamState = iota
	StateMessageStarted
	StateTextContent
	StateToolCall
	StateDone
)

// UsageCallback is called when streaming completes with usage information.
type UsageCallback func(inputTokens, outputTokens int)

// StreamProcessor handles the transformation of OpenAI SSE streams to Anthropic format.
type StreamProcessor struct {
	mu sync.Mutex

	// State tracking
	state       StreamState
	messageID   string
	targetModel string

	// Content tracking
	textStarted    bool
	textBlockIndex int

	// Thinking/reasoning tracking (for O1/O3 models)
	thinkingStarted    bool
	thinkingBlockIndex int

	// Tool call tracking
	toolCallIndex   int
	activeToolCalls map[int]*toolCallState

	// Usage tracking
	usage         *models.Usage
	usageCallback UsageCallback

	// Output
	writer io.Writer

	// Grok XML tool call extraction
	xmlBuffer        string
	extractedXMLCall bool
	xmlToolCallID    int
}

type toolCallState struct {
	id          string
	name        string
	arguments   string
	blockIndex  int
	started     bool
	closed      bool
}

// NewStreamProcessor creates a new stream processor.
func NewStreamProcessor(writer io.Writer, messageID, targetModel string) *StreamProcessor {
	return &StreamProcessor{
		writer:             writer,
		messageID:          messageID,
		targetModel:        targetModel,
		state:              StateIdle,
		textBlockIndex:     0,
		thinkingBlockIndex: -1, // Thinking comes before text if present
		toolCallIndex:      0,
		activeToolCalls:    make(map[int]*toolCallState),
	}
}

// SetUsageCallback sets the callback function for usage reporting.
// This is called when the stream completes with final token usage.
func (sp *StreamProcessor) SetUsageCallback(callback UsageCallback) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.usageCallback = callback
}

// GetUsage returns the final usage statistics from the stream.
// This should be called after ProcessStream completes.
func (sp *StreamProcessor) GetUsage() (inputTokens, outputTokens int) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	if sp.usage != nil {
		return sp.usage.PromptTokens, sp.usage.CompletionTokens
	}
	return 0, 0
}

// ProcessStream reads an OpenAI SSE stream and writes Anthropic SSE events.
func (sp *StreamProcessor) ProcessStream(reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	// Increase buffer size for large chunks
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Handle data lines
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Handle [DONE] signal
			if data == "[DONE]" {
				return sp.finalize()
			}

			// Parse chunk
			var chunk models.OpenAIStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				// Log but continue on parse errors
				continue
			}

			if err := sp.processChunk(&chunk); err != nil {
				return fmt.Errorf("processing chunk: %w", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanning stream: %w", err)
	}

	return sp.finalize()
}

// processChunk handles a single OpenAI stream chunk.
func (sp *StreamProcessor) processChunk(chunk *models.OpenAIStreamChunk) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Track usage if provided
	if chunk.Usage != nil {
		sp.usage = chunk.Usage
	}

	// Emit message_start on first chunk
	if sp.state == StateIdle {
		if err := sp.emitMessageStart(); err != nil {
			return err
		}
		sp.state = StateMessageStarted
	}

	// Process each choice
	for _, choice := range chunk.Choices {
		if err := sp.processChoice(&choice); err != nil {
			return err
		}

		// Handle finish reason
		if choice.FinishReason != "" {
			if err := sp.handleFinishReason(choice.FinishReason); err != nil {
				return err
			}
		}
	}

	return nil
}

// processChoice processes a single choice from the stream chunk.
func (sp *StreamProcessor) processChoice(choice *models.StreamChoice) error {
	delta := &choice.Delta

	// Handle reasoning/thinking content first (for O1/O3 models)
	// Thinking comes before regular text output
	if delta.Reasoning != "" {
		if err := sp.handleThinkingContent(delta.Reasoning); err != nil {
			return err
		}
	}

	// Handle text content
	if delta.Content != "" {
		if err := sp.handleTextContent(delta.Content); err != nil {
			return err
		}
	}

	// Handle tool calls
	if len(delta.ToolCalls) > 0 {
		for _, tc := range delta.ToolCalls {
			if err := sp.handleToolCall(&tc); err != nil {
				return err
			}
		}
	}

	return nil
}

// handleThinkingContent handles reasoning/thinking content from O1/O3 models.
// This is emitted as a "thinking" content block in Anthropic format.
func (sp *StreamProcessor) handleThinkingContent(thinking string) error {
	// Start thinking block if not started
	if !sp.thinkingStarted {
		// Thinking block comes at index 0, text starts at index 1
		sp.thinkingBlockIndex = 0
		sp.textBlockIndex = 1 // Shift text block to index 1

		if err := sp.emitThinkingBlockStart(sp.thinkingBlockIndex); err != nil {
			return err
		}
		sp.thinkingStarted = true
	}

	// Emit thinking delta
	return sp.emitThinkingBlockDelta(sp.thinkingBlockIndex, thinking)
}

// handleTextContent handles text content from the stream.
func (sp *StreamProcessor) handleTextContent(text string) error {
	// Check if this is a Grok model that might emit XML tool calls
	if isGrokModelStream(sp.targetModel) {
		cleanedText, toolCalls := sp.processGrokXML(text)
		text = cleanedText

		// If we extracted tool calls, emit them
		for _, tc := range toolCalls {
			if err := sp.emitExtractedToolCall(tc); err != nil {
				return err
			}
		}

		// If no text left after extraction, skip
		if text == "" {
			return nil
		}
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

// handleToolCall handles a tool call from the stream.
func (sp *StreamProcessor) handleToolCall(tc *models.OpenAIToolCall) error {
	tcState, exists := sp.activeToolCalls[tc.Index]

	if !exists {
		// New tool call
		tcState = &toolCallState{
			blockIndex: sp.textBlockIndex + len(sp.activeToolCalls) + 1,
		}
		if sp.textStarted {
			tcState.blockIndex = sp.textBlockIndex + 1 + len(sp.activeToolCalls)
		} else {
			tcState.blockIndex = len(sp.activeToolCalls)
		}
		sp.activeToolCalls[tc.Index] = tcState
	}

	// Update tool call info
	if tc.ID != "" {
		tcState.id = tc.ID
	}
	if tc.Function.Name != "" {
		tcState.name = tc.Function.Name
	}
	if tc.Function.Arguments != "" {
		tcState.arguments += tc.Function.Arguments
	}

	// Start tool block if we have enough info and not started
	if tcState.id != "" && tcState.name != "" && !tcState.started {
		// Close text block if open
		if sp.textStarted && sp.state == StateTextContent {
			if err := sp.emitContentBlockStop(sp.textBlockIndex); err != nil {
				return err
			}
			sp.state = StateToolCall
		}

		if err := sp.emitContentBlockStart(tcState.blockIndex, "tool_use", tcState.id, tcState.name); err != nil {
			return err
		}
		tcState.started = true
	}

	// Emit tool input delta if we have arguments
	if tcState.started && tc.Function.Arguments != "" {
		if err := sp.emitContentBlockDelta(tcState.blockIndex, "input_json_delta", "", tc.Function.Arguments); err != nil {
			return err
		}
	}

	return nil
}

// handleFinishReason handles the finish reason from the stream.
func (sp *StreamProcessor) handleFinishReason(reason string) error {
	// Close any open thinking block first (thinking comes before text)
	if sp.thinkingStarted {
		if err := sp.emitContentBlockStop(sp.thinkingBlockIndex); err != nil {
			return err
		}
	}

	// Close any open text block
	if sp.textStarted && sp.state == StateTextContent {
		if err := sp.emitContentBlockStop(sp.textBlockIndex); err != nil {
			return err
		}
	}

	// Close any open tool blocks
	for _, tcState := range sp.activeToolCalls {
		if tcState.started && !tcState.closed {
			if err := sp.emitContentBlockStop(tcState.blockIndex); err != nil {
				return err
			}
			tcState.closed = true
		}
	}

	// Map finish reason to Anthropic stop reason
	stopReason := mapFinishReason(reason)

	// Emit message_delta
	return sp.emitMessageDelta(stopReason)
}

// finalize completes the stream processing.
func (sp *StreamProcessor) finalize() error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Call usage callback if set and we have usage data
	if sp.usageCallback != nil && sp.usage != nil {
		sp.usageCallback(sp.usage.PromptTokens, sp.usage.CompletionTokens)
	}

	// Emit message_stop
	if err := sp.emitMessageStop(); err != nil {
		return err
	}

	// Emit [DONE]
	return sp.writeSSE("", "[DONE]")
}

// emitMessageStart emits a message_start event.
func (sp *StreamProcessor) emitMessageStart() error {
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
				InputTokens:  100, // Placeholder
				OutputTokens: 1,
			},
		},
	}

	if err := sp.writeEvent(models.EventMessageStart, event); err != nil {
		return err
	}

	// Emit ping after message_start
	return sp.writeEvent(models.EventPing, models.PingEvent{Type: models.EventPing})
}

// emitContentBlockStart emits a content_block_start event.
func (sp *StreamProcessor) emitContentBlockStart(index int, blockType, id, name string) error {
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
func (sp *StreamProcessor) emitContentBlockDelta(index int, deltaType, text, partialJSON string) error {
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

// emitThinkingBlockStart emits a content_block_start event for a thinking block.
func (sp *StreamProcessor) emitThinkingBlockStart(index int) error {
	event := models.ContentBlockStartEvent{
		Type:  models.EventContentBlockStart,
		Index: index,
		ContentBlock: models.ContentBlockStartData{
			Type:     "thinking",
			Thinking: "",
		},
	}

	return sp.writeEvent(models.EventContentBlockStart, event)
}

// emitThinkingBlockDelta emits a content_block_delta event for thinking content.
func (sp *StreamProcessor) emitThinkingBlockDelta(index int, thinking string) error {
	event := models.ContentBlockDeltaEvent{
		Type:  models.EventContentBlockDelta,
		Index: index,
		Delta: models.DeltaData{
			Type:     "thinking_delta",
			Thinking: thinking,
		},
	}

	return sp.writeEvent(models.EventContentBlockDelta, event)
}

// emitContentBlockStop emits a content_block_stop event.
func (sp *StreamProcessor) emitContentBlockStop(index int) error {
	event := models.ContentBlockStopEvent{
		Type:  models.EventContentBlockStop,
		Index: index,
	}

	return sp.writeEvent(models.EventContentBlockStop, event)
}

// emitMessageDelta emits a message_delta event.
func (sp *StreamProcessor) emitMessageDelta(stopReason string) error {
	outputTokens := 0
	if sp.usage != nil {
		outputTokens = sp.usage.CompletionTokens
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
func (sp *StreamProcessor) emitMessageStop() error {
	event := models.MessageStopEvent{
		Type: models.EventMessageStop,
	}

	return sp.writeEvent(models.EventMessageStop, event)
}

// writeEvent writes an SSE event to the output.
func (sp *StreamProcessor) writeEvent(eventType string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling event data: %w", err)
	}

	return sp.writeSSE(eventType, string(jsonData))
}

// writeSSE writes raw SSE data to the output.
func (sp *StreamProcessor) writeSSE(event, data string) error {
	var output string
	if event != "" {
		output = fmt.Sprintf("event: %s\ndata: %s\n\n", event, data)
	} else {
		output = fmt.Sprintf("data: %s\n\n", data)
	}

	_, err := sp.writer.Write([]byte(output))
	return err
}

// mapFinishReason maps OpenAI finish_reason to Anthropic stop_reason.
func mapFinishReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "tool_calls":
		return "tool_use"
	case "length":
		return "max_tokens"
	case "content_filter":
		return "end_turn"
	default:
		return "end_turn"
	}
}

// isGrokModelStream checks if the model is a Grok model (for streaming context).
func isGrokModelStream(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "grok") || strings.HasPrefix(m, "x-ai/")
}

// extractedToolCall represents a tool call extracted from Grok XML.
type extractedToolCall struct {
	name      string
	arguments map[string]interface{}
}

// processGrokXML buffers text and extracts Grok XML tool calls.
// Returns cleaned text (with XML removed) and any extracted tool calls.
func (sp *StreamProcessor) processGrokXML(text string) (string, []extractedToolCall) {
	// Accumulate text in buffer
	sp.xmlBuffer += text

	// Pattern for complete Grok XML tool calls
	// Format: <xai:function_call name="func_name"><xai:parameter name="param">value</xai:parameter></xai:function_call>
	xmlPattern := regexp.MustCompile(`<xai:function_call\s+name="([^"]+)">(.*?)</xai:function_call>`)

	matches := xmlPattern.FindAllStringSubmatch(sp.xmlBuffer, -1)

	if len(matches) == 0 {
		// Check if we have a partial XML that needs buffering
		if strings.Contains(sp.xmlBuffer, "<xai:function_call") && !strings.Contains(sp.xmlBuffer, "</xai:function_call>") {
			// Partial XML - keep buffering, don't emit text yet
			return "", nil
		}

		// No XML found, return buffer and clear
		result := sp.xmlBuffer
		sp.xmlBuffer = ""
		return result, nil
	}

	// Extract tool calls from matches
	var toolCalls []extractedToolCall
	for _, match := range matches {
		if len(match) >= 3 {
			funcName := match[1]
			paramsXML := match[2]

			// Parse parameters from XML
			params := parseXMLParameters(paramsXML)

			toolCalls = append(toolCalls, extractedToolCall{
				name:      funcName,
				arguments: params,
			})
		}
	}

	// Remove XML from buffer and return cleaned text
	cleanedText := xmlPattern.ReplaceAllString(sp.xmlBuffer, "")
	sp.xmlBuffer = ""

	// Clean up whitespace
	cleanedText = strings.TrimSpace(cleanedText)

	return cleanedText, toolCalls
}

// parseXMLParameters extracts parameters from Grok XML parameter format.
func parseXMLParameters(xmlContent string) map[string]interface{} {
	params := make(map[string]interface{})

	// Pattern for parameters: <xai:parameter name="param_name">value</xai:parameter>
	paramPattern := regexp.MustCompile(`<xai:parameter\s+name="([^"]+)">(.*?)</xai:parameter>`)

	matches := paramPattern.FindAllStringSubmatch(xmlContent, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			paramName := match[1]
			paramValue := match[2]

			// Try to parse as JSON first (for objects/arrays)
			var jsonValue interface{}
			if err := json.Unmarshal([]byte(paramValue), &jsonValue); err == nil {
				params[paramName] = jsonValue
			} else {
				// Use as string
				params[paramName] = paramValue
			}
		}
	}

	return params
}

// emitExtractedToolCall emits an extracted Grok XML tool call as Anthropic format.
func (sp *StreamProcessor) emitExtractedToolCall(tc extractedToolCall) error {
	// Close text block if open
	if sp.textStarted && sp.state == StateTextContent {
		if err := sp.emitContentBlockStop(sp.textBlockIndex); err != nil {
			return err
		}
	}

	// Generate a unique tool call ID
	sp.xmlToolCallID++
	toolID := fmt.Sprintf("toolu_grok_%d", sp.xmlToolCallID)

	// Calculate block index
	blockIndex := sp.textBlockIndex
	if sp.textStarted {
		blockIndex = sp.textBlockIndex + 1 + len(sp.activeToolCalls)
	} else {
		blockIndex = len(sp.activeToolCalls)
	}

	// Emit content_block_start for tool_use
	if err := sp.emitContentBlockStart(blockIndex, "tool_use", toolID, tc.name); err != nil {
		return err
	}

	// Convert arguments to JSON and emit
	argsJSON, err := json.Marshal(tc.arguments)
	if err != nil {
		argsJSON = []byte("{}")
	}

	if err := sp.emitContentBlockDelta(blockIndex, "input_json_delta", "", string(argsJSON)); err != nil {
		return err
	}

	// Close the tool block
	if err := sp.emitContentBlockStop(blockIndex); err != nil {
		return err
	}

	sp.extractedXMLCall = true
	sp.state = StateToolCall

	return nil
}

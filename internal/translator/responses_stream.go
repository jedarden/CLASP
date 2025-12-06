// Package translator handles protocol translation between Anthropic and OpenAI formats.
package translator

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/jedarden/clasp/pkg/models"
)

// ResponsesStreamProcessor handles the transformation of OpenAI Responses API SSE streams
// to Anthropic format.
type ResponsesStreamProcessor struct {
	mu sync.Mutex

	// State tracking
	state        StreamState
	messageID    string
	targetModel  string
	responseID   string // Tracks the Responses API response ID

	// Content tracking
	textStarted    bool
	textBlockIndex int

	// Function call tracking
	funcCallIndex    int
	activeFuncCalls  map[int]*funcCallState

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
		writer:          writer,
		messageID:       messageID,
		targetModel:     targetModel,
		state:           StateIdle,
		textBlockIndex:  0,
		funcCallIndex:   0,
		activeFuncCalls: make(map[int]*funcCallState),
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

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			if data == "[DONE]" {
				return sp.finalize()
			}

			var event models.ResponsesStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
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

	switch event.Type {
	case models.EventResponseCreated:
		return sp.handleResponseCreated(event)

	case models.EventOutputItemAdded:
		return sp.handleOutputItemAdded(event)

	case models.EventContentPartDelta:
		return sp.handleContentPartDelta(event)

	case models.EventContentPartDone:
		return sp.handleContentPartDone(event)

	case models.EventFunctionCallArgs:
		return sp.handleFunctionCallDelta(event)

	case models.EventOutputItemDone:
		return sp.handleOutputItemDone(event)

	case models.EventResponseCompleted:
		return sp.handleResponseCompleted(event)

	case models.EventResponseFailed:
		return sp.handleResponseFailed(event)
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
	case "message":
		// Will be handled by content deltas
	case "function_call":
		// Start tracking the function call
		fcState := &funcCallState{
			id:         event.Item.CallID,
			name:       event.Item.Name,
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

		// Emit content_block_start for tool_use
		if err := sp.emitContentBlockStart(fcState.blockIndex, "tool_use", fcState.id, fcState.name); err != nil {
			return err
		}
		fcState.started = true
	}

	return nil
}

// handleContentPartDelta handles the response.content_part.delta event.
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
func (sp *ResponsesStreamProcessor) handleFunctionCallDelta(event *models.ResponsesStreamEvent) error {
	if event.Delta == nil {
		return nil
	}

	// Find the matching function call state
	for _, fcState := range sp.activeFuncCalls {
		if fcState.started && !fcState.closed {
			fcState.arguments += event.Delta.Delta
			if err := sp.emitContentBlockDelta(fcState.blockIndex, "input_json_delta", "", event.Delta.Delta); err != nil {
				return err
			}
			break
		}
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
		for _, fcState := range sp.activeFuncCalls {
			if fcState.started && !fcState.closed && fcState.id == event.Item.CallID {
				if err := sp.emitContentBlockStop(fcState.blockIndex); err != nil {
					return err
				}
				fcState.closed = true
				break
			}
		}
	}

	return nil
}

// handleResponseCompleted handles the response.completed event.
func (sp *ResponsesStreamProcessor) handleResponseCompleted(event *models.ResponsesStreamEvent) error {
	if event.Response != nil && event.Response.Usage != nil {
		sp.usage = event.Response.Usage
	}

	// Close any open blocks
	if sp.textStarted && sp.state == StateTextContent {
		if err := sp.emitContentBlockStop(sp.textBlockIndex); err != nil {
			return err
		}
	}

	for _, fcState := range sp.activeFuncCalls {
		if fcState.started && !fcState.closed {
			if err := sp.emitContentBlockStop(fcState.blockIndex); err != nil {
				return err
			}
			fcState.closed = true
		}
	}

	// Determine stop reason
	stopReason := "end_turn"
	if len(sp.activeFuncCalls) > 0 {
		stopReason = "tool_use"
	}

	return sp.emitMessageDelta(stopReason)
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

// calculateNextBlockIndex calculates the next content block index.
func (sp *ResponsesStreamProcessor) calculateNextBlockIndex() int {
	index := len(sp.activeFuncCalls)
	if sp.textStarted {
		index = sp.textBlockIndex + 1 + len(sp.activeFuncCalls)
	}
	return index
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

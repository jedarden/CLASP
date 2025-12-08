package translator

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jedarden/clasp/pkg/models"
)

func TestResponsesStreamProcessor_OutputTextDelta(t *testing.T) {
	var buf bytes.Buffer
	sp := NewResponsesStreamProcessor(&buf, "msg_test", "gpt-5")

	// Simulate response.output_text.delta event
	event := &models.ResponsesStreamEvent{
		Type:      models.EventOutputTextDelta,
		DeltaText: "Hello, world!",
	}

	if err := sp.processEvent(event); err != nil {
		t.Fatalf("processEvent failed: %v", err)
	}

	// Verify output contains text delta
	output := buf.String()
	if !strings.Contains(output, "text_delta") {
		t.Errorf("output should contain text_delta, got: %s", output)
	}
	if !strings.Contains(output, "Hello, world!") {
		t.Errorf("output should contain 'Hello, world!', got: %s", output)
	}
}

func TestResponsesStreamProcessor_RefusalDelta(t *testing.T) {
	var buf bytes.Buffer
	sp := NewResponsesStreamProcessor(&buf, "msg_test", "gpt-5")

	// Simulate response.refusal.delta event
	event := &models.ResponsesStreamEvent{
		Type:      models.EventRefusalDelta,
		DeltaText: "I cannot assist with that.",
	}

	if err := sp.processEvent(event); err != nil {
		t.Fatalf("processEvent failed: %v", err)
	}

	// Verify output contains refusal text with prefix
	output := buf.String()
	if !strings.Contains(output, "[Refused]") {
		t.Errorf("output should contain '[Refused]' prefix, got: %s", output)
	}
	if !strings.Contains(output, "I cannot assist with that.") {
		t.Errorf("output should contain refusal text, got: %s", output)
	}
}

func TestResponsesStreamProcessor_ReasoningDelta(t *testing.T) {
	var buf bytes.Buffer
	sp := NewResponsesStreamProcessor(&buf, "msg_test", "gpt-5")

	// Simulate response.reasoning_text.delta event
	event := &models.ResponsesStreamEvent{
		Type:      models.EventReasoningTextDelta,
		DeltaText: "Let me think about this step by step...",
	}

	if err := sp.processEvent(event); err != nil {
		t.Fatalf("processEvent failed: %v", err)
	}

	// Verify output contains thinking block
	output := buf.String()
	if !strings.Contains(output, "thinking") {
		t.Errorf("output should contain 'thinking' block type, got: %s", output)
	}
	if !strings.Contains(output, "thinking_delta") {
		t.Errorf("output should contain 'thinking_delta' delta type, got: %s", output)
	}
}

func TestResponsesStreamProcessor_ReasoningThenText(t *testing.T) {
	var buf bytes.Buffer
	sp := NewResponsesStreamProcessor(&buf, "msg_test", "gpt-5")

	// First: reasoning event
	reasoningEvent := &models.ResponsesStreamEvent{
		Type:      models.EventReasoningTextDelta,
		DeltaText: "Thinking...",
	}
	if err := sp.processEvent(reasoningEvent); err != nil {
		t.Fatalf("processEvent (reasoning) failed: %v", err)
	}

	// Then: text output event
	textEvent := &models.ResponsesStreamEvent{
		Type:      models.EventOutputTextDelta,
		DeltaText: "The answer is 42.",
	}
	if err := sp.processEvent(textEvent); err != nil {
		t.Fatalf("processEvent (text) failed: %v", err)
	}

	output := buf.String()

	// Verify both thinking and text content exist
	if !strings.Contains(output, "thinking") {
		t.Errorf("output should contain thinking block, got: %s", output)
	}
	if !strings.Contains(output, "The answer is 42.") {
		t.Errorf("output should contain text content, got: %s", output)
	}

	// Verify thinking block was closed before text started
	thinkingStopIndex := strings.Index(output, `"type":"content_block_stop"`)
	textStartIndex := strings.LastIndex(output, `"type":"content_block_start"`)
	if thinkingStopIndex == -1 || textStartIndex == -1 {
		t.Errorf("expected both content_block_stop and content_block_start events")
	}
	if thinkingStopIndex > textStartIndex {
		t.Errorf("thinking block should be closed before text block starts")
	}
}

func TestResponsesStreamProcessor_IncompleteResponse(t *testing.T) {
	var buf bytes.Buffer
	sp := NewResponsesStreamProcessor(&buf, "msg_test", "gpt-5")

	// Start with some text
	textEvent := &models.ResponsesStreamEvent{
		Type:      models.EventOutputTextDelta,
		DeltaText: "This is an incomplete",
	}
	if err := sp.processEvent(textEvent); err != nil {
		t.Fatalf("processEvent (text) failed: %v", err)
	}

	// Then incomplete event
	incompleteEvent := &models.ResponsesStreamEvent{
		Type: models.EventResponseIncomplete,
	}
	if err := sp.processEvent(incompleteEvent); err != nil {
		t.Fatalf("processEvent (incomplete) failed: %v", err)
	}

	output := buf.String()

	// Verify stop reason is max_tokens
	if !strings.Contains(output, "max_tokens") {
		t.Errorf("incomplete response should have 'max_tokens' stop reason, got: %s", output)
	}
}

func TestResponsesStreamProcessor_FunctionCallComplete(t *testing.T) {
	var buf bytes.Buffer
	sp := NewResponsesStreamProcessor(&buf, "msg_test", "gpt-5")

	// Output item added with function_call type
	itemAddedEvent := &models.ResponsesStreamEvent{
		Type: models.EventOutputItemAdded,
		Item: &models.ResponsesItem{
			Type:   "function_call",
			CallID: "fc_123",
			Name:   "get_weather",
		},
	}
	if err := sp.processEvent(itemAddedEvent); err != nil {
		t.Fatalf("processEvent (item added) failed: %v", err)
	}

	// Function call arguments delta
	argsEvent := &models.ResponsesStreamEvent{
		Type: models.EventFunctionCallArgs,
		Delta: &models.ResponsesDelta{
			Delta: `{"location": "New York"}`,
		},
	}
	if err := sp.processEvent(argsEvent); err != nil {
		t.Fatalf("processEvent (args) failed: %v", err)
	}

	// Response completed
	completedEvent := &models.ResponsesStreamEvent{
		Type: models.EventResponseCompleted,
		Response: &models.ResponsesResponse{
			ID:     "resp_123",
			Status: "completed",
			Usage: &models.ResponsesUsage{
				InputTokens:  100,
				OutputTokens: 50,
			},
		},
	}
	if err := sp.processEvent(completedEvent); err != nil {
		t.Fatalf("processEvent (completed) failed: %v", err)
	}

	output := buf.String()

	// Verify tool_use content block
	if !strings.Contains(output, "tool_use") {
		t.Errorf("output should contain tool_use content block, got: %s", output)
	}
	if !strings.Contains(output, "get_weather") {
		t.Errorf("output should contain function name 'get_weather', got: %s", output)
	}
	if !strings.Contains(output, `"stop_reason":"tool_use"`) {
		t.Errorf("output should have tool_use stop_reason, got: %s", output)
	}
}

func TestResponsesStreamProcessor_ResponseIDTracking(t *testing.T) {
	var buf bytes.Buffer
	sp := NewResponsesStreamProcessor(&buf, "msg_test", "gpt-5")

	// Response created with ID
	createdEvent := &models.ResponsesStreamEvent{
		Type: models.EventResponseCreated,
		Response: &models.ResponsesResponse{
			ID: "resp_abc123",
		},
	}
	if err := sp.processEvent(createdEvent); err != nil {
		t.Fatalf("processEvent failed: %v", err)
	}

	// Verify response ID is tracked
	if sp.GetResponseID() != "resp_abc123" {
		t.Errorf("GetResponseID() = %q, want %q", sp.GetResponseID(), "resp_abc123")
	}
}

func TestResponsesStreamProcessor_UsageCallback(t *testing.T) {
	var buf bytes.Buffer
	sp := NewResponsesStreamProcessor(&buf, "msg_test", "gpt-5")

	var callbackInput, callbackOutput int
	sp.SetUsageCallback(func(input, output int) {
		callbackInput = input
		callbackOutput = output
	})

	// Response completed with usage
	completedEvent := &models.ResponsesStreamEvent{
		Type: models.EventResponseCompleted,
		Response: &models.ResponsesResponse{
			ID:     "resp_123",
			Status: "completed",
			Usage: &models.ResponsesUsage{
				InputTokens:  150,
				OutputTokens: 75,
			},
		},
	}
	if err := sp.processEvent(completedEvent); err != nil {
		t.Fatalf("processEvent failed: %v", err)
	}

	// Finalize to trigger callback
	if err := sp.finalize(); err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	// Verify callback was called with correct values
	if callbackInput != 150 {
		t.Errorf("callback input = %d, want 150", callbackInput)
	}
	if callbackOutput != 75 {
		t.Errorf("callback output = %d, want 75", callbackOutput)
	}
}

func TestResponsesStreamProcessor_EventSequence(t *testing.T) {
	var buf bytes.Buffer
	sp := NewResponsesStreamProcessor(&buf, "msg_test", "gpt-5")

	events := []struct {
		Type string
		Event *models.ResponsesStreamEvent
	}{
		{"created", &models.ResponsesStreamEvent{Type: models.EventResponseCreated, Response: &models.ResponsesResponse{ID: "resp_1"}}},
		{"output_text.delta", &models.ResponsesStreamEvent{Type: models.EventOutputTextDelta, DeltaText: "Hello"}},
		{"output_text.delta", &models.ResponsesStreamEvent{Type: models.EventOutputTextDelta, DeltaText: " World"}},
		{"completed", &models.ResponsesStreamEvent{Type: models.EventResponseCompleted, Response: &models.ResponsesResponse{Status: "completed", Usage: &models.ResponsesUsage{InputTokens: 10, OutputTokens: 5}}}},
	}

	for _, e := range events {
		if err := sp.processEvent(e.Event); err != nil {
			t.Fatalf("processEvent (%s) failed: %v", e.Type, err)
		}
	}

	output := buf.String()

	// Verify proper event sequence
	messageStartIdx := strings.Index(output, "message_start")
	contentBlockStartIdx := strings.Index(output, "content_block_start")
	contentBlockDeltaIdx := strings.Index(output, "content_block_delta")
	messageDeltaIdx := strings.Index(output, "message_delta")

	if messageStartIdx == -1 || contentBlockStartIdx == -1 || contentBlockDeltaIdx == -1 || messageDeltaIdx == -1 {
		t.Errorf("missing expected events in output: %s", output)
	}

	// Verify order
	if messageStartIdx > contentBlockStartIdx {
		t.Errorf("message_start should come before content_block_start")
	}
	if contentBlockStartIdx > contentBlockDeltaIdx {
		t.Errorf("content_block_start should come before content_block_delta")
	}
	if contentBlockDeltaIdx > messageDeltaIdx {
		t.Errorf("content_block_delta should come before message_delta")
	}
}

func TestResponsesStreamProcessor_AllEventTypes(t *testing.T) {
	// Test that all supported event types are handled without error
	eventTypes := []string{
		models.EventResponseCreated,
		models.EventResponseQueued,
		models.EventResponseInProgress,
		models.EventResponseCompleted,
		models.EventResponseFailed,
		models.EventResponseIncomplete,
		models.EventOutputItemAdded,
		models.EventOutputItemDone,
		models.EventContentPartAdded,
		models.EventContentPartDelta,
		models.EventContentPartDone,
		models.EventOutputTextDelta,
		models.EventOutputTextDone,
		models.EventRefusalDelta,
		models.EventRefusalDone,
		models.EventReasoningTextDelta,
		models.EventReasoningTextDone,
		models.EventReasoningSummaryPartAdded,
		models.EventReasoningSummaryPartDone,
		models.EventReasoningSummaryTextDelta,
		models.EventReasoningSummaryTextDone,
		models.EventFunctionCallArgs,
		models.EventFunctionCallDone,
		models.EventRateLimitsUpdated,
	}

	for _, eventType := range eventTypes {
		t.Run(eventType, func(t *testing.T) {
			var buf bytes.Buffer
			sp := NewResponsesStreamProcessor(&buf, "msg_test", "gpt-5")

			event := &models.ResponsesStreamEvent{
				Type: eventType,
				Response: &models.ResponsesResponse{
					ID:     "resp_test",
					Status: "completed",
					Usage:  &models.ResponsesUsage{InputTokens: 1, OutputTokens: 1},
				},
				Item: &models.ResponsesItem{
					Type:   "message",
					CallID: "fc_test",
					Name:   "test_func",
				},
				Delta: &models.ResponsesDelta{
					Type:    "text_delta",
					Text:    "test",
					Delta:   "{}",
					Refusal: "test refusal",
				},
				DeltaText: "test delta",
			}

			// Should not panic or return error
			err := sp.processEvent(event)
			if err != nil {
				t.Errorf("processEvent(%s) returned error: %v", eventType, err)
			}
		})
	}
}

func TestResponsesStreamProcessor_FullStream(t *testing.T) {
	// Test with direct processEvent calls to ensure proper event handling
	var buf bytes.Buffer
	sp := NewResponsesStreamProcessor(&buf, "msg_test", "gpt-5")

	// Manually process events as the actual streaming would
	events := []*models.ResponsesStreamEvent{
		{
			Type:     models.EventResponseCreated,
			Response: &models.ResponsesResponse{ID: "resp_123", Status: "in_progress"},
		},
		{
			Type: models.EventOutputItemAdded,
			Item: &models.ResponsesItem{Type: "message", ID: "item_1"},
		},
		{
			Type: models.EventContentPartDelta,
			Delta: &models.ResponsesDelta{Type: "text_delta", Text: "Hello, "},
		},
		{
			Type: models.EventContentPartDelta,
			Delta: &models.ResponsesDelta{Type: "text_delta", Text: "world!"},
		},
		{
			Type:     models.EventResponseCompleted,
			Response: &models.ResponsesResponse{ID: "resp_123", Status: "completed", Usage: &models.ResponsesUsage{InputTokens: 10, OutputTokens: 5}},
		},
	}

	for i, event := range events {
		if err := sp.processEvent(event); err != nil {
			t.Fatalf("processEvent %d failed: %v", i, err)
		}
	}

	// Finalize the stream
	if err := sp.finalize(); err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	output := buf.String()

	// Parse the output to verify structure
	lines := strings.Split(output, "\n")
	var eventTypes []string
	for _, line := range lines {
		if strings.HasPrefix(line, "event: ") {
			eventType := strings.TrimPrefix(line, "event: ")
			eventTypes = append(eventTypes, eventType)
		}
	}

	// Verify we got the essential Anthropic event types
	// message_start, ping, content_block_start, content_block_delta(s), content_block_stop, message_delta, message_stop
	if len(eventTypes) < 6 {
		t.Errorf("expected at least 6 events, got %d: %v\nFull output:\n%s", len(eventTypes), eventTypes, output)
	}

	// Verify message_stop is present
	if !strings.Contains(output, "message_stop") {
		t.Errorf("output should contain message_stop event")
	}

	// Verify [DONE] is present
	if !strings.Contains(output, "[DONE]") {
		t.Errorf("output should contain [DONE] marker")
	}

	// Verify text_delta content type is present
	if !strings.Contains(output, "text_delta") {
		t.Errorf("output should contain text_delta content, got: %s", output)
	}

	// Verify actual text content
	if !strings.Contains(output, "Hello") || !strings.Contains(output, "world") {
		t.Errorf("output should contain streamed text 'Hello' and 'world', got: %s", output)
	}
}

func TestResponsesStreamProcessor_ErrorHandling(t *testing.T) {
	var buf bytes.Buffer
	sp := NewResponsesStreamProcessor(&buf, "msg_test", "gpt-5")

	// Response failed event
	failedEvent := &models.ResponsesStreamEvent{
		Type: models.EventResponseFailed,
		Error: &models.ResponsesError{
			Code:    "server_error",
			Message: "Internal server error",
		},
	}
	if err := sp.processEvent(failedEvent); err != nil {
		t.Fatalf("processEvent failed: %v", err)
	}

	output := buf.String()

	// Verify error is included in output
	if !strings.Contains(output, "server_error") {
		t.Errorf("output should contain error code, got: %s", output)
	}
	if !strings.Contains(output, "Internal server error") {
		t.Errorf("output should contain error message, got: %s", output)
	}
}

// TestResponsesStreamProcessor_FunctionCallIDTranslation verifies that function call IDs
// are properly translated between Responses API format (fc_xxx) and Anthropic format (call_xxx).
// This was a bug where handleOutputItemDone failed to close tool blocks because it compared
// the translated Anthropic ID with the raw Responses API ID.
func TestResponsesStreamProcessor_FunctionCallIDTranslation(t *testing.T) {
	var buf bytes.Buffer
	sp := NewResponsesStreamProcessor(&buf, "msg_test", "gpt-5")

	// Output item added with function_call type (Responses API uses fc_ prefix)
	itemAddedEvent := &models.ResponsesStreamEvent{
		Type: models.EventOutputItemAdded,
		Item: &models.ResponsesItem{
			Type:   "function_call",
			CallID: "fc_abc123xyz", // Responses API format
			Name:   "search_files",
		},
	}
	if err := sp.processEvent(itemAddedEvent); err != nil {
		t.Fatalf("processEvent (item added) failed: %v", err)
	}

	// Function call arguments delta
	argsEvent := &models.ResponsesStreamEvent{
		Type: models.EventFunctionCallArgs,
		Delta: &models.ResponsesDelta{
			Delta: `{"query": "test"}`,
		},
	}
	if err := sp.processEvent(argsEvent); err != nil {
		t.Fatalf("processEvent (args) failed: %v", err)
	}

	// Output item done - this should close the function call block
	// The CallID is in Responses API format (fc_xxx), but internally we stored
	// the translated Anthropic format (call_xxx). The fix ensures we translate
	// the incoming ID before comparison.
	itemDoneEvent := &models.ResponsesStreamEvent{
		Type: models.EventOutputItemDone,
		Item: &models.ResponsesItem{
			Type:   "function_call",
			CallID: "fc_abc123xyz", // Same Responses API format ID
			Name:   "search_files",
		},
	}
	if err := sp.processEvent(itemDoneEvent); err != nil {
		t.Fatalf("processEvent (item done) failed: %v", err)
	}

	output := buf.String()

	// Verify the tool call block was started and closed properly
	if !strings.Contains(output, "tool_use") {
		t.Errorf("output should contain tool_use content block, got: %s", output)
	}
	if !strings.Contains(output, "search_files") {
		t.Errorf("output should contain function name 'search_files', got: %s", output)
	}

	// Verify content_block_stop was emitted for the tool block
	// Count content_block_stop events
	stopCount := strings.Count(output, "content_block_stop")
	if stopCount < 1 {
		t.Errorf("expected at least 1 content_block_stop event (for tool_use), got %d\nOutput: %s", stopCount, output)
	}

	// Verify the ID is translated to Anthropic format in the output (call_xxx)
	if !strings.Contains(output, "call_abc123xyz") {
		t.Errorf("output should contain translated tool ID 'call_abc123xyz', got: %s", output)
	}
}

// TestResponsesStreamProcessor_FunctionCallArgumentsDelta tests that function call arguments
// are properly streamed when the delta is a string (stored in DeltaText by the unmarshaller).
// This test covers the fix for the bug where function_call_arguments.delta events were ignored
// because event.Delta was nil (the delta is a string, not an object).
func TestResponsesStreamProcessor_FunctionCallArgumentsDelta(t *testing.T) {
	var buf bytes.Buffer
	sp := NewResponsesStreamProcessor(&buf, "msg_test", "gpt-5")

	// First: add the function call item (this is how Responses API works)
	addEvent := &models.ResponsesStreamEvent{
		Type: models.EventOutputItemAdded,
		Item: &models.ResponsesItem{
			Type:   "function_call",
			ID:     "fc_test123",
			CallID: "fc_test123",
			Name:   "Task",
		},
	}
	if err := sp.processEvent(addEvent); err != nil {
		t.Fatalf("processEvent (output_item.added) failed: %v", err)
	}

	// Verify tool_use block was started
	output := buf.String()
	if !strings.Contains(output, "tool_use") {
		t.Errorf("output should contain 'tool_use' block type, got: %s", output)
	}
	if !strings.Contains(output, "Task") {
		t.Errorf("output should contain function name 'Task', got: %s", output)
	}

	// Then: stream function call arguments via DeltaText (the actual format)
	// This simulates the real event format: {"delta": "{\"prompt\":", "type": "response.function_call_arguments.delta"}
	deltaEvent1 := &models.ResponsesStreamEvent{
		Type:      models.EventFunctionCallArgs,
		DeltaText: "{\"prompt\":",
	}
	if err := sp.processEvent(deltaEvent1); err != nil {
		t.Fatalf("processEvent (function_call_arguments.delta 1) failed: %v", err)
	}

	deltaEvent2 := &models.ResponsesStreamEvent{
		Type:      models.EventFunctionCallArgs,
		DeltaText: "\"Hello world\"}",
	}
	if err := sp.processEvent(deltaEvent2); err != nil {
		t.Fatalf("processEvent (function_call_arguments.delta 2) failed: %v", err)
	}

	// Verify output contains the argument deltas as input_json_delta
	// Note: The arguments appear JSON-escaped in the SSE output (inside partial_json)
	output = buf.String()
	if !strings.Contains(output, "input_json_delta") {
		t.Errorf("output should contain 'input_json_delta', got: %s", output)
	}
	// The first chunk is JSON-escaped as: {\"prompt\":
	if !strings.Contains(output, `{\"prompt\":`) {
		t.Errorf("output should contain first argument chunk (escaped), got: %s", output)
	}
	// The second chunk is JSON-escaped as: \"Hello world\"}
	if !strings.Contains(output, `\"Hello world\"}`) {
		t.Errorf("output should contain second argument chunk (escaped), got: %s", output)
	}
}

// Helper to verify JSON structure in output
func parseSSEEvents(output string) ([]map[string]interface{}, error) {
	var events []map[string]interface{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				continue
			}
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue // Skip non-JSON lines
			}
			events = append(events, event)
		}
	}

	return events, nil
}

// Package translator provides protocol translation between Anthropic and OpenAI formats.
package translator

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jedarden/clasp/pkg/models"
)

func TestMapFinishReason(t *testing.T) {
	tests := []struct {
		reason   string
		expected string
	}{
		{"stop", "end_turn"},
		{"tool_calls", "tool_use"},
		{"length", "max_tokens"},
		{"content_filter", "end_turn"},
		{"unknown", "end_turn"},
		{"", "end_turn"},
	}

	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			result := mapFinishReason(tt.reason)
			if result != tt.expected {
				t.Errorf("mapFinishReason(%q) = %q, want %q", tt.reason, result, tt.expected)
			}
		})
	}
}

func TestNewStreamProcessor(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	if sp.messageID != "msg_123" {
		t.Errorf("messageID = %q, want %q", sp.messageID, "msg_123")
	}
	if sp.targetModel != "gpt-4o" {
		t.Errorf("targetModel = %q, want %q", sp.targetModel, "gpt-4o")
	}
	if sp.state != StateIdle {
		t.Errorf("state = %v, want %v", sp.state, StateIdle)
	}
	if sp.textBlockIndex != 0 {
		t.Errorf("textBlockIndex = %d, want 0", sp.textBlockIndex)
	}
}

func TestStreamProcessor_SetUsageCallback(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	var calledWith struct {
		input, output int
	}

	sp.SetUsageCallback(func(input, output int) {
		calledWith.input = input
		calledWith.output = output
	})

	// Simulate processing with usage
	input := "data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n"
	sp.ProcessStream(strings.NewReader(input))

	if calledWith.input != 10 {
		t.Errorf("callback input = %d, want 10", calledWith.input)
	}
	if calledWith.output != 5 {
		t.Errorf("callback output = %d, want 5", calledWith.output)
	}
}

func TestStreamProcessor_GetUsage(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	// Before processing, should return 0, 0
	input, output := sp.GetUsage()
	if input != 0 || output != 0 {
		t.Errorf("GetUsage() before processing = (%d, %d), want (0, 0)", input, output)
	}

	// After processing with usage
	streamInput := "data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}],\"usage\":{\"prompt_tokens\":15,\"completion_tokens\":8}}\n\ndata: [DONE]\n"
	sp.ProcessStream(strings.NewReader(streamInput))

	input, output = sp.GetUsage()
	if input != 15 {
		t.Errorf("GetUsage() input = %d, want 15", input)
	}
	if output != 8 {
		t.Errorf("GetUsage() output = %d, want 8", output)
	}
}

func TestStreamProcessor_ProcessStream_TextContent(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	input := `data: {"choices":[{"delta":{"content":"Hello"}}]}

data: {"choices":[{"delta":{"content":" world"}}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Check for expected events
	expectedEvents := []string{
		"event: message_start",
		"event: ping",
		"event: content_block_start",
		"event: content_block_delta",
		"\"text\":\"Hello\"",
		"\"text\":\" world\"",
		"event: content_block_stop",
		"event: message_delta",
		"event: message_stop",
		"data: [DONE]",
	}

	for _, expected := range expectedEvents {
		if !strings.Contains(output, expected) {
			t.Errorf("Output missing %q", expected)
		}
	}
}

func TestStreamProcessor_ProcessStream_ToolCall(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	input := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_123","function":{"name":"get_weather","arguments":""}}]}}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"loc"}}]}}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ation\":\"NYC\"}"}}]}}]}

data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Check for tool call events
	expectedEvents := []string{
		"event: message_start",
		"event: content_block_start",
		"\"type\":\"tool_use\"",
		"\"id\":\"call_123\"",
		"\"name\":\"get_weather\"",
		"event: content_block_delta",
		"input_json_delta",
		"event: content_block_stop",
		"event: message_delta",
		"\"stop_reason\":\"tool_use\"",
	}

	for _, expected := range expectedEvents {
		if !strings.Contains(output, expected) {
			t.Errorf("Output missing %q", expected)
		}
	}
}

func TestStreamProcessor_ProcessStream_MixedContent(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	input := `data: {"choices":[{"delta":{"content":"Let me check the weather."}}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_456","function":{"name":"get_weather","arguments":""}}]}}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"NYC\"}"}}]}}]}

data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Should have text content followed by tool use
	if !strings.Contains(output, "\"text\":\"Let me check the weather.\"") {
		t.Error("Output missing text content")
	}
	if !strings.Contains(output, "\"type\":\"tool_use\"") {
		t.Error("Output missing tool_use block")
	}
}

func TestStreamProcessor_ProcessStream_EmptyLines(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	// Stream with extra empty lines
	input := `

data: {"choices":[{"delta":{"content":"Test"}}]}


data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]

`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	// Should complete without error
	if !strings.Contains(buf.String(), "\"text\":\"Test\"") {
		t.Error("Output missing expected content")
	}
}

func TestStreamProcessor_ProcessStream_InvalidJSON(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	// Stream with invalid JSON (should be skipped)
	input := `data: {invalid json}

data: {"choices":[{"delta":{"content":"Valid"}}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	// Should still process valid data
	if !strings.Contains(buf.String(), "\"text\":\"Valid\"") {
		t.Error("Output missing valid content")
	}
}

func TestStreamProcessor_WriteSSE(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	err := sp.writeSSE("test_event", "{\"data\":\"test\"}")
	if err != nil {
		t.Fatalf("writeSSE failed: %v", err)
	}

	expected := "event: test_event\ndata: {\"data\":\"test\"}\n\n"
	if buf.String() != expected {
		t.Errorf("writeSSE output = %q, want %q", buf.String(), expected)
	}
}

func TestStreamProcessor_WriteSSE_NoEvent(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	err := sp.writeSSE("", "[DONE]")
	if err != nil {
		t.Fatalf("writeSSE failed: %v", err)
	}

	expected := "data: [DONE]\n\n"
	if buf.String() != expected {
		t.Errorf("writeSSE output = %q, want %q", buf.String(), expected)
	}
}

func TestStreamProcessor_WriteEvent(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	event := models.PingEvent{Type: "ping"}
	err := sp.writeEvent("ping", event)
	if err != nil {
		t.Fatalf("writeEvent failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "event: ping") {
		t.Error("Output missing event line")
	}
	if !strings.Contains(output, "\"type\":\"ping\"") {
		t.Error("Output missing event data")
	}
}

func TestStreamProcessor_EmitMessageStart(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_test", "gpt-4o")

	err := sp.emitMessageStart()
	if err != nil {
		t.Fatalf("emitMessageStart failed: %v", err)
	}

	output := buf.String()

	// Should emit message_start then ping
	if !strings.Contains(output, "event: message_start") {
		t.Error("Output missing message_start event")
	}
	if !strings.Contains(output, "event: ping") {
		t.Error("Output missing ping event")
	}
	if !strings.Contains(output, "\"id\":\"msg_test\"") {
		t.Error("Output missing message ID")
	}
	if !strings.Contains(output, "\"model\":\"gpt-4o\"") {
		t.Error("Output missing model")
	}
}

func TestStreamProcessor_EmitContentBlockStart_Text(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	err := sp.emitContentBlockStart(0, "text", "", "")
	if err != nil {
		t.Fatalf("emitContentBlockStart failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "event: content_block_start") {
		t.Error("Output missing event")
	}
	if !strings.Contains(output, "\"type\":\"text\"") {
		t.Error("Output missing type")
	}
	if !strings.Contains(output, "\"index\":0") {
		t.Error("Output missing index")
	}
}

func TestStreamProcessor_EmitContentBlockStart_ToolUse(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	err := sp.emitContentBlockStart(1, "tool_use", "call_123", "get_weather")
	if err != nil {
		t.Fatalf("emitContentBlockStart failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "\"type\":\"tool_use\"") {
		t.Error("Output missing type")
	}
	if !strings.Contains(output, "\"id\":\"call_123\"") {
		t.Error("Output missing id")
	}
	if !strings.Contains(output, "\"name\":\"get_weather\"") {
		t.Error("Output missing name")
	}
}

func TestStreamProcessor_EmitContentBlockDelta_Text(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	err := sp.emitContentBlockDelta(0, "text_delta", "Hello", "")
	if err != nil {
		t.Fatalf("emitContentBlockDelta failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "event: content_block_delta") {
		t.Error("Output missing event")
	}
	if !strings.Contains(output, "\"type\":\"text_delta\"") {
		t.Error("Output missing type")
	}
	if !strings.Contains(output, "\"text\":\"Hello\"") {
		t.Error("Output missing text")
	}
}

func TestStreamProcessor_EmitContentBlockDelta_JSON(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	err := sp.emitContentBlockDelta(0, "input_json_delta", "", "{\"key\":")
	if err != nil {
		t.Fatalf("emitContentBlockDelta failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "\"type\":\"input_json_delta\"") {
		t.Error("Output missing type")
	}
	if !strings.Contains(output, "\"partial_json\":\"{\\\"key\\\":\"") {
		t.Error("Output missing partial_json")
	}
}

func TestStreamProcessor_EmitContentBlockStop(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	err := sp.emitContentBlockStop(0)
	if err != nil {
		t.Fatalf("emitContentBlockStop failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "event: content_block_stop") {
		t.Error("Output missing event")
	}
	if !strings.Contains(output, "\"index\":0") {
		t.Error("Output missing index")
	}
}

func TestStreamProcessor_EmitMessageDelta(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")
	sp.usage = &models.Usage{CompletionTokens: 42}

	err := sp.emitMessageDelta("end_turn")
	if err != nil {
		t.Fatalf("emitMessageDelta failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "event: message_delta") {
		t.Error("Output missing event")
	}
	if !strings.Contains(output, "\"stop_reason\":\"end_turn\"") {
		t.Error("Output missing stop_reason")
	}
	if !strings.Contains(output, "\"output_tokens\":42") {
		t.Error("Output missing output_tokens")
	}
}

func TestStreamProcessor_EmitMessageStop(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	err := sp.emitMessageStop()
	if err != nil {
		t.Fatalf("emitMessageStop failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "event: message_stop") {
		t.Error("Output missing event")
	}
}

func TestStreamProcessor_ProcessChunk_TracksUsage(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")

	chunk := &models.OpenAIStreamChunk{
		Choices: []models.StreamChoice{
			{Delta: models.StreamDelta{Content: "Hi"}},
		},
		Usage: &models.Usage{
			PromptTokens:     20,
			CompletionTokens: 10,
		},
	}

	err := sp.processChunk(chunk)
	if err != nil {
		t.Fatalf("processChunk failed: %v", err)
	}

	if sp.usage == nil {
		t.Fatal("usage should not be nil")
	}
	if sp.usage.PromptTokens != 20 {
		t.Errorf("usage.PromptTokens = %d, want 20", sp.usage.PromptTokens)
	}
	if sp.usage.CompletionTokens != 10 {
		t.Errorf("usage.CompletionTokens = %d, want 10", sp.usage.CompletionTokens)
	}
}

func TestStreamProcessor_HandleFinishReason_StopReasons(t *testing.T) {
	tests := []struct {
		reason       string
		expectedStop string
	}{
		{"stop", "end_turn"},
		{"tool_calls", "tool_use"},
		{"length", "max_tokens"},
	}

	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			var buf bytes.Buffer
			sp := NewStreamProcessor(&buf, "msg_123", "gpt-4o")
			sp.state = StateTextContent
			sp.textStarted = true

			err := sp.handleFinishReason(tt.reason)
			if err != nil {
				t.Fatalf("handleFinishReason failed: %v", err)
			}

			output := buf.String()
			expected := "\"stop_reason\":\"" + tt.expectedStop + "\""
			if !strings.Contains(output, expected) {
				t.Errorf("Output missing %q", expected)
			}
		})
	}
}

func TestStreamState_Constants(t *testing.T) {
	// Verify state constants exist and have correct values
	if StateIdle != 0 {
		t.Errorf("StateIdle = %d, want 0", StateIdle)
	}
	if StateMessageStarted != 1 {
		t.Errorf("StateMessageStarted = %d, want 1", StateMessageStarted)
	}
	if StateTextContent != 2 {
		t.Errorf("StateTextContent = %d, want 2", StateTextContent)
	}
	if StateToolCall != 3 {
		t.Errorf("StateToolCall = %d, want 3", StateToolCall)
	}
	if StateDone != 4 {
		t.Errorf("StateDone = %d, want 4", StateDone)
	}
}

// Helper to parse SSE event data
func parseSSEEvent(line string) (string, map[string]interface{}, error) {
	parts := strings.SplitN(line, "\n", 2)
	if len(parts) < 2 {
		return "", nil, nil
	}

	eventType := strings.TrimPrefix(parts[0], "event: ")
	dataLine := strings.TrimPrefix(parts[1], "data: ")

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(dataLine), &data); err != nil {
		return eventType, nil, err
	}

	return eventType, data, nil
}

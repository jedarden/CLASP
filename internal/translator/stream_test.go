// Package translator provides protocol translation between Anthropic and OpenAI formats.
package translator

import (
	"bytes"
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

// Grok XML extraction tests

func TestIsGrokModelStream(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"x-ai/grok-3-beta", true},
		{"grok-3-mini", true},
		{"x-ai/grok-2", true},
		{"GROK-3", true},
		{"gpt-4o", false},
		{"claude-3", false},
		{"o1", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := isGrokModelStream(tt.model)
			if result != tt.expected {
				t.Errorf("isGrokModelStream(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestParseXMLParameters(t *testing.T) {
	xmlContent := `<xai:parameter name="location">NYC</xai:parameter><xai:parameter name="unit">celsius</xai:parameter>`

	params := parseXMLParameters(xmlContent)

	if params["location"] != "NYC" {
		t.Errorf("location = %v, want %q", params["location"], "NYC")
	}
	if params["unit"] != "celsius" {
		t.Errorf("unit = %v, want %q", params["unit"], "celsius")
	}
}

func TestParseXMLParameters_JSONValue(t *testing.T) {
	xmlContent := `<xai:parameter name="count">42</xai:parameter><xai:parameter name="enabled">true</xai:parameter>`

	params := parseXMLParameters(xmlContent)

	// Numbers and booleans should be parsed from JSON
	if params["count"] != float64(42) {
		t.Errorf("count = %v (type %T), want 42", params["count"], params["count"])
	}
	if params["enabled"] != true {
		t.Errorf("enabled = %v, want true", params["enabled"])
	}
}

func TestProcessGrokXML_CompleteToolCall(t *testing.T) {
	sp := NewStreamProcessor(&bytes.Buffer{}, "msg_123", "x-ai/grok-3-beta")

	text := `Here is the result: <xai:function_call name="get_weather"><xai:parameter name="location">NYC</xai:parameter></xai:function_call>`

	cleanedText, toolCalls := sp.processGrokXML(text)

	if strings.Contains(cleanedText, "<xai:function_call") {
		t.Error("Cleaned text should not contain XML")
	}
	if len(toolCalls) != 1 {
		t.Fatalf("len(toolCalls) = %d, want 1", len(toolCalls))
	}
	if toolCalls[0].name != "get_weather" {
		t.Errorf("toolCalls[0].name = %q, want %q", toolCalls[0].name, "get_weather")
	}
	if toolCalls[0].arguments["location"] != "NYC" {
		t.Errorf("location argument = %v, want %q", toolCalls[0].arguments["location"], "NYC")
	}
}

func TestProcessGrokXML_PartialXML_Buffering(t *testing.T) {
	sp := NewStreamProcessor(&bytes.Buffer{}, "msg_123", "x-ai/grok-3-beta")

	// First chunk: partial XML
	text1 := `Let me call the tool: <xai:function_call name="test"`
	cleanedText, toolCalls := sp.processGrokXML(text1)

	// Should buffer and not emit anything yet
	if cleanedText != "" {
		t.Errorf("Should not emit text with partial XML, got %q", cleanedText)
	}
	if len(toolCalls) != 0 {
		t.Error("Should not extract tool calls from partial XML")
	}

	// Second chunk: complete the XML
	text2 := `><xai:parameter name="x">1</xai:parameter></xai:function_call>`
	_, toolCalls = sp.processGrokXML(text2)

	if len(toolCalls) != 1 {
		t.Fatalf("len(toolCalls) = %d, want 1 after completing XML", len(toolCalls))
	}
	if toolCalls[0].name != "test" {
		t.Errorf("toolCalls[0].name = %q, want %q", toolCalls[0].name, "test")
	}
}

func TestProcessGrokXML_NoXML(t *testing.T) {
	sp := NewStreamProcessor(&bytes.Buffer{}, "msg_123", "x-ai/grok-3-beta")

	text := "This is just regular text without any XML."
	cleanedText, toolCalls := sp.processGrokXML(text)

	if cleanedText != text {
		t.Errorf("cleanedText = %q, want %q", cleanedText, text)
	}
	if len(toolCalls) != 0 {
		t.Error("Should not extract any tool calls from text without XML")
	}
}

func TestProcessGrokXML_MultipleToolCalls(t *testing.T) {
	sp := NewStreamProcessor(&bytes.Buffer{}, "msg_123", "x-ai/grok-3-beta")

	text := `<xai:function_call name="func1"><xai:parameter name="a">1</xai:parameter></xai:function_call> and <xai:function_call name="func2"><xai:parameter name="b">2</xai:parameter></xai:function_call>`

	cleanedText, toolCalls := sp.processGrokXML(text)

	if len(toolCalls) != 2 {
		t.Fatalf("len(toolCalls) = %d, want 2", len(toolCalls))
	}
	if toolCalls[0].name != "func1" {
		t.Errorf("toolCalls[0].name = %q, want %q", toolCalls[0].name, "func1")
	}
	if toolCalls[1].name != "func2" {
		t.Errorf("toolCalls[1].name = %q, want %q", toolCalls[1].name, "func2")
	}
	if strings.Contains(cleanedText, "<xai:function_call") {
		t.Error("Cleaned text should not contain XML")
	}
}

// Thinking/Reasoning block tests (for O1/O3 models)

func TestStreamProcessor_ProcessStream_ThinkingContent(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "o1-preview")

	// Stream with reasoning content (as provided by some providers)
	input := `data: {"choices":[{"delta":{"reasoning":"Let me think about this..."}}]}

data: {"choices":[{"delta":{"reasoning":" I should analyze the problem."}}]}

data: {"choices":[{"delta":{"content":"Here's my answer."}}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Check for thinking block events
	expectedEvents := []string{
		"event: message_start",
		"event: content_block_start",
		"\"type\":\"thinking\"",
		"event: content_block_delta",
		"\"type\":\"thinking_delta\"",
		"\"thinking\":\"Let me think about this...\"",
		"\"thinking\":\" I should analyze the problem.\"",
		"event: content_block_stop",  // thinking block stop
		"event: content_block_start", // text block start
		"\"type\":\"text\"",
		"\"text\":\"Here's my answer.\"",
		"event: content_block_stop", // text block stop
		"event: message_delta",
		"event: message_stop",
	}

	for _, expected := range expectedEvents {
		if !strings.Contains(output, expected) {
			t.Errorf("Output missing %q", expected)
		}
	}
}

func TestStreamProcessor_ThinkingBlockIndex(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "o1")

	// Verify initial state
	if sp.thinkingBlockIndex != -1 {
		t.Errorf("thinkingBlockIndex initial = %d, want -1", sp.thinkingBlockIndex)
	}
	if sp.textBlockIndex != 0 {
		t.Errorf("textBlockIndex initial = %d, want 0", sp.textBlockIndex)
	}

	// Process thinking content
	err := sp.handleThinkingContent("Thinking...")
	if err != nil {
		t.Fatalf("handleThinkingContent failed: %v", err)
	}

	// After thinking starts, thinking is at 0, text shifts to 1
	if sp.thinkingBlockIndex != 0 {
		t.Errorf("thinkingBlockIndex after thinking = %d, want 0", sp.thinkingBlockIndex)
	}
	if sp.textBlockIndex != 1 {
		t.Errorf("textBlockIndex after thinking = %d, want 1", sp.textBlockIndex)
	}
}

func TestStreamProcessor_EmitThinkingBlockStart(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "o1")

	err := sp.emitThinkingBlockStart(0)
	if err != nil {
		t.Fatalf("emitThinkingBlockStart failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "event: content_block_start") {
		t.Error("Output missing event")
	}
	if !strings.Contains(output, "\"type\":\"thinking\"") {
		t.Error("Output missing thinking type")
	}
	if !strings.Contains(output, "\"index\":0") {
		t.Error("Output missing index")
	}
}

func TestStreamProcessor_EmitThinkingBlockDelta(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "o1")

	err := sp.emitThinkingBlockDelta(0, "Step 1: analyze the problem")
	if err != nil {
		t.Fatalf("emitThinkingBlockDelta failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "event: content_block_delta") {
		t.Error("Output missing event")
	}
	if !strings.Contains(output, "\"type\":\"thinking_delta\"") {
		t.Error("Output missing thinking_delta type")
	}
	if !strings.Contains(output, "\"thinking\":\"Step 1: analyze the problem\"") {
		t.Error("Output missing thinking content")
	}
}

func TestStreamProcessor_ProcessStream_ThinkingThenToolCall(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "o1-preview")

	// Stream with reasoning followed by tool call
	input := `data: {"choices":[{"delta":{"reasoning":"I need to get the weather..."}}]}

data: {"choices":[{"delta":{"content":"Let me check."}}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_123","function":{"name":"get_weather","arguments":""}}]}}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"NYC\"}"}}]}}]}

data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Should have all three block types in correct order
	if !strings.Contains(output, "\"type\":\"thinking\"") {
		t.Error("Output missing thinking block")
	}
	if !strings.Contains(output, "\"type\":\"text\"") {
		t.Error("Output missing text block")
	}
	if !strings.Contains(output, "\"type\":\"tool_use\"") {
		t.Error("Output missing tool_use block")
	}
	if !strings.Contains(output, "\"stop_reason\":\"tool_use\"") {
		t.Error("Output missing tool_use stop reason")
	}
}

func TestStreamProcessor_HandleFinishReason_ClosesThinkingBlock(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "o1")

	// Simulate thinking and text content
	sp.state = StateMessageStarted
	sp.thinkingStarted = true
	sp.thinkingBlockIndex = 0
	sp.textStarted = true
	sp.textBlockIndex = 1
	sp.state = StateTextContent

	err := sp.handleFinishReason("stop")
	if err != nil {
		t.Fatalf("handleFinishReason failed: %v", err)
	}

	output := buf.String()

	// Should emit two content_block_stop events (thinking and text)
	stopCount := strings.Count(output, "event: content_block_stop")
	if stopCount != 2 {
		t.Errorf("Expected 2 content_block_stop events, got %d", stopCount)
	}
}

func TestStreamProcessor_ThinkingOnlyNoText(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_123", "o1")

	// Stream with only thinking, no regular content
	input := `data: {"choices":[{"delta":{"reasoning":"Thinking deeply..."}}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "\"type\":\"thinking\"") {
		t.Error("Output missing thinking block")
	}
	if !strings.Contains(output, "event: content_block_stop") {
		t.Error("Output missing content_block_stop")
	}
	if !strings.Contains(output, "event: message_stop") {
		t.Error("Output missing message_stop")
	}
}

func TestStreamProcessor_ReasoningContent_KimiK25(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_kimi", "kimi-k2.5")

	// Simulate Kimi K2.5 streaming with reasoning_content field
	input := `data: {"choices":[{"delta":{"reasoning_content":"Let me analyze this problem step by step."}}]}

data: {"choices":[{"delta":{"reasoning_content":"First, I'll break it down."}}]}

data: {"choices":[{"delta":{"content":"Based on my analysis, the answer is 42."}}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Should have thinking block (from reasoning_content) and text block
	if !strings.Contains(output, "\"type\":\"thinking\"") {
		t.Error("Output missing thinking block from reasoning_content")
	}
	if !strings.Contains(output, "\"thinking\":\"Let me analyze this problem step by step.") {
		t.Error("Output missing initial thinking content")
	}
	if !strings.Contains(output, "\"thinking\":\"First, I'll break it down.") {
		t.Error("Output missing second thinking delta")
	}
	if !strings.Contains(output, "\"type\":\"text\"") {
		t.Error("Output missing text block")
	}
	if !strings.Contains(output, "\"text\":\"Based on my analysis, the answer is 42.") {
		t.Error("Output missing text content")
	}
	if !strings.Contains(output, "\"stop_reason\":\"end_turn\"") {
		t.Error("Output missing stop reason")
	}

	// Verify block ordering: thinking (index 0) before text (index 1)
	thinkingStartIndex := strings.Index(output, `"type":"thinking"`)
	textStartIndex := strings.Index(output, `"type":"text"`)
	if thinkingStartIndex < 0 || textStartIndex < 0 {
		t.Fatal("Could not find block start markers")
	}
	if thinkingStartIndex > textStartIndex {
		t.Error("Thinking block should come before text block")
	}
}

func TestStreamProcessor_ReasoningContent_DeepSeekR1(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_deepseek", "deepseek-r1")

	// Simulate DeepSeek-R1 streaming with reasoning_content
	input := `data: {"choices":[{"delta":{"reasoning_content":"This requires careful reasoning."}}]}

data: {"choices":[{"delta":{"reasoning_content":"Let me think through the logic."}}]}

data: {"choices":[{"delta":{"content":"Therefore, the solution is optimal."}}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Verify thinking content is emitted
	if !strings.Contains(output, "\"type\":\"thinking\"") {
		t.Error("Output missing thinking block")
	}
	if !strings.Contains(output, "\"thinking\":\"This requires careful reasoning.") {
		t.Error("Output missing first thinking delta")
	}
	if !strings.Contains(output, "\"thinking\":\"Let me think through the logic.") {
		t.Error("Output missing second thinking delta")
	}
	if !strings.Contains(output, "\"type\":\"text\"") {
		t.Error("Output missing text block")
	}

	// Verify content_block_stop for thinking block
	if !strings.Contains(output, "event: content_block_stop") {
		t.Error("Output missing content_block_stop")
	}
}

func TestStreamProcessor_ReasoningContent_WithToolCalls(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_tools", "kimi-k2.5")

	// Simulate reasoning_content followed by tool call
	input := `data: {"choices":[{"delta":{"reasoning_content":"I need to use a tool for this."}}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_xyz","function":{"name":"search","arguments":""}}]}}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"query\":\"test\"}"}}]}}]}

data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Should have thinking block and tool_use block
	if !strings.Contains(output, "\"type\":\"thinking\"") {
		t.Error("Output missing thinking block")
	}
	if !strings.Contains(output, "\"thinking\":\"I need to use a tool for this.") {
		t.Error("Output missing thinking content")
	}
	if !strings.Contains(output, "\"type\":\"tool_use\"") {
		t.Error("Output missing tool_use block")
	}
	if !strings.Contains(output, "\"name\":\"search\"") {
		t.Error("Output missing tool name")
	}
	if !strings.Contains(output, "\"stop_reason\":\"tool_use\"") {
		t.Error("Output missing tool_use stop reason")
	}
}

func TestStreamProcessor_ReasoningContent_OnlyNoText(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_only_thinking", "deepseek-r1")

	// Simulate only reasoning_content with no text content
	input := `data: {"choices":[{"delta":{"reasoning_content":"Thinking deeply about this..."}}]}

data: {"choices":[{"delta":{"reasoning_content":"More analysis..."}}]}

data: {"choices":[{"delta":{},"finish_reason":"length"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Should have thinking block but no text block
	if !strings.Contains(output, "\"type\":\"thinking\"") {
		t.Error("Output missing thinking block")
	}
	if !strings.Contains(output, "\"thinking\":\"Thinking deeply about this...") {
		t.Error("Output missing first thinking delta")
	}
	if !strings.Contains(output, "\"thinking\":\"More analysis...") {
		t.Error("Output missing second thinking delta")
	}

	// Should not have text block
	if strings.Contains(output, "\"type\":\"text\"") {
		t.Error("Output should not have text block when only reasoning_content is present")
	}

	// Verify stop reason
	if !strings.Contains(output, "\"stop_reason\":\"max_tokens\"") {
		t.Error("Output missing max_tokens stop reason")
	}
}

func TestStreamProcessor_BothReasoningFields(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_both", "test-model")

	// Test both reasoning and reasoning_content in same stream (edge case)
	// The processor should handle both
	input := `data: {"choices":[{"delta":{"reasoning":"Azure style reasoning"}}]}

data: {"choices":[{"delta":{"reasoning_content":"Kimi style reasoning"}}]}

data: {"choices":[{"delta":{"content":"Final answer"}}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Should emit thinking blocks for both fields
	if !strings.Contains(output, "\"type\":\"thinking\"") {
		t.Error("Output missing thinking blocks")
	}

	thinkingCount := strings.Count(output, "\"thinking\":")
	if thinkingCount < 2 {
		t.Errorf("Expected at least 2 thinking deltas, got %d", thinkingCount)
	}

	if !strings.Contains(output, "\"type\":\"text\"") {
		t.Error("Output missing text block")
	}
}

func TestStreamProcessor_ReasoningContent_EmptyString(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_empty", "kimi-k2.5")

	// Empty reasoning_content should not start a thinking block
	input := `data: {"choices":[{"delta":{"reasoning_content":""}}]}

data: {"choices":[{"delta":{"content":"Normal text"}}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Should not have thinking block for empty string
	if strings.Contains(output, "\"type\":\"thinking\"") {
		t.Error("Output should not have thinking block for empty reasoning_content")
	}

	// Should have text block
	if !strings.Contains(output, "\"type\":\"text\"") {
		t.Error("Output missing text block")
	}
}

func TestStreamProcessor_ReasoningContent_AfterText(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_reverse", "test-model")

	// Test reasoning_content appearing after text (unusual but should be handled)
	input := `data: {"choices":[{"delta":{"content":"Text first"}}]}

data: {"choices":[{"delta":{"reasoning_content":"Reasoning after text"}}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Should have both blocks
	if !strings.Contains(output, "\"type\":\"text\"") {
		t.Error("Output missing text block")
	}
	if !strings.Contains(output, "\"type\":\"thinking\"") {
		t.Error("Output missing thinking block")
	}
	// Both blocks should be emitted regardless of order
}

func TestStreamProcessor_ReasoningContent_MultipleChunks(t *testing.T) {
	var buf bytes.Buffer
	sp := NewStreamProcessor(&buf, "msg_chunks", "deepseek-r1")

	// Test multiple reasoning_content chunks accumulating
	input := `data: {"choices":[{"delta":{"reasoning_content":"Chunk 1"}}]}

data: {"choices":[{"delta":{"reasoning_content":"Chunk 2"}}]}

data: {"choices":[{"delta":{"reasoning_content":"Chunk 3"}}]}

data: {"choices":[{"delta":{"content":"Answer"}}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Should have all thinking deltas
	if !strings.Contains(output, "\"thinking\":\"Chunk 1\"") {
		t.Error("Output missing first chunk")
	}
	if !strings.Contains(output, "\"thinking\":\"Chunk 2\"") {
		t.Error("Output missing second chunk")
	}
	if !strings.Contains(output, "\"thinking\":\"Chunk 3\"") {
		t.Error("Output missing third chunk")
	}

	// Verify only one content_block_start for thinking
	startCount := strings.Count(output, `"type":"thinking"`)
	if startCount != 1 {
		t.Errorf("Expected 1 thinking block start, got %d", startCount)
	}
}

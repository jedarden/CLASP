package tests

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jedarden/clasp/internal/translator"
	"github.com/jedarden/clasp/pkg/models"
)

func TestStreamProcessor_TextContent(t *testing.T) {
	// Simulate OpenAI SSE stream
	streamData := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":", world!"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]
`

	var buf bytes.Buffer
	processor := translator.NewStreamProcessor(&buf, "msg_test123", "gpt-4o")
	err := processor.ProcessStream(strings.NewReader(streamData))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Verify essential events are present
	expectedEvents := []string{
		"event: message_start",
		"event: content_block_start",
		"event: content_block_delta",
		"event: content_block_stop",
		"event: message_delta",
		"event: message_stop",
		"data: [DONE]",
	}

	for _, expected := range expectedEvents {
		if !strings.Contains(output, expected) {
			t.Errorf("output missing expected event: %s", expected)
		}
	}

	// Verify text deltas contain our content
	if !strings.Contains(output, "Hello") {
		t.Error("output should contain 'Hello'")
	}
	if !strings.Contains(output, "world") {
		t.Error("output should contain 'world'")
	}

	// Verify message_start format
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "event: message_start") {
			// Next line should be data:
			if i+1 < len(lines) && strings.HasPrefix(lines[i+1], "data: ") {
				data := strings.TrimPrefix(lines[i+1], "data: ")
				var event models.MessageStartEvent
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					t.Errorf("failed to parse message_start event: %v", err)
				}
				if event.Type != "message_start" {
					t.Errorf("expected type 'message_start', got '%s'", event.Type)
				}
				if event.Message.Role != "assistant" {
					t.Errorf("expected role 'assistant', got '%s'", event.Message.Role)
				}
			}
			break
		}
	}
}

func TestStreamProcessor_ToolCall(t *testing.T) {
	// Simulate OpenAI SSE stream with tool call
	streamData := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Let me read that file."},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"Read","arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"file_path\":"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"/src/main.go\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}

data: [DONE]
`

	var buf bytes.Buffer
	processor := translator.NewStreamProcessor(&buf, "msg_test456", "gpt-4o")
	err := processor.ProcessStream(strings.NewReader(streamData))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Verify tool_use block events
	if !strings.Contains(output, `"type":"tool_use"`) {
		t.Error("output should contain tool_use block")
	}

	if !strings.Contains(output, `"name":"Read"`) {
		t.Error("output should contain tool name 'Read'")
	}

	if !strings.Contains(output, `"id":"call_123"`) {
		t.Error("output should contain tool call id 'call_123'")
	}

	// Verify input_json_delta events
	if !strings.Contains(output, `"type":"input_json_delta"`) {
		t.Error("output should contain input_json_delta events")
	}

	// Verify message_delta has tool_use stop reason
	if !strings.Contains(output, `"stop_reason":"tool_use"`) {
		t.Error("output should contain stop_reason 'tool_use'")
	}
}

func TestStreamProcessor_EmptyStream(t *testing.T) {
	streamData := `data: [DONE]
`

	var buf bytes.Buffer
	processor := translator.NewStreamProcessor(&buf, "msg_empty", "gpt-4o")
	err := processor.ProcessStream(strings.NewReader(streamData))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Even empty stream should have message_stop and [DONE]
	if !strings.Contains(output, "event: message_stop") {
		t.Error("output should contain message_stop event")
	}
	if !strings.Contains(output, "data: [DONE]") {
		t.Error("output should contain [DONE]")
	}
}

func TestStreamProcessor_MultipleToolCalls(t *testing.T) {
	// Simulate OpenAI SSE stream with multiple parallel tool calls
	streamData := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_001","type":"function","function":{"name":"Read","arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_002","type":"function","function":{"name":"Glob","arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\":\"/src\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"arguments":"{\"pattern\":\"*.go\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`

	var buf bytes.Buffer
	processor := translator.NewStreamProcessor(&buf, "msg_multi", "gpt-4o")
	err := processor.ProcessStream(strings.NewReader(streamData))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Verify both tool calls are present
	if !strings.Contains(output, `"name":"Read"`) {
		t.Error("output should contain tool 'Read'")
	}
	if !strings.Contains(output, `"name":"Glob"`) {
		t.Error("output should contain tool 'Glob'")
	}
	if !strings.Contains(output, `"id":"call_001"`) {
		t.Error("output should contain call id 'call_001'")
	}
	if !strings.Contains(output, `"id":"call_002"`) {
		t.Error("output should contain call id 'call_002'")
	}
}

func TestStreamProcessor_UsageCallback(t *testing.T) {
	// Simulate OpenAI SSE stream with usage info at the end
	streamData := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":150,"completion_tokens":25,"total_tokens":175}}

data: [DONE]
`

	var buf bytes.Buffer
	processor := translator.NewStreamProcessor(&buf, "msg_usage", "gpt-4o")

	// Track callback invocation
	var callbackInvoked bool
	var receivedInputTokens, receivedOutputTokens int

	processor.SetUsageCallback(func(inputTokens, outputTokens int) {
		callbackInvoked = true
		receivedInputTokens = inputTokens
		receivedOutputTokens = outputTokens
	})

	err := processor.ProcessStream(strings.NewReader(streamData))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	// Verify callback was invoked with correct values
	if !callbackInvoked {
		t.Error("usage callback was not invoked")
	}
	if receivedInputTokens != 150 {
		t.Errorf("expected 150 input tokens, got %d", receivedInputTokens)
	}
	if receivedOutputTokens != 25 {
		t.Errorf("expected 25 output tokens, got %d", receivedOutputTokens)
	}

	// Verify GetUsage also returns correct values
	gotInput, gotOutput := processor.GetUsage()
	if gotInput != 150 {
		t.Errorf("GetUsage: expected 150 input tokens, got %d", gotInput)
	}
	if gotOutput != 25 {
		t.Errorf("GetUsage: expected 25 output tokens, got %d", gotOutput)
	}
}

func TestStreamProcessor_UsageCallback_NoUsageData(t *testing.T) {
	// Simulate OpenAI SSE stream WITHOUT usage info (some providers don't send it)
	streamData := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`

	var buf bytes.Buffer
	processor := translator.NewStreamProcessor(&buf, "msg_nousage", "gpt-4o")

	// Track callback invocation - should NOT be called when no usage data
	var callbackInvoked bool

	processor.SetUsageCallback(func(inputTokens, outputTokens int) {
		callbackInvoked = true
	})

	err := processor.ProcessStream(strings.NewReader(streamData))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	// Callback should not be invoked when no usage data
	if callbackInvoked {
		t.Error("usage callback should not be invoked when no usage data is present")
	}

	// GetUsage should return zeros
	gotInput, gotOutput := processor.GetUsage()
	if gotInput != 0 || gotOutput != 0 {
		t.Errorf("GetUsage: expected 0/0 without usage data, got %d/%d", gotInput, gotOutput)
	}
}

func TestStreamProcessor_UsageCallback_LateUsage(t *testing.T) {
	// Simulate stream where usage comes in a separate final chunk (OpenAI behavior)
	streamData := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Test response"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[],"usage":{"prompt_tokens":200,"completion_tokens":50,"total_tokens":250}}

data: [DONE]
`

	var buf bytes.Buffer
	processor := translator.NewStreamProcessor(&buf, "msg_lateusage", "gpt-4o")

	var receivedInputTokens, receivedOutputTokens int

	processor.SetUsageCallback(func(inputTokens, outputTokens int) {
		receivedInputTokens = inputTokens
		receivedOutputTokens = outputTokens
	})

	err := processor.ProcessStream(strings.NewReader(streamData))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	// Verify usage was captured even from late chunk
	if receivedInputTokens != 200 {
		t.Errorf("expected 200 input tokens, got %d", receivedInputTokens)
	}
	if receivedOutputTokens != 50 {
		t.Errorf("expected 50 output tokens, got %d", receivedOutputTokens)
	}
}

func BenchmarkStreamProcessor(b *testing.B) {
	streamData := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello, "},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"world!"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		processor := translator.NewStreamProcessor(&buf, "msg_bench", "gpt-4o")
		_ = processor.ProcessStream(strings.NewReader(streamData))
	}
}

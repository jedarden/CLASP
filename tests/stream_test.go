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

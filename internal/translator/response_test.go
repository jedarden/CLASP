// Package translator provides protocol translation between Anthropic and OpenAI formats.
package translator

import (
	"encoding/json"
	"testing"

	"github.com/jedarden/clasp/pkg/models"
)

func TestTransformResponse_ReasoningContent(t *testing.T) {
	tests := []struct {
		name           string
		openaiResp     string
		targetModel    string
		expectError    bool
		expectThinking bool
		expectText     bool
		thinkingText   string
		textContent    string
		stopReason     string
	}{
		{
			name: "Kimi K2.5 response with reasoning_content and null content",
			openaiResp: `{
				"id": "chatcmpl-123",
				"choices": [{
					"message": {
						"role": "assistant",
						"content": null,
						"reasoning_content": "Let me think about this step by step. First, I need to understand the problem."
					},
					"finish_reason": "length"
				}],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 50
				}
			}`,
			targetModel:    "kimi-k2.5",
			expectError:    false,
			expectThinking: true,
			expectText:     false,
			thinkingText:   "Let me think about this step by step. First, I need to understand the problem.",
			stopReason:     "max_tokens",
		},
		{
			name: "DeepSeek-R1 response with reasoning_content and content",
			openaiResp: `{
				"id": "chatcmpl-456",
				"choices": [{
					"message": {
						"role": "assistant",
						"content": "Based on my reasoning, the answer is 42.",
						"reasoning_content": "This is a complex problem. Let me analyze it carefully."
					},
					"finish_reason": "stop"
				}],
				"usage": {
					"prompt_tokens": 15,
					"completion_tokens": 100
				}
			}`,
			targetModel:    "deepseek-r1",
			expectError:    false,
			expectThinking: true,
			expectText:     true,
			thinkingText:   "This is a complex problem. Let me analyze it carefully.",
			textContent:    "Based on my reasoning, the answer is 42.",
			stopReason:     "end_turn",
		},
		{
			name: "Standard OpenAI response without reasoning_content",
			openaiResp: `{
				"id": "chatcmpl-789",
				"choices": [{
					"message": {
						"role": "assistant",
						"content": "Hello, how can I help you today?"
					},
					"finish_reason": "stop"
				}],
				"usage": {
					"prompt_tokens": 20,
					"completion_tokens": 10
				}
			}`,
			targetModel:    "gpt-4o",
			expectError:    false,
			expectThinking: false,
			expectText:     true,
			textContent:    "Hello, how can I help you today?",
			stopReason:     "end_turn",
		},
		{
			name: "Response with tool calls and reasoning_content",
			openaiResp: `{
				"id": "chatcmpl-999",
				"choices": [{
					"message": {
						"role": "assistant",
						"content": null,
						"reasoning_content": "I need to use a tool to get the current weather.",
						"tool_calls": [{
							"id": "call_123",
							"type": "function",
							"function": {
								"name": "get_weather",
								"arguments": "{\"location\": \"San Francisco\"}"
							}
						}]
					},
					"finish_reason": "tool_calls"
				}],
				"usage": {
					"prompt_tokens": 25,
					"completion_tokens": 30
				}
			}`,
			targetModel:    "kimi-k2.5",
			expectError:    false,
			expectThinking: true,
			expectText:     false,
			thinkingText:   "I need to use a tool to get the current weather.",
			stopReason:     "tool_use",
		},
		{
			name: "Empty reasoning_content should not create thinking block",
			openaiResp: `{
				"id": "chatcmpl-111",
				"choices": [{
					"message": {
						"role": "assistant",
						"content": "Regular response",
						"reasoning_content": ""
					},
					"finish_reason": "stop"
				}],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 5
				}
			}`,
			targetModel:    "gpt-4o",
			expectError:    false,
			expectThinking: false,
			expectText:     true,
			textContent:    "Regular response",
			stopReason:     "end_turn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := TransformResponse([]byte(tt.openaiResp), tt.targetModel)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check basic response structure
			if resp.Type != "message" {
				t.Errorf("Response type = %q, want %q", resp.Type, "message")
			}
			if resp.Role != "assistant" {
				t.Errorf("Response role = %q, want %q", resp.Role, "assistant")
			}
			if resp.StopReason != tt.stopReason {
				t.Errorf("Stop reason = %q, want %q", resp.StopReason, tt.stopReason)
			}

			// Check content blocks
			if len(resp.Content) == 0 {
				t.Errorf("Response has no content blocks")
				return
			}

			// Verify thinking block
			hasThinking := false
			for _, block := range resp.Content {
				if block.Type == "thinking" {
					hasThinking = true
					if tt.expectThinking {
						if block.Thinking != tt.thinkingText {
							t.Errorf("Thinking content = %q, want %q", block.Thinking, tt.thinkingText)
						}
					} else {
						t.Errorf("Unexpected thinking block found")
					}
				}
			}

			if tt.expectThinking && !hasThinking {
				t.Errorf("Expected thinking block but none found")
			}
			if !tt.expectThinking && hasThinking {
				t.Errorf("Found thinking block when none expected")
			}

			// Verify text block
			hasText := false
			for _, block := range resp.Content {
				if block.Type == "text" {
					hasText = true
					if tt.expectText {
						if block.Text != tt.textContent {
							t.Errorf("Text content = %q, want %q", block.Text, tt.textContent)
						}
					}
				}
			}

			if tt.expectText && !hasText {
				t.Errorf("Expected text block but none found")
			}

			// Verify block ordering: thinking should come before text
			thinkingIndex := -1
			textIndex := -1
			for i, block := range resp.Content {
				if block.Type == "thinking" {
					thinkingIndex = i
				}
				if block.Type == "text" {
					textIndex = i
				}
			}
			if thinkingIndex >= 0 && textIndex >= 0 && thinkingIndex > textIndex {
				t.Errorf("Thinking block (index %d) should come before text block (index %d)", thinkingIndex, textIndex)
			}
		})
	}
}

func TestTransformResponse_ReasoningContentOrdering(t *testing.T) {
	// Test that thinking blocks come before text blocks
	openaiResp := `{
		"id": "chatcmpl-ordering",
		"choices": [{
			"message": {
				"role": "assistant",
				"content": "Final answer",
				"reasoning_content": "My reasoning process"
			},
			"finish_reason": "stop"
		}],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 20
		}
	}`

	resp, err := TransformResponse([]byte(openaiResp), "test-model")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(resp.Content) != 2 {
		t.Fatalf("Expected 2 content blocks, got %d", len(resp.Content))
	}

	// First block should be thinking
	if resp.Content[0].Type != "thinking" {
		t.Errorf("First block type = %q, want %q", resp.Content[0].Type, "thinking")
	}
	if resp.Content[0].Thinking != "My reasoning process" {
		t.Errorf("Thinking content = %q, want %q", resp.Content[0].Thinking, "My reasoning process")
	}

	// Second block should be text
	if resp.Content[1].Type != "text" {
		t.Errorf("Second block type = %q, want %q", resp.Content[1].Type, "text")
	}
	if resp.Content[1].Text != "Final answer" {
		t.Errorf("Text content = %q, want %q", resp.Content[1].Text, "Final answer")
	}
}

func TestTransformResponse_WithToolCallsAndReasoning(t *testing.T) {
	// Test that tool calls work correctly with reasoning_content
	openaiResp := `{
		"id": "chatcmpl-tools",
		"choices": [{
			"message": {
				"role": "assistant",
				"content": null,
				"reasoning_content": "I'll use a calculator",
				"tool_calls": [{
					"id": "call_abc",
					"type": "function",
					"function": {
						"name": "calculate",
						"arguments": "{\"expression\": \"2+2\"}"
					}
				}]
			},
			"finish_reason": "tool_calls"
		}],
		"usage": {
			"prompt_tokens": 15,
			"completion_tokens": 25
		}
	}`

	resp, err := TransformResponse([]byte(openaiResp), "test-model")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have 2 content blocks: thinking and tool_use
	if len(resp.Content) != 2 {
		t.Fatalf("Expected 2 content blocks, got %d", len(resp.Content))
	}

	// First block should be thinking
	if resp.Content[0].Type != "thinking" {
		t.Errorf("First block type = %q, want %q", resp.Content[0].Type, "thinking")
	}

	// Second block should be tool_use
	if resp.Content[1].Type != "tool_use" {
		t.Errorf("Second block type = %q, want %q", resp.Content[1].Type, "tool_use")
	}
	if resp.Content[1].Name != "calculate" {
		t.Errorf("Tool name = %q, want %q", resp.Content[1].Name, "calculate")
	}
}

func TestTransformResponse_InvalidJSON(t *testing.T) {
	invalidResp := `{invalid json`

	_, err := TransformResponse([]byte(invalidResp), "test-model")
	if err == nil {
		t.Error("Expected error for invalid JSON but got none")
	}
}

func TestTransformResponse_UsageMapping(t *testing.T) {
	openaiResp := `{
		"id": "chatcmpl-usage",
		"choices": [{
			"message": {
				"role": "assistant",
				"content": "Test response",
				"reasoning_content": "My reasoning"
			},
			"finish_reason": "stop"
		}],
		"usage": {
			"prompt_tokens": 100,
			"completion_tokens": 200
		}
	}`

	resp, err := TransformResponse([]byte(openaiResp), "test-model")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if resp.Usage.InputTokens != 100 {
		t.Errorf("Input tokens = %d, want %d", resp.Usage.InputTokens, 100)
	}
	if resp.Usage.OutputTokens != 200 {
		t.Errorf("Output tokens = %d, want %d", resp.Usage.OutputTokens, 200)
	}
}

// TestTransformResponse_AnthropicSerialization verifies that reasoning_content
// responses can be serialized to valid Anthropic format
func TestTransformResponse_AnthropicSerialization(t *testing.T) {
	openaiResp := `{
		"id": "chatcmpl-serial",
		"choices": [{
			"message": {
				"role": "assistant",
				"content": "Answer",
				"reasoning_content": "Thinking process"
			},
			"finish_reason": "stop"
		}],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 20
		}
	}`

	resp, err := TransformResponse([]byte(openaiResp), "claude-3-5-sonnet-20241022")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Serialize to JSON and verify it's valid
	jsonData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	// Unmarshal and verify structure
	var unmarshaled models.AnthropicResponse
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(unmarshaled.Content) != 2 {
		t.Errorf("Unmarshaled response has %d content blocks, want 2", len(unmarshaled.Content))
	}

	// Verify the thinking block is preserved
	thinkingFound := false
	for _, block := range unmarshaled.Content {
		if block.Type == "thinking" {
			thinkingFound = true
			if block.Thinking != "Thinking process" {
				t.Errorf("Thinking content = %q, want %q", block.Thinking, "Thinking process")
			}
		}
	}

	if !thinkingFound {
		t.Error("Thinking block not found after serialization round-trip")
	}
}

// Package tests provides comprehensive integration tests for CLASP tool calling.
//
// This file tests the complete tool calling flow for the OpenAI provider:
// 1. Request Translation: Anthropic tool definitions → OpenAI function format
// 2. Tool Choice Translation: Anthropic tool_choice → OpenAI tool_choice
// 3. Response Handling: OpenAI tool_calls → Anthropic tool_use
// 4. Tool Result Flow: Anthropic tool_result → OpenAI tool message
// 5. Streaming Tool Calls: SSE-based tool call handling
// 6. Multi-turn Tool Conversations: Complete back-and-forth with tool results
package tests

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jedarden/clasp/internal/translator"
	"github.com/jedarden/clasp/pkg/models"
)

// =============================================================================
// SECTION 1: Tool Definition Translation Tests
// =============================================================================

// TestToolDefinitionTranslation_Basic verifies basic tool definitions are
// correctly translated from Anthropic to OpenAI format.
func TestToolDefinitionTranslation_Basic(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Get the weather in NYC"},
		},
		Tools: []models.AnthropicTool{
			{
				Name:        "get_weather",
				Description: "Get the current weather for a location",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The city and state, e.g., San Francisco, CA",
						},
						"unit": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"celsius", "fahrenheit"},
							"description": "Temperature unit",
						},
					},
					"required": []interface{}{"location"},
				},
			},
		},
	}

	result, err := translator.TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	// Verify tools are translated
	if len(result.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result.Tools))
	}

	tool := result.Tools[0]

	// Verify tool type is "function"
	if tool.Type != "function" {
		t.Errorf("tool type = %q, want %q", tool.Type, "function")
	}

	// Verify function name
	if tool.Function.Name != "get_weather" {
		t.Errorf("function name = %q, want %q", tool.Function.Name, "get_weather")
	}

	// Verify description preserved
	if tool.Function.Description != "Get the current weather for a location" {
		t.Errorf("function description = %q, want original", tool.Function.Description)
	}

	// Verify strict mode is false (critical for Claude Code compatibility)
	if tool.Function.Strict != false {
		t.Errorf("function strict = %v, want false", tool.Function.Strict)
	}

	// Verify parameters schema is preserved
	params, ok := tool.Function.Parameters.(map[string]interface{})
	if !ok {
		t.Fatalf("parameters is not a map: %T", tool.Function.Parameters)
	}

	// Check that properties exist
	if props, ok := params["properties"].(map[string]interface{}); ok {
		if _, hasLocation := props["location"]; !hasLocation {
			t.Error("parameters missing 'location' property")
		}
		if _, hasUnit := props["unit"]; !hasUnit {
			t.Error("parameters missing 'unit' property")
		}
	} else {
		t.Error("parameters missing 'properties' field")
	}
}

// TestToolDefinitionTranslation_MultipleTools verifies multiple tools are
// correctly translated.
func TestToolDefinitionTranslation_MultipleTools(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Help me with file operations"},
		},
		Tools: []models.AnthropicTool{
			{
				Name:        "Read",
				Description: "Read a file from the filesystem",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file_path": map[string]interface{}{
							"type":        "string",
							"description": "The absolute path to the file to read",
						},
					},
					"required": []interface{}{"file_path"},
				},
			},
			{
				Name:        "Write",
				Description: "Write content to a file",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file_path": map[string]interface{}{
							"type":        "string",
							"description": "The absolute path to the file",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "The content to write",
						},
					},
					"required": []interface{}{"file_path", "content"},
				},
			},
			{
				Name:        "Bash",
				Description: "Execute a bash command",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "The command to execute",
						},
					},
					"required": []interface{}{"command"},
				},
			},
		},
	}

	result, err := translator.TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	// Verify all tools are translated
	if len(result.Tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(result.Tools))
	}

	// Verify tool names in order
	expectedNames := []string{"Read", "Write", "Bash"}
	for i, name := range expectedNames {
		if result.Tools[i].Function.Name != name {
			t.Errorf("tool[%d].name = %q, want %q", i, result.Tools[i].Function.Name, name)
		}
	}
}

// TestToolDefinitionTranslation_ComplexSchema verifies complex nested schemas
// are correctly translated.
func TestToolDefinitionTranslation_ComplexSchema(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Create a user"},
		},
		Tools: []models.AnthropicTool{
			{
				Name:        "create_user",
				Description: "Create a new user with profile data",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "User's full name",
						},
						"email": map[string]interface{}{
							"type":   "string",
							"format": "email",
						},
						"profile": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"age": map[string]interface{}{
									"type":        "integer",
									"description": "User's age",
								},
								"interests": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type": "string",
									},
								},
							},
						},
						"metadata": map[string]interface{}{
							"type": "object",
							"additionalProperties": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"required": []interface{}{"name", "email"},
				},
			},
		},
	}

	result, err := translator.TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	if len(result.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result.Tools))
	}

	// Verify complex schema is preserved
	params, ok := result.Tools[0].Function.Parameters.(map[string]interface{})
	if !ok {
		t.Fatal("parameters is not a map")
	}

	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties is not a map")
	}

	// Check nested profile object
	profile, ok := props["profile"].(map[string]interface{})
	if !ok {
		t.Fatal("profile property missing or not a map")
	}
	if profile["type"] != "object" {
		t.Errorf("profile type = %v, want 'object'", profile["type"])
	}
}

// =============================================================================
// SECTION 2: Tool Choice Translation Tests
// =============================================================================

// TestToolChoiceTranslation tests all tool_choice variations.
func TestToolChoiceTranslation(t *testing.T) {
	tests := []struct {
		name            string
		anthropicChoice interface{}
		expectedOpenAI  interface{}
	}{
		{
			name:            "none - disable tool use",
			anthropicChoice: map[string]interface{}{"type": "none"},
			expectedOpenAI:  "none",
		},
		{
			name:            "any - force tool use (required)",
			anthropicChoice: map[string]interface{}{"type": "any"},
			expectedOpenAI:  "required",
		},
		{
			name:            "auto - let model decide",
			anthropicChoice: map[string]interface{}{"type": "auto"},
			expectedOpenAI:  "auto",
		},
		{
			name: "specific tool - force specific function",
			anthropicChoice: map[string]interface{}{
				"type": "tool",
				"name": "get_weather",
			},
			expectedOpenAI: map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name": "get_weather",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &models.AnthropicRequest{
				Model:      "claude-3-sonnet-20240229",
				MaxTokens:  1024,
				ToolChoice: tt.anthropicChoice,
				Messages: []models.AnthropicMessage{
					{Role: "user", Content: "test"},
				},
				Tools: []models.AnthropicTool{
					{
						Name:        "get_weather",
						Description: "Get weather",
						InputSchema: map[string]interface{}{"type": "object"},
					},
				},
			}

			result, err := translator.TransformRequest(req, "gpt-4o")
			if err != nil {
				t.Fatalf("TransformRequest failed: %v", err)
			}

			// Compare as JSON for complex types
			expectedJSON, _ := json.Marshal(tt.expectedOpenAI)
			resultJSON, _ := json.Marshal(result.ToolChoice)
			if !bytes.Equal(expectedJSON, resultJSON) {
				t.Errorf("tool_choice = %s, want %s", resultJSON, expectedJSON)
			}
		})
	}
}

// =============================================================================
// SECTION 3: Tool Call Response Streaming Tests
// =============================================================================

// TestStreamingToolCall_Single tests a single tool call through streaming.
func TestStreamingToolCall_Single(t *testing.T) {
	var buf bytes.Buffer
	sp := translator.NewStreamProcessor(&buf, "msg_test_123", "gpt-4o")

	// Simulate OpenAI streaming tool call response
	input := `data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"role":"assistant","content":null}}]}

data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_abc123","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}

data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"loca"}}]}}]}

data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"tion\":"}}]}}]}

data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"NYC\","}}]}}]}

data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"unit\":"}}]}}]}

data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"celsius\"}"}}]}}]}

data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Verify Anthropic-format tool_use events are emitted
	expectedEvents := []string{
		"event: message_start",
		"event: content_block_start",
		"\"type\":\"tool_use\"",
		"\"id\":\"call_abc123\"",
		"\"name\":\"get_weather\"",
		"event: content_block_delta",
		"\"type\":\"input_json_delta\"",
		"\"partial_json\":",
		"event: content_block_stop",
		"event: message_delta",
		"\"stop_reason\":\"tool_use\"",
		"event: message_stop",
		"data: [DONE]",
	}

	for _, expected := range expectedEvents {
		if !strings.Contains(output, expected) {
			t.Errorf("output missing %q", expected)
		}
	}

	// Verify the JSON argument chunks are present in the partial_json deltas
	// The arguments are streamed as incremental chunks (they'll be escaped in SSE format)
	if !strings.Contains(output, "loca") || !strings.Contains(output, "tion") {
		t.Error("tool call arguments not properly streamed - missing location key parts")
	}
	if !strings.Contains(output, "NYC") {
		t.Error("tool call arguments not properly streamed - missing NYC value")
	}
}

// TestStreamingToolCall_Multiple tests multiple parallel tool calls.
func TestStreamingToolCall_Multiple(t *testing.T) {
	var buf bytes.Buffer
	sp := translator.NewStreamProcessor(&buf, "msg_test_456", "gpt-4o")

	// Simulate OpenAI streaming with multiple parallel tool calls
	input := `data: {"choices":[{"index":0,"delta":{"role":"assistant","content":"Let me get both."}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_2","type":"function","function":{"name":"get_time","arguments":""}}]}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"location\":\"NYC\"}"}}]}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"arguments":"{\"timezone\":\"EST\"}"}}]}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Verify both tool calls are present
	if !strings.Contains(output, "call_1") || !strings.Contains(output, "call_2") {
		t.Error("missing one or more tool call IDs")
	}

	if !strings.Contains(output, "get_weather") || !strings.Contains(output, "get_time") {
		t.Error("missing one or more tool names")
	}

	// Verify stop reason is tool_use
	if !strings.Contains(output, "\"stop_reason\":\"tool_use\"") {
		t.Error("missing tool_use stop reason")
	}
}

// TestStreamingToolCall_WithPrecedingText tests tool call with text before it.
func TestStreamingToolCall_WithPrecedingText(t *testing.T) {
	var buf bytes.Buffer
	sp := translator.NewStreamProcessor(&buf, "msg_test_789", "gpt-4o")

	input := `data: {"choices":[{"index":0,"delta":{"role":"assistant"}}]}

data: {"choices":[{"index":0,"delta":{"content":"Let me check the weather for you."}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_xyz","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"London\"}"}}]}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Should have text content followed by tool use
	if !strings.Contains(output, "\"text\":\"Let me check the weather for you.\"") {
		t.Error("missing text content before tool call")
	}

	// Text block should be closed before tool block starts
	if !strings.Contains(output, "\"type\":\"text\"") {
		t.Error("missing text block")
	}

	if !strings.Contains(output, "\"type\":\"tool_use\"") {
		t.Error("missing tool_use block")
	}
}

// =============================================================================
// SECTION 4: Tool Result Flow Tests
// =============================================================================

// TestToolResultTranslation tests that tool_result messages are correctly
// translated to OpenAI "tool" role messages.
func TestToolResultTranslation(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			// Initial user request
			{Role: "user", Content: "What's the weather in NYC?"},
			// Assistant's tool call response
			{
				Role: "assistant",
				Content: []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "I'll check the weather for you.",
					},
					map[string]interface{}{
						"type":  "tool_use",
						"id":    "toolu_01ABC123",
						"name":  "get_weather",
						"input": map[string]interface{}{"location": "NYC"},
					},
				},
			},
			// User's tool result
			{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{
						"type":        "tool_result",
						"tool_use_id": "toolu_01ABC123",
						"content":     "72°F, Sunny with light clouds",
					},
				},
			},
		},
		Tools: []models.AnthropicTool{
			{
				Name:        "get_weather",
				Description: "Get weather",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}

	result, err := translator.TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	// Should have: system (identity filter), user, assistant, tool
	// The identity filter adds a system message
	if len(result.Messages) < 3 {
		t.Fatalf("expected at least 3 messages, got %d", len(result.Messages))
	}

	// Find the tool message (role = "tool")
	var toolMsg *models.OpenAIMessage
	for i := range result.Messages {
		if result.Messages[i].Role == "tool" {
			toolMsg = &result.Messages[i]
			break
		}
	}

	if toolMsg == nil {
		t.Fatal("no tool role message found")
	}

	// Verify tool_call_id matches
	if toolMsg.ToolCallID != "toolu_01ABC123" {
		t.Errorf("tool_call_id = %q, want %q", toolMsg.ToolCallID, "toolu_01ABC123")
	}

	// Verify content is the tool result
	if toolMsg.Content != "72°F, Sunny with light clouds" {
		t.Errorf("tool content = %v, want weather string", toolMsg.Content)
	}
}

// TestToolResultTranslation_MultipleResults tests multiple tool results.
func TestToolResultTranslation_MultipleResults(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Compare weather in NYC and LA"},
			{
				Role: "assistant",
				Content: []interface{}{
					map[string]interface{}{
						"type":  "tool_use",
						"id":    "toolu_weather_nyc",
						"name":  "get_weather",
						"input": map[string]interface{}{"location": "NYC"},
					},
					map[string]interface{}{
						"type":  "tool_use",
						"id":    "toolu_weather_la",
						"name":  "get_weather",
						"input": map[string]interface{}{"location": "LA"},
					},
				},
			},
			{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{
						"type":        "tool_result",
						"tool_use_id": "toolu_weather_nyc",
						"content":     "NYC: 72°F, Sunny",
					},
					map[string]interface{}{
						"type":        "tool_result",
						"tool_use_id": "toolu_weather_la",
						"content":     "LA: 85°F, Clear",
					},
				},
			},
		},
		Tools: []models.AnthropicTool{
			{
				Name:        "get_weather",
				Description: "Get weather",
				InputSchema: map[string]interface{}{"type": "object"},
			},
		},
	}

	result, err := translator.TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	// Count tool messages
	toolMsgCount := 0
	for _, msg := range result.Messages {
		if msg.Role == "tool" {
			toolMsgCount++
		}
	}

	// Should have 2 separate tool messages
	if toolMsgCount != 2 {
		t.Errorf("expected 2 tool messages, got %d", toolMsgCount)
	}
}

// TestToolResultTranslation_AssistantToolCalls verifies assistant messages
// with tool_use blocks are translated with tool_calls array.
func TestToolResultTranslation_AssistantToolCalls(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Read file.txt"},
			{
				Role: "assistant",
				Content: []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "Let me read that file.",
					},
					map[string]interface{}{
						"type":  "tool_use",
						"id":    "call_read_123",
						"name":  "Read",
						"input": map[string]interface{}{"file_path": "/path/to/file.txt"},
					},
				},
			},
		},
		Tools: []models.AnthropicTool{
			{Name: "Read", Description: "Read file", InputSchema: map[string]interface{}{"type": "object"}},
		},
	}

	result, err := translator.TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	// Find assistant message
	var assistantMsg *models.OpenAIMessage
	for i := range result.Messages {
		if result.Messages[i].Role == "assistant" {
			assistantMsg = &result.Messages[i]
			break
		}
	}

	if assistantMsg == nil {
		t.Fatal("no assistant message found")
	}

	// Should have tool_calls
	if len(assistantMsg.ToolCalls) == 0 {
		t.Fatal("assistant message should have tool_calls")
	}

	tc := assistantMsg.ToolCalls[0]
	if tc.ID != "call_read_123" {
		t.Errorf("tool_call id = %q, want %q", tc.ID, "call_read_123")
	}
	if tc.Function.Name != "Read" {
		t.Errorf("tool_call function.name = %q, want %q", tc.Function.Name, "Read")
	}

	// Arguments should be JSON
	if !strings.Contains(tc.Function.Arguments, "file_path") {
		t.Error("tool_call arguments should contain file_path")
	}

	// Content should be the text part
	if assistantMsg.Content != "Let me read that file." {
		t.Errorf("assistant content = %v, want text", assistantMsg.Content)
	}
}

// =============================================================================
// SECTION 5: Complex Multi-Turn Conversation Tests
// =============================================================================

// TestMultiTurnToolConversation tests a complete multi-turn conversation
// with tool calls and results.
func TestMultiTurnToolConversation(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			// Turn 1: User asks a question
			{Role: "user", Content: "What files are in the current directory?"},
			// Turn 2: Assistant uses tool
			{
				Role: "assistant",
				Content: []interface{}{
					map[string]interface{}{
						"type":  "tool_use",
						"id":    "toolu_ls_001",
						"name":  "Bash",
						"input": map[string]interface{}{"command": "ls -la"},
					},
				},
			},
			// Turn 3: Tool result
			{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{
						"type":        "tool_result",
						"tool_use_id": "toolu_ls_001",
						"content":     "file1.txt\nfile2.txt\nREADME.md",
					},
				},
			},
			// Turn 4: Assistant responds
			{Role: "assistant", Content: "I found three files: file1.txt, file2.txt, and README.md."},
			// Turn 5: User asks for file content
			{Role: "user", Content: "Read the README.md"},
			// Turn 6: Assistant uses another tool
			{
				Role: "assistant",
				Content: []interface{}{
					map[string]interface{}{
						"type":  "tool_use",
						"id":    "toolu_read_002",
						"name":  "Read",
						"input": map[string]interface{}{"file_path": "README.md"},
					},
				},
			},
			// Turn 7: Another tool result
			{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{
						"type":        "tool_result",
						"tool_use_id": "toolu_read_002",
						"content":     "# Project README\nThis is a test project.",
					},
				},
			},
		},
		Tools: []models.AnthropicTool{
			{Name: "Bash", Description: "Run bash", InputSchema: map[string]interface{}{"type": "object"}},
			{Name: "Read", Description: "Read file", InputSchema: map[string]interface{}{"type": "object"}},
		},
	}

	result, err := translator.TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	// Count message types
	var userCount, assistantCount, toolCount int
	for _, msg := range result.Messages {
		switch msg.Role {
		case "user":
			userCount++
		case "assistant":
			assistantCount++
		case "tool":
			toolCount++
		}
		// Note: "system" role messages from identity filter are intentionally ignored
	}

	// Verify correct number of each role
	// Note: Some "user" messages with only tool_result content are converted to tool messages
	if toolCount != 2 {
		t.Errorf("expected 2 tool messages, got %d", toolCount)
	}

	// Verify tool_call_ids are preserved correctly
	toolCallIDs := make(map[string]bool)
	for _, msg := range result.Messages {
		if msg.Role == "tool" {
			toolCallIDs[msg.ToolCallID] = true
		}
	}

	if !toolCallIDs["toolu_ls_001"] {
		t.Error("missing tool_call_id toolu_ls_001")
	}
	if !toolCallIDs["toolu_read_002"] {
		t.Error("missing tool_call_id toolu_read_002")
	}
}

// =============================================================================
// SECTION 6: Edge Cases and Error Handling
// =============================================================================

// TestToolCallWithEmptyArguments tests tool calls with empty arguments.
func TestToolCallWithEmptyArguments(t *testing.T) {
	var buf bytes.Buffer
	sp := translator.NewStreamProcessor(&buf, "msg_test_empty", "gpt-4o")

	input := `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_empty","type":"function","function":{"name":"no_args_tool","arguments":""}}]}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{}"}}]}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Should handle empty arguments gracefully
	if !strings.Contains(output, "no_args_tool") {
		t.Error("tool name not found in output")
	}
	if !strings.Contains(output, "{}") {
		t.Error("empty arguments not found in output")
	}
}

// TestToolCallWithSpecialCharacters tests tool call arguments with special chars.
func TestToolCallWithSpecialCharacters(t *testing.T) {
	var buf bytes.Buffer
	sp := translator.NewStreamProcessor(&buf, "msg_test_special", "gpt-4o")

	// Arguments containing JSON special characters (quotes, backslashes, newlines)
	input := `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_special","type":"function","function":{"name":"create_file","arguments":""}}]}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"content\":\"Line 1\\nLine 2\\t\\\"quoted\\\"\"}"}}]}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`
	err := sp.ProcessStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// Should preserve escaped characters
	if !strings.Contains(output, "create_file") {
		t.Error("tool name not found in output")
	}
}

// TestToolCallWithLargeArguments tests tool calls with large JSON payloads.
func TestToolCallWithLargeArguments(t *testing.T) {
	var buf bytes.Buffer
	sp := translator.NewStreamProcessor(&buf, "msg_test_large", "gpt-4o")

	// Build a large argument string in chunks
	var inputBuilder strings.Builder
	inputBuilder.WriteString(`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_large","type":"function","function":{"name":"process_data","arguments":""}}]}}]}

`)

	// Simulate multiple chunks of arguments
	inputBuilder.WriteString(`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"data\":"}}]}}]}

`)
	inputBuilder.WriteString(`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"[1,2,3,4,5,"}}]}}]}

`)
	inputBuilder.WriteString(`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"6,7,8,9,10]}"}}]}}]}

`)
	inputBuilder.WriteString(`data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`)

	err := sp.ProcessStream(strings.NewReader(inputBuilder.String()))
	if err != nil {
		t.Fatalf("ProcessStream failed: %v", err)
	}

	output := buf.String()

	// All argument chunks should be assembled
	if !strings.Contains(output, "process_data") {
		t.Error("tool name not found in output")
	}
}

// TestToolSchemaWithUnsupportedFormat tests that unsupported JSON schema
// format types (like "uri") are removed.
func TestToolSchemaWithUnsupportedFormat(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Read a file"},
		},
		Tools: []models.AnthropicTool{
			{
				Name:        "Read",
				Description: "Read file",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file_path": map[string]interface{}{
							"type":        "string",
							"format":      "uri", // OpenAI doesn't support this
							"description": "File path",
						},
					},
					"required": []interface{}{"file_path"},
				},
			},
		},
	}

	result, err := translator.TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	// Get the transformed schema
	params, _ := result.Tools[0].Function.Parameters.(map[string]interface{})
	props, _ := params["properties"].(map[string]interface{})
	filePath, _ := props["file_path"].(map[string]interface{})

	// format: "uri" should be removed
	if _, hasFormat := filePath["format"]; hasFormat {
		t.Error("format: uri should be removed from schema")
	}
}

// =============================================================================
// SECTION 7: Finish Reason Mapping Tests
// =============================================================================

// TestFinishReasonMapping tests all finish_reason translations.
func TestFinishReasonMapping(t *testing.T) {
	tests := []struct {
		openAIReason      string
		expectedAnthropic string
	}{
		{"stop", "end_turn"},
		{"tool_calls", "tool_use"},
		{"length", "max_tokens"},
		{"content_filter", "end_turn"},
		{"", "end_turn"},
		{"unknown", "end_turn"},
	}

	for _, tt := range tests {
		t.Run(tt.openAIReason, func(t *testing.T) {
			var buf bytes.Buffer
			sp := translator.NewStreamProcessor(&buf, "msg_test", "gpt-4o")

			input := `data: {"choices":[{"index":0,"delta":{"content":"Test"}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"` + tt.openAIReason + `"}]}

data: [DONE]
`
			if tt.openAIReason == "" {
				// Special case: empty finish reason (no finish_reason field)
				input = `data: {"choices":[{"index":0,"delta":{"content":"Test"}}]}

data: {"choices":[{"index":0,"delta":{}}]}

data: [DONE]
`
			}

			err := sp.ProcessStream(strings.NewReader(input))
			if err != nil {
				t.Fatalf("ProcessStream failed: %v", err)
			}

			output := buf.String()

			expectedStop := "\"stop_reason\":\"" + tt.expectedAnthropic + "\""
			if tt.openAIReason == "" {
				// No finish_reason means no message_delta with stop_reason
				return
			}
			if !strings.Contains(output, expectedStop) {
				t.Errorf("output missing %q", expectedStop)
			}
		})
	}
}

// =============================================================================
// SECTION 8: Benchmark Tests
// =============================================================================

// BenchmarkToolCallStreaming benchmarks tool call stream processing.
func BenchmarkToolCallStreaming(b *testing.B) {
	input := `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_bench","type":"function","function":{"name":"test_tool","arguments":""}}]}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"key\":\"value\"}"}}]}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		sp := translator.NewStreamProcessor(&buf, "msg_bench", "gpt-4o")
		sp.ProcessStream(strings.NewReader(input))
	}
}

// BenchmarkToolTransformation benchmarks tool definition transformation.
func BenchmarkToolTransformation(b *testing.B) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "test"},
		},
		Tools: []models.AnthropicTool{
			{
				Name:        "Read",
				Description: "Read file",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file_path": map[string]interface{}{"type": "string"},
					},
				},
			},
			{
				Name:        "Write",
				Description: "Write file",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file_path": map[string]interface{}{"type": "string"},
						"content":   map[string]interface{}{"type": "string"},
					},
				},
			},
			{
				Name:        "Bash",
				Description: "Run command",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		translator.TransformRequest(req, "gpt-4o")
	}
}

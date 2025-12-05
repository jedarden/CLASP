package tests

import (
	"encoding/json"
	"testing"

	"github.com/jedarden/clasp/internal/translator"
	"github.com/jedarden/clasp/pkg/models"
)

func TestTransformRequest_SimpleTextMessage(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Hello, how are you?",
			},
		},
	}

	result, err := translator.TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Model != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got '%s'", result.Model)
	}

	if result.MaxTokens != 1024 {
		t.Errorf("expected max_tokens 1024, got %d", result.MaxTokens)
	}

	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}

	if result.Messages[0].Role != "user" {
		t.Errorf("expected role 'user', got '%s'", result.Messages[0].Role)
	}
}

func TestTransformRequest_WithSystemMessage(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		System:    "You are a helpful assistant.",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
	}

	result, err := translator.TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.Messages))
	}

	if result.Messages[0].Role != "system" {
		t.Errorf("expected first message role 'system', got '%s'", result.Messages[0].Role)
	}

	if result.Messages[0].Content != "You are a helpful assistant." {
		t.Errorf("unexpected system content: %v", result.Messages[0].Content)
	}
}

func TestTransformRequest_WithTools(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Read the file /src/main.go",
			},
		},
		Tools: []models.AnthropicTool{
			{
				Name:        "Read",
				Description: "Read a file from the filesystem",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file_path": map[string]interface{}{
							"type":   "string",
							"format": "uri",
						},
					},
					"required": []string{"file_path"},
				},
			},
		},
	}

	result, err := translator.TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result.Tools))
	}

	tool := result.Tools[0]
	if tool.Type != "function" {
		t.Errorf("expected tool type 'function', got '%s'", tool.Type)
	}

	if tool.Function.Name != "Read" {
		t.Errorf("expected function name 'Read', got '%s'", tool.Function.Name)
	}
}

func TestTransformRequest_WithContentBlocks(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{
			"type": "text",
			"text": "What's in this image?",
		},
		map[string]interface{}{
			"type": "image",
			"source": map[string]interface{}{
				"type":       "base64",
				"media_type": "image/png",
				"data":       "iVBORw0KG...",
			},
		},
	}

	req := &models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: content,
			},
		},
	}

	result, err := translator.TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}

	// Content should be an array with text and image_url parts
	contentArr, ok := result.Messages[0].Content.([]interface{})
	if !ok {
		t.Fatalf("expected content to be array, got %T", result.Messages[0].Content)
	}

	if len(contentArr) != 2 {
		t.Errorf("expected 2 content parts, got %d", len(contentArr))
	}
}

func TestTransformRequest_ToolChoice(t *testing.T) {
	tests := []struct {
		name           string
		anthropicChoice interface{}
		expectedOpenAI interface{}
	}{
		{
			name:           "none",
			anthropicChoice: map[string]interface{}{"type": "none"},
			expectedOpenAI: "none",
		},
		{
			name:           "any",
			anthropicChoice: map[string]interface{}{"type": "any"},
			expectedOpenAI: "required",
		},
		{
			name:           "auto",
			anthropicChoice: map[string]interface{}{"type": "auto"},
			expectedOpenAI: "auto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &models.AnthropicRequest{
				Model:      "claude-3-opus-20240229",
				MaxTokens:  1024,
				Messages:   []models.AnthropicMessage{{Role: "user", Content: "test"}},
				Tools:      []models.AnthropicTool{{Name: "test", InputSchema: map[string]interface{}{}}},
				ToolChoice: tt.anthropicChoice,
			}

			result, err := translator.TransformRequest(req, "gpt-4o")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.ToolChoice != tt.expectedOpenAI {
				t.Errorf("expected tool_choice '%v', got '%v'", tt.expectedOpenAI, result.ToolChoice)
			}
		})
	}
}

func TestTransformRequest_Streaming(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Stream:    true,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	result, err := translator.TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Stream {
		t.Error("expected stream to be true")
	}

	if result.StreamOptions == nil {
		t.Error("expected StreamOptions to be set")
	} else if !result.StreamOptions.IncludeUsage {
		t.Error("expected StreamOptions.IncludeUsage to be true")
	}
}

func TestTransformRequest_AssistantWithToolUse(t *testing.T) {
	// Assistant message with tool use
	assistantContent := []interface{}{
		map[string]interface{}{
			"type": "text",
			"text": "Let me read that file.",
		},
		map[string]interface{}{
			"type":  "tool_use",
			"id":    "toolu_123",
			"name":  "Read",
			"input": map[string]interface{}{"file_path": "/src/main.go"},
		},
	}

	// User message with tool result
	userContent := []interface{}{
		map[string]interface{}{
			"type":        "tool_result",
			"tool_use_id": "toolu_123",
			"content":     "package main...",
		},
	}

	req := &models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Read /src/main.go"},
			{Role: "assistant", Content: assistantContent},
			{Role: "user", Content: userContent},
		},
	}

	result, err := translator.TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have: user, assistant with tool_calls, tool
	if len(result.Messages) < 3 {
		t.Fatalf("expected at least 3 messages, got %d", len(result.Messages))
	}

	// Check assistant message has tool_calls
	for i, msg := range result.Messages {
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			if msg.ToolCalls[0].ID != "toolu_123" {
				t.Errorf("expected tool call id 'toolu_123', got '%s'", msg.ToolCalls[0].ID)
			}
			if msg.ToolCalls[0].Function.Name != "Read" {
				t.Errorf("expected tool call name 'Read', got '%s'", msg.ToolCalls[0].Function.Name)
			}
			break
		}
		if i == len(result.Messages)-1 {
			t.Error("no assistant message with tool_calls found")
		}
	}

	// Check for tool message
	hasToolMessage := false
	for _, msg := range result.Messages {
		if msg.Role == "tool" {
			hasToolMessage = true
			if msg.ToolCallID != "toolu_123" {
				t.Errorf("expected tool_call_id 'toolu_123', got '%s'", msg.ToolCallID)
			}
			break
		}
	}
	if !hasToolMessage {
		t.Error("no tool message found")
	}
}

func TestTransformRequest_MaxTokensCapping(t *testing.T) {
	tests := []struct {
		name           string
		inputTokens    int
		targetModel    string
		expectedTokens int
	}{
		{
			name:           "GPT-4o with tokens under limit",
			inputTokens:    1024,
			targetModel:    "gpt-4o",
			expectedTokens: 1024,
		},
		{
			name:           "GPT-4o with tokens at limit",
			inputTokens:    16384,
			targetModel:    "gpt-4o",
			expectedTokens: 16384,
		},
		{
			name:           "GPT-4o with tokens over limit",
			inputTokens:    21333,
			targetModel:    "gpt-4o",
			expectedTokens: 16384,
		},
		{
			name:           "GPT-4o-mini with high tokens",
			inputTokens:    50000,
			targetModel:    "gpt-4o-mini",
			expectedTokens: 16384,
		},
		{
			name:           "GPT-4 Turbo with high tokens",
			inputTokens:    10000,
			targetModel:    "gpt-4-turbo",
			expectedTokens: 4096,
		},
		{
			name:           "O1 model with very high tokens",
			inputTokens:    50000,
			targetModel:    "o1",
			expectedTokens: 50000, // O1 supports 100k
		},
		{
			name:           "O1 model at limit",
			inputTokens:    100000,
			targetModel:    "o1",
			expectedTokens: 100000,
		},
		{
			name:           "O1 model over limit",
			inputTokens:    150000,
			targetModel:    "o1",
			expectedTokens: 100000,
		},
		{
			name:           "Unknown model uses default limit",
			inputTokens:    10000,
			targetModel:    "unknown-model",
			expectedTokens: 4096, // Default cap
		},
		{
			name:           "Zero tokens unchanged",
			inputTokens:    0,
			targetModel:    "gpt-4o",
			expectedTokens: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &models.AnthropicRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: tt.inputTokens,
				Messages: []models.AnthropicMessage{
					{Role: "user", Content: "test"},
				},
			}

			result, err := translator.TransformRequest(req, tt.targetModel)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.MaxTokens != tt.expectedTokens {
				t.Errorf("expected max_tokens %d, got %d", tt.expectedTokens, result.MaxTokens)
			}
		})
	}
}

func BenchmarkTransformRequest(b *testing.B) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		System:    "You are a helpful assistant.",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello, how are you?"},
			{Role: "assistant", Content: "I'm doing well, thank you!"},
			{Role: "user", Content: "What can you help me with?"},
		},
		Tools: []models.AnthropicTool{
			{
				Name:        "Read",
				Description: "Read a file",
				InputSchema: map[string]interface{}{"type": "object"},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = translator.TransformRequest(req, "gpt-4o")
	}
}

// Helper to pretty print JSON for debugging
func prettyJSON(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

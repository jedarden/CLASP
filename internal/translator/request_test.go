// Package translator provides protocol translation between Anthropic and OpenAI formats.
package translator

import (
	"encoding/json"
	"testing"

	"github.com/jedarden/clasp/pkg/models"
)

func TestCapMaxTokens(t *testing.T) {
	tests := []struct {
		name        string
		maxTokens   int
		targetModel string
		expected    int
	}{
		{
			name:        "GPT-4o under limit",
			maxTokens:   8000,
			targetModel: "gpt-4o",
			expected:    8000,
		},
		{
			name:        "GPT-4o at limit",
			maxTokens:   16384,
			targetModel: "gpt-4o",
			expected:    16384,
		},
		{
			name:        "GPT-4o over limit",
			maxTokens:   20000,
			targetModel: "gpt-4o",
			expected:    16384,
		},
		{
			name:        "GPT-4o-mini over limit",
			maxTokens:   32000,
			targetModel: "gpt-4o-mini",
			expected:    16384,
		},
		{
			name:        "GPT-4 Turbo over limit",
			maxTokens:   8000,
			targetModel: "gpt-4-turbo",
			expected:    4096,
		},
		{
			name:        "O1 model high limit",
			maxTokens:   100000,
			targetModel: "o1",
			expected:    100000,
		},
		{
			name:        "O1-preview over limit",
			maxTokens:   50000,
			targetModel: "o1-preview",
			expected:    32768,
		},
		{
			name:        "Unknown model uses default",
			maxTokens:   10000,
			targetModel: "unknown-model",
			expected:    4096,
		},
		{
			name:        "Zero tokens unchanged",
			maxTokens:   0,
			targetModel: "gpt-4o",
			expected:    0,
		},
		{
			name:        "Negative tokens unchanged",
			maxTokens:   -1,
			targetModel: "gpt-4o",
			expected:    -1,
		},
		{
			name:        "Prefix match for model variant",
			maxTokens:   20000,
			targetModel: "gpt-4o-2024-11-20",
			expected:    16384,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := capMaxTokens(tt.maxTokens, tt.targetModel)
			if result != tt.expected {
				t.Errorf("capMaxTokens(%d, %q) = %d, want %d", tt.maxTokens, tt.targetModel, result, tt.expected)
			}
		})
	}
}

func TestTransformRequest_BasicText(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: 1000,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello, world!"},
		},
	}

	result, err := TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	if result.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", result.Model, "gpt-4o")
	}
	if result.MaxTokens != 1000 {
		t.Errorf("MaxTokens = %d, want %d", result.MaxTokens, 1000)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(result.Messages))
	}
	if result.Messages[0].Role != "user" {
		t.Errorf("Messages[0].Role = %q, want %q", result.Messages[0].Role, "user")
	}
}

func TestTransformRequest_WithSystemMessage(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: 1000,
		System:    "You are a helpful assistant.",
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello!"},
		},
	}

	result, err := TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	if len(result.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(result.Messages))
	}
	if result.Messages[0].Role != "system" {
		t.Errorf("Messages[0].Role = %q, want %q", result.Messages[0].Role, "system")
	}
	if result.Messages[0].Content != "You are a helpful assistant." {
		t.Errorf("Messages[0].Content = %q, want %q", result.Messages[0].Content, "You are a helpful assistant.")
	}
}

func TestTransformRequest_Streaming(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: 1000,
		Stream:    true,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello!"},
		},
	}

	result, err := TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	if !result.Stream {
		t.Error("Stream should be true")
	}
	if result.StreamOptions == nil {
		t.Fatal("StreamOptions should not be nil")
	}
	if !result.StreamOptions.IncludeUsage {
		t.Error("StreamOptions.IncludeUsage should be true")
	}
}

func TestTransformRequest_WithTools(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: 1000,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "What's the weather?"},
		},
		Tools: []models.AnthropicTool{
			{
				Name:        "get_weather",
				Description: "Get the current weather",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "City name",
						},
					},
					"required": []string{"location"},
				},
			},
		},
	}

	result, err := TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	if len(result.Tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(result.Tools))
	}
	if result.Tools[0].Type != "function" {
		t.Errorf("Tools[0].Type = %q, want %q", result.Tools[0].Type, "function")
	}
	if result.Tools[0].Function.Name != "get_weather" {
		t.Errorf("Tools[0].Function.Name = %q, want %q", result.Tools[0].Function.Name, "get_weather")
	}
}

func TestTransformRequest_ToolChoice(t *testing.T) {
	tests := []struct {
		name       string
		toolChoice interface{}
		expected   interface{}
	}{
		{
			name:       "none",
			toolChoice: map[string]interface{}{"type": "none"},
			expected:   "none",
		},
		{
			name:       "any",
			toolChoice: map[string]interface{}{"type": "any"},
			expected:   "required",
		},
		{
			name:       "auto",
			toolChoice: map[string]interface{}{"type": "auto"},
			expected:   "auto",
		},
		{
			name:       "specific tool",
			toolChoice: map[string]interface{}{"type": "tool", "name": "get_weather"},
			expected: map[string]interface{}{
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
				MaxTokens:  1000,
				ToolChoice: tt.toolChoice,
				Messages: []models.AnthropicMessage{
					{Role: "user", Content: "Test"},
				},
				Tools: []models.AnthropicTool{
					{Name: "get_weather", Description: "Get weather"},
				},
			}

			result, err := TransformRequest(req, "gpt-4o")
			if err != nil {
				t.Fatalf("TransformRequest failed: %v", err)
			}

			// Compare as JSON for complex types
			expectedJSON, _ := json.Marshal(tt.expected)
			resultJSON, _ := json.Marshal(result.ToolChoice)
			if string(expectedJSON) != string(resultJSON) {
				t.Errorf("ToolChoice = %s, want %s", resultJSON, expectedJSON)
			}
		})
	}
}

func TestTransformRequest_StopSequences(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:         "claude-3-sonnet-20240229",
		MaxTokens:     1000,
		StopSequences: []string{"STOP", "END"},
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Test"},
		},
	}

	result, err := TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	if len(result.Stop) != 2 {
		t.Fatalf("len(Stop) = %d, want 2", len(result.Stop))
	}
	if result.Stop[0] != "STOP" || result.Stop[1] != "END" {
		t.Errorf("Stop = %v, want [STOP END]", result.Stop)
	}
}

func TestTransformRequest_Temperature(t *testing.T) {
	temp := 0.7
	req := &models.AnthropicRequest{
		Model:       "claude-3-sonnet-20240229",
		MaxTokens:   1000,
		Temperature: &temp,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Test"},
		},
	}

	result, err := TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	if result.Temperature == nil {
		t.Fatal("Temperature should not be nil")
	}
	if *result.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want %f", *result.Temperature, 0.7)
	}
}

func TestExtractSystemContent(t *testing.T) {
	tests := []struct {
		name     string
		system   interface{}
		expected string
	}{
		{
			name:     "string system",
			system:   "You are helpful",
			expected: "You are helpful",
		},
		{
			name: "array system",
			system: []interface{}{
				map[string]interface{}{"type": "text", "text": "Part 1"},
				map[string]interface{}{"type": "text", "text": "Part 2"},
			},
			expected: "Part 1\n\nPart 2",
		},
		{
			name:     "empty string",
			system:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractSystemContent(tt.system)
			if err != nil {
				t.Fatalf("extractSystemContent failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("extractSystemContent = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTransformTools(t *testing.T) {
	tools := []models.AnthropicTool{
		{
			Name:        "calculator",
			Description: "Perform math",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"expression": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}

	result := transformTools(tools)

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	if result[0].Type != "function" {
		t.Errorf("result[0].Type = %q, want %q", result[0].Type, "function")
	}
	if result[0].Function.Name != "calculator" {
		t.Errorf("result[0].Function.Name = %q, want %q", result[0].Function.Name, "calculator")
	}
	if result[0].Function.Description != "Perform math" {
		t.Errorf("result[0].Function.Description = %q, want %q", result[0].Function.Description, "Perform math")
	}
}

func TestCleanupSchema_RemovesUnsupportedFormat(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":   "string",
				"format": "uri",
			},
			"name": map[string]interface{}{
				"type": "string",
			},
		},
	}

	result := cleanupSchema(schema)
	resultMap := result.(map[string]interface{})
	props := resultMap["properties"].(map[string]interface{})
	urlProp := props["url"].(map[string]interface{})

	if _, exists := urlProp["format"]; exists {
		t.Error("format: uri should have been removed")
	}
}

func TestTransformToolChoice(t *testing.T) {
	tests := []struct {
		name     string
		choice   interface{}
		expected interface{}
	}{
		{
			name:     "nil",
			choice:   nil,
			expected: nil,
		},
		{
			name:     "none type",
			choice:   map[string]interface{}{"type": "none"},
			expected: "none",
		},
		{
			name:     "any type",
			choice:   map[string]interface{}{"type": "any"},
			expected: "required",
		},
		{
			name:     "auto type",
			choice:   map[string]interface{}{"type": "auto"},
			expected: "auto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformToolChoice(tt.choice)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("transformToolChoice = %v, want nil", result)
				}
				return
			}
			expectedJSON, _ := json.Marshal(tt.expected)
			resultJSON, _ := json.Marshal(result)
			if string(expectedJSON) != string(resultJSON) {
				t.Errorf("transformToolChoice = %s, want %s", resultJSON, expectedJSON)
			}
		})
	}
}

func TestGetTextContent(t *testing.T) {
	content := []models.ContentBlock{
		{Type: "text", Text: "Hello "},
		{Type: "image", Source: &models.ImageSource{MediaType: "image/png", Data: "base64"}},
		{Type: "text", Text: "world!"},
	}

	result := getTextContent(content)
	expected := "Hello world!"

	if result != expected {
		t.Errorf("getTextContent = %q, want %q", result, expected)
	}
}

func TestParseContent_String(t *testing.T) {
	content := "Simple text"
	result, err := parseContent(content)
	if err != nil {
		t.Fatalf("parseContent failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	if result[0].Type != "text" {
		t.Errorf("result[0].Type = %q, want %q", result[0].Type, "text")
	}
	if result[0].Text != "Simple text" {
		t.Errorf("result[0].Text = %q, want %q", result[0].Text, "Simple text")
	}
}

func TestParseContent_Array(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{
			"type": "text",
			"text": "Hello",
		},
	}
	result, err := parseContent(content)
	if err != nil {
		t.Fatalf("parseContent failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	if result[0].Type != "text" {
		t.Errorf("result[0].Type = %q, want %q", result[0].Type, "text")
	}
}

func TestTransformAssistantMessage_WithToolUse(t *testing.T) {
	content := []models.ContentBlock{
		{Type: "text", Text: "Let me help with that."},
		{
			Type:  "tool_use",
			ID:    "call_123",
			Name:  "get_weather",
			Input: map[string]interface{}{"location": "NYC"},
		},
	}

	result := transformAssistantMessage(content)

	if result.Role != "assistant" {
		t.Errorf("Role = %q, want %q", result.Role, "assistant")
	}
	if result.Content != "Let me help with that." {
		t.Errorf("Content = %q, want %q", result.Content, "Let me help with that.")
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(result.ToolCalls))
	}
	if result.ToolCalls[0].ID != "call_123" {
		t.Errorf("ToolCalls[0].ID = %q, want %q", result.ToolCalls[0].ID, "call_123")
	}
	if result.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("ToolCalls[0].Function.Name = %q, want %q", result.ToolCalls[0].Function.Name, "get_weather")
	}
}

func TestExtractToolResults(t *testing.T) {
	content := []models.ContentBlock{
		{Type: "text", Text: "Here is the result"},
		{Type: "tool_result", ToolUseID: "call_123", Content: "Sunny, 72°F"},
	}

	results := extractToolResults(content)

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Role != "tool" {
		t.Errorf("results[0].Role = %q, want %q", results[0].Role, "tool")
	}
	if results[0].ToolCallID != "call_123" {
		t.Errorf("results[0].ToolCallID = %q, want %q", results[0].ToolCallID, "call_123")
	}
	if results[0].Content != "Sunny, 72°F" {
		t.Errorf("results[0].Content = %q, want %q", results[0].Content, "Sunny, 72°F")
	}
}

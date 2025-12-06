package translator

import (
	"encoding/json"
	"testing"

	"github.com/jedarden/clasp/pkg/models"
)

func TestTransformRequestToResponses_BasicMessage(t *testing.T) {
	temp := 0.7
	req := &models.AnthropicRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Hello, world!",
			},
		},
		MaxTokens:   1024,
		Temperature: &temp,
		Stream:      true,
	}

	result, err := TransformRequestToResponses(req, "gpt-5.1-codex", "")
	if err != nil {
		t.Fatalf("TransformRequestToResponses failed: %v", err)
	}

	if result.Model != "gpt-5.1-codex" {
		t.Errorf("Model = %q, want %q", result.Model, "gpt-5.1-codex")
	}

	if result.MaxOutputTokens != 1024 {
		t.Errorf("MaxOutputTokens = %d, want %d", result.MaxOutputTokens, 1024)
	}

	if !result.Stream {
		t.Error("Stream should be true")
	}

	if result.Temperature == nil || *result.Temperature != 0.7 {
		t.Error("Temperature not set correctly")
	}

	if len(result.Input) != 1 {
		t.Fatalf("Input length = %d, want 1", len(result.Input))
	}

	if result.Input[0].Role != "user" {
		t.Errorf("Input[0].Role = %q, want %q", result.Input[0].Role, "user")
	}

	if result.Input[0].Content != "Hello, world!" {
		t.Errorf("Input[0].Content = %q, want %q", result.Input[0].Content, "Hello, world!")
	}
}

func TestTransformRequestToResponses_WithSystem(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:  "claude-3-5-sonnet-20241022",
		System: "You are a helpful assistant.",
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
		MaxTokens: 1024,
	}

	result, err := TransformRequestToResponses(req, "gpt-5", "")
	if err != nil {
		t.Fatalf("TransformRequestToResponses failed: %v", err)
	}

	if result.Instructions == "" {
		t.Error("Instructions should be set from system message")
	}

	// Instructions should have identity filtering applied
	if result.Instructions == "You are a helpful assistant." {
		t.Error("Instructions should have identity filtering applied")
	}
}

func TestTransformRequestToResponses_WithPreviousResponseID(t *testing.T) {
	req := &models.AnthropicRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Continue",
			},
		},
		MaxTokens: 1024,
	}

	result, err := TransformRequestToResponses(req, "gpt-5", "resp_abc123")
	if err != nil {
		t.Fatalf("TransformRequestToResponses failed: %v", err)
	}

	if result.PreviousResponseID != "resp_abc123" {
		t.Errorf("PreviousResponseID = %q, want %q", result.PreviousResponseID, "resp_abc123")
	}
}

func TestTransformRequestToResponses_WithTools(t *testing.T) {
	req := &models.AnthropicRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "What's the weather?",
			},
		},
		MaxTokens: 1024,
		Tools: []models.AnthropicTool{
			{
				Name:        "get_weather",
				Description: "Get weather for a location",
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

	result, err := TransformRequestToResponses(req, "gpt-5", "")
	if err != nil {
		t.Fatalf("TransformRequestToResponses failed: %v", err)
	}

	if len(result.Tools) != 1 {
		t.Fatalf("Tools length = %d, want 1", len(result.Tools))
	}

	if result.Tools[0].Type != "function" {
		t.Errorf("Tools[0].Type = %q, want %q", result.Tools[0].Type, "function")
	}

	if result.Tools[0].Function == nil {
		t.Fatal("Tools[0].Function is nil")
	}

	if result.Tools[0].Function.Name != "get_weather" {
		t.Errorf("Tools[0].Function.Name = %q, want %q", result.Tools[0].Function.Name, "get_weather")
	}
}

func TestTransformRequestToResponses_WithToolResult(t *testing.T) {
	req := &models.AnthropicRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []models.AnthropicMessage{
			{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{
						"type":        "tool_result",
						"tool_use_id": "toolu_abc123",
						"content":     "The weather is sunny",
					},
				},
			},
		},
		MaxTokens: 1024,
	}

	result, err := TransformRequestToResponses(req, "gpt-5", "")
	if err != nil {
		t.Fatalf("TransformRequestToResponses failed: %v", err)
	}

	// Should have the user message and the tool result
	if len(result.Input) < 1 {
		t.Fatalf("Input length = %d, want at least 1", len(result.Input))
	}
}

func TestTransformRequestToResponses_WithThinking(t *testing.T) {
	req := &models.AnthropicRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Solve this complex problem",
			},
		},
		MaxTokens: 1024,
		Thinking: &models.ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: 20000,
		},
	}

	result, err := TransformRequestToResponses(req, "gpt-5", "")
	if err != nil {
		t.Fatalf("TransformRequestToResponses failed: %v", err)
	}

	if result.ReasoningEffort != "high" {
		t.Errorf("ReasoningEffort = %q, want %q", result.ReasoningEffort, "high")
	}
}

func TestTransformRequestToResponses_AssistantMessage(t *testing.T) {
	req := &models.AnthropicRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
			{
				Role:    "assistant",
				Content: "Hi there!",
			},
			{
				Role:    "user",
				Content: "How are you?",
			},
		},
		MaxTokens: 1024,
	}

	result, err := TransformRequestToResponses(req, "gpt-5", "")
	if err != nil {
		t.Fatalf("TransformRequestToResponses failed: %v", err)
	}

	if len(result.Input) != 3 {
		t.Fatalf("Input length = %d, want 3", len(result.Input))
	}

	// Check roles
	if result.Input[0].Role != "user" {
		t.Errorf("Input[0].Role = %q, want %q", result.Input[0].Role, "user")
	}
	if result.Input[1].Role != "assistant" {
		t.Errorf("Input[1].Role = %q, want %q", result.Input[1].Role, "assistant")
	}
	if result.Input[2].Role != "user" {
		t.Errorf("Input[2].Role = %q, want %q", result.Input[2].Role, "user")
	}
}

func TestTransformRequestToResponses_JSONMarshal(t *testing.T) {
	temp := 0.7
	req := &models.AnthropicRequest{
		Model:  "claude-3-5-sonnet-20241022",
		System: "Be helpful",
		Messages: []models.AnthropicMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
		MaxTokens:   1024,
		Temperature: &temp,
		Stream:      true,
	}

	result, err := TransformRequestToResponses(req, "gpt-5.1-codex", "resp_prev123")
	if err != nil {
		t.Fatalf("TransformRequestToResponses failed: %v", err)
	}

	// Verify it marshals to valid JSON
	jsonData, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result to JSON: %v", err)
	}

	// Verify it contains expected fields
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if parsed["model"] != "gpt-5.1-codex" {
		t.Errorf("JSON model = %v, want gpt-5.1-codex", parsed["model"])
	}

	if parsed["previous_response_id"] != "resp_prev123" {
		t.Errorf("JSON previous_response_id = %v, want resp_prev123", parsed["previous_response_id"])
	}

	if _, ok := parsed["input"]; !ok {
		t.Error("JSON missing input field")
	}
}

func TestMapBudgetToReasoningEffortResponses(t *testing.T) {
	tests := []struct {
		budget   int
		expected string
	}{
		{1000, "low"},
		{3999, "low"},
		{4000, "medium"},
		{15999, "medium"},
		{16000, "high"},
		{50000, "high"},
	}

	for _, tt := range tests {
		result := mapBudgetToReasoningEffortResponses(tt.budget)
		if result != tt.expected {
			t.Errorf("mapBudgetToReasoningEffortResponses(%d) = %q, want %q", tt.budget, result, tt.expected)
		}
	}
}

func TestTransformRequestToResponses_MaxOutputTokensMinimum(t *testing.T) {
	tests := []struct {
		name          string
		maxTokens     int
		expectedMin   int
	}{
		{"below minimum", 1, 16},
		{"at minimum", 16, 16},
		{"above minimum", 100, 100},
		{"zero value", 0, 16},
		{"very low", 5, 16},
		{"normal value", 1024, 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &models.AnthropicRequest{
				Model: "claude-3-5-sonnet-20241022",
				Messages: []models.AnthropicMessage{
					{Role: "user", Content: "Hello"},
				},
				MaxTokens: tt.maxTokens,
			}

			result, err := TransformRequestToResponses(req, "gpt-5", "")
			if err != nil {
				t.Fatalf("TransformRequestToResponses failed: %v", err)
			}

			if result.MaxOutputTokens != tt.expectedMin {
				t.Errorf("MaxOutputTokens = %d, want %d", result.MaxOutputTokens, tt.expectedMin)
			}
		})
	}
}

func TestTransformRequestToResponses_ToolNameTopLevel(t *testing.T) {
	req := &models.AnthropicRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "What's the weather?"},
		},
		MaxTokens: 1024,
		Tools: []models.AnthropicTool{
			{
				Name:        "get_weather",
				Description: "Get weather for a location",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		},
	}

	result, err := TransformRequestToResponses(req, "gpt-5", "")
	if err != nil {
		t.Fatalf("TransformRequestToResponses failed: %v", err)
	}

	if len(result.Tools) != 1 {
		t.Fatalf("Tools length = %d, want 1", len(result.Tools))
	}

	// Check that Name is at top level (required for Responses API)
	if result.Tools[0].Name != "get_weather" {
		t.Errorf("Tools[0].Name = %q, want %q (top-level name for Responses API)", result.Tools[0].Name, "get_weather")
	}

	// Check that Description is at top level
	if result.Tools[0].Description != "Get weather for a location" {
		t.Errorf("Tools[0].Description = %q, want %q", result.Tools[0].Description, "Get weather for a location")
	}

	// Check that Parameters is at top level
	if result.Tools[0].Parameters == nil {
		t.Error("Tools[0].Parameters should not be nil (top-level parameters for Responses API)")
	}

	// Also verify backwards compatibility with nested Function
	if result.Tools[0].Function == nil {
		t.Error("Tools[0].Function should not be nil (backwards compatibility)")
	}
	if result.Tools[0].Function.Name != "get_weather" {
		t.Errorf("Tools[0].Function.Name = %q, want %q", result.Tools[0].Function.Name, "get_weather")
	}

	// Verify JSON output has name at top level
	jsonData, err := json.Marshal(result.Tools[0])
	if err != nil {
		t.Fatalf("Failed to marshal tool: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal tool JSON: %v", err)
	}

	// Responses API requires "name" at top level
	if parsed["name"] != "get_weather" {
		t.Errorf("JSON tool.name = %v, want 'get_weather' at top level", parsed["name"])
	}
}

func TestTransformRequestToResponses_ContentTypes(t *testing.T) {
	// Test that content parts use correct types for Responses API
	req := &models.AnthropicRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []models.AnthropicMessage{
			{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "What is in this image?",
					},
					map[string]interface{}{
						"type": "image",
						"source": map[string]interface{}{
							"type":       "base64",
							"media_type": "image/png",
							"data":       "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
						},
					},
				},
			},
		},
		MaxTokens: 1024,
	}

	result, err := TransformRequestToResponses(req, "gpt-5", "")
	if err != nil {
		t.Fatalf("TransformRequestToResponses failed: %v", err)
	}

	if len(result.Input) != 1 {
		t.Fatalf("Input length = %d, want 1", len(result.Input))
	}

	// The input should be a message with array content
	input := result.Input[0]
	if input.Type != "message" {
		t.Errorf("Input[0].Type = %q, want %q", input.Type, "message")
	}

	// Verify the content is an array with proper types
	contentArray, ok := input.Content.([]interface{})
	if !ok {
		t.Fatalf("Input[0].Content should be an array, got %T", input.Content)
	}

	if len(contentArray) != 2 {
		t.Fatalf("Content array length = %d, want 2", len(contentArray))
	}

	// Check first part is input_text
	part0 := contentArray[0].(models.ResponsesContentPart)
	if part0.Type != "input_text" {
		t.Errorf("Content[0].Type = %q, want %q (Responses API requires input_text for user text)", part0.Type, "input_text")
	}

	// Check second part is input_image
	part1 := contentArray[1].(models.ResponsesContentPart)
	if part1.Type != "input_image" {
		t.Errorf("Content[1].Type = %q, want %q (Responses API requires input_image for images)", part1.Type, "input_image")
	}
}

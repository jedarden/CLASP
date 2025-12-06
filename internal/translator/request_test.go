// Package translator provides protocol translation between Anthropic and OpenAI formats.
package translator

import (
	"encoding/json"
	"strings"
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
	// Identity filtering adds a prefix to all system messages
	content, ok := result.Messages[0].Content.(string)
	if !ok {
		t.Fatalf("Messages[0].Content is not a string")
	}
	// Should contain the identity filter prefix and the original content
	if !strings.Contains(content, "You are NOT Claude") {
		t.Errorf("System message should contain identity filter prefix")
	}
	if !strings.Contains(content, "You are a helpful assistant.") {
		t.Errorf("System message should contain original content")
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

// Thinking parameter mapping tests

func TestMapBudgetToReasoningEffort(t *testing.T) {
	tests := []struct {
		budgetTokens int
		expected     string
	}{
		{1000, "minimal"},
		{3999, "minimal"},
		{4000, "low"},
		{15999, "low"},
		{16000, "medium"},
		{31999, "medium"},
		{32000, "high"},
		{100000, "high"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := mapBudgetToReasoningEffort(tt.budgetTokens)
			if result != tt.expected {
				t.Errorf("mapBudgetToReasoningEffort(%d) = %q, want %q", tt.budgetTokens, result, tt.expected)
			}
		})
	}
}

func TestIsO1OrO3Model(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"o1", true},
		{"o1-preview", true},
		{"o1-mini", true},
		{"o3", true},
		{"o3-mini", true},
		{"openai/o1", true},
		{"openai/o3-mini", true},
		{"gpt-4o", false},
		{"gpt-4", false},
		{"claude-3", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := isO1OrO3Model(tt.model)
			if result != tt.expected {
				t.Errorf("isO1OrO3Model(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestIsGrokModel(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"x-ai/grok-3-beta", true},
		{"grok-3-mini", true},
		{"x-ai/grok-2", true},
		{"gpt-4o", false},
		{"claude-3", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := isGrokModel(tt.model)
			if result != tt.expected {
				t.Errorf("isGrokModel(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestIsGemini3Model(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"gemini-3-pro", true},
		{"google/gemini-3-ultra", true},
		{"gemini-2.5-pro", false},
		{"gpt-4o", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := isGemini3Model(tt.model)
			if result != tt.expected {
				t.Errorf("isGemini3Model(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestIsGemini25Model(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"gemini-2.5-pro", true},
		{"google/gemini-2.5-flash", true},
		{"gemini-2-5-pro", true},
		{"gemini-3-pro", false},
		{"gpt-4o", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := isGemini25Model(tt.model)
			if result != tt.expected {
				t.Errorf("isGemini25Model(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestIsQwenModel(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"qwen-2.5-72b", true},
		{"qwen/qwen-2.5-coder", true},
		{"gpt-4o", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := isQwenModel(tt.model)
			if result != tt.expected {
				t.Errorf("isQwenModel(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestIsMiniMaxModel(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"minimax-pro", true},
		{"minimax/minimax-01", true},
		{"gpt-4o", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := isMiniMaxModel(tt.model)
			if result != tt.expected {
				t.Errorf("isMiniMaxModel(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestIsDeepSeekModel(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"deepseek-r1", true},
		{"deepseek/deepseek-v3", true},
		{"deepseek-coder", true},
		{"gpt-4o", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := isDeepSeekModel(tt.model)
			if result != tt.expected {
				t.Errorf("isDeepSeekModel(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestApplyThinkingParameters_O1Model(t *testing.T) {
	req := &models.AnthropicRequest{
		Thinking: &models.ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: 20000,
		},
	}
	openAIReq := &models.OpenAIRequest{
		MaxTokens: 4096,
	}

	applyThinkingParameters(req, openAIReq, "o1")

	if openAIReq.ReasoningEffort != "medium" {
		t.Errorf("ReasoningEffort = %q, want %q", openAIReq.ReasoningEffort, "medium")
	}
	if openAIReq.MaxTokens != 0 {
		t.Errorf("MaxTokens should be cleared for O1 models, got %d", openAIReq.MaxTokens)
	}
	if openAIReq.MaxCompletionTokens != 4096 {
		t.Errorf("MaxCompletionTokens = %d, want %d", openAIReq.MaxCompletionTokens, 4096)
	}
}

func TestApplyThinkingParameters_O3Model(t *testing.T) {
	req := &models.AnthropicRequest{
		Thinking: &models.ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: 50000,
		},
	}
	openAIReq := &models.OpenAIRequest{
		MaxTokens: 8000,
	}

	applyThinkingParameters(req, openAIReq, "o3-mini")

	if openAIReq.ReasoningEffort != "high" {
		t.Errorf("ReasoningEffort = %q, want %q", openAIReq.ReasoningEffort, "high")
	}
	if openAIReq.MaxCompletionTokens != 8000 {
		t.Errorf("MaxCompletionTokens = %d, want %d", openAIReq.MaxCompletionTokens, 8000)
	}
}

func TestApplyThinkingParameters_Grok(t *testing.T) {
	req := &models.AnthropicRequest{
		Thinking: &models.ThinkingConfig{
			BudgetTokens: 25000,
		},
	}
	openAIReq := &models.OpenAIRequest{}

	applyThinkingParameters(req, openAIReq, "x-ai/grok-3-mini")

	if openAIReq.ReasoningEffort != "high" {
		t.Errorf("ReasoningEffort = %q, want %q", openAIReq.ReasoningEffort, "high")
	}
}

func TestApplyThinkingParameters_Gemini25(t *testing.T) {
	req := &models.AnthropicRequest{
		Thinking: &models.ThinkingConfig{
			BudgetTokens: 30000, // Over 24k limit
		},
	}
	openAIReq := &models.OpenAIRequest{}

	applyThinkingParameters(req, openAIReq, "google/gemini-2.5-pro")

	if openAIReq.ThinkingConfig == nil {
		t.Fatal("ThinkingConfig should be set")
	}
	if openAIReq.ThinkingConfig.ThinkingBudget != 24576 {
		t.Errorf("ThinkingBudget = %d, want %d (capped)", openAIReq.ThinkingConfig.ThinkingBudget, 24576)
	}
}

func TestApplyThinkingParameters_Gemini3(t *testing.T) {
	tests := []struct {
		budget   int
		expected string
	}{
		{10000, "low"},
		{20000, "high"},
	}

	for _, tt := range tests {
		req := &models.AnthropicRequest{
			Thinking: &models.ThinkingConfig{
				BudgetTokens: tt.budget,
			},
		}
		openAIReq := &models.OpenAIRequest{}

		applyThinkingParameters(req, openAIReq, "gemini-3-pro")

		if openAIReq.ThinkingLevel != tt.expected {
			t.Errorf("ThinkingLevel with budget %d = %q, want %q", tt.budget, openAIReq.ThinkingLevel, tt.expected)
		}
	}
}

func TestApplyThinkingParameters_Qwen(t *testing.T) {
	req := &models.AnthropicRequest{
		Thinking: &models.ThinkingConfig{
			BudgetTokens: 15000,
		},
	}
	openAIReq := &models.OpenAIRequest{}

	applyThinkingParameters(req, openAIReq, "qwen-2.5-72b")

	if openAIReq.EnableThinking == nil || !*openAIReq.EnableThinking {
		t.Error("EnableThinking should be true")
	}
	if openAIReq.ThinkingBudget != 15000 {
		t.Errorf("ThinkingBudget = %d, want %d", openAIReq.ThinkingBudget, 15000)
	}
}

func TestApplyThinkingParameters_MiniMax(t *testing.T) {
	req := &models.AnthropicRequest{
		Thinking: &models.ThinkingConfig{
			BudgetTokens: 10000,
		},
	}
	openAIReq := &models.OpenAIRequest{}

	applyThinkingParameters(req, openAIReq, "minimax-pro")

	if openAIReq.ReasoningSplit == nil || !*openAIReq.ReasoningSplit {
		t.Error("ReasoningSplit should be true")
	}
}

func TestApplyThinkingParameters_DeepSeek(t *testing.T) {
	req := &models.AnthropicRequest{
		Thinking: &models.ThinkingConfig{
			BudgetTokens: 10000,
		},
	}
	openAIReq := &models.OpenAIRequest{}

	applyThinkingParameters(req, openAIReq, "deepseek-r1")

	// DeepSeek should not set any thinking parameters
	if openAIReq.ReasoningEffort != "" {
		t.Errorf("ReasoningEffort should be empty for DeepSeek, got %q", openAIReq.ReasoningEffort)
	}
	if openAIReq.ThinkingConfig != nil {
		t.Error("ThinkingConfig should be nil for DeepSeek")
	}
}

func TestApplyThinkingParameters_NoThinking(t *testing.T) {
	req := &models.AnthropicRequest{
		Thinking: nil,
	}
	openAIReq := &models.OpenAIRequest{
		MaxTokens: 4096,
	}

	applyThinkingParameters(req, openAIReq, "o1")

	// Should not modify anything when thinking is nil
	if openAIReq.ReasoningEffort != "" {
		t.Errorf("ReasoningEffort should be empty, got %q", openAIReq.ReasoningEffort)
	}
	if openAIReq.MaxTokens != 4096 {
		t.Errorf("MaxTokens should be unchanged, got %d", openAIReq.MaxTokens)
	}
}

func TestApplyThinkingParameters_ZeroBudget(t *testing.T) {
	req := &models.AnthropicRequest{
		Thinking: &models.ThinkingConfig{
			BudgetTokens: 0,
		},
	}
	openAIReq := &models.OpenAIRequest{}

	applyThinkingParameters(req, openAIReq, "o1")

	// Should not apply parameters when budget is 0
	if openAIReq.ReasoningEffort != "" {
		t.Errorf("ReasoningEffort should be empty for zero budget, got %q", openAIReq.ReasoningEffort)
	}
}

func TestTransformRequest_WithThinking(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 4096,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Solve this math problem step by step"},
		},
		Thinking: &models.ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: 16000,
		},
	}

	result, err := TransformRequest(req, "o1-preview")
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	if result.ReasoningEffort != "medium" {
		t.Errorf("ReasoningEffort = %q, want %q", result.ReasoningEffort, "medium")
	}
	if result.MaxTokens != 0 {
		t.Errorf("MaxTokens should be 0 for O1 models, got %d", result.MaxTokens)
	}
	if result.MaxCompletionTokens != 4096 {
		t.Errorf("MaxCompletionTokens = %d, want %d", result.MaxCompletionTokens, 4096)
	}
}

// Identity filtering tests

func TestFilterIdentity_ClaudeCodeIdentity(t *testing.T) {
	input := "You are Claude Code, Anthropic's official CLI tool for developers."
	result := filterIdentity(input)

	if strings.Contains(result, "You are Claude Code, Anthropic's official CLI") {
		t.Error("Should replace Claude Code identity")
	}
	if !strings.Contains(result, "You are NOT Claude") {
		t.Error("Should add 'You are NOT Claude' prefix")
	}
	if !strings.Contains(result, "This is Claude Code, an AI-powered CLI tool") {
		t.Error("Should contain neutral replacement")
	}
}

func TestFilterIdentity_ModelNameReference(t *testing.T) {
	input := "You are powered by the model named Sonnet 4.5."
	result := filterIdentity(input)

	if strings.Contains(result, "Sonnet 4.5") {
		t.Error("Should replace specific model name reference")
	}
	if !strings.Contains(result, "You are powered by an AI model.") {
		t.Error("Should contain neutral model reference")
	}
}

func TestFilterIdentity_ClaudeBackgroundInfo(t *testing.T) {
	input := "Hello <claude_background_info>secret info here</claude_background_info> world"
	result := filterIdentity(input)

	if strings.Contains(result, "claude_background_info") {
		t.Error("Should remove claude_background_info blocks")
	}
	if strings.Contains(result, "secret info here") {
		t.Error("Should remove content inside claude_background_info")
	}
	if !strings.Contains(result, "Hello") || !strings.Contains(result, "world") {
		t.Error("Should preserve content outside claude_background_info")
	}
}

func TestFilterIdentity_MultipleNewlines(t *testing.T) {
	input := "Line 1\n\n\n\n\nLine 2"
	result := filterIdentity(input)

	if strings.Contains(result, "\n\n\n") {
		t.Error("Should collapse multiple newlines to double newline")
	}
}

func TestFilterIdentity_Prefix(t *testing.T) {
	input := "You are a helpful assistant."
	result := filterIdentity(input)

	if !strings.HasPrefix(result, "Note: You are NOT Claude.") {
		t.Error("Should have identity clarification prefix")
	}
}

func TestTransformMessages_GrokModel_AddsJSONInstruction(t *testing.T) {
	req := &models.AnthropicRequest{
		System: "You are a helpful assistant.",
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	messages, err := transformMessages(req, "x-ai/grok-3-beta")
	if err != nil {
		t.Fatalf("transformMessages failed: %v", err)
	}

	if len(messages) < 1 {
		t.Fatal("Should have at least one message")
	}

	systemContent, ok := messages[0].Content.(string)
	if !ok {
		t.Fatal("System content should be a string")
	}

	if !strings.Contains(systemContent, "NEVER use XML format like <xai:function_call>") {
		t.Error("System message for Grok should contain JSON instruction")
	}
}

func TestTransformMessages_GrokModel_NoSystemMessage_AddsJSONInstruction(t *testing.T) {
	req := &models.AnthropicRequest{
		System: nil,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	messages, err := transformMessages(req, "grok-3-mini")
	if err != nil {
		t.Fatalf("transformMessages failed: %v", err)
	}

	// Should add a system message with JSON instruction
	if len(messages) < 2 {
		t.Fatal("Should have at least 2 messages (system + user)")
	}

	systemContent, ok := messages[0].Content.(string)
	if !ok {
		t.Fatal("First message content should be a string")
	}

	if !strings.Contains(systemContent, "NEVER use XML format like <xai:function_call>") {
		t.Error("Grok should have JSON instruction even without system message")
	}
}

func TestTransformMessages_NonGrokModel_NoJSONInstruction(t *testing.T) {
	req := &models.AnthropicRequest{
		System: "You are a helpful assistant.",
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	messages, err := transformMessages(req, "gpt-4o")
	if err != nil {
		t.Fatalf("transformMessages failed: %v", err)
	}

	if len(messages) < 1 {
		t.Fatal("Should have at least one message")
	}

	systemContent, ok := messages[0].Content.(string)
	if !ok {
		t.Fatal("System content should be a string")
	}

	if strings.Contains(systemContent, "NEVER use XML format like <xai:function_call>") {
		t.Error("Non-Grok model should NOT have XML instruction")
	}
}

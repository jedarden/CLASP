package translator

import (
	"encoding/json"
	"strings"
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

	// Responses API uses nested reasoning.effort, not top-level reasoning_effort
	if result.Reasoning == nil {
		t.Fatal("Reasoning should not be nil when thinking is enabled")
	}
	if result.Reasoning.Effort != "high" {
		t.Errorf("Reasoning.Effort = %q, want %q", result.Reasoning.Effort, "high")
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

func TestTransformRequestToResponses_ReasoningJSONStructure(t *testing.T) {
	// Test that reasoning is correctly nested in JSON output
	req := &models.AnthropicRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Complex reasoning task"},
		},
		MaxTokens: 1024,
		Thinking: &models.ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: 10000, // Should map to "medium"
		},
	}

	result, err := TransformRequestToResponses(req, "gpt-5.1-codex", "")
	if err != nil {
		t.Fatalf("TransformRequestToResponses failed: %v", err)
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result to JSON: %v", err)
	}

	// Parse and verify structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify "reasoning" is a nested object, not a top-level "reasoning_effort"
	if _, hasOldField := parsed["reasoning_effort"]; hasOldField {
		t.Error("Should NOT have top-level 'reasoning_effort' field - Responses API uses nested 'reasoning.effort'")
	}

	reasoning, hasReasoning := parsed["reasoning"].(map[string]interface{})
	if !hasReasoning {
		t.Fatal("JSON should have 'reasoning' object")
	}

	effort, hasEffort := reasoning["effort"].(string)
	if !hasEffort {
		t.Fatal("JSON 'reasoning' object should have 'effort' field")
	}

	if effort != "medium" {
		t.Errorf("reasoning.effort = %q, want %q", effort, "medium")
	}

	// Log the actual JSON for debugging
	t.Logf("Generated JSON: %s", string(jsonData))
}

func TestTranslateToolCallID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Chat Completions call_ prefix",
			input:    "call_abc123xyz",
			expected: "fc_abc123xyz",
		},
		{
			name:     "Anthropic toolu_ prefix",
			input:    "toolu_01ABCDEF",
			expected: "fc_01ABCDEF",
		},
		{
			name:     "Already fc_ prefix - no change",
			input:    "fc_existingid",
			expected: "fc_existingid",
		},
		{
			name:     "Other format - adds fc_ prefix",
			input:    "custom123",
			expected: "fc_custom123",
		},
		{
			name:     "Complex call_ ID",
			input:    "call_9dKc3kP5QeGf8AvBnCmD",
			expected: "fc_9dKc3kP5QeGf8AvBnCmD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translateToolCallID(tt.input)
			if result != tt.expected {
				t.Errorf("translateToolCallID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTranslateResponsesIDToAnthropic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Responses API fc_ prefix",
			input:    "fc_abc123xyz",
			expected: "call_abc123xyz",
		},
		{
			name:     "Already call_ prefix - no change",
			input:    "call_existing",
			expected: "call_existing",
		},
		{
			name:     "Already toolu_ prefix - no change",
			input:    "toolu_01ABC",
			expected: "toolu_01ABC",
		},
		{
			name:     "Other format - adds call_ prefix",
			input:    "custom123",
			expected: "call_custom123",
		},
		{
			name:     "Complex fc_ ID",
			input:    "fc_9dKc3kP5QeGf8AvBnCmD",
			expected: "call_9dKc3kP5QeGf8AvBnCmD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TranslateResponsesIDToAnthropic(tt.input)
			if result != tt.expected {
				t.Errorf("TranslateResponsesIDToAnthropic(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToolCallIDRoundTrip(t *testing.T) {
	// Test that converting call_ -> fc_ -> call_ preserves the original ID suffix
	tests := []struct {
		name    string
		input   string
	}{
		{"Standard call ID", "call_abc123"},
		{"Numeric call ID", "call_12345"},
		{"Long call ID", "call_9dKc3kP5QeGf8AvBnCmD"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Anthropic -> Responses API
			responsesID := translateToolCallID(tt.input)
			if !strings.HasPrefix(responsesID, "fc_") {
				t.Errorf("translateToolCallID(%q) = %q, expected fc_ prefix", tt.input, responsesID)
			}

			// Responses API -> Anthropic
			anthropicID := TranslateResponsesIDToAnthropic(responsesID)
			if !strings.HasPrefix(anthropicID, "call_") {
				t.Errorf("TranslateResponsesIDToAnthropic(%q) = %q, expected call_ prefix", responsesID, anthropicID)
			}

			// Verify the suffix is preserved
			originalSuffix := strings.TrimPrefix(tt.input, "call_")
			finalSuffix := strings.TrimPrefix(anthropicID, "call_")
			if originalSuffix != finalSuffix {
				t.Errorf("Round-trip suffix mismatch: original %q, final %q", originalSuffix, finalSuffix)
			}
		})
	}
}

func TestTransformRequestToResponses_ToolResultIDTranslation(t *testing.T) {
	// Test that tool results have their IDs translated for Responses API
	req := &models.AnthropicRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []models.AnthropicMessage{
			{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{
						"type":        "tool_result",
						"tool_use_id": "call_abc123",  // Chat Completions format
						"content":     "Result from tool",
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

	// Find the function_call_output item
	var foundFCOutput bool
	for _, input := range result.Input {
		if input.Type == "function_call_output" {
			foundFCOutput = true
			// The ID should be translated to fc_ format
			if !strings.HasPrefix(input.CallID, "fc_") {
				t.Errorf("function_call_output CallID = %q, expected fc_ prefix for Responses API", input.CallID)
			}
			if input.CallID != "fc_abc123" {
				t.Errorf("function_call_output CallID = %q, expected %q", input.CallID, "fc_abc123")
			}
		}
	}

	if !foundFCOutput {
		t.Error("Expected to find function_call_output item in result")
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

func TestIdentifyTrulyRequired(t *testing.T) {
	tests := []struct {
		name     string
		props    map[string]interface{}
		schema   map[string]interface{}
		expected []string
	}{
		{
			name: "All truly required",
			props: map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The name",
				},
				"age": map[string]interface{}{
					"type":        "integer",
					"description": "The age",
				},
			},
			schema: map[string]interface{}{
				"required": []interface{}{"name", "age"},
			},
			expected: []string{"name", "age"},
		},
		{
			name: "Has default - should be excluded",
			props: map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The name",
				},
				"count": map[string]interface{}{
					"type":        "integer",
					"description": "The count",
					"default":     10,
				},
			},
			schema: map[string]interface{}{
				"required": []interface{}{"name", "count"},
			},
			expected: []string{"name"},
		},
		{
			name: "Described as optional - should be excluded",
			props: map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The name",
				},
				"model": map[string]interface{}{
					"type":        "string",
					"description": "Optional model to use for this agent",
				},
			},
			schema: map[string]interface{}{
				"required": []interface{}{"name", "model"},
			},
			expected: []string{"name"},
		},
		{
			name: "Nullable - should be excluded",
			props: map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The name",
				},
				"resume": map[string]interface{}{
					"type":        "string",
					"description": "Agent ID to resume",
					"nullable":    true,
				},
			},
			schema: map[string]interface{}{
				"required": []interface{}{"name", "resume"},
			},
			expected: []string{"name"},
		},
		{
			name: "Defaults to - should be excluded",
			props: map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The name",
				},
				"model": map[string]interface{}{
					"type":        "string",
					"description": "Model ID. Defaults to haiku for quick tasks.",
				},
			},
			schema: map[string]interface{}{
				"required": []interface{}{"name", "model"},
			},
			expected: []string{"name"},
		},
		{
			name: "If not specified - should be excluded",
			props: map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The name",
				},
				"count": map[string]interface{}{
					"type":        "integer",
					"description": "Number of items. If not specified, uses 10.",
				},
			},
			schema: map[string]interface{}{
				"required": []interface{}{"name", "count"},
			},
			expected: []string{"name"},
		},
		{
			name: "Not in original required - should be excluded",
			props: map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The name",
				},
				"extra": map[string]interface{}{
					"type":        "string",
					"description": "Extra info",
				},
			},
			schema: map[string]interface{}{
				"required": []interface{}{"name"},
			},
			expected: []string{"name"},
		},
		{
			name:   "Empty properties",
			props:  map[string]interface{}{},
			schema: map[string]interface{}{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := identifyTrulyRequired(tt.props, tt.schema)

			// Compare slices (order may vary)
			if len(result) != len(tt.expected) {
				t.Errorf("identifyTrulyRequired returned %v, want %v", result, tt.expected)
				return
			}

			// Check all expected values are present
			resultMap := make(map[string]bool)
			for _, r := range result {
				resultMap[r] = true
			}
			for _, e := range tt.expected {
				if !resultMap[e] {
					t.Errorf("Expected %q in result, got %v", e, result)
				}
			}
		})
	}
}

func TestTransformToolsToResponses_StrictModeFalse(t *testing.T) {
	// Test that tools are created with strict=false to avoid validation issues
	// when Anthropic marks all parameters as required but Claude only provides truly required ones
	tools := []models.AnthropicTool{
		{
			Name:        "Task",
			Description: "Launch a task agent",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Task description",
					},
					"model": map[string]interface{}{
						"type":        "string",
						"description": "Optional model to use",
					},
					"resume": map[string]interface{}{
						"type":        "string",
						"description": "Optional agent ID to resume",
					},
					"subagent_type": map[string]interface{}{
						"type":        "string",
						"description": "Type of specialized agent",
					},
				},
				// Anthropic marks ALL as required, but model and resume are optional
				"required": []interface{}{"description", "model", "resume", "subagent_type"},
			},
		},
	}

	result := transformToolsToResponses(tools)

	if len(result) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(result))
	}

	tool := result[0]

	// Verify strict mode is false
	if tool.Function == nil {
		t.Fatal("Tool.Function should not be nil")
	}
	if tool.Function.Strict {
		t.Error("Tool.Function.Strict should be false to allow lenient validation")
	}

	// Verify the required array has been cleaned up
	params, ok := tool.Parameters.(map[string]interface{})
	if !ok {
		t.Fatal("Tool.Parameters should be a map")
	}

	requiredRaw, hasRequired := params["required"]
	if hasRequired {
		required, ok := requiredRaw.([]string)
		if ok {
			// Only description and subagent_type should be required
			// (model and resume have "optional" in description)
			for _, r := range required {
				if r == "model" || r == "resume" {
					t.Errorf("Parameter %q should NOT be in required array (it's optional)", r)
				}
			}
		}
	}

	// Log for debugging
	jsonData, _ := json.MarshalIndent(tool, "", "  ")
	t.Logf("Generated tool JSON:\n%s", string(jsonData))
}

func TestTransformToolsToResponses_AdditionalPropertiesFalse(t *testing.T) {
	// Test that tools are created with additionalProperties: false
	// OpenAI Responses API REQUIRES this field even when strict: false is set.
	// Without it, you get: "Invalid schema for function '...': 'additionalProperties'
	// is required to be supplied and to be false"
	tools := []models.AnthropicTool{
		{
			Name:        "Task",
			Description: "Launch a task agent",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Task description",
					},
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "The task for the agent to perform",
					},
					"subagent_type": map[string]interface{}{
						"type":        "string",
						"description": "Type of specialized agent",
					},
				},
				"required": []interface{}{"description", "prompt", "subagent_type"},
			},
		},
	}

	result := transformToolsToResponses(tools)

	if len(result) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(result))
	}

	tool := result[0]

	// Verify additionalProperties is false at top level
	params, ok := tool.Parameters.(map[string]interface{})
	if !ok {
		t.Fatal("Tool.Parameters should be a map")
	}

	additionalProps, hasAdditional := params["additionalProperties"]
	if !hasAdditional {
		t.Error("Tool.Parameters should have additionalProperties field (required by OpenAI Responses API)")
	} else if additionalProps != false {
		t.Errorf("Tool.Parameters.additionalProperties should be false, got %v", additionalProps)
	}

	// Log for debugging
	jsonData, _ := json.MarshalIndent(tool, "", "  ")
	t.Logf("Generated tool JSON with additionalProperties:\n%s", string(jsonData))
}

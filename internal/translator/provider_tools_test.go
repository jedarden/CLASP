// Package translator handles protocol translation between Anthropic and OpenAI formats.
package translator

import (
	"encoding/json"
	"testing"

	"github.com/jedarden/clasp/pkg/models"
)

// TestDetectProviderFromModel tests provider detection from model name patterns.

func TestDetectProviderFromModel_Gemini(t *testing.T) {
	tests := []struct {
		model    string
		expected ProviderType
	}{
		{"gemini-2.5-pro", ProviderGemini},
		{"gemini-2.5-flash", ProviderGemini},
		{"gemini-3-flash", ProviderGemini},
		{"gemini-pro", ProviderGemini},
		{"google/gemini-2.5-pro", ProviderGemini},
		{"google/gemini-ultra", ProviderGemini},
		{"GEMINI-PRO", ProviderGemini}, // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := DetectProviderFromModel(tt.model)
			if result != tt.expected {
				t.Errorf("DetectProviderFromModel(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

func TestDetectProviderFromModel_DeepSeek(t *testing.T) {
	tests := []struct {
		model    string
		expected ProviderType
	}{
		{"deepseek-chat", ProviderDeepSeek},
		{"deepseek-coder", ProviderDeepSeek},
		{"deepseek-r1", ProviderDeepSeek},
		{"deepseek-v3", ProviderDeepSeek},
		{"deepseek/deepseek-r1", ProviderDeepSeek},
		{"deepseek/DEEPSEEK-CHAT", ProviderDeepSeek}, // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := DetectProviderFromModel(tt.model)
			if result != tt.expected {
				t.Errorf("DetectProviderFromModel(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

func TestDetectProviderFromModel_Qwen(t *testing.T) {
	tests := []struct {
		model    string
		expected ProviderType
	}{
		{"qwen-2.5-72b", ProviderQwen},
		{"qwen-turbo", ProviderQwen},
		{"qwen-plus", ProviderQwen},
		{"qwen-coder", ProviderQwen},
		{"qwen/qwen-2.5-pro", ProviderQwen},
		{"QWEN-PRO", ProviderQwen}, // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := DetectProviderFromModel(tt.model)
			if result != tt.expected {
				t.Errorf("DetectProviderFromModel(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

func TestDetectProviderFromModel_MiniMax(t *testing.T) {
	tests := []struct {
		model    string
		expected ProviderType
	}{
		{"minimax-01", ProviderMiniMax},
		{"minimax-pro", ProviderMiniMax},
		{"minimax/minimax-01", ProviderMiniMax},
		{"MINIMAX-PRO", ProviderMiniMax}, // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := DetectProviderFromModel(tt.model)
			if result != tt.expected {
				t.Errorf("DetectProviderFromModel(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

func TestDetectProviderFromModel_Grok(t *testing.T) {
	tests := []struct {
		model    string
		expected ProviderType
	}{
		{"grok-3", ProviderGrok},
		{"grok-2", ProviderGrok},
		{"grok-beta", ProviderGrok},
		{"x-ai/grok-3", ProviderGrok},
		{"x-ai/grok-3-mini", ProviderGrok},
		{"X-AI/grok-2", ProviderGrok}, // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := DetectProviderFromModel(tt.model)
			if result != tt.expected {
				t.Errorf("DetectProviderFromModel(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

func TestDetectProviderFromModel_Ollama(t *testing.T) {
	tests := []struct {
		model    string
		expected ProviderType
	}{
		{"llama3", ProviderOllama},
		{"llama3:latest", ProviderOllama},
		{"mistral", ProviderOllama},
		{"mistral-7b", ProviderOllama},
		{"phi3", ProviderOllama},
		{"phi-3-medium", ProviderOllama},
		{"codellama", ProviderOllama},
		{"codellama:13b", ProviderOllama},
		{"gemma", ProviderOllama},
		{"gemma:7b", ProviderOllama},
		{"LLAMA3", ProviderOllama}, // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := DetectProviderFromModel(tt.model)
			if result != tt.expected {
				t.Errorf("DetectProviderFromModel(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

func TestDetectProviderFromModel_OpenRouter(t *testing.T) {
	tests := []struct {
		model    string
		expected ProviderType
	}{
		{"openai/gpt-4o", ProviderOpenRouter},
		{"anthropic/claude-3-5-sonnet", ProviderOpenRouter},
		{"meta-llama/llama-3", ProviderOpenRouter},
		{"unknown-provider/model", ProviderOpenRouter},
		{"cohere/command-r", ProviderOpenRouter},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := DetectProviderFromModel(tt.model)
			if result != tt.expected {
				t.Errorf("DetectProviderFromModel(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

func TestDetectProviderFromModel_OpenAI(t *testing.T) {
	tests := []struct {
		model    string
		expected ProviderType
	}{
		{"gpt-4o", ProviderOpenAI},
		{"gpt-4o-mini", ProviderOpenAI},
		{"gpt-4-turbo", ProviderOpenAI},
		{"gpt-3.5-turbo", ProviderOpenAI},
		{"o1", ProviderOpenAI},
		{"o1-preview", ProviderOpenAI},
		{"o3-mini", ProviderOpenAI},
		{"claude-3-opus-20240229", ProviderOpenAI}, // non-gemini Claude models default to OpenAI
		{"", ProviderOpenAI},                         // empty string defaults to OpenAI
		{"random-model-name", ProviderOpenAI},        // unknown defaults to OpenAI
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := DetectProviderFromModel(tt.model)
			if result != tt.expected {
				t.Errorf("DetectProviderFromModel(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

// TestTransformToolsForProvider tests per-provider tool schema transformation.

func TestTransformToolsForProvider_OpenAI(t *testing.T) {
	tools := []models.AnthropicTool{
		{
			Name:        "calculator",
			Description: "Perform math operations",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"expression": map[string]interface{}{
						"type":        "string",
						"description": "Math expression to evaluate",
					},
				},
				"required": []interface{}{"expression"},
			},
		},
	}

	result := TransformToolsForProvider(tools, ProviderOpenAI, "gpt-4o")

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	if result[0].Type != "function" {
		t.Errorf("result[0].Type = %q, want %q", result[0].Type, "function")
	}

	if result[0].Function.Name != "calculator" {
		t.Errorf("result[0].Function.Name = %q, want %q", result[0].Function.Name, "calculator")
	}

	if result[0].Function.Description != "Perform math operations" {
		t.Errorf("result[0].Function.Description = %q, want %q", result[0].Function.Description, "Perform math operations")
	}

	// OpenAI should have strict=false
	if result[0].Function.Strict != false {
		t.Errorf("result[0].Function.Strict = %v, want false", result[0].Function.Strict)
	}
}

func TestTransformToolsForProvider_Gemini(t *testing.T) {
	tools := []models.AnthropicTool{
		{
			Name:        "search_web",
			Description: "Search the web",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query",
					},
					"url": map[string]interface{}{
						"type":   "string",
						"format": "uri", // Should be removed for Gemini
					},
				},
				"required": []interface{}{"query"},
			},
		},
	}

	result := TransformToolsForProvider(tools, ProviderGemini, "gemini-2.5-pro")

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	// Check that uri format was removed
	params, ok := result[0].Function.Parameters.(map[string]interface{})
	if !ok {
		t.Fatal("Parameters should be a map")
	}

	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Properties should be a map")
	}

	urlProp, ok := props["url"].(map[string]interface{})
	if !ok {
		t.Fatal("url property should exist")
	}

	if _, hasFormat := urlProp["format"]; hasFormat {
		t.Error("uri format should have been removed for Gemini")
	}

	// Gemini 2.5 should have strict=false (only Gemini 3+ has strict=true)
	if result[0].Function.Strict != false {
		t.Errorf("Gemini 2.5 should have strict=false, got %v", result[0].Function.Strict)
	}
}

func TestTransformToolsForProvider_Gemini3_StrictMode(t *testing.T) {
	tools := []models.AnthropicTool{
		{
			Name:        "test_tool",
			Description: "Test tool",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	result := TransformToolsForProvider(tools, ProviderGemini, "gemini-3-pro")

	if !result[0].Function.Strict {
		t.Error("Gemini 3 should have strict=true")
	}

	// Test with google/gemini/3 prefix
	result2 := TransformToolsForProvider(tools, ProviderGemini, "google/gemini-3-flash")

	if !result2[0].Function.Strict {
		t.Error("Gemini 3 with google/ prefix should have strict=true")
	}
}

func TestTransformToolsForProvider_DeepSeek(t *testing.T) {
	tools := []models.AnthropicTool{
		{
			Name:        "code_executor",
			Description: "Execute code",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"code": map[string]interface{}{
						"type":        "string",
						"description": "Code to execute",
					},
				},
				"required": []interface{}{"code"},
			},
		},
	}

	result := TransformToolsForProvider(tools, ProviderDeepSeek, "deepseek-chat")

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	// DeepSeek non-v3 models should have strict=false
	if result[0].Function.Strict != false {
		t.Errorf("DeepSeek chat should have strict=false, got %v", result[0].Function.Strict)
	}
}

func TestTransformToolsForProvider_DeepSeekV3_StrictMode(t *testing.T) {
	tools := []models.AnthropicTool{
		{
			Name:        "test_tool",
			Description: "Test tool",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	tests := []struct {
		model         string
		expectStrict  bool
	}{
		{"deepseek-v3", true},
		{"deepseek-v3.1", true},
		{"deepseek-v3.2", true},
		{"deepseek-chat", false},
		{"deepseek-coder", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := TransformToolsForProvider(tools, ProviderDeepSeek, tt.model)
			if result[0].Function.Strict != tt.expectStrict {
				t.Errorf("DeepSeek %s: strict = %v, want %v", tt.model, result[0].Function.Strict, tt.expectStrict)
			}
		})
	}
}

func TestTransformToolsForProvider_Qwen(t *testing.T) {
	tools := []models.AnthropicTool{
		{
			Name:        "analyze",
			Description: "Analyze data",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"data": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}

	result := TransformToolsForProvider(tools, ProviderQwen, "qwen-2.5-72b")

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	// Qwen doesn't support strict mode
	if result[0].Function.Strict != false {
		t.Errorf("Qwen should have strict=false, got %v", result[0].Function.Strict)
	}
}

func TestTransformToolsForProvider_Grok(t *testing.T) {
	tools := []models.AnthropicTool{
		{
			Name:        "post_tweet",
			Description: "Post a tweet",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}

	result := TransformToolsForProvider(tools, ProviderGrok, "grok-3")

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	// Grok doesn't use strict mode
	if result[0].Function.Strict != false {
		t.Errorf("Grok should have strict=false, got %v", result[0].Function.Strict)
	}
}

func TestTransformToolsForProvider_Ollama(t *testing.T) {
	tools := []models.AnthropicTool{
		{
			Name:        "simple_tool",
			Description: "Simple tool for Ollama",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":     "string",
						"format":   "uri",
						"pattern":  "^[a-z]+$",
						"minLength": 1,
						"maxLength": 100,
					},
					"count": map[string]interface{}{
						"type":    "integer",
						"minimum": 0,
						"maximum": 100,
					},
					"items": map[string]interface{}{
						"type":      "array",
						"minItems":  1,
						"maxItems":  10,
					},
				},
				"required": []interface{}{"text", "count", "items"},
			},
		},
	}

	result := TransformToolsForProvider(tools, ProviderOllama, "llama3")

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	// Check that advanced schema features were removed
	params, ok := result[0].Function.Parameters.(map[string]interface{})
	if !ok {
		t.Fatal("Parameters should be a map")
	}

	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Properties should be a map")
	}

	textProp, ok := props["text"].(map[string]interface{})
	if !ok {
		t.Fatal("text property should exist")
	}

	// Ollama should remove format, pattern, minLength, maxLength
	if _, hasFormat := textProp["format"]; hasFormat {
		t.Error("format should have been removed for Ollama")
	}
	if _, hasPattern := textProp["pattern"]; hasPattern {
		t.Error("pattern should have been removed for Ollama")
	}
	if _, hasMinLength := textProp["minLength"]; hasMinLength {
		t.Error("minLength should have been removed for Ollama")
	}
	if _, hasMaxLength := textProp["maxLength"]; hasMaxLength {
		t.Error("maxLength should have been removed for Ollama")
	}

	countProp, ok := props["count"].(map[string]interface{})
	if !ok {
		t.Fatal("count property should exist")
	}

	if _, hasMinimum := countProp["minimum"]; hasMinimum {
		t.Error("minimum should have been removed for Ollama")
	}
	if _, hasMaximum := countProp["maximum"]; hasMaximum {
		t.Error("maximum should have been removed for Ollama")
	}

	itemsProp, ok := props["items"].(map[string]interface{})
	if !ok {
		t.Fatal("items property should exist")
	}

	if _, hasMinItems := itemsProp["minItems"]; hasMinItems {
		t.Error("minItems should have been removed for Ollama")
	}
	if _, hasMaxItems := itemsProp["maxItems"]; hasMaxItems {
		t.Error("maxItems should have been removed for Ollama")
	}

	// Ollama should filter required to only non-boolean types
	req, ok := params["required"].([]string)
	if !ok {
		t.Fatal("required should be []string")
	}

	// text (string) should remain, count (integer) and items (array) might be filtered
	if len(req) == 0 {
		t.Error("Ollama should keep some required fields")
	}
}

func TestTransformToolsForProvider_MultipleTools(t *testing.T) {
	tools := []models.AnthropicTool{
		{
			Name:        "tool1",
			Description: "First tool",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "tool2",
			Description: "Second tool",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "tool3",
			Description: "Third tool",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	result := TransformToolsForProvider(tools, ProviderOpenAI, "gpt-4o")

	if len(result) != 3 {
		t.Fatalf("len(result) = %d, want 3", len(result))
	}

	if result[0].Function.Name != "tool1" {
		t.Errorf("result[0].Function.Name = %q, want %q", result[0].Function.Name, "tool1")
	}
	if result[1].Function.Name != "tool2" {
		t.Errorf("result[1].Function.Name = %q, want %q", result[1].Function.Name, "tool2")
	}
	if result[2].Function.Name != "tool3" {
		t.Errorf("result[2].Function.Name = %q, want %q", result[2].Function.Name, "tool3")
	}
}

func TestTransformToolsForProvider_EmptyTools(t *testing.T) {
	tools := []models.AnthropicTool{}

	result := TransformToolsForProvider(tools, ProviderOpenAI, "gpt-4o")

	if len(result) != 0 {
		t.Fatalf("len(result) = %d, want 0", len(result))
	}
}

// TestCleanupSchemaForGemini tests Gemini-specific schema cleanup.

func TestCleanupSchemaForGemini_RemovesURIFormat(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":   "string",
				"format": "uri",
			},
			"ref": map[string]interface{}{
				"type":   "string",
				"format": "uri-reference",
			},
		},
	}

	result := cleanupSchemaForGemini(schema)
	resultMap := result.(map[string]interface{})
	props := resultMap["properties"].(map[string]interface{})

	urlProp := props["url"].(map[string]interface{})
	if _, hasFormat := urlProp["format"]; hasFormat {
		t.Error("uri format should be removed")
	}

	refProp := props["ref"].(map[string]interface{})
	if _, hasFormat := refProp["format"]; hasFormat {
		t.Error("uri-reference format should be removed")
	}
}

func TestCleanupSchemaForGemini_AddsDescriptions(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"param1": map[string]interface{}{
				"type": "string",
			},
		},
	}

	result := cleanupSchemaForGemini(schema)
	resultMap := result.(map[string]interface{})
	props := resultMap["properties"].(map[string]interface{})

	param1 := props["param1"].(map[string]interface{})
	if desc, ok := param1["description"]; !ok || desc == "" {
		t.Error("Gemini requires descriptions for all properties")
	}
}

// TestCleanupSchemaForDeepSeek tests DeepSeek-specific schema cleanup.

func TestCleanupSchemaForDeepSeek_V32AddsAdditionalProperties(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"param": map[string]interface{}{
				"type": "string",
			},
		},
	}

	result := cleanupSchemaForDeepSeek(schema, "deepseek-v3.2")
	resultMap := result.(map[string]interface{})

	if addProps, ok := resultMap["additionalProperties"]; !ok || addProps != false {
		t.Error("DeepSeek V3.2 should set additionalProperties to false")
	}
}

func TestCleanupSchemaForDeepSeek_RemovesURIFormat(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":   "string",
				"format": "uri",
			},
		},
	}

	result := cleanupSchemaForDeepSeek(schema, "deepseek-v3")
	resultMap := result.(map[string]interface{})
	props := resultMap["properties"].(map[string]interface{})

	urlProp := props["url"].(map[string]interface{})
	if _, hasFormat := urlProp["format"]; hasFormat {
		t.Error("uri format should be removed for DeepSeek")
	}
}

// TestProviderSupportsTools tests tool support detection.

func TestProviderSupportsTools(t *testing.T) {
	tests := []struct {
		name        string
		provider    ProviderType
		model       string
		expectSupport bool
	}{
		{"OpenAI", ProviderOpenAI, "gpt-4o", true},
		{"Azure", ProviderAzure, "gpt-4o", true},
		{"OpenRouter", ProviderOpenRouter, "openai/gpt-4o", true},
		{"Gemini", ProviderGemini, "gemini-2.5-pro", true},
		{"DeepSeek chat", ProviderDeepSeek, "deepseek-chat", true},
		{"DeepSeek R1 Speciale", ProviderDeepSeek, "deepseek-r1-speciale", false},
		{"Qwen", ProviderQwen, "qwen-2.5-72b", true},
		{"MiniMax", ProviderMiniMax, "minimax-01", true},
		{"Grok", ProviderGrok, "grok-3", true},
		{"Ollama llama3", ProviderOllama, "llama3", true},
		{"Ollama mistral", ProviderOllama, "mistral", true},
		{"Ollama mixtral", ProviderOllama, "mixtral", true},
		{"Ollama command-r", ProviderOllama, "command-r", true},
		{"Ollama unsupported model", ProviderOllama, "some-model", false},
		{"Custom", ProviderCustom, "custom-model", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProviderSupportsTools(tt.provider, tt.model)
			if result != tt.expectSupport {
				t.Errorf("ProviderSupportsTools(%q, %q) = %v, want %v", tt.provider, tt.model, result, tt.expectSupport)
			}
		})
	}
}

// TestProviderRequiresThoughtSignature tests thought signature requirement detection.

func TestProviderRequiresThoughtSignature(t *testing.T) {
	tests := []struct {
		name        string
		provider    ProviderType
		model       string
		expectRequired bool
	}{
		{"Gemini 3", ProviderGemini, "gemini-3-pro", true},
		{"Gemini 3 with prefix", ProviderGemini, "google/gemini-3-flash", true},
		{"Gemini 2.5", ProviderGemini, "gemini-2.5-pro", false},
		{"Gemini 2", ProviderGemini, "gemini-pro", false},
		{"OpenAI", ProviderOpenAI, "gpt-4o", false},
		{"DeepSeek", ProviderDeepSeek, "deepseek-chat", false},
		{"Qwen", ProviderQwen, "qwen-2.5", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProviderRequiresThoughtSignature(tt.provider, tt.model)
			if result != tt.expectRequired {
				t.Errorf("ProviderRequiresThoughtSignature(%q, %q) = %v, want %v", tt.provider, tt.model, result, tt.expectRequired)
			}
		})
	}
}

// TestTransformToolsForProvider_NilParameters tests handling of nil parameters.

func TestTransformToolsForProvider_NilParameters(t *testing.T) {
	tools := []models.AnthropicTool{
		{
			Name:        "no_params",
			Description: "Tool with no parameters",
			InputSchema: nil,
		},
	}

	result := TransformToolsForProvider(tools, ProviderOpenAI, "gpt-4o")

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	if result[0].Function.Parameters != nil {
		t.Error("nil parameters should remain nil")
	}
}

// TestTransformToolsForProvider_ComplexSchema tests complex nested schema transformation.

func TestTransformToolsForProvider_ComplexSchema(t *testing.T) {
	tools := []models.AnthropicTool{
		{
			Name:        "complex_tool",
			Description: "Tool with complex schema",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"nested": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"inner": map[string]interface{}{
								"type":   "string",
								"format": "uri",
							},
						},
					},
					"array": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type":   "string",
							"format": "uri",
						},
					},
				},
			},
		},
	}

	result := TransformToolsForProvider(tools, ProviderGemini, "gemini-2.5-pro")

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	// Verify nested cleanup happened
	params, ok := result[0].Function.Parameters.(map[string]interface{})
	if !ok {
		t.Fatal("Parameters should be a map")
	}

	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Properties should be a map")
	}

	nested, ok := props["nested"].(map[string]interface{})
	if !ok {
		t.Fatal("nested property should exist")
	}

	nestedProps, ok := nested["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("nested properties should exist")
	}

	inner, ok := nestedProps["inner"].(map[string]interface{})
	if !ok {
		t.Fatal("inner property should exist")
	}

	if _, hasFormat := inner["format"]; hasFormat {
		t.Error("format should be removed from nested property")
	}

	arrayItems, ok := props["array"].(map[string]interface{})
	if !ok {
		t.Fatal("array property should exist")
	}

	items, ok := arrayItems["items"].(map[string]interface{})
	if !ok {
		t.Fatal("items should exist")
	}

	if _, hasFormat := items["format"]; hasFormat {
		t.Error("format should be removed from array items")
	}
}

// TestTransformToolsForProvider_JSONMarshaling tests that tools can be marshaled to JSON.

func TestTransformToolsForProvider_JSONMarshaling(t *testing.T) {
	tools := []models.AnthropicTool{
		{
			Name:        "test_tool",
			Description: "Test tool for JSON marshaling",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"param1": map[string]interface{}{
						"type":        "string",
						"description": "First parameter",
					},
				},
				"required": []interface{}{"param1"},
			},
		},
	}

	result := TransformToolsForProvider(tools, ProviderOpenAI, "gpt-4o")

	// Try to marshal to JSON
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal to JSON: %v", err)
	}

	// Try to unmarshal back
	var unmarshaled []models.OpenAITool
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal from JSON: %v", err)
	}

	if len(unmarshaled) != 1 {
		t.Fatalf("len(unmarshaled) = %d, want 1", len(unmarshaled))
	}

	if unmarshaled[0].Function.Name != "test_tool" {
		t.Errorf("unmarshaled[0].Function.Name = %q, want %q", unmarshaled[0].Function.Name, "test_tool")
	}
}

// TestDetectProviderFromModel_EdgeCases tests edge cases for provider detection.

func TestDetectProviderFromModel_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		expected    ProviderType
	}{
		{"empty string", "", ProviderOpenAI},
		{"case insensitive gemini", "GEMINI-PRO", ProviderGemini},
		{"case insensitive deepseek", "DEEPSEEK-CHAT", ProviderDeepSeek},
		{"case insensitive qwen", "QWEN-TURBO", ProviderQwen},
		{"case insensitive minimax", "MINIMAX-PRO", ProviderMiniMax},
		{"case insensitive grok", "GROK-3", ProviderGrok},
		{"case insensitive llama", "LLAMA3", ProviderOllama},
		{"case insensitive mistral", "MISTRAL", ProviderOllama},
		{"case insensitive phi", "PHI3", ProviderOllama},
		{"case insensitive codellama", "CODELLAMA", ProviderOllama},
		{"case insensitive gemma", "GEMMA", ProviderOllama},
		{"google prefix with slash", "google/gemini-2.5-pro", ProviderGemini},
		{"deepseek prefix with slash", "deepseek/deepseek-r1", ProviderDeepSeek},
		{"qwen prefix with slash", "qwen/qwen-2.5", ProviderQwen},
		{"minimax prefix with slash", "minimax/minimax-01", ProviderMiniMax},
		{"x-ai prefix with slash", "x-ai/grok-3", ProviderGrok},
		{"openrouter openai", "openai/gpt-4o", ProviderOpenRouter},
		{"openrouter anthropic", "anthropic/claude-3-5-sonnet", ProviderOpenRouter},
		{"google models are gemini not openrouter", "google/gemini-pro", ProviderGemini},
		{"openrouter unknown", "unknown/model", ProviderOpenRouter},
		{"llama3 with tag", "llama3:latest", ProviderOllama},
		{"mistral with variant", "mistral-7b-instruct", ProviderOllama},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectProviderFromModel(tt.model)
			if result != tt.expected {
				t.Errorf("DetectProviderFromModel(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

// TestTransformToolsForProvider_Azure tests Azure provider uses standard OpenAI format.

func TestTransformToolsForProvider_Azure(t *testing.T) {
	tools := []models.AnthropicTool{
		{
			Name:        "azure_tool",
			Description: "Tool for Azure",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	result := TransformToolsForProvider(tools, ProviderAzure, "gpt-4o")

	// Azure should use same format as OpenAI
	if result[0].Type != "function" {
		t.Errorf("Azure should use function type, got %q", result[0].Type)
	}

	if result[0].Function.Strict != false {
		t.Error("Azure should have strict=false")
	}
}

// TestTransformToolsForProvider_Custom tests custom provider uses standard OpenAI format.

func TestTransformToolsForProvider_Custom(t *testing.T) {
	tools := []models.AnthropicTool{
		{
			Name:        "custom_tool",
			Description: "Tool for custom provider",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	result := TransformToolsForProvider(tools, ProviderCustom, "custom-model")

	// Custom should use same format as OpenAI
	if result[0].Type != "function" {
		t.Errorf("Custom should use function type, got %q", result[0].Type)
	}

	if result[0].Function.Strict != false {
		t.Error("Custom should have strict=false")
	}
}

// Package translator handles protocol translation between Anthropic and OpenAI formats.
package translator

import (
	"encoding/json"
	"strings"

	"github.com/jedarden/clasp/pkg/models"
)

// ProviderType identifies the target provider for tool transformation.
type ProviderType string

const (
	ProviderOpenAI     ProviderType = "openai"
	ProviderAzure      ProviderType = "azure"
	ProviderOpenRouter ProviderType = "openrouter"
	ProviderGemini     ProviderType = "gemini"
	ProviderDeepSeek   ProviderType = "deepseek"
	ProviderOllama     ProviderType = "ollama"
	ProviderQwen       ProviderType = "qwen"
	ProviderMiniMax    ProviderType = "minimax"
	ProviderGrok       ProviderType = "grok"
	ProviderCustom     ProviderType = "custom"
)

// DetectProviderFromModel determines the provider based on model name patterns.
func DetectProviderFromModel(model string) ProviderType {
	m := strings.ToLower(model)

	switch {
	case strings.HasPrefix(m, "gemini") || strings.Contains(m, "google/"):
		return ProviderGemini
	case strings.HasPrefix(m, "deepseek") || strings.Contains(m, "deepseek/"):
		return ProviderDeepSeek
	case strings.HasPrefix(m, "qwen") || strings.Contains(m, "qwen/"):
		return ProviderQwen
	case strings.HasPrefix(m, "minimax") || strings.Contains(m, "minimax/"):
		return ProviderMiniMax
	case strings.HasPrefix(m, "grok") || strings.HasPrefix(m, "x-ai/"):
		return ProviderGrok
	case strings.HasPrefix(m, "llama") || strings.HasPrefix(m, "mistral") ||
		strings.HasPrefix(m, "phi") || strings.HasPrefix(m, "codellama") ||
		strings.HasPrefix(m, "gemma"):
		return ProviderOllama
	case strings.Contains(m, "/"):
		// Has provider prefix like "openai/gpt-4o" or "anthropic/claude"
		return ProviderOpenRouter
	default:
		return ProviderOpenAI
	}
}

// TransformToolsForProvider converts Anthropic tools to provider-specific format.
func TransformToolsForProvider(tools []models.AnthropicTool, provider ProviderType, model string) []models.OpenAITool {
	result := make([]models.OpenAITool, len(tools))

	for i, tool := range tools {
		result[i] = transformToolForProvider(tool, provider, model)
	}

	return result
}

// transformToolForProvider transforms a single tool for the target provider.
func transformToolForProvider(tool models.AnthropicTool, provider ProviderType, model string) models.OpenAITool {
	// Get base tool definition
	toolName := tool.Name
	toolDescription := tool.Description
	toolParams := tool.InputSchema

	// Handle computer use tools
	if isComputerUseTool(tool.Type) {
		toolName, toolDescription, toolParams = transformComputerUseTool(tool)
	} else if IsClaudeCodeTool(tool.Name) {
		toolName, toolDescription, toolParams = GetClaudeCodeToolDefinition(tool)
	}

	// Apply provider-specific transformations
	switch provider {
	case ProviderGemini:
		return transformToolForGemini(toolName, toolDescription, toolParams, model)
	case ProviderDeepSeek:
		return transformToolForDeepSeek(toolName, toolDescription, toolParams, model)
	case ProviderQwen:
		return transformToolForQwen(toolName, toolDescription, toolParams)
	case ProviderGrok:
		return transformToolForGrok(toolName, toolDescription, toolParams)
	case ProviderOllama:
		return transformToolForOllama(toolName, toolDescription, toolParams)
	default:
		// OpenAI, Azure, OpenRouter, Custom - use standard OpenAI format
		return transformToolForOpenAI(toolName, toolDescription, toolParams)
	}
}

// transformToolForOpenAI creates standard OpenAI function format.
func transformToolForOpenAI(name, description string, params interface{}) models.OpenAITool {
	cleanedParams := cleanupSchemaForChatCompletions(params)

	return models.OpenAITool{
		Type: "function",
		Function: models.OpenAIFunction{
			Name:        name,
			Description: description,
			Parameters:  cleanedParams,
			Strict:      false,
		},
	}
}

// transformToolForGemini creates Gemini-compatible function format.
// Gemini uses OpenAPI schema format with some specific requirements:
// - Strict validation in Gemini 3+
// - thoughtSignature support for function call continuity
// - No format: "uri" support
func transformToolForGemini(name, description string, params interface{}, model string) models.OpenAITool {
	// Clean up schema for Gemini
	cleanedParams := cleanupSchemaForGemini(params)

	// Gemini 3+ uses strict function calling by default
	isGemini3Plus := strings.Contains(strings.ToLower(model), "gemini-3") ||
		strings.Contains(strings.ToLower(model), "gemini/3")

	return models.OpenAITool{
		Type: "function",
		Function: models.OpenAIFunction{
			Name:        name,
			Description: description,
			Parameters:  cleanedParams,
			Strict:      isGemini3Plus, // Gemini 3+ enforces strict validation
		},
	}
}

// cleanupSchemaForGemini prepares a schema for Gemini's function calling.
func cleanupSchemaForGemini(schema interface{}) interface{} {
	if schema == nil {
		return nil
	}

	data, err := json.Marshal(schema)
	if err != nil {
		return schema
	}

	var schemaMap map[string]interface{}
	if err := json.Unmarshal(data, &schemaMap); err != nil {
		return schema
	}

	cleanupSchemaMapForGemini(schemaMap)
	return schemaMap
}

// cleanupSchemaMapForGemini recursively cleans up schema for Gemini.
func cleanupSchemaMapForGemini(schema map[string]interface{}) {
	// Remove unsupported format types
	if format, ok := schema["format"].(string); ok {
		if format == "uri" || format == "uri-reference" {
			delete(schema, "format")
		}
	}

	// Gemini requires description for all properties
	if _, hasDesc := schema["description"]; !hasDesc {
		if _, hasType := schema["type"]; hasType {
			schema["description"] = "Parameter value"
		}
	}

	// Process properties
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		for propName, v := range props {
			if propMap, ok := v.(map[string]interface{}); ok {
				// Ensure each property has a description
				if _, hasDesc := propMap["description"]; !hasDesc {
					propMap["description"] = "The " + propName + " parameter"
				}
				cleanupSchemaMapForGemini(propMap)
			}
		}
	}

	// Recurse into items (for arrays)
	if items, ok := schema["items"].(map[string]interface{}); ok {
		cleanupSchemaMapForGemini(items)
	}
}

// transformToolForDeepSeek creates DeepSeek-compatible function format.
// DeepSeek V3.1+ supports strict function calling and thinking with tools.
// DeepSeek uses OpenAI-compatible format but with some specifics.
func transformToolForDeepSeek(name, description string, params interface{}, model string) models.OpenAITool {
	// Clean up schema for DeepSeek
	cleanedParams := cleanupSchemaForDeepSeek(params, model)

	// DeepSeek V3.1+ supports strict mode
	m := strings.ToLower(model)
	supportsStrict := strings.Contains(m, "v3.1") || strings.Contains(m, "v3.2") ||
		strings.Contains(m, "deepseek-v3")

	return models.OpenAITool{
		Type: "function",
		Function: models.OpenAIFunction{
			Name:        name,
			Description: description,
			Parameters:  cleanedParams,
			Strict:      supportsStrict,
		},
	}
}

// cleanupSchemaForDeepSeek prepares a schema for DeepSeek's function calling.
func cleanupSchemaForDeepSeek(schema interface{}, model string) interface{} {
	if schema == nil {
		return nil
	}

	data, err := json.Marshal(schema)
	if err != nil {
		return schema
	}

	var schemaMap map[string]interface{}
	if err := json.Unmarshal(data, &schemaMap); err != nil {
		return schema
	}

	// DeepSeek V3.2 has specific tool calling format changes
	m := strings.ToLower(model)
	isV32 := strings.Contains(m, "v3.2")

	cleanupSchemaMapForDeepSeek(schemaMap, isV32)
	return schemaMap
}

// cleanupSchemaMapForDeepSeek recursively cleans up schema for DeepSeek.
func cleanupSchemaMapForDeepSeek(schema map[string]interface{}, isV32 bool) {
	// Remove unsupported format types
	if format, ok := schema["format"].(string); ok {
		if format == "uri" {
			delete(schema, "format")
		}
	}

	// DeepSeek V3.2 requires additionalProperties: false for strict mode
	if isV32 {
		if schemaType, ok := schema["type"].(string); ok && schemaType == "object" {
			schema["additionalProperties"] = false
		}
	}

	// Process properties and filter required array
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		// For DeepSeek, keep only truly required parameters
		trulyRequired := identifyTrulyRequiredForChat(props, schema)
		if len(trulyRequired) > 0 {
			schema["required"] = trulyRequired
		} else {
			delete(schema, "required")
		}

		for _, v := range props {
			if propMap, ok := v.(map[string]interface{}); ok {
				cleanupSchemaMapForDeepSeek(propMap, isV32)
			}
		}
	}

	// Recurse into items
	if items, ok := schema["items"].(map[string]interface{}); ok {
		cleanupSchemaMapForDeepSeek(items, isV32)
	}
}

// transformToolForQwen creates Qwen-compatible function format.
// Qwen uses OpenAI-compatible format with some extensions.
func transformToolForQwen(name, description string, params interface{}) models.OpenAITool {
	cleanedParams := cleanupSchemaForChatCompletions(params)

	return models.OpenAITool{
		Type: "function",
		Function: models.OpenAIFunction{
			Name:        name,
			Description: description,
			Parameters:  cleanedParams,
			Strict:      false, // Qwen doesn't enforce strict mode
		},
	}
}

// transformToolForGrok creates Grok-compatible function format.
// Grok uses OpenAI-compatible format but may use XML for function calls
// (which we handle with system message injection).
func transformToolForGrok(name, description string, params interface{}) models.OpenAITool {
	cleanedParams := cleanupSchemaForChatCompletions(params)

	return models.OpenAITool{
		Type: "function",
		Function: models.OpenAIFunction{
			Name:        name,
			Description: description,
			Parameters:  cleanedParams,
			Strict:      false,
		},
	}
}

// transformToolForOllama creates Ollama-compatible function format.
// Ollama uses OpenAI-compatible format at /v1/chat/completions.
func transformToolForOllama(name, description string, params interface{}) models.OpenAITool {
	// Ollama has limited function calling support
	// Keep schemas simple for better compatibility
	cleanedParams := cleanupSchemaForOllama(params)

	return models.OpenAITool{
		Type: "function",
		Function: models.OpenAIFunction{
			Name:        name,
			Description: description,
			Parameters:  cleanedParams,
			Strict:      false,
		},
	}
}

// cleanupSchemaForOllama simplifies a schema for Ollama's limited function calling.
func cleanupSchemaForOllama(schema interface{}) interface{} {
	if schema == nil {
		return nil
	}

	data, err := json.Marshal(schema)
	if err != nil {
		return schema
	}

	var schemaMap map[string]interface{}
	if err := json.Unmarshal(data, &schemaMap); err != nil {
		return schema
	}

	cleanupSchemaMapForOllama(schemaMap)
	return schemaMap
}

// cleanupSchemaMapForOllama simplifies schema for Ollama.
func cleanupSchemaMapForOllama(schema map[string]interface{}) {
	// Remove format types that Ollama may not support
	delete(schema, "format")

	// Remove advanced JSON Schema features
	delete(schema, "pattern")
	delete(schema, "minLength")
	delete(schema, "maxLength")
	delete(schema, "minimum")
	delete(schema, "maximum")
	delete(schema, "minItems")
	delete(schema, "maxItems")

	// Keep required array minimal
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		// Only keep absolutely essential required fields
		var essentialRequired []string
		if req, ok := schema["required"].([]interface{}); ok {
			for _, r := range req {
				if s, ok := r.(string); ok {
					// Only include non-boolean, non-optional fields
					if propVal, exists := props[s]; exists {
						if propMap, ok := propVal.(map[string]interface{}); ok {
							if propType, ok := propMap["type"].(string); ok && propType != "boolean" {
								essentialRequired = append(essentialRequired, s)
							}
						}
					}
				}
			}
		}
		if len(essentialRequired) > 0 {
			schema["required"] = essentialRequired
		} else {
			delete(schema, "required")
		}

		for _, v := range props {
			if propMap, ok := v.(map[string]interface{}); ok {
				cleanupSchemaMapForOllama(propMap)
			}
		}
	}

	// Recurse into items
	if items, ok := schema["items"].(map[string]interface{}); ok {
		cleanupSchemaMapForOllama(items)
	}
}

// ProviderSupportsTools checks if a provider supports function calling.
func ProviderSupportsTools(provider ProviderType, model string) bool {
	switch provider {
	case ProviderOpenAI, ProviderAzure, ProviderOpenRouter:
		return true
	case ProviderGemini:
		// All Gemini models support function calling
		return true
	case ProviderDeepSeek:
		// DeepSeek V3+ supports function calling
		// DeepSeek R1 Speciale does NOT support tool calling
		m := strings.ToLower(model)
		if strings.Contains(m, "speciale") {
			return false
		}
		return true
	case ProviderQwen:
		return true
	case ProviderMiniMax:
		return true
	case ProviderGrok:
		return true
	case ProviderOllama:
		// Limited support - depends on the model
		m := strings.ToLower(model)
		// Models known to support function calling in Ollama
		return strings.Contains(m, "llama3") || strings.Contains(m, "mistral") ||
			strings.Contains(m, "mixtral") || strings.Contains(m, "command-r")
	case ProviderCustom:
		// Assume custom providers support tools
		return true
	default:
		return true
	}
}

// ProviderRequiresThoughtSignature checks if provider needs thought signatures
// for multi-turn function calling (Gemini 3+ feature).
func ProviderRequiresThoughtSignature(provider ProviderType, model string) bool {
	if provider == ProviderGemini {
		m := strings.ToLower(model)
		return strings.Contains(m, "gemini-3") || strings.Contains(m, "gemini/3")
	}
	return false
}

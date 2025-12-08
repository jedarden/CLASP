// Package translator handles protocol translation between Anthropic and OpenAI formats.
package translator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jedarden/clasp/pkg/models"
)

// TransformRequestToResponses converts an Anthropic request to OpenAI Responses API format.
// This is used for models that require the /v1/responses endpoint.
func TransformRequestToResponses(req *models.AnthropicRequest, targetModel string, previousResponseID string) (*models.ResponsesRequest, error) {
	// Enforce minimum max_output_tokens of 16 (Responses API requirement)
	maxOutputTokens := req.MaxTokens
	if maxOutputTokens < 16 {
		maxOutputTokens = 16
	}

	responsesReq := &models.ResponsesRequest{
		Model:              targetModel,
		Stream:             req.Stream,
		MaxOutputTokens:    maxOutputTokens,
		Temperature:        req.Temperature,
		TopP:               req.TopP,
		PreviousResponseID: previousResponseID,
	}

	// Transform system message to instructions
	if req.System != nil {
		systemContent, err := extractSystemContent(req.System)
		if err != nil {
			return nil, fmt.Errorf("extracting system content: %w", err)
		}
		if systemContent != "" {
			// Apply identity filtering
			responsesReq.Instructions = filterIdentity(systemContent)
		}
	}

	// Build input array from messages
	inputs, err := transformMessagesToInput(req)
	if err != nil {
		return nil, fmt.Errorf("transforming messages: %w", err)
	}
	responsesReq.Input = inputs

	// Transform tools
	if len(req.Tools) > 0 {
		responsesReq.Tools = transformToolsToResponses(req.Tools)
	}

	// Transform tool choice
	if req.ToolChoice != nil {
		responsesReq.ToolChoice = transformToolChoiceToResponses(req.ToolChoice)
	}

	// Transform thinking/reasoning parameters
	applyThinkingParametersToResponses(req, responsesReq, targetModel)

	return responsesReq, nil
}

// transformMessagesToInput converts Anthropic messages to Responses input format.
func transformMessagesToInput(req *models.AnthropicRequest) ([]models.ResponsesInput, error) {
	var inputs []models.ResponsesInput

	for _, msg := range req.Messages {
		content, err := parseContent(msg.Content)
		if err != nil {
			return nil, err
		}

		switch msg.Role {
		case "user":
			input := transformUserMessageToInput(content)
			inputs = append(inputs, input)
			// Handle tool results within user message
			toolResults := extractToolResultsForResponses(content)
			inputs = append(inputs, toolResults...)

		case "assistant":
			assistantInputs := transformAssistantMessageToInput(content)
			inputs = append(inputs, assistantInputs...)
		}
	}

	return inputs, nil
}

// transformUserMessageToInput converts a user message to Responses input.
// Note: Responses API uses "input_text" and "input_image" for user content types.
func transformUserMessageToInput(content []models.ContentBlock) models.ResponsesInput {
	var parts []models.ResponsesContentPart

	for _, block := range content {
		switch block.Type {
		case "text":
			// Responses API requires "input_text" for user text content
			parts = append(parts, models.ResponsesContentPart{
				Type: "input_text",
				Text: block.Text,
			})
		case "image":
			if block.Source != nil {
				dataURL := fmt.Sprintf("data:%s;base64,%s", block.Source.MediaType, block.Source.Data)
				// Responses API requires "input_image" for image content
				parts = append(parts, models.ResponsesContentPart{
					Type: "input_image",
					ImageURL: &models.ImageURL{
						URL: dataURL,
					},
				})
			}
		}
	}

	// If only text content, use string format for simplicity
	if len(parts) == 1 && parts[0].Type == "input_text" {
		return models.ResponsesInput{
			Type:    "message",
			Role:    "user",
			Content: parts[0].Text,
		}
	}

	// Otherwise use array format with proper content types
	if len(parts) == 0 {
		return models.ResponsesInput{
			Type:    "message",
			Role:    "user",
			Content: "",
		}
	}

	return models.ResponsesInput{
		Type:    "message",
		Role:    "user",
		Content: contentPartsToResponsesInterface(parts),
	}
}

// contentPartsToResponsesInterface converts content parts to interface for JSON marshaling.
func contentPartsToResponsesInterface(parts []models.ResponsesContentPart) interface{} {
	result := make([]interface{}, len(parts))
	for i, p := range parts {
		result[i] = p
	}
	return result
}

// translateToolCallID converts tool call IDs between Anthropic/Chat Completions format and Responses API format.
// Anthropic/Chat Completions uses "call_xxx" prefix, while Responses API requires "fc_xxx" prefix.
func translateToolCallID(id string) string {
	if id == "" {
		return id
	}

	// Convert Chat Completions "call_xxx" → Responses API "fc_xxx"
	if strings.HasPrefix(id, "call_") {
		return "fc_" + strings.TrimPrefix(id, "call_")
	}

	// Convert Anthropic "toolu_xxx" → Responses API "fc_xxx"
	if strings.HasPrefix(id, "toolu_") {
		return "fc_" + strings.TrimPrefix(id, "toolu_")
	}

	// Already in Responses API format
	if strings.HasPrefix(id, "fc_") {
		return id
	}

	// Generate ID with correct prefix for any other format
	return "fc_" + id
}

// extractToolResultsForResponses extracts tool results and converts to function_call_output items.
func extractToolResultsForResponses(content []models.ContentBlock) []models.ResponsesInput {
	var results []models.ResponsesInput

	for _, block := range content {
		if block.Type == "tool_result" {
			// In Responses API, tool results are represented as function_call_output items
			// The ID must use "fc_" prefix for Responses API compatibility
			results = append(results, models.ResponsesInput{
				Type:   "function_call_output",
				CallID: translateToolCallID(block.ToolUseID),
				Output: block.Content,
			})
		}
	}

	return results
}

// transformAssistantMessageToInput converts an assistant message to Responses input items.
func transformAssistantMessageToInput(content []models.ContentBlock) []models.ResponsesInput {
	var inputs []models.ResponsesInput
	var textParts []string

	for _, block := range content {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "tool_use":
			// First, emit any accumulated text as a message
			if len(textParts) > 0 {
				inputs = append(inputs, models.ResponsesInput{
					Type:    "message",
					Role:    "assistant",
					Content: strings.Join(textParts, ""),
				})
				textParts = nil
			}
			// Convert tool_use to function_call item
			// IMPORTANT: The ID must use "fc_" prefix for Responses API compatibility
			inputJSON, _ := json.Marshal(block.Input)
			translatedID := translateToolCallID(block.ID)
			inputs = append(inputs, models.ResponsesInput{
				Type:      "function_call",
				ID:        translatedID,
				CallID:    translatedID,
				Name:      block.Name,
				Arguments: string(inputJSON),
			})
		}
	}

	// Emit remaining text
	if len(textParts) > 0 {
		inputs = append(inputs, models.ResponsesInput{
			Type:    "message",
			Role:    "assistant",
			Content: strings.Join(textParts, ""),
		})
	}

	return inputs
}

// transformToolsToResponses converts Anthropic tools to Responses API format.
// The Responses API uses a flattened tool structure where name, description, and parameters
// are at the top level (not nested in a "function" object like Chat Completions API).
//
// IMPORTANT: We set Strict=false because Anthropic's tool schemas mark ALL parameters as
// required, but Claude Code only provides values for truly required parameters. With strict
// mode enabled, OpenAI rejects tool calls when optional parameters are missing.
func transformToolsToResponses(tools []models.AnthropicTool) []models.ResponsesTool {
	result := make([]models.ResponsesTool, len(tools))
	strictFalse := false // Pointer to false for explicit strict:false

	for i, tool := range tools {
		// Clean up input schema and fix required array
		params := cleanupSchemaForResponses(tool.InputSchema)

		// Responses API uses a flattened structure with name at top level
		result[i] = models.ResponsesTool{
			Type:        "function",
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  params,
			Strict:      &strictFalse, // CRITICAL: Must set strict:false at top level for Responses API
			// Also set Function for backwards compatibility, but Responses API
			// primarily uses the top-level fields
			Function: &models.ResponsesFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
				Strict:      false, // CRITICAL: Don't use strict mode - Anthropic marks all params as required
			},
		}
	}

	return result
}

// cleanupSchemaForResponses prepares an Anthropic tool schema for the Responses API.
// This includes removing unsupported format types and fixing the required array
// to only include truly required parameters.
func cleanupSchemaForResponses(schema interface{}) interface{} {
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

	// Clean up the schema
	cleanupSchemaMapForResponses(schemaMap)
	return schemaMap
}

// cleanupSchemaMapForResponses recursively cleans up schema properties for Responses API.
// Key fix: Only include truly required parameters (those without defaults and not nullable).
// Also adds "additionalProperties": false which is REQUIRED by OpenAI Responses API.
func cleanupSchemaMapForResponses(schema map[string]interface{}) {
	// Remove unsupported format types
	if format, ok := schema["format"].(string); ok {
		if format == "uri" {
			delete(schema, "format")
		}
	}

	// CRITICAL: Remove any "strict" field from the schema itself
	// Claude Code may include "strict": true in tool definitions, which causes
	// validation failures when optional parameters are missing.
	delete(schema, "strict")

	// CRITICAL: Add additionalProperties: false at top level of object schemas
	// OpenAI Responses API REQUIRES this even when strict: false is set.
	// Without this, you get: "Invalid schema for function '...': 'additionalProperties'
	// is required to be supplied and to be false"
	if schemaType, ok := schema["type"].(string); ok && schemaType == "object" {
		schema["additionalProperties"] = false
	}

	// Process properties and fix required array
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		// Identify truly required parameters
		trulyRequired := identifyTrulyRequired(props, schema)
		if len(trulyRequired) > 0 {
			schema["required"] = trulyRequired
		} else {
			// If no truly required params, remove required array entirely
			delete(schema, "required")
		}

		// Recurse into properties
		for _, v := range props {
			if propMap, ok := v.(map[string]interface{}); ok {
				cleanupSchemaMapForResponses(propMap)
			}
		}
	}

	// Recurse into items (for arrays)
	if items, ok := schema["items"].(map[string]interface{}); ok {
		cleanupSchemaMapForResponses(items)
	}
}

// identifyTrulyRequired determines which parameters are truly required.
// A parameter is truly required if:
// 1. It appears in the original required array
// 2. It doesn't have a default value
// 3. It's not nullable
// 4. It doesn't have a description containing optional-indicating phrases
// 5. It's not a boolean type (booleans are almost always optional flags)
func identifyTrulyRequired(props map[string]interface{}, schema map[string]interface{}) []string {
	var trulyRequired []string

	// Get original required array
	originalRequired := make(map[string]bool)
	if req, ok := schema["required"].([]interface{}); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				originalRequired[s] = true
			}
		}
	}

	for propName, propVal := range props {
		propMap, ok := propVal.(map[string]interface{})
		if !ok {
			continue
		}

		// Skip if not in original required array
		if !originalRequired[propName] {
			continue
		}

		// Skip if it has a default value
		if _, hasDefault := propMap["default"]; hasDefault {
			continue
		}

		// Skip if nullable
		if nullable, ok := propMap["nullable"].(bool); ok && nullable {
			continue
		}

		// Skip boolean types - they're almost always optional flags
		if propType, ok := propMap["type"].(string); ok && propType == "boolean" {
			continue
		}

		// Skip if description indicates it's optional
		if desc, ok := propMap["description"].(string); ok {
			descLower := strings.ToLower(desc)
			if strings.Contains(descLower, "optional") ||
				strings.Contains(descLower, "(optional)") ||
				strings.Contains(descLower, "if not specified") ||
				strings.Contains(descLower, "defaults to") ||
				strings.Contains(descLower, "set to true to") ||
				strings.Contains(descLower, "set to false to") ||
				strings.Contains(descLower, "if provided") ||
				strings.Contains(descLower, "when provided") ||
				strings.Contains(descLower, "can be omitted") ||
				strings.Contains(descLower, "not required") {
				continue
			}
		}

		trulyRequired = append(trulyRequired, propName)
	}

	return trulyRequired
}

// transformToolChoiceToResponses converts Anthropic tool_choice to Responses API format.
func transformToolChoiceToResponses(choice interface{}) interface{} {
	if choice == nil {
		return nil
	}

	// Handle map format
	if choiceMap, ok := choice.(map[string]interface{}); ok {
		if typeVal, ok := choiceMap["type"].(string); ok {
			switch typeVal {
			case "none":
				return "none"
			case "any":
				return "required"
			case "auto":
				return "auto"
			case "tool":
				if name, ok := choiceMap["name"].(string); ok {
					return map[string]interface{}{
						"type": "function",
						"function": map[string]interface{}{
							"name": name,
						},
					}
				}
			}
		}
	}

	return choice
}

// applyThinkingParametersToResponses maps thinking parameters for Responses API.
// The Responses API requires reasoning parameters under the nested "reasoning" object.
func applyThinkingParametersToResponses(req *models.AnthropicRequest, responsesReq *models.ResponsesRequest, targetModel string) {
	if req.Thinking == nil || req.Thinking.BudgetTokens <= 0 {
		return
	}

	budgetTokens := req.Thinking.BudgetTokens

	// For GPT-5+ models via Responses API, use nested reasoning.effort
	// The Responses API moved this from top-level "reasoning_effort" to "reasoning.effort"
	responsesReq.Reasoning = &models.ResponsesReasoning{
		Effort: mapBudgetToReasoningEffortResponses(budgetTokens),
	}
}

// mapBudgetToReasoningEffortResponses converts budget_tokens to reasoning_effort for Responses API.
func mapBudgetToReasoningEffortResponses(budgetTokens int) string {
	switch {
	case budgetTokens < 4000:
		return "low"
	case budgetTokens < 16000:
		return "medium"
	default:
		return "high"
	}
}

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

// extractToolResultsForResponses extracts tool results and converts to function_call_output items.
func extractToolResultsForResponses(content []models.ContentBlock) []models.ResponsesInput {
	var results []models.ResponsesInput

	for _, block := range content {
		if block.Type == "tool_result" {
			// In Responses API, tool results are represented as function_call_output items
			results = append(results, models.ResponsesInput{
				Type:   "function_call_output",
				CallID: block.ToolUseID,
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
			inputJSON, _ := json.Marshal(block.Input)
			inputs = append(inputs, models.ResponsesInput{
				Type:      "function_call",
				ID:        block.ID,
				CallID:    block.ID,
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
func transformToolsToResponses(tools []models.AnthropicTool) []models.ResponsesTool {
	result := make([]models.ResponsesTool, len(tools))

	for i, tool := range tools {
		// Clean up input schema
		params := cleanupSchema(tool.InputSchema)

		// Responses API uses a flattened structure with name at top level
		result[i] = models.ResponsesTool{
			Type:        "function",
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  params,
			// Also set Function for backwards compatibility, but Responses API
			// primarily uses the top-level fields
			Function: &models.ResponsesFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
			},
		}
	}

	return result
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
func applyThinkingParametersToResponses(req *models.AnthropicRequest, responsesReq *models.ResponsesRequest, targetModel string) {
	if req.Thinking == nil || req.Thinking.BudgetTokens <= 0 {
		return
	}

	budgetTokens := req.Thinking.BudgetTokens

	// For GPT-5+ models via Responses API, use reasoning_effort
	responsesReq.ReasoningEffort = mapBudgetToReasoningEffortResponses(budgetTokens)
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

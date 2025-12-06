// Package translator handles protocol translation between Anthropic and OpenAI formats.
package translator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jedarden/clasp/pkg/models"
)

// Model max_tokens limits for OpenAI models.
// These limits represent the maximum output tokens each model supports.
var modelMaxTokenLimits = map[string]int{
	// GPT-4o series
	"gpt-4o":            16384,
	"gpt-4o-2024-11-20": 16384,
	"gpt-4o-2024-08-06": 16384,
	"gpt-4o-2024-05-13": 4096,
	"gpt-4o-mini":       16384,
	"gpt-4o-mini-2024-07-18": 16384,
	// GPT-4 Turbo
	"gpt-4-turbo":            4096,
	"gpt-4-turbo-2024-04-09": 4096,
	"gpt-4-turbo-preview":    4096,
	"gpt-4-0125-preview":     4096,
	"gpt-4-1106-preview":     4096,
	// GPT-4
	"gpt-4":       8192,
	"gpt-4-32k":   8192,
	"gpt-4-0613":  8192,
	"gpt-4-32k-0613": 8192,
	// GPT-3.5 Turbo
	"gpt-3.5-turbo":             4096,
	"gpt-3.5-turbo-0125":        4096,
	"gpt-3.5-turbo-1106":        4096,
	"gpt-3.5-turbo-16k":         4096,
	// O1 models (reasoning models)
	"o1":         100000,
	"o1-preview": 32768,
	"o1-mini":    65536,
}

// defaultMaxTokenLimit is used when the model is not in the known list.
const defaultMaxTokenLimit = 4096

// capMaxTokens ensures max_tokens doesn't exceed the target model's limit.
func capMaxTokens(maxTokens int, targetModel string) int {
	if maxTokens <= 0 {
		return maxTokens
	}

	// Look up model limit
	limit, ok := modelMaxTokenLimits[targetModel]
	if !ok {
		// Try prefix matching for model variants
		for modelPrefix, modelLimit := range modelMaxTokenLimits {
			if strings.HasPrefix(targetModel, modelPrefix) {
				limit = modelLimit
				ok = true
				break
			}
		}
	}

	// If still not found, use default
	if !ok {
		limit = defaultMaxTokenLimit
	}

	// Cap to model limit
	if maxTokens > limit {
		return limit
	}
	return maxTokens
}

// TransformRequest converts an Anthropic request to OpenAI format.
func TransformRequest(req *models.AnthropicRequest, targetModel string) (*models.OpenAIRequest, error) {
	openAIReq := &models.OpenAIRequest{
		Model:       targetModel,
		Stream:      req.Stream,
		MaxTokens:   capMaxTokens(req.MaxTokens, targetModel),
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}

	// Transform stop sequences
	if len(req.StopSequences) > 0 {
		openAIReq.Stop = req.StopSequences
	}

	// Build messages
	messages, err := transformMessages(req)
	if err != nil {
		return nil, fmt.Errorf("transforming messages: %w", err)
	}
	openAIReq.Messages = messages

	// Transform tools
	if len(req.Tools) > 0 {
		openAIReq.Tools = transformTools(req.Tools)
	}

	// Transform tool choice
	if req.ToolChoice != nil {
		openAIReq.ToolChoice = transformToolChoice(req.ToolChoice)
	}

	// Enable usage tracking for streaming
	if req.Stream {
		openAIReq.StreamOptions = &models.StreamOptions{
			IncludeUsage: true,
		}
	}

	// Transform thinking/reasoning parameters based on target model
	applyThinkingParameters(req, openAIReq, targetModel)

	return openAIReq, nil
}

// applyThinkingParameters maps Anthropic thinking.budget_tokens to model-specific parameters.
// This enables extended reasoning capabilities across different model providers.
func applyThinkingParameters(req *models.AnthropicRequest, openAIReq *models.OpenAIRequest, targetModel string) {
	if req.Thinking == nil || req.Thinking.BudgetTokens <= 0 {
		return
	}

	budgetTokens := req.Thinking.BudgetTokens

	// Detect model family and apply appropriate reasoning parameters
	switch {
	case isO1OrO3Model(targetModel):
		// OpenAI O1/O3 models use reasoning_effort
		openAIReq.ReasoningEffort = mapBudgetToReasoningEffort(budgetTokens)
		// For O1/O3, we can also use max_completion_tokens instead of max_tokens
		if openAIReq.MaxTokens > 0 {
			openAIReq.MaxCompletionTokens = openAIReq.MaxTokens
			openAIReq.MaxTokens = 0 // Clear max_tokens as O1/O3 prefer max_completion_tokens
		}

	case isGrokModel(targetModel):
		// Grok 3 Mini supports reasoning_effort (low/high only)
		if budgetTokens >= 20000 {
			openAIReq.ReasoningEffort = "high"
		} else {
			openAIReq.ReasoningEffort = "low"
		}

	case isGemini3Model(targetModel):
		// Gemini 3 uses thinking_level (low/high)
		if budgetTokens >= 16000 {
			openAIReq.ThinkingLevel = "high"
		} else {
			openAIReq.ThinkingLevel = "low"
		}

	case isGemini25Model(targetModel):
		// Gemini 2.5 uses thinking_config with budget cap at 24k
		budget := budgetTokens
		if budget > 24576 {
			budget = 24576
		}
		openAIReq.ThinkingConfig = &models.OpenRouterThinkingConfig{
			ThinkingBudget: budget,
		}

	case isQwenModel(targetModel):
		// Qwen uses enable_thinking and thinking_budget
		enabled := true
		openAIReq.EnableThinking = &enabled
		openAIReq.ThinkingBudget = budgetTokens

	case isMiniMaxModel(targetModel):
		// MiniMax uses reasoning_split
		enabled := true
		openAIReq.ReasoningSplit = &enabled

	case isDeepSeekModel(targetModel):
		// DeepSeek R1 doesn't support reasoning params via API
		// Just strip the thinking parameter (no-op)
	}
}

// mapBudgetToReasoningEffort converts budget_tokens to OpenAI reasoning_effort.
func mapBudgetToReasoningEffort(budgetTokens int) string {
	switch {
	case budgetTokens < 4000:
		return "minimal"
	case budgetTokens < 16000:
		return "low"
	case budgetTokens < 32000:
		return "medium"
	default:
		return "high"
	}
}

// isO1OrO3Model checks if the model is an OpenAI O1 or O3 reasoning model.
func isO1OrO3Model(model string) bool {
	m := strings.ToLower(model)
	return strings.HasPrefix(m, "o1") || strings.HasPrefix(m, "o3") ||
		strings.Contains(m, "openai/o1") || strings.Contains(m, "openai/o3")
}

// isGrokModel checks if the model is a Grok model (x-ai).
func isGrokModel(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "grok") || strings.HasPrefix(m, "x-ai/")
}

// isGemini3Model checks if the model is a Gemini 3 model.
func isGemini3Model(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "gemini-3") || strings.Contains(m, "gemini/3")
}

// isGemini25Model checks if the model is a Gemini 2.5 model.
func isGemini25Model(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "gemini-2.5") || strings.Contains(m, "gemini-2-5") ||
		strings.Contains(m, "gemini/2.5")
}

// isQwenModel checks if the model is a Qwen model.
func isQwenModel(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "qwen") || strings.HasPrefix(m, "qwen/")
}

// isMiniMaxModel checks if the model is a MiniMax model.
func isMiniMaxModel(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "minimax")
}

// isDeepSeekModel checks if the model is a DeepSeek model.
func isDeepSeekModel(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "deepseek") || strings.HasPrefix(m, "deepseek/")
}

// transformMessages converts Anthropic messages to OpenAI format.
func transformMessages(req *models.AnthropicRequest) ([]models.OpenAIMessage, error) {
	var messages []models.OpenAIMessage

	// Handle system message
	if req.System != nil {
		systemContent, err := extractSystemContent(req.System)
		if err != nil {
			return nil, fmt.Errorf("extracting system content: %w", err)
		}
		if systemContent != "" {
			messages = append(messages, models.OpenAIMessage{
				Role:    "system",
				Content: systemContent,
			})
		}
	}

	// Transform each message
	for _, msg := range req.Messages {
		openAIMsg, err := transformMessage(msg)
		if err != nil {
			return nil, fmt.Errorf("transforming message: %w", err)
		}
		messages = append(messages, openAIMsg...)
	}

	return messages, nil
}

// extractSystemContent extracts the system message content.
func extractSystemContent(system interface{}) (string, error) {
	switch s := system.(type) {
	case string:
		return s, nil
	case []interface{}:
		var parts []string
		for _, item := range s {
			if block, ok := item.(map[string]interface{}); ok {
				if text, ok := block["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n\n"), nil
	default:
		// Try JSON marshaling and unmarshaling
		data, err := json.Marshal(system)
		if err != nil {
			return "", err
		}
		var str string
		if err := json.Unmarshal(data, &str); err == nil {
			return str, nil
		}
		return string(data), nil
	}
}

// transformMessage converts a single Anthropic message to OpenAI format.
// May return multiple messages (e.g., for tool results).
func transformMessage(msg models.AnthropicMessage) ([]models.OpenAIMessage, error) {
	content, err := parseContent(msg.Content)
	if err != nil {
		return nil, err
	}

	var result []models.OpenAIMessage

	switch msg.Role {
	case "user":
		result = append(result, transformUserMessage(content))
		// Handle tool results within user message
		toolResults := extractToolResults(content)
		for _, tr := range toolResults {
			result = append(result, tr)
		}
	case "assistant":
		assistantMsg := transformAssistantMessage(content)
		result = append(result, assistantMsg)
	default:
		// Pass through other roles
		result = append(result, models.OpenAIMessage{
			Role:    msg.Role,
			Content: getTextContent(content),
		})
	}

	return result, nil
}

// parseContent parses message content which can be string or []ContentBlock.
func parseContent(content interface{}) ([]models.ContentBlock, error) {
	switch c := content.(type) {
	case string:
		return []models.ContentBlock{{Type: "text", Text: c}}, nil
	case []interface{}:
		var blocks []models.ContentBlock
		for _, item := range c {
			block, err := parseContentBlock(item)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
		}
		return blocks, nil
	default:
		// Try JSON marshaling
		data, err := json.Marshal(content)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal content: %w", err)
		}

		// Try as string
		var str string
		if err := json.Unmarshal(data, &str); err == nil {
			return []models.ContentBlock{{Type: "text", Text: str}}, nil
		}

		// Try as array
		var arr []interface{}
		if err := json.Unmarshal(data, &arr); err == nil {
			return parseContent(arr)
		}

		return nil, fmt.Errorf("unsupported content type: %T", content)
	}
}

// parseContentBlock parses a single content block.
func parseContentBlock(item interface{}) (models.ContentBlock, error) {
	data, err := json.Marshal(item)
	if err != nil {
		return models.ContentBlock{}, err
	}

	var block models.ContentBlock
	if err := json.Unmarshal(data, &block); err != nil {
		return models.ContentBlock{}, err
	}

	return block, nil
}

// transformUserMessage transforms user message content to OpenAI format.
func transformUserMessage(content []models.ContentBlock) models.OpenAIMessage {
	var parts []models.OpenAIContentPart

	for _, block := range content {
		switch block.Type {
		case "text":
			parts = append(parts, models.OpenAIContentPart{
				Type: "text",
				Text: block.Text,
			})
		case "image":
			if block.Source != nil {
				dataURL := fmt.Sprintf("data:%s;base64,%s", block.Source.MediaType, block.Source.Data)
				parts = append(parts, models.OpenAIContentPart{
					Type: "image_url",
					ImageURL: &models.ImageURL{
						URL: dataURL,
					},
				})
			}
		}
		// Skip tool_result blocks - handled separately
	}

	// If only text content, use string format
	if len(parts) == 1 && parts[0].Type == "text" {
		return models.OpenAIMessage{
			Role:    "user",
			Content: parts[0].Text,
		}
	}

	// Otherwise use array format
	if len(parts) == 0 {
		return models.OpenAIMessage{
			Role:    "user",
			Content: "",
		}
	}

	return models.OpenAIMessage{
		Role:    "user",
		Content: contentPartsToInterface(parts),
	}
}

// contentPartsToInterface converts content parts to interface for JSON marshaling.
func contentPartsToInterface(parts []models.OpenAIContentPart) interface{} {
	result := make([]interface{}, len(parts))
	for i, p := range parts {
		result[i] = p
	}
	return result
}

// extractToolResults extracts tool result blocks and converts to OpenAI tool messages.
func extractToolResults(content []models.ContentBlock) []models.OpenAIMessage {
	var results []models.OpenAIMessage

	for _, block := range content {
		if block.Type == "tool_result" {
			results = append(results, models.OpenAIMessage{
				Role:       "tool",
				Content:    block.Content,
				ToolCallID: block.ToolUseID,
			})
		}
	}

	return results
}

// transformAssistantMessage transforms assistant message content to OpenAI format.
func transformAssistantMessage(content []models.ContentBlock) models.OpenAIMessage {
	msg := models.OpenAIMessage{
		Role: "assistant",
	}

	var textParts []string
	var toolCalls []models.OpenAIToolCall

	for i, block := range content {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "tool_use":
			inputJSON, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, models.OpenAIToolCall{
				ID:    block.ID,
				Type:  "function",
				Index: i,
				Function: models.OpenAIFunctionCall{
					Name:      block.Name,
					Arguments: string(inputJSON),
				},
			})
		}
	}

	if len(textParts) > 0 {
		msg.Content = strings.Join(textParts, "")
	}

	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}

	return msg
}

// getTextContent extracts text content from content blocks.
func getTextContent(content []models.ContentBlock) string {
	var parts []string
	for _, block := range content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "")
}

// transformTools converts Anthropic tools to OpenAI format.
func transformTools(tools []models.AnthropicTool) []models.OpenAITool {
	result := make([]models.OpenAITool, len(tools))

	for i, tool := range tools {
		// Clean up input schema (remove format: "uri" which OpenAI doesn't support)
		params := cleanupSchema(tool.InputSchema)

		result[i] = models.OpenAITool{
			Type: "function",
			Function: models.OpenAIFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
			},
		}
	}

	return result
}

// cleanupSchema removes unsupported schema properties.
func cleanupSchema(schema interface{}) interface{} {
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

	cleanupSchemaMap(schemaMap)
	return schemaMap
}

// cleanupSchemaMap recursively cleans up schema properties.
func cleanupSchemaMap(schema map[string]interface{}) {
	// Remove unsupported format types
	if format, ok := schema["format"].(string); ok {
		if format == "uri" {
			delete(schema, "format")
		}
	}

	// Recurse into properties
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		for _, v := range props {
			if propMap, ok := v.(map[string]interface{}); ok {
				cleanupSchemaMap(propMap)
			}
		}
	}

	// Recurse into items (for arrays)
	if items, ok := schema["items"].(map[string]interface{}); ok {
		cleanupSchemaMap(items)
	}
}

// transformToolChoice converts Anthropic tool_choice to OpenAI format.
func transformToolChoice(choice interface{}) interface{} {
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

	// Pass through if already in OpenAI format
	return choice
}

// Package translator handles protocol translation between Anthropic and OpenAI formats.
package translator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jedarden/clasp/pkg/models"
)

// TransformRequest converts an Anthropic request to OpenAI format.
func TransformRequest(req *models.AnthropicRequest, targetModel string) (*models.OpenAIRequest, error) {
	openAIReq := &models.OpenAIRequest{
		Model:       targetModel,
		Stream:      req.Stream,
		MaxTokens:   req.MaxTokens,
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

	return openAIReq, nil
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

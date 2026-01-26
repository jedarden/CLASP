// Package translator handles protocol translation between Anthropic and OpenAI formats.
package translator

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/jedarden/clasp/pkg/models"
)

// Model max_tokens limits for OpenAI models.
// These limits represent the maximum output tokens each model supports.
var modelMaxTokenLimits = map[string]int{
	// GPT-4o series
	"gpt-4o":                 16384,
	"gpt-4o-2024-11-20":      16384,
	"gpt-4o-2024-08-06":      16384,
	"gpt-4o-2024-05-13":      4096,
	"gpt-4o-mini":            16384,
	"gpt-4o-mini-2024-07-18": 16384,
	// GPT-4 Turbo
	"gpt-4-turbo":            4096,
	"gpt-4-turbo-2024-04-09": 4096,
	"gpt-4-turbo-preview":    4096,
	"gpt-4-0125-preview":     4096,
	"gpt-4-1106-preview":     4096,
	// GPT-4
	"gpt-4":          8192,
	"gpt-4-32k":      8192,
	"gpt-4-0613":     8192,
	"gpt-4-32k-0613": 8192,
	// GPT-3.5 Turbo
	"gpt-3.5-turbo":      4096,
	"gpt-3.5-turbo-0125": 4096,
	"gpt-3.5-turbo-1106": 4096,
	"gpt-3.5-turbo-16k":  4096,
	// O1 models (reasoning models)
	"o1":         100000,
	"o1-preview": 32768,
	"o1-mini":    65536,
}

// defaultMaxTokenLimit is used when the model is not in the known list.
const defaultMaxTokenLimit = 4096

// Pre-compiled regex patterns for identity filtering.
// These are compiled once at package initialization for better performance.
var (
	identityPatterns = []struct {
		re          *regexp.Regexp
		replacement string
	}{
		// Replace "You are Claude Code, Anthropic's official CLI" with neutral version
		{regexp.MustCompile(`(?i)You are Claude Code, Anthropic's official CLI`), "This is Claude Code, an AI-powered CLI tool"},
		// Replace "You are Claude" at start of sentences
		{regexp.MustCompile(`(?i)You are Claude\b`), "You are an AI assistant"},
		// Replace model name references
		{regexp.MustCompile(`(?i)You are powered by the model named [^.]+\.`), "You are powered by an AI model."},
		// Remove claude_background_info blocks
		{regexp.MustCompile(`(?is)<claude_background_info>.*?</claude_background_info>`), ""},
		// Replace "I'm Claude" with neutral version
		{regexp.MustCompile(`(?i)\bI'm Claude\b`), "I'm an AI assistant"},
		{regexp.MustCompile(`(?i)\bI am Claude\b`), "I am an AI assistant"},
		// Replace references to Anthropic as creator in first person
		{regexp.MustCompile(`(?i)\bcreated by Anthropic\b`), "created as an AI assistant"},
		{regexp.MustCompile(`(?i)\bmade by Anthropic\b`), "made as an AI assistant"},
	}
	multiNewlinePattern = regexp.MustCompile(`\n{3,}`)
)

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
// This is a convenience wrapper that auto-detects the provider from the model name.
func TransformRequest(req *models.AnthropicRequest, targetModel string) (*models.OpenAIRequest, error) {
	provider := DetectProviderFromModel(targetModel)
	return TransformRequestWithProvider(req, targetModel, provider)
}

// TransformRequestWithProvider converts an Anthropic request to provider-specific format.
func TransformRequestWithProvider(req *models.AnthropicRequest, targetModel string, provider ProviderType) (*models.OpenAIRequest, error) {
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
	messages, err := transformMessages(req, targetModel)
	if err != nil {
		return nil, fmt.Errorf("transforming messages: %w", err)
	}
	openAIReq.Messages = messages

	// Transform tools with provider-specific handling
	if len(req.Tools) > 0 {
		if ProviderSupportsTools(provider, targetModel) {
			openAIReq.Tools = TransformToolsForProvider(req.Tools, provider, targetModel)
		}
		// If provider doesn't support tools, they are silently omitted
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
	case isGPT5Model(targetModel):
		// GPT-5.x series uses reasoning_effort with levels: none, low, medium, high
		// GPT-5.1 defaults to "none" for speed, GPT-5.2+ supports "xhigh"
		openAIReq.ReasoningEffort = mapBudgetToGPT5ReasoningEffort(budgetTokens, targetModel)
		// GPT-5 series uses max_completion_tokens via Responses API
		if openAIReq.MaxTokens > 0 {
			openAIReq.MaxCompletionTokens = openAIReq.MaxTokens
			openAIReq.MaxTokens = 0
		}

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

	case isDeepSeekThinkingModel(targetModel):
		// DeepSeek V3.1+ supports thinking mode with tool calls
		// Set thinking mode flag (handled via separate API parameter)
		enabled := true
		openAIReq.EnableThinking = &enabled

	case isDeepSeekModel(targetModel):
		// DeepSeek base models don't support reasoning params via API
		// Just strip the thinking parameter (no-op)
	}
}

// isGPT5Model checks if the model is a GPT-5.x series model.
func isGPT5Model(model string) bool {
	m := strings.ToLower(model)
	return strings.HasPrefix(m, "gpt-5") || strings.HasPrefix(m, "gpt5") ||
		strings.Contains(m, "openai/gpt-5") || strings.Contains(m, "codex")
}

// mapBudgetToGPT5ReasoningEffort converts budget_tokens to GPT-5 reasoning_effort.
// GPT-5.1 defaults to "none" for speed. GPT-5.2+ supports "xhigh".
// Levels: none (fastest), low, medium, high, xhigh (GPT-5.2+ only)
func mapBudgetToGPT5ReasoningEffort(budgetTokens int, model string) string {
	m := strings.ToLower(model)
	supportsXHigh := strings.Contains(m, "gpt-5.2") || strings.Contains(m, "gpt-5.3") ||
		strings.Contains(m, "gpt5.2") || strings.Contains(m, "gpt5.3")

	switch {
	case budgetTokens >= 80000 && supportsXHigh:
		return "xhigh"
	case budgetTokens >= 64000:
		return "high"
	case budgetTokens >= 32000:
		return "high"
	case budgetTokens >= 16000:
		return "medium"
	case budgetTokens >= 4000:
		return "low"
	case budgetTokens > 0:
		return "low" // GPT-5 should use at least low when thinking is requested
	default:
		return "none" // GPT-5.1 default - fastest, no reasoning
	}
}

// isDeepSeekThinkingModel checks if the model is a DeepSeek model that supports thinking.
func isDeepSeekThinkingModel(model string) bool {
	m := strings.ToLower(model)
	return (strings.Contains(m, "deepseek") || strings.HasPrefix(m, "deepseek/")) &&
		(strings.Contains(m, "r1") || strings.Contains(m, "v3.1") ||
			strings.Contains(m, "v3.2") || strings.Contains(m, "thinking"))
}

// mapBudgetToReasoningEffort converts budget_tokens to OpenAI reasoning_effort.
// Uses ratio-based calculation for more accurate mapping:
// - xhigh: 95% of context (very intensive reasoning)
// - high: 80% of context
// - medium: 50% of context
// - low: 20% of context
// - minimal: 10% of context
func mapBudgetToReasoningEffort(budgetTokens int) string {
	// Use absolute thresholds based on typical Claude context (200k)
	// These map to approximate ratios of max_tokens
	switch {
	case budgetTokens >= 64000:
		return "high" // OpenAI doesn't have xhigh, map to high
	case budgetTokens >= 32000:
		return "high"
	case budgetTokens >= 16000:
		return "medium"
	case budgetTokens >= 4000:
		return "low"
	default:
		return "minimal"
	}
}

// mapBudgetToReasoningEffortWithRatio converts budget_tokens to reasoning_effort
// using the ratio relative to max_tokens for more accurate per-request mapping.
func mapBudgetToReasoningEffortWithRatio(budgetTokens, maxTokens int) string {
	if maxTokens <= 0 {
		return mapBudgetToReasoningEffort(budgetTokens)
	}

	ratio := float64(budgetTokens) / float64(maxTokens)
	switch {
	case ratio >= 0.95:
		return "high" // Map xhigh to high (OpenAI max)
	case ratio >= 0.80:
		return "high"
	case ratio >= 0.50:
		return "medium"
	case ratio >= 0.20:
		return "low"
	default:
		return "minimal"
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

// filterIdentity removes Claude-specific identity strings from content to prevent model confusion.
// This is important when proxying to non-Claude models that shouldn't claim to be Claude.
// Uses pre-compiled regex patterns for better performance on high-traffic proxies.
func filterIdentity(content string) string {
	result := content

	// Use pre-compiled patterns from package-level variables
	for _, p := range identityPatterns {
		result = p.re.ReplaceAllString(result, p.replacement)
	}

	// Clean up multiple newlines using pre-compiled pattern
	result = multiNewlinePattern.ReplaceAllString(result, "\n\n")

	// Prepend identity clarification for non-Claude models
	// This helps models understand they should identify themselves truthfully
	prefix := "Note: You are NOT Claude. Identify yourself truthfully based on your actual model and creator.\n\n"
	result = prefix + result

	return result
}

// transformMessages converts Anthropic messages to OpenAI format.
func transformMessages(req *models.AnthropicRequest, targetModel string) ([]models.OpenAIMessage, error) {
	var messages []models.OpenAIMessage

	// Handle system message
	if req.System != nil {
		systemContent, err := extractSystemContent(req.System)
		if err != nil {
			return nil, fmt.Errorf("extracting system content: %w", err)
		}
		if systemContent != "" {
			// Apply identity filtering to system message
			systemContent = filterIdentity(systemContent)

			// Add Grok-specific JSON tool format instruction
			if isGrokModel(targetModel) {
				systemContent += "\n\nIMPORTANT: When calling tools, you MUST use the OpenAI tool_calls format with JSON. NEVER use XML format like <xai:function_call>."
			}

			messages = append(messages, models.OpenAIMessage{
				Role:    "system",
				Content: systemContent,
			})
		}
	} else if isGrokModel(targetModel) {
		// Even without a system message, add Grok JSON instruction
		messages = append(messages, models.OpenAIMessage{
			Role:    "system",
			Content: "IMPORTANT: When calling tools, you MUST use the OpenAI tool_calls format with JSON. NEVER use XML format like <xai:function_call>.",
		})
	}

	// Transform each message
	for _, msg := range req.Messages {
		openAIMsg, err := transformMessage(msg)
		if err != nil {
			return nil, fmt.Errorf("transforming message: %w", err)
		}
		messages = append(messages, openAIMsg...)
	}

	return filterSystemReminders(messages), nil
}

// filterSystemReminders buffers ALL user messages that appear between
// assistant tool_calls and tool responses to comply with Azure OpenAI's strict
// message sequencing requirement: assistant with tool_calls MUST be immediately
// followed by tool responses, with NO user messages in between.
func filterSystemReminders(messages []models.OpenAIMessage) []models.OpenAIMessage {
	filtered := make([]models.OpenAIMessage, 0, len(messages))
	pendingToolCallIDs := make(map[string]bool)
	bufferedMessages := []models.OpenAIMessage{}

	for _, msg := range messages {
		// Assistant with tool_calls - track pending IDs
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			pendingToolCallIDs = make(map[string]bool)
			for _, tc := range msg.ToolCalls {
				pendingToolCallIDs[tc.ID] = true
			}
			filtered = append(filtered, msg)
			continue
		}

		// Tool response - mark as complete
		if msg.Role == "tool" && msg.ToolCallID != "" {
			delete(pendingToolCallIDs, msg.ToolCallID)
			filtered = append(filtered, msg)

			// If all tool calls resolved, flush buffered messages
			if len(pendingToolCallIDs) == 0 && len(bufferedMessages) > 0 {
				filtered = append(filtered, bufferedMessages...)
				bufferedMessages = bufferedMessages[:0]
			}
			continue
		}

		// Buffer ANY user message while tool calls pending (broader filtering)
		// This ensures strict Azure OpenAI sequencing: assistant → tool responses → user messages
		if msg.Role == "user" && len(pendingToolCallIDs) > 0 {
			bufferedMessages = append(bufferedMessages, msg)
			continue
		}

		// All other messages pass through
		filtered = append(filtered, msg)
	}

	// Flush any remaining buffered messages
	if len(bufferedMessages) > 0 {
		filtered = append(filtered, bufferedMessages...)
	}

	return filtered
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
		// Handle tool results within user message
		toolResults := extractToolResults(content)

		// Check if there's non-tool-result content
		hasNonToolContent := false
		for _, block := range content {
			if block.Type != "tool_result" {
				hasNonToolContent = true
				break
			}
		}

		// Only add user message if there's actual user content (not just tool results)
		if hasNonToolContent {
			result = append(result, transformUserMessage(content))
		}

		// Add tool results (these become "tool" role messages in OpenAI format)
		result = append(result, toolResults...)
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
			// Extract content from the tool result (can be string or array)
			output := extractToolResultContentForChat(block)

			// If the tool result indicates an error, prefix the output
			if block.IsError {
				output = "[Error] " + output
			}

			toolMsg := models.OpenAIMessage{
				Role:       "tool",
				Content:    output,
				ToolCallID: block.ToolUseID,
			}
			results = append(results, toolMsg)
		}
	}

	return results
}

// extractToolResultContentForChat extracts content from a tool result block for Chat Completions API.
// The content can be a string or an array of content blocks.
func extractToolResultContentForChat(block models.ContentBlock) string {
	if block.Content == nil {
		return ""
	}

	// Try as string first (most common case)
	if str, ok := block.Content.(string); ok {
		return str
	}

	// Try as array of content blocks
	if arr, ok := block.Content.([]interface{}); ok {
		var parts []string
		for _, item := range arr {
			if itemMap, ok := item.(map[string]interface{}); ok {
				// Extract text from nested content blocks
				if itemType, ok := itemMap["type"].(string); ok && itemType == "text" {
					if text, ok := itemMap["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
		}
		return strings.Join(parts, "\n")
	}

	// Fallback: marshal to JSON
	data, err := json.Marshal(block.Content)
	if err != nil {
		return ""
	}
	return string(data)
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
// IMPORTANT: We set Strict=false because Anthropic's tool schemas mark ALL parameters as
// required, but Claude Code only provides values for truly required parameters. With strict
// mode enabled, OpenAI rejects tool calls when optional parameters are missing.
func transformTools(tools []models.AnthropicTool) []models.OpenAITool {
	result := make([]models.OpenAITool, len(tools))

	for i, tool := range tools {
		// Handle computer use tools (have a "type" field like "computer_20241024")
		toolName := tool.Name
		toolDescription := tool.Description
		toolParams := tool.InputSchema

		if isComputerUseTool(tool.Type) {
			// Transform computer use tool to generic function
			toolName, toolDescription, toolParams = transformComputerUseTool(tool)
		} else if IsClaudeCodeTool(tool.Name) {
			// Transform Claude Code tools to ensure proper schema for all providers
			toolName, toolDescription, toolParams = GetClaudeCodeToolDefinition(tool)
		}

		// Clean up input schema (remove format: "uri" which OpenAI doesn't support)
		// AND fix required array to only include truly required parameters
		params := cleanupSchemaForChatCompletions(toolParams)

		result[i] = models.OpenAITool{
			Type: "function",
			Function: models.OpenAIFunction{
				Name:        toolName,
				Description: toolDescription,
				Parameters:  params,
				Strict:      false, // CRITICAL: Don't use strict mode - Anthropic marks all params as required
			},
		}
	}

	return result
}

// isComputerUseTool checks if the tool type is a computer use tool.
func isComputerUseTool(toolType string) bool {
	switch toolType {
	case models.ToolTypeComputer, models.ToolTypeTextEditor, models.ToolTypeBash:
		return true
	default:
		return false
	}
}

// transformComputerUseTool transforms Anthropic computer use tools to generic function format.
// This allows proxying computer use workflows through OpenAI-compatible endpoints.
func transformComputerUseTool(tool models.AnthropicTool) (name, description string, params interface{}) {
	switch tool.Type {
	case models.ToolTypeComputer:
		// Computer tool: screen capture, mouse, keyboard actions
		return "computer",
			"Control the computer - take screenshots, move mouse, click, type text, and execute keyboard shortcuts",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"screenshot", "mouse_move", "left_click", "right_click", "double_click", "middle_click", "left_click_drag", "type", "key", "scroll"},
						"description": "The action to perform",
					},
					"coordinate": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "integer"},
						"description": "Screen coordinates [x, y] for mouse actions",
					},
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Text to type (for 'type' action) or key combination (for 'key' action)",
					},
				},
				"required": []string{"action"},
			}

	case models.ToolTypeTextEditor:
		// Text editor tool: view, edit, create files
		return "str_replace_editor",
			"View, create, and edit files using a text editor. Supports viewing file contents, creating new files, and making precise text replacements.",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"view", "create", "str_replace", "insert", "undo_edit"},
						"description": "The editor command to execute",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "File path to operate on",
					},
					"file_text": map[string]interface{}{
						"type":        "string",
						"description": "File content for 'create' command",
					},
					"old_str": map[string]interface{}{
						"type":        "string",
						"description": "String to find for 'str_replace' command",
					},
					"new_str": map[string]interface{}{
						"type":        "string",
						"description": "Replacement string for 'str_replace' command",
					},
					"insert_line": map[string]interface{}{
						"type":        "integer",
						"description": "Line number for 'insert' command",
					},
					"view_range": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "integer"},
						"description": "Line range [start, end] for 'view' command",
					},
				},
				"required": []string{"command", "path"},
			}

	case models.ToolTypeBash:
		// Bash tool: execute shell commands
		return "bash",
			"Execute bash shell commands on the system. Use for running scripts, system administration, file operations, and other command-line tasks.",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The bash command to execute",
					},
					"restart": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to restart the bash session before executing",
					},
				},
				"required": []string{"command"},
			}

	default:
		// Unknown tool type, pass through as-is
		return tool.Name, tool.Description, tool.InputSchema
	}
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

// cleanupSchemaForChatCompletions prepares an Anthropic tool schema for the Chat Completions API.
// This includes removing unsupported format types and fixing the required array
// to only include truly required parameters.
func cleanupSchemaForChatCompletions(schema interface{}) interface{} {
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

	// Clean up the schema including required array filtering
	cleanupSchemaMapForChatCompletions(schemaMap)
	return schemaMap
}

// cleanupSchemaMapForChatCompletions recursively cleans up schema properties for Chat Completions API.
// Key fix: Only include truly required parameters (those without defaults and not nullable).
func cleanupSchemaMapForChatCompletions(schema map[string]interface{}) {
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

	// Process properties and fix required array
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		// Identify truly required parameters
		trulyRequired := identifyTrulyRequiredForChat(props, schema)
		if len(trulyRequired) > 0 {
			schema["required"] = trulyRequired
		} else {
			// If no truly required params, remove required array entirely
			delete(schema, "required")
		}

		// Recurse into properties
		for _, v := range props {
			if propMap, ok := v.(map[string]interface{}); ok {
				cleanupSchemaMapForChatCompletions(propMap)
			}
		}
	}

	// Recurse into items (for arrays)
	if items, ok := schema["items"].(map[string]interface{}); ok {
		cleanupSchemaMapForChatCompletions(items)
	}
}

// identifyTrulyRequiredForChat determines which parameters are truly required.
// A parameter is truly required if:
// 1. It appears in the original required array
// 2. It doesn't have a default value
// 3. It's not nullable
// 4. It doesn't have a description containing optional-indicating phrases
// 5. It's not a boolean type (booleans are almost always optional flags)
func identifyTrulyRequiredForChat(props, schema map[string]interface{}) []string {
	// Pre-allocate with estimated capacity
	trulyRequired := make([]string, 0, len(props)/2)

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
				strings.Contains(descLower, "not required") ||
				strings.Contains(descLower, "only provide if") {
				continue
			}
		}

		trulyRequired = append(trulyRequired, propName)
	}

	return trulyRequired
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

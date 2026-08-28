// Package translator handles protocol translation between Anthropic and OpenAI formats.
package translator

import (
	"encoding/json"

	"github.com/jedarden/clasp/pkg/models"
)

// TransformResponse translates a non-streaming OpenAI Chat Completions response to Anthropic format.
func TransformResponse(openaiResp []byte, targetModel string) (*models.AnthropicResponse, error) {
	// Parse OpenAI response
	var rawResp struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Role             string `json:"role"`
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content,omitempty"` // Kimi K2.5, DeepSeek-R1
				ToolCalls        []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(openaiResp, &rawResp); err != nil {
		return nil, err
	}

	// Build Anthropic response
	anthropicResp := &models.AnthropicResponse{
		ID:    rawResp.ID,
		Type:  "message",
		Role:  "assistant",
		Model: targetModel,
		Usage: &models.AnthropicUsage{
			InputTokens:  rawResp.Usage.PromptTokens,
			OutputTokens: rawResp.Usage.CompletionTokens,
		},
	}

	if len(rawResp.Choices) > 0 {
		choice := rawResp.Choices[0]
		anthropicResp.StopReason = mapFinishReason(choice.FinishReason)

		// Add reasoning content as a thinking block first (if present)
		// This handles Kimi K2.5, DeepSeek-R1, and similar models
		if choice.Message.ReasoningContent != "" {
			anthropicResp.Content = append(anthropicResp.Content, models.AnthropicContentBlock{
				Type:     "thinking",
				Thinking: choice.Message.ReasoningContent,
			})
		}

		// Add text content (if present)
		if choice.Message.Content != "" {
			anthropicResp.Content = append(anthropicResp.Content, models.AnthropicContentBlock{
				Type: "text",
				Text: choice.Message.Content,
			})
		}

		// Add tool calls
		for _, tc := range choice.Message.ToolCalls {
			var input interface{}
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)

			anthropicResp.Content = append(anthropicResp.Content, models.AnthropicContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			})
		}
	}

	return anthropicResp, nil
}

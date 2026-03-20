package session

import (
	"testing"

	"github.com/jedarden/clasp/pkg/models"
)

func TestGenerateSessionKey_NilRequest(t *testing.T) {
	key := GenerateSessionKey(nil)
	if key != "" {
		t.Errorf("expected empty key for nil request, got %q", key)
	}
}

func TestGenerateSessionKey_ModelOnly(t *testing.T) {
	req := &models.AnthropicRequest{
		Model: "claude-sonnet-4-20250514",
	}
	key := GenerateSessionKey(req)
	if key == "" {
		t.Error("expected non-empty key")
	}
	if len(key) != 16 {
		t.Errorf("expected key length 16, got %d", len(key))
	}
}

func TestGenerateSessionKey_WithSystemString(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:  "claude-sonnet-4-20250514",
		System: "You are a helpful assistant.",
	}
	key := GenerateSessionKey(req)
	if key == "" {
		t.Error("expected non-empty key")
	}
}

func TestGenerateSessionKey_WithSystemArray(t *testing.T) {
	req := &models.AnthropicRequest{
		Model: "claude-sonnet-4-20250514",
		System: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "System prompt part 1",
			},
			map[string]interface{}{
				"type": "text",
				"text": "System prompt part 2",
			},
		},
	}
	key := GenerateSessionKey(req)
	if key == "" {
		t.Error("expected non-empty key")
	}
}

func TestGenerateSessionKey_WithMessages(t *testing.T) {
	req := &models.AnthropicRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []models.AnthropicMessage{
			{
				Role: "user",
				Content: []models.ContentBlock{
					{Type: "text", Text: "Hello, how are you?"},
				},
			},
		},
	}
	key := GenerateSessionKey(req)
	if key == "" {
		t.Error("expected non-empty key")
	}
}

func TestGenerateSessionKey_Stability(t *testing.T) {
	// Same request should produce the same key
	req := &models.AnthropicRequest{
		Model:  "claude-sonnet-4-20250514",
		System: "You are a helpful assistant.",
		Messages: []models.AnthropicMessage{
			{
				Role: "user",
				Content: []models.ContentBlock{
					{Type: "text", Text: "Hello"},
				},
			},
		},
	}

	key1 := GenerateSessionKey(req)
	key2 := GenerateSessionKey(req)

	if key1 != key2 {
		t.Errorf("expected same key for same request, got %q and %q", key1, key2)
	}
}

func TestGenerateSessionKey_DifferentModels(t *testing.T) {
	req1 := &models.AnthropicRequest{
		Model: "claude-sonnet-4-20250514",
	}
	req2 := &models.AnthropicRequest{
		Model: "claude-opus-4-20250514",
	}

	key1 := GenerateSessionKey(req1)
	key2 := GenerateSessionKey(req2)

	if key1 == key2 {
		t.Errorf("expected different keys for different models, got %q for both", key1)
	}
}

func TestGenerateSessionKey_DifferentSystems(t *testing.T) {
	req1 := &models.AnthropicRequest{
		Model:  "claude-sonnet-4-20250514",
		System: "You are a helpful assistant.",
	}
	req2 := &models.AnthropicRequest{
		Model:  "claude-sonnet-4-20250514",
		System: "You are a coding assistant.",
	}

	key1 := GenerateSessionKey(req1)
	key2 := GenerateSessionKey(req2)

	if key1 == key2 {
		t.Errorf("expected different keys for different systems, got %q for both", key1)
	}
}

func TestGenerateSessionKey_DifferentFirstMessages(t *testing.T) {
	req1 := &models.AnthropicRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []models.AnthropicMessage{
			{
				Role: "user",
				Content: []models.ContentBlock{
					{Type: "text", Text: "What is Python?"},
				},
			},
		},
	}
	req2 := &models.AnthropicRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []models.AnthropicMessage{
			{
				Role: "user",
				Content: []models.ContentBlock{
					{Type: "text", Text: "What is Go?"},
				},
			},
		},
	}

	key1 := GenerateSessionKey(req1)
	key2 := GenerateSessionKey(req2)

	if key1 == key2 {
		t.Errorf("expected different keys for different first messages, got %q for both", key1)
	}
}

func TestGenerateSessionKey_SameSessionDifferentLength(t *testing.T) {
	// Two requests with same model/system/first-message but different conversation length
	// should have the same session key (for compaction to work)
	baseReq := &models.AnthropicRequest{
		Model:  "claude-sonnet-4-20250514",
		System: "You are a helpful assistant.",
		Messages: []models.AnthropicMessage{
			{
				Role: "user",
				Content: []models.ContentBlock{
					{Type: "text", Text: "What is Python?"},
				},
			},
		},
	}

	// Extended conversation
	extendedReq := &models.AnthropicRequest{
		Model:  "claude-sonnet-4-20250514",
		System: "You are a helpful assistant.",
		Messages: []models.AnthropicMessage{
			{
				Role: "user",
				Content: []models.ContentBlock{
					{Type: "text", Text: "What is Python?"},
				},
			},
			{
				Role: "assistant",
				Content: []models.ContentBlock{
					{Type: "text", Text: "Python is a programming language."},
				},
			},
			{
				Role: "user",
				Content: []models.ContentBlock{
					{Type: "text", Text: "Tell me more."},
				},
			},
		},
	}

	key1 := GenerateSessionKey(baseReq)
	key2 := GenerateSessionKey(extendedReq)

	if key1 != key2 {
		t.Errorf("expected same key for continuation, got %q and %q", key1, key2)
	}
}

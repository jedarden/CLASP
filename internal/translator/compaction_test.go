package translator

import (
	"testing"

	"github.com/jedarden/clasp/pkg/models"
)

func TestSessionKey(t *testing.T) {
	req := &models.AnthropicRequest{
		Model: "gpt-5.1-codex-max",
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
			{Role: "user", Content: "How are you?"},
		},
	}

	key1 := SessionKey(req)
	if key1 == "" {
		t.Fatal("expected non-empty session key")
	}

	// Same first message → same key.
	req2 := &models.AnthropicRequest{
		Model: "gpt-5.1-codex-max",
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}
	key2 := SessionKey(req2)
	if key1 != key2 {
		t.Errorf("expected same key for same first message, got %q vs %q", key1, key2)
	}

	// Different first message → different key.
	req3 := &models.AnthropicRequest{
		Model: "gpt-5.1-codex-max",
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Different start"},
		},
	}
	key3 := SessionKey(req3)
	if key1 == key3 {
		t.Error("expected different keys for different first messages")
	}
}

func TestSessionKeyNoUserMessage(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:    "gpt-5.1-codex-max",
		Messages: []models.AnthropicMessage{},
	}
	if key := SessionKey(req); key != "" {
		t.Errorf("expected empty key for no messages, got %q", key)
	}
}

func TestSessionKeyDifferentModels(t *testing.T) {
	msg := []models.AnthropicMessage{{Role: "user", Content: "Hello"}}

	k1 := SessionKey(&models.AnthropicRequest{Model: "gpt-5.1-codex-max", Messages: msg})
	k2 := SessionKey(&models.AnthropicRequest{Model: "gpt-4o", Messages: msg})

	if k1 == k2 {
		t.Error("expected different keys for different models")
	}
}

func TestExtractResponseID(t *testing.T) {
	body := []byte(`{"id":"resp_abc123","object":"response","status":"completed"}`)
	id := ExtractResponseID(body)
	if id != "resp_abc123" {
		t.Errorf("expected resp_abc123, got %q", id)
	}
}

func TestExtractResponseIDInvalid(t *testing.T) {
	if id := ExtractResponseID([]byte(`not json`)); id != "" {
		t.Errorf("expected empty string for invalid JSON, got %q", id)
	}
}

func TestTrimMessagesForCompaction(t *testing.T) {
	messages := []models.AnthropicMessage{
		{Role: "user", Content: "msg1"},
		{Role: "assistant", Content: "resp1"},
		{Role: "user", Content: "msg2"},
		{Role: "assistant", Content: "resp2"},
		{Role: "user", Content: "msg3"},
	}

	// Normal trim.
	trimmed := TrimMessagesForCompaction(messages, 2)
	if len(trimmed) != 3 {
		t.Errorf("expected 3 trimmed messages, got %d", len(trimmed))
	}
	if trimmed[0].Content != "msg2" {
		t.Errorf("expected msg2, got %v", trimmed[0].Content)
	}

	// Zero messageCount → no trim.
	if TrimMessagesForCompaction(messages, 0) != nil {
		t.Error("expected nil for zero messageCount")
	}

	// messageCount == len → no trim (nothing new).
	if TrimMessagesForCompaction(messages, len(messages)) != nil {
		t.Error("expected nil when messageCount equals length")
	}

	// messageCount > len → no trim.
	if TrimMessagesForCompaction(messages, 100) != nil {
		t.Error("expected nil when messageCount exceeds length")
	}
}

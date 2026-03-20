package cache

import (
	"testing"
	"time"

	"github.com/jedarden/clasp/pkg/models"
)

func TestPromptCache_BasicOperations(t *testing.T) {
	pc := NewPromptCache(10, time.Hour)

	resp := &models.AnthropicResponse{
		ID:         "msg_123",
		Type:       "message",
		Role:       "assistant",
		Model:      "gpt-4o",
		StopReason: "end_turn",
		Content: []models.AnthropicContentBlock{
			{Type: "text", Text: "Hello, world!"},
		},
	}

	// Set and Get
	pc.Set("prefix_key", resp, 100)

	got, tokens, found := pc.Get("prefix_key")
	if !found {
		t.Fatal("Expected to find cached response")
	}
	if got.ID != resp.ID {
		t.Errorf("Expected ID %s, got %s", resp.ID, got.ID)
	}
	if tokens != 100 {
		t.Errorf("Expected 100 tokens, got %d", tokens)
	}

	// Get non-existent key
	_, _, found = pc.Get("non_existent")
	if found {
		t.Error("Expected not to find non-existent key")
	}
}

func TestPromptCache_LRUEviction(t *testing.T) {
	pc := NewPromptCache(3, time.Hour)

	for i := 0; i < 3; i++ {
		pc.Set(string(rune('a'+i)), &models.AnthropicResponse{ID: string(rune('a' + i))}, 50)
	}

	// All 3 present
	for i := 0; i < 3; i++ {
		if _, _, found := pc.Get(string(rune('a' + i))); !found {
			t.Errorf("Expected to find key %c", rune('a'+i))
		}
	}

	// Add 4th entry - should evict "a"
	pc.Set("d", &models.AnthropicResponse{ID: "d"}, 50)

	if _, _, found := pc.Get("a"); found {
		t.Error("Expected 'a' to be evicted")
	}
	for _, key := range []string{"b", "c", "d"} {
		if _, _, found := pc.Get(key); !found {
			t.Errorf("Expected to find key %s", key)
		}
	}
}

func TestPromptCache_LRUAccess(t *testing.T) {
	pc := NewPromptCache(3, time.Hour)

	pc.Set("a", &models.AnthropicResponse{ID: "a"}, 50)
	pc.Set("b", &models.AnthropicResponse{ID: "b"}, 50)
	pc.Set("c", &models.AnthropicResponse{ID: "c"}, 50)

	// Access "a" to make it recently used
	pc.Get("a")

	// Add 4th - should evict "b" (oldest accessed)
	pc.Set("d", &models.AnthropicResponse{ID: "d"}, 50)

	if _, _, found := pc.Get("b"); found {
		t.Error("Expected 'b' to be evicted")
	}
	for _, key := range []string{"a", "c", "d"} {
		if _, _, found := pc.Get(key); !found {
			t.Errorf("Expected to find key %s", key)
		}
	}
}

func TestPromptCache_TTLExpiry(t *testing.T) {
	pc := NewPromptCache(10, 100*time.Millisecond)

	pc.Set("key", &models.AnthropicResponse{ID: "test"}, 50)

	if _, _, found := pc.Get("key"); !found {
		t.Error("Expected to find cached response")
	}

	time.Sleep(150 * time.Millisecond)

	if _, _, found := pc.Get("key"); found {
		t.Error("Expected cache entry to be expired")
	}
}

func TestPromptCache_Stats(t *testing.T) {
	pc := NewPromptCache(10, time.Hour)

	// Initial stats
	stats := pc.Stats()
	if stats.Size != 0 || stats.MaxSize != 10 || stats.Hits != 0 || stats.Misses != 0 || stats.SavingsTokens != 0 {
		t.Error("Expected initial stats to be zero")
	}

	pc.Set("key", &models.AnthropicResponse{ID: "test"}, 200)

	// Hit
	pc.Get("key")
	stats = pc.Stats()
	if stats.Size != 1 || stats.Hits != 1 || stats.Misses != 0 || stats.SavingsTokens != 200 {
		t.Errorf("Unexpected stats after hit: %+v", stats)
	}

	// Miss
	pc.Get("non_existent")
	stats = pc.Stats()
	if stats.Hits != 1 || stats.Misses != 1 || stats.HitRate != 50 {
		t.Errorf("Unexpected stats after miss: %+v", stats)
	}
}

func TestPromptCache_Clear(t *testing.T) {
	pc := NewPromptCache(10, time.Hour)

	for i := 0; i < 5; i++ {
		pc.Set(string(rune('a'+i)), &models.AnthropicResponse{ID: string(rune('a' + i))}, 50)
	}

	if pc.Size() != 5 {
		t.Errorf("Expected size 5, got %d", pc.Size())
	}

	pc.Clear()

	if pc.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", pc.Size())
	}
}

func TestHasCacheControlMarkers_WithSystemPrompt(t *testing.T) {
	req := &models.AnthropicRequest{
		System: []models.ContentBlock{
			{Type: "text", Text: "You are helpful.", CacheControl: &models.CacheControl{Type: "ephemeral"}},
		},
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	if !HasCacheControlMarkers(req) {
		t.Error("Expected markers in system prompt")
	}
}

func TestHasCacheControlMarkers_WithMessageContent(t *testing.T) {
	req := &models.AnthropicRequest{
		Messages: []models.AnthropicMessage{
			{
				Role: "user",
				Content: []models.ContentBlock{
					{Type: "text", Text: "Hello", CacheControl: &models.CacheControl{Type: "ephemeral"}},
				},
			},
		},
	}

	if !HasCacheControlMarkers(req) {
		t.Error("Expected markers in message content")
	}
}

func TestHasCacheControlMarkers_WithTools(t *testing.T) {
	req := &models.AnthropicRequest{
		Tools: []models.AnthropicTool{
			{Name: "get_weather", CacheControl: &models.CacheControl{Type: "ephemeral"}},
		},
	}

	if !HasCacheControlMarkers(req) {
		t.Error("Expected markers in tools")
	}
}

func TestHasCacheControlMarkers_NoMarkers(t *testing.T) {
	req := &models.AnthropicRequest{
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	if HasCacheControlMarkers(req) {
		t.Error("Expected no markers")
	}
}

func TestHasCacheControlMarkers_StringContent(t *testing.T) {
	req := &models.AnthropicRequest{
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"}, // string content, no blocks
		},
	}

	if HasCacheControlMarkers(req) {
		t.Error("Expected no markers for string content")
	}
}

func TestGeneratePromptCacheKey_Basic(t *testing.T) {
	req := &models.AnthropicRequest{
		Model: "gpt-4o",
		System: []models.ContentBlock{
			{Type: "text", Text: "You are helpful.", CacheControl: &models.CacheControl{Type: "ephemeral"}},
		},
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	key, tokens, ok := GeneratePromptCacheKey(req)
	if !ok {
		t.Fatal("Expected request to be cacheable")
	}
	if key == "" {
		t.Error("Expected non-empty key")
	}
	if tokens <= 0 {
		t.Error("Expected positive token estimate")
	}

	// Same request should produce same key
	key2, _, _ := GeneratePromptCacheKey(req)
	if key != key2 {
		t.Error("Expected same key for same request")
	}
}

func TestGeneratePromptCacheKey_StreamingNotCacheable(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:  "gpt-4o",
		Stream: true,
		System: []models.ContentBlock{
			{Type: "text", Text: "System", CacheControl: &models.CacheControl{Type: "ephemeral"}},
		},
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, _, ok := GeneratePromptCacheKey(req)
	if ok {
		t.Error("Expected streaming request to not be cacheable")
	}
}

func TestGeneratePromptCacheKey_HighTemperatureNotCacheable(t *testing.T) {
	temp := 0.7
	req := &models.AnthropicRequest{
		Model:       "gpt-4o",
		Temperature: &temp,
		System: []models.ContentBlock{
			{Type: "text", Text: "System", CacheControl: &models.CacheControl{Type: "ephemeral"}},
		},
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, _, ok := GeneratePromptCacheKey(req)
	if ok {
		t.Error("Expected high temperature request to not be cacheable")
	}
}

func TestGeneratePromptCacheKey_NoMarkersNotCacheable(t *testing.T) {
	req := &models.AnthropicRequest{
		Model: "gpt-4o",
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, _, ok := GeneratePromptCacheKey(req)
	if ok {
		t.Error("Expected request without cache_control markers to not be cacheable")
	}
}

func TestGeneratePromptCacheKey_SameSystemDifferentMessages(t *testing.T) {
	// Two requests with same system prompt (cache_control) but different messages.
	// Since only system has cache_control, prefix is just model+system (no messages).
	// Different messages are all "suffix" and don't affect the prefix key.
	systemBlock := models.ContentBlock{Type: "text", Text: "You are helpful.", CacheControl: &models.CacheControl{Type: "ephemeral"}}

	req1 := &models.AnthropicRequest{
		Model: "gpt-4o",
		System: []models.ContentBlock{systemBlock},
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "What is 2+2?"},
		},
	}

	req2 := &models.AnthropicRequest{
		Model: "gpt-4o",
		System: []models.ContentBlock{systemBlock},
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "What is 3+3?"},
		},
	}

	key1, _, ok1 := GeneratePromptCacheKey(req1)
	key2, _, ok2 := GeneratePromptCacheKey(req2)

	if !ok1 || !ok2 {
		t.Fatal("Expected both requests to be cacheable")
	}

	if key1 != key2 {
		t.Error("Expected same prefix key for requests with same cached prefix but different suffix")
	}
}

func TestGeneratePromptCacheKey_DifferentPrefix(t *testing.T) {
	block1 := models.ContentBlock{Type: "text", Text: "System A", CacheControl: &models.CacheControl{Type: "ephemeral"}}
	block2 := models.ContentBlock{Type: "text", Text: "System B", CacheControl: &models.CacheControl{Type: "ephemeral"}}

	req1 := &models.AnthropicRequest{
		Model:    "gpt-4o",
		System:   []models.ContentBlock{block1},
		Messages: []models.AnthropicMessage{{Role: "user", Content: "Hello"}},
	}

	req2 := &models.AnthropicRequest{
		Model:    "gpt-4o",
		System:   []models.ContentBlock{block2},
		Messages: []models.AnthropicMessage{{Role: "user", Content: "Hello"}},
	}

	key1, _, _ := GeneratePromptCacheKey(req1)
	key2, _, _ := GeneratePromptCacheKey(req2)

	if key1 == key2 {
		t.Error("Expected different keys for different system prompts")
	}
}

func TestGeneratePromptCacheKey_DifferentModel(t *testing.T) {
	block := models.ContentBlock{Type: "text", Text: "System", CacheControl: &models.CacheControl{Type: "ephemeral"}}

	req1 := &models.AnthropicRequest{
		Model:    "gpt-4o",
		System:   []models.ContentBlock{block},
		Messages: []models.AnthropicMessage{{Role: "user", Content: "Hello"}},
	}

	req2 := &models.AnthropicRequest{
		Model:    "gpt-4o-mini",
		System:   []models.ContentBlock{block},
		Messages: []models.AnthropicMessage{{Role: "user", Content: "Hello"}},
	}

	key1, _, _ := GeneratePromptCacheKey(req1)
	key2, _, _ := GeneratePromptCacheKey(req2)

	if key1 == key2 {
		t.Error("Expected different keys for different models")
	}
}

func TestGeneratePromptCacheKey_SamePrefixDifferentSuffix(t *testing.T) {
	// Scenario: cache_control on message index 2, message index 3 is the varying suffix.
	// The prefix (messages 0-2) is identical; only the uncached suffix (message 3) differs.
	cachedContent := []models.ContentBlock{
		{Type: "text", Text: "Cached question", CacheControl: &models.CacheControl{Type: "ephemeral"}},
	}

	req1 := &models.AnthropicRequest{
		Model: "gpt-4o",
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "First message"},
			{Role: "assistant", Content: "First response"},
			{Role: "user", Content: cachedContent},
			{Role: "user", Content: "Suffix A"}, // uncached suffix, differs
		},
	}

	req2 := &models.AnthropicRequest{
		Model: "gpt-4o",
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "First message"},
			{Role: "assistant", Content: "First response"},
			{Role: "user", Content: cachedContent},
			{Role: "user", Content: "Suffix B"}, // uncached suffix, differs
		},
	}

	key1, _, ok1 := GeneratePromptCacheKey(req1)
	key2, _, ok2 := GeneratePromptCacheKey(req2)

	if !ok1 || !ok2 {
		t.Fatal("Expected both requests to be cacheable")
	}

	if key1 != key2 {
		t.Error("Expected same prefix key for same cached prefix with different uncached suffix")
	}
}

func TestPromptCache_EstimatedTokenSavings(t *testing.T) {
	pc := NewPromptCache(10, time.Hour)

	resp := &models.AnthropicResponse{ID: "test"}
	pc.Set("key", resp, 500)

	// First hit should save 500 tokens
	_, tokens1, _ := pc.Get("key")
	if tokens1 != 500 {
		t.Errorf("Expected 500 saved tokens, got %d", tokens1)
	}

	// Second hit should add another 500
	pc.Get("key")
	stats := pc.Stats()
	if stats.SavingsTokens != 1000 {
		t.Errorf("Expected 1000 total saved tokens, got %d", stats.SavingsTokens)
	}
}

func TestPromptCache_DefaultMaxSize(t *testing.T) {
	pc := NewPromptCache(0, time.Hour)
	if pc.Size() != 0 {
		t.Error("Expected empty cache")
	}

	// Default should be 100
	pc.Set("a", &models.AnthropicResponse{ID: "a"}, 50)
	pc.Set("b", &models.AnthropicResponse{ID: "b"}, 50)
	if pc.Size() != 2 {
		t.Errorf("Expected size 2, got %d", pc.Size())
	}
}

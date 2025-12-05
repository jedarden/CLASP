package tests

import (
	"testing"
	"time"

	"github.com/jedarden/clasp/internal/proxy"
	"github.com/jedarden/clasp/pkg/models"
)

func TestRequestCache_BasicOperations(t *testing.T) {
	cache := proxy.NewRequestCache(10, time.Hour)

	// Create a test response
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

	// Test Set and Get
	cache.Set("test_key", resp)

	got, found := cache.Get("test_key")
	if !found {
		t.Error("Expected to find cached response")
	}
	if got.ID != resp.ID {
		t.Errorf("Expected ID %s, got %s", resp.ID, got.ID)
	}
	if got.Content[0].Text != resp.Content[0].Text {
		t.Errorf("Expected text %s, got %s", resp.Content[0].Text, got.Content[0].Text)
	}

	// Test Get non-existent key
	_, found = cache.Get("non_existent")
	if found {
		t.Error("Expected not to find non-existent key")
	}
}

func TestRequestCache_LRUEviction(t *testing.T) {
	// Cache with max 3 entries
	cache := proxy.NewRequestCache(3, time.Hour)

	// Add 3 entries
	for i := 0; i < 3; i++ {
		resp := &models.AnthropicResponse{ID: string(rune('a' + i))}
		cache.Set(string(rune('a'+i)), resp)
	}

	// Verify all 3 are present
	for i := 0; i < 3; i++ {
		key := string(rune('a' + i))
		if _, found := cache.Get(key); !found {
			t.Errorf("Expected to find key %s", key)
		}
	}

	// Add 4th entry - should evict "a" (oldest)
	cache.Set("d", &models.AnthropicResponse{ID: "d"})

	// "a" should be evicted
	if _, found := cache.Get("a"); found {
		t.Error("Expected 'a' to be evicted")
	}

	// "b", "c", "d" should still exist
	for _, key := range []string{"b", "c", "d"} {
		if _, found := cache.Get(key); !found {
			t.Errorf("Expected to find key %s", key)
		}
	}
}

func TestRequestCache_LRUAccess(t *testing.T) {
	// Cache with max 3 entries
	cache := proxy.NewRequestCache(3, time.Hour)

	// Add 3 entries
	cache.Set("a", &models.AnthropicResponse{ID: "a"})
	cache.Set("b", &models.AnthropicResponse{ID: "b"})
	cache.Set("c", &models.AnthropicResponse{ID: "c"})

	// Access "a" to make it recently used
	cache.Get("a")

	// Add 4th entry - should evict "b" (oldest accessed)
	cache.Set("d", &models.AnthropicResponse{ID: "d"})

	// "b" should be evicted (oldest since "a" was recently accessed)
	if _, found := cache.Get("b"); found {
		t.Error("Expected 'b' to be evicted")
	}

	// "a", "c", "d" should still exist
	for _, key := range []string{"a", "c", "d"} {
		if _, found := cache.Get(key); !found {
			t.Errorf("Expected to find key %s", key)
		}
	}
}

func TestRequestCache_TTLExpiry(t *testing.T) {
	// Cache with 100ms TTL
	cache := proxy.NewRequestCache(10, 100*time.Millisecond)

	resp := &models.AnthropicResponse{ID: "test"}
	cache.Set("key", resp)

	// Should find it immediately
	if _, found := cache.Get("key"); !found {
		t.Error("Expected to find cached response")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Should not find it after TTL
	if _, found := cache.Get("key"); found {
		t.Error("Expected cache entry to be expired")
	}
}

func TestRequestCache_Stats(t *testing.T) {
	cache := proxy.NewRequestCache(10, time.Hour)

	// Initial stats
	size, maxSize, hits, misses, hitRate := cache.Stats()
	if size != 0 || maxSize != 10 || hits != 0 || misses != 0 || hitRate != 0 {
		t.Error("Expected initial stats to be zero")
	}

	// Add an entry
	cache.Set("key", &models.AnthropicResponse{ID: "test"})

	// Hit
	cache.Get("key")
	size, _, hits, misses, _ = cache.Stats()
	if size != 1 || hits != 1 || misses != 0 {
		t.Errorf("Expected size=1, hits=1, misses=0; got size=%d, hits=%d, misses=%d", size, hits, misses)
	}

	// Miss
	cache.Get("non_existent")
	_, _, hits, misses, hitRate = cache.Stats()
	if hits != 1 || misses != 1 || hitRate != 50 {
		t.Errorf("Expected hits=1, misses=1, hitRate=50; got hits=%d, misses=%d, hitRate=%.2f", hits, misses, hitRate)
	}
}

func TestRequestCache_Clear(t *testing.T) {
	cache := proxy.NewRequestCache(10, time.Hour)

	// Add entries
	for i := 0; i < 5; i++ {
		cache.Set(string(rune('a'+i)), &models.AnthropicResponse{ID: string(rune('a' + i))})
	}

	if cache.Size() != 5 {
		t.Errorf("Expected size 5, got %d", cache.Size())
	}

	// Clear
	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", cache.Size())
	}

	// Verify entries are gone
	for i := 0; i < 5; i++ {
		if _, found := cache.Get(string(rune('a' + i))); found {
			t.Error("Expected entry to be cleared")
		}
	}
}

func TestGenerateCacheKey_BasicRequest(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	key, cacheable := proxy.GenerateCacheKey(req)
	if !cacheable {
		t.Error("Expected request to be cacheable")
	}
	if key == "" {
		t.Error("Expected non-empty cache key")
	}

	// Same request should produce same key
	key2, _ := proxy.GenerateCacheKey(req)
	if key != key2 {
		t.Error("Expected same key for same request")
	}
}

func TestGenerateCacheKey_StreamingNotCacheable(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1024,
		Stream:    true,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, cacheable := proxy.GenerateCacheKey(req)
	if cacheable {
		t.Error("Expected streaming request to not be cacheable")
	}
}

func TestGenerateCacheKey_HighTemperatureNotCacheable(t *testing.T) {
	temp := 0.5
	req := &models.AnthropicRequest{
		Model:       "claude-3-5-sonnet-20241022",
		MaxTokens:   1024,
		Temperature: &temp,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, cacheable := proxy.GenerateCacheKey(req)
	if cacheable {
		t.Error("Expected high temperature request to not be cacheable")
	}
}

func TestGenerateCacheKey_ZeroTemperatureIsCacheable(t *testing.T) {
	temp := 0.0
	req := &models.AnthropicRequest{
		Model:       "claude-3-5-sonnet-20241022",
		MaxTokens:   1024,
		Temperature: &temp,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, cacheable := proxy.GenerateCacheKey(req)
	if !cacheable {
		t.Error("Expected zero temperature request to be cacheable")
	}
}

func TestGenerateCacheKey_DifferentMessagesProduceDifferentKeys(t *testing.T) {
	req1 := &models.AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	req2 := &models.AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Goodbye"},
		},
	}

	key1, _ := proxy.GenerateCacheKey(req1)
	key2, _ := proxy.GenerateCacheKey(req2)

	if key1 == key2 {
		t.Error("Expected different messages to produce different keys")
	}
}

func TestGenerateCacheKey_DifferentModelsProduceDifferentKeys(t *testing.T) {
	req1 := &models.AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	req2 := &models.AnthropicRequest{
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	key1, _ := proxy.GenerateCacheKey(req1)
	key2, _ := proxy.GenerateCacheKey(req2)

	if key1 == key2 {
		t.Error("Expected different models to produce different keys")
	}
}

func TestGenerateCacheKey_WithTools(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1024,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "What's the weather?"},
		},
		Tools: []models.AnthropicTool{
			{
				Name:        "get_weather",
				Description: "Get weather for a location",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		},
	}

	key, cacheable := proxy.GenerateCacheKey(req)
	if !cacheable {
		t.Error("Expected request with tools to be cacheable")
	}
	if key == "" {
		t.Error("Expected non-empty cache key")
	}
}

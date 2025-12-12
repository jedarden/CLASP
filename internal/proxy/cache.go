// Package proxy implements the HTTP proxy server.
package proxy

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/jedarden/clasp/pkg/models"
)

// CacheEntry represents a cached response.
type CacheEntry struct {
	Response  *models.AnthropicResponse
	CreatedAt time.Time
	Hits      int64
}

// RequestCache implements an LRU cache for API responses.
type RequestCache struct {
	mu sync.RWMutex

	// Configuration
	maxSize int
	ttl     time.Duration

	// Storage
	cache map[string]*list.Element
	lru   *list.List

	// Metrics
	hits   int64
	misses int64
}

// lruEntry holds cache key and entry for LRU list.
type lruEntry struct {
	key   string
	entry *CacheEntry
}

// NewRequestCache creates a new request cache.
// maxSize: maximum number of entries (0 = unlimited, not recommended)
// ttl: time-to-live for entries (0 = never expire)
func NewRequestCache(maxSize int, ttl time.Duration) *RequestCache {
	if maxSize <= 0 {
		maxSize = 1000 // Default to 1000 entries
	}
	return &RequestCache{
		maxSize: maxSize,
		ttl:     ttl,
		cache:   make(map[string]*list.Element),
		lru:     list.New(),
	}
}

// GenerateCacheKey creates a deterministic cache key from a request.
// Only caches requests where the response would be deterministic:
// - Same model, messages, system prompt, tools, and max_tokens
// - Excludes streaming requests (they need fresh responses)
// - Excludes requests with temperature > 0 (non-deterministic)
func GenerateCacheKey(req *models.AnthropicRequest) (string, bool) {
	// Don't cache streaming requests
	if req.Stream {
		return "", false
	}

	// Don't cache non-deterministic requests (temperature > 0)
	if req.Temperature != nil && *req.Temperature > 0 {
		return "", false
	}

	// Create a normalized representation for hashing
	normalized := struct {
		Model      string                    `json:"model"`
		System     interface{}               `json:"system"`
		Messages   []models.AnthropicMessage `json:"messages"`
		Tools      []models.AnthropicTool    `json:"tools"`
		ToolChoice interface{}               `json:"tool_choice,omitempty"`
		MaxTokens  int                       `json:"max_tokens"`
	}{
		Model:      req.Model,
		System:     req.System,
		Messages:   req.Messages,
		Tools:      req.Tools,
		ToolChoice: req.ToolChoice,
		MaxTokens:  req.MaxTokens,
	}

	// Marshal to JSON for consistent representation
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", false
	}

	// Create SHA256 hash
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), true
}

// Get retrieves a cached response if it exists and is not expired.
func (rc *RequestCache) Get(key string) (*models.AnthropicResponse, bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	elem, ok := rc.cache[key]
	if !ok {
		rc.misses++
		return nil, false
	}

	lruEnt, ok := elem.Value.(*lruEntry)
	if !ok {
		rc.misses++
		return nil, false
	}
	entry := lruEnt.entry

	// Check TTL
	if rc.ttl > 0 && time.Since(entry.CreatedAt) > rc.ttl {
		// Entry expired, remove it
		rc.removeElement(elem)
		rc.misses++
		return nil, false
	}

	// Cache hit - move to front of LRU and increment hits
	rc.lru.MoveToFront(elem)
	entry.Hits++
	rc.hits++

	return entry.Response, true
}

// Set stores a response in the cache.
func (rc *RequestCache) Set(key string, response *models.AnthropicResponse) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Check if entry already exists
	if elem, ok := rc.cache[key]; ok {
		// Update existing entry
		rc.lru.MoveToFront(elem)
		if lruEnt, typeOK := elem.Value.(*lruEntry); typeOK {
			lruEnt.entry = &CacheEntry{
				Response:  response,
				CreatedAt: time.Now(),
			}
		}
		return
	}

	// Evict oldest entries if at capacity
	for rc.lru.Len() >= rc.maxSize {
		rc.removeOldest()
	}

	// Add new entry
	entry := &CacheEntry{
		Response:  response,
		CreatedAt: time.Now(),
	}
	elem := rc.lru.PushFront(&lruEntry{key: key, entry: entry})
	rc.cache[key] = elem
}

// removeElement removes an element from both the cache map and LRU list.
func (rc *RequestCache) removeElement(elem *list.Element) {
	rc.lru.Remove(elem)
	if kv, ok := elem.Value.(*lruEntry); ok {
		delete(rc.cache, kv.key)
	}
}

// removeOldest removes the oldest entry from the cache.
func (rc *RequestCache) removeOldest() {
	elem := rc.lru.Back()
	if elem != nil {
		rc.removeElement(elem)
	}
}

// Stats returns cache statistics.
func (rc *RequestCache) Stats() (size, maxSize int, hits, misses int64, hitRate float64) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	size = rc.lru.Len()
	maxSize = rc.maxSize
	hits = rc.hits
	misses = rc.misses

	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	return
}

// Clear removes all entries from the cache.
func (rc *RequestCache) Clear() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.cache = make(map[string]*list.Element)
	rc.lru = list.New()
	// Keep metrics
}

// Size returns the current number of entries.
func (rc *RequestCache) Size() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.lru.Len()
}

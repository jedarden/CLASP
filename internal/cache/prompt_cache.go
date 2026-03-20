// Package cache implements prompt caching simulation for non-Anthropic backends.
package cache

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jedarden/clasp/pkg/models"
)

const charsPerToken = 4 // rough character-to-token estimate

// PromptCacheStats holds metrics for the prompt cache.
type PromptCacheStats struct {
	Size          int
	MaxSize       int
	Hits          int64
	Misses        int64
	SavingsTokens int64
	HitRate       float64
}

// PromptCacheEntry holds a cached response with token estimate.
type PromptCacheEntry struct {
	Response            *models.AnthropicResponse
	PrefixTokenEstimate int
	CreatedAt           time.Time
	Hits                int64
}

type promptLRUEntry struct {
	key   string
	entry *PromptCacheEntry
}

// PromptCache implements prefix-based LRU caching that simulates Anthropic's
// cache_control behavior for non-Anthropic backends. It caches full responses
// keyed by the hash of the cacheable prefix (system prompt + messages marked
// with cache_control). Requests with identical prefixes but different suffixes
// will return cached responses.
type PromptCache struct {
	mu sync.RWMutex

	maxSize int
	ttl     time.Duration

	cache map[string]*list.Element
	lru   *list.List

	hits          int64
	misses        int64
	savingsTokens int64
}

// NewPromptCache creates a new prompt cache.
func NewPromptCache(maxSize int, ttl time.Duration) *PromptCache {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &PromptCache{
		maxSize: maxSize,
		ttl:     ttl,
		cache:   make(map[string]*list.Element),
		lru:     list.New(),
	}
}

// HasCacheControlMarkers checks if any content blocks or tools have cache_control set.
func HasCacheControlMarkers(req *models.AnthropicRequest) bool {
	// Check system prompt
	if blocks, ok := req.System.([]models.ContentBlock); ok {
		for _, block := range blocks {
			if block.CacheControl != nil {
				return true
			}
		}
	}

	// Check messages
	for _, msg := range req.Messages {
		if blocks, ok := msg.Content.([]models.ContentBlock); ok {
			for _, block := range blocks {
				if block.CacheControl != nil {
					return true
				}
			}
		}
	}

	// Check tools
	for _, tool := range req.Tools {
		if tool.CacheControl != nil {
			return true
		}
	}

	return false
}

// GeneratePromptCacheKey creates a cache key from the cacheable prefix of a request.
// The prefix includes: model + system prompt + tools + all messages up to and including
// the last message containing a cache_control marker.
// Returns the key, estimated prefix tokens, and whether the request is eligible.
func GeneratePromptCacheKey(req *models.AnthropicRequest) (string, int, bool) {
	// Don't cache streaming or non-deterministic requests
	if req.Stream {
		return "", 0, false
	}
	if req.Temperature != nil && *req.Temperature > 0 {
		return "", 0, false
	}

	if !HasCacheControlMarkers(req) {
		return "", 0, false
	}

	// Find the last message index with cache_control
	lastCachedMsgIdx := -1
	for i, msg := range req.Messages {
		if blocks, ok := msg.Content.([]models.ContentBlock); ok {
			for _, block := range blocks {
				if block.CacheControl != nil {
					lastCachedMsgIdx = i
					break
				}
			}
		}
	}

	// Build prefix for hashing
	type prefix struct {
		Model    string                    `json:"model"`
		System   interface{}               `json:"system"`
		Messages []models.AnthropicMessage `json:"messages"`
		Tools    []models.AnthropicTool    `json:"tools"`
	}

	p := prefix{
		Model:  req.Model,
		System: req.System,
		Tools:  req.Tools,
	}

	if lastCachedMsgIdx >= 0 {
		p.Messages = req.Messages[:lastCachedMsgIdx+1]
	}

	data, err := json.Marshal(p)
	if err != nil {
		return "", 0, false
	}

	hash := sha256.Sum256(data)

	// Estimate tokens (rough: 4 chars per token)
	tokenEstimate := len(data) / charsPerToken
	if tokenEstimate == 0 {
		tokenEstimate = 1
	}

	return hex.EncodeToString(hash[:]), tokenEstimate, true
}

// Get retrieves a cached response if it exists and is not expired.
// Returns the response, estimated prefix tokens saved, and whether found.
func (pc *PromptCache) Get(key string) (*models.AnthropicResponse, int, bool) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	elem, ok := pc.cache[key]
	if !ok {
		atomic.AddInt64(&pc.misses, 1)
		return nil, 0, false
	}

	lruEnt, ok := elem.Value.(*promptLRUEntry)
	if !ok {
		atomic.AddInt64(&pc.misses, 1)
		return nil, 0, false
	}
	entry := lruEnt.entry

	// Check TTL
	if pc.ttl > 0 && time.Since(entry.CreatedAt) > pc.ttl {
		pc.removeElement(elem)
		atomic.AddInt64(&pc.misses, 1)
		return nil, 0, false
	}

	// Cache hit
	pc.lru.MoveToFront(elem)
	entry.Hits++
	atomic.AddInt64(&pc.hits, 1)
	atomic.AddInt64(&pc.savingsTokens, int64(entry.PrefixTokenEstimate))

	return entry.Response, entry.PrefixTokenEstimate, true
}

// Set stores a response in the cache.
func (pc *PromptCache) Set(key string, response *models.AnthropicResponse, prefixTokens int) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Update existing entry
	if elem, ok := pc.cache[key]; ok {
		pc.lru.MoveToFront(elem)
		if lruEnt, typeOK := elem.Value.(*promptLRUEntry); typeOK {
			lruEnt.entry = &PromptCacheEntry{
				Response:            response,
				PrefixTokenEstimate: prefixTokens,
				CreatedAt:           time.Now(),
			}
		}
		return
	}

	// Evict oldest entries if at capacity
	for pc.lru.Len() >= pc.maxSize {
		pc.removeOldest()
	}

	entry := &PromptCacheEntry{
		Response:            response,
		PrefixTokenEstimate: prefixTokens,
		CreatedAt:           time.Now(),
	}
	elem := pc.lru.PushFront(&promptLRUEntry{key: key, entry: entry})
	pc.cache[key] = elem
}

// removeElement removes an element from both the cache map and LRU list.
func (pc *PromptCache) removeElement(elem *list.Element) {
	pc.lru.Remove(elem)
	if kv, ok := elem.Value.(*promptLRUEntry); ok {
		delete(pc.cache, kv.key)
	}
}

// removeOldest removes the oldest entry from the cache.
func (pc *PromptCache) removeOldest() {
	elem := pc.lru.Back()
	if elem != nil {
		pc.removeElement(elem)
	}
}

// Stats returns cache statistics.
func (pc *PromptCache) Stats() PromptCacheStats {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	hits := atomic.LoadInt64(&pc.hits)
	misses := atomic.LoadInt64(&pc.misses)
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	return PromptCacheStats{
		Size:          pc.lru.Len(),
		MaxSize:       pc.maxSize,
		Hits:          hits,
		Misses:        misses,
		SavingsTokens: atomic.LoadInt64(&pc.savingsTokens),
		HitRate:       hitRate,
	}
}

// Clear removes all entries from the cache.
func (pc *PromptCache) Clear() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.cache = make(map[string]*list.Element)
	pc.lru = list.New()
}

// Size returns the current number of entries.
func (pc *PromptCache) Size() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.lru.Len()
}

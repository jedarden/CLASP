// Package session manages conversation session state for Responses API compaction.
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/jedarden/clasp/pkg/models"
)

// GenerateSessionKey creates a stable session key from an Anthropic request.
// The key is derived from the model and system prompt, which remain stable
// across multi-turn conversations.
//
// For clients that send a custom X-CLASP-Session-ID header, that value should
// be used directly instead of calling this function.
func GenerateSessionKey(req *models.AnthropicRequest) string {
	if req == nil {
		return ""
	}

	// Build a hash input from stable conversation elements
	hasher := sha256.New()

	// Include the model (after alias resolution)
	if req.Model != "" {
		hasher.Write([]byte(req.Model))
	}

	// Include the system prompt (stable across conversation turns)
	systemContent := extractSystemString(req.System)
	if systemContent != "" {
		hasher.Write([]byte("\x00")) // separator
		hasher.Write([]byte(systemContent))
	}

	// Include first user message to distinguish different conversations
	// with the same model/system (e.g., two parallel coding sessions)
	if len(req.Messages) > 0 {
		firstMsg := req.Messages[0]
		if firstMsg.Role == "user" {
			hasher.Write([]byte("\x00"))
			contentBytes, _ := json.Marshal(firstMsg.Content)
			hasher.Write(contentBytes)
		}
	}

	// Return truncated hex hash (16 chars is sufficient for session keys)
	return hex.EncodeToString(hasher.Sum(nil))[:16]
}

// extractSystemString extracts the system content as a string for hashing.
func extractSystemString(system interface{}) string {
	if system == nil {
		return ""
	}

	// Handle string system prompt
	if s, ok := system.(string); ok {
		return s
	}

	// Handle array of system content blocks
	if blocks, ok := system.([]interface{}); ok {
		var result string
		for _, block := range blocks {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if text, ok := blockMap["text"].(string); ok {
					result += text
				}
			}
		}
		return result
	}

	// Fallback: marshal to JSON
	bytes, err := json.Marshal(system)
	if err != nil {
		return ""
	}
	return string(bytes)
}

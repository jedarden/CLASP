// Package translator - compaction.go provides helpers for Responses API
// previous_response_id chaining (compaction).
package translator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/jedarden/clasp/pkg/models"
)

// SessionKey derives a stable session key from the first user message in the
// conversation. This fingerprints the conversation so that continuation
// requests (which carry the same opening message) map to the same session.
//
// Returns an empty string if no user message is present, which disables
// compaction for that request.
func SessionKey(req *models.AnthropicRequest) string {
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			contentJSON, err := json.Marshal(msg.Content)
			if err != nil {
				return ""
			}
			// Include model so different model conversations don't collide.
			h := sha256.New()
			h.Write([]byte(req.Model))
			h.Write(contentJSON)
			sum := h.Sum(nil)
			return hex.EncodeToString(sum[:16])
		}
	}
	return ""
}

// ExtractResponseID parses a Responses API JSON response body and returns the
// top-level "id" field. Returns an empty string if the body cannot be parsed.
func ExtractResponseID(body []byte) string {
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return ""
	}
	return resp.ID
}

// TrimMessagesForCompaction returns only the messages added since the previous
// compacted response. messageCount is the number of messages that were present
// when the previous response was generated.
//
// Returns nil when the slice cannot be meaningfully trimmed (e.g. messageCount
// is zero, exceeds the current length, or only covers a trailing subset of the
// same length), which signals the caller to use the full conversation context.
func TrimMessagesForCompaction(messages []models.AnthropicMessage, messageCount int) []models.AnthropicMessage {
	if messageCount <= 0 || messageCount >= len(messages) {
		return nil
	}
	return messages[messageCount:]
}

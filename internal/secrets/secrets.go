// Package secrets provides utilities for secure handling of sensitive data.
// This package ensures API keys and other secrets are never exposed in logs,
// error messages, or exported configurations.
package secrets

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Common API key patterns for detection and masking
var (
	// apiKeyPatterns matches common API key formats
	apiKeyPatterns = []*regexp.Regexp{
		regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),                   // OpenAI/Anthropic style
		regexp.MustCompile(`sk-or-[a-zA-Z0-9-]{20,}`),               // OpenRouter style
		regexp.MustCompile(`sk-ant-[a-zA-Z0-9-]{20,}`),              // Anthropic style
		regexp.MustCompile(`Bearer\s+[a-zA-Z0-9._-]+`),              // Bearer tokens
		regexp.MustCompile(`"api[_-]?key"\s*:\s*"[^"]+"`),           // JSON api_key fields
		regexp.MustCompile(`"authorization"\s*:\s*"[^"]+"`),         // JSON authorization fields
		regexp.MustCompile(`"x-api-key"\s*:\s*"[^"]+"`),             // JSON x-api-key fields
		regexp.MustCompile(`(?i)api[_-]?key[=:]\s*[a-zA-Z0-9._-]+`), // Generic key patterns
	}

	// Sensitive field names that should be masked in JSON
	sensitiveFields = []string{
		"api_key",
		"apiKey",
		"api-key",
		"authorization",
		"Authorization",
		"x-api-key",
		"X-Api-Key",
		"secret",
		"password",
		"token",
		"bearer",
	}
)

// MaskAPIKey masks an API key for safe display.
// Shows only the first 4 and last 4 characters.
// For very short keys, returns "***".
func MaskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// MaskBearer masks a Bearer token in a string.
func MaskBearer(s string) string {
	re := regexp.MustCompile(`Bearer\s+([a-zA-Z0-9._-]+)`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		parts := strings.SplitN(match, " ", 2)
		if len(parts) == 2 {
			return "Bearer " + MaskAPIKey(parts[1])
		}
		return match
	})
}

// MaskAllSecrets masks all known secret patterns in a string.
// Useful for sanitizing log output or error messages.
func MaskAllSecrets(s string) string {
	result := s

	// Mask sk-* style keys
	skPattern := regexp.MustCompile(`sk-[a-zA-Z0-9_-]{10,}`)
	result = skPattern.ReplaceAllStringFunc(result, MaskAPIKey)

	// Mask sk-or-* style keys
	skOrPattern := regexp.MustCompile(`sk-or-[a-zA-Z0-9_-]{10,}`)
	result = skOrPattern.ReplaceAllStringFunc(result, MaskAPIKey)

	// Mask sk-ant-* style keys
	skAntPattern := regexp.MustCompile(`sk-ant-[a-zA-Z0-9_-]{10,}`)
	result = skAntPattern.ReplaceAllStringFunc(result, MaskAPIKey)

	// Mask Bearer tokens
	result = MaskBearer(result)

	return result
}

// MaskJSONSecrets masks sensitive fields in JSON data.
// This is useful for sanitizing request/response logs.
func MaskJSONSecrets(jsonData []byte) []byte {
	// Try to parse as JSON
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		// Not valid JSON, try string masking instead
		return []byte(MaskAllSecrets(string(jsonData)))
	}

	// Recursively mask sensitive fields
	maskMapSecrets(data)

	// Re-marshal with indentation for readability
	result, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return []byte(MaskAllSecrets(string(jsonData)))
	}

	return result
}

// maskMapSecrets recursively masks sensitive fields in a map.
func maskMapSecrets(data map[string]interface{}) {
	for key, value := range data {
		// Check if this is a sensitive field
		if isSensitiveField(key) {
			if strVal, ok := value.(string); ok {
				data[key] = MaskAPIKey(strVal)
			}
			continue
		}

		// Recursively process nested structures
		switch v := value.(type) {
		case map[string]interface{}:
			maskMapSecrets(v)
		case []interface{}:
			data[key] = maskSliceSecretsAndReturn(v)
		case string:
			// Check if the string value contains secrets
			data[key] = MaskAllSecrets(v)
		}
	}
}

// maskSliceSecretsAndReturn recursively masks sensitive fields in a slice and returns it.
func maskSliceSecretsAndReturn(data []interface{}) []interface{} {
	result := make([]interface{}, len(data))
	for i, value := range data {
		switch v := value.(type) {
		case map[string]interface{}:
			maskMapSecrets(v)
			result[i] = v
		case []interface{}:
			result[i] = maskSliceSecretsAndReturn(v)
		case string:
			result[i] = MaskAllSecrets(v)
		default:
			result[i] = v
		}
	}
	return result
}

// isSensitiveField checks if a field name indicates sensitive data.
func isSensitiveField(name string) bool {
	nameLower := strings.ToLower(name)

	// Check exact matches first
	for _, sensitive := range sensitiveFields {
		if strings.EqualFold(sensitive, nameLower) {
			return true
		}
	}

	// Non-sensitive field names to exclude from pattern matching
	nonSensitive := []string{
		"max_tokens", "maxTokens", "total_tokens", "input_tokens", "output_tokens",
		"completion_tokens", "prompt_tokens", "tokenCount", "token_count",
	}
	for _, ns := range nonSensitive {
		if strings.EqualFold(ns, nameLower) {
			return false
		}
	}

	// Check for common patterns only for fields that look like they might be credentials
	// Must contain "key" or "secret" but not in a numeric/token context
	if strings.Contains(nameLower, "api_key") ||
		strings.Contains(nameLower, "apikey") ||
		strings.Contains(nameLower, "api-key") ||
		strings.Contains(nameLower, "secret_key") ||
		strings.Contains(nameLower, "secretkey") ||
		strings.Contains(nameLower, "private_key") ||
		nameLower == "secret" ||
		nameLower == "password" ||
		strings.Contains(nameLower, "credential") {
		return true
	}

	return false
}

// SanitizeHeaders masks sensitive headers for logging.
// Returns a new map with sensitive values masked.
func SanitizeHeaders(headers map[string][]string) map[string][]string {
	result := make(map[string][]string)
	for key, values := range headers {
		keyLower := strings.ToLower(key)
		if keyLower == "authorization" ||
			keyLower == "x-api-key" ||
			keyLower == "api-key" ||
			strings.Contains(keyLower, "key") ||
			strings.Contains(keyLower, "token") ||
			strings.Contains(keyLower, "secret") {
			// Mask all values
			maskedValues := make([]string, len(values))
			for i, v := range values {
				maskedValues[i] = MaskAPIKey(v)
			}
			result[key] = maskedValues
		} else {
			result[key] = values
		}
	}
	return result
}

// RedactForLog returns a string safe for logging.
// This is the primary function to use when logging any user-provided or
// configuration-derived data that might contain secrets.
func RedactForLog(s string) string {
	return MaskAllSecrets(s)
}

// IsPotentialSecret checks if a string looks like it might be a secret.
func IsPotentialSecret(s string) bool {
	if len(s) < 16 {
		return false
	}
	// Check for common key prefixes
	if strings.HasPrefix(s, "sk-") {
		return true
	}
	// Check for high entropy (many different characters)
	if hasHighEntropy(s) {
		return true
	}
	return false
}

// hasHighEntropy is a simple heuristic to detect potential secrets.
// Secrets typically have high character diversity.
func hasHighEntropy(s string) bool {
	if len(s) < 16 {
		return false
	}
	charSet := make(map[rune]bool)
	for _, c := range s {
		charSet[c] = true
	}
	// If more than 60% of characters are unique, likely a secret
	return float64(len(charSet))/float64(len(s)) > 0.6
}

// FormatKeySource returns a display string for where an API key came from.
func FormatKeySource(envVarName string, hasDirectKey bool) string {
	if envVarName != "" {
		return "${" + envVarName + "}"
	}
	if hasDirectKey {
		return "(stored in profile)"
	}
	return "(not configured)"
}

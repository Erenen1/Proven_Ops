package security

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	// Private key blocks
	privateKeyRegex = regexp.MustCompile(`(?s)-----BEGIN[ A-Z0-9_-]*PRIVATE KEY-----.*?-----END[ A-Z0-9_-]*PRIVATE KEY-----`)

	// JWT tokens (Bearer eyJ... or raw eyJ...)
	jwtRegex = regexp.MustCompile(`(?:Bearer\s+)?\beyJ[a-zA-Z0-9_-]{5,}\.[a-zA-Z0-9_-]{5,}\.[a-zA-Z0-9_-]+\b`)

	// URI passwords: scheme://user:pass@host
	uriPasswordRegex = regexp.MustCompile(`([a-zA-Z]+://[^:]+:)([^@]+)(@)`)

	// Common API tokens
	awsKeyRegex    = regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)
	githubKeyRegex = regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9_]{36,255}\b`)
	genericTokenRe = regexp.MustCompile(`(?i)(?:api_key|token|secret|password|passwd)\s*[:=]\s*["']?([a-zA-Z0-9_\-\.]{8,})["']?`)

	sensitiveKeys = map[string]bool{
		"password":      true,
		"passwd":        true,
		"secret":        true,
		"token":         true,
		"jwt":           true,
		"private_key":   true,
		"client_secret": true,
		"api_key":       true,
		"auth_token":    true,
		"bootstrap_token": true,
	}
)

// RedactString sanitizes strings containing potential passwords, tokens, or private keys
func RedactString(input string) string {
	if input == "" {
		return ""
	}

	result := privateKeyRegex.ReplaceAllString(input, "[REDACTED_PRIVATE_KEY]")
	result = jwtRegex.ReplaceAllString(result, "Bearer [REDACTED_JWT]")
	result = uriPasswordRegex.ReplaceAllString(result, "${1}[REDACTED]${3}")
	result = awsKeyRegex.ReplaceAllString(result, "[REDACTED_AWS_KEY]")
	result = githubKeyRegex.ReplaceAllString(result, "[REDACTED_GITHUB_TOKEN]")

	result = genericTokenRe.ReplaceAllStringFunc(result, func(match string) string {
		parts := strings.SplitN(match, ":", 2)
		if len(parts) == 2 {
			return parts[0] + ": [REDACTED]"
		}
		parts = strings.SplitN(match, "=", 2)
		if len(parts) == 2 {
			return parts[0] + "= [REDACTED]"
		}
		return match
	})

	return result
}

// RedactMap recursively redacts sensitive keys and values in maps
func RedactMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	cleaned := make(map[string]any)

	for k, v := range m {
		lowerK := strings.ToLower(k)
		if sensitiveKeys[lowerK] {
			cleaned[k] = "[REDACTED]"
			continue
		}

		switch val := v.(type) {
		case string:
			cleaned[k] = RedactString(val)
		case map[string]any:
			cleaned[k] = RedactMap(val)
		case []any:
			cleaned[k] = redactSlice(val)
		default:
			cleaned[k] = v
		}
	}

	return cleaned
}

func redactSlice(s []any) []any {
	out := make([]any, len(s))
	for i, item := range s {
		switch val := item.(type) {
		case string:
			out[i] = RedactString(val)
		case map[string]any:
			out[i] = RedactMap(val)
		case []any:
			out[i] = redactSlice(val)
		default:
			out[i] = item
		}
	}
	return out
}

// RedactJSON parses, redacts, and marshals JSON bytes
func RedactJSON(data []byte) []byte {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		// If not a map, redact as raw string
		return []byte(RedactString(string(data)))
	}
	redacted := RedactMap(m)
	out, _ := json.Marshal(redacted)
	return out
}

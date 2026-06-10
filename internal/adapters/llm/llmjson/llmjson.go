// Package llmjson extracts a JSON value from LLM output that may be
// wrapped in markdown fences or surrounded by commentary.
package llmjson

import "strings"

// Extract trims markdown code fences and leading/trailing prose so the
// result starts at the first JSON object or array.
func Extract(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		if end := strings.LastIndex(text, "```"); end >= 0 {
			text = text[:end]
		}
		text = strings.TrimSpace(text)
	}
	start := strings.IndexAny(text, "{[")
	if start < 0 {
		return text
	}
	var close byte = '}'
	if text[start] == '[' {
		close = ']'
	}
	end := strings.LastIndexByte(text, close)
	if end <= start {
		return text[start:]
	}
	return text[start : end+1]
}

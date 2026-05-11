package security

import "strings"

func LooksLikePromptInjection(text string) bool {
	t := strings.ToLower(text)
	return strings.Contains(t, "ignore previous instructions") || strings.Contains(t, "ignore as instruções anteriores")
}

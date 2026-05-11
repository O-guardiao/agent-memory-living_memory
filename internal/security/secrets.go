package security

import "strings"

func LooksLikeSecret(text string) bool {
	t := strings.ToLower(text)
	return strings.Contains(t, "api key") || strings.Contains(t, "password") || strings.Contains(t, "senha") || strings.Contains(t, "secret")
}

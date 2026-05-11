package security

import "strings"

func LooksLikePII(text string) bool {
	t := strings.ToLower(text)
	return strings.Contains(t, "@") || strings.Contains(t, "cpf") || strings.Contains(t, "telefone")
}

package ingestion

import "strings"

func NormalizeText(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

package consolidation

// Detect contradictory memory candidates and link them with relation_type=contradicts.

import (
	"strings"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
)

const subjectOverlapThreshold = 0.5

var negationMarkers = []string{
	"não", "nao", "not ", "never", "no longer", "stopped", "deixei de", "nunca", "jamais",
}

// Contradicts reports whether two memories of the same durable type talk
// about the same subject with opposite polarity.
func Contradicts(a, b memory.Memory) bool {
	if a.Type != b.Type {
		return false
	}
	if a.Type != memory.TypeFact && a.Type != memory.TypePreference {
		return false
	}
	if subjectOverlap(a.Content, b.Content) < subjectOverlapThreshold {
		return false
	}
	return hasNegation(a.Content) != hasNegation(b.Content)
}

// subjectOverlap compares the leading tokens (the likely subject phrase).
func subjectOverlap(a, b string) float64 {
	headA := leadingTokens(a, 5)
	headB := leadingTokens(b, 5)
	if len(headA) == 0 || len(headB) == 0 {
		return 0
	}
	setB := make(map[string]struct{}, len(headB))
	for _, token := range headB {
		setB[token] = struct{}{}
	}
	hits := 0
	for _, token := range headA {
		if _, ok := setB[token]; ok {
			hits++
		}
	}
	return float64(hits) / float64(len(headA))
}

func leadingTokens(text string, n int) []string {
	tokens := []string{}
	for _, token := range strings.Fields(strings.ToLower(text)) {
		token = strings.Trim(token, " .,;:!?()[]{}\"'")
		if len(token) <= 2 {
			continue
		}
		if hasNegationToken(token) {
			continue
		}
		tokens = append(tokens, token)
		if len(tokens) == n {
			break
		}
	}
	return tokens
}

func hasNegation(text string) bool {
	t := " " + strings.ToLower(text) + " "
	for _, marker := range negationMarkers {
		if strings.Contains(t, " "+marker) {
			return true
		}
	}
	return false
}

func hasNegationToken(token string) bool {
	switch token {
	case "não", "nao", "not", "never", "nunca", "jamais":
		return true
	}
	return false
}

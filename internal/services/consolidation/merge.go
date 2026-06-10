package consolidation

// Merge duplicate memories while preserving source event IDs and confidence.

import (
	"strings"
	"time"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
)

const mergeThreshold = 0.82

// Merge folds incoming into existing: source event IDs are united, the
// strongest confidence/importance win, and the version is bumped.
func Merge(existing, incoming memory.Memory, now time.Time) memory.Memory {
	merged := existing
	merged.SourceEventIDs = unionStrings(existing.SourceEventIDs, incoming.SourceEventIDs)
	if incoming.Confidence > merged.Confidence {
		merged.Confidence = incoming.Confidence
	}
	if incoming.Importance > merged.Importance {
		merged.Importance = incoming.Importance
	}
	if incoming.CreatedAt.Before(merged.CreatedAt) {
		merged.CreatedAt = incoming.CreatedAt
	}
	merged.UpdatedAt = now
	merged.Version++
	return merged
}

// Similar returns the token Jaccard similarity of two contents in [0,1].
func Similar(a, b string) float64 {
	setA := tokenSet(a)
	setB := tokenSet(b)
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}
	intersection := 0
	for token := range setA {
		if _, ok := setB[token]; ok {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	return float64(intersection) / float64(union)
}

func tokenSet(text string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, token := range strings.Fields(strings.ToLower(text)) {
		token = strings.Trim(token, " .,;:!?()[]{}\"'")
		if len(token) > 2 {
			out[token] = struct{}{}
		}
	}
	return out
}

func unionStrings(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, list := range [][]string{a, b} {
		for _, item := range list {
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

package consolidation

// Resolve valid_from/valid_until and newest-known facts here.

import (
	"time"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
)

const supersedeThreshold = 0.6

// ShouldSupersede reports whether incoming is a newer statement of the same
// fact/preference: similar enough to be about the same thing but different
// enough not to be a duplicate merge.
func ShouldSupersede(existing, incoming memory.Memory) bool {
	if existing.Type != incoming.Type {
		return false
	}
	if existing.Type != memory.TypeFact && existing.Type != memory.TypePreference {
		return false
	}
	similarity := Similar(existing.Content, incoming.Content)
	return similarity >= supersedeThreshold && similarity < mergeThreshold
}

// Supersede closes the validity window of old in favor of the newer memory.
func Supersede(old *memory.Memory, now time.Time) {
	old.Status = memory.StatusSuperseded
	until := now
	old.ValidUntil = &until
	old.UpdatedAt = now
}

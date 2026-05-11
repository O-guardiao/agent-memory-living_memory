package ingestion

import (
	"strings"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
)

func IsDuplicate(existing []memory.Memory, candidate memory.Memory) bool {
	c := strings.ToLower(strings.TrimSpace(candidate.Content))
	for _, mem := range existing {
		if strings.ToLower(strings.TrimSpace(mem.Content)) == c {
			return true
		}
	}
	return false
}

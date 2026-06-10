package consolidation

// Maintain memory_versions and superseded records here.

import (
	"time"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/ports"
)

// maxVersionHistory bounds the snapshot list kept in memory metadata.
const maxVersionHistory = 5

// Snapshot captures the current state of a memory before it changes.
func Snapshot(mem memory.Memory, reason string, idgen ports.IDGenerator, now time.Time) memory.Version {
	return memory.Version{
		ID:           idgen.NewID("ver"),
		MemoryID:     mem.ID,
		Version:      mem.Version,
		Content:      mem.Content,
		ChangeReason: reason,
		CreatedAt:    now,
	}
}

// AppendVersion stores the snapshot in the memory's metadata, keeping at
// most maxVersionHistory entries (oldest dropped first).
func AppendVersion(mem *memory.Memory, version memory.Version) {
	if mem.Metadata == nil {
		mem.Metadata = map[string]any{}
	}
	history, _ := mem.Metadata["versions"].([]memory.Version)
	history = append(history, version)
	if len(history) > maxVersionHistory {
		history = history[len(history)-maxVersionHistory:]
	}
	mem.Metadata["versions"] = history
}

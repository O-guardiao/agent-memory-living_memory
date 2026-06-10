package contextsvc

// Add source-event citations and memory provenance rendering here.

import (
	"fmt"
	"strings"

	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

// BuildCitations maps selected candidates to provenance citations.
func BuildCitations(candidates []retrieval.Candidate) []retrieval.Citation {
	if len(candidates) == 0 {
		return nil
	}
	citations := make([]retrieval.Citation, 0, len(candidates))
	for _, candidate := range candidates {
		citations = append(citations, retrieval.Citation{
			MemoryID:       candidate.Memory.ID,
			SourceEventIDs: candidate.Memory.SourceEventIDs,
			Score:          candidate.Score,
			Source:         candidate.Source,
			Reasons:        candidate.Reasons,
		})
	}
	return citations
}

// RenderProvenance formats a citation for prompts and logs, e.g.
// "mem_x (score 0.82, vector_search) ← evt_a, evt_b".
func RenderProvenance(c retrieval.Citation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (score %.2f", c.MemoryID, c.Score)
	if len(c.Reasons) > 0 {
		fmt.Fprintf(&b, ", %s", strings.Join(c.Reasons, "+"))
	}
	b.WriteString(")")
	if len(c.SourceEventIDs) > 0 {
		fmt.Fprintf(&b, " ← %s", strings.Join(c.SourceEventIDs, ", "))
	}
	return b.String()
}

package contextsvc

import (
	"strings"
	"testing"
	"time"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

func candidateWith(id, content string, score float64) retrieval.Candidate {
	mem := memory.NewMemory(time.Unix(0, 0), memory.TypeFact, memory.ScopeUser, content)
	mem.ID = id
	mem.SourceEventIDs = []string{"evt_" + id}
	return retrieval.Candidate{Memory: mem, Score: score, Source: "vector", Reasons: []string{"vector_search"}}
}

func TestBuildCitationsAndRender(t *testing.T) {
	citations := BuildCitations([]retrieval.Candidate{candidateWith("m1", "fato", 0.82)})
	if len(citations) != 1 || citations[0].MemoryID != "m1" {
		t.Fatalf("unexpected citations: %+v", citations)
	}
	rendered := RenderProvenance(citations[0])
	if !strings.Contains(rendered, "m1") || !strings.Contains(rendered, "evt_m1") {
		t.Fatalf("unexpected provenance: %s", rendered)
	}
}

func TestEnforceBudgetDropsLowestAndWarns(t *testing.T) {
	long := strings.Repeat("palavra ", 50)
	c1 := candidateWith("keep", long, 0.9)
	c2 := candidateWith("drop", long, 0.1)
	pack := retrieval.ContextPack{
		RelevantFacts: []memory.Memory{c1.Memory, c2.Memory},
		TokenEstimate: EstimateTokens(long) * 2,
	}
	EnforceBudget(&pack, []retrieval.Candidate{c1, c2}, EstimateTokens(long))
	if len(pack.RelevantFacts) != 1 || pack.RelevantFacts[0].ID != "keep" {
		t.Fatalf("expected lowest-score drop, got %+v", pack.RelevantFacts)
	}
	found := false
	for _, warning := range pack.Warnings {
		if warning == "context_compressed_to_budget" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing budget warning: %v", pack.Warnings)
	}
}

func TestEnforceBudgetNoOpWithinBudget(t *testing.T) {
	c := candidateWith("m", "curto", 0.5)
	pack := retrieval.ContextPack{RelevantFacts: []memory.Memory{c.Memory}, TokenEstimate: 5}
	EnforceBudget(&pack, []retrieval.Candidate{c}, 100)
	if len(pack.Warnings) != 0 || len(pack.RelevantFacts) != 1 {
		t.Fatalf("expected no-op, got %+v", pack)
	}
}

package contextsvc

import (
	"context"

	"github.com/agent-memory/agent-memory/internal/domain/agentic"
	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
	agenticsvc "github.com/agent-memory/agent-memory/internal/services/agentic"
	retrievalsvc "github.com/agent-memory/agent-memory/internal/services/retrieval"
)

type Assembler struct {
	retrieval *retrievalsvc.Service
	agentic   *agenticsvc.Service
}

func NewAssembler(retrieval *retrievalsvc.Service) *Assembler {
	return &Assembler{retrieval: retrieval}
}

func NewAssemblerWithControl(retrieval *retrievalsvc.Service, agenticSvc *agenticsvc.Service) *Assembler {
	return &Assembler{retrieval: retrieval, agentic: agenticSvc}
}

func (a *Assembler) Assemble(ctx context.Context, q retrieval.Query) (retrieval.ContextPack, error) {
	candidates, trace, err := a.retrieval.Retrieve(ctx, q)
	if err != nil {
		return retrieval.ContextPack{}, err
	}
	pack := retrieval.ContextPack{TraceID: trace.ID, TokenEstimate: trace.TokenEstimate}
	for _, c := range candidates {
		mem := c.Memory
		switch mem.Type {
		case memory.TypePreference:
			pack.Preferences = append(pack.Preferences, mem)
		case memory.TypeProcedure:
			pack.Procedures = append(pack.Procedures, mem)
		case memory.TypeFact:
			if mem.ProjectID != "" {
				pack.ProjectMemory = append(pack.ProjectMemory, mem)
			} else {
				pack.RelevantFacts = append(pack.RelevantFacts, mem)
			}
		case memory.TypeEpisode:
			pack.RecentEpisodes = append(pack.RecentEpisodes, mem)
		default:
			pack.RelevantFacts = append(pack.RelevantFacts, mem)
		}
	}
	if len(candidates) == 0 {
		pack.Warnings = append(pack.Warnings, "no_relevant_memory_found")
	}
	pack.Citations = BuildCitations(candidates)
	if q.TokenBudget > 0 {
		EnforceBudget(&pack, candidates, q.TokenBudget)
	}
	if a.agentic != nil {
		control, err := a.agentic.AssembleControlContext(ctx, agentic.ControlRequest{
			Scope: agentic.Scope{
				TenantID:  q.TenantID,
				UserID:    q.UserID,
				AgentID:   q.AgentID,
				ProjectID: q.ProjectID,
				SessionID: q.SessionID,
			},
			Query:       q.Text,
			TokenBudget: q.Limit * 220,
		})
		if err == nil && (control.ActiveSpec != nil || control.ActivePlan != nil || len(control.Rails) > 0 || len(control.RecommendedSkills) > 0) {
			pack.Control = &control
			pack.TokenEstimate += control.TokenEstimate
		}
	}
	return pack, nil
}

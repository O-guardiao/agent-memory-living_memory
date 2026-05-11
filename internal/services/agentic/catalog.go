package agentic

import domain "github.com/agent-memory/agent-memory/internal/domain/agentic"

func DefaultSkills() []domain.SkillCard {
	return []domain.SkillCard{
		{
			Name:          "brainstorming",
			Description:   "turn a rough request into a small, explicit design before implementation",
			UseWhen:       []string{"unclear requirements", "multiple possible approaches", "new feature", "architecture decision"},
			Inputs:        []string{"task", "constraints", "success criteria"},
			Outputs:       []string{"short design", "tradeoffs", "open questions"},
			TokenOverhead: 220,
			DefaultPhase:  domain.PhaseTriage,
		},
		{
			Name:          "spec-writing",
			Description:   "externalize requirements, non-goals, risks, and acceptance criteria into a durable artifact",
			UseWhen:       []string{"high-risk change", "multi-step execution", "team handoff", "user asks for everything ready"},
			Inputs:        []string{"approved design", "evidence", "constraints"},
			Outputs:       []string{"SPEC artifact", "acceptance criteria", "risk list"},
			TokenOverhead: 320,
			DefaultPhase:  domain.PhaseSpecify,
		},
		{
			Name:          "research-grounding",
			Description:   "force claims and decisions to cite fresh external or repository evidence",
			UseWhen:       []string{"URLs provided", "latest/current information", "market comparison", "unknown technology", "legal/security/compliance claims"},
			Inputs:        []string{"research question", "allowed sources", "recency needs"},
			Outputs:       []string{"evidence list", "source-backed decision", "uncertainties"},
			TokenOverhead: 280,
			DefaultPhase:  domain.PhaseResearch,
		},
		{
			Name:          "planning",
			Description:   "break an approved spec into small, verifiable tasks with exact files and checks",
			UseWhen:       []string{"implementation requested", "change touches multiple files", "agent needs handoff", "TDD desired"},
			Inputs:        []string{"SPEC artifact", "repo evidence", "risk budget"},
			Outputs:       []string{"PLAN artifact", "task checklist", "verification commands"},
			TokenOverhead: 260,
			DefaultPhase:  domain.PhasePlan,
		},
		{
			Name:          "execution",
			Description:   "execute one bounded plan step at a time while preserving context and audit trail",
			UseWhen:       []string{"approved plan", "clear next step", "implementation phase"},
			Inputs:        []string{"current task", "files", "expected checks"},
			Outputs:       []string{"code or artifact change", "test result", "step status"},
			TokenOverhead: 180,
			DefaultPhase:  domain.PhaseExecute,
		},
		{
			Name:          "verification",
			Description:   "verify outputs against spec, tests, evidence, and hallucination risks before completion",
			UseWhen:       []string{"before final answer", "after code changes", "before storing memory", "after research"},
			Inputs:        []string{"output", "spec", "plan", "evidence"},
			Outputs:       []string{"pass/fail", "gaps", "repair actions"},
			TokenOverhead: 240,
			DefaultPhase:  domain.PhaseVerify,
		},
		{
			Name:          "systematic-debugging",
			Description:   "recover from failures by isolating root cause instead of guessing or adding unrelated changes",
			UseWhen:       []string{"test failure", "unexpected behavior", "agent is stuck", "regression"},
			Inputs:        []string{"symptom", "last change", "logs", "expected behavior"},
			Outputs:       []string{"hypothesis", "minimal repro", "fix", "regression check"},
			TokenOverhead: 260,
			DefaultPhase:  domain.PhaseRecover,
		},
		{
			Name:          "memory-governance",
			Description:   "decide what deserves durable memory and what must be scoped, updated, redacted, or forgotten",
			UseWhen:       []string{"new preference", "contradiction", "PII", "security-sensitive fact", "long-term project state"},
			Inputs:        []string{"candidate memory", "source event", "scope", "retention policy"},
			Outputs:       []string{"memory decision", "scope", "confidence", "forget/update action"},
			TokenOverhead: 160,
			DefaultPhase:  domain.PhaseVerify,
		},
	}
}

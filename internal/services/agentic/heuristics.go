package agentic

import (
	"strings"
	"unicode"

	domain "github.com/agent-memory/agent-memory/internal/domain/agentic"
)

func inferRisk(req domain.DecisionRequest) domain.RiskLevel {
	if req.Risk != "" {
		return req.Risk
	}
	text := strings.ToLower(req.Task)
	critical := []string{"credential", "secret", "production", "prod", "security", "compliance", "gdpr", "lgpd", "payment", "delete", "migration", "database", "multi-tenant", "tenant", "pii"}
	for _, k := range critical {
		if strings.Contains(text, k) {
			return domain.RiskHigh
		}
	}
	if wordCount(text) > 80 || strings.Count(text, ",")+strings.Count(text, ";") > 4 || containsAny(text, "architecture", "arquitetura", "benchmark", "mercado", "zip", "everything ready", "tudo pronto") {
		return domain.RiskMedium
	}
	return domain.RiskLow
}

func inferHorizon(req domain.DecisionRequest) domain.Horizon {
	text := strings.ToLower(req.Task)
	if containsAny(text, "longo prazo", "long term", "roadmap", "arquitetura", "platform", "plataforma", "mercado", "produção", "production") {
		return domain.HorizonLong
	}
	if containsAny(text, "médio", "medio", "medium", "planejar", "plan", "implementar", "build", "zip", "entregar") || wordCount(text) > 45 {
		return domain.HorizonMedium
	}
	return domain.HorizonShort
}

func needsResearch(req domain.DecisionRequest) bool {
	text := strings.ToLower(req.Task)
	if req.HasEvidence {
		return false
	}
	if strings.Contains(text, "http://") || strings.Contains(text, "https://") {
		return true
	}
	return containsAny(text, "latest", "atual", "atuais", "mercado", "market", "benchmark", "paper", "arxiv", "github", "preço", "lei", "regulação", "hoje")
}

func needsSpec(req domain.DecisionRequest, risk domain.RiskLevel, horizon domain.Horizon) bool {
	if req.HasApprovedSpec {
		return false
	}
	if risk == domain.RiskHigh || risk == domain.RiskCritical {
		return true
	}
	if horizon == domain.HorizonLong || horizon == domain.HorizonMedium {
		return true
	}
	text := strings.ToLower(req.Task)
	return containsAny(text, "arquitetura", "architecture", "design", "implementar", "build", "gerar", "zip", "tudo pronto", "everything ready")
}

func needsPlan(req domain.DecisionRequest) bool {
	if req.HasPlan {
		return false
	}
	text := strings.ToLower(req.Task)
	return req.UserAskedForCode || containsAny(text, "implementar", "gerar", "build", "code", "código", "codigo", "zip", "pronto", "execute", "desenvolvimento")
}

func chooseSkills(req domain.DecisionRequest, risk domain.RiskLevel, horizon domain.Horizon, catalog []domain.SkillCard) []domain.SkillCard {
	wanted := map[string]bool{}
	if needsSpec(req, risk, horizon) {
		wanted["brainstorming"] = true
		wanted["spec-writing"] = true
	}
	if needsResearch(req) {
		wanted["research-grounding"] = true
	}
	if needsPlan(req) || req.HasApprovedSpec {
		wanted["planning"] = true
	}
	if req.HasPlan || req.UserAskedForCode {
		wanted["execution"] = true
		wanted["verification"] = true
	}
	if containsAny(strings.ToLower(req.Task), "bug", "erro", "error", "falha", "failing", "debug") {
		wanted["systematic-debugging"] = true
	}
	if containsAny(strings.ToLower(req.Task), "memória", "memoria", "memory", "lembrar", "forget", "esquecer", "pii", "lgpd") {
		wanted["memory-governance"] = true
	}
	out := make([]domain.SkillCard, 0, len(wanted))
	for _, skill := range catalog {
		if wanted[skill.Name] {
			out = append(out, skill)
		}
	}
	return out
}

func buildRails(req domain.DecisionRequest, risk domain.RiskLevel, horizon domain.Horizon) []domain.RailFinding {
	findings := []domain.RailFinding{}
	if needsResearch(req) {
		findings = append(findings, domain.RailFinding{
			Code:       "needs_evidence",
			Severity:   domain.RiskMedium,
			Message:    "task depends on fresh, external, or user-provided sources",
			Action:     "run research-grounding before persisting claims or making market/current recommendations",
			TokenCost:  280,
			Confidence: 0.86,
		})
	}
	if needsSpec(req, risk, horizon) {
		findings = append(findings, domain.RailFinding{
			Code:       "needs_spec",
			Severity:   risk,
			Message:    "task is large or risky enough that direct execution may create rework or hallucinated assumptions",
			Action:     "create or retrieve a compact SPEC artifact before planning or execution",
			TokenCost:  320,
			Confidence: 0.78,
		})
	}
	if needsPlan(req) {
		findings = append(findings, domain.RailFinding{
			Code:       "needs_plan",
			Severity:   domain.RiskMedium,
			Message:    "implementation request should be decomposed into verifiable tasks",
			Action:     "create a PLAN artifact with checkpoints and verification commands",
			TokenCost:  260,
			Confidence: 0.74,
		})
	}
	if containsAny(strings.ToLower(req.Task), "credencial", "credential", "secret", "token", "pii", "lgpd", "gdpr") {
		findings = append(findings, domain.RailFinding{
			Code:       "sensitive_data",
			Severity:   domain.RiskHigh,
			Message:    "task may involve sensitive data or durable memory with compliance implications",
			Action:     "scope memory narrowly, redact secrets, and create a deletion/update path",
			TokenCost:  160,
			Confidence: 0.80,
		})
	}
	return findings
}

func chooseRoute(req domain.DecisionRequest, risk domain.RiskLevel, horizon domain.Horizon) domain.Route {
	if needsResearch(req) {
		return domain.RouteResearch
	}
	if needsSpec(req, risk, horizon) {
		return domain.RouteSpecify
	}
	if needsPlan(req) {
		return domain.RoutePlan
	}
	if req.HasPlan || req.UserAskedForCode {
		return domain.RouteExecute
	}
	if req.Phase == domain.PhaseVerify {
		return domain.RouteVerify
	}
	return domain.RouteAnswerDirectly
}

func contextBudget(req domain.DecisionRequest, skills []domain.SkillCard, rails []domain.RailFinding) int {
	budget := req.TokenBudget
	if budget <= 0 {
		budget = 900
	}
	if budget < 350 {
		budget = 350
	}
	if budget > 2500 {
		budget = 2500
	}
	cost := 120
	for _, s := range skills {
		cost += s.TokenOverhead / 4
	}
	for _, r := range rails {
		cost += r.TokenCost / 5
	}
	if cost > budget {
		return budget
	}
	return cost
}

func containsAny(text string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(text, n) {
			return true
		}
	}
	return false
}

func wordCount(text string) int {
	count := 0
	inWord := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			if inWord {
				count++
				inWord = false
			}
			continue
		}
		inWord = true
	}
	if inWord {
		count++
	}
	return count
}

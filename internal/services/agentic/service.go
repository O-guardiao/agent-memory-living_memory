package agentic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "github.com/agent-memory/agent-memory/internal/domain/agentic"
	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
	"github.com/agent-memory/agent-memory/internal/ports"
)

const (
	kindSpec = "agentic_spec"
	kindPlan = "agentic_plan"
)

var ErrInvalidAgenticRequest = errors.New("invalid agentic request")

type Service struct {
	memories ports.MemoryStore
	idgen    ports.IDGenerator
	clock    ports.Clock
	skills   []domain.SkillCard
}

func NewService(memories ports.MemoryStore, idgen ports.IDGenerator, clock ports.Clock) *Service {
	return &Service{
		memories: memories,
		idgen:    idgen,
		clock:    clock,
		skills:   DefaultSkills(),
	}
}

func (s *Service) Skills() []domain.SkillCard {
	out := make([]domain.SkillCard, len(s.skills))
	copy(out, s.skills)
	return out
}

func (s *Service) Decide(ctx context.Context, req domain.DecisionRequest) (domain.DecisionAdvice, error) {
	_ = ctx
	if strings.TrimSpace(req.Task) == "" {
		return domain.DecisionAdvice{}, fmt.Errorf("%w: task is required", ErrInvalidAgenticRequest)
	}
	risk := inferRisk(req)
	horizon := inferHorizon(req)
	skills := chooseSkills(req, risk, horizon, s.skills)
	rails := buildRails(req, risk, horizon)
	route := chooseRoute(req, risk, horizon)

	advice := domain.DecisionAdvice{
		Route:             route,
		Reason:            reasonFor(route, risk, horizon),
		Risk:              risk,
		Horizon:           horizon,
		RecommendedSkills: skills,
		Rails:             rails,
		RequiredActions:   actionsFor(route, rails),
		ContextBudget:     contextBudget(req, skills, rails),
		ShouldWriteSpec:   route == domain.RouteSpecify || hasRail(rails, "needs_spec"),
		ShouldResearch:    route == domain.RouteResearch || hasRail(rails, "needs_evidence"),
		ShouldExecute:     route == domain.RouteExecute,
		ShouldVerify:      route == domain.RouteVerify || route == domain.RouteExecute || req.UserAskedForCode,
	}
	return advice, nil
}

func (s *Service) CreateSpec(ctx context.Context, req domain.SpecRequest) (domain.SpecArtifact, error) {
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Goal) == "" {
		return domain.SpecArtifact{}, fmt.Errorf("%w: title and goal are required", ErrInvalidAgenticRequest)
	}
	now := s.clock.Now()
	spec := domain.SpecArtifact{
		ID:                 s.idgen.NewID("spec"),
		Title:              strings.TrimSpace(req.Title),
		Goal:               strings.TrimSpace(req.Goal),
		Background:         strings.TrimSpace(req.Background),
		Constraints:        compactStrings(req.Constraints),
		AcceptanceCriteria: compactStrings(req.AcceptanceCriteria),
		NonGoals:           compactStrings(req.NonGoals),
		Risks:              compactStrings(req.Risks),
		Evidence:           compactStrings(req.Evidence),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	spec.Summary = summarizeSpec(spec)
	mem, err := memoryForSpec(req.Scope, spec, now)
	if err != nil {
		return domain.SpecArtifact{}, err
	}
	return spec, s.memories.Upsert(ctx, mem)
}

func (s *Service) CreatePlan(ctx context.Context, req domain.PlanRequest) (domain.PlanArtifact, error) {
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Objective) == "" {
		return domain.PlanArtifact{}, fmt.Errorf("%w: title and objective are required", ErrInvalidAgenticRequest)
	}
	if len(req.Steps) == 0 {
		return domain.PlanArtifact{}, fmt.Errorf("%w: at least one plan step is required", ErrInvalidAgenticRequest)
	}
	now := s.clock.Now()
	risk := req.Risk
	if risk == "" {
		risk = domain.RiskMedium
	}
	budget := req.TokenBudget
	if budget <= 0 {
		budget = 1200
	}
	steps := normalizeSteps(req.Steps)
	plan := domain.PlanArtifact{
		ID:           s.idgen.NewID("plan"),
		SpecID:       strings.TrimSpace(req.SpecID),
		Title:        strings.TrimSpace(req.Title),
		Objective:    strings.TrimSpace(req.Objective),
		Steps:        steps,
		Checkpoints:  compactStrings(req.Checkpoints),
		TokenBudget:  budget,
		Risk:         risk,
		EvidenceRefs: compactStrings(req.EvidenceRefs),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	plan.Summary = summarizePlan(plan)
	mem, err := memoryForPlan(req.Scope, plan, now)
	if err != nil {
		return domain.PlanArtifact{}, err
	}
	return plan, s.memories.Upsert(ctx, mem)
}

func (s *Service) GetSpec(ctx context.Context, scope domain.Scope, id string) (domain.SpecArtifact, error) {
	mem, err := s.memories.Get(ctx, scope.TenantID, id)
	if err != nil {
		return domain.SpecArtifact{}, err
	}
	return specFromMemory(mem)
}

func (s *Service) GetPlan(ctx context.Context, scope domain.Scope, id string) (domain.PlanArtifact, error) {
	mem, err := s.memories.Get(ctx, scope.TenantID, id)
	if err != nil {
		return domain.PlanArtifact{}, err
	}
	return planFromMemory(mem)
}

func (s *Service) AssembleControlContext(ctx context.Context, req domain.ControlRequest) (domain.ControlContext, error) {
	spec, _ := s.selectSpec(ctx, req)
	plan, _ := s.selectPlan(ctx, req)
	decision, _ := s.Decide(ctx, domain.DecisionRequest{
		Scope:           req.Scope,
		Task:            req.Query,
		HasApprovedSpec: spec != nil,
		HasPlan:         plan != nil,
		TokenBudget:     req.TokenBudget,
	})

	control := domain.ControlContext{
		ActiveSpec:        spec,
		ActivePlan:        plan,
		RecommendedSkills: decision.RecommendedSkills,
		Rails:             decision.Rails,
		Instructions:      buildControlInstructions(spec, plan, decision),
	}
	if plan != nil {
		control.CurrentStep = currentPlanStep(*plan)
	}
	control.TokenEstimate = estimateControlTokens(control)
	return control, nil
}

func (s *Service) selectSpec(ctx context.Context, req domain.ControlRequest) (*domain.SpecArtifact, error) {
	if req.SpecID != "" {
		spec, err := s.GetSpec(ctx, req.Scope, req.SpecID)
		if err != nil {
			return nil, err
		}
		return &spec, nil
	}
	mems, err := s.memories.List(ctx, queryForScope(req.Scope, req.Query))
	if err != nil {
		return nil, err
	}
	var selected *domain.SpecArtifact
	for _, mem := range mems {
		if !isKind(mem, kindSpec) {
			continue
		}
		spec, err := specFromMemory(mem)
		if err != nil {
			continue
		}
		if selected == nil || spec.UpdatedAt.After(selected.UpdatedAt) {
			copy := spec
			selected = &copy
		}
	}
	if selected == nil {
		return nil, memory.ErrNotFound
	}
	return selected, nil
}

func (s *Service) selectPlan(ctx context.Context, req domain.ControlRequest) (*domain.PlanArtifact, error) {
	if req.PlanID != "" {
		plan, err := s.GetPlan(ctx, req.Scope, req.PlanID)
		if err != nil {
			return nil, err
		}
		return &plan, nil
	}
	mems, err := s.memories.List(ctx, queryForScope(req.Scope, req.Query))
	if err != nil {
		return nil, err
	}
	var selected *domain.PlanArtifact
	for _, mem := range mems {
		if !isKind(mem, kindPlan) {
			continue
		}
		plan, err := planFromMemory(mem)
		if err != nil {
			continue
		}
		if req.SpecID != "" && plan.SpecID != req.SpecID {
			continue
		}
		if selected == nil || plan.UpdatedAt.After(selected.UpdatedAt) {
			copy := plan
			selected = &copy
		}
	}
	if selected == nil {
		return nil, memory.ErrNotFound
	}
	return selected, nil
}

func queryForScope(scope domain.Scope, text string) retrieval.Query {
	return retrieval.Query{
		TenantID:  scope.TenantID,
		UserID:    scope.UserID,
		AgentID:   scope.AgentID,
		ProjectID: scope.ProjectID,
		SessionID: scope.SessionID,
		Text:      text,
	}
}

func memoryForSpec(scope domain.Scope, spec domain.SpecArtifact, now time.Time) (memory.Memory, error) {
	payload, err := json.Marshal(spec)
	if err != nil {
		return memory.Memory{}, err
	}
	mem := memory.NewMemory(now, memory.TypeProcedure, memoryScope(scope), specToMarkdown(spec))
	mem.ID = spec.ID
	mem.TenantID = scope.TenantID
	mem.UserID = scope.UserID
	mem.AgentID = scope.AgentID
	mem.ProjectID = scope.ProjectID
	mem.SessionID = scope.SessionID
	mem.Summary = spec.Summary
	mem.Confidence = 0.92
	mem.Importance = 0.88
	mem.Metadata = map[string]any{
		"agentic_kind":  kindSpec,
		"artifact_json": string(payload),
		"title":         spec.Title,
	}
	return mem, nil
}

func memoryForPlan(scope domain.Scope, plan domain.PlanArtifact, now time.Time) (memory.Memory, error) {
	payload, err := json.Marshal(plan)
	if err != nil {
		return memory.Memory{}, err
	}
	mem := memory.NewMemory(now, memory.TypeProcedure, memoryScope(scope), planToMarkdown(plan))
	mem.ID = plan.ID
	mem.TenantID = scope.TenantID
	mem.UserID = scope.UserID
	mem.AgentID = scope.AgentID
	mem.ProjectID = scope.ProjectID
	mem.SessionID = scope.SessionID
	mem.Summary = plan.Summary
	mem.Confidence = 0.90
	mem.Importance = 0.86
	mem.Metadata = map[string]any{
		"agentic_kind":  kindPlan,
		"artifact_json": string(payload),
		"spec_id":       plan.SpecID,
		"title":         plan.Title,
	}
	return mem, nil
}

func memoryScope(scope domain.Scope) memory.Scope {
	switch {
	case scope.SessionID != "":
		return memory.ScopeSession
	case scope.ProjectID != "":
		return memory.ScopeProject
	case scope.AgentID != "":
		return memory.ScopeAgent
	case scope.UserID != "":
		return memory.ScopeUser
	default:
		return memory.ScopeTenant
	}
}

func specFromMemory(mem memory.Memory) (domain.SpecArtifact, error) {
	if !isKind(mem, kindSpec) {
		return domain.SpecArtifact{}, memory.ErrNotFound
	}
	var spec domain.SpecArtifact
	if err := json.Unmarshal([]byte(artifactJSON(mem)), &spec); err != nil {
		return domain.SpecArtifact{}, err
	}
	return spec, nil
}

func planFromMemory(mem memory.Memory) (domain.PlanArtifact, error) {
	if !isKind(mem, kindPlan) {
		return domain.PlanArtifact{}, memory.ErrNotFound
	}
	var plan domain.PlanArtifact
	if err := json.Unmarshal([]byte(artifactJSON(mem)), &plan); err != nil {
		return domain.PlanArtifact{}, err
	}
	return plan, nil
}

func isKind(mem memory.Memory, kind string) bool {
	if mem.Metadata == nil {
		return false
	}
	got, _ := mem.Metadata["agentic_kind"].(string)
	return got == kind
}

func artifactJSON(mem memory.Memory) string {
	if mem.Metadata == nil {
		return "{}"
	}
	if raw, ok := mem.Metadata["artifact_json"].(string); ok {
		return raw
	}
	return "{}"
}

func summarizeSpec(spec domain.SpecArtifact) string {
	parts := []string{spec.Title, spec.Goal}
	if len(spec.AcceptanceCriteria) > 0 {
		parts = append(parts, "acceptance: "+strings.Join(spec.AcceptanceCriteria, "; "))
	}
	return truncate(strings.Join(parts, " | "), 420)
}

func summarizePlan(plan domain.PlanArtifact) string {
	return truncate(fmt.Sprintf("%s | %s | %d steps | risk=%s", plan.Title, plan.Objective, len(plan.Steps), plan.Risk), 420)
}

func specToMarkdown(spec domain.SpecArtifact) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# SPEC: %s\n\nGoal: %s\n", spec.Title, spec.Goal)
	writeList(&b, "Acceptance Criteria", spec.AcceptanceCriteria)
	writeList(&b, "Constraints", spec.Constraints)
	writeList(&b, "Non Goals", spec.NonGoals)
	writeList(&b, "Risks", spec.Risks)
	writeList(&b, "Evidence", spec.Evidence)
	return b.String()
}

func planToMarkdown(plan domain.PlanArtifact) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# PLAN: %s\n\nObjective: %s\nRisk: %s\nToken budget: %d\n", plan.Title, plan.Objective, plan.Risk, plan.TokenBudget)
	for _, step := range plan.Steps {
		fmt.Fprintf(&b, "\n## %s: %s\n%s\n", step.ID, step.Title, step.Description)
		writeList(&b, "Files", step.Files)
		writeList(&b, "Verification", step.Verification)
	}
	writeList(&b, "Checkpoints", plan.Checkpoints)
	return b.String()
}

func writeList(b *strings.Builder, title string, values []string) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s:\n", title)
	for _, v := range values {
		fmt.Fprintf(b, "- %s\n", v)
	}
}

func normalizeSteps(steps []domain.PlanStep) []domain.PlanStep {
	out := make([]domain.PlanStep, 0, len(steps))
	for i, step := range steps {
		step.ID = strings.TrimSpace(step.ID)
		if step.ID == "" {
			step.ID = fmt.Sprintf("T%d", i+1)
		}
		step.Title = strings.TrimSpace(step.Title)
		step.Description = strings.TrimSpace(step.Description)
		step.Status = strings.TrimSpace(step.Status)
		if step.Status == "" {
			step.Status = "pending"
		}
		step.Files = compactStrings(step.Files)
		step.Verification = compactStrings(step.Verification)
		step.ExpectedEvidence = compactStrings(step.ExpectedEvidence)
		out = append(out, step)
	}
	return out
}

func currentPlanStep(plan domain.PlanArtifact) *domain.PlanStep {
	for _, step := range plan.Steps {
		status := strings.ToLower(strings.TrimSpace(step.Status))
		if status != "done" && status != "complete" && status != "completed" {
			copy := step
			return &copy
		}
	}
	return nil
}

func buildControlInstructions(spec *domain.SpecArtifact, plan *domain.PlanArtifact, decision domain.DecisionAdvice) []string {
	instructions := []string{
		"Inject only the smallest control context that changes the next decision.",
		"Prefer evidence-backed memories and active spec/plan artifacts over generic model priors.",
		"If evidence is missing for a current or external claim, route through research-grounding before committing it to memory.",
	}
	if spec != nil {
		instructions = append(instructions, "Treat the active SPEC as the source of truth for goal, non-goals, constraints, and acceptance criteria.")
	}
	if plan != nil {
		instructions = append(instructions, "Execute only the current PLAN step, then verify and update state before continuing.")
	}
	if decision.ShouldVerify {
		instructions = append(instructions, "Before completion, verify output against spec, plan, retrieved evidence, and safety rails.")
	}
	return instructions
}

func estimateControlTokens(control domain.ControlContext) int {
	words := 0
	for _, instruction := range control.Instructions {
		words += wordCount(instruction)
	}
	if control.ActiveSpec != nil {
		words += wordCount(control.ActiveSpec.Summary)
	}
	if control.ActivePlan != nil {
		words += wordCount(control.ActivePlan.Summary)
	}
	if control.CurrentStep != nil {
		words += wordCount(control.CurrentStep.Title) + wordCount(control.CurrentStep.Description)
	}
	words += len(control.RecommendedSkills) * 12
	words += len(control.Rails) * 18
	return int(float64(words) * 1.35)
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}

func truncate(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxLen {
		return value
	}
	return strings.TrimSpace(value[:maxLen-1]) + "…"
}

func reasonFor(route domain.Route, risk domain.RiskLevel, horizon domain.Horizon) string {
	switch route {
	case domain.RouteResearch:
		return "fresh or external evidence is required before reliable planning or memory updates"
	case domain.RouteSpecify:
		return fmt.Sprintf("%s-risk, %s-horizon work benefits from a compact spec before execution", risk, horizon)
	case domain.RoutePlan:
		return "implementation was requested but the next steps are not yet decomposed into verifiable work"
	case domain.RouteExecute:
		return "a plan exists or execution was explicitly requested; run one bounded step and verify it"
	case domain.RouteVerify:
		return "verification is the active phase"
	default:
		return "task is low-risk enough to answer directly with retrieved memory"
	}
}

func actionsFor(route domain.Route, rails []domain.RailFinding) []string {
	actions := []string{}
	for _, rail := range rails {
		actions = append(actions, rail.Action)
	}
	if len(actions) == 0 {
		switch route {
		case domain.RouteAnswerDirectly:
			actions = append(actions, "answer directly and avoid storing low-value transient context")
		case domain.RouteExecute:
			actions = append(actions, "execute the current bounded plan step")
		}
	}
	return compactStrings(actions)
}

func hasRail(rails []domain.RailFinding, code string) bool {
	for _, rail := range rails {
		if rail.Code == code {
			return true
		}
	}
	return false
}

package agentic

import "time"

type Phase string

const (
	PhaseTriage   Phase = "triage"
	PhaseSpecify  Phase = "specify"
	PhaseResearch Phase = "research"
	PhasePlan     Phase = "plan"
	PhaseExecute  Phase = "execute"
	PhaseVerify   Phase = "verify"
	PhaseRecover  Phase = "recover"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type Horizon string

const (
	HorizonShort  Horizon = "short"
	HorizonMedium Horizon = "medium"
	HorizonLong   Horizon = "long"
)

type Route string

const (
	RouteAnswerDirectly Route = "answer_directly"
	RouteClarify        Route = "clarify"
	RouteSpecify        Route = "specify"
	RouteResearch       Route = "research"
	RoutePlan           Route = "plan"
	RouteExecute        Route = "execute"
	RouteVerify         Route = "verify"
	RouteRecover        Route = "recover"
)

type Scope struct {
	TenantID  string `json:"tenant_id"`
	UserID    string `json:"user_id,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

type SkillCard struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	UseWhen       []string `json:"use_when"`
	Inputs        []string `json:"inputs"`
	Outputs       []string `json:"outputs"`
	TokenOverhead int      `json:"token_overhead"`
	DefaultPhase  Phase    `json:"default_phase"`
}

type RailFinding struct {
	Code       string    `json:"code"`
	Severity   RiskLevel `json:"severity"`
	Message    string    `json:"message"`
	Action     string    `json:"action"`
	TokenCost  int       `json:"token_cost,omitempty"`
	Confidence float64   `json:"confidence"`
}

type DecisionRequest struct {
	Scope
	Task             string    `json:"task"`
	Phase            Phase     `json:"phase,omitempty"`
	Risk             RiskLevel `json:"risk,omitempty"`
	TokenBudget      int       `json:"token_budget,omitempty"`
	HasApprovedSpec  bool      `json:"has_approved_spec,omitempty"`
	HasPlan          bool      `json:"has_plan,omitempty"`
	HasEvidence      bool      `json:"has_evidence,omitempty"`
	UserAskedForCode bool      `json:"user_asked_for_code,omitempty"`
}

type DecisionAdvice struct {
	Route             Route         `json:"route"`
	Reason            string        `json:"reason"`
	Risk              RiskLevel     `json:"risk"`
	Horizon           Horizon       `json:"horizon"`
	RecommendedSkills []SkillCard   `json:"recommended_skills,omitempty"`
	Rails             []RailFinding `json:"rails,omitempty"`
	RequiredActions   []string      `json:"required_actions,omitempty"`
	ContextBudget     int           `json:"context_budget"`
	ShouldWriteSpec   bool          `json:"should_write_spec"`
	ShouldResearch    bool          `json:"should_research"`
	ShouldExecute     bool          `json:"should_execute"`
	ShouldVerify      bool          `json:"should_verify"`
}

type SpecRequest struct {
	Scope
	Title              string   `json:"title"`
	Goal               string   `json:"goal"`
	Background         string   `json:"background,omitempty"`
	Constraints        []string `json:"constraints,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	NonGoals           []string `json:"non_goals,omitempty"`
	Risks              []string `json:"risks,omitempty"`
	Evidence           []string `json:"evidence,omitempty"`
}

type SpecArtifact struct {
	ID                 string    `json:"id"`
	Title              string    `json:"title"`
	Goal               string    `json:"goal"`
	Background         string    `json:"background,omitempty"`
	Constraints        []string  `json:"constraints,omitempty"`
	AcceptanceCriteria []string  `json:"acceptance_criteria,omitempty"`
	NonGoals           []string  `json:"non_goals,omitempty"`
	Risks              []string  `json:"risks,omitempty"`
	Evidence           []string  `json:"evidence,omitempty"`
	Summary            string    `json:"summary"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type PlanStep struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Files            []string `json:"files,omitempty"`
	Verification     []string `json:"verification,omitempty"`
	ExpectedEvidence []string `json:"expected_evidence,omitempty"`
	Status           string   `json:"status,omitempty"`
}

type PlanRequest struct {
	Scope
	SpecID       string     `json:"spec_id,omitempty"`
	Title        string     `json:"title"`
	Objective    string     `json:"objective"`
	Steps        []PlanStep `json:"steps"`
	Checkpoints  []string   `json:"checkpoints,omitempty"`
	TokenBudget  int        `json:"token_budget,omitempty"`
	Risk         RiskLevel  `json:"risk,omitempty"`
	EvidenceRefs []string   `json:"evidence_refs,omitempty"`
}

type PlanArtifact struct {
	ID           string     `json:"id"`
	SpecID       string     `json:"spec_id,omitempty"`
	Title        string     `json:"title"`
	Objective    string     `json:"objective"`
	Steps        []PlanStep `json:"steps"`
	Checkpoints  []string   `json:"checkpoints,omitempty"`
	TokenBudget  int        `json:"token_budget"`
	Risk         RiskLevel  `json:"risk"`
	EvidenceRefs []string   `json:"evidence_refs,omitempty"`
	Summary      string     `json:"summary"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ControlRequest struct {
	Scope
	Query       string `json:"query"`
	TokenBudget int    `json:"token_budget,omitempty"`
	SpecID      string `json:"spec_id,omitempty"`
	PlanID      string `json:"plan_id,omitempty"`
}

type ControlContext struct {
	ActiveSpec        *SpecArtifact `json:"active_spec,omitempty"`
	ActivePlan        *PlanArtifact `json:"active_plan,omitempty"`
	CurrentStep       *PlanStep     `json:"current_step,omitempty"`
	RecommendedSkills []SkillCard   `json:"recommended_skills,omitempty"`
	Rails             []RailFinding `json:"rails,omitempty"`
	Instructions      []string      `json:"instructions,omitempty"`
	TokenEstimate     int           `json:"token_estimate"`
}

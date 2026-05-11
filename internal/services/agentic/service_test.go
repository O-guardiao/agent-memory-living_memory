package agentic_test

import (
	"context"
	"strings"
	"testing"
	"time"

	memstorage "github.com/agent-memory/agent-memory/internal/adapters/storage/memory"
	domain "github.com/agent-memory/agent-memory/internal/domain/agentic"
	svc "github.com/agent-memory/agent-memory/internal/services/agentic"
)

type fakeClock struct{}

func (fakeClock) Now() time.Time { return time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC) }

type fakeID struct{ n int }

func (f *fakeID) NewID(prefix string) string {
	f.n++
	return prefix + "_test_" + string(rune('0'+f.n))
}

func TestDecideRoutesComplexCurrentWorkToResearchAndSpec(t *testing.T) {
	service := svc.NewService(memstorage.NewMemoryStore(), &fakeID{}, fakeClock{})
	advice, err := service.Decide(context.Background(), domain.DecisionRequest{
		Scope:       domain.Scope{TenantID: "tenant_test", UserID: "user_1"},
		Task:        "Analise https://example.com e gere uma arquitetura completa de memória agentica para produção com benchmarks atuais",
		TokenBudget: 900,
	})
	if err != nil {
		t.Fatal(err)
	}
	if advice.Route != domain.RouteResearch {
		t.Fatalf("expected research route, got %s", advice.Route)
	}
	if !advice.ShouldResearch || !advice.ShouldWriteSpec {
		t.Fatalf("expected research and spec requirements, got %#v", advice)
	}
	if len(advice.RecommendedSkills) == 0 {
		t.Fatalf("expected recommended skills")
	}
	if advice.ContextBudget <= 0 || advice.ContextBudget > 900 {
		t.Fatalf("context budget should be bounded, got %d", advice.ContextBudget)
	}
}

func TestCreateSpecPlanAndAssembleControlContext(t *testing.T) {
	service := svc.NewService(memstorage.NewMemoryStore(), &fakeID{}, fakeClock{})
	ctx := context.Background()
	scope := domain.Scope{TenantID: "tenant_test", UserID: "user_1", ProjectID: "proj_memory"}

	spec, err := service.CreateSpec(ctx, domain.SpecRequest{
		Scope: scope,
		Title: "Agentic control plane",
		Goal:  "Add compact spec, plan, rails, and skill guidance without bloating every prompt.",
		AcceptanceCriteria: []string{
			"Only inject control context when it changes the next decision.",
			"Every implementation step has verification.",
		},
		Constraints: []string{"Keep token overhead bounded."},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(spec.ID, "spec_") {
		t.Fatalf("expected spec id, got %s", spec.ID)
	}

	plan, err := service.CreatePlan(ctx, domain.PlanRequest{
		Scope:     scope,
		SpecID:    spec.ID,
		Title:     "Implement control plane",
		Objective: "Ship a working optional control-plane layer.",
		Steps: []domain.PlanStep{{
			Title:        "Add domain and service",
			Description:  "Create agentic types and service methods.",
			Verification: []string{"go test ./internal/services/agentic"},
		}},
		Checkpoints: []string{"Spec created", "Plan created", "Tests pass"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.SpecID != spec.ID {
		t.Fatalf("expected plan to reference spec %s, got %s", spec.ID, plan.SpecID)
	}

	control, err := service.AssembleControlContext(ctx, domain.ControlRequest{
		Scope:       scope,
		Query:       "implementar control plane",
		SpecID:      spec.ID,
		PlanID:      plan.ID,
		TokenBudget: 1200,
	})
	if err != nil {
		t.Fatal(err)
	}
	if control.ActiveSpec == nil || control.ActivePlan == nil || control.CurrentStep == nil {
		t.Fatalf("expected active spec, plan, and current step, got %#v", control)
	}
	if control.TokenEstimate <= 0 {
		t.Fatalf("expected positive token estimate")
	}
}

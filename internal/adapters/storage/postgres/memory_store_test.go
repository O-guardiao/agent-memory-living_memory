package postgres

import (
	"strings"
	"testing"

	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

func TestBuildMemoryListQueryAppliesTenantProjectStatusTypeAndTimeFilters(t *testing.T) {
	sql, args := buildMemoryListQuery(retrieval.Query{
		TenantID:  "tenant_a",
		UserID:    "user_1",
		ProjectID: "psi",
		Limit:     25,
		Filters: map[string]string{
			"status":        "active",
			"type":          "fact",
			"created_after": "2026-05-01T00:00:00Z",
		},
	}, false)

	for _, fragment := range []string{
		"tenant_id = $1",
		"user_id = $2",
		"project_id = $3",
		"status = $4",
		"type = $5",
		"created_at >= $6",
		"ORDER BY importance DESC, updated_at DESC",
		"LIMIT $7",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("missing SQL fragment %q in %s", fragment, sql)
		}
	}
	if len(args) != 7 {
		t.Fatalf("expected 7 args, got %d: %#v", len(args), args)
	}
}

func TestBuildMemoryListQueryUsesILIKEForTextSearch(t *testing.T) {
	sql, args := buildMemoryListQuery(retrieval.Query{
		TenantID: "tenant_a",
		UserID:   "user_1",
		Text:     "memory reliability",
		Limit:    10,
	}, true)

	if !strings.Contains(sql, "(content ILIKE $4 OR summary ILIKE $4)") {
		t.Fatalf("missing text search predicate in %s", sql)
	}
	if args[3] != "%memory reliability%" {
		t.Fatalf("unexpected text search arg: %#v", args)
	}
}

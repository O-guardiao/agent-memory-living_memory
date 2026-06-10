package retrievalsvc

// Add tenant/project/user/session filters and privacy gates here.

import (
	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

// ApplyScopeFilters propagates explicitly-set agent/project scope into the
// vector payload filters (the payload carries agent_id/project_id keys).
// Session is not in the payload, so session scoping happens post-fetch via
// PassesScope when the caller opts in with filters.session_only=true.
func ApplyScopeFilters(q retrieval.Query) retrieval.Query {
	addFilter := func(key, value string) {
		if value == "" {
			return
		}
		if q.Filters == nil {
			q.Filters = map[string]string{}
		}
		if _, ok := q.Filters[key]; !ok {
			q.Filters[key] = value
		}
	}
	addFilter("agent_id", q.AgentID)
	addFilter("project_id", q.ProjectID)
	return q
}

// PassesScope enforces the opt-in session post-filter.
func PassesScope(mem memory.Memory, q retrieval.Query) bool {
	if q.SessionID != "" && q.Filters["session_only"] == "true" && mem.SessionID != q.SessionID {
		return false
	}
	return true
}

// PassesPrivacy drops credential-labeled memories always and PII-labeled
// memories unless the caller opts in with filters.include_pii=true. Only
// enforced when privacy gates are enabled.
func PassesPrivacy(mem memory.Memory, q retrieval.Query) bool {
	for _, label := range mem.SafetyLabels {
		switch label {
		case memory.SafetyCredential:
			return false
		case memory.SafetyPII:
			if q.Filters["include_pii"] != "true" {
				return false
			}
		}
	}
	return true
}

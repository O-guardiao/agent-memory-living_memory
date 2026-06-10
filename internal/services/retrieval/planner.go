package retrievalsvc

// Planner extension point: decide whether to use vector, keyword, graph, recent events or full-context fallback.

import (
	"strconv"

	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

type SearchPlan struct {
	UseText    bool
	UseVector  bool
	UseKeyword bool
	UseGraph   bool
	GraphDepth int
}

// Plan reproduces the MVP defaults (text+vector always, graph when a store
// is wired) with two refinements: an empty query skips the wasted vector
// embedding, and filters can disable or deepen the graph expansion.
func Plan(q retrieval.Query, hasGraph, hasKeyword bool) SearchPlan {
	plan := SearchPlan{
		UseText:    true,
		UseVector:  q.Text != "",
		UseKeyword: hasKeyword && q.Text != "",
		UseGraph:   hasGraph && q.Filters["graph"] != "off",
		GraphDepth: 2,
	}
	if raw, ok := q.Filters["graph_depth"]; ok {
		if depth, err := strconv.Atoi(raw); err == nil && depth >= 1 && depth <= 4 {
			plan.GraphDepth = depth
		}
	}
	return plan
}

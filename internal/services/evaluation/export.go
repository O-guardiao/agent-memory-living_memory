package evaluation

// Export traces, answers and costs to evals/reports compatible JSONL.

import (
	"encoding/json"
	"io"
)

// costPerToken is a documented heuristic constant for rough cost reporting.
const costPerToken = 0.000003

type exportLine struct {
	TraceID       string             `json:"trace_id"`
	Query         string             `json:"query"`
	TenantID      string             `json:"tenant_id"`
	UserID        string             `json:"user_id"`
	CandidateIDs  []string           `json:"candidate_ids,omitempty"`
	SelectedIDs   []string           `json:"selected_ids,omitempty"`
	Scores        map[string]float64 `json:"scores,omitempty"`
	LatencyMS     int64              `json:"latency_ms"`
	TokenEstimate int                `json:"token_estimate"`
	CostEstimate  float64            `json:"cost_estimate"`
	CreatedAt     string             `json:"created_at"`
}

// ExportJSONL writes one JSON object per record, compatible with the
// evals/report tooling.
func ExportJSONL(w io.Writer, records []Record) error {
	encoder := json.NewEncoder(w)
	for _, rec := range records {
		line := exportLine{
			TraceID:       rec.TraceID,
			Query:         rec.Query.Text,
			TenantID:      rec.Query.TenantID,
			UserID:        rec.Query.UserID,
			CandidateIDs:  rec.CandidateIDs,
			SelectedIDs:   rec.SelectedIDs,
			Scores:        rec.Scores,
			LatencyMS:     rec.LatencyMS,
			TokenEstimate: rec.TokenEstimate,
			CostEstimate:  float64(rec.TokenEstimate) * costPerToken,
			CreatedAt:     rec.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		}
		if err := encoder.Encode(line); err != nil {
			return err
		}
	}
	return nil
}

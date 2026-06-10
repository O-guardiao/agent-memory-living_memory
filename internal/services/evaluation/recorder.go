package evaluation

// Record retrieval inputs/outputs for offline benchmark replay.

import (
	"sync"
	"time"

	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

const defaultMaxRecords = 10000

// Record captures one retrieval round-trip for offline replay.
type Record struct {
	TraceID       string             `json:"trace_id"`
	Query         retrieval.Query    `json:"query"`
	CandidateIDs  []string           `json:"candidate_ids,omitempty"`
	SelectedIDs   []string           `json:"selected_ids,omitempty"`
	Scores        map[string]float64 `json:"scores,omitempty"`
	LatencyMS     int64              `json:"latency_ms"`
	TokenEstimate int                `json:"token_estimate"`
	CreatedAt     time.Time          `json:"created_at"`
}

// Recorder is a bounded in-memory ring buffer of retrieval records.
type Recorder struct {
	mu      sync.Mutex
	max     int
	records []Record
}

func NewRecorder(max int) *Recorder {
	if max <= 0 {
		max = defaultMaxRecords
	}
	return &Recorder{max: max}
}

func (r *Recorder) Record(rec Record) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec)
	if len(r.records) > r.max {
		r.records = r.records[len(r.records)-r.max:]
	}
}

func (r *Recorder) Snapshot() []Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Record, len(r.records))
	copy(out, r.records)
	return out
}

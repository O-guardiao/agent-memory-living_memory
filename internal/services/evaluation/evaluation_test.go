package evaluation

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

func TestRecorderRingBuffer(t *testing.T) {
	recorder := NewRecorder(2)
	for i := 0; i < 3; i++ {
		recorder.Record(Record{TraceID: string(rune('a' + i))})
	}
	records := recorder.Snapshot()
	if len(records) != 2 || records[0].TraceID != "b" || records[1].TraceID != "c" {
		t.Fatalf("expected last two records, got %+v", records)
	}
}

func TestExportJSONL(t *testing.T) {
	var buf bytes.Buffer
	err := ExportJSONL(&buf, []Record{{
		TraceID:       "trace_1",
		Query:         retrieval.Query{TenantID: "t", UserID: "u", Text: "qual stack?"},
		SelectedIDs:   []string{"m1"},
		LatencyMS:     12,
		TokenEstimate: 100,
		CreatedAt:     time.Unix(0, 0).UTC(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["trace_id"] != "trace_1" || parsed["query"] != "qual stack?" {
		t.Fatalf("unexpected line: %v", parsed)
	}
	if parsed["cost_estimate"].(float64) <= 0 {
		t.Fatal("expected positive cost estimate")
	}
}

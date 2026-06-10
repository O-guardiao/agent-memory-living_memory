package httpadapter

import (
	"net/http"

	"github.com/agent-memory/agent-memory/internal/services/evaluation"
)

// handleEvalsExport streams the recorded retrievals as JSONL. It returns
// 404 when the recorder is disabled, mirroring the nil-Agentic guard.
func (s *Server) handleEvalsExport(w http.ResponseWriter, r *http.Request) {
	if s.deps.Evaluation == nil {
		writeError(w, http.StatusNotFound, errNotFound)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	if err := evaluation.ExportJSONL(w, s.deps.Evaluation.Snapshot()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
	}
}

package httpadapter

import (
	"net/http"

	"github.com/agent-memory/agent-memory/internal/services/ingestion"
)

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	var req ingestion.IngestRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	tenantID, err := s.tenantFor(r, req.TenantID)
	if err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	req.TenantID = tenantID
	if s.deps.Config.IngestionMode == "async" {
		if s.deps.AsyncIngestion == nil {
			writeError(w, http.StatusServiceUnavailable, errServiceUnavailable)
			return
		}
		if err := s.deps.AsyncIngestion.Enqueue(r.Context(), req); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":    "accepted",
			"topic":     ingestion.TopicIngestEvents,
			"tenant_id": req.TenantID,
			"user_id":   req.UserID,
		})
		return
	}
	resp, err := s.deps.Ingestion.Ingest(r.Context(), req)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

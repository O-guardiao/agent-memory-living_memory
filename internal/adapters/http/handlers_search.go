package httpadapter

import (
	"net/http"

	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

type searchResponse struct {
	Memories []retrieval.Candidate `json:"memories"`
	TraceID  string                `json:"trace_id"`
}

func (s *Server) handleMemorySearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	var q retrieval.Query
	if err := decodeJSON(r, &q); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if q.TenantID == "" {
		q.TenantID = s.deps.Config.DefaultTenant
	}
	candidates, trace, err := s.deps.Retrieval.Retrieve(r.Context(), q)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, searchResponse{Memories: candidates, TraceID: trace.ID})
}

package httpadapter

import (
	"net/http"
	"strings"
)

func (s *Server) handleTraceByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/traces/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, errNotFound)
		return
	}
	tenantID, err := s.tenantFor(r, r.URL.Query().Get("tenant_id"))
	if err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	trace, err := s.deps.Traces.GetTrace(r.Context(), tenantID, id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, trace)
}

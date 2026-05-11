package httpadapter

import (
	"net/http"

	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

func (s *Server) handleContextAssemble(w http.ResponseWriter, r *http.Request) {
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
	pack, err := s.deps.Context.Assemble(r.Context(), q)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, pack)
}

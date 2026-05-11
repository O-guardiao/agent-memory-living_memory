package httpadapter

import (
	"net/http"
	"strings"

	"github.com/agent-memory/agent-memory/internal/domain/policy"
)

func (s *Server) handleMemoryByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/memories/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, errNotFound)
		return
	}
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		tenantID = s.deps.Config.DefaultTenant
	}

	switch r.Method {
	case http.MethodGet:
		mem, err := s.deps.Memories.Get(r.Context(), tenantID, id)
		if err != nil {
			writeError(w, statusForError(err), err)
			return
		}
		writeJSON(w, http.StatusOK, mem)
	case http.MethodDelete:
		receipt, err := s.deps.Forgetting.Delete(r.Context(), policy.DeletionRequest{TenantID: tenantID, MemoryID: id, Reason: r.URL.Query().Get("reason")})
		if err != nil {
			writeError(w, statusForError(err), err)
			return
		}
		writeJSON(w, http.StatusOK, receipt)
	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

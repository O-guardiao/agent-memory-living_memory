package httpadapter

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/agent-memory/agent-memory/internal/domain/policy"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

func (s *Server) handleMemoryByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/memories/")
	if redactID, ok := strings.CutSuffix(id, "/redact"); ok {
		s.handleMemoryRedact(w, r, redactID)
		return
	}
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, errNotFound)
		return
	}
	tenantID, err := s.tenantFor(r, r.URL.Query().Get("tenant_id"))
	if err != nil {
		writeError(w, http.StatusForbidden, err)
		return
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

// handleMemoryRedact masks PII in a memory (partial deletion) and returns
// a deletion-style receipt.
func (s *Server) handleMemoryRedact(w http.ResponseWriter, r *http.Request, id string) {
	if s.deps.Redactor == nil {
		writeError(w, http.StatusNotFound, errNotFound)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, errNotFound)
		return
	}
	tenantID, err := s.tenantFor(r, r.URL.Query().Get("tenant_id"))
	if err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	receipt, err := s.deps.Redactor.Redact(r.Context(), tenantID, id, r.URL.Query().Get("reason"))
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}

// handleMemoryList lists memories for a tenant/user without a query.
func (s *Server) handleMemoryList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	tenantID, err := s.tenantFor(r, r.URL.Query().Get("tenant_id"))
	if err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	memories, err := s.deps.Memories.List(r.Context(), retrieval.Query{
		TenantID: tenantID,
		UserID:   r.URL.Query().Get("user_id"),
		Limit:    limit,
	})
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"memories": memories})
}

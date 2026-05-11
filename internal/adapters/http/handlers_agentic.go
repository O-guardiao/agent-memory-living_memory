package httpadapter

import (
	"net/http"
	"strings"

	"github.com/agent-memory/agent-memory/internal/domain/agentic"
)

func (s *Server) handleAgenticSkills(w http.ResponseWriter, r *http.Request) {
	if s.deps.Agentic == nil {
		writeError(w, http.StatusNotFound, errNotFound)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": s.deps.Agentic.Skills()})
}

func (s *Server) handleAgenticDecide(w http.ResponseWriter, r *http.Request) {
	if s.deps.Agentic == nil {
		writeError(w, http.StatusNotFound, errNotFound)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	var req agentic.DecisionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.TenantID = s.defaultTenant(req.TenantID)
	advice, err := s.deps.Agentic.Decide(r.Context(), req)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, advice)
}

func (s *Server) handleAgenticContext(w http.ResponseWriter, r *http.Request) {
	if s.deps.Agentic == nil {
		writeError(w, http.StatusNotFound, errNotFound)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	var req agentic.ControlRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.TenantID = s.defaultTenant(req.TenantID)
	control, err := s.deps.Agentic.AssembleControlContext(r.Context(), req)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, control)
}

func (s *Server) handleAgenticSpecs(w http.ResponseWriter, r *http.Request) {
	if s.deps.Agentic == nil {
		writeError(w, http.StatusNotFound, errNotFound)
		return
	}
	if r.URL.Path != "/v1/agentic/specs" {
		s.handleAgenticSpecByID(w, r)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	var req agentic.SpecRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.TenantID = s.defaultTenant(req.TenantID)
	spec, err := s.deps.Agentic.CreateSpec(r.Context(), req)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, spec)
}

func (s *Server) handleAgenticPlans(w http.ResponseWriter, r *http.Request) {
	if s.deps.Agentic == nil {
		writeError(w, http.StatusNotFound, errNotFound)
		return
	}
	if r.URL.Path != "/v1/agentic/plans" {
		s.handleAgenticPlanByID(w, r)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	var req agentic.PlanRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.TenantID = s.defaultTenant(req.TenantID)
	plan, err := s.deps.Agentic.CreatePlan(r.Context(), req)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, plan)
}

func (s *Server) handleAgenticSpecByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/agentic/specs/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, errNotFound)
		return
	}
	scope := scopeFromQuery(r, s.defaultTenant(r.URL.Query().Get("tenant_id")))
	spec, err := s.deps.Agentic.GetSpec(r.Context(), scope, id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, spec)
}

func (s *Server) handleAgenticPlanByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/agentic/plans/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, errNotFound)
		return
	}
	scope := scopeFromQuery(r, s.defaultTenant(r.URL.Query().Get("tenant_id")))
	plan, err := s.deps.Agentic.GetPlan(r.Context(), scope, id)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) defaultTenant(tenantID string) string {
	if tenantID != "" {
		return tenantID
	}
	return s.deps.Config.DefaultTenant
}

func scopeFromQuery(r *http.Request, tenantID string) agentic.Scope {
	q := r.URL.Query()
	return agentic.Scope{
		TenantID:  tenantID,
		UserID:    q.Get("user_id"),
		AgentID:   q.Get("agent_id"),
		ProjectID: q.Get("project_id"),
		SessionID: q.Get("session_id"),
	}
}

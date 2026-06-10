package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/agent-memory/agent-memory/internal/adapters/http/middleware"
	"github.com/agent-memory/agent-memory/internal/config"
	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/ports"
	agenticsvc "github.com/agent-memory/agent-memory/internal/services/agentic"
	contextsvc "github.com/agent-memory/agent-memory/internal/services/context"
	"github.com/agent-memory/agent-memory/internal/services/evaluation"
	"github.com/agent-memory/agent-memory/internal/services/forgetting"
	"github.com/agent-memory/agent-memory/internal/services/ingestion"
	retrievalsvc "github.com/agent-memory/agent-memory/internal/services/retrieval"
	"github.com/agent-memory/agent-memory/internal/telemetry"
)

type Dependencies struct {
	Ingestion      *ingestion.Service
	AsyncIngestion *ingestion.AsyncService
	Retrieval      *retrievalsvc.Service
	Context        *contextsvc.Assembler
	Agentic        *agenticsvc.Service
	Forgetting     *forgetting.Service
	Memories       ports.MemoryStore
	Traces         ports.TraceStore
	Config         config.Config
	Limiter        middleware.RateLimiter
	Evaluation     *evaluation.Recorder
	Redactor       *forgetting.Redactor
	Metrics        *telemetry.Metrics
}

type Server struct {
	mux  *http.ServeMux
	deps Dependencies
}

func NewServer(deps Dependencies) *Server {
	s := &Server{mux: http.NewServeMux(), deps: deps}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func decodeJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
}

func statusForError(err error) int {
	if errors.Is(err, memory.ErrNotFound) {
		return http.StatusNotFound
	}
	return http.StatusBadRequest
}

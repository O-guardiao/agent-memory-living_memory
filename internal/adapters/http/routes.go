package httpadapter

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.HandleFunc("/v1/events", s.withMiddleware(s.handleEvents))
	s.mux.HandleFunc("/v1/memories/search", s.withMiddleware(s.handleMemorySearch))
	s.mux.HandleFunc("/v1/memories/", s.withMiddleware(s.handleMemoryByID))
	s.mux.HandleFunc("/v1/context/assemble", s.withMiddleware(s.handleContextAssemble))
	s.mux.HandleFunc("/v1/traces/", s.withMiddleware(s.handleTraceByID))
	s.mux.HandleFunc("/v1/agentic/skills", s.withMiddleware(s.handleAgenticSkills))
	s.mux.HandleFunc("/v1/agentic/decide", s.withMiddleware(s.handleAgenticDecide))
	s.mux.HandleFunc("/v1/agentic/context", s.withMiddleware(s.handleAgenticContext))
	s.mux.HandleFunc("/v1/agentic/specs", s.withMiddleware(s.handleAgenticSpecs))
	s.mux.HandleFunc("/v1/agentic/specs/", s.withMiddleware(s.handleAgenticSpecs))
	s.mux.HandleFunc("/v1/agentic/plans", s.withMiddleware(s.handleAgenticPlans))
	s.mux.HandleFunc("/v1/agentic/plans/", s.withMiddleware(s.handleAgenticPlans))
}

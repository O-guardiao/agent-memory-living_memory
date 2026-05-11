package httpadapter

import "net/http"

func (s *Server) withMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Agent-Memory", "mvp")
		next(w, r)
	}
}

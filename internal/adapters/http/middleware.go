package httpadapter

import (
	"errors"
	"net/http"

	"github.com/agent-memory/agent-memory/internal/adapters/http/middleware"
)

var errTenantForbidden = errors.New("tenant not allowed for this key")

func (s *Server) withMiddleware(next http.HandlerFunc) http.HandlerFunc {
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Agent-Memory", "mvp")
		if s.deps.Metrics != nil {
			s.deps.Metrics.Counter("http_requests_total").Inc()
		}
		next(w, r)
	})
	h = middleware.RateLimit(s.deps.Limiter)(h)
	h = middleware.Tenant(s.deps.Config.DefaultTenant)(h)
	h = middleware.Auth(middleware.ParseAPIKeys(s.deps.Config.APIKeys))(h)
	h = middleware.RequestID(nil)(h)
	return h.ServeHTTP
}

// tenantFor resolves the tenant for a request: an explicit body/query value
// wins (subject to API-key binding), then the middleware-resolved tenant,
// then the configured default.
func (s *Server) tenantFor(r *http.Request, explicit string) (string, error) {
	if explicit != "" {
		if bound, ok := middleware.BoundTenantFromContext(r.Context()); ok && bound != explicit {
			return "", errTenantForbidden
		}
		return explicit, nil
	}
	if t := middleware.TenantFromContext(r.Context()); t != "" {
		return t, nil
	}
	return s.deps.Config.DefaultTenant, nil
}

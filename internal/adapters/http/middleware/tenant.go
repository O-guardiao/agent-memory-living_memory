package middleware

// Add tenant extraction and authorization here.

import "net/http"

const tenantHeader = "X-Tenant-ID"

// Tenant resolves the request tenant in priority order: tenant bound to the
// API key, X-Tenant-ID header, then the configured default. A header that
// conflicts with a key-bound tenant is rejected.
func Tenant(defaultTenant string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resolved := defaultTenant
			header := r.Header.Get(tenantHeader)
			if bound, ok := BoundTenantFromContext(r.Context()); ok {
				if header != "" && header != bound {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"error":"tenant not allowed for this key"}`))
					return
				}
				resolved = bound
			} else if header != "" {
				resolved = header
			}
			next.ServeHTTP(w, r.WithContext(WithTenant(r.Context(), resolved)))
		})
	}
}

package middleware

// Add authentication middleware here. The MVP server does not require auth.

import (
	"context"
	"net/http"
	"strings"
)

// ParseAPIKeys parses a comma-separated list of "key" or "key:tenant_id"
// entries into a key→tenant map. An empty tenant means the key may act on
// any tenant.
func ParseAPIKeys(raw string) map[string]string {
	keys := make(map[string]string)
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		key, tenant, _ := strings.Cut(entry, ":")
		if key != "" {
			keys[key] = tenant
		}
	}
	return keys
}

// Auth validates Bearer or X-API-Key credentials against the configured
// keys. An empty key set keeps the MVP open-access behavior.
func Auth(keys map[string]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if len(keys) == 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := bearerToken(r)
			if key == "" {
				key = r.Header.Get("X-API-Key")
			}
			tenant, ok := keys[key]
			if key == "" || !ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			ctx := r.Context()
			if tenant != "" {
				ctx = context.WithValue(ctx, boundTenantKey, tenant)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(auth) > len(prefix) && strings.EqualFold(auth[:len(prefix)], prefix) {
		return strings.TrimSpace(auth[len(prefix):])
	}
	return ""
}

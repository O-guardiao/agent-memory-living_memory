package middleware

import "context"

type ctxKey int

const (
	requestIDKey ctxKey = iota
	tenantKey
	boundTenantKey
)

func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantKey, tenantID)
}

func TenantFromContext(ctx context.Context) string {
	id, _ := ctx.Value(tenantKey).(string)
	return id
}

// BoundTenantFromContext returns the tenant an API key is restricted to,
// when auth is enabled and the key is tenant-scoped.
func BoundTenantFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(boundTenantKey).(string)
	return id, ok && id != ""
}

package middleware

// Add request ID propagation here.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const requestIDHeader = "X-Request-ID"

// RequestID propagates the inbound X-Request-ID header, generating one when
// absent, and echoes it on the response. A nil generate uses the default.
func RequestID(generate func() string) func(http.Handler) http.Handler {
	if generate == nil {
		generate = newRequestID
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(requestIDHeader)
			if id == "" {
				id = generate()
			}
			w.Header().Set(requestIDHeader, id)
			ctx := context.WithValue(r.Context(), requestIDKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func newRequestID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "req_00000000"
	}
	return "req_" + hex.EncodeToString(buf)
}

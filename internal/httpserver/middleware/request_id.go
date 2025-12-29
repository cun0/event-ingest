package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type ctxKeyRequestID struct{}

// for debugging and tracing
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			rid := newRequestID()
			w.Header().Set("X-Request-Id", rid)
			ctx := context.WithValue(r.Context(), ctxKeyRequestID{}, rid)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}
}

func GetRequestID(ctx context.Context) string {
	v := ctx.Value(ctxKeyRequestID{})
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

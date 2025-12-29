// internal/httpapi/middleware/recover.go
package middleware

import (
	"net/http"

	"github.com/cun0/insider-case/internal/jsonlog"
)

func Recover(logger *jsonlog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					props := map[string]string{
						"request_id": GetRequestID(r.Context()),
						"method":     r.Method,
						"path":       r.URL.Path,
					}
					logger.PrintErrorWithTrace(errFromPanic(rec), props)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

type panicError struct{ msg string }

func (e panicError) Error() string { return e.msg }

func errFromPanic(rec any) error {
	switch v := rec.(type) {
	case error:
		return v
	case string:
		return panicError{msg: v}
	default:
		return panicError{msg: "panic"}
	}
}

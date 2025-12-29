package middleware

import (
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/cun0/insider-case/internal/jsonlog"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(p []byte) (int, error) {
	if sr.status == 0 {
		sr.status = http.StatusOK
	}
	n, err := sr.ResponseWriter.Write(p)
	sr.bytes += n
	return n, err
}

func AccessLog(logger *jsonlog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sr := &statusRecorder{ResponseWriter: w}

			next.ServeHTTP(sr, r)

			props := map[string]string{
				"request_id":  GetRequestID(r.Context()),
				"method":      r.Method,
				"path":        r.URL.Path,
				"status":      strconv.Itoa(sr.status),
				"bytes":       strconv.Itoa(sr.bytes),
				"duration_ms": strconv.FormatInt(time.Since(start).Milliseconds(), 10),
				"remote_ip":   clientIP(r),
			}

			logger.PrintInfo("request completed", props)
		}
		return http.HandlerFunc(fn)
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

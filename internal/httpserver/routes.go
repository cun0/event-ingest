package httpserver

import (
	"net/http"
	"time"

	"github.com/cun0/insider-case/internal/httpserver/middleware"
	"github.com/cun0/insider-case/internal/ingest"
	"github.com/cun0/insider-case/internal/jsonlog"
)

type Config struct {
	RequestTimeout time.Duration
}

func BuildHandler(cfg Config, logger *jsonlog.Logger, sink ingest.Sink, events EventBatchStore, metrics MetricsStore) http.Handler {
	h := New(logger, sink, events, metrics)

	mux := http.NewServeMux()

	const (
		maxEventsBody     = 256 << 10 // 256KB
		maxEventsBulkBody = 5 << 20   // 5MB
	)

	mux.HandleFunc("/healthz", h.Healthz)

	mux.Handle("/events",
		middleware.BodyLimit(maxEventsBody)(
			http.HandlerFunc(h.PostEvent),
		),
	)

	mux.Handle("/events/bulk",
		middleware.BodyLimit(maxEventsBulkBody)(
			http.HandlerFunc(h.PostEventsBulk),
		),
	)

	mux.HandleFunc("/metrics", h.GetMetrics)

	var handler http.Handler = mux
	handler = middleware.AccessLog(logger)(handler)
	handler = middleware.Timeout(cfg.RequestTimeout)(handler)
	handler = middleware.RequestID()(handler)
	handler = middleware.Recover(logger)(handler)

	return handler
}

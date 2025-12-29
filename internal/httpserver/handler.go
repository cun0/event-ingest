package httpserver

import (
	"context"
	"net/http"
	"time"

	"github.com/cun0/insider-case/internal/domain"
	"github.com/cun0/insider-case/internal/ingest"
	"github.com/cun0/insider-case/internal/jsonlog"
	"github.com/cun0/insider-case/internal/repo"
)

type MetricsStore interface {
	Totals(ctx context.Context, eventName string, from, to time.Time, channel string) (repo.MetricsTotals, error)
	ByChannel(ctx context.Context, eventName string, from, to time.Time, channel string) ([]repo.MetricsByChannelRow, error)
}

type EventBatchStore interface {
	InsertBatch(ctx context.Context, events []domain.Event) (map[string]struct{}, error)
}

type Handler struct {
	logger  *jsonlog.Logger
	ingest  ingest.Sink
	events  EventBatchStore
	metrics MetricsStore
	clock   func() time.Time
}

func New(logger *jsonlog.Logger, sink ingest.Sink, events EventBatchStore, metrics MetricsStore) *Handler {
	return &Handler{
		logger:  logger,
		ingest:  sink,
		events:  events,
		metrics: metrics,
		clock:   time.Now,
	}
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

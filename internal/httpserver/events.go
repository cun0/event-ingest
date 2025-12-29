package httpserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/cun0/insider-case/internal/domain"
	"github.com/cun0/insider-case/internal/httpserver/middleware"
	"github.com/cun0/insider-case/internal/ingest"
)

const statusClientClosedRequest = 499

func (h *Handler) PostEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var p domain.EventPayload
	if err := decodeJSON(r.Body, &p); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := h.clock().UTC()
	if err := p.Validate(now); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ev, err := p.ToEvent(now)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	res, err := h.ingest.Submit(r.Context(), ev)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			writeError(w, statusClientClosedRequest, "client closed request")
			return
		}
		if errors.Is(err, ingest.ErrStopped) {
			writeError(w, http.StatusServiceUnavailable, "ingestion temporarily unavailable")
			return
		}

		h.logger.PrintError(err, map[string]string{
			"request_id": middleware.GetRequestID(r.Context()),
			"component":  "post_event",
		})
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	status := "inserted"
	if res.Duplicate() {
		status = "duplicate"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    status,
		"dedup_key": ev.DedupKey,
	})
}

package httpserver

import (
	"net/http"

	"github.com/cun0/insider-case/internal/domain"
)

func (h *Handler) PostEventsBulk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var payloads []domain.EventPayload
	if err := decodeJSON(r.Body, &payloads); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(payloads) == 0 {
		writeError(w, http.StatusBadRequest, "empty payload")
		return
	}

	now := h.clock().UTC()

	events := make([]domain.Event, 0, len(payloads))
	invalid := 0

	for i := range payloads {
		p := payloads[i]

		if err := p.Validate(now); err != nil {
			invalid++
			continue
		}

		ev, err := p.ToEvent(now)
		if err != nil {
			invalid++
			continue
		}

		events = append(events, ev)
	}

	// Nothing valid.
	if len(events) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"received":   len(payloads),
			"inserted":   0,
			"duplicate":  0,
			"invalid":    invalid,
			"processed":  0,
			"batch_fail": 0,
		})
		return
	}

	// Chunk to avoid Postgres param limit (65535 params).
	// InsertBatch uses 8 params per row => max rows ~= 8191. Keep safe margin.
	const chunkSize = 4000

	inserted := 0
	duplicate := 0
	batchFail := 0

	for start := 0; start < len(events); start += chunkSize {
		end := start + chunkSize
		if end > len(events) {
			end = len(events)
		}

		chunk := events[start:end]

		insertedKeys, err := h.events.InsertBatch(r.Context(), chunk)
		if err != nil {
			// If a chunk fails, count it as batch_fail; we don't know duplicates/inserted.
			batchFail += len(chunk)
			continue
		}

		// insertedKeys contains only keys that were actually inserted.
		inserted += len(insertedKeys)
		duplicate += (len(chunk) - len(insertedKeys))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"received":   len(payloads),
		"processed":  len(events),
		"inserted":   inserted,
		"duplicate":  duplicate,
		"invalid":    invalid,
		"batch_fail": batchFail,
	})
}

package httpserver

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cun0/insider-case/internal/repo"
)

func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	q := r.URL.Query()

	eventName := strings.TrimSpace(q.Get("event_name"))
	if eventName == "" {
		writeError(w, http.StatusBadRequest, "event_name is required")
		return
	}

	now := h.clock().UTC()

	from, ok, err := parseUnixParam(q.Get("from"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid from")
		return
	}
	to, ok2, err := parseUnixParam(q.Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid to")
		return
	}

	// If not provided: default last 1 hour.
	if !ok2 {
		to = now
	}
	if !ok {
		from = to.Add(-1 * time.Hour)
	}

	from = from.UTC()
	to = to.UTC()

	if !from.Before(to) {
		writeError(w, http.StatusBadRequest, "from must be < to")
		return
	}

	channel := strings.TrimSpace(q.Get("channel")) // optional filter

	totals, err := h.metrics.Totals(r.Context(), eventName, from, to, channel)
	if err != nil {
		h.logger.PrintError(err, map[string]string{
			"component": "get_metrics",
		})
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	byChannel, err := h.metrics.ByChannel(r.Context(), eventName, from, to, channel)
	if err != nil {
		h.logger.PrintError(err, map[string]string{
			"component": "get_metrics",
		})
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	type channelRow struct {
		Channel string `json:"channel"`
		Total   int64  `json:"total"`
		Unique  int64  `json:"unique"`
	}

	out := make([]channelRow, 0, len(byChannel))
	for _, r := range byChannel {
		out = append(out, channelRow{
			Channel: r.Channel,
			Total:   r.Total,
			Unique:  r.Unique,
		})
	}

	resp := map[string]any{
		"event_name": eventName,
		"from":       from.Unix(),
		"to":         to.Unix(),
		"total":      totals.Total,
		"unique":     totals.Unique,
		"group_by":   "channel",
		"breakdown":  out,
	}

	if channel != "" {
		resp["channel"] = channel
	}

	writeJSON(w, http.StatusOK, resp)
}

// parseUnixParam accepts seconds or milliseconds.
// return ok=false if empty.
func parseUnixParam(v string) (t time.Time, ok bool, err error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, false, nil
	}

	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return time.Time{}, false, err
	}

	// millis heuristic
	if n >= 1_000_000_000_000 {
		return time.UnixMilli(n).UTC(), true, nil
	}

	return time.Unix(n, 0).UTC(), true, nil
}

// compile guard (remove if unused in your project)
var _ = repo.MetricsTotals{}

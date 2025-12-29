package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MetricsRepo struct {
	pool *pgxpool.Pool
}

func NewMetricsRepo(pool *pgxpool.Pool) *MetricsRepo {
	return &MetricsRepo{pool: pool}
}

type MetricsTotals struct {
	Total  int64
	Unique int64
}

type MetricsByChannelRow struct {
	Channel string
	Total   int64
	Unique  int64
}

// channel optional: pass "" to not filter.
func (r *MetricsRepo) Totals(ctx context.Context, eventName string, from, to time.Time, channel string) (MetricsTotals, error) {
	const q = `
SELECT
  COUNT(*)::bigint AS total,
  COUNT(DISTINCT user_id)::bigint AS unique
FROM events
WHERE event_name = $1
  AND ts >= $2
  AND ts <  $3
  AND ($4 = '' OR channel = $4);
`
	var out MetricsTotals
	err := r.pool.QueryRow(ctx, q, eventName, from, to, channel).Scan(&out.Total, &out.Unique)
	return out, err
}

// channel optional: pass "" to not filter.
func (r *MetricsRepo) ByChannel(ctx context.Context, eventName string, from, to time.Time, channel string) ([]MetricsByChannelRow, error) {
	const q = `
SELECT
  channel,
  COUNT(*)::bigint AS total,
  COUNT(DISTINCT user_id)::bigint AS unique
FROM events
WHERE event_name = $1
  AND ts >= $2
  AND ts <  $3
  AND ($4 = '' OR channel = $4)
GROUP BY channel
ORDER BY channel;
`
	rows, err := r.pool.Query(ctx, q, eventName, from, to, channel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MetricsByChannelRow
	for rows.Next() {
		var rrow MetricsByChannelRow
		if err := rows.Scan(&rrow.Channel, &rrow.Total, &rrow.Unique); err != nil {
			return nil, err
		}
		out = append(out, rrow)
	}
	return out, rows.Err()
}

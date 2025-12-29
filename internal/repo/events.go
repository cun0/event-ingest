package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/cun0/insider-case/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EventRepo struct {
	pool *pgxpool.Pool
}

func NewEventRepo(pool *pgxpool.Pool) *EventRepo {
	return &EventRepo{pool: pool}
}

func (r *EventRepo) InsertOne(ctx context.Context, e domain.Event) (inserted bool, err error) {
	const q = `
	INSERT INTO events (dedup_key, event_name, channel, campaign_id, user_id, ts, tags, metadata)
	VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8::jsonb)
	ON CONFLICT (dedup_key) DO NOTHING
	RETURNING 1;
`

	var one int
	err = r.pool.QueryRow(ctx, q,
		e.DedupKey,
		e.EventName,
		e.Channel,
		e.CampaignID,
		e.UserID,
		e.Timestamp,
		e.Tags,
		toJSONBText(e.Metadata),
	).Scan(&one)

	if err == nil {
		return true, nil
	}
	if err == pgx.ErrNoRows {
		return false, nil
	}
	return false, err
}

func (r *EventRepo) InsertBatch(ctx context.Context, events []domain.Event) (map[string]struct{}, error) {
	inserted := make(map[string]struct{})
	if len(events) == 0 {
		return inserted, nil
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sql, args := buildInsertBatchSQL(events)

	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		inserted[k] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return inserted, nil
}

func buildInsertBatchSQL(events []domain.Event) (string, []any) {
	var b strings.Builder
	// 8 params per event.
	args := make([]any, 0, len(events)*8)

	b.WriteString(`
	INSERT INTO events (dedup_key, event_name, channel, campaign_id, user_id, ts, tags, metadata)
	VALUES
`)

	argPos := 1
	for i, e := range events {
		if i > 0 {
			b.WriteString(",\n")
		}

		b.WriteString(fmt.Sprintf(
			"($%d,$%d,$%d,NULLIF($%d,''),$%d,$%d,$%d,$%d::jsonb)",
			argPos, argPos+1, argPos+2, argPos+3, argPos+4, argPos+5, argPos+6, argPos+7,
		))

		args = append(args,
			e.DedupKey,
			e.EventName,
			e.Channel,
			e.CampaignID,
			e.UserID,
			e.Timestamp,
			e.Tags,
			toJSONBText(e.Metadata),
		)

		argPos += 8
	}

	b.WriteString(`
	ON CONFLICT (dedup_key) DO NOTHING
	RETURNING dedup_key;
`)

	return b.String(), args
}

func toJSONBText(raw []byte) string {
	if len(raw) == 0 {
		return `{}`
	}
	return string(raw)
}

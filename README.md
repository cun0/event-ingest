# Event Ingestion Service

A minimal, deterministic event ingestion service focused on correctness, idempotency, and low-latency writes under high throughput.

## Architecture Overview

<img width="991" height="703" alt="Screenshot 2025-12-29 at 02 01 18" src="https://github.com/user-attachments/assets/0690b207-c60f-44c0-9d66-97b5effc8dd5" />



## High-Level Design

The system accepts events over HTTP, performs deterministic deduplication, and persists them durably using PostgreSQL.  
Write throughput is optimized via a single-writer, group-commit ingestion path, while preserving strict durability guarantees.

---

## Key Features

| Feature | Description |
|------|-------------|
| Idempotent ingestion | Deterministic `dedup_key` + PostgreSQL `UNIQUE` constraint + `ON CONFLICT DO NOTHING` |
| Low-latency writes | Group-commit via a single writer with a short batching window |
| Durable acknowledgements | `/events` returns success only after DB commit (not fire-and-forget) |
| Canonicalization | Tags are order-insensitive; metadata is normalized for stable hashing |
| Bulk ingest | `/events/bulk` with chunked batch inserts |
| Out-of-the-box run | Docker Compose, migrations, and Make targets |

---

## Key Implementation Details

### Batching & Writes

- `/events` requests are enqueued and processed by a single writer (group-commit).
- The writer flushes when either `BATCH_WINDOW` elapses or `MAX_BATCH` is reached.
- A flush is a single database transaction: batch insert (`INSERT ... ON CONFLICT DO NOTHING`) followed by commit.
- HTTP responses are returned only after the commit succeeds (not fire-and-forget).
- The queue is bounded: requests wait up to the request timeout, otherwise an error is returned (bounded latency).

### Bulk & Metrics

- `/events/bulk` bypasses the queue and writes directly in larger batches (chunked to avoid PostgreSQL parameter limits).
- `GET /metrics` is served via direct SQL aggregation queries (`COUNT`, `COUNT DISTINCT`, `GROUP BY`).

---

## Idempotency

Idempotency is enforced via a deterministic `dedup_key` and a unique constraint in PostgreSQL.

The `dedup_key` is derived from:
- `event_name`, `channel`, `campaign_id`, `user_id`
- a normalized timestamp (millisecond resolution to avoid sec/ms representation differences)
- normalized `tags` (trimmed, deduplicated, sorted; order-insensitive)
- normalized `metadata` (canonical JSON; whitespace and key order do not affect the key)

---

## Performance & Tuning

- Batch size: 800 events  
- Flush window: 2ms (configurable)  
- Queue capacity: 50,000 events (bounded backpressure)  

---

## Ingestion Performance Approach (Low Latency, Durable Writes)

### Why not “one request = one commit”?
At high throughput, committing per request becomes the primary bottleneck. The goal is to reduce commit overhead without sacrificing correctness or durability.

### Group-commit via a single writer
`POST /events` uses a single-writer, group-commit model: validated events are briefly buffered, flushed as a batch in a single transaction, and acknowledged only after commit.

### Durability (not fire-and-forget)
The in-memory queue is used strictly for batching, not for durability. A `200` response from `/events` guarantees the event has been durably written to PostgreSQL. Events that were queued but not yet committed may be lost on process crash, and those requests would not have received a success response.

### Why this pattern?
This approach reduces commit pressure while keeping ingestion deterministic. Multiple requests share a single commit, preserving durability and correctness while keeping the implementation small and easy to reason about.

---

## Why PostgreSQL?

PostgreSQL is chosen for deterministic ingestion correctness within this scope:
- Explicit transactional semantics and durability via WAL
- Simple idempotency enforcement using a unique constraint
- Efficient metrics queries with standard indexing

A custom file-based WAL or append-only log is a valid alternative, but implementing it correctly (fsync policy, rotation, recovery, corruption handling, backpressure) significantly increases scope and risk. Here, durability is delegated to PostgreSQL’s WAL, allowing the application to focus on group-commit and deterministic deduplication.

---

## Why not ClickHouse (in this implementation)?

ClickHouse excels at analytics, but exact idempotency and deduplication are merge-dependent and eventually consistent. For this case, deterministic ingestion correctness is prioritized.  
In a production setup, ClickHouse would be a natural downstream analytics sink (e.g. via CDC or an outbox pattern from PostgreSQL).

---

## Alternative Approaches Considered

- **Commit per request:** simplest approach, but does not scale due to commit overhead.
- **Custom file-based WAL:** viable, but significantly more complex for this scope.
- **ClickHouse-first:** strong for analytics, weaker for strict ingestion correctness.

---

## TODO (Next Steps)

- Expand unit / integration tests (timestamp edge cases, canonicalization logic, duplicate delivery cases)
- Add Prometheus metrics and some basic dashboards (queue depth, batch sizes, flush latency, p95/p99 request latency)
- Add some API hardening (request size limits, rate limiting, simple auth via API key)
- Improve `/events/bulk` response  and extend the current load-test target


## Production-Grade Considerations

For a production-grade setup, this design would probably need a few extensions:
- Proper admission control and backpressure based on SLOs, not just a fixed queue size (with alerting on queue depth / latency)
- Multiple writers sharded by `dedup_key` to increase sustained throughput
- Partitioning and retention policies for the events table to keep writes manageable over time
- A separate analytics pipeline (e.g. outbox or CDC into ClickHouse), keeping ingestion correctness isolated
- More operational work around deploy strategy, DB migrations in CI/CD, and basic runbooks


## How to run

## Quick start
Requirements: Docker + Docker Compose

```bash
make up
make curl-health
make curl-event
make curl-metrics
make curl-bulk
make duplicate-test

(These commands act as quick smoke tests to verify ingestion, idempotency, and metrics end-to-end.)
```

Logs:
```bash
make logs
```

Stop (and remove volumes):
```bash
make down
```

Local run (API outside Docker) is also supported if you have Postgres running:
```bash
export PORT=8080
export DATABASE_URL="postgres://insider:pass@localhost:5432/insider?sslmode=disable"
go run ./cmd/api
```

---

## API

### POST /events
Accepts and processes a single event payload.

Request body:
```json
{
  "event_name": "purchase",
  "channel": "mobile",
  "campaign_id": "cmp-1",
  "user_id": "u1",
  "timestamp": 1735345600,
  "tags": ["a", "b"],
  "metadata": { "amount": 10, "currency": "TRY" }
}
```

Response:
```json
{ "status": "inserted", "dedup_key": "..." }
```
or
```json
{ "status": "duplicate", "dedup_key": "..." }
```

Validation notes:
- `timestamp` accepts unix seconds (10 digits) or unix milliseconds (13 digits).
- `timestamp` must not be in the future (small clock skew tolerated).
- `metadata` must be valid JSON if present.
- `tags` may be empty (`[]`) and is stored as an empty array (never `NULL`).

---

### GET /metrics
Returns aggregated metric data over a time range.

Query params:
- `event_name` (required)
- `from` (optional, unix sec/ms; default `to - 1h`)
- `to` (optional, unix sec/ms; default `now`)
- `channel` (optional filter)

Response includes:
- `total` = total events
- `unique` = distinct `user_id`
- breakdown by `channel` (extra dimension)

---

### POST /events/bulk
Body: JSON array of `/events` payloads.

Response (counts only):
```json
{
  "received": 1000,
  "processed": 980,
  "inserted": 900,
  "duplicate": 80,
  "invalid": 20,
  "batch_fail": 0
}
```


APP_NAME=insider-case

.PHONY: up down logs ps rebuild test run curl-health curl-event curl-metrics

up:
	docker compose up -d --build

down:
	docker compose down -v

rebuild:
	docker compose build --no-cache

ps:
	docker compose ps

logs:
	docker compose logs -f api

test:
	go test ./...

run:
	PORT=8080 DATABASE_URL="postgres://insider:pass@localhost:5432/insider?sslmode=disable" \
	LOG_LEVEL=INFO REQUEST_TIMEOUT=3s BATCH_WINDOW=2ms MAX_BATCH=800 QUEUE_SIZE=50000 \
	go run ./cmd/api

curl-health:
	curl -s localhost:8080/healthz && echo

curl-event:
	@now=$$(date +%s); \
	curl -s -X POST localhost:8080/events \
	  -H 'Content-Type: application/json' \
	  -d "{\"event_name\":\"purchase\",\"channel\":\"mobile\",\"campaign_id\":\"cmp-1\",\"user_id\":\"u1\",\"timestamp\":$$now,\"tags\":[\"a\",\"b\"],\"metadata\":{\"amount\":10,\"currency\":\"TRY\"}}" \
	| jq '.' || cat

curl-bulk:
	@now=$$(date +%s); \
	curl -s -X POST localhost:8080/events/bulk \
	  -H 'Content-Type: application/json' \
	  -d "[{\"event_name\":\"purchase\",\"channel\":\"mobile\",\"user_id\":\"u1\",\"timestamp\":$$now},{\"event_name\":\"purchase\",\"channel\":\"web\",\"user_id\":\"u2\",\"timestamp\":$$now}]" \
	| jq '.' || cat

curl-metrics:
	@from=$$(( $$(date +%s) - 3600 )); to=$$(date +%s); \
	curl -s "localhost:8080/metrics?event_name=purchase&from=$$from&to=$$to" \
	| jq '.' || cat


duplicate-test:
	@echo "Testing idempotency (sending same event twice)..."
	@now=$$(date +%s); \
	payload="{\"event_name\":\"duplicate_test\",\"channel\":\"test\",\"user_id\":\"test-user\",\"timestamp\":$$now}"; \
	echo "First request:"; \
	curl -s -X POST localhost:8080/events -H 'Content-Type: application/json' -d "$$payload" | jq '.' || cat; \
	echo "\nSecond request (should be duplicate):"; \
	curl -s -X POST localhost:8080/events -H 'Content-Type: application/json' -d "$$payload" | jq '.' || cat

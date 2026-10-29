.PHONY: run test generate db

run:
	set -a; . ./.env; set +a; go run ./cmd/api

# Integration tests need TEST_DATABASE_URL (a disposable database; its schema is dropped).
test:
	go vet ./... && go test ./...

generate:
	go tool sqlc generate

db:
	docker compose up -d postgres

TEST_DATABASE_URL ?= postgres://cellarkeep:cellarkeep@localhost:5433/cellarkeep_test

.PHONY: build test test-db lint generate db-up

build:
	go build ./...

test:
	go test ./internal/config/ ./internal/domain/ ./internal/auth/

test-db:
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test ./...

lint:
	go vet ./...
	go tool golangci-lint run

generate:
	go generate ./...

db-up:
	docker compose up -d db db-test

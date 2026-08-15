GO ?= go
APP_NAME ?= printa-api
BIN_DIR ?= bin
APP_PORT ?= 18080

.PHONY: help fmt format-check vet test build run verify-migrations migrate-up migrate-down openapi-check check clean

help:
	@printf '%s\n' \
	  'Printa backend development commands:' \
  '  make fmt               Format Go source files' \
  '  make format-check      List Go files that require formatting' \
  '  make vet               Run go vet' \
	  '  make test              Run all Go tests' \
	  '  make build             Build bin/printa-api' \
	  '  make run               Run the API using the current environment' \
	  '  make verify-migrations Verify numbered up/down migration pairs' \
	  '  make migrate-up        Apply migrations (requires DATABASE_URL and migrate CLI)' \
	  '  make migrate-down      Roll back one migration (requires DATABASE_URL and migrate CLI)' \
	  '  make openapi-check     Verify the embedded OpenAPI contract exists' \
	  '  make check             Run the standard pre-commit quality checks' \
	  '  make clean             Remove locally built binaries'

fmt:
	$(GO) fmt ./...

format-check:
	@unformatted="$$(gofmt -l $$(find . -type f -name '*.go' -not -path './vendor/*'))"; \
	if [ -n "$$unformatted" ]; then \
		printf 'The following Go files need formatting:\n%s\n' "$$unformatted"; \
		exit 1; \
	fi

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(APP_NAME) ./cmd/api

run:
	APP_PORT=$(APP_PORT) $(GO) run ./cmd/api

verify-migrations:
	@./scripts/verify_migrations.sh

migrate-up:
	@: $${DATABASE_URL:?DATABASE_URL must be set}
	migrate -path ./migrations -database "$$DATABASE_URL" up

migrate-down:
	@: $${DATABASE_URL:?DATABASE_URL must be set}
	migrate -path ./migrations -database "$$DATABASE_URL" down 1

openapi-check:
	@test -s internal/apidocs/openapi.yaml
	@grep -q '^openapi: 3.0.3' internal/apidocs/openapi.yaml

check: vet test verify-migrations openapi-check build

clean:
	rm -rf $(BIN_DIR)

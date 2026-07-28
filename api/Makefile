.PHONY: build run test lint fmt tidy migrate-up migrate-down migrate-new docker-up docker-down

APP_NAME = cadence-api
BUILD_DIR = bin
MIGRATIONS_PATH ?= migrations

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/api

run: build
	./$(BUILD_DIR)/$(APP_NAME)

test:
	go test ./... -race -count=1

lint:
	@test -z "$$(gofmt -l .)" || { echo "gofmt: files need formatting:"; gofmt -l .; exit 1; }
	go vet ./...
	golangci-lint run ./...

fmt:
	gofumpt -w .

tidy:
	go mod tidy

# The migration chain owns the schema and the RLS policies. DATABASE_URL must
# point at the target database; the Supabase dashboard is never the editor.
migrate-up:
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" down 1

migrate-new:
	migrate create -ext sql -dir $(MIGRATIONS_PATH) -seq $(NAME)

# Local Postgres for development. Tests bring their own via testcontainers.
docker-up:
	docker compose up -d

docker-down:
	docker compose down

.PHONY: build run test test-integration gate openapi dev-up fmt tidy migrate-up migrate-down migrate-new docker-up docker-down

APP_NAME = cadence-api
BUILD_DIR = bin

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/api

run: build
	./$(BUILD_DIR)/$(APP_NAME)

# A convenience subset. `gate` is the authority on whether this module is green.
test:
	go test ./... -race -count=1

# The integration suite brings up a real Postgres through testcontainers, so it
# needs a Docker daemon. It sits behind a build tag rather than inside the fast
# gate: a developer without Docker still gets a green `gate`, and CI runs this
# as a required job of its own. Green here is not optional — it is where the
# migration chain and the role separation are actually proven.
test-integration:
	go test -tags=integration ./... -count=1

# gofmt -> build -> vet -> golangci-lint -> test -race. The same script CI and
# the Ralph loop run, so there is one definition of green and not three.
gate:
	../scripts/gate/go.sh

# Regenerate the committed contract. A test fails when this has not been run
# after a type changed, because openapi.json is what the Kotlin client and the
# dashboard client are generated from — a stale one renames their fields.
openapi:
	go run ./cmd/openapi -out openapi.json

fmt:
	gofumpt -w .

tidy:
	go mod tidy

# The migration chain owns the schema and the RLS policies; the Supabase
# dashboard is never the editor.
#
# These run cmd/migrate rather than the external `migrate` CLI. The CLI was a
# tool nobody had installed — not the gate, not CI, not a new machine — so the
# chain could only be applied by whoever happened to have it. The connection
# string is DATABASE_MIGRATION_URL, the owner role: DATABASE_URL is the request
# path and owns nothing.
migrate-up:
	go run ./cmd/migrate up

# Rolls back one migration. `go run ./cmd/migrate down all` unwinds the chain.
migrate-down:
	go run ./cmd/migrate down

migrate-new:
	go run ./cmd/migrate new "$(NAME)"

# Local Postgres for development. Tests bring their own via testcontainers.
docker-up:
	docker compose up -d

docker-down:
	docker compose down

# One command for a usable local stack: Postgres, GoTrue, the chain applied and
# a password on the request-path role. Requires DATABASE_MIGRATION_URL — see
# .env.example.
#
# The last step exists because the chain creates cadence_app without a password
# on purpose: the credential is provisioned outside the repository, in Railway
# and Supabase variables. A developer machine has no such place, so this target
# plays that part. It is local-only, and the password it sets is the one in
# .env.example — never run it against a deployed database.
dev-up: docker-up
	@until docker compose exec -T postgres pg_isready -U cadence -d cadence >/dev/null 2>&1; do sleep 1; done
	$(MAKE) migrate-up
	docker compose exec -T postgres psql -q -U cadence -d cadence \
		-c "ALTER ROLE cadence_app PASSWORD 'cadence_app'"
	@until curl -fsS http://localhost:9999/health >/dev/null 2>&1; do sleep 1; done
	@echo
	@echo "local stack ready:"
	@echo "  postgres  localhost:5433"
	@echo "  gotrue    localhost:9999  (JWKS at /.well-known/jwks.json)"
	@echo
	@echo "the API needs AUTH_JWT_ISSUER=http://localhost:9999 — see .env.example"

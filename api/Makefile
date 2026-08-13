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
	go test -tags=integration ./... -race -count=1

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

# One command for a usable local stack: Postgres, GoTrue, the chain applied,
# passwords on the two LOGIN roles and the identity provider joined to the hook.
# Requires DATABASE_MIGRATION_URL.
#
# The password steps exist because the chain creates both LOGIN roles without
# one on purpose: the credential is provisioned outside the repository, in the
# deployment's variable store. A developer machine has no such place, so this
# target plays that part. It is local-only, and the passwords it sets are the
# ones this target sets — never run it against a deployed database.
#
# The membership is here rather than in docker-compose.yml for an ordering
# reason: cadence_auth_hook is created by migration 000005, and compose brings
# GoTrue up before any migration has run. It is granted after migrate-up and
# needs no restart — a membership is resolved when the function is called, and
# GoTrue calls the hook only while issuing a token.
#
# The same grant is the deployment's last provisioning step, and it is not
# optional: measured against supabase/gotrue:v2.194.0 on 2026-08-10, revoking it
# makes GoTrue answer POST /token with 500 and `permission denied for schema app`
# (SQLSTATE 42501). Sign-in stops for everybody rather than tokens quietly
# losing their product role — the loud failure, which is the one to want.
#
# The window between `docker compose up` and this line ends in the same 500, and
# it has two halves. Before `migrate-up` the schema does not exist yet and the
# log says 3F000. After it, and before this line, the log says exactly what a
# revoked grant says — 42501, permission denied for schema app — because that is
# exactly what it is. Worth knowing that the two are indistinguishable: the
# larger half of the window looks like the failure above, and only the order of
# events tells them apart.
#
# ON_ERROR_STOP is not decoration. psql given several -c runs them all and exits
# 0 after any of them fails, so without it a failed GRANT — a role that was never
# created, a chain applied only in part — reaches "local stack ready" and is
# discovered at the first sign-in, three steps later. The last statement is the
# check rather than another change: it asserts the privilege this target exists
# to provision, and 1/0 is how a psql statement is made to fail on a false one.
dev-up: docker-up
	@until docker compose exec -T postgres pg_isready -U cadence -d cadence >/dev/null 2>&1; do sleep 1; done
	$(MAKE) migrate-up
	docker compose exec -T postgres psql -q -v ON_ERROR_STOP=1 -U cadence -d cadence \
		-c "ALTER ROLE cadence_app PASSWORD 'cadence_app'" \
		-c "ALTER ROLE cadence_service_app PASSWORD 'cadence_service_app'" \
		-c "GRANT cadence_auth_hook TO cadence_gotrue" \
		-c "SELECT 1/has_function_privilege('cadence_gotrue', 'app.access_token_hook(pg_catalog.jsonb)', 'EXECUTE')::int"
	@n=0; until curl -fsS http://localhost:9999/health >/dev/null 2>&1; do \
		n=$$((n+1)); \
		if [ $$n -ge 60 ]; then \
			echo "gotrue did not come up in 60s — try: docker compose logs auth" >&2; \
			exit 1; \
		fi; \
		sleep 1; \
	done
	@echo
	@echo "local stack ready:"
	@echo "  postgres  localhost:5433"
	@echo "  gotrue    localhost:9999  (JWKS at /.well-known/jwks.json)"
	@echo
	@echo "the API needs AUTH_JWT_ISSUER, AUTH_JWT_SESSION_KIDS and AUTH_JWT_ADMIN_KID; config.Load refuses to start without them"

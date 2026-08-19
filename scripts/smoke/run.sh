#!/usr/bin/env bash
# The critical path through a browser, against the whole stack on this machine.
#
# Not part of scripts/gate/all.sh and not a CI job: what CI would need is a deployment, and that is
# held by SKL-01 and SKL-06. A job that can only be red says nothing, so this is a command a person
# runs — and the spec says so rather than implying it.
#
# Everything below the browser is the real thing: Postgres and GoTrue in compose, the provisioner and
# the API as processes, the dashboard's dev server started by Playwright. The values here are the
# ones api/docker-compose.yml already commits; none of them is a secret and none of them is a
# deployment's.
set -euo pipefail

cd "$(dirname "$0")/../.."

export DATABASE_MIGRATION_URL="postgres://cadence:cadence@localhost:5433/cadence?sslmode=disable"
export DATABASE_URL="postgres://cadence_app:cadence_app@localhost:5433/cadence?sslmode=disable"
export DATABASE_SERVICE_URL="postgres://cadence_service_app:cadence_service_app@localhost:5433/cadence?sslmode=disable"
export AUTH_JWT_ISSUER="http://localhost:9999"
export AUTH_JWT_AUDIENCE="authenticated"
# The signing key of the two in GOTRUE_JWT_KEYS, and the admin key that must never be a session key.
export AUTH_JWT_SESSION_KIDS="a1323fc3-c42d-37ae-8be6-1ebfb524403f"
export AUTH_JWT_ADMIN_KID="xr8-HA0zUfG0zdB40mJdFNwX3DvkinwdnhXB1g2W_Dw"
export PROVISIONER_URL="http://localhost:8081"
export PROVISIONER_SHARED_SECRET="local-development-only-not-a-secret-32ch"
export PROVISIONER_ENVIRONMENT="development"
export PROVISIONER_GOTRUE_URL="http://localhost:9999"
export PROVISIONER_GOTRUE_ADMIN_JWK='{"kty":"EC","crv":"P-256","alg":"ES256","use":"sig","kid":"xr8-HA0zUfG0zdB40mJdFNwX3DvkinwdnhXB1g2W_Dw","x":"rtiaY2jMvLAFdxMFASN2GZ5EigwL4ka3TjAb4Yz_wE8","y":"ye74OWWh2AjB4jPLni3z3OIpByWz2QCh-u7N7DachxM","d":"XPA_FXtwbdWb0sxa9Dr7MZdKQboGjZA4jfvFhfvTIBA"}'
export SEED_ENVIRONMENT="development"
export SEED_PASSWORD="${SEED_PASSWORD:-a-seeded-password-nobody-uses}"
export CORS_ALLOWED_ORIGINS="http://localhost:5173"

# The addresses the run creates, and the one thing the teardown looks for.
export SMOKE_RUN_ID="${SMOKE_RUN_ID:-$(date +%s)}"

started=()
stop() {
    for pid in "${started[@]:-}"; do
        [ -n "$pid" ] && kill "$pid" 2>/dev/null || true
    done
}
trap stop EXIT

echo "==> the database and the identity provider"
make -C api dev-up >/dev/null

# The clinic's own component, and the API in front of it. As processes rather than compose services:
# neither has an image in this repository, and `go run` is what the Makefile already uses for the
# commands beside them.
echo "==> the provisioner"
(cd api && go run ./cmd/provisioner >/tmp/cadence-smoke-provisioner.log 2>&1) &
started+=($!)

echo "==> the API"
(cd api && go run ./cmd/api >/tmp/cadence-smoke-api.log 2>&1) &
started+=($!)

# Waited for rather than slept through: `go run` compiles first, and how long that takes depends on
# the build cache rather than on anything worth guessing.
#
# Any answer at all counts as listening, and for the provisioner that is the only thing that can:
# it serves no health route on purpose — a sixth path is a sixth thing reachable by whoever reaches
# it — so a refusal is what proves the port is answering. `-f` is therefore absent here, and the
# status is read instead: with it, a 401 would look exactly like nothing listening.
listening() {
    local url=$1 name=$2

    for _ in $(seq 1 90); do
        if [ "$(curl -s -o /dev/null -w '%{http_code}' --max-time 2 "$url")" != "000" ]; then
            return 0
        fi

        sleep 1
    done

    echo "$name did not come up — see /tmp/cadence-smoke-*.log" >&2

    return 1
}

listening "http://localhost:8081/invite" "the provisioner"
listening "http://localhost:8080/healthz" "the API"

echo "==> the clinic"
# The administrator first: staff are created by one, and migration 000006 refuses role='admin' from
# the service path whoever asks. Repeating a run finds them already there and says so.
ask() {
    curl -fsS -X POST "http://localhost:8081$1" \
        -H "X-Cadence-Provisioner-Secret: $PROVISIONER_SHARED_SECRET" \
        -H 'Content-Type: application/json' -d "$2"
}

account_in() { python3 -c 'import json,sys; print((json.load(sys.stdin).get("account") or {}).get("id",""))'; }

# Looked up before it is invited, because an invitation of an address the provider already holds is
# refused — and a second run of this script is the ordinary case, not the exception.
admin=$(ask /users/lookup '{"email":"admin@clinic.example"}' | account_in)
if [ -z "$admin" ]; then
    admin=$(ask /invite '{"email":"admin@clinic.example"}' | account_in)
fi

(cd api && go run ./cmd/bootstrap-admin "$admin" "Пётр Аверин") || echo "    (the clinic already has an administrator)"

ask /users/password "{\"id\":\"$admin\",\"password\":\"$SEED_PASSWORD\"}" >/dev/null

(cd api && go run ./cmd/seed)

echo "==> the smoke test"
(cd web && npm run --silent smoke)

#!/usr/bin/env bash
# Quality gate for api/. The same script runs locally, in CI and inside the
# Ralph loop — one definition of "green", or the three drift apart.
#
# Order is deliberate: formatting first because it is the cheapest signal,
# then compilation, then static analysis, then the tests.
set -euo pipefail

cd "$(dirname "$0")/../../api"

echo "==> gofmt"
unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
    echo "these files are not formatted (run 'gofumpt -w .'):"
    echo "$unformatted"
    exit 1
fi

echo "==> go build"
go build ./...

echo "==> go vet"
go vet ./...

# The seam used to compose `SET LOCAL ROLE <name>`; it now passes the role as a
# bound parameter, and this check is what keeps the next edit from composing one
# back in. gosec's G201/G202 would not see it: those rules match on SQL keywords
# — SELECT, INSERT, FROM and friends — so a concatenated SET or GRANT is
# invisible to them. This check matches on no keywords at all. It requires the
# text of every statement to come from constants, whatever the statement says.
#
# The harness is out of scope: it builds identifiers from its own constants and
# counters, never from a token, and a build tag already keeps it out of a normal
# build while depguard fails the gate if production code imports it anyway.
echo "==> sql authorship"
go run ./cmd/sqlauthorship -skip testsupport ./internal ./cmd

# The trust boundary, and the only guard left holding it now that provisioner
# shares this module (2026-08-14). GoTrue accepts an admin token signed by any
# key it is configured with, so the process holding that key can create, delete
# and re-password any account — which is why it is a component of its own, and
# why the API may not so much as read the variable it arrives in.
#
# A grep and not a linter: what has to be absent is a string, and it can enter
# through os.Getenv, a manifest, a comment that invites the next person to add
# it, or a log line naming the variable.
#
# The control below is not optional. If provisioner renames the variable, this
# check would go on searching for a string nothing spells any more — green, and
# watching nothing. That failure is silent, which is the one a gate cannot have.
#
# It reads the component's source and not its tests, because a rename lands in
# config.go while a fixture keeps the old spelling for a while: measured, that
# is exactly the state in which a whole-directory grep stays green and this rule
# is already watching a dead string.
echo "==> the admin key stays out of the API"
admin_key_variable=PROVISIONER_GOTRUE_ADMIN_JWK

if ! grep -rq --include='*.go' --exclude='*_test.go' "$admin_key_variable" cmd/provisioner; then
    echo "cmd/provisioner does not mention $admin_key_variable: either the component stopped" >&2
    echo "reading the admin key, or the variable was renamed and the rule below now" >&2
    echo "searches for a string nothing uses. Fix this check before trusting it." >&2
    exit 1
fi

if grep -rn "$admin_key_variable" cmd/api internal; then
    echo >&2
    echo "the admin key's variable is named above, inside the API." >&2
    echo "GoTrue admits an admin token signed by any configured key, so a process that can" >&2
    echo "read this variable can create, delete and re-password every account in the clinic." >&2
    echo "It belongs to cmd/provisioner alone; the API asks that component instead." >&2
    exit 1
fi

echo "==> golangci-lint"
if command -v golangci-lint >/dev/null 2>&1; then
    golangci-lint run ./...
else
    echo "golangci-lint is not installed — install it before trusting this gate" >&2
    exit 1
fi

echo "==> go test -race"
go test ./... -race -count=1

echo "go gate: green"

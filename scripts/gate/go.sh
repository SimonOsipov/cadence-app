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

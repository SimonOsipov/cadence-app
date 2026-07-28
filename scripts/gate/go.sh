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

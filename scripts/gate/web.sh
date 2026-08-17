#!/usr/bin/env bash
# Quality gate for web/. The same script runs locally and in CI — one definition
# of "green", or the two drift apart.
#
# Order is the cheapest signal first: the toolchain, then types, then static
# analysis, then the tests, then the build that has to survive all of them.
set -euo pipefail

cd "$(dirname "$0")/../../web"

echo "==> node"
if ! command -v node >/dev/null 2>&1; then
    echo "node is not installed — install the version in web/.nvmrc" >&2
    exit 1
fi

pinned=$(tr -d '[:space:]' < .nvmrc)
running=$(node --version | sed 's/^v//')
pinned_major=${pinned%%.*}
running_major=${running%%.*}

# The major alone. A patch difference is noise; a major is the line along which
# Node removes APIs, and react-router already refuses anything below 22.22.
if [ "$running_major" != "$pinned_major" ]; then
    echo "node is $running and web/.nvmrc pins $pinned — the gate would be measuring" >&2
    echo "a runtime nobody else runs. Switch with 'nvm use' before trusting it." >&2
    exit 1
fi

echo "==> npm ci"
# ci rather than install: the lockfile is the pinned dependency set, and an
# install that quietly resolves something newer makes this gate a different gate
# from the one CI runs.
npm ci

echo "==> tsc"
npm run --silent typecheck

# The prototype is in-browser Babel JSX that never compiles. Excluded in
# tsconfig.json — and the exclusion is asserted here rather than trusted, because
# a widened `include` is a one-word edit and the failure it causes reads as a
# broken prototype rather than as a broken config.
#
# The file list is captured and checked for being non-empty before it is
# searched. Piped straight into `grep -q` this reports success when tsc itself
# fails — the consumer finds nothing, and «found nothing» is what a passing check
# looks like. It is also the shape that takes SIGPIPE once the list outgrows the
# pipe buffer, and the list is already 35KB.
echo "==> the frozen prototype is outside the TypeScript project"
typescript_files=$(npx tsc --noEmit --listFilesOnly)
if [ -z "$typescript_files" ]; then
    echo "tsc listed no files at all, so this check measured nothing" >&2
    exit 1
fi
# grep -c and not grep -q, for the reason the paragraph above gives: -q exits on
# the first hit, the writer takes SIGPIPE, and pipefail turns that into a status
# the `if` reads as «found nothing». Measured on this list: detected at 58KB,
# missed at 64KB, and the list is already 35KB.
#
# The count is checked for being a number before it is compared, because `[ "" -gt
# 0 ]` is a status of 2 and an `if` reads that as «no», which is the same silence
# this whole block is about. Unreachable above — the emptiness guard already ran —
# and written out so the shape stays the one changed-stacks.sh uses.
inside_prototype=$(echo "$typescript_files" | grep -cE '/web/prototype/' || true)
case $inside_prototype in
    '' | *[!0-9]*)
        echo "counting the prototype's files produced '$inside_prototype', so nothing was measured" >&2
        exit 1
        ;;
esac
if [ "$inside_prototype" -gt 0 ]; then
    echo "tsconfig.json reaches web/prototype — it is frozen and must not compile" >&2
    exit 1
fi

# The same question of Vitest, and asked of the runner rather than of its
# configuration: a vitest.config.ts added later takes precedence over the `test`
# block in vite.config.ts, and `test.projects` can re-add a path — both leave an
# assertion about that block green while the runner collects something else.
echo "==> Vitest collects nothing outside src/"
collected=$(npx vitest list --filesOnly)
if [ -z "$collected" ]; then
    echo "vitest collected no files at all, so this check measured nothing" >&2
    exit 1
fi
# The paths come back relative to this directory, so the anchor is `^src/` and
# not a path fragment — measured, `vitest list` prints `src/app.test.tsx`.
outside=$(echo "$collected" | grep -v '^src/' || true)
if [ -n "$outside" ]; then
    echo "Vitest collects files outside src/:" >&2
    printf '  %s\n' "$outside" >&2
    echo "step 5's Playwright spec lives in tests/ and would be run by the wrong runner" >&2
    exit 1
fi

# The CSS is the source and tokens.ts is its output; the module is committed so it
# is readable in a diff and importable without a build step, and this is what keeps
# that copy from becoming a second source of truth.
echo "==> the token module matches the stylesheet"
node scripts/generate-tokens.ts --check

# Derived from the prototype rather than retyped, and re-derived here so the set
# cannot drift from the screens it was taken off. The other direction — every icon
# in the set is actually drawn by this application — arrives with the Overview in
# step 3; today the application draws none, and a check that must be switched off
# to pass is not a check.
echo "==> the icon subset matches the prototype"
node scripts/derive-icons.ts --check

echo "==> eslint"
npm run --silent lint

echo "==> vitest"
npm run --silent test

echo "==> vite build"
npx vite build

# The build does not fail on an asset it could not resolve — it warns and exits 0.
# See the script's own header for the measurement.
echo "==> every asset the build points at is in the build"
node scripts/check-build-assets.mjs

echo "web gate: green"

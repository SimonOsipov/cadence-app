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
echo "==> the frozen prototype is outside the TypeScript project"
if npx tsc --noEmit --listFilesOnly | grep -q '/web/prototype/'; then
    echo "tsconfig.json reaches web/prototype — it is frozen and must not compile" >&2
    exit 1
fi

echo "==> eslint"
npm run --silent lint

echo "==> vitest"
npm run --silent test

echo "==> vite build"
npx vite build

echo "web gate: green"

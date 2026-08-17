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

# Both directions now: the set is what src/** draws, so an icon nothing uses cannot
# sit in the bundle, and every name it finds has to be one the prototype drew, so a
# screen cannot invent one the design never had.
echo "==> the icon subset is what the dashboard draws"
node scripts/derive-icons.ts --check

echo "==> eslint"
npm run --silent lint

# What the code that ships actually gets. The probes below prove the selectors match something; they
# cannot prove the rules reach `src/features/overview/**`, and a config block with a narrower glob takes
# them away there while both probes stay green. Every component rather than one witness, because a block
# narrower still spares whichever file the check names — measured, a block over `patient-*.tsx` and
# `triage.tsx` leaves roster.tsx reporting 8 while `var(--paper)` in patient-card.tsx is not refused.
#
# The root is `src/features` and both extensions, matching the rule block's own scope
# (`src/features/**/*.{ts,tsx}`): M6's screens land as new directories beside `overview/`, and `flags.ts`
# — the flag palette's colour table, the likeliest file in the tree to grow a `var(--…)` — is a `.ts`. `__probes__`
# is excluded because it is globally ignored, and `--print-config` on an ignored file prints the bare
# word `undefined` — the reader below then dies on a JSON parse rather than reporting a count.
echo "==> the rules reach the components, not just the probes"
# Read from a file, not a pipe, so the `exit 1` below ends the script rather than a subshell — and
# -print0 because `web/prototype/Doctor Dashboard.html` is one directory away and a word-split path
# reaches eslint as two unreadable ones.
checked=0
listing=$(mktemp)
trap 'rm -f "$listing"' EXIT

# Materialised first because a process substitution's exit status is invisible to `set -e`: a find that
# dies partway emits what it read and the loop reports a clean count over a truncated list. As a simple
# command its status is read. The same invisibility is what makes the zero guard below reachable.
find src/features \( -name '*.ts' -o -name '*.tsx' \) \
    ! -name '*.test.ts' ! -name '*.test.tsx' ! -path '*__probes__*' -print0 > "$listing"

while IFS= read -r -d '' component; do
    checked=$((checked + 1))

    # Severity first: --print-config keeps a rule's options after `off`, so counting them alone reads a
    # disabled rule as fully armed. Measured — `'no-restricted-syntax': 'off'` prints [0, ...8 selectors].
    selectors=$(npx eslint --print-config "$component" |
        node -e 'let j="";process.stdin.on("data",d=>j+=d).on("end",()=>{const r=JSON.parse(j).rules?.["no-restricted-syntax"];console.log(Array.isArray(r)&&r[0]===2?r.length-1:0)})')

    case $selectors in '' | *[!0-9]*) echo "reading the config for $component produced '$selectors'" >&2; exit 1;; esac

    if [ "$selectors" -ne 8 ]; then
        echo "$component is linted with $selectors restricted-syntax selectors, want 8 — a component is" >&2
        echo "not getting the rules the probes say exist" >&2
        exit 1
    fi
done < "$listing"

if [ "$checked" -eq 0 ]; then
    echo "no components found to check the lint rules against, so this check measured nothing" >&2
    exit 1
fi

echo "all $checked components carry the 8 restricted-syntax selectors"

# A typo in an esquery selector leaves the gate green for ever: there are no violations in the source, so
# a rule matching nothing looks exactly like a rule everything obeys. The probes violate on purpose, are
# ignored by an ordinary run, and are linted here with the count required.
echo "==> the lint rules still refuse what they are for"
for probe in aggregates:15 css-variables:2; do
    file="src/features/__probes__/${probe%%:*}.tsx"
    want="${probe##*:}"
    got=$(npx eslint --no-ignore "$file" 2>&1 | grep -c 'no-restricted-syntax' || true)

    if [ "$got" -ne "$want" ]; then
        echo "$file drew $got refusals, want $want — the rule is not refusing what it was written for" >&2
        npx eslint --no-ignore "$file" >&2 || true
        exit 1
    fi
done

# The other half of the same criterion. ESLint holds `var(--…)` to src/tokens/ for
# the files it parses; it parses no CSS, and src/fonts/fonts.css is already outside
# tokens/. stylelint was declined — a second linter and a second config for one rule
# — so the rule is a grep.
#
# One grep per file, and its own status read. Two shapes were tried and measured
# broken before this one. A word-split variable let a path with a space split into
# two unreadable ones, and `|| true` made grep's 2 indistinguishable from «found
# nothing» — the repository already carries such a path a directory away,
# web/prototype/Doctor Dashboard.html. Then `find -exec grep {} +`, which fixed the
# splitting but not the reading: the status that comes back is find's, and find
# collapses grep's 2 into its own 1 — measured, a healthy run and an unreadable file
# both answer 1, so the threshold was dead.
echo "==> var(--…) stays inside src/tokens"
listed=0
searched=0
loose=''

while IFS= read -r -d '' stylesheet; do
    listed=$((listed + 1))

    set +e
    found=$(grep -niE 'var[[:space:]]*\([[:space:]]*--' "$stylesheet")
    status=$?
    set -e

    case "$status" in
        0)
            loose="$loose$stylesheet: $found"
            searched=$((searched + 1))
            ;;
        1) searched=$((searched + 1)) ;;
        *)
            echo "grep answered $status for $stylesheet, so this check did not read it" >&2
            exit 1
            ;;
    esac
done < <(find src -name '*.css' -not -path 'src/tokens/*' -print0)

if [ "$listed" -eq 0 ] || [ "$searched" -ne "$listed" ]; then
    echo "read $searched of $listed stylesheet(s) outside src/tokens, so this check measured nothing" >&2
    exit 1
fi

if [ -n "$loose" ]; then
    echo "a custom property is named outside src/tokens, where a typo renders as nothing:" >&2
    printf '  %s\n' "$loose" >&2
    exit 1
fi

echo "==> vitest"
npm run --silent test

echo "==> vite build"
npx vite build

# The build does not fail on an asset it could not resolve — it warns and exits 0.
# See the script's own header for the measurement.
echo "==> every asset the build points at is in the build"
node scripts/check-build.ts

echo "web gate: green"

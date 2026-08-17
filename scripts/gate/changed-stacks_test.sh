#!/usr/bin/env bash
# What changed-stacks.sh answers, for the inputs whose answer matters.
#
# The large-input case is not padding. The pipeline this replaced answered false
# for a change it had matched, once the matching paths passed roughly 50KB — the
# size `git ls-files` reaches on a push to main — because `grep -q` exits early
# and SIGPIPE becomes the pipeline's status under `pipefail`. It went unnoticed
# because the wrong answer skips a job, and a skipped job satisfies a required
# check.
set -euo pipefail

cd "$(dirname "$0")"

failures=0

# The local grep may be ugrep, whose -qv disagrees with POSIX; the script under
# test avoids -q entirely, but a future edit reintroducing one would be measured
# here against whichever grep this machine has. CI runs GNU grep.
echo "==> grep: $(grep --version 2>/dev/null | head -1)"

check() {
    local name=$1 want=$2 stack=$3 input=$4
    local got
    got=$(printf '%s\n' "$input" | ./changed-stacks.sh | grep "^$stack=" | cut -d= -f2)

    if [ "$got" != "$want" ]; then
        echo "FAIL: $name — $stack=$got, want $stack=$want" >&2
        failures=$((failures + 1))
    else
        printf '  ok  %-44s %s=%s\n' "$name" "$stack" "$got"
    fi
}

echo "==> the web filter"
check "the dashboard's own source"      true  web "web/src/app.tsx"
check "the frozen prototype alone"      false web "web/prototype/dd-app.jsx"
check "the prototype beside the API"    false web "$(printf 'web/prototype/dd-app.jsx\napi/main.go')"
check "the prototype beside the app"    true  web "$(printf 'web/prototype/dd-app.jsx\nweb/src/app.tsx')"
check "the gate script itself"          true  web "scripts/gate/web.sh"
check "nothing of the dashboard's"      false web "api/main.go"

echo "==> the other stacks"
check "go source"                       true  api     "api/internal/identity/onboarding.go"
check "go untouched"                    false api     "web/src/app.tsx"
check "kmp source"                      true  kmp     "kmp/shared/src/Foo.kt"
check "a ruleset edit reaches the gate" true  scripts ".github/rulesets/main.json"
# The gate scripts themselves, and the mutant this closes is the sharp one: drop
# `scripts/` from that pattern and a pull request touching only scripts/ skips the
# Shell gate — which is where shellcheck, ruleset.sh and this very test run. The
# filter's own guard, switched off by the filter's own error.
check "a gate script edit reaches the gate" true  scripts "scripts/gate/go.sh"
check "nothing of the scripts'"          false scripts "web/src/app.tsx"

# The regression the shape of this script exists for. A whole tree's worth of
# matching paths after the first non-prototype line: with an early-exiting
# consumer the producer is killed mid-write and the answer flips to false.
echo "==> a change too large for an early exit"
large=$(printf 'web/src/app.tsx\n'; for i in $(seq 1 4000); do printf 'web/src/generated/module-%05d.tsx\n' "$i"; done)
check "four thousand dashboard files"   true  web "$large"

if [ "$failures" -gt 0 ]; then
    echo "changed-stacks: $failures failing case(s)" >&2
    exit 1
fi

echo "changed-stacks: green"

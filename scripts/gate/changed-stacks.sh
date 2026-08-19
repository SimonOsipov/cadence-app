#!/usr/bin/env bash
# Decides which stacks a change touches. CI reads the answer to skip the gates a
# change cannot affect.
#
# A script rather than an inline `run:` block in ci.yml, because the answer is
# load-bearing in the dangerous direction. A skipped job satisfies a required
# status check — see scripts/gate/ruleset.sh — so a filter that wrongly answers
# false does not turn a pull request red. It merges the change having gated it
# with nothing. Inline it was reachable by neither shellcheck nor a test.
#
# Reads the newline-separated file list on stdin, writes `name=true|false` lines
# on stdout.
set -euo pipefail

changed=$(cat)

# No `grep -q` anywhere below, and that is not a style choice. Under `pipefail` a
# -q consumer exits on its first hit, the producer takes SIGPIPE, and the pipeline
# reports 141 — so the helper answers false for an input it matched. It is a race
# and not a boundary: measured here, the old form is right below ~24KB of
# *matching* paths, answers differently from run to run around 30KB, and is wrong
# above ~35KB. What travels that pipe is the matched subset and not the change —
# for a push to main it is under a kilobyte here — so what reaches the band is a
# pull request touching thousands of files under one stack, which the test carries.
#
# emit refuses mid-stream, with earlier lines already written. That is safe and
# worth saying: the step fails, «Changed stacks» goes red, and it is itself a
# required context — so the pull request is blocked by a red check rather than by
# a skipped one.
count() { echo "$changed" | grep -cE "$1" || true; }

# emit runs in this shell rather than inside a command substitution, and that is
# the whole point of its shape. An `exit 1` raised within `$( … )` ends only the
# subshell: the caller's `echo` still succeeds, the script walks off the end with
# a zero status, and every value it printed is empty. Empty is falsy to every
# `if:` in ci.yml, so that mistake skips all four gates at once — which is the
# same failure this file exists to prevent, widened.
emit() {
    local name=$1 counted=$2

    case $counted in
        '' | *[!0-9]*)
            echo "the counter for '$name' produced '$counted', so nothing was measured" >&2
            exit 1
            ;;
    esac

    if [ "$counted" -gt 0 ]; then
        echo "$name=true"
    else
        echo "$name=false"
    fi
}

emit api "$(count '^(api/|scripts/gate/(go|all)\.sh|\.github/workflows/ci\.yml)')"
emit kmp "$(count '^(kmp/|scripts/gate/(kmp|ios|all)\.sh|\.github/workflows/ci\.yml)')"

# Four paths outside web/ are in here because the web gate reads them, and a gate
# that is skipped for a change it would have failed on is worse than no gate: the
# merge leaves main red for whoever pushes next.
#
#   api/openapi.json    src/api is generated from it and committed, and the gate
#                       regenerates and compares — a contract change touching no
#                       file under web/ is exactly what that check is for
#   web/prototype       port.test.ts compares the ported CSS against it byte for
#                       byte, and derive-icons re-derives the icon subset from it
#   CadenceColors.kt    reconciliation.test.ts compares the palettes
#   composeResources    fonts.test.ts hashes the faces against the ones bundled
#                       for mobile
#
# The prototype used to be subtracted here, on the grounds that no gate builds it.
# That stopped being true the moment the web gate started reading it.
emit web "$(count '^(web/|api/openapi\.json|kmp/composeApp/src/commonMain/(kotlin/app/cadence/design/CadenceColors\.kt|composeResources/font/)|scripts/gate/(web|all)\.sh|\.github/workflows/ci\.yml)')"

# The whole of .github/, not just workflows/: the shell gate also verifies the
# committed ruleset, and a change to the ruleset alone has to reach the job that
# checks it.
emit scripts "$(count '^(scripts/|\.github/)')"

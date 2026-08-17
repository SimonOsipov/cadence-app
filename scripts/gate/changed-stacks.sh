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
# reports 141 — so the helper answers false for an input it matched. Measured on
# this machine: the old form flips between 27KB and 32KB of *matching* paths. What
# travels that pipe is the matched subset, not the change — for a push to main it
# is under a kilobyte here — so the case that reaches the threshold is a pull
# request touching thousands of files under one stack, which the test carries.
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

# web/prototype is excluded on purpose: it is a frozen visual specification that
# no gate builds, and a change to it alone must not spend a runner.
#
# Two greps rather than a negative lookahead: grep -E is POSIX ERE and has none,
# so `(?!prototype/)` would match nothing and the answer would be false forever.
emit web "$(
    echo "$changed" |
        grep -E '^(web/|scripts/gate/(web|all)\.sh|\.github/workflows/ci\.yml)' |
        grep -cvE '^web/prototype/' || true
)"

# The whole of .github/, not just workflows/: the shell gate also verifies the
# committed ruleset, and a change to the ruleset alone has to reach the job that
# checks it.
emit scripts "$(count '^(scripts/|\.github/)')"

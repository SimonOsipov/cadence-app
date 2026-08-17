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
# -q consumer exits on its first hit, the producer takes SIGPIPE, and the
# pipeline reports 141 — so the helper answers false for an input it matched.
# Reproduced at 50KB of matching paths, which is the size `git ls-files` reaches
# on a push to main.
count() { echo "$changed" | grep -cE "$1" || true; }
truth() { [ "$1" -gt 0 ] && echo true || echo false; }

matches() { truth "$(count "$1")"; }

# A second helper rather than a negative lookahead: grep -E is POSIX ERE and has
# none, so `(?!prototype/)` would match nothing and the answer would be false
# forever — silently.
matches_except() {
    truth "$(echo "$changed" | grep -E "$1" | grep -cvE "$2" || true)"
}

echo "api=$(matches '^(api/|scripts/gate/(go|all)\.sh|\.github/workflows/ci\.yml)')"
echo "kmp=$(matches '^(kmp/|scripts/gate/(kmp|ios|all)\.sh|\.github/workflows/ci\.yml)')"

# web/prototype is excluded on purpose: it is a frozen visual specification that
# no gate builds, and a change to it alone must not spend a runner.
echo "web=$(matches_except '^(web/|scripts/gate/(web|all)\.sh|\.github/workflows/ci\.yml)' '^web/prototype/')"

# The whole of .github/, not just workflows/: the shell gate also verifies the
# committed ruleset, and a change to the ruleset alone has to reach the job that
# checks it.
echo "scripts=$(matches '^(scripts/|\.github/)')"

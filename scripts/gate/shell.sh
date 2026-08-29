#!/usr/bin/env bash
# Lints the shell this repository runs unattended: the gates and the Ralph loop.
#
# A script rather than an inline block in the workflow, for the reason the stack filter is one:
# a block in ci.yml is reachable by CI alone, and this check was invisible to `all.sh` until an
# unquoted expansion passed every local stage and failed the build.
set -euo pipefail

cd "$(dirname "$0")/../.."

if ! command -v shellcheck >/dev/null 2>&1; then
    echo "shellcheck is not installed — this gate needs it" >&2
    exit 1
fi

echo "==> shellcheck ($(shellcheck --version | awk '/^version:/ {print $2}'))"
shellcheck -x scripts/gate/*.sh scripts/ralph/*.sh

echo "shell gate: green"

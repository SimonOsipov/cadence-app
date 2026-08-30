#!/usr/bin/env bash
# Lints the shell this repository runs unattended: the gates and the Ralph loop. A script and not
# an inline block in ci.yml, so `all.sh` can run it — see the measurement there.
set -euo pipefail

cd "$(dirname "$0")/../.."

if ! command -v shellcheck >/dev/null 2>&1; then
    echo "shellcheck is not installed — this gate needs it" >&2
    exit 1
fi

echo "==> shellcheck ($(shellcheck --version | awk '/^version:/ {print $2}'))"
shellcheck -x scripts/gate/*.sh scripts/ralph/*.sh

echo "shell gate: green"

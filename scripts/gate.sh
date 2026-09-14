#!/usr/bin/env bash
# Only `all` is real: gate/all.sh runs every stack as one pass, so lint and test fail rather than pretend.
set -euo pipefail
cd "$(dirname "$0")/.."

case "${1:-all}" in
  all) exec ./scripts/gate/all.sh ;;
  lint|test) echo "gate: cadence-app does not split lint from test — run 'gate.sh all'" >&2; exit 1 ;;
  *) echo "usage: gate.sh [lint|test|all]" >&2; exit 2 ;;
esac

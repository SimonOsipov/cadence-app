#!/usr/bin/env bash
# Runs every gate that applies on this machine. This is the command to run
# before opening a pull request.
#
# web/ has no gate yet: the Vite dashboard does not exist (SKL-09). web/prototype
# is a frozen visual specification and is never built.
set -euo pipefail

here="$(dirname "$0")"
ran=()
skipped=()

"$here/go.sh"
ran+=("go")

"$here/kmp.sh"
ran+=("kmp")

if xcodebuild -version >/dev/null 2>&1; then
    "$here/ios.sh"
    ran+=("ios")
else
    echo "skipping the iOS gate: no Xcode on this host"
    skipped+=("ios: no Xcode")
fi

echo
# Never an unqualified "all green": on a host without Xcode a third of the gate,
# including every Compose UI test, did not run.
if [ ${#skipped[@]} -eq 0 ]; then
    echo "gates green: ${ran[*]}"
else
    echo "gates green: ${ran[*]} — NOT RUN: ${skipped[*]}"
fi

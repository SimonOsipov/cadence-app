#!/usr/bin/env bash
# Runs every gate that applies on this machine. This is the command to run
# before opening a pull request.
#
# web/prototype is a frozen visual specification and is never built; the gate
# below covers web/ proper.
set -euo pipefail

here="$(dirname "$0")"
ran=()
skipped=()

# First because it is the cheapest. What it proves is narrow and worth stating
# exactly: every job in ci.yml appears in the committed ruleset, and nothing in
# the ruleset names a job that does not exist. Whether that ruleset is actually
# in force on GitHub is not knowable from here — applying it needs admin, which
# nobody running this script has.
"$here/ruleset.sh"
ran+=("ruleset")

# Cheap, and about the same thing one layer down: which gates run at all. A filter
# answering false skips a job, and a skipped job satisfies a required check.
"$here/changed-stacks_test.sh"
ran+=("changed-stacks")

"$here/go.sh"
ran+=("go")

"$here/kmp.sh"
ran+=("kmp")

# Needs a Node matching web/.nvmrc; the gate says so itself rather than being
# skipped silently, because a dashboard nobody built is not a dashboard that
# passed.
"$here/web.sh"
ran+=("web")

# The integration suite is where forced RLS, the low-privilege request role and
# the migration chain are actually proven — go.sh runs `go test ./...` with no
# build tag, so it reports "[no test files]" for the package those live in. This
# script called itself the pre-PR command while never running them, and did not
# even list them as skipped.
if docker info >/dev/null 2>&1; then
    (cd "$here/../../api" && make test-integration)
    ran+=("api-integration")
else
    echo "skipping the API integration suite: no Docker daemon"
    skipped+=("api-integration: no Docker daemon")
fi

if xcodebuild -version >/dev/null 2>&1; then
    "$here/ios.sh"
    ran+=("ios")
else
    echo "skipping the iOS gate: no Xcode on this host"
    skipped+=("ios: no Xcode")
fi

echo
# Never an unqualified "all green": on a host without Xcode the majority of the
# KMP tests — 293 of 537, every Compose UI test there is — did not run, and
# without Docker neither did the RLS invariants.
if [ ${#skipped[@]} -eq 0 ]; then
    echo "gates green: ${ran[*]}"
else
    echo "gates green: ${ran[*]} — NOT RUN: ${skipped[*]}"
fi

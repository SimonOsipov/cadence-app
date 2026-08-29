#!/usr/bin/env bash
# Quality gate for kmp/ — the parts that run on any machine, Linux included.
# The iOS half lives in ios.sh because it needs Xcode.
#
# Order is deliberate: style and static analysis first (seconds), then the unit
# tests, then the Android build.
set -euo pipefail

cd "$(dirname "$0")/../../kmp"

echo "==> ktlint"
./gradlew ktlintCheck

echo "==> detekt"
./gradlew detekt

# `:shared` only, and not by choice of this line. composeApp declares no
# `withHostTestBuilder {}` in its build file, so `testAndroidHostTest` resolves
# to `:shared:testAndroidHostTest` and nothing else.
#
# Counted 2026-08-29, every figure by running it rather than by arithmetic: 369
# here, 356 of `:shared` again on the iOS simulator target, and composeApp's 494
# Compose UI tests, which run under ios.sh on a macOS host and nowhere else. The
# figures that stood here said 244 and 293 and had been overtaken by the ported
# screens; recounting one number of several is how a comment ends up part true,
# which is what happened to the fourth one on the first attempt at this paragraph.
#
# Fourteen of the 369 run under a Robolectric runtime, which is what secure storage
# needed to be checkable at all — the fifteenth test in androidHostTest is the
# platform seam's, and it needs no runtime. The Keychain half is not here either: it needs an
# app bundle, so it is an XCTest target under ios.sh.

echo "==> unit tests (:shared only — composeApp needs ios.sh)"
./gradlew testAndroidHostTest

# The generator brings its own credential helpers — every *Api extends ApiClient, which holds a
# map of them — and they come into the tree because the client will not compile without them.
# They stay unused: the token is attached by the transport, in one place, and two mechanisms
# under refresh-token rotation mean a revoked session. Grepped rather than promised, and outside
# the generated tree because inside it the calls are the generator's own.
echo "==> the generated credential helpers stay unused"
callers=$(grep -rn "setBearerToken\|setAccessToken\|setUsername\|setPassword" \
    --include="*.kt" shared/src composeApp/src androidApp/src 2>/dev/null |
    grep -v "/generated/" || true)
if [ -n "$callers" ]; then
    echo "the transport is not the only owner of the token any more:" >&2
    echo "$callers" >&2
    exit 1
fi

echo "==> the client matches the contract"
./gradlew :shared:openApiDrift

echo "==> assemble"
./gradlew :androidApp:assembleDebug

echo
echo "kmp gate: green — :shared only. composeApp's Compose UI tests did NOT run;"
echo "                 they need ios.sh and a macOS host with Xcode."

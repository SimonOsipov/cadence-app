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
# to `:shared:testAndroidHostTest` and nothing else — 353 of them, counted on
# 2026-08-29 rather than carried forward: the figure that stood here said 244
# and had been overtaken by the ported screens. Eight run under a Robolectric
# runtime, which is what secure storage needed to be checkable at all. The other 293 are every Compose UI test there is, and they run under
# ios.sh, on macOS, and nowhere else.
#
# This comment used to claim the opposite: "unqualified, so every module that
# grows host tests is picked up". composeApp grew 293 of them and was not
# picked up, and the line read as if it had been.
echo "==> unit tests (:shared only — composeApp needs ios.sh)"
./gradlew testAndroidHostTest

echo "==> assemble"
./gradlew :androidApp:assembleDebug

echo
echo "kmp gate: green — :shared only. composeApp's Compose UI tests did NOT run;"
echo "                 they need ios.sh and a macOS host with Xcode."

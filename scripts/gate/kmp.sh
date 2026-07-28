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

# Unqualified, so every module that grows host tests is picked up without
# anyone remembering to edit this line.
echo "==> unit tests"
./gradlew testAndroidHostTest

echo "==> assemble"
./gradlew :androidApp:assembleDebug

echo "kmp gate: green"

#!/usr/bin/env bash
# The iOS half of the kmp/ gate. Separate from kmp.sh because it needs Xcode,
# and on CI that means a macOS runner, billed at ten times the Linux rate — so
# it is worth being able to run the cheap half alone.
set -euo pipefail

# A generic destination builds without depending on which simulator devices a
# given machine happens to have installed. Override it locally when you want to
# build against one specific device.
#
# ARCHS is pinned to arm64 below: a generic destination otherwise asks for an
# x86_64 slice too, and the only simulator target declared in the build is
# iosSimulatorArm64. Intel simulators would need an iosX64() target first.
DESTINATION=${DESTINATION:-generic/platform=iOS Simulator}

cd "$(dirname "$0")/../../kmp"

if ! xcodebuild -version >/dev/null 2>&1; then
    echo "Xcode is not available — this gate needs a macOS host with Xcode" >&2
    exit 1
fi

echo "==> kotlin tests on the iOS simulator target"
./gradlew :shared:iosSimulatorArm64Test :composeApp:iosSimulatorArm64Test

echo "==> xcodegen (the .xcodeproj is generated from project.yml)"
# The drift check compares the regenerated project byte for byte, so it is only
# meaningful with the XcodeGen version the committed project was written by:
# a different one reorders objects and rewrites its own version markers. A
# mismatch skips the check loudly instead of failing a pull request that changed
# nothing.
expected_xcodegen=$(tr -d '[:space:]' < iosApp/.xcodegen-version)
if ! command -v xcodegen >/dev/null 2>&1; then
    echo "xcodegen is not installed — skipping the drift check (want $expected_xcodegen)" >&2
elif [ "$(xcodegen --version | tr -dc '0-9.')" != "$expected_xcodegen" ]; then
    # A laptop with the wrong version gets a loud skip; CI installs the pinned
    # one, so there the skip means the check has quietly stopped running.
    echo "xcodegen $(xcodegen --version) != pinned $expected_xcodegen — skipping the drift check" >&2
    if [ -n "${CI:-}" ]; then
        echo "refusing to skip it on CI, where the version is pinned" >&2
        exit 1
    fi
else
    echo "    xcodegen $expected_xcodegen"
    (cd iosApp && xcodegen generate)
    # A generated project that differs from the committed one means project.yml
    # and the checked-in .xcodeproj have drifted apart.
    if ! git diff --quiet -- iosApp/iosApp.xcodeproj; then
        echo "iosApp.xcodeproj is out of date — regenerate it and commit the result" >&2
        git --no-pager diff --stat -- iosApp/iosApp.xcodeproj >&2
        exit 1
    fi
fi

echo "==> xcodebuild (iOS simulator)"
xcodebuild \
    -project iosApp/iosApp.xcodeproj \
    -scheme iosApp \
    -configuration Debug \
    -destination "$DESTINATION" \
    ARCHS=arm64 \
    build

# The XCTest bundle, and it needs a named device rather than the generic destination the
# build above takes: a test run has to boot a simulator, and `generic/platform=` names none.
#
# It exists because a keychain needs an app identity. Measured 2026-08-29: SecItemAdd from
# a Kotlin/Native test binary answers -25291, errSecNotAvailable, so the Keychain suite
# cannot live in shared/src/iosTest at all — only the refusal does.
# `OS=latest` rather than a pinned runtime, and a device name that has to exist: a test run
# boots a simulator, and `generic/platform=` names none to boot. CI sets this explicitly,
# because a change of macOS image otherwise takes the whole gate down over a device list.
TEST_DESTINATION=${TEST_DESTINATION:-platform=iOS Simulator,name=iPhone 17,OS=latest}

echo "==> xcodebuild test (the Keychain suite, hosted by the app)"
xcodebuild \
    -project iosApp/iosApp.xcodeproj \
    -scheme iosApp \
    -configuration Debug \
    -destination "$TEST_DESTINATION" \
    ARCHS=arm64 \
    test

echo "ios gate: green"

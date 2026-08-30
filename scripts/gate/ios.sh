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

# Kotlin/Native links what is reachable, so this measures reachability itself — the Android grep
# can only measure that the module is on the variant, a debug APK being unminified.
#
# Counted 2026-08-30 by running it: 41 for the screen and 5 for the entry point with the property,
# 0 and 0 without, and the ObjC header carries the entry point only with it — which is what makes
# the screen callable from Swift rather than merely present.
echo "==> the debug screen is in the framework only behind -Pcadence.debugTools"
framework=composeApp/build/bin/iosSimulatorArm64/debugFramework/ComposeApp.framework

# Counted into a variable rather than asked with `grep -q`, and the reason is not style. `grep -q`
# exits at the first match, `strings` takes SIGPIPE, and under `set -o pipefail` the pipeline that
# FOUND the marker reports failure — so the check read «present» as «absent» and could never pass.
# Measured on the framework this file greps: the marker is there 41 times and `grep -q` answered no.
# Read from the Kotlin constant rather than spelled here — kmp.sh does the same, and a second
# spelling is a check that goes quiet the day the class is renamed. Resolved at top level and not
# inside $( … ): an exit there ends the subshell and the caller reads the empty output as data.
marker=$(sed -n 's/.*DEBUG_SCREEN_MARKER: String = "\([^"]*\)".*/\1/p' \
    debugTools/src/commonMain/kotlin/app/cadence/debug/DebugScreen.kt)
[ -n "$marker" ] || {
    echo "DEBUG_SCREEN_MARKER could not be read from the source — this check greps for nothing" >&2
    exit 1
}

screen_hits() {
    local found
    found=$(strings -a "$framework/ComposeApp" | grep -c "$marker" || true)
    echo "$found"
}

./gradlew :composeApp:linkDebugFrameworkIosSimulatorArm64 --rerun-tasks
if [ "$(screen_hits)" -ne 0 ]; then
    echo "the debug screen is in a framework built without -Pcadence.debugTools" >&2
    exit 1
fi

./gradlew :composeApp:linkDebugFrameworkIosSimulatorArm64 --rerun-tasks -Pcadence.debugTools
if [ "$(screen_hits)" -eq 0 ]; then
    echo "the debug screen is missing from a framework built with -Pcadence.debugTools" >&2
    exit 1
fi
# Counted the way screen_hits is, and for the same reason: `grep -c` on a missing file prints
# nothing, and `[ "" -eq 0 ]` is a usage error the `if` reads as false — the check would walk past
# an absent header. The header's existence is asserted first.
header=$framework/Headers/ComposeApp.h
[ -f "$header" ] || {
    echo "the framework header is not there — nothing was grepped" >&2
    exit 1
}
entry_hits=$(grep -c debugViewController "$header" || true)
if [ "${entry_hits:-0}" -eq 0 ]; then
    echo "debugViewController is not in the framework header — Swift cannot reach the screen" >&2
    exit 1
fi

# Left as the property found it, so the xcodebuild below measures the shipping shape.
./gradlew :composeApp:linkDebugFrameworkIosSimulatorArm64 --rerun-tasks

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

# The XCTest bundle. It exists because a keychain needs an app identity. Measured 2026-08-29: SecItemAdd from
# a Kotlin/Native test binary answers -25291, errSecNotAvailable, so the Keychain suite
# cannot live in shared/src/iosTest at all — only the refusal does.
# A named device with OS=latest, because a test run boots a simulator and the generic
# destination above names none to boot. This default is the only place the device is written:
# CI takes it as it stands and overrides the variable on the day the runner image stops
# carrying this one.
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

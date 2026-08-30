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
# The roots are checked rather than swallowed: `2>/dev/null` over a renamed one would answer
# empty and pass, which is the shape of a guard that cannot fail. setApiKey is in the pattern
# because ApiKeyAuth is the one helper that can put a credential in the query string, which is
# what the spec forbids by name. iosApp/ is searched too: :shared is exported into the
# framework, so ApiClient's setters are reachable from Swift.
roots=(shared/src composeApp/src androidApp/src iosApp/iosApp iosApp/iosAppTests)
for root in "${roots[@]}"; do
    [ -d "$root" ] || { echo "the credential guard searches $root, which is not there" >&2; exit 1; }
done
callers=$(grep -rn --include='*.kt' --include='*.swift' \
    "setBearerToken\|setAccessToken\|setApiKey\|setUsername\|setPassword" \
    "${roots[@]}" | grep -v "/generated/" || true)
if [ -n "$callers" ]; then
    echo "the transport is not the only owner of the token any more:" >&2
    echo "$callers" >&2
    exit 1
fi

echo "==> the client matches the contract"
./gradlew :shared:openApiDrift :shared:ktorAlignment

# The address list the release refusal holds, run rather than read. It was read twice and read
# wrong twice — a case-sensitive alternative admitted LOCALHOST, and a later one admitted
# Docker's own bridge — and until this ran here the only evidence for it was one by-hand pass.
# One of each direction is enough to catch the next narrowing.
echo "==> a release refuses an address that is not the product's"
if ./gradlew -q :shared:refuseDevAddressInRelease -Pcadence.apiBase=https://172.17.0.1:8080 >/dev/null 2>&1; then
    echo "the release refusal admitted a private address" >&2
    exit 1
fi
./gradlew -q :shared:refuseDevAddressInRelease -Pcadence.apiBase=https://api.cadence.ru

echo "==> assemble"
./gradlew :androidApp:assembleDebug

# The debug screen is in the debug artifact and not in the release one, checked by grepping the
# artifacts themselves. The spec calls this the only check that holds regardless of the
# mechanism, and it is the one that would have caught a `debugMain` directory Gradle ignored
# silently — the reason :debugTools is a module at all.
#
# The release build is paid for here rather than assumed: measured, the marker is in
# classes12.dex of the debug APK nineteen times and in none of the release APK's three. An
# address is passed because a release against the dev one is refused, which is another gate.
echo "==> the debug screen ships in debug and not in release"
./gradlew :androidApp:assembleRelease -Pcadence.apiBase=https://api.cadence.example

marker=CadenceDebugScreen
hits_in() {
    local apk=$1 total=0
    local work
    work=$(mktemp -d)
    unzip -q -o "$apk" 'classes*.dex' -d "$work"
    for dex in "$work"/*.dex; do
        total=$((total + $(strings "$dex" | grep -c "$marker")))
    done
    rm -rf "$work"
    echo "$total"
}

debug_apk=$(find androidApp/build/outputs/apk/debug -name '*.apk' | head -1)
release_apk=$(find androidApp/build/outputs/apk/release -name '*.apk' | head -1)
[ -n "$debug_apk" ] && [ -n "$release_apk" ] || {
    echo "the artifacts to grep are not there" >&2
    exit 1
}

if [ "$(hits_in "$debug_apk")" -eq 0 ]; then
    echo "$marker is absent from the debug artifact — the module is wired to nothing" >&2
    exit 1
fi
if [ "$(hits_in "$release_apk")" -ne 0 ]; then
    echo "$marker is in the release artifact" >&2
    exit 1
fi

echo
echo "kmp gate: green — :shared only. composeApp's Compose UI tests did NOT run;"
echo "                 they need ios.sh and a macOS host with Xcode."

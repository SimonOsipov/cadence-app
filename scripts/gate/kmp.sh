#!/usr/bin/env bash
# Quality gate for kmp/ — the parts that run on any machine, Linux included.
# The iOS half lives in ios.sh because it needs Xcode.
#
# Order is deliberate: style and static analysis first (seconds), then the unit
# tests, then the Android build.
set -euo pipefail

cd "$(dirname "$0")/../../kmp"

echo "==> ktlint"
# The property is passed so ktlint sees the Apple composition root at all. Its source directory
# is registered only with the module it calls — registering it unconditionally would compile a
# file whose imports are not on the path — so without this flag `DebugViewController.kt` is in no
# ktlint task: measured, four violations planted in it were reported 0 times without and 4 with.
# detekt needs no flag; it takes its own path list, which now names both roots.
./gradlew ktlintCheck -Pcadence.debugTools

echo "==> detekt"
./gradlew detekt

# Two modules and not one, which this line got wrong until it was measured: `:shared` and
# `:debugTools` both declare `withHostTestBuilder {}`, so the unqualified task resolves to both.
# composeApp declares none, so its tests are not here — they need ios.sh.
#
# Counted 2026-08-30 by running it rather than by arithmetic: 383 in `:shared` and 12 in
# `:debugTools`. Two figures elsewhere, re-measured the same day: 367 of `:shared` again on the
# iOS simulator target, and composeApp's 494 Compose UI tests, which run under ios.sh on a macOS
# host and nowhere else. Every figure here is the whole measurement — correcting one of several
# is how this paragraph was part-true twice.
#
# Four files in `:shared` run under a Robolectric runtime, which is what secure storage needed to
# be checkable at all. The Keychain half is not here either: it needs an app bundle, so it is an
# XCTest target under ios.sh.

echo "==> unit tests (:shared and :debugTools — composeApp needs ios.sh)"
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
# Both addresses, each refused on its own. GoTrue is a second service on a second port, and it
# is where the password goes — a release pointed at a dev identity server is the worse of the
# two leaks. Checking only the API would pass a build that ships one.
echo "==> a release refuses an address that is not the product's"
good_api=https://api.cadence.ru
good_auth=https://auth.cadence.ru
if ./gradlew -q :shared:refuseDevAddressInRelease \
    -Pcadence.apiBase=https://172.17.0.1:8080 -Pcadence.authBase="$good_auth" >/dev/null 2>&1; then
    echo "the release refusal admitted a private API address" >&2
    exit 1
fi
if ./gradlew -q :shared:refuseDevAddressInRelease \
    -Pcadence.apiBase="$good_api" -Pcadence.authBase=http://localhost:9999 >/dev/null 2>&1; then
    echo "the release refusal admitted a dev identity server" >&2
    exit 1
fi
./gradlew -q :shared:refuseDevAddressInRelease -Pcadence.apiBase="$good_api" -Pcadence.authBase="$good_auth"

echo "==> assemble"
./gradlew :androidApp:assembleDebug

# The debug screen is in the debug artifact and not in the release one, checked by grepping the
# artifacts themselves. The spec calls this the only check that holds regardless of the
# mechanism, and it is the one that would have caught a `debugMain` directory Gradle ignored
# silently — the reason :debugTools is a module at all.
#
# Three builds and not two. The third passes `-Pcadence.debugTools` at a **release** build: that
# property is the Apple switch, Gradle properties are global to the invocation, and while the
# module was declared on composeApp's commonMain it put the screen — with its sign-in wiring and
# the dev addresses behind it — into the release APK. Measured at five occurrences in the dex
# before the dependency moved to iosMain. Nothing in a variant catches that, so it is checked.
#
# The marker is read out of the Kotlin constant rather than repeated here: two spellings of one
# string is a check that goes quiet the day somebody renames the class.
echo "==> the debug screen ships in debug, and in no release"
./gradlew :androidApp:assembleRelease -Pcadence.debugTools \
    -Pcadence.apiBase=https://api.cadence.example -Pcadence.authBase=https://auth.cadence.example
release_with_property=$(find androidApp/build/outputs/apk/release -name '*.apk' | head -1)
cp "$release_with_property" "${TMPDIR:-/tmp}/cadence-release-with-property.apk"
./gradlew :androidApp:assembleRelease -Pcadence.apiBase=https://api.cadence.example \
    -Pcadence.authBase=https://auth.cadence.example

marker=$(sed -n 's/.*DEBUG_SCREEN_MARKER: String = "\([^"]*\)".*/\1/p' \
    debugTools/src/commonMain/kotlin/app/cadence/debug/DebugScreen.kt)
[ -n "$marker" ] || {
    echo "DEBUG_SCREEN_MARKER could not be read from the source — this check greps for nothing" >&2
    exit 1
}

# Sets $marker_hits rather than answering on stdout: an `exit 1` inside $( … ) ends only the
# subshell — the trap is recorded in changed-stacks.sh.
marker_hits=0
count_in() {
    local apk=$1 needle=$2 work dex found=0
    marker_hits=0
    work=$(mktemp -d)
    unzip -q -o "$apk" 'classes*.dex' -d "$work" || {
        echo "$apk could not be unpacked" >&2
        exit 1
    }
    for dex in "$work"/*.dex; do
        [ -e "$dex" ] || continue
        found=1
        marker_hits=$((marker_hits + $(grep -ac "$needle" "$dex" || true)))
    done
    rm -rf "$work"
    [ "$found" -eq 1 ] || {
        echo "$apk holds no classes*.dex — nothing was grepped" >&2
        exit 1
    }
}

debug_apk=$(find androidApp/build/outputs/apk/debug -name '*.apk' | head -1)
release_apk=$(find androidApp/build/outputs/apk/release -name '*.apk' | head -1)
if [ -z "$debug_apk" ] || [ -z "$release_apk" ]; then
    echo "the artifacts to grep are not there" >&2
    exit 1
fi

count_in "$debug_apk" "$marker"
if [ "$marker_hits" -eq 0 ]; then
    echo "$marker is absent from the debug artifact — the module is not on the variant" >&2
    exit 1
fi

# Inclusion is not reachability, and on Android the grep above measures only the first. A debug
# build is not minified, so `debugImplementation` ships the whole module whether or not anything
# calls it: with the composition root deleted the marker still answered 8 in the dex. What makes
# the screen reachable here is the manifest entry, so that is what is checked.
#
# The same defect is loud on Apple and silent here — Kotlin/Native drops what nothing calls, so
# `strings` on the framework answered 0 for exactly this. ios.sh owns that half.
# The task directory under merged_manifests carries the AGP task name, so the path is found
# rather than spelled. Found empty means the build did not produce one, which is not a pass.
root_entry='android:name="app.cadence.android.DebugActivity"'
debug_manifests=$(find androidApp/build/intermediates/merged_manifests/debug -name AndroidManifest.xml)
release_manifests=$(find androidApp/build/intermediates/merged_manifests/release -name AndroidManifest.xml)
if [ -z "$debug_manifests" ] || [ -z "$release_manifests" ]; then
    echo "the merged manifests to grep are not there" >&2
    exit 1
fi
# shellcheck disable=SC2086  # deliberately word-split: find answered one path per line
if ! grep -q "$root_entry" $debug_manifests; then
    echo "the debug variant has no DebugActivity — the screen is compiled and unreachable" >&2
    exit 1
fi
# The manifest is a separate file from the class it names, and AGP does not mind one naming a
# class that is not there — it is a crash at launch, not a build failure. So both halves are
# asked: declared, and present. Deleting the composition root left the manifest check green.
count_in "$debug_apk" DebugActivity
if [ "$marker_hits" -eq 0 ]; then
    echo "DebugActivity is declared but not in the debug artifact" >&2
    exit 1
fi
# shellcheck disable=SC2086
if grep -q "$root_entry" $release_manifests; then
    echo "DebugActivity is in the release manifest" >&2
    exit 1
fi
count_in "$release_apk" "$marker"
if [ "$marker_hits" -ne 0 ]; then
    echo "$marker is in the release artifact" >&2
    exit 1
fi
count_in "${TMPDIR:-/tmp}/cadence-release-with-property.apk" "$marker"
if [ "$marker_hits" -ne 0 ]; then
    echo "$marker reached a release artifact through -Pcadence.debugTools" >&2
    exit 1
fi

# Where an invitation lands, on both platforms, and read from the Kotlin constant rather than
# spelled here: a scheme renamed in one place and not the other is a mail that opens nothing, and
# nothing else in the tree would notice. The plist is checked here and not in ios.sh so a machine
# without Xcode still measures it — the file is text either way.
accept=$(sed -n 's/.*ACCEPT_LINK: String = "\([^:]*\):.*/\1/p' \
    shared/src/commonMain/kotlin/app/cadence/shared/auth/Invitation.kt)
# Shape, not merely non-empty: measured by turning the constant into "$SCHEME://accept", which
# sed reads as the literal `$SCHEME` — a non-empty answer that then fails both greps below for
# the wrong reason. An empty one would be worse still: `grep -c ""` matches every line and both
# checks would pass having measured nothing.
case $accept in
    *[!a-z0-9.+-]* | "")
        # Braced deliberately: bash reads the first byte of a following multibyte character as
        # part of the name, and `$accept»` is then an unbound variable rather than the message.
        echo "'${accept}' is not a scheme — ACCEPT_LINK could not be read from Invitation.kt" >&2
        exit 1
        ;;
esac

manifest_scheme=$(grep -c "android:scheme=\"$accept\"" androidApp/src/main/AndroidManifest.xml || true)
if [ "$manifest_scheme" -eq 0 ]; then
    echo "the Android manifest declares no $accept:// scheme — an invitation opens nothing" >&2
    exit 1
fi

plist_scheme=$(grep -c "<string>$accept</string>" iosApp/iosApp/Info.plist || true)
if [ "$plist_scheme" -eq 0 ]; then
    echo "Info.plist declares no $accept:// scheme — an invitation opens nothing" >&2
    exit 1
fi

echo
echo "kmp gate: green — :shared and :debugTools. composeApp's Compose UI tests did NOT run;"
echo "                 they need ios.sh and a macOS host with Xcode."

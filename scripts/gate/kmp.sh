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

# Where an invitation lands, and it is four declarations that have to agree: the Kotlin constant,
# the Android manifest, Info.plist, and the mail template that sends patients there. All read from
# ACCEPT_LINK rather than spelled here — measured, a host renamed in the manifest alone leaves the
# scheme in place, every Kotlin and Go test green, and an invitation that opens nothing.
#
# The plist is checked here and not in ios.sh so a machine without Xcode still measures it.
accept=$(sed -n 's/.*ACCEPT_LINK: String = "\([^"]*\)".*/\1/p' \
    shared/src/commonMain/kotlin/app/cadence/shared/auth/Invitation.kt)
scheme=${accept%%://*}
host=${accept#*://}

# Shape, not merely non-empty: measured by turning the constant into "$SCHEME://accept", which sed
# reads as the literal `$SCHEME` — a non-empty answer that then fails every grep below for the
# wrong reason. An empty one would be worse still: `grep -c ""` matches every line and the checks
# would pass having measured nothing.
case $scheme$host in
    *[!a-z0-9.+-]* | "")
        # Braced deliberately: bash reads the first byte of a following multibyte character as
        # part of the name, and `$accept»` is then an unbound variable rather than the message.
        echo "'${accept}' is not a deep link — ACCEPT_LINK could not be read from Invitation.kt" >&2
        exit 1
        ;;
esac

# `grep -c` on a file that is not there prints nothing and answers 2; `|| true` swallows the
# status, and `[ "" -eq 0 ]` is then an error bash reads as false — so the check passes having
# read nothing. Measured under bash, which is what runs this. Every count below goes through
# here, and the shape is changed-stacks.sh's `emit`, for the same reason it has one.
# The `exit 1` below ends the command substitution its caller wraps it in, not this script — the
# trap changed-stacks.sh records. What carries it out is `set -e` on the assignment, whose status
# is the substitution's: measured, the script stops before the next line. Drop `set -e` and this
# guard goes back to being one that cannot fail.
counted() {
    local what=$1 count=$2

    case $count in
        '' | *[!0-9]*)
            echo "counting $what produced '$count', so nothing was measured" >&2
            exit 1
            ;;
    esac

    echo "$count"
}

manifest=androidApp/src/main/AndroidManifest.xml
# All five inside **one** <intent-filter>, which is the whole of what the check is worth: a filter
# carrying the scheme and host while another carries the action and categories resolves to nothing,
# and the previous version — greps over the concatenation of every block holding the host — passed
# exactly that. Measured on a manifest mid-scheme-migration.
#
# Comment spans are cut out of each line first, because the tags survive inside <!-- -->: a deep
# link parked in a comment satisfied all five. Cut rather than the line dropped, so an ordinary
# trailing comment beside a live declaration does not take the declaration with it.
#
# The three beyond scheme and host are not decoration: without VIEW the filter does not match what
# a mail client sends, without DEFAULT an implicit intent resolves to nothing, and without
# BROWSABLE the mail client hands the link to nobody.
if ! awk -v scheme="android:scheme=\"$scheme\"" -v host="android:host=\"$host\"" '
    BEGIN {
        need[1] = "android.intent.action.VIEW"
        need[2] = "android.intent.category.DEFAULT"
        need[3] = "android.intent.category.BROWSABLE"
        wanted = 5
    }
    {
        need[4] = scheme
        need[5] = host

        line = ""
        rest = $0
        while (rest != "") {
            if (commented) {
                at = index(rest, "-->")
                if (at == 0) { rest = ""; break }
                rest = substr(rest, at + 3)
                commented = 0
            } else {
                at = index(rest, "<!--")
                if (at == 0) { line = line rest; break }
                line = line substr(rest, 1, at - 1)
                rest = substr(rest, at + 4)
                commented = 1
            }
        }
    }
    line ~ /<intent-filter/ { inside = 1; for (i = 1; i <= wanted; i++) seen[i] = 0 }
    inside {
        for (i = 1; i <= wanted; i++) if (index(line, need[i])) seen[i] = 1
    }
    line ~ /<\/intent-filter>/ {
        if (inside) {
            whole = 1
            for (i = 1; i <= wanted; i++) if (!seen[i]) whole = 0
            if (whole) found = 1
        }
        inside = 0
    }
    END { exit found ? 0 : 1 }
' "$manifest"; then
    echo "$manifest has no single intent-filter carrying $scheme, $host, VIEW, DEFAULT and" \
        "BROWSABLE together — an invitation opens nothing on Android" >&2
    exit 1
fi

# Both keys, nested, and nested structurally: either alone is half the check — moving the schemes
# array to the top level satisfies a CFBundleURLSchemes range, misspelling that key satisfies a
# CFBundleURLTypes one, and iOS registers nothing in either state, reading the list only through
# CFBundleURLTypes. Array depth and not indentation, because the first version of this anchored the
# outer range on a leading tab: measured, re-indenting the file to spaces returned the guard to its
# earlier strength with no signal.
if ! awk -v scheme="<string>$scheme</string>" '
    /<key>CFBundleURLTypes<\/key>/ { types = 1; depth = 0; next }
    types {
        if (/<array>/) depth++
        if (/<\/array>/) {
            depth--
            if (depth <= 0) { types = 0; schemes = 0; next }
        }
        if (/<key>CFBundleURLSchemes<\/key>/) { schemes = 1; inner = 0; next }
        if (schemes) {
            if (/<array>/) inner++
            if (/<\/array>/) {
                inner--
                if (inner <= 0) { schemes = 0; next }
            }
            if (index($0, scheme)) found = 1
        }
    }
    END { exit found ? 0 : 1 }
' iosApp/iosApp/Info.plist; then
    echo "Info.plist declares no $scheme:// in a CFBundleURLSchemes array inside" \
        "CFBundleURLTypes — an invitation opens nothing on iOS" >&2
    exit 1
fi

# The other end of the same agreement, and the one nothing else in either stack would notice: the
# app can be registered for an address no invitation ever names.
template=../api/mail-templates/invite.html
template_link=$(counted "$accept in $template" "$(grep -cF "$accept?token_hash=" "$template" || true)")
if [ "$template_link" -eq 0 ]; then
    echo "$template does not send patients to $accept — the app is registered for an address" \
        "no invitation names" >&2
    exit 1
fi

# The password floor the screen states, against the one the provider enforces. Two numbers in two
# stacks and nothing else compares them: a screen promising ten where the server takes six lets a
# patient set a password weaker than the product chose, and promising twelve where it takes ten
# refuses one the server would have accepted — after they typed it.
stated=$(counted "PASSWORD_MIN_LENGTH in AcceptanceCopy.kt" \
    "$(sed -n 's/.*PASSWORD_MIN_LENGTH = \([0-9][0-9]*\).*/\1/p' \
        composeApp/src/commonMain/kotlin/app/cadence/AcceptanceCopy.kt || true)")
enforced=$(counted "GOTRUE_PASSWORD_MIN_LENGTH in docker-compose.yml" \
    "$(sed -n 's/.*GOTRUE_PASSWORD_MIN_LENGTH: *"\([0-9][0-9]*\)".*/\1/p' \
        ../api/docker-compose.yml || true)")
if [ "$stated" -ne "$enforced" ]; then
    echo "the app states a $stated-character password and the deployment enforces $enforced —" \
        "one of them refuses a patient the other invited" >&2
    exit 1
fi

# Registration is not reading. The checks above say the system will hand the app a link; nothing
# else says either root does anything with it, and no test can — composeApp has no Android
# host-test builder and the Swift host is in no Kotlin suite. So the declarations are read where
# they live. The Apple entry point is read out of the Kotlin source rather than spelled twice: a
# rename then fails this check instead of silencing it.
#
# A textual check and named as one: it holds that the calls are there, not that they carry a
# token through. That half is a by-hand pass on both platforms.
# XML with every <!-- --> span cut out. Its own copy of what the intent-filter check does inline:
# that one is a state machine over lines and cannot be handed a pre-stripped file without being
# rewritten, which is a change to a guard that has already been measured.
uncommented() {
    awk '{
        line = ""
        rest = $0
        while (rest != "") {
            if (commented) {
                at = index(rest, "-->")
                if (at == 0) { rest = ""; break }
                rest = substr(rest, at + 3)
                commented = 0
            } else {
                at = index(rest, "<!--")
                if (at == 0) { line = line rest; break }
                line = line substr(rest, 1, at - 1)
                rest = substr(rest, at + 4)
                commented = 1
            }
        }
        print line
    }' "$1"
}

activity=androidApp/src/main/kotlin/app/cadence/android/MainActivity.kt
swift_host=iosApp/iosApp/ContentView.swift
opener=$(sed -n 's/^fun \([A-Za-z]*\)(link: String).*/\1/p' \
    composeApp/src/iosMain/kotlin/app/cadence/MainViewController.kt)
[ -n "$opener" ] || {
    echo "the Apple link entry point could not be read from MainViewController.kt —" \
        "this check greps for nothing" >&2
    exit 1
}

launch_link=$(counted "the launch link in $activity" \
    "$(grep -cF 'intent?.dataString' "$activity" || true)")
later_links=$(counted "onNewIntent in $activity" \
    "$(grep -cF 'override fun onNewIntent' "$activity" || true)")
# The link has to survive a recreation with the answer to it, which is held beside it: without
# these two the four Compose tests on recreation stay green — they hand the link to CadenceRoot
# directly — while on Android it is gone by the time the composition comes back.
kept_link=$(counted "onSaveInstanceState in $activity" \
    "$(grep -cF 'override fun onSaveInstanceState' "$activity" || true)")
restored_link=$(counted "the kept link read back in $activity" \
    "$(grep -cF 'savedInstanceState?.getString' "$activity" || true)")
# Through the comment stripper, because the tags survive inside <!-- -->: the intent-filter check
# above pays for exactly this, measured on a manifest with a deep link parked in a comment.
single_top=$(counted "launchMode in $manifest" \
    "$(uncommented "$manifest" | grep -cF 'android:launchMode="singleTop"' || true)")
if [ "$launch_link" -eq 0 ] || [ "$later_links" -eq 0 ] || [ "$single_top" -eq 0 ] ||
    [ "$kept_link" -eq 0 ] || [ "$restored_link" -eq 0 ]; then
    echo "$activity does not read the link it was opened with, does not keep it across a" \
        "recreation, or $manifest does not make the activity singleTop — an invitation reaches" \
        "Android and stops there" >&2
    exit 1
fi

apple_links=$(counted "onOpenURL in $swift_host" \
    "$(grep -cF '.onOpenURL' "$swift_host" || true)")
apple_handover=$(counted "$opener in $swift_host" \
    "$(grep -cF "$opener(" "$swift_host" || true)")
if [ "$apple_links" -eq 0 ] || [ "$apple_handover" -eq 0 ]; then
    echo "$swift_host does not hand what onOpenURL gives it to $opener —" \
        "an invitation reaches iOS and stops there" >&2
    exit 1
fi

echo
echo "kmp gate: green — :shared and :debugTools. composeApp's Compose UI tests did NOT run;"
echo "                 they need ios.sh and a macOS host with Xcode."

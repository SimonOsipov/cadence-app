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
# A boundary before the name, because these are substrings of ordinary identifiers: the vendor's
# own `resetPasswordForEmail` ends in `setPasswordForEmail` and tripped this check the day a
# recovery call was written. Matching a helper's name inside another word reports a caller that
# does not exist, which is the shape that gets a guard switched off rather than fixed.
callers=$(grep -rnE --include='*.kt' --include='*.swift' \
    "(^|[^A-Za-z])(setBearerToken|setAccessToken|setApiKey|setUsername|setPassword)" \
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
recover=$(sed -n 's/.*RECOVER_LINK: String = "\([^"]*\)".*/\1/p' \
    shared/src/commonMain/kotlin/app/cadence/shared/auth/Invitation.kt)
scheme=${accept%%://*}
host=${accept#*://}
recover_host=${recover#*://}

# Shape, not merely non-empty: measured by turning the constant into "$SCHEME://accept", which sed
# reads as the literal `$SCHEME` — a non-empty answer that then fails every grep below for the
# wrong reason. An empty one would be worse still: `grep -c ""` matches every line and the checks
# would pass having measured nothing.
case $scheme$host$recover_host in
    *[!a-z0-9.+-]* | "")
        # Braced deliberately: bash reads the first byte of a following multibyte character as
        # part of the name, and `$accept»` is then an unbound variable rather than the message.
        echo "'${accept}' and '${recover}' are not both deep links — one of ACCEPT_LINK and" \
            "RECOVER_LINK could not be read from Invitation.kt" >&2
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
registered() {
    awk -v scheme="android:scheme=\"$scheme\"" -v host="android:host=\"$1\"" '
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
' "$manifest"
}

# Both links, each asked for a filter of its own: a filter is matched whole, so a second host on
# the first one would not register anything the first does not already cover.
for landing in "$host" "$recover_host"; do
    if ! registered "$landing"; then
        echo "$manifest has no single intent-filter carrying $scheme, $landing, VIEW, DEFAULT and" \
            "BROWSABLE together — that link opens nothing on Android" >&2
        exit 1
    fi
done


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

# The rest of the chain an invitation travels, and no single stack's gate would notice a break in
# it. Since the interstitial went in, the template no longer names the scheme at all: it names a
# page on the dashboard, and the page names the scheme. So all three are asked — the page hands the
# token to the address the Kotlin constant declares, the template sends patients to that page, and
# the dashboard actually serves it. Two of the three files are outside kmp/, and this is the only
# gate that can see the whole line.
page=../web/src/features/auth/open-in-app-page.tsx
routes=../web/src/app.tsx

# Both mails travel the same three links, and the page's own path is the scheme's host by
# construction — cadence://accept is served from /accept — so one reading covers the route and the
# address that has to reach it.
for landing in "$accept:../api/mail-templates/invite.html" \
    "$recover:../api/mail-templates/recovery.html"; do
    link=${landing%%:../*}
    template=../${landing#*:../}
    at=${link#*://}

    hands_over=$(counted "$link in $page" "$(grep -cF "'$link'" "$page" || true)")
    if [ "$hands_over" -eq 0 ]; then
        echo "$page does not hand the token to $link — that mail reaches the dashboard and stops" >&2
        exit 1
    fi

    served=$(counted "the /$at landing in $routes" "$(grep -cF "path: '/$at'" "$routes" || true)")
    if [ "$served" -eq 0 ]; then
        echo "$routes serves no /$at — the address that mail names is a redirect to the" \
            "dashboard's door" >&2
        exit 1
    fi

    sent=$(counted "the /$at page in $template" "$(grep -cF "/$at#token_hash=" "$template" || true)")
    if [ "$sent" -eq 0 ]; then
        echo "$template does not send patients to /$at — the page is served for an address no" \
            "mail names" >&2
        exit 1
    fi
done

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
# XML with every <!-- --> span cut out. A second copy of what the intent-filter check does inline,
# because deleting that one would rewrite every `line` reference in a guard already measured.
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
# No Compose test can see this half: they compose the tree themselves, and it is Android that takes
# the activity away. What these two keep is the link the activity is answering, which is not
# necessarily the intent the system hands a recreated one.
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

# Kotlin source with its `//` comments cut and KDoc bodies dropped, for the greps below. Weaker than
# `uncommented()` above, which cuts XML spans within a line as well: a one-line KDoc and a `/* */`
# opening on its own line both survive this, so a call parked inside one would still be counted.
# Checked against the eleven files it is applied to below — none carries `//` inside a string
# literal, which is the other thing that would make cutting from it wrong.
kotlin_code() {
    sed -e 's|//.*||' "$@" | grep -v '^[[:space:]]*\*'
}

# Not Russian, and the one thing in the block a patient reads that is not.
THE_BRAND=Cadence

# The three screens this block designed from tokens, and not composeApp at large: the screens
# ported from the prototype already carry FontWeight and Color.Transparent, so a blanket rule
# would fail on work this step did not do.
echo "==> the sign-in block draws from the theme and speaks Russian"
block_dir=composeApp/src/commonMain/kotlin/app/cadence
block_screens=(
    "$block_dir/SignInScreen.kt" "$block_dir/SignInPrompt.kt"
    "$block_dir/AcceptanceScreen.kt"
    "$block_dir/RecoveryScreen.kt" "$block_dir/RecoveryPrompt.kt"
    "$block_dir/CadenceSplash.kt" "$block_dir/PasswordWords.kt"
)
block_copy=(
    "$block_dir/SignInCopy.kt" "$block_dir/AcceptanceCopy.kt"
    "$block_dir/RecoveryCopy.kt"
)
for f in "${block_screens[@]}" "${block_copy[@]}"; do
    [ -f "$f" ] || {
        echo "$f is not there — this check greps for nothing" >&2
        exit 1
    }
done

# File by file, because `kotlin_code` over the whole list would number lines into a concatenation
# with the comments already deleted — a position that resolves to nothing a reader can open.
own_paint=""
for f in "${block_screens[@]}" "${block_copy[@]}"; do
    own_paint+=$(kotlin_code "$f" |
        grep -nE 'Color\(|Color\.|ui\.graphics\.Color|FontFamily|FontWeight|FontStyle|ui\.text\.font' |
        sed "s|^|$f:|" || true)
done
if [ -n "$own_paint" ]; then
    echo "the sign-in block brought a colour or a face of its own rather than a CadenceTheme token:" >&2
    echo "$own_paint" >&2
    exit 1
fi

# The screens as well as the copy objects, because «the copy lives in the objects» was itself only
# a convention: a literal written straight into a screen went past the first version of this rule.
# Empty literals are excluded by `+` — `mutableStateOf("")` is a field's initial state, not copy —
# and the comments are cut first, or a Go status name quoted inside one would answer for a screen.
#
# «Not pure ASCII» rather than a Cyrillic range, and measured rather than reasoned: under LC_ALL=C
# a multibyte bracket range degrades to a byte range and *over*-matches — `[а-яА-Я]` matched a line
# of «»— alone on BSD grep 2.6.0, while `[^ -~]` answered identically in C and in UTF-8. The cost
# is that a literal of punctuation alone would pass; there is none, and the count below is what
# notices if the objects ever go empty.
latin_copy=$(kotlin_code "${block_copy[@]}" "${block_screens[@]}" |
    grep -ohE '"[^"]+"' | grep -v '[^ -~]' | grep -vxF "\"$THE_BRAND\"" || true)
if [ -n "$latin_copy" ]; then
    echo "the block's copy says something that is not Russian:" >&2
    echo "$latin_copy" >&2
    exit 1
fi

russian_copy=$(counted "the block's Russian copy" \
    "$(kotlin_code "${block_copy[@]}" | grep -ohE '"[^"]+"' | grep -c '[^ -~]' || true)")
if [ "$russian_copy" -eq 0 ]; then
    echo "the block's copy objects hold no Russian at all — the refusal above measured nothing" >&2
    exit 1
fi

# No Compose test reaches the platform-facing root, so these two greps are the whole of what stands
# behind it. The parens keep the imports from answering for the calls.
root=$block_dir/CadenceRoot.kt
zone_reported=$(counted "the zone report in $root" \
    "$(kotlin_code "$root" | grep -cF 'reportZoneWhileSignedIn(' || true)")
if [ "$zone_reported" -eq 0 ]; then
    echo "$root never reports the device's zone — a patient who flew keeps the schedule they left" >&2
    exit 1
fi

# The shared transport and not one per composition: built without an engine the client owns the
# engine it makes and nothing closes it, so the plain constructor here leaks a pool on every
# activity recreation.
shared_transport=$(counted "the shared transport in $root" \
    "$(kotlin_code "$root" | grep -cF 'cadenceHttpClientFor(' || true)")
if [ "$shared_transport" -eq 0 ]; then
    echo "$root builds its own API transport — an engine leaks on every activity recreation" >&2
    exit 1
fi

echo
echo "kmp gate: green — :shared and :debugTools. composeApp's Compose UI tests did NOT run;"
echo "                 they need ios.sh and a macOS host with Xcode."

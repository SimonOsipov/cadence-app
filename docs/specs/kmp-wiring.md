---
type: spec
project: cadence
status: approved
priority: p2
created: 2026-07-28
todoist_parent: "6h8xwxQ2mQWC4Mpq"
components: [kmp-app]
proposal: "[[20-Projects/cadence/architecture/proposals/api-openapi-code-first|architecture/proposals/api-openapi-code-first]]"
---
<!-- SNAPSHOT (read-only copy). Master: 20-Projects/cadence/specs/kmp-wiring.md in vault prll-vault. Edit the vault note, then re-export — never edit here. -->

# KMP Wiring

## Summary

Teach the mobile app to talk: a typed client for our API, sign-in through Supabase Auth, the session in the platform's secure storage, and a hidden debug screen showing live responses from the deployed dev API.

Today `kmp/` can draw a placeholder and nothing else — there is no networking in it at all. After this block, the M2 and M3 screens are added as calls against a finished client.

The first draft got NEEDS-REWORK from the judge, and three findings changed it substantively: the stated mechanism for keeping the debug screen out of release does not physically exist in this project; a step declared unblocked required test infrastructure that does not exist; and a second token refresh was being introduced on top of the one `supabase-kt` already owns, which under refresh-token rotation means a silent sign-out. This draft was written after verifying mechanisms, not just versions.

## User Story

**As a** developer about to port the sign-in (M2) and dose-logging (M3) screens
**I want** a typed client, working sign-in, and a session that survives a restart and lives where it belongs
**So that** a screen is a method call and a rendered response, not a negotiation with transport, tokens, and storage

## Acceptance Criteria

**Secure storage**
- [ ] The session and the PKCE verifier cache live in the platform's secure storage. On iOS — Keychain with `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly` and `kSecAttrSynchronizable = false`: otherwise the patient's session goes into an encrypted backup and travels to a new device
- [ ] On Android — our own encryption with a Keystore key (AES/GCM) over a file: `androidx.security:security-crypto` is deprecated with no direct replacement, and cannot be relied on
- [ ] The Keychain survives app deletion — on the first launch after a reinstall the storage is wiped via a marker, otherwise somebody else's session comes back to life
- [ ] Unreadable storage (a changed lock screen, corruption, a reinstall) is treated as "no session", not as a crash
- [ ] Acceptance is enforced by the gate: Android test infrastructure with a runtime is created (Robolectric or `androidDeviceTest` with an emulator in CI), and `scripts/gate/kmp.sh` runs it. The new source sets are added to the detekt allow-list in `kmp/build.gradle.kts` — otherwise it silently does not analyse them
- [ ] A spike is done: does the Keychain work from `iosSimulatorArm64Test`? If not (`errSecMissingEntitlement` is expected) the test moves to an XCTest host in `iosApp/`, and that is written down

**Client**
- [ ] The API base is taken from the build configuration through a **named mechanism**: modules on `com.android.kotlin.multiplatform.library` have neither `buildConfig` nor build variants, so this requires a Gradle task generating a Kotlin file into a registered `srcDir`, or `BuildKonfig`
- [ ] A release build with a dev address **fails**, and that is an enforced rule, not an intention
- [ ] The client is generated from `openapi.json` with `openapi-generator`; only `apis/`, `models/`, and `infrastructure/` land in the repository — the generator emits an entire standalone Gradle project with wrappers, tests in a source set invalid for KMP, and **no** `androidTarget()`, so it cannot be wired in as-is
- [ ] Generated code is excluded from ktlint and deliberately decided for detekt (its path allow-list does not contain `generated/` — silent blindness is worse than a red gate)
- [ ] The gate fails if regeneration changes the committed output
- [ ] The Ktor version is aligned in the version catalog. Today both sides are literally on 3.5.1, but the pair will diverge on the first update to either
- [ ] Type mapping is verified on the dose: what huma emits into the schema is decided so that the generated Kotlin is precise. The generator maps `BigDecimal` → `Double` and `UUID` → `String`; for `{value, unit}` this gets settled here rather than discovered in M3

**Token and refresh**
- [ ] There is **one** owner of token refresh — the `supabase-kt` Auth module. There is no separate refresh in the transport: two independent mechanisms under refresh-token rotation mean a revoked session
- [ ] Refresh is serialized: several concurrent 401s produce **exactly one** refresh call and a successful retry of every request. Proven by a `MockEngine` test with three or more concurrent 401s
- [ ] The token is attached by the transport layer, not by each call; only as a header, never in the query string
- [ ] If the Ktor logging plugin is installed — it has `sanitizeHeader` on `Authorization`, verified by a test with an intercepting logger
- [ ] Types carrying a token do not print it in `toString()` — verified by a test

**Debug screen**
- [ ] The screen lives in a **separate Gradle module** `:debugTools`, not in a build-variant source set: `composeApp` has no variants, and an unregistered directory is ignored by Gradle silently — the screen would have ended up nowhere while acceptance passed vacuously
- [ ] On Android the module is wired in through `debugImplementation` from `:androidApp`, where the variants are real; on iOS — by a build property, and the release build is built without it
- [ ] Acceptance is a **grep of the release artifact** for the screen's class name. That is the only check that holds regardless of the mechanism
- [ ] The screen obtains a token through Supabase Auth with the test user from SKL-01 and shows a live `GET /v1/me`
- [ ] `/healthz` is called with a **raw call** on a client instance without the auth plugin: it is deliberately outside the OpenAPI spec, so it is absent from the generated client and there is nothing to attach a Bearer to
- [ ] Three states are distinguishable: API unavailable · refresh succeeded and the retry went through · refresh failed and the session was cleared. "Expired" and "401" must not be separated — the API returns them with an indistinguishable body on purpose

## Scope / Non-scope

**In scope:** secure storage together with the test infrastructure for it, API base configuration, generating and consuming the client, transport with a single refresh owner, the debug module, `MockEngine` tests.

**Out of scope, named explicitly:**
- The sign-in and invite-acceptance screens — M2.
- Photo upload to Storage — M3.
- **A persistent read cache** and the dose retry queue with an idempotency key (invariant 6) — M3. This is the analogue of the partner's React Query task: the cache layer is deferred, not absent.
- Sending the timezone (invariant 5) — M2, together with real sign-in.
- Biometric lock (invariant 3, second half) — M10.
- Certificate pinning — not in the MVP.
- Crash reports. The claim "tokens do not reach crash reports" is only verifiable where Sentry exists — that is the "Deploy and Observability" block.

**Blocking:**

| What is needed | From where | What is impossible without it |
|---|---|---|
| `openapi.json` | "API Skeleton", step 2 | generating the client (step 2) |
| a Supabase project, a test user | SKL-01 (manual) | the debug screen (step 3) |
| a deployed dev API | SKL-06 (manual) | the debug screen (step 3) |

Step 1 is not blocked by external tasks, **but it is not free either**: it brings Android runtime test infrastructure into the project, which does not exist today, and it requires a Keychain spike. That is the step's work, not a precondition for it.

## What already exists (DONE)

Verified against the code on 2026-07-28; the numbers were corrected after review.

- **Three** Gradle modules: `:shared`, `:composeApp`, `:androidApp`. `iosApp/` is an XcodeGen project, not a build module
- `shared` contains exactly the `Platform` seam (`expect` plus two `actual`s) and three tests for it. No networking, no storage, no domain models
- The design system in `composeApp/…/design`: palette, radii, spacing, typography, theme, **41** icons (the number is pinned by an assertion in a test), **16** public composables, counted as functions annotated `@Composable`: five text, four controls, four surfaces, two icon-render overloads, and the theme itself. The "nine primitives" from the first draft and the "14" from the judge's feedback are different counting rules; here the rule is named so the number can be re-checked
- Versions: Kotlin 2.4.10, Compose MP 1.11.1, AGP 9.3.1, Gradle 9.6.1 by checksum — all four confirmed
- **17** test functions producing **18** runs: `PlatformTest` from `commonTest` executes on both the Android host and the iOS simulator. Compose UI tests run only on the iOS target — the Android target deliberately has no host-test builder
- The `scripts/gate/kmp.sh` and `ios.sh` gates, the `kmp-android` and `kmp-ios` jobs in CI
- **No release build exists on either platform**: `ios.sh` builds Debug only, and `:androidApp`'s release has no minification and no signing. The acceptance criterion "check the release artifact" requires a build path that does not yet exist — that is part of step 3

**What does not exist at all:** Ktor, serialization, storage, DI, navigation, domain models, `supabase-kt`, logging (neither Napier nor Kermit). This is the first block bringing external dependencies into `kmp/` beyond Compose.

## Technical detail

**Versions verified by the judge:** `openapi-generator` 7.24.0 · `supabase-kt` 3.7.0 · Ktor 3.5.1 on both sides literally · `multiplatform-settings` 1.3.0 — **not optional**, `supabase-kt` depends on it hard.

A documentation trap: the description of the `library=multiplatform` option in the generator's reference still says "Ktor 1.6.7". That has been untrue since 2024 — the truth is in the build template, which pins Ktor 3.5.1.

**Storage: the default is bad on both platforms.** `SettingsSessionManager` from `supabase-kt` stores the entire session, refresh token included, in plaintext: in `SharedPreferences` on Android, in `NSUserDefaults` on iOS. The first draft of this spec claimed the problem was Android-only — that was an understatement. Separately: `codeVerifierCache` has the same plaintext default and sits on the PKCE invite-acceptance path in M2; it has to be substituted too.

**A cheaper path than our own `SessionManager`.** The judge pointed it out, and it is better: keep `SettingsSessionManager`, but hand it our own `Settings` — `KeychainSettings` on Apple, encrypted storage on Android. Less of our own code on the authentication path of a medical app, and the extension point is a supported one. A custom `SessionManager` implementation stays the fallback if the `Settings` parameters do not reach the Keychain accessibility class we need.

**A debug module, not a source set.** `composeApp` (the `com.android.kotlin.multiplatform.library` plugin) has neither `buildTypes` nor variants — there is a single Android compilation. A `debugMain` directory would have been ignored silently. Hence the separate `:debugTools` module: on Android it is wired in with `debugImplementation` from `:androidApp`, where `com.android.application` and real variants exist; on iOS — by a build property. There is one check and it does not depend on the mechanism: grep the release artifact.

**Token refresh — one owner.** `supabase-kt` has its own auto-refresh. A second mechanism in the transport creates a race: Supabase rotates refresh tokens, and several concurrent refreshes with one token inside the reuse-detection window mean a revoked session. Refresh is delegated to Auth and serialized; if the stock Ktor `Auth` plugin with `bearer` is used, it serializes on its own — in which case that should be named rather than reimplemented.

**Consuming the generated code.** The generator emits a standalone project: its own `build.gradle.kts`, `settings.gradle.kts`, wrapper, `docs/`, tests in `src/test/kotlin` (an invalid source set for KMP), and its build has no `androidTarget()` — an included build is impossible in principle. The strategy: generate into a separate directory, copy only `apis/`, `models/`, and `infrastructure/` into a registered `srcDir`, `.openapi-generator-ignore` for the rest, and declare the Ktor engine by hand (the template pulls the obsolete `ktor-client-ios` instead of `ktor-client-darwin`).

**Type mapping is a decision, not a consequence.** The generator maps `BigDecimal` to `Double`, `UUID` to `String`, `object` to `String`. The project convention is "numbers are data, a dose is `{value, unit}`". So the shape huma emits into the schema for a dose is chosen here, so that the generated Kotlin is precise. Adjacent: the multiplatform generator has an open bug serializing enums by constant name instead of value — any enum in the contract will hit it.

**What can actually be claimed about logs.** There is no logging in `kmp/` today. Exactly two things are verifiable: `sanitizeHeader` on `Authorization`, if a logging plugin ever appears, and that a token-carrying type does not print it in `toString()`. The crash-report claim was removed — its place is where Sentry exists.

**Files:**

```
kmp/
  shared/src/
    commonMain/kotlin/app/cadence/shared/
      net/ApiConfig.kt            generated by a build task
      net/ApiClient.kt            a wrapper over the generated client
      auth/SupabaseAuth.kt        sign-in and refresh via supabase-kt
      storage/SecureSettings.kt   expect: Settings over secure storage
      generated/                  apis/, models/, infrastructure/ — those only
    androidMain/…/SecureSettings.android.kt   Keystore AES/GCM
    iosMain/…/SecureSettings.ios.kt           Keychain, AfterFirstUnlockThisDeviceOnly
    commonTest/…                  MockEngine: transport, concurrent 401s, sanitize
  debugTools/                     a separate module: the debug screen
```

## Architecture decision

Both forks are closed by decisions in the [[20-Projects/cadence/architecture/proposals/api-openapi-code-first|proposal note]]: the client is generated from `openapi.json`, and Supabase Auth is reached through `supabase-kt`. No new note is required — this is the client half of an already approved choice.

What changed after the review: storage substitution is done not with our own `SessionManager` but with our own `Settings` under the stock manager — less of our code on the authentication path; token refresh is not duplicated but delegated to its owner; the debug screen moves into a module, because the mechanism from the first draft does not exist in this project.

The main trade-off is unchanged: `supabase-kt` covers three milestones' worth of work, but brings a third-party dependency onto the authentication path of a medical app and by default stores the session in plaintext on both platforms. Substituting the storage is a condition of the choice.

## Component deltas

### kmp-app.md
- MODIFIED: "Shape" — `shared/` gains a client generated from `openapi.json`, a wrapper over it, and secure storage behind `expect/actual`; a `:debugTools` module appears, absent from release builds.
- ADDED: to "Shape" — `supabase-kt` as the way to reach Supabase Auth, with mandatory substitution of the session storage and the PKCE verifier cache.
- MODIFIED: invariant 3 — clarified that the library default does not satisfy it on **either** platform and gets substituted; for the Keychain the accessibility class and the synchronization ban are pinned.
- ADDED: the single owner of token refresh is the Auth module; refresh is serialized.

### api.md
- REMOVED: from "Open questions" — choosing the Kotlin client generator. Decided.
- ADDED: to "Contracts" — the shape in which a dose is emitted into the schema is chosen so that the generated Kotlin client is precise.

## Decomposition

Mapping to the original tasks: all three steps close SKL-11.

### step-1: Secure storage and the test infrastructure for it

`Settings` over Keychain and Keystore behind `expect/actual`, with explicit accessibility parameters and a wipe after reinstall. Unreadable storage is "no session", not a crash.

This is also where what makes it checkable appears: Android tests with a runtime (Robolectric or a device in CI), an extension of `scripts/gate/kmp.sh`, the new source sets in the detekt allow-list. Plus the Keychain spike from `iosSimulatorArm64Test` with the XCTest-host fallback.

> [!deviation] 2026-08-29
> **The Apple `Settings` is ours, not the library's — and the first reason recorded here was
> false.** It said `KeychainSettings(serviceName: String)` is the whole constructor, so the two
> attributes could not reach it, and cited that as measured. It was not measured; it was read
> off documentation that lists one constructor. Review objected, and the 1.3.0 klib settles it:
> the class carries a second `vararg defaultProperties: Pair<CFStringRef?, CFTypeRef?>`
> constructor, and the attributes do reach it. The lesson is one this project has already
> written down — measure the library, not the docs — and it was not applied.
>
> Two reasons stand in its place. That constructor is `@ExperimentalSettingsApi` on top of the
> class's own `@ExperimentalSettingsImplementation`, which is two experimental opt-ins on the
> authentication path of a medical app; and `KeychainSettings` stores one keychain item per
> key, while this store is one blob — and the blob is what carries «read it or do not overwrite
> it», the invariant round one's critical was about. The spec's fallback clause is conditional
> on the attributes being unreachable, so **this is a deviation from the spec's condition, not
> a use of its escape hatch**, and it is worth re-opening if the per-key shape is ever wanted.
>
> **Robolectric ships no AndroidKeyStore provider** — `KeyStore.getInstance` answers
> «AndroidKeyStore not found», measured. So `AndroidVault` takes its key as a parameter with the
> Keystore as the default: the format, the IV's place, tampering, wiping and a key that cannot
> be produced are all measured, and what stays unmeasured is the key acquisition itself, which
> is platform API rather than ours. `androidDeviceTest` with an emulator was refused, because
> `scripts/gate/kmp.sh` promises to run on any machine including Linux. **Named gap:** the
> `KeyGenParameterSpec` — block mode, padding, 256 bits — has no executable witness on any
> runtime.
>
> **`secureSettings` takes a name, which the spec did not say.** The acceptance criteria put the
> session *and* the PKCE verifier in secure storage, and the store is written back whole — one
> store shared between them would have the session's next write drop the verifier out of it,
> silently, in the middle of accepting an invite. One instance per name, and one fresh-install
> marker per store: a single marker is put down by whichever secret is asked for first and
> would retire the guard before the second was ever touched.
>
> **Review found a critical and four majors, and the critical was invisible from inside.** A
> YAML indent hung the Kotlin framework build phase on the new test target, so nothing produced
> the framework `iosApp` links; the gate was green here only because the directory was already
> on disk, and a clean clone — which is what CI is — would have failed. Re-verified by moving
> the built framework away first.
>
> **«Unreadable is no session» needed its second half.** `Vault.read` answers Absent, Present or
> Unavailable, and a store loaded from Unavailable refuses to write: the blob goes back whole,
> so one write after one failed read replaced a live session with nothing. A device locked
> between boot and first unlock is exactly that case, and it is the case a background token
> refresh runs in. The test certifying the criterion asserted the reads and never that the bytes
> survived. Statuses are answered rather than discarded on both platforms, and the fresh-install
> guard marks itself only after a wipe that answered.
>
> **The gate was structurally blind to these.** detekt's `SwallowedException` and
> `TooGenericExceptionCaught` allow two parameter names by default, `_` and `expected` — the
> first is what Kotlin encourages for an unused parameter and the second is what a test writer
> reaches for, so between them the default arms nothing. Narrowing to drop `_` alone left every
> site invisible under the other name, which is what the second round caught. The allow-list is
> empty now: a swallow is justified at its site or it is not justified. **Named gap:** there is no
> logging in `:shared` at all, so every refusal above is silent to a developer as well as to the
> patient. Not fixed here — it is a dependency decision, and it belongs with the transport.
>
> **Round two found four majors, every one inside round one's repairs.** `Vault.write` was given
> an answer nobody read, so a refused write still looked kept. `remove` was gated by the same
> flag as writing, so signing out of a store that could not be read did nothing at rest — the
> opposite sign of the trade the flag exists to make. Android collapsed «corrupt» into «cannot
> read now», so one kill mid-write would have left the store unwritable for the life of the
> installation. And the citation above was wrong. **Named gap:** the per-name instance map is
> not atomic, which becomes reachable when step 2 puts the session manager on a background
> dispatcher.

todoist: "6h8xx8WR6rgcmxpq"

### step-1a: An XCTest host for the Keychain suite

Opened by step 1's spike, which the spec named as a fallback without decomposing it.

**Measured 2026-08-29.** `SecItemAdd` from `iosSimulatorArm64Test` answers -25291,
`errSecNotAvailable`: a Kotlin/Native test binary is not an app bundle and has no keychain.
The spec predicted -34018, `errSecMissingEntitlement` — the outcome it named was right and
the code it guessed was not.

An `iosAppTests` bundle hosted by `iosApp`, `:shared` exported from the ComposeApp framework
so its types reach the Objective-C header, the suite moved there, and `ios.sh` running it.
One test is the reason the others are not enough: both keychain attributes are read back off
the item, because no behavioural assertion can see them.

todoist: "6hPMWf3X2QP9fmvq"

### step-2: The client from `openapi.json`, configuration, and transport

⏸ **Requires `openapi.json`** from the "API Skeleton" block, step 2.

Generation, copying only the needed directories, exclusion from ktlint and a decision for detekt, the drift gate. The API base from the build through a named mechanism plus an enforced rule against a release with a dev address. Aligning Ktor. The type-mapping decision for the dose. Transport with a single refresh owner and a test for concurrent 401s.

todoist: "6h8xx8gf3vH47vPH"

### step-3: Sign-in through Supabase and the debug module

⏸ **Requires SKL-01 and SKL-06.**

`supabase-kt` with its `Settings` from step 1, including `codeVerifierCache`. The `:debugTools` module: sign-in with a test user, `GET /v1/me` through the generated client, `/healthz` as a raw call without the auth plugin, three distinguishable states. Acceptance includes a release build path that does not exist today, and a grep of the artifact.

todoist: "6h8xx8cjPvPr5PXq"

## Open questions

> [!decision] 2026-07-29 — **one endpoint is enough; step 3 closes here.** The screen proves the pipeline: generation, the `Authorization` header, deserialization, and the error shape. A second endpoint says nothing new about the pipeline — it says something about the endpoint. Waiting for M2 would mean holding the entire KMP block open for a whole milestone for a claim that is already proven.

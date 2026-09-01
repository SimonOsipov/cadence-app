---
type: spec
project: cadence
status: approved
priority: p1
created: 2026-07-30
todoist_parent: "6h9MFpWmv25CjwmH"
components: [kmp-app, identity]
proposal: "[[20-Projects/cadence/architecture/proposals/first-live-read-and-sign-in|architecture/proposals/first-live-read-and-sign-in]]"
---

<!-- SNAPSHOT (read-only copy). Master: 20-Projects/cadence/specs/mobile-sign-in.md in vault prll-vault. Edit the vault note, then re-export — never edit here. -->

# Patient sign-in: session, navigation, three screens

## Summary

The patient opens the email, lands in the app, **sets a password**, and ends up
inside with a role in the token. This block lays protected navigation on top of
the session from "KMP Wiring", plus three screens the frozen prototype does not
have — it begins with a user who is already signed in. They are designed out of
the existing tokens.

The spec was rewritten after the first review. What was reversed is the central
thing: accepting an invite without a password was not a property of GoTrue but a
choice of ours, and a bad one — a patient who lost their session to an ordinary
event stayed dependent on email, while the sign-in screen asked for a password
that did not exist.

## User Story

**As a** clinic patient
**I want** to accept the invite, set a password, and not retype it on every launch
**So that** the app opens where I left it, and losing the session does not mean
waiting for an email

## Acceptance Criteria

- [ ] The `cadence://` scheme is registered on both platforms — an intent-filter in the Android manifest and `CFBundleURLTypes` on iOS; without it the invitation opens nothing
- [ ] The invitation **leads into the app, not into a browser**: the Russian template links to `cadence://accept?token_hash={{ .TokenHash }}`, and the app exchanges that token itself with `POST /verify`, taking the session out of the response body. Neither token ever rides in a URL fragment, and the redirect allow-list does not govern this path at all
- [ ] PKCE is **not** the acceptance flow, and that is measured rather than chosen: GoTrue v2.194.0 accepts a `code_challenge` on the admin route and ignores it. See [[20-Projects/cadence/architecture/proposals/invite-acceptance-without-pkce|the proposal]]. The verifier cache stays installed as groundwork for a future provider sign-in, with the reason recorded beside it
- [ ] A cold start from the link lands on the acceptance screen, not on the home screen and not on sign-in
- [ ] The acceptance screen **requires** setting a password: `PUT /user` from the link's session allows it, which makes this our choice. Without a password, losing the session would leave the patient dependent on email and on the rate limit
- [ ] The link is single-use: following it again yields `otp_expired`, and the screen explains that in Russian — GoTrue's own strings are English, so the copy is ours to write
- [ ] Navigation splits into a pre-sign-in and a post-sign-in area; without a valid session the second is unreachable by direct navigation, by the system back button, or by deep link
- [ ] Launching with a valid session opens the app **straight** inside: an intermediate sign-in screen reads as having been signed out
- [ ] A session expiring in the background routes to sign-in **once**, not once per concurrent request — token refresh has a single owner, and navigation does not break that property
- [ ] Unreadable storage means "no session", not a crash
- [ ] The sign-in screen accepts an address and a password; the refusal does not distinguish "no such address" from "wrong password" — a refusal that distinguishes turns the form into an existence oracle
- [ ] Signing out works and leads to the sign-in screen; with a mandatory password it has stopped being a voluntary lockout
- [ ] Recovery calls `POST /recover`, does not confirm that an address exists, and explains the rate limit when it fires; returning via the link leads to setting a new password
- [ ] `POST /v1/me/session` is called on sign-in and on every launch, carrying the device timezone; a timezone changed between launches gets through
- [ ] The three screens are drawn with `CadenceTheme` tokens and introduce no new colours or fonts — enforced by a rule, not by a promise
- [ ] Screen and refusal copy is Russian, enforced by the same rule
- [ ] `AppTest.kt` (`appShowsTheBrand`, `appShowsThePlatformItRunsOn`) asserts the contents of the `App.kt` placeholder that step 1 replaces with a navigation host — both tests get rewritten, and that is named here rather than discovered by the build
- [ ] Coverage limits are named honestly: Compose UI tests today run only on the iOS target (`kmp-wiring` has no Android host-test builder), and `otp_expired` behaviour is verified against the live GoTrue that stands in the **api** harness, not in `kmp/` — so what is verified here is a copy, not the behaviour
- [ ] The KMP gate is green

## Scope / Non-scope

**In scope.** Scheme registration and the PKCE flow. Protected navigation. The
sign-in, invite-acceptance-with-mandatory-password, and recovery screens. Signing
out. Calling `POST /v1/me/session` on sign-in and on launch. Compose UI tests.

**Out of scope.** The twenty-four product screens — ported from M3. Biometric
lock — M10. Universal Links and App Links — "Deploy"; `cadence://` is what works
here. The iOS E2E tool — an open question on `kmp-app`, and it stays open.
Session storage and client generation — "KMP Wiring". The `POST /v1/me/session`
endpoint — the neighbouring block; only the call belongs here.

**Blocked and flagged.** The criterion "the debug screen is absent from the
release artifact" has nothing to check against: no release build exists on either
platform, it is produced by step 3 of "KMP Wiring", which is blocked on SKL-01
and SKL-06. Here the property is inherited and not re-verified.

## What already exists (DONE)

**None of the following — they are `approved` prerequisites, not things in the
repository.** "KMP Wiring" (session in secure storage, client from
`openapi.json`, a single owner of token refresh, the PKCE cache, a debug module
kept out of release), "Onboarding" (invites, Russian templates), "First Live
Read" (`POST /v1/me/session`).

What does exist and work: `kmp/composeApp` with the design system —
`CadenceTheme`, `CadenceColors`, `CadenceTypography`, `CadenceText`,
`CadenceIcon`, `CadenceSurfaces`, `CadenceControls`, `CadenceDimens` — and the
`App.kt` placeholder covered by `AppTest.kt`. There is not a single product
screen.

**Measured against live GoTrue on 2026-07-30:** following the link returns a
session and a refresh token, and does not set a password; `PUT /user` from that
same session **does** set one, after which password sign-in works; the link is
single-use, and a repeat yields `otp_expired`; `POST /recover` is public and
rate-limited per user.

**Measured again on 2026-09-01, against the digest-pinned v2.194.0:**
`POST /admin/generate_link` answers with `action_link`, `hashed_token` and
`email_otp` and needs no SMTP at all; following that link redirects to
`cadence://accept#access_token=…&refresh_token=…`, and sending a `code_challenge`
with the request changes nothing — it is accepted and ignored, so **PKCE is
unreachable for an admin-issued invitation**. `POST /verify` with an unspent
`token_hash` answers `200` with the whole session in the body. `{{ .TokenHash }}`
renders in the mail template. Incidentally: GoTrue will not hand `SMTP_USER` and
`SMTP_PASS` to a server that offers no STARTTLS — it answers `QUIT` and `/invite`
fails with `500`.

## Technical detail

### Scheme and flow

`cadence://` is registered by an intent-filter in the Android manifest and by
`CFBundleURLTypes` on iOS. The redirect allow-list does not enter into it: the
invitation does not travel through GoTrue's redirect at all.

**The invitation links straight into the app.** The Russian template renders
`cadence://accept?token_hash={{ .TokenHash }}`; the app takes that token and calls
`POST /verify` with type `invite`, receiving the session in the response body.
Against catching an implicit fragment this buys two things: the session never sits
in an address — which on Android any application declaring the same scheme can
read — and there is no browser hop in the middle.

PKCE was the approved flow and is not reachable: the admin route accepts a
`code_challenge` and ignores it, measured above. The `codeVerifier` cache stays
where "KMP Wiring" put it, as groundwork for a provider sign-in, and says so.

### The acceptance screen

Opens on a cold start with a link. Following the link already yields a session —
so the screen does not ask for a password as a condition of entry, it **requires
setting** one as the condition for completing acceptance. This is our choice, and
it closes three things: the sign-in screen stops asking for something that does
not exist, losing the session stops meaning waiting for an email, and signing out
stops being a lockout.

Following the same link a second time — an ordinary case, the email opened on two
devices — is explained in Russian.

### Navigation

Two areas, with the transition driven by session validity rather than by screen
order. Three leak paths are verified: direct navigation, the system back button,
and a deep link.

Launching with a session opens straight inside: a flash of the sign-in screen
reads as having been signed out, and in a medical app that reads as data loss.

Expiry in the background routes to sign-in once. Token refresh has a single owner
— the property comes from "KMP Wiring", and navigation is obliged not to break it.

### Sign-in, sign-out, recovery

Sign-in takes an address and a password, and refuses without distinguishing the
cause. Signing out leads to the sign-in screen. Recovery calls the public
`POST /recover`, does not confirm that an address exists, and explains the rate
limit when it fires: `MAILER_MAX_FREQUENCY` applies per user, and without an
explanation the patient sees silence.

### Rendering and verifiability

Three screens built from existing tokens: the prototype does not have them
because it starts with a signed-in user. The absence of new colours, new fonts,
and non-Russian strings is enforced by a rule — otherwise the criterion has no
way to go red.

Honestly about coverage: Compose UI tests run only on the iOS target, because
"KMP Wiring" has no Android host-test builder; and `otp_expired` cannot be
verified against live GoTrue here — the container stands in the `api` harness. In
both places what can be verified is, and what cannot is written down.

## Architecture decision

The block adds no contracts: it consumes the session from "KMP Wiring", invites
from "Onboarding", and the endpoint from "First Live Read".

The single substantive decision is the mandatory password on acceptance, and the
first draft got it wrong twice: it made the password optional and justified that
with "measured GoTrue behaviour". The measurement says the opposite — `PUT /user`
from the link's session does set a password. There was no constraint; there was a
poor choice. The analysis is in the
[[20-Projects/cadence/architecture/proposals/first-live-read-and-sign-in|proposal]].

## Component deltas

### kmp-app.md
- ADDED: to "Shape" — navigation splits into a pre-sign-in and a post-sign-in area; the transition is determined by session validity and verified against direct navigation, the system back button, and a deep link
- ADDED: to "Shape" — the `cadence://` scheme is registered on both platforms, and the invitation links into the app rather than through GoTrue's redirect: the app exchanges a `token_hash` for the session and neither token rides in a URL fragment
- ADDED: invariant — launching with a valid session opens the app straight inside: an intermediate sign-in screen reads as having been signed out
- ADDED: invariant — accepting an invite **requires** setting a password: the link sign-in is single-use, and without a password losing the session means depending on email
- MODIFIED: invariant 5 — the timezone is sent by calling `POST /v1/me/session` on sign-in and on every launch
- ADDED: to "Screens" — sign-in, invite acceptance, and password recovery are designed from tokens: the frozen prototype does not contain them
- ADDED: known limitation — Compose UI tests run only on the iOS target; GoTrue behaviour is verified in the `api` harness, and what runs here is only a copy

### identity.md
- ADDED: to contracts — accepting an invite requires setting a password: the link is single-use and is itself the credential, and without a password the patient depends on email every time the session is lost

## Decomposition

### step-1: Protected navigation on top of the session

Two areas, transition by session validity, launching with a session opens straight
inside, expiry in the background routes to sign-in once, unreadable storage means
"no session". `App.kt` is replaced by a navigation host, `AppTest.kt` is rewritten.

Tests: the three leak paths are closed; ten concurrent 401s produce one transition.
todoist: "6h9MFwg28gg442WH"

> [!deviation] 2026-08-31
> **Приёмка называла два теста, которых нет.** Пункт про `AppTest.kt` говорит про
> `appShowsTheBrand` и `appShowsThePlatformItRunsOn` — обе заглушки удалены коммитом `0cb752b`
> («показуха уступила приложению, которое подменяла») задолго до этого шага. В файле семь
> тестов, и они проверяют не заглушку, а перенесённые экраны: вкладку, полосу протокола,
> шторку записи, оба мастера. Переписаны все семь — каждый теперь подаёт сессию явно, потому
> что `App` принимает состояние. Ни один не ослаблен: утверждения те же.
>
> **Состояние сессии живёт в `:shared`, а не в `:composeApp`.** У `composeApp` нет андроидного
> host-test builder'а, поэтому решённое в нём меряется на одной платформе. Отображение статусов,
> дедупликация и разбор причин отказа — обычные тесты в `:shared`, идут везде; на Compose UI
> остаются только сами ворота.
>
> **Отказ обновления не переводит на вход, и это измерено, а не выведено.** Первая редакция
> сопоставляла `RefreshFailureCause.InternalServerError` с «вышел из сессии», прочитав название.
> В артефакте 3.7.0 `AuthImpl` ветвится по `NETWORK_ERROR_CODES`, который его статический
> инициализатор строит как `[500, 502, 503, 504, 520, 521, 522, 523, 524, 530]`: статус **из**
> списка даёт `InternalServerError`, сессия **сохраняется**, назначается повтор. **Любой** статус
> вне списка идёт в `clearSession()` и приезжает как `NotAuthenticated` — второго списка на этом
> пути нет (`SIGN_OUT_IGNORE_CODES` = `[401, 403, 404]` лежит рядом, но читается в `signOut`).
> Первая редакция этого абзаца приписывала тройку пути обновления: ложная цитата вместо ложной
> цитаты, найденная вторым раундом. Измерено по нашему GoTrue 31.08.2026: отозванный токен
> отвечает `400 refresh_token_not_found`. То
> есть обе причины `RefreshFailure` означают «ещё пытаемся», а единственный отказ —
> `NotAuthenticated`, и именно он служит пункту «истечение в фоне переводит на вход». Прежнее
> отображение выбрасывало бы каждого пациента на экран входа при перезапуске GoTrue.
>
> **Клиент аутентификации — один на процесс.** `cadenceAuthFor` вместо `cadenceAuth` на обоих
> корнях: каждый `SupabaseClient` поднимает своё автообновление на своей области, а активность
> пересоздаётся при смене масштаба шрифта, плотности и локали, которых нет в `configChanges`.
> Два клиента — два владельца одного токена обновления, и под ротацией проигравший тратит уже
> потраченный.
>
> **«Нечитаемое хранилище — это «нет сессии»» держится двумя звеньями, стык не измерен, и это
> сознательно.** `VaultSettingsTest.unreadableStorageIsNoSessionRatherThanACrash` держит первое
> звено (хранилище → пусто), `SessionStateTest.noSessionIsSignedOut` — второе
> (`NotAuthenticated` → `SignedOut`). Между ними вендор, и в артефакте 3.7.0 измерено, что из
> `Deciding` ведут **три** пути, и названия не называют ни одного. `AuthImpl.init` зовёт
> `loadFromStorage` до `setupPlatform`, и прочитанная из хранилища сессия уходит через
> `importSession` — который ставит `Authenticated` сразу лишь пока до истечения остаётся больше
> `expiresIn × (1 − SESSION_REFRESH_THRESHOLD)`. Константа равна **0.8** и означает долю
> прожитого, а не остаток: при токене GoTrue в 3600 с порог — последние 720 с.
> Под порогом — а под ним **любой** холодный старт позже 48 минут после выдачи токена —
> `importSession` вместо этого ждёт обновления, и статус ставит исход сетевого обмена. Когда
> читать **нечего**, статус не пишется вовсе, и этот путь ждёт `UtilsKt.initDone` —
> единственное, кроме `AuthImpl.clearSession`, место, где строится `NotAuthenticated`. На Apple
> `setupPlatform` зовёт `initDone` сам; на Android — только когда `enableLifecycleCallbacks`
> выключен, иначе из колбэка ON_START у `ProcessLifecycleOwner`, приезжающего из
> `androidx.lifecycle:lifecycle-process`, объявленного `supabase-kt-android` версией 2.10.0. У
> колбэка два сторожа: `alwaysAutoRefresh` — умолчание, и здесь не меняется, а
> `isAutoRefreshRunning` — не настройка вовсе, это `sessionJob?.isActive`, и она истинна всегда,
> когда сессия была прочитана: `importSession` зовёт `recreateSessionJob` по обе стороны порога.
> То есть этот колбэк кончает `Deciding` только на пути «сессии нет» — единственном, где
> `initDone` вообще есть что писать. Сквозной тест не написан намеренно: он мерил бы
> вендорскую обвязку под Robolectric, а не наше отображение. Цена записана здесь: пустой экран
> видит и пациент **с** сессией — на время сетевого обмена при обычном холодном старте, — а
> если `Deciding` не кончится, он останется пустым навсегда, и ни один тест этого не увидит.



### step-2: The `cadence://` scheme and the token exchange

Registering the intent-filter and `CFBundleURLTypes`, pointing the invite template
at `cadence://accept?token_hash={{ .TokenHash }}`, and the exchange itself: the app
calls `POST /verify` with the token it was handed and takes the session from the
body. The `codeVerifier` cache keeps its place and gains the note saying why it has
no consumer.

Tests: a link opens the app rather than a browser on both platforms; an unspent
token yields a session; a spent one is refused and the refusal is distinguishable
from a network failure. The exchange is measured against the live GoTrue in the
**api** harness — what runs in `kmp/` is a copy.
todoist: "6h9MFwpfXW69VQQH"

### step-3: The acceptance screen with a mandatory password

Acceptance from the session step 2 obtained, mandatory password entry, a Russian
explanation when the token was already spent. Tests: acceptance from a fresh link
only completes with a password; a repeat of the same link is explained rather than
failing with a generic refusal.
todoist: "6h9MFx2QHfxwMPmH"

### step-4: The sign-in screen and signing out

Address and password, a refusal that does not distinguish the cause, signing out
to the sign-in screen. Tests: a successful sign-in; the refusal is identical for
an unknown address and a wrong password; after signing out the protected area is
unreachable.
todoist: "6h9MFx9f7hGGQHxq"

### step-5: Password recovery

`POST /recover` without confirming that an address exists, an explanation of the
rate limit, returning via the link to set a new password. Test: a known and an
unknown address produce the same response.
todoist: "6h9MFxGpCR5xmVJq"

### step-6: Timezone and the Compose UI test suite

Calling `POST /v1/me/session` on sign-in and on launch with the device timezone.
Consolidating the block's paths into a suite that grows with every ported screen
from here on. The rule on tokens and Russian strings. The KMP gate green.
todoist: "6h9MFxMVRgXxWHXH"

## Open questions

> [!question] Обычный холодный старт показывает пустой экран на время сетевого обновления
> токена: измерено в артефакте 3.7.0 — сессия под порогом обновления (позже 48 минут после
> выдачи, то есть 80% срока нынешнего часового токена) кончает `Deciding` только исходом обмена
> с GoTrue. Сегодня это пустой `Box` — шаг 1
> не рисует ничего, чтобы не мигать экраном входа. Шагам 3–5, которые рисуют область до входа,
> нужно решение: оставить пустоту или дать заставку. Решать до того, как экраны нарисованы.

> [!question] Requiring a password lengthens invite acceptance by one screen. For
> a pilot with dozens of patients that is acceptable, but it is worth checking
> against the first real ones: if people drop off at this step, we will have to
> return to an optional password and solve session loss some other way.

> [!question] The iOS E2E tool remains an open question on the `kmp-app` note
> ("decided in M2"). This block provides the first scenario to choose it against,
> but the choice itself is out of scope and stays outstanding.

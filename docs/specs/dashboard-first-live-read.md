---
type: spec
project: cadence
status: approved
priority: p1
created: 2026-07-30
todoist_parent: "6h9MFpPJ3mvpjRcq"
components: [web-dashboard, api, identity, provisioner]
proposal: "[[20-Projects/cadence/architecture/proposals/first-live-read-and-sign-in|architecture/proposals/first-live-read-and-sign-in]]"
---
<!-- SNAPSHOT (read-only copy). Master: 20-Projects/cadence/specs/dashboard-first-live-read.md in vault prll-vault. Edit the vault note, then re-export — never edit here. -->

# First live read: the doctor signs into the dashboard and sees their patients

## Описание

The first block whose output you can see with your eyes: the doctor opens the
dashboard, signs in, sees the roster of assigned patients on real data along with
the state of their invites, and creates a new one through a form. The fixtures
from "Dashboard Skeleton" are replaced by a live read through policies.

The spec was rewritten after the first review. Two things were reversed: invite
state was derived from `joined_at` and was wrong in four states; and the
permissions the spec prescribed simply do not exist on those roles. Both
corrections are in the proposal.

## User Story

**As a** clinic doctor
**I want** to sign into the dashboard and see my patients and the state of their invites
**So that** I can tell who is already in the app and who has not opened the email for three days

## Acceptance Criteria

- [ ] `GET /v1/dashboard/overview` answers on the **final** §11 route and lives in the `identity` context: it does not extend the context registry in `internal/router`, and `TestEveryBoundedContextIsRegistered` stays green
- [ ] There is **no** doctor filter in the query: policies do the selection. Test — two doctors with different assignments get different sets from the same code
- [ ] Invite state comes from GoTrue via the `provisioner` batch lookup by a list of identifiers: accepted — `confirmed_at` or `last_sign_in_at` is non-empty; otherwise pending; expired — `invited_at` plus TTL in the past while both are empty
- [ ] An unavailable `provisioner` yields an "unknown" state on every row, not an empty roster and not a page error; the timeout is shorter than the request budget
- [ ] A patient token on this route gets `403` with its own problem type, not `200` with zero rows: an empty response is indistinguishable from a breakage
- [ ] Age and every aggregate are computed by the server; the rule "do not derive them in components" carries over from the fixture stage to live data through the same mechanism already in place — through the source, not through arithmetic
- [ ] The roster pages with **keyset** pagination over a stable ordering: `LIMIT/OFFSET` over a policy-filtered set skips and duplicates rows precisely when assignments change between pages — that is, in the scenario another criterion requires us to support
- [ ] `POST /v1/me/session` accepts a timezone, validates it against `pg_timezone_names`, and updates `profiles.timezone`. **For the patient only**, and the barrier is the statement rather than the absence of a grant: `cadence_admin` holds a table-wide `UPDATE` and `profiles_admin` is `USING (true)`, so the write carries its own `WHERE user_id = $2` and the handler answers doctors and admins `204` without a write rather than a refusal
- [ ] `cmd/seed` creates accounts through `provisioner`, **sets their passwords** with the dev operation, and writes the data — all in one command, because the identifiers exist only after the call. Seeds are not a migration: skipping one in production tears the sequential chain
- [ ] `cmd/seed` refuses to run against a production environment, and the refusal is covered by a test
- [ ] The TS client is generated from the committed `openapi.json`, the generated output is committed, and the gate fails on drift — the same technique as KMP's
- [ ] Doctor sign-in goes directly to GoTrue; `doctor` and `admin` are let inside, a patient gets a clear refusal. Signing out works, and the doctor's own identity replaces the hardcoded header
- [ ] A web route for invite acceptance exists: a doctor invited by `POST /v1/providers` lands on it and sets a password. Without the route, an approved endpoint invites people into nowhere
- [ ] Session storage in the browser is chosen and recorded as a decision; concurrent 401s produce exactly one token refresh — proven by a test
- [ ] The new-patient form carries everything the endpoint accepts: name, address, assignments **with a care-team role**, and demographics (`dob`, `sex`, `height_cm`, `target_weight_kg`); the `409` and `503` refusals are in Russian; double submission is impossible
- [ ] The other five Overview sections (the stats bar, triage, schedule, patient card, side menu) stay on fixtures **explicitly**, marked in code with a deadline — M6. No silently mixed state is left behind
- [ ] The Playwright smoke test is extended to the critical path and **cleans up after itself**: addresses of the form `e2e+<runid>@`, deletion through `provisioner` before a profile exists, or a separate teardown
- [ ] The edits to existing tests are made: `router_test.go:TestCommittedDocumentIsUpToDate` and `identity/handler_test.go:TestMeIsInTheContract` require `make openapi` in the same commit; `scripts/gate/ruleset.sh` reconciles job names against `ci.yml`, and the new TS-client gate job must appear in `main.json`
- [ ] `make gate`, `make test-integration`, and the web gate are green

## Scope / Non-scope

**In scope.** `GET /v1/dashboard/overview` v0 with state from `provisioner`.
`POST /v1/me/session` (timezone only, patient only). `cmd/seed`. TS client
generation and its gate. Doctor sign-in, sign-out, identity in the header, the web
route for invite acceptance. A live roster instead of fixtures. The new-patient
form in full. A smoke test covering the critical path, with cleanup.

**Out of scope.** Flags, adherence, sparklines, `lastSeen` — M6 extends the same
endpoint. The other five Overview sections stay on fixtures until M6 — explicitly.
The "full chart", the protocol editor, the admin panel, chat. The mobile screens —
the neighbouring block. The "Analytics", "Protocols", "Schedule", and "Patients"
pages — out of scope by the decision recorded on the `web-dashboard` note.

**Blocked and flagged.** Steps 4–5 of "Dashboard Skeleton" (deployment) are held
by SKL-01 and SKL-06, so the smoke test runs against the local harness until they
land. Actually delivering email to seeded accounts is held by SMTP from SKL-01 —
on dev, mail goes nowhere, and that is acceptable.

## Что уже реализовано (DONE)

**None of the following — they are `approved` prerequisites, not things in the
repository.** "Dashboard Skeleton" steps 1–3 (Vite, tokens, Overview on fixtures,
the transport and fixture layer), "Identity", "Trust Boundary" (`provisioner` with
five operations, including the batch lookup and setting a password outside
production), "Onboarding" (`POST /v1/patients`, `invites.invited_at`, the
production link TTL).

What does exist and work: `web/prototype/` as the visual specification of the
Overview; `api/openapi.json` with its drift gate; the registry of eleven contexts
in `internal/router` with the two tests that guard it.

## Технические детали

### `GET /v1/dashboard/overview`

Lives in the `identity` context — there is no separate `dashboard` among the
eleven, and creating one would break `TestEveryBoundedContextIsRegistered` and
violate invariant 2 of the `api` note about reading other contexts' tables.

The route is final per §11: M6 extends it with flags and adherence rather than
creating a second one. There is no doctor filter in the query — policies do the
selection: a condition in the query would become a second source of truth
alongside the policy.

Invite state is derived from the `provisioner` batch lookup response:
`confirmed_at` or `last_sign_in_at` non-empty — accepted; both empty and
`invited_at + TTL` in the past — expired; otherwise pending. Those two fields are
exactly what the lookup response carries, and it carries them for exactly this.

A `provisioner` failure does not bring the page down: rows arrive with state
"unknown". A roster without states is more useful than an empty screen.

Pagination is **keyset**: the policy does the selection, the set changes on
reassignment, and `LIMIT/OFFSET` over a changing set skips and duplicates rows.
The ordering is stable and the index is named.

### `POST /v1/me/session`

Accepts an IANA timezone, validates it against `pg_timezone_names`, writes
`profiles.timezone`. The column is writable **by the patient only** — and by the
administrator, which the first draft of this spec had wrong: `cadence_doctor`
holds no `UPDATE` on `profiles`, but `cadence_admin` holds a table-wide one and
`profiles_admin` is `USING (true)`, and `cadence_app` is a member of that role.
So the row is chosen by the statement's own predicate and not only by
`profiles_own_update`, and the handler refuses to write for anyone but a patient.
A call from a doctor or an administrator is `204` without a write rather than a
refusal: there is one client for every role and it should not branch.

This operation does **not** record the moment of invite acceptance — the source
for that is GoTrue. `joined_at` stays what it was: the moment of first app launch.

### Seeds

`cmd/seed`, not a migration: the identifiers exist only after the `provisioner`
call, and a migration skipped in production tears a strictly sequential chain. The
command creates accounts, sets their passwords with the dev operation, and writes
the data — Марина Волкова with full demographics, a care team of three
(endocrinologist, dietitian, nurse), the roster patients. The refusal against a
production environment is covered by a test.

The passwords are not decoration: without them the seeded doctor cannot sign in,
by hand or in the smoke test.

### Sign-in and acceptance on the web

Straight to GoTrue, the way the mobile app goes. `doctor` and `admin` are let
inside. The web route for invite acceptance is mandatory: `POST /v1/providers`
already invites doctors, and without it they land in nowhere. Signing out and the
doctor's identity in the header come from IDN-10, together with it.

Session storage in the browser is chosen and recorded: `httpOnly` is impossible,
GoTrue is a separate source. One token refresh for concurrent 401s — as already
done in KMP.

### The new-patient form

Designed from tokens — the prototype does not have it. It carries everything the
endpoint accepts, including the care-team role: without it the approved rule "the
doctor must assign themselves, and may add a second" has no client.

### The smoke test

Extended to "sign in → roster → create → appears". It cleans up after itself:
`e2e+<runid>@` addresses, teardown through `provisioner` or by another path.
Without cleanup the second run hits address uniqueness, and deletion is
conditioned on the absence of a profile — which a freshly created patient
precisely does have.

## Архитектурное решение

The block consumes the access model rather than changing it: policies select the
roster, state is taken from whoever owns it, and aggregates are computed by the
server.

The key correction to the first draft: invite state was derived from `joined_at`,
that is, from the moment of first app launch, and was wrong in four states — the
worst of them showed "expired" for a patient who had opened the link, and pushed
the doctor toward a re-invite that answers `422`. The source was moved to GoTrue,
and as a side effect that rescued the invariant "exactly six policy shapes": the
doctor no longer needs read access to `invites`.

The analysis and the remaining decisions are in the
[[20-Projects/cadence/architecture/proposals/first-live-read-and-sign-in|proposal]].

## Дельты компонентов

### web-dashboard.md
- MODIFIED: "Shape" — React **19** and TypeScript **6**, not 18 and 7: `react-router@8.3.0` has peer `react >=19.2.7`, and `typescript-eslint` holds peer `<6.1.0`
- MODIFIED: "Shape" — static assets are not on Railway: per ADR-008, deployment goes to Timeweb App Platform
- ADDED: to "Contracts" — the client is generated from the committed `openapi.json`, the generated output is committed, and the gate fails on drift
- ADDED: to "Contracts" — sign-in straight to GoTrue, signing out, a web route for staff invite acceptance; browser session storage accepted with the risk recorded
- ADDED: invariant — a patient who reaches the dashboard gets a clear refusal, not an empty screen: an empty one is indistinguishable from a breakage
- ADDED: invariant — invite state comes from GoTrue through `provisioner`; its unavailability yields state "unknown", not an empty roster
- MODIFIED: "Screens" — the Overview moves to a live read only for the roster; the other five sections stay on fixtures until M6, explicitly

### api.md
- ADDED: to "Contracts" — `GET /v1/dashboard/overview` on the final §11 route, in the `identity` context; `POST /v1/me/session` (timezone only)
- ADDED: invariant — patient selection in the overview is done by policies, not by a condition in the query
- ADDED: known limitation — the timezone is written for patients only, and that is held by the handler's role switch plus the statement's own `WHERE user_id = $2`, **not** by the absence of a grant: `cadence_admin` holds a table-wide `UPDATE` on `profiles` under a `USING (true)` policy
- ADDED: to "Contracts" — `database.IsUnavailable` is the one definition of «the database did not answer»; a route that means «repeat the request» answers 503 through it
- ADDED: to the error shape — a patient token on the overview route yields `403`, not `200` with zero rows

### identity.md
- MODIFIED: invite state is derived from GoTrue's `confirmed_at` and `last_sign_in_at`, not from our data; `joined_at` stays the moment of first app launch and is **not** the moment of acceptance
- ADDED: to contracts — `cmd/seed` creates people and sets their passwords; seeds are not a migration and refuse to run in production

### provisioner.md
- MODIFIED: contracts — the batch lookup by a list of identifiers is used by the dashboard roster; the set-password operation is used by `cmd/seed`

## Decomposition

### step-1: `POST /v1/me/session`

Timezone validated against `pg_timezone_names`, written for the patient only,
`204` without a write for doctors and admins. Tests: an unknown timezone yields
`400`; a doctor's call neither writes nor fails; a changed timezone gets through.
todoist: "6h9MFwp6MxhMp5qq"

> [!deviation] 2026-08-17
> Spec said: doctors and admins have no rights on the column, so the absence of a
> grant is the barrier. Measured: `cadence_admin` holds `GRANT SELECT, INSERT,
> UPDATE, DELETE ON app.profiles` (000004:219) under `profiles_admin FOR ALL
> USING (true)` (000004:62), and `cadence_app` is a member of `cadence_admin`
> (000001:218). An `UPDATE` bounded only by `profiles_own_update` would therefore
> have rewritten every profile in the clinic for a caller holding an admin token,
> and answered 204. Actually done: the statement carries `WHERE user_id = $2` and
> asserts it matched exactly one row, and the handler answers staff without
> reaching the database. Why: the policy narrows the statement for
> `cadence_patient` only, and `RecordTimezone` is exported and takes whatever
> caller it is given.

> [!deviation] 2026-08-17
> Spec said: nothing about a token carrying no role, or about the database being
> unreachable. Actually done: an account the invitation reached and provisioning
> did not is answered `403` rather than `204`, because 204 would claim a write
> that did not happen; and a failure that means «not now» is answered `503`
> through the new `database.IsUnavailable` rather than `/problems/internal`.
> `400`, `401`, `403` and `503` are declared on the operation, so both generated
> client surfaces can branch on them — `401` among them although the middleware
> and not the handler usually raises it, because declaring any status at all
> makes huma drop the default response. Why: both were review findings, and the second was
> measured — a Postgres that is down returns `*pgconn.ConnectError` with no
> SQLSTATE, which the first attempt at the classifier did not read.

### step-2: `GET /v1/dashboard/overview` v0

The final route in the `identity` context, no doctor filter, keyset pagination,
age from the server, `403` on a patient token. Tests: two doctors get different
sets from the same code; an admin sees everyone; reassignment changes the output;
paging neither skips nor duplicates when assignments change between pages.
todoist: "6h9MFwrWRf4v4JVq"

> [!deviation] 2026-08-18
> Spec said: nothing about a token carrying no product role, and nothing about a
> page size. Actually done: an account the invitation reached and provisioning
> did not is answered `403` — the same refusal the session route gives it —
> rather than falling into the seam and coming back a `500`; and a page of no
> rows is refused rather than indexed, because the method is exported past the
> schema that pins the route's minimum. Why: both are states the code can reach
> and the step did not name.

> [!deviation] 2026-08-18
> Spec said: a patient token gets `403` **with its own problem type**. Actually
> done: `403` with `/problems/forbidden`, which `httpserver.problemTypeFor`
> derives from the status alone and every other `403` in the API therefore
> shares; what distinguishes this refusal is its Russian sentence. Why: a type
> per refusal is a change to the platform's closed set of problem types, which
> is `api.md`'s to decide rather than this step's. Recorded here rather than
> read as satisfied.

### step-3: Invite state from `provisioner`

Batch lookup by the page's identifiers, derivation of the three states, "unknown"
on failure with a timeout shorter than the request budget. Tests: all three states
match the actual one; a `provisioner` failure yields rows with "unknown" rather
than an empty roster; a patient who opened the link but never launched the app is
shown as **accepted**, not expired — that is the case the source was moved for.
todoist: "6h9MFx6MpCF9Mqcq"

> [!deviation] 2026-08-19
> Spec said: the expired state is `invited_at + TTL` in the past, and the DONE
> section names `invites.invited_at` among the prerequisites. Actually done:
> `invited_at` comes from the identity provider, through the same batch lookup
> as the other two instants, and the roster reads no `app.invites` at all.
> Measured on the cycle database: a doctor's SELECT touching that table fails
> with `permission denied for table invites` (42501) — 000008 grants them INSERT
> only, an administrator SELECT — so deriving the state from our own row would
> have failed for the one caller this screen exists for. Measured on the pinned
> GoTrue image: `/admin/users/{id}` states a top-level `invited_at`, and the
> component's answer now carries it. Why: the source with no grant problem is
> the one the acceptance criterion already named — "invite state comes from
> GoTrue via the `provisioner` batch lookup".

> [!deviation] 2026-08-19
> Spec said: nothing about how large a page may be. Actually done: the roster
> refuses a page larger than `identity.MaxPageSize` (100), which is the number
> the route's schema already pinned and the number the component enforces on
> `LookupBatch`; the client refuses a longer list before the round trip, and two
> tests compare the three spellings. Why: this closed the open question the
> Todoist task carried into this step — the component's 100 against a client
> that bounded nothing. A page past the bound is not an error anyone would see:
> the component answers 400 and every row on the screen comes back "unknown".

> [!deviation] 2026-08-19
> Spec said: nothing about a top-level versus a nested instant. Measured on the
> pinned image and written into the component: `identities[0].last_sign_in_at`
> is set at the moment of the invitation, so a reader of that field would draw
> every invited patient as accepted. Only the top-level one is decoded.

> [!question] Measured while step 3 was being built, and it is step 4's to
> answer: setting a password through the dev operation confirms nothing — the
> account keeps no `confirmed_at` and no top-level `last_sign_in_at`. Seeded
> patients will therefore be drawn **pending**, and **expired** three days after
> the seed was run. Either the seed confirms the accounts it creates, or the
> dashboard's seeded roster reads as a clinic nobody has joined.

### step-4: `cmd/seed`

Accounts through `provisioner`, passwords through the dev operation, data — in one
command. Марина Волкова, a care team of three, the roster patients. The refusal in
production is covered by a test.
todoist: "6h9MFx8V9c6w2prq"

### step-5: TS client generation and the drift gate

The client from the committed `openapi.json`, the generated output committed, the
gate failing on divergence, the job created **and** added to
`.github/rulesets/main.json` — otherwise `scripts/gate/ruleset.sh` goes red.
Wiring into React Query.
todoist: "6h9MFxGX42pm22HH"

### step-6: Sign-in, sign-out, identity, invite acceptance on the web

Straight to GoTrue, routes for `doctor` and `admin` only, a clear refusal for a
patient, signing out, the doctor's identity instead of the hardcoded header, the
invite-acceptance route with password entry. Session storage chosen and recorded.
Tests: concurrent 401s produce one refresh; an invited doctor completes acceptance
and signs in.
todoist: "6h9MFxMqfmc82R8q"

### step-7: A live roster instead of fixtures

The roster moves to the endpoint. Loading, empty, and error states are preserved.
The other five sections stay on fixtures **with a marker and a deadline** — no
silently mixed state is left behind. The rule "aggregates are not derived in
components" carries over to live data through the same mechanism.
todoist: "6h9MFxVCgFXXJFGq"

### step-8: The new-patient form

From tokens. Name, address, assignments with a care-team role, demographics. `409`
and `503` in Russian, protection against double submission. Tests: the doctor
assigns themselves and adds a second specialist; a repeated address shows a clear
message.
todoist: "6h9MFxVvCx68CxWH"

### step-9: A smoke test to the critical path, with cleanup

Playwright: sign in → roster → create → appears. `e2e+<runid>@` addresses, cleanup
after itself. While the "Dashboard Skeleton" deployment steps are blocked by
SKL-01/06 the smoke test runs against the local harness — and that is written
down, not implied.
todoist: "6h9MFxg2wC44vR7q"

## Открытые вопросы

> [!question] Doctor session storage: in memory with a silent refresh, or
> `localStorage`. The first loses the session when the tab reloads, the second is
> readable by any script. The dashboard is an internal tool with no user-generated
> content, but the decision gets made at step 6 and recorded.

> [!question] Smoke-test cleanup: deletion through `provisioner` is conditioned on
> the absence of a profile, and a patient created by the smoke test has one. So
> either cleanup goes by another path (direct deletion in the test database), or
> the deletion condition gets an exception for the test environment. Decided at
> step 9; no effect on production.

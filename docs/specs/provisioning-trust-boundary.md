---
type: spec
project: cadence
status: approved
priority: p1
created: 2026-07-30
todoist_parent: "6h9JPwH5pfVwVmrq"
components: [provisioner, api, data-layer, identity, audit]
proposal: "[[20-Projects/cadence/architecture/proposals/invites-and-onboarding|architecture/proposals/invites-and-onboarding]]"
---
<!-- SNAPSHOT (read-only copy). Master: 20-Projects/cadence/specs/provisioning-trust-boundary.md in vault prll-vault. Edit the vault note, then re-export — never edit here. -->

# The provisioning trust boundary: the admin key leaves the API

## Описание

The first of the two onboarding blocks. It creates no patients at all — it builds
the boundary inside which creating them is safe, and closes three escalation paths
found by measuring against live GoTrue.

A `provisioner` component appears — the sole holder of the GoTrue admin key, with
an exhaustive list of operations and no password setting in production. Writing
`role='admin'` from the request path is forbidden by policy. The absence of access
to the `auth` schema stops being an accident of configuration and becomes an
invariant with a walk that verifies it. The audit actor starts being derived from
the principal instead of being taken as a parameter.

The second block is onboarding itself: the harness with GoTrue, the endpoints, the
templates, the cycle tests.

## User Story

**As a** clinic doctor
**I want** a compromise of the API not to let anyone impersonate me
**So that** a doctor's signature on a clinical change means me specifically, and the
audit remains a basis for a conversation about trust in the data

## Acceptance Criteria

- [ ] The private GoTrue admin key is read only by the `provisioner` process; a gate rule grepping `api/cmd/api` and `api/internal/**` forbids any mention of its variable — a configuration test cannot prove this, and we do not rely on one
- [ ] `provisioner` exposes **exhaustively five** operations — invite, look up by address, look up by a list of identifiers, delete an account, set a password **outside production**; a walk over the **real** mux proves nothing else is mounted, including debug and metrics paths
- [ ] User lookup is an exact match inside the component, one result or none; the response is narrowed to `{id, confirmed_at, last_sign_in_at}`. A substring and an empty string are refused, never a list: GoTrue's `?filter=` is a substring match and is itself **case-sensitive**, so the address is lowercased before the call
- [ ] Deletion is hard-conditioned, and the condition is checked by the **caller**, who passes the proof: only an account that has an invite record and no row in `profiles` may be deleted. Without deletion, an address whose owner managed to open the link and set a password can be neither reused nor cleared — a dead end with no way out
- [ ] There is no enumeration among the operations. Setting a password exists **only outside production** — the seeds need it so the doctor has something to sign in with — and refuses to exist against a production environment; verified by a test, not by configuration
- [ ] The batch lookup by a list of identifiers returns the same three fields for each: otherwise the dashboard roster would produce an N+1 through a component never designed for it
- [ ] What is protected is the **list**, not its length: a walk over the real mux proves that exactly what was declared is mounted. The surface has grown three times, and that is recorded rather than papered over
- [ ] The secret is compared in constant time; two values are accepted as current; a request with no secret and one with a wrong secret are rejected; the secret does not reach logs, URLs, or the query string
- [ ] A probe against the deployed harness's public name: no route resolves to `provisioner`. The claim "only the first service is proxied" is a routing property, and prose is not accepted for it
- [ ] The admin token carries `exp ≤ 60s` and `aud`/`iss` values that the API verifier rejects; the refusal is checked for each reason **separately**, so a regression in one does not go unnoticed
- [ ] `GOTRUE_JWT_KEYS` contains two keys with **distinct** `kid`s, exactly one of which signs; the list of permitted session `kid`s is mandatory, an empty one fails startup, and it **does not intersect** the admin `kid`
- [ ] A contract test pins five measured GoTrue behaviours: the admin token is accepted against any configured key; `aud` and `iss` are not checked; `exp` is optional; `no signing key found` when no signing marker is present; `multiple signing keys detected` when two are
- [ ] The GoTrue image is pinned by digest: the model rests on undocumented behaviours, and an upgrade must run into the contract test
- [ ] The `kid` check stands **before** key resolution, so a flood of unknown `kid`s never reaches the JWKS refresh budget; the refusal names the `kid` and the reason
- [ ] The rotation order is written down and walked through on the harness: `kid` into the list → deploy the API → hand over the signing marker → hold the old one for at least the refresh-token lifetime → remove. The reverse order refuses all authenticated traffic
- [ ] Neither `cadence_app`, nor `cadence_service_app`, nor any role they can transition into has `USAGE` on the `auth` schema or a single grant on its relations; the schema owner is `cadence_gotrue`. The walk checks for **absence** and fails when a grant appears
- [ ] The service-path policy on `profiles` carries `WITH CHECK (role IN ('patient','doctor'))`: an attempt to write `role='admin'` from the request path is rejected by the database, not by code
- [ ] `WithService` derives the actor from `auth.PrincipalFrom(ctx)` and **refuses** without a principal; paths with no human use a separate constructor. A handler is incapable of naming somebody else's `uuid` — the distinction is made at compile time, not by an argument's value
- [ ] The HTTP call timeout is smaller than `idle_in_transaction_session_timeout`, and both are smaller than the request context deadline, which serves as the connection acquisition budget; `pgxpool` has no separate budget, and three independent knobs do not exist
- [ ] The time limits are set with `ALTER ROLE` on the **LOGIN** role `cadence_service_app` and duplicated in the service pool's connection string: `SET ROLE` does not pick up the target role's settings, and live connections hold the old values until they are recreated
- [ ] Recorded as accepted: both limits are `USERSET`, that is, a default against forgetfulness rather than a barrier against hostile code
- [ ] The service pool gets an explicit `MaxConns` and its own constructor; the request gains a context deadline, which today exists nowhere
- [ ] `make gate` and `make test-integration` are green, including the edits to existing tests listed in the steps

## Scope / Non-scope

**In scope.** The `provisioner` component and its client. Two keys with
non-intersecting `kid`s, the pinned list, contract tests for GoTrue behaviours,
pinning the image by digest, the rotation order. The walk asserting no access to
the `auth` schema. The ban on writing `role='admin'` from the request path. The
actor from the principal. Time bounds and the service pool constructor.

**Out of scope.** Everything that creates a patient: `POST /v1/patients`,
`POST /v1/providers`, GoTrue in the test harness, the email templates,
`cmd/bootstrap-admin`, the "invite → acceptance" cycle tests — the second block.
The `invites` table is **not implemented at all** (the decision is in the
proposal). Mailing rate limits and link lifetime — the second block, together with
the decision about `POST /recover`. mTLS between the API and `provisioner` — a
deliberate omission for the pilot. A full rewrite of `architecture/overview.md`,
made stale by ADR-008.

## Что уже реализовано (DONE)

From the identity block: seven roles, two seams, `app.jwt_subject()`, six tables,
six policy shapes, three registries over the `app` schema, the separation of `auth`
and `auth/token`.

From the local environment: GoTrue v2.194.0 on ES256, JWKS at the derived address,
registration disabled.

**Measured against live GoTrue on 2026-07-30** — facts, each reproduced by a request:

- HS256 is rejected regardless of `GOTRUE_JWT_SECRET`. ES256 with a configured key
  carrying `role: service_role` or `role: supabase_admin` — 200 on the admin routes.
- A second key without `key_ops: ["sign"]` is usable on the admin routes; sessions
  are signed by the first. Order does not matter. Both without the marker →
  `no signing key found`. Both with it → `multiple signing keys detected`. An
  identical `kid` fails startup in one order and produces a silent breakage in the
  other.
- Admin routes check **neither** `aud`, nor `iss`, nor the presence of `sub`, nor
  the presence of `exp`. An expired `exp` is the only cause of refusal.
- `auth.users.confirmation_token` = `auth.one_time_tokens.token_hash` = the `token`
  parameter in the link: **the value in the database is itself the credential**.
- `POST /recover` is public and unauthenticated, and `verify?type=recovery`
  additionally **confirms** the account.
- `?filter=` is a substring match, and an empty string returns everyone.
- The identity-capture chain: admin key → `PUT /admin/users/{id}` with a password →
  password grant → a token signed by the **session** key. Pinning the `kid` does not
  stop it.
- Both time limits are `USERSET`; `SET ROLE` does not pick up the target role's
  settings; `pgxpool` has no connection acquisition timeout.
- The `auth` schema is today owned by the container's superuser — by accident of
  configuration, not by rule.

## Технические детали

### The `provisioner` component

`cmd/provisioner`: the sole holder of the private admin key. Three routes. It is
not proxied outward; the App Platform constraints (ports 80 and 443 taken, the
first service in the manifest is proxied, `volumes` forbidden) coincide with the
requirement here, but are not treated as the boundary — that is verified by a probe.

Lookup is an exact match **inside** the component: GoTrue's substring `filter` is
not exposed outward, because otherwise the API holds the clinic's directory. The
address is lowercased before the call: `filter` is case-sensitive, and `/invite`
locks the address, so without alignment mixed case gives an empty lookup against an
existing account. The response is narrowed to three fields, and `confirmed_at` and
`last_sign_in_at` are in it not for convenience — the claim rule in the second
block rests on them.

Deletion was added by the fourth review round and exists for one scenario: the
invite went out, the transaction rolled back, and the person managed to open the
link and set a password. Then `/invite` answers `422`, we have no email change, and
there is no access to `auth` — the address is burned forever. The condition is hard
and the proof is passed by the caller: only an account that has an invite record
and no row in `profiles` may be deleted. The component cannot see the `app` schema,
so it cannot check the condition itself — and that is written down rather than
papered over.

The token: `role: service_role`, `exp ≤ 60s`, and `aud` and `iss` set to values our
verifier rejects. GoTrue does not check them, which makes this a free second cause
of refusal, so a regression that loses the `kid` pinning stays closed.

The secret: constant-time comparison, two current values from day one — a shape of
configuration, not a rotation choreography. Against a compromised API the secret is
useless by construction, and that is written down rather than papered over.

### Keys and the verifier

Two JWKs with **distinct** `kid`s. "Exactly one signer" is provided by GoTrue
itself — both degenerate cases fail startup — and that is pinned by a contract test
together with the three undocumented behaviours the model rests on. The image is
pinned by digest: otherwise an upgrade silently carries the foundation away.

The list of session `kid`s is mandatory configuration, an empty one fails startup,
and its non-intersection with the admin `kid` is verified: otherwise a compromised
`provisioner` issues an admin session in one step, and that is the only barrier
existing in that direction.

The check sits in `keyfunc` before key resolution — that way a flood of unknown
`kid`s never reaches the JWKS refresh budget, so pinning actually relieves the
budget. The refusal message names the `kid` and the reason, otherwise a rotation
incident is indistinguishable from a stream of bad tokens.

### Zero access to the `auth` schema

A public `/recover` plus a readable token value means: any access to `auth` equals
capturing the identity of any doctor, and no admin key is needed for it. So the
absence of access is a load-bearing control, and it is verified like the identity
block's registries, only inverted: the walk requires the **absence** of `USAGE` and
of grants on every relation in the schema for `cadence_app`, `cadence_service_app`,
and every role they can transition into. The owner of `auth` is set to
`cadence_gotrue`.

### The ban on `role='admin'` from the request path

The service-path policy on `profiles` gains
`WITH CHECK (role IN ('patient','doctor'))`. This closes the chain found by the
review: a compromised API can invite an arbitrary address, but cannot make the
invitee an admin. Admins come into being only from the second block's one-off
command, under a role the API process does not possess.

### The audit actor

`WithService` derives the actor from `auth.PrincipalFrom(ctx)` and refuses without a
principal. For paths with no human there is a separate constructor. The distinction
is at compile time: actor forgery survived two drafts of the proposal precisely
because it was an argument's value — and it is the motive of the whole story.

### Time bounds

The ordering has two knobs, not three: the HTTP timeout is smaller than
`idle_in_transaction_session_timeout`, and both are smaller than the request context
deadline, which serves as the connection acquisition budget, because `pgxpool` has
no budget of its own. The limits are set with `ALTER ROLE` on the LOGIN role
`cadence_service_app` — on `cadence_service` they would silently do nothing, since
`SET ROLE` does not pick them up — and duplicated in the connection string, because
live connections hold the old values until recreated. Both GUCs are `USERSET`: a
default against forgetfulness, not a barrier, and that is written down.

The service pool gets its own constructor with an explicit `MaxConns` — today
`postgres.go` sets neither it, nor a connection timeout, nor an execution mode.

## Архитектурное решение

Whoever holds the GoTrue admin key can reset any user's password and obtain a
session token by legitimate means — verified with a chain of three calls. The
consequence is not "access to data" but **capture of a named doctor's identity**,
that is, the nullification of audit attribution. So the key moves out into a
component with an exhaustive two operations.

Two previous drafts were not enough, and both mistakes are worth naming: pinning
the `kid` is not a barrier, and inviting an arbitrary address is harmless **only**
after writing `role='admin'` from the request path is forbidden. And the
load-bearing control all along was one nobody had declared: the absence of access
to the `auth` schema.

Rejected: the key in the API with an in-process narrowing adapter — a compromised
process ignores the restriction. The full account of the three measurement rounds
is in the
[[20-Projects/cadence/architecture/proposals/invites-and-onboarding|proposal]].

## Дельты компонентов

### overview.md
- ADDED: a fifth deployed component, `provisioner` — the sole holder of the GoTrue admin key. It is absent from the partner's document: there that role was played by the key issued by Supabase
- ADDED: system-level invariant — the private admin key exists in no process other than `provisioner`
- ADDED: system-level invariant — no role the API process can transition into has access to the `auth` schema: a public `/recover` and a readable token value make any such access equal to identity capture

### provisioner.md (new note)
- ADDED: purpose — the sole holder of the GoTrue admin key
- ADDED: contracts — invite, look up by exact match, look up by a list of identifiers, delete with proof, set a password outside production, **exhaustively five**, verified by a walk over the real mux including debug paths
- ADDED: invariant — the lookup response is narrowed to `{id, confirmed_at, last_sign_in_at}`; GoTrue's substring `filter` is not exposed outward, and the address is lowercased before the call, because `filter` is case-sensitive
- ADDED: invariant — deletion is conditioned on an invite record and the absence of a profile; the proof is passed by the caller, because the component cannot see the `app` schema
- ADDED: invariant — the issued token carries `exp ≤ 60s` and `aud`/`iss` values rejected by the API verifier
- ADDED: invariant — it is not proxied outward, and that is verified by a probe against the public name rather than by a platform property; the secret is compared in constant time, and two values are current
- ADDED: known limitation — against a compromised API the secret is useless by construction

### api.md
- ADDED: to "Dependencies" — `provisioner`; the API does not call GoTrue's admin contract and does not hold its key
- ADDED: invariant — the list of permitted session `kid`s is mandatory, an empty one fails startup, and it does not intersect the admin `kid`. This is independent revocability and defence in depth, not a barrier
- ADDED: invariant — the external call timeout is smaller than `idle_in_transaction_session_timeout`, and both are smaller than the request context deadline; the limits are duplicated in the connection string, because `ALTER ROLE` does not reach live connections, and both GUCs are `USERSET`
- ADDED: to the wiring — the service pool has its own constructor with an explicit `MaxConns`

### data-layer.md
- MODIFIED: the Auth contract — two keys with distinct `kid`s, exactly one signing; both degenerate cases fail GoTrue's startup, and that is pinned by a contract test
- ADDED: to the Auth contract — the step-by-step order of session key rotation
- ADDED: invariant — the GoTrue image is pinned by digest, because the model rests on three of its undocumented behaviours
- ADDED: invariant — the owner of the `auth` schema is `cadence_gotrue`; the request-path roles have neither `USAGE` in it nor grants on its relations, and that is verified by a walk asserting absence
- ADDED: to the Auth contract — `auth.users.confirmation_token` equals the value in the link, that is, the value in the database is the credential; hence the previous invariant

### identity.md
- MODIFIED: `invites` is **not implemented**: after the derived fields were moved out, the §03 entity carries no facts of its own
- ADDED: invariant — writing `role='admin'` from the request path is forbidden by the service-path policy; admins come into being only through a one-off command under a role the API process does not possess

### audit.md
- MODIFIED: invariant 3 — the actor is **derived from the principal** rather than taken as a parameter; for paths with no human there is a separate constructor, and the distinction is made at compile time

## Decomposition

### step-1: Two keys, non-intersecting `kid`s, the pinned list

A second key with a different `kid`; the list of permitted session `kid`s as
mandatory API configuration, an empty one failing startup, with non-intersection
against the admin one verified. The check in `keyfunc` before key resolution, with a
message naming the `kid` and the reason. The GoTrue image pinned by digest. The
rotation order written down and walked through on the harness.

Edits to existing tests, listed explicitly: `verifier_test.go` — the `newVerifier`
helper gains the list, so until it is amended nothing in `verifier_test.go` or
`middleware_test.go` compiles; `TestVerifyPicksUpRotatedKeys` and
`TestVerifyPicksUpRotatedKeysOverANetwork` assert that a **new** `kid` is picked up
without a restart — pinning deliberately cancels that, and the tests are rewritten
to rotation within a permitted `kid`; `TestVerifyRejects/"unknown key id"` stays
green for a different reason and moves onto the new refusal;
`TestVerifyAcceptsBothPermittedAlgorithms` — the fixture's `kid` goes into the list.
`config_test.go` — `clearEnv` and `setRequired` gain the new variable, and
`TestLoadRejectsBadInput` gains a case for it.
todoist: "6h9JQ33h8JVfHrGq"

> [!deviation] 2026-08-13
> Spec said: one new configuration variable — the permitted session `kid` list.
> Actually done: **two**, `AUTH_JWT_SESSION_KIDS` and `AUTH_JWT_ADMIN_KID`, with
> three rejection cases in `TestLoadRejectsBadInput` rather than one. Why: the
> non-intersection check has nothing to compare against without a concrete admin
> `kid`, and the API cannot derive one. It never sees `GOTRUE_JWT_KEYS`, and the
> JWKS cannot supply it — measured on the pinned image, GoTrue rewrites
> `key_ops` to `["verify"]` on **both** published keys, so the signing marker is
> invisible from outside. A value fetched over the network could not gate
> startup in any case: `config.Load` does no I/O. That same probe is what shows
> the check is load-bearing rather than decorative — both `kid`s are genuinely
> published, so without it an admin-signed token would resolve against a real
> public key.

> [!deviation] 2026-08-13
> Spec said: the edits to existing tests are a closed list of five. Actually
> done: two more — `TestVerifyRateLimitsUnknownKeyIDRefresh` and
> `TestRateLimitedUnknownKeyIDIsStillTheCallersProblem` had their junk `kid`s
> added to the permitted list. Why: both exist to exercise the key resolver's
> rate limiting, and the new check sits **before** resolution, so without
> pre-permitting they would have gone green without ever reaching the path they
> are named for — passing for the wrong reason. Confirmed by mutation that each
> still reaches it: forcing the limiter to `rate.Inf` reddens the first, and
> forcing `classifyKeyFailure` down the outage branch reddens the second.

### step-2: Contract tests for GoTrue behaviours

The five measured behaviours are pinned by tests against real GoTrue: the admin
token is accepted against any configured key; `aud` is not checked; `iss` is not
checked; `no signing key found` when no signing marker is present; `multiple
signing keys detected` when two are. The first three are what the model rests on;
the last two are what makes "exactly one signer" a property rather than an
operator's promise.

These tests are the mechanism by which an image upgrade runs into red rather than
into production.
todoist: "6h9JQ36G2R3R6rxq"

### step-3: Zero access to the `auth` schema

The schema owner is set to `cadence_gotrue`. The walk requires the **absence** of
`USAGE` and of any grant on every relation in the `auth` schema for `cadence_app`,
`cadence_service_app`, and every role they can transition into. The walk checks for
absence, so it fails when a grant **appears** — that is, it works as a guard rather
than as a snapshot.

A comment on the spot explains why this is a load-bearing control: `POST /recover`
is public, and `auth.users.confirmation_token` is the credential.
todoist: "6h9JQ3Gp3Vg2xwjH"

> [!deviation] 2026-08-14
> The step describes only the walk. Two things outside its text were needed to
> write it, both in the test harness and neither touching the access model:
> - testsupport.StartGoTrueOn (and the prepareForGoTrue split behind it),
>   because the chain's roles and GoTrue's relations must be in one catalogue
>   for "no access to auth" to be a question Postgres can answer. GoTrue in the
>   harness already existed for step-2.
> - TestAlienRolesHoldNothingInTheApplicationSchema now plants service_role only
>   when it is missing and drops it only when it planted it. Roles are cluster
>   objects and the GoTrue harness creates the same Supabase names, so the bare
>   CREATE ROLE failed with 42710 once an identity provider ran first in the
>   same test binary, and an unconditional DROP would take the role out from
>   under the next one. The assertion is unchanged.

### step-4: The ban on `role='admin'` from the request path, and the actor from the principal

The service-path policy on `profiles` gains
`WITH CHECK (role IN ('patient','doctor'))`; the policy registry is updated.
`WithService` derives the actor from the principal and refuses without one; for
paths with no human a separate constructor appears, and the distinction is a
compile-time one.

Tests: an attempt to write `role='admin'` through the service path is rejected by
the database; a handler cannot name somebody else's `uuid`; a path with no human
writes `actor_job`; an audit row on a doctor's behalf names the doctor.
todoist: "6h9JQ3VvvxJh45jH"

> [!deviation] 2026-08-14
> The step names no edits to existing tests, unlike step-1. Several were needed.
> Most are mechanical consequences of the new signature — call sites moving to
> WithServiceJob, the audit assertion moving from actor_job to actor_id — and
> are not recorded here. Three are decisions:
> - TestNoPolicyBodyNamesAProductRole -> TestNoPolicyBodyDecidesTheCallersRole.
>   000004's header declares "no policy body here contains the literals
>   'patient', 'doctor' or 'admin' — a test asserts that", and this step's
>   WITH CHECK contains two of them. The rule that sentence stood for is that the
>   *caller's* role is never compared against a value; the new check constrains
>   the row's value instead. The test now says both halves: the two policies
>   permitted to name a role at all are declared, and no policy may name one in a
>   body that also reads a claim. Both halves were mutation-tested. The
>   correction to 000004's header is written into 000006, since 000004 is
>   immutable.
> - TestUnwindingTheChainOneStepAtATimeReachesTheBase asserted that a down
>   migration strictly reduces the object count. 000006 is the first migration
>   whose whole content is a rewritten policy, so its down file changes the
>   schema without changing the count. The witness is now a description of the
>   schema including every policy's predicate; the count is still asserted in the
>   direction that still means something, that a rollback does not add.
> - Two fixtures that seeded an admin profile through the service path (stand()
>   in identity, seedProfiles() in the database package) now write that row with
>   the superuser. Their assertions are unchanged.

### step-5: The `provisioner` component

`cmd/provisioner`: the holder of the admin key, five operations, a token with
`exp ≤ 60s` and foreign `aud`/`iss`, exact-match lookup with the address lowercased,
a narrowed response, deletion with proof from the caller, a secret with constant-time
comparison and two current values. The manifest and the variables, accounting for
the App Platform constraints.

Tests: a walk over the real mux proves exactly five routes, including the absence of
debug and metrics ones; a substring and an empty string in a lookup are refused,
never a list; mixed case finds an existing account; deletion without proof is
rejected; a request with no secret and one with a wrong secret are rejected; the
issued token is rejected by the API verifier on `kid`, on `aud`, and on `iss` — each
reason separately — and accepted by GoTrue. A probe against the deployed harness's
public name: no route resolves to `provisioner`.
todoist: "6h9JQ3gPxrhPFMmH"

> [!deviation] 2026-08-14
> The step asks for "the manifest and the variables, accounting for the App
> Platform constraints". No manifest was written.
> docs/specs/deploy-observability.md carries the criterion "today there is
> neither a Dockerfile nor an App Platform manifest, and no other spec creates
> them" and claims both artefacts; writing one here would break that spec's
> acceptance criterion and add a platform artefact this story has no way to
> verify. What is written instead: the placement provisioner requires — never
> the first service, a port of its own rather than 80 or 443, no volumes — in
> cmd/provisioner/main.go's package doc, and the variables in load(), the
> loader that fails startup without them, per the project's no-.env.example
> rule.
> The probe against the deployed harness's public name is written
> (scripts/probe/provisioner-is-not-proxied.sh) and has not been run: there is
> no deployment, and standing one up is SKL-06, a manual blocker. That
> acceptance criterion is open (⏸ Requires SKL-06), not satisfied, and is the
> one claim about this component that is stated rather than measured.

### step-6: The `provisioner` client and the time bounds

The interface is declared by the consumer in `internal/identity`, the implementation
lives in the platform package and is not imported into the context. The timeout
covers connection, TLS, and response. `ALTER ROLE cadence_service_app` in a
migration sets `statement_timeout` and `idle_in_transaction_session_timeout`, and
the same values are duplicated in the service pool's connection string. The service
pool gets its own constructor with an explicit `MaxConns`, and the request gets a
context deadline.

Tests: the numbers are ordered by an assertion, not by a comment; the limits sit on
`cadence_service_app` rather than on `cadence_service`; the token does not reach
logs, URLs, or the query string, and request dumps are forbidden by a gate rule; a
grep rule forbids mentioning the admin key's variable in `api/cmd/api` and
`api/internal/**`.
todoist: "6h9JQ3pfvMFFQ8Xq"

> [!deviation] 2026-08-14
> Three things outside the step's own text:
> - `describeSchema` in `TestUnwindingTheChainOneStepAtATimeReachesTheBase`
>   now also describes `pg_db_role_setting` for the chain's roles. 000007 is
>   the first migration that touches no object in the app schema, so without
>   this its down file could be emptied and the rollback witness would stay
>   green. Measured both ways.
> - `httpserver.RequestDeadline` and the middleware carrying it are new. The
>   step names "the request gets a context deadline" and the acceptance
>   criteria call it the connection-acquisition budget; nothing set one
>   before, and the incoming request is where it belongs.
> - `provisioning.LookupBatch` answers an empty list without calling the
>   component, which refuses one with a 400. A clinic with no patients yet
>   is an ordinary state rather than a caller's mistake.

## Открытые вопросы

> [!decision] 2026-08-14 — **all six steps are implemented and merged** (PR #8,
> `scripts/gate/all.sh` green including the integration suite). The spec stays
> `approved` rather than going to `done`, because one acceptance criterion is not
> satisfied and no code can satisfy it: the probe against the deployed harness's
> public name has never been run, there being no deployment. It reopens with SKL-06
> — todoist: `6hGwxFFhF9Grp43q`.
> The component notes were brought to the implemented state on the same day; the
> proposal is **not** condensed into an ADR yet, because it covers both blocks and
> `patient-onboarding` is unbuilt.

> [!question] `LookupBatch` does not bound its list. The component contract says
> 100 identifiers; the timeout ladder was sized against a different figure. The two
> do not agree, and which one moves is a design decision rather than a patch.
> todoist: `6hGwxFMW5WJQM6Fq`.

> [!question] The choreography of shared-secret rotation: two current values
> overlapping — the shape is accepted, but the replacement order (who goes first,
> how long both are held) is not chosen. Decided at step 5; affects only a RUNBOOK
> that does not yet exist.

> [!decision] 2026-08-14. `provisioner` lives at **`cmd/provisioner` inside the
> `api` module** — as step-5's own text already said, and against the lean recorded
> below it, which is kept for the record. The two places contradicted each other and
> would have left the module boundary to be settled by whoever wrote the code first.
>
> What is given up: the compiler stops being the thing that proves the component
> cannot reach the database. What holds the boundary instead is step-6's grep rule
> (`api/cmd/api` and `api/internal/**` may not mention the admin key's variable) plus
> the absence of any `pgx` import under `cmd/provisioner` — both checked by the gate,
> neither by the type system. If the boundary is ever broken in review, the answer is
> to lift `provisioner` into its own module, not to add a third guard.
>
> ~~Where `provisioner` lives in the repository: `cmd/provisioner` inside the `api`
> module, or its own module. The first is simpler and shares the platform wiring; the
> second is more honest about the boundary — a component that must not be able to
> reach the database will not compile against `pgx`. I lean toward the second
> precisely because of the dependency graph.~~

> [!question] The `USERSET` time limits are called a default rather than a barrier.
> Whether to additionally forbid lifting them — say, with a separate role lacking
> the `SET` privilege — has not been checked and is probably impossible without a
> superuser. Written down so it does not look like an oversight.

---
type: spec
project: cadence
status: done
priority: p2
created: 2026-07-28
todoist_parent: "6h8vJH5qvF2cvq2H"
components: [api, data-layer, identity, kmp-app, web-dashboard]
proposal: "[[20-Projects/cadence/architecture/proposals/api-openapi-code-first|architecture/proposals/api-openapi-code-first]]"
---

<!-- SNAPSHOT (read-only copy). Master: 20-Projects/cadence/specs/api-skeleton.md in vault prll-vault. Edit the vault note, then re-export — never edit here. -->

# API Skeleton

## Summary

Take `api/` from a skeleton that can only answer `/healthz` to an application that owns its schema, has a working low-privilege request path, a versioned contract, and verified authentication. After this block, the first clinical endpoint is lines in places already prepared, not one more layer of foundation.

The spec passed two independent judges (2026-07-28), and their main finding changed its shape: in the first draft the `cadence_authenticated` role was created `NOLOGIN` and therefore used by nobody — the request path kept running as the database owner, and an acceptance criterion masked that by checking an attribute of a dormant role. The invariant "the database is the authority on access" is delivered only by the combination of three roles and forced RLS.

## User Story

**As a** developer starting M2 with invitations, profiles, and the first RLS policies
**I want** a migration chain with real role separation, an assembled `/v1`, a test harness on real Postgres, Bearer token verification, and a proven impersonation seam
**So that** the first clinical table is added together with its policy into a harness that is ready, rather than together with a foundation that still has to be built

## Acceptance Criteria

**Schema and roles**
- [x] `api/migrations/` is the single chain; the `migrate-up`/`migrate-down`/`migrate-new` targets call `go run ./cmd/migrate`, and the external `migrate` CLI is no longer needed
- [x] The base migration creates three roles: `cadence_owner` (owns the objects; migrations run as it), `cadence_app` (`LOGIN`, owns nothing — this is the role in `DATABASE_URL`), `cadence_authenticated` (`NOLOGIN`, the impersonation target), plus `GRANT cadence_authenticated TO cadence_app`
- [x] A test checks the role **from `DATABASE_URL`**, not the dormant one: `rolsuper = false AND rolbypassrls = false AND rolcreaterole = false`, and that it owns no object in the application schema
- [x] An invariant test walks `pg_class` and requires `relrowsecurity AND relforcerowsecurity` for every table in the application schema. Today it passes vacuously (there are no tables) and becomes a gate the moment the first one appears
- [x] The base migration is verified under a **non-superuser**: in Supabase the `postgres` role is not a superuser, and `ALTER DEFAULT PRIVILEGES` is bound to the creating role and requires an explicit `FOR ROLE`
- [x] `migrate-down` rolls back cleanly: `REVOKE` and `DROP OWNED BY` before `DROP ROLE`, with `CREATE ROLE` guarded by an existence check (roles are cluster objects, not database ones)

**Contract**
- [x] `/v1` is assembled with huma on top of chi; each of the 11 contexts has a route registration point
- [x] `openapi.json` is produced from the types, exported by the `cmd/openapi` command, **committed**, and the gate fails if a rebuild changes it — without that, refactoring a Go type silently renames a field on both client surfaces
- [x] The spec declares the `bearer` security scheme, and the `/v1` operations require it — otherwise the generated clients will not send `Authorization`
- [x] Any non-2xx response is `application/problem+json`: 401, 404, 405, 422, 500, 503. The fifth path of the old shape (`internalErrorBody`, written bypassing `Error()`) is converted too, along with its `Content-Type`
- [x] `detail` for 5xx is a fixed string plus `request_id`; the underlying error is never serialized. The 401 response does not distinguish the reason for refusal — the distinction lives only in the log
- [x] `/healthz` stays outside `/v1`, unauthenticated and outside the OpenAPI spec; `openapi.json` and `/docs` are also unauthenticated

**Tests**
- [x] Integration tests sit behind the `integration` build tag, with a `make test-integration` target and a required CI job. `scripts/gate/go.sh` stays green on a host with no Docker
- [x] A database per test (not a shared transaction): the RLS policies in M2 require connecting as a different role and connection-level GUCs, which a shared transaction does not provide
- [x] The fixtures include a key pair, a local JWKS on an `httptest.Server`, and a helper issuing tokens with arbitrary claims
- [x] A "deny by default" test: a walk over the router via `chi.Walk` requires a 401 without a token from every route except an explicit exemption list
- [x] The `testsupport` package is forbidden from being imported outside `_test.go` via `depguard`, and is closed behind a build tag

**Authentication**
- [x] `SUPABASE_JWT_ISSUER` and `SUPABASE_JWT_AUDIENCE` are mandatory in the config; the JWKS address is **derived** from the issuer and is not a separate knob
- [x] Verified: the signature, a closed algorithm list `{RS256, ES256}`, a mandatory `exp`, a mandatory `kid`, `iss`, `aud`, `nbf`, and a clock-skew allowance
- [x] The JWKS refresh triggered by an unknown `kid` is rate-limited — otherwise a stream of random `kid`s takes authentication down through Supabase's rate limit
- [x] An unavailable JWKS with an empty cache means refusal (401/503), not a pass-through
- [x] `GET /v1/me` returns a **narrow** typed principal (`sub`, `role`, `exp`) rather than Supabase's raw claims: those carry email, phone, and `app_metadata`, and handing them over in bulk would extend the contract implicitly and permanently
- [x] There is no session state in the API

**The impersonation seam**
- [x] `database.WithCaller(ctx, pool, principal, fn)` runs the body in a transaction with the role and claims set; the claims are set **parametrically**, not by string concatenation
- [x] Proven by tests on a temporary table: inside, the intended role is in effect; after exit, the role is reset; a hostile claim value breaks nothing

## Scope / Non-scope

**In scope:** the migration chain and three roles, forced RLS as an enforced invariant, versioned routing, contract generation and pinning, the error shape, the test harness, JWT verification, `GET /v1/me`, the impersonation seam.

**Out of scope, named explicitly:**
- Clinical tables and policies. The base migration creates roles and grants, not data.
- The RLS policy regression suite — it begins in M2 with the first table. What is built here is the harness and the invariant it will stand on.
- Live Supabase. **The SKL-04 item "migrations are applied against cadence-dev" moves to SKL-06** (the deployment applies migrations) — when SKL-04 is closed this is recorded in a comment on the task so the item is not lost.
- Generating clients from `openapi.json` — SKL-11 (Kotlin). For the dashboard, client generation is not described in SKL-09; the question will surface there.
- Invariant 5 of the `data-layer` note ("every migration extends the policy test suite") is formally not satisfied by the base migration: there are no policies, because there are no tables. Deliberate.

## What already exists (DONE)

From BST-06 (`8ed170f`) in `api/`. Verified by the judge against the code on 2026-07-28:

- `cmd/api/main.go` — the composition root, signals, a deadline for connecting to the database **and a deadline for the `/healthz` probe** (the timeout lives here, not in `httpserver.Health`)
- `internal/platform/config` — configuration from the environment with validation at startup, `DATABASE_URL` mandatory, `DATABASE_SERVICE_URL` read and empty by default, 152 lines of tests
- `internal/platform/httpserver` — chi, RequestID, a structured log, its own recoverer, a CORS allow-list, graceful shutdown, `/healthz`, tests
- `internal/platform/database` — the pgx pool with fail-fast, `HealthCheck`, `RunMigrations` (written, and **called nowhere** — the only mentions are inside the file itself)
- `internal/<11 contexts>` — packages with a `doc.go` and no code
- `Makefile` — already contains `migrate-up`/`migrate-down`/`migrate-new`, and they call the **external `migrate` binary**, which is present neither in the gate nor in CI. They have to be moved onto `cmd/migrate`
- `.golangci.yml` v2, `.env.example`, `docker-compose.yml` with `postgres:17-alpine` on 5433

**What gets rewritten.** `ErrorResponse{error, code}` is used on four product paths — 404 and 405 (`server.go`), 500 on panic (`server.go`), 503 (`health.go`) — and there is a **fifth**: the `internalErrorBody` constant in `response.go`, written straight into `JSON()` on a marshalling failure, bypassing `Error()` and covered by no test. There are **three** test files reading the type: `response_test.go`, `server_test.go`, and `health_test.go`. Deleting the type without amending all three breaks the package build.

## Technical detail

**New dependencies** (existence and compatibility verified by the judge; `humachi` was built and run with our chi on Go 1.26.4):

| Module | Version | What for |
|---|---|---|
| `github.com/danielgtaylor/huma/v2` | v2.39.0 | typed routes, validation, OpenAPI generation |
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | parsing and verifying JWTs |
| `github.com/MicahParks/keyfunc/v3` | v3.8.1 | a caching JWKS key source |
| `github.com/testcontainers/testcontainers-go` | v0.43.0 | real Postgres in tests |
| `github.com/testcontainers/testcontainers-go/modules/postgres` | v0.43.0 | a separate module, pulling `pgx v5.9.2` — MVS resolves it against our v5.10.0 |

**Roles.** Three, not one. `cadence_owner` owns the schema and the objects, and migrations are applied as it (`DATABASE_MIGRATION_URL`). `cadence_app` is `LOGIN`, `NOSUPERUSER`, `NOBYPASSRLS`, `NOCREATEROLE`, and owns nothing; this is `DATABASE_URL`. `cadence_authenticated` is `NOLOGIN`, the target of `SET LOCAL ROLE`; without `GRANT cadence_authenticated TO cadence_app` the seam from step-4 simply will not run. A role without RLS bypass that owns the tables does not stop being a hole: the owner can disable RLS, so ownership and connection are separated here rather than in M2.

**Mounting huma.** `humachi` v2.39.0 has no prefix-aware variant: when mounted on a `/v1` subrouter the operation is served correctly, but the document ends up with `paths: {"/me": …}` and the document itself moves under the auth middleware — verified experimentally. So routes are registered with full `/v1/...` paths on the root mux, and the middleware is attached by prefix with explicit exemptions (`/healthz`, `openapi.json`, `/docs`).

**Token verification.** Besides the signature: a closed algorithm list `{RS256, ES256}` (Supabase issues ES256 by default for new projects and RSA for those created after May 2025 — the actual algorithm of `cadence-dev` is confirmed in SKL-01), a mandatory `exp` (in `jwt/v5` it is not mandatory by default), a mandatory `kid` (otherwise keyfunc tries keys in turn), `iss`, `aud`, `nbf`, and a clock-skew allowance. Separately: Supabase's `aud` equals `authenticated` in every project and provides no project isolation — the binding is carried by `iss` and by the identity of the keys. This has to be stated honestly, so that the test "foreign audience → 401" does not create a false sense of protection.

**The `Authorization` header** is parsed with a case-insensitive scheme; a token in the query string is forbidden — the same rule as for the socket in the overview ("no token in the URL, so it does not end up in logs").

**A known limitation.** The role is taken from the token, with no database lookup. Demoting a doctor takes effect after the token's TTL, and the RLS policies in M2 will inherit that boundary. Recorded now rather than discovered later.

**Test isolation** — a database per test. A shared transaction is cheaper but does not allow connecting as a different role and checking connection-level GUCs, which is exactly what the policy test suite in M2 will require. The container is shared across the run via `TestMain`, so parallel packages do not each stand up a Postgres.

**Protection against bypassing authentication.** A mechanism, not an intention: `testsupport` is closed behind a build tag and forbidden from being imported via `depguard` everywhere except `_test.go`; the JWKS URL is derived from the issuer and is not a separate knob — otherwise production differs from the test by one variable in Railway; `SUPABASE_JWT_ISSUER` and `SUPABASE_JWT_AUDIENCE` are mandatory, and an empty value fails startup rather than silently disabling verification.

**Leaks in problem+json.** The format carries `detail`, `instance`, and `errors[].value`. The danger is concrete: a wrapped pgx error carries a fragment of SQL with table names into `detail`, and `errors[].value` echoes the submitted value back — for the dose-logging form that is medical data, and it would reach Sentry too. The current code holds this property and has non-leakage tests; on a change of format it must be preserved.

**Files:**

```
api/
  cmd/migrate/main.go                       applying the chain (reuses RunMigrations)
  cmd/openapi/main.go                       exporting openapi.json without starting the app
  openapi.json                              committed; the gate checks for drift
  migrations/000001_base.up.sql|.down.sql   three roles, grants, default privileges with FOR ROLE
  internal/platform/httpserver/
    problem.go, problem_test.go             RFC 7807 instead of ErrorResponse
    api.go                                  assembling huma.API on the root mux
  internal/platform/auth/
    jwks.go, principal.go, middleware.go    + tests
  internal/platform/database/
    caller.go, caller_test.go               the impersonation seam
  internal/platform/testsupport/            build tag + depguard
    postgres.go, keys.go
  internal/identity/handler.go              GET /v1/me
```

## Architecture decision

The contract is born from the code: huma on top of chi, and the spec is a build output. The rationale and the alternatives are in the [[20-Projects/cadence/architecture/proposals/api-openapi-code-first|proposal note]].

The judge raised the strongest counter-argument: under code-first the contract becomes whatever the code happened to be, and renaming a Go struct field silently renames a JSON field on two client surfaces while looking like a refactor in review. The argument is half fair and closes cheaply: `openapi.json` is committed, and the gate fails if a rebuild changes it. A contract change must appear in a diff and be confirmed deliberately — the review artifact comes back without moving to spec-first.

The error shape is RFC 7807. Migrations are applied by a separate command under their own role: the "Environments" section of the overview requires it, and it also removes the race with several instances.

## Component deltas

### api.md
- MODIFIED: "Shape" — `huma` is added (typed `/v1` routes, validation, OpenAPI generation) on top of `chi`; `golang-migrate` is applied by a separate `cmd/migrate` command under the owner role.
- MODIFIED: "Contracts" — `openapi.json` is produced from the types, committed, and protected from drift by the gate; the `bearer` security scheme is declared; `/healthz`, `openapi.json`, and `/docs` are outside the spec and outside authentication; the shape of any non-2xx response is `application/problem+json`.
- ADDED: to "Shape" — `internal/platform/auth` (token verification), `internal/platform/testsupport` (a harness closed off from production by a build tag and depguard), `database.WithCaller` (the impersonation seam).
- ADDED: to "Invariants" — the request-path role owns no schema objects; separating the owner from the application is mandatory.
- REMOVED: from "Open questions" — choosing the server-side generator. Choosing the Kotlin client generator remains (SKL-11).

### data-layer.md
- MODIFIED: invariant 1 — clarified that forced RLS is verified by an executable invariant test over `pg_class` rather than merely declared.
- MODIFIED: invariant 3 — the chain lives in `api/migrations/` and is applied by the `cadence_owner` role; the request path runs as `cadence_app`, which owns nothing.
- ADDED: the names of the three roles and their purpose.

### identity.md
- ADDED: to "Contracts" — `GET /v1/me`, returning a narrow principal (`sub`, `role`, `exp`) from a verified token, with no database lookup.
- ADDED: a known limitation — the role is taken from the token, so a role change takes effect after the token's TTL.

### kmp-app.md, web-dashboard.md
- MODIFIED: the client generation source is named precisely — the committed `openapi.json` of the `api` module, including the `bearer` security scheme.

## Decomposition

### step-1: Roles, the migration chain, and the Postgres harness

Three roles separating ownership from connection, the grants, and default privileges with an explicit `FOR ROLE`. `cmd/migrate` under its own URL; the Makefile targets move onto it, and the external CLI goes away.

The test harness in full: a container via `TestMain`, a database per test, applying the chain. On top of it, the substantive tests: the attributes of the role from `DATABASE_URL`, the absence of ownership, a clean rollback, application under a non-superuser, and the forced-RLS invariant test over `pg_class`.

Integration tests are closed behind the `integration` build tag; `make test-integration` and a required CI job appear. The fast gate stays green without Docker.

> [!deviation] 2026-07-29 — migrations do not connect as `cadence_owner`, they become it
> Spec said: `cadence_owner` owns the schema, migrations run as it, and it is in `DATABASE_MIGRATION_URL`. Actually done: `cadence_owner` is created `NOLOGIN`, and `DATABASE_MIGRATION_URL` carries a **bootstrap role** — `postgres` in Supabase, the container's superuser locally — which is granted membership in `cadence_owner` and reaches ownership through `SET ROLE`. Why: golang-migrate takes one connection string for the whole chain, and `cadence_owner` is created by the very first migration — on the first application there is nobody to connect as.
> A consequence that matters for M2: the role from `DATABASE_MIGRATION_URL` is a `CREATEROLE` role permanently belonging to `cadence_owner`, that is, capable of disabling RLS on any application table. The policy test suite is written against that fact, not against a picture in which "the migrator left and never comes back".
> Every subsequent migration must begin with `SET ROLE cadence_owner`. This is not discipline but an invariant: the integration walk over `pg_class` fails on any schema table the owner does not own.

> [!deviation] 2026-07-29 — `cadence_app` is created `NOINHERIT`, and the chain declares attributes rather than creating them once
> Spec said: three roles and `GRANT cadence_authenticated TO cadence_app`. Actually done: the same plus `NOINHERIT` on `cadence_app` and an explicit `WITH INHERIT FALSE` on the membership. Why: with inheritance the membership hands `cadence_authenticated`'s privileges to any query without any `SET ROLE` — the seam from step-4 remains callable but stops being mandatory, and skipping it becomes invisible. With `NOINHERIT` a query that bypasses the seam has no privileges on the tables at all and fails loudly.
> Three findings that made this take more than one line. Since PostgreSQL 16 the creating role receives membership with `SET FALSE` — `pg_has_role` shows `MEMBER` while `SET ROLE` is forbidden. Inheritance lives in the membership itself, and a membership is keyed by its grantor: a `GRANT` amends only its own row, so the chain first does a `REVOKE` and then grants anew. And what the chain cannot revoke — a membership granted by a superuser — it **detects and stops on**, with explicit text, instead of reporting a separation it did not achieve.
> The `SUPERUSER` and `BYPASSRLS` attributes can only be changed by someone who holds them, so they are checked rather than set: a foreign role carrying them fails the application of the chain.

> [!deviation] 2026-07-29 — the application schema is named `app`; `cmd/migrate` gained a `force`
> Spec said: "the application schema", unnamed. Actually done: `app`. Why: a dedicated schema makes the question "what does the request path own" precise and keeps our objects away from `public`, which in Supabase is shared with extensions.
> `cmd/migrate force <n>` is beyond the spec. Why: a chain that fails midway marks the version `dirty`, after which every `up` and `down` refuses to work. The external `migrate` CLI, which could do `force`, goes away with this step; without a replacement, recovering from a partial application would mean editing `schema_migrations` by hand in production.

todoist: "6h8vJP3vWQXJxHWq"

### step-2: huma, `/v1`, problem+json, and the pinned contract

huma via `humachi` on the root mux, with routes registered as full paths. Registration points for the 11 contexts. The `bearer` security scheme in the spec's configuration.

The error shape moves to problem+json on all five paths, including `internalErrorBody` and the `Content-Type`; three test files are rewritten; the non-leakage tests are preserved. `cmd/openapi` exports `openapi.json`, the file is committed, and the gate checks for drift.

> [!deviation] 2026-07-29 — the context registry lives in `internal/router`, not in `platform`
> Spec said: the step's files are `problem.go`, `api.go`, and `cmd/openapi/main.go`. Actually done: plus a new `internal/router` package and a `routes.go` in each of the eleven contexts. Why: a context's registration point belongs in the context itself, otherwise "where does routing live" gets decided anew with every endpoint. And the list of all eleven must be in one place so that the claim "every context is mounted" becomes verifiable: a test reconciles the registry against the directories under `internal/` and fails on a twelfth context that was added and forgotten.
> Why not in `platform`: `platform` is transport, config, and database, and it does not know about the domain contexts. The registry knows about all eleven, so importing it from `platform` would invert the dependency. The cost is one special case in the registry test, excluding the `router` package itself from the list of contexts.

> [!deviation] 2026-07-29 — error scrubbing happens in `MarshalJSON`, and logging in huma's transformer
> Spec said: the error shape moves to problem+json, and `detail` for 5xx is a fixed string. Actually done: the same, but scrubbing moved into `Problem.MarshalJSON` and the log write into huma's transformer. Why: the review showed that the first draft logged nothing, although the code claimed in four places that "the distinction lives in the log". The reason is specific: `huma.Error500InternalServerError` builds the error **without a context**, so a construction hook does not see the request and cannot write the `request_id` — and that covers most errors. huma's transformer is the only hook that sees every error together with its request.
> Scrubbing accordingly moved into `MarshalJSON`, because by the time of the transformer the value still has to be complete (otherwise there is nothing to log), and by the time of the bytes it has to already be empty. The side effect turned out to be more valuable than the intent: there is no path to the wire that bypasses marshalling, so forgetting to scrub is now physically impossible.
> huma's `SchemasPath` and `CreateHooks` were also disabled: by default it mixes a `$schema` field into every body, assembled from the `Host` header, and opens a third unauthenticated path, `/schemas`. The first would give the error two different shapes — one from huma and one from the router — and the second is something step 3 would have had to know about.

todoist: "6h8vJP7cfmxpcXrq"

### step-3: Token verification against JWKS and `GET /v1/me`

The key fixture: a pair, a local JWKS, and issuance of tokens with arbitrary claims. Middleware by the `/v1` prefix with exemptions. The full list of checks from the technical detail. `SUPABASE_JWT_ISSUER` and `SUPABASE_JWT_AUDIENCE` mandatory in the config, with JWKS derived from the issuer; `config.go`, `config_test.go`, and `.env.example` are amended.

`GET /v1/me` returns a narrow principal. Tests: a valid token — 200; expired, without `exp`, with a foreign issuer, with a foreign `kid`, with a broken signature, with a forbidden algorithm, and with no header — 401 with an indistinguishable body. The "deny by default" test via `chi.Walk`. `testsupport` closed behind the tag and `depguard`.

> [!deviation] 2026-07-29 — the key fixtures have no build tag; the mechanism is `depguard`
> Spec said: `testsupport` is closed behind a tag and `depguard`. Actually done: the `integration` tag stayed only on `postgres.go`; `keys.go` — key pairs, the local JWKS, and token issuance — has no tag. Why: the tag would have closed every authentication test off from the fast gate: `go test ./...` would stop building them, and silently. A silently skipped authentication test is worse than a slow one.
> Protection against reaching production is not weakened by this, because it was never the tag that provided it: `depguard` forbids importing `testsupport` from any file other than `_test.go`. The tag was a second layer for the package that needs Docker — and that is where it stayed.

> [!deviation] 2026-07-29 — the middleware closes everything, not the `/v1` prefix
> Spec said: middleware by the `/v1` prefix with exemptions. Actually done: the guard is attached to the whole mux, and the exemptions are an exact (not prefix) list of six transport paths: `/healthz`, `/docs`, and the four documents huma mounts on a single `OpenAPIPath` (the current version and the downgrade to 3.0, each in JSON and YAML). Why: the rule "we close `/v1`" leaves everything outside `/v1` open, and every path added later is open until somebody remembers to close it. The inverse formulation errs on the safe side: a path nobody thought about answers 401. As a bonus, an unknown path stops being distinguishable from a known one — 401, not 404.
> Exact comparison rather than prefix: `/healthz` as a prefix would also open `/healthzzz`. Both properties are pinned by tests, and both were verified by mutation.
> Beyond the step's files, `internal/router/Mount` appeared — the single assembly of the HTTP surface, called both by `cmd/api` and by the `chi.Walk` test. Without it, the test would walk a router assembled for the test and would assert nothing about what the process listens on.

> [!deviation] 2026-07-29 — `GET /v1/me` answers with `expires_at` in RFC 3339
> Spec said: a narrow principal `sub`, `role`, `exp`. Actually done: `sub`, `role`, `expires_at` — a typed `time.Time`, `format: date-time` in the contract. Why: a field named `exp` reads as the claim of the same name, that is, as a number of seconds, and returning an RFC 3339 string under that name is an invitation to parse it wrongly on both client surfaces. The set of fields did not change: there are three, and a test fails if there are four.

> [!deviation] 2026-07-29 — the budget for an unscheduled JWKS refresh is 3 s, not 100 ms
> Spec said: the refresh triggered by an unknown `kid` is rate-limited. Actually done: the same, plus an explicit `RateLimitWaitMax` of 3 seconds. Why: in `jwkset` v0.11.1 that parameter limits more than the wait for quota — a context is made from it and passed into the refresh HTTP request itself. A value chosen "so as not to hang the request" silently becomes the request timeout, and at 100 ms an unscheduled refresh against Supabase never completes: after a key rotation, every token with the new `kid` is rejected until the scheduled refresh an hour later. Found by review, reproduced with a fixture delayed by 300 ms which is now in the suite — on localhost this bug is invisible.
> This does not bring the hang back: with a quota of "once per 5 minutes" a rate-limited request would have to wait minutes, which exceeds the deadline, so the limiter refuses immediately. The budget is spent only by the single refresh that is permitted.
> The error classification was corrected in the same place: an exhausted quota is a refusal of the token (`ErrTokenRejected`), not key unavailability. Otherwise a stream of junk `kid`s fills the log with an incident that is not happening, and a real incident becomes indistinguishable from it.
> A second review round refined the wording: one deadline covers both the wait for quota and the request itself, and the limiter sleeps if the wait fits inside the deadline. So a request arriving at the end of the window would get permission with a nearly exhausted deadline — the same defect, but once per 5 minutes instead of once per hour. The deadline is assembled as a sum: 250 ms for the wait plus 3 s guaranteed for the request. At most one request can end up sleeping — the limiter reserves the quota before sleeping, and everything arriving alongside sees a wait of minutes and refuses immediately.

> [!deviation] 2026-07-29 — the `WWW-Authenticate` header belongs to the status, not to the caller
> Spec said: any non-2xx is `application/problem+json`, and a 401 does not distinguish the reason for refusal. Actually done: the same, plus `Problem.GetHeaders()` — an implementation of `huma.HeadersError` used by both writers, `WriteProblem` and huma's error path. Why: RFC 7235 requires a challenge on every 401, and the type has two writers. A challenge set at the call site is a rule that one of the two will eventually skip, and then the header set starts telling you which layer refused. This is the same argument already recorded above `normalise`.

todoist: "6h8vJP8V4VH2JHvq"

### step-4: The impersonation seam

`database.WithCaller(ctx, pool, principal, fn)`: a transaction, `SET LOCAL ROLE cadence_authenticated`, and claims through `set_config` with a parameter — not by string concatenation, otherwise a claim value from the token becomes an injection vector against our own session.

Proven on a temporary table inside the test transaction: inside, the intended role is in effect; after exit, the role is reset; the claims are readable; a hostile claim value breaks nothing. No clinical tables are needed for this — and that is precisely why the seam is written now, while the harness is fresh, rather than together with the first table in M2.

> [!deviation] 2026-07-29 — the proof is on an owner's table in the `app` schema, not on a temporary one
> Spec said: proven on a temporary table inside the test transaction. Actually done: the probe is an ordinary table in the `app` schema, created under `SET ROLE cadence_owner` through the bootstrap role, in a separate database per test (isolation exists anyway — a database per test, not a shared transaction). Why: a temporary table created by `cadence_app` would belong to `cadence_app` and be accessible to it regardless — the test would pass while saying nothing about default privileges for `cadence_authenticated` or about `NOINHERIT`, that is, nothing about why the seam exists. A probe in `app` under the owner is exactly the shape of table that will appear in M2, and its privileges come from the base migration rather than from a line added in the test.
> A side result: the same suite proves the converse property too — a query bypassing the seam gets `42501` on an application table. Without the owner's probe there is nothing to state that claim on.

> [!deviation] 2026-07-29 — the seam takes `database.Caller`, not `auth.Principal`
> Spec said: `database.WithCaller(ctx, pool, principal, fn)`. Actually done: the third argument is `database.Caller{Subject, Role}`, declared in `database` itself. Why: the project rule is that interfaces belong to the consumer, and the core does not import drivers or transports. By accepting `auth.Principal`, the database layer would start depending on a package that knows about JWKS, HTTP headers, and token lifetimes. The cost is one struct literal at the call site; the gain is that `database` does not know authentication exists at all.
> At the same time, two boundaries M2 must close rather than inherit were written into `Caller`'s doc comment: `Subject` is checked only for non-emptiness (the Supabase idiom casts `sub` to `uuid`, and a policy written that way gives `22P02` on a non-UUID — that is, a 500 instead of a refusal), and in a stock Supabase project `Role` equals the literal `authenticated` for everyone — the product roles arrive through the token issuance hook, and exactly where the hook puts them is decided in M2.

todoist: "6h8vJPHrWcQGPhWH"

## Open questions

> [!decision] 2026-07-28 — yes, added. The item "pin down the actual token signing algorithm and `iss`" was added to the description of task SKL-01. The spec holds a closed list `{RS256, ES256}`, so step-3 is not blocked; confirming the algorithm is a precondition of the first connection to live Supabase (SKL-06), not of this work.

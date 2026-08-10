---
type: spec
project: cadence
status: approved
priority: p1
created: 2026-07-30
todoist_parent: "6h9HmMjhjgWQRmJq"
components: [identity, data-layer, api, audit]
proposal: "[[20-Projects/cadence/architecture/proposals/identity-schema-and-policies|architecture/proposals/identity-schema-and-policies]]"
---

<!-- SNAPSHOT (read-only copy). Master: 20-Projects/cadence/specs/identity-schema-and-policies.md in vault prll-vault. Edit the vault note, then re-export — never edit here. -->

# Identity: the schema with forced RLS, the access policies, and the policy test suite

## Summary

The first tables of the `app` schema and the first access model. Today the schema
is empty: `000001_base` lays out three roles and default privileges, and the
`pg_class` walk demanding forced RLS of every table passes vacuously for want of
tables. This block puts down six tables, seven roles, six policy shapes, two
registries of grants and policies in the tests, two separate write seams, and a
regression suite proving the isolation of medical data by behaviour.

The spec went through three rounds of independent review and was rewritten twice:
the first draft specified a privilege escalation, and the second tried to close it
with column grants, which is impossible while the product role is a claim rather
than a Postgres role. The account of the rounds is in the proposal.

## User Story

**As a** clinic doctor
**I want** to see the data only of the patients assigned to me
**So that** the isolation of medical data is held by the database rather than by
careful code

## Acceptance Criteria

- [ ] Six tables are created in the `app` schema, owned by `cadence_owner`, with RLS enabled and forced on each; every §03 column is either present or its absence is recorded as a decision
- [ ] The **grants** registry reconciles `has_table_privilege` and `has_column_privilege` for all seven roles on every table; a surplus or missing grant fails the test
- [ ] The **policies** registry reconciles, per table, the set of policies, their verbs, and their `TO`; a policy with no explicit `TO` (`polroles = '{0}'`) fails the test
- [ ] The **functions** registry reconciles, via `pg_proc`, the owner, `prosecdef`, the **value** of `proconfig`, and the ACL; a function in the `app` schema with `EXECUTE` for `PUBLIC` or without `search_path = pg_catalog, pg_temp` fails the test
- [ ] Default privileges do not exist: there is no `ALTER DEFAULT PRIVILEGES` in the chain, and nothing is granted on sequences
- [ ] `CREATE ROLE … BYPASSRLS` under the role from `DATABASE_MIGRATION_URL` fails — the block's assumption is verified rather than quoted
- [ ] `SET ROLE cadence_service` from a `cadence_app` connection fails with `42501`; `pg_auth_members` contains no (`cadence_service`, `cadence_app`) pair; no membership of any of the seven roles carries `inherit_option`
- [ ] A patient who overwrites `request.jwt.claims` inside their own transaction **does not become an admin**: the product role is a Postgres role, not a claim. The test asserts the residual property — subject substitution within one's own role — with a reference to the invariant
- [ ] A patient cannot change `profiles.role` — `cadence_patient` has no grant on the column; `has_column_privilege('cadence_patient','app.profiles','role','UPDATE')` is false
- [ ] A patient can neither insert nor delete a `care_team_assignments` row, including their own: deleting an assignment would be a way to hide from the doctor
- [ ] A patient cannot write the clinical fields of `patient_profiles` (`dob`, `sex`, `height_cm`) — the clinic enters those through the service path
- [ ] A patient reads `profiles` and `provider_profiles` of every specialist assigned to them, and reads none who is not assigned
- [ ] A doctor reads their own profile and the profiles of assigned patients, does not see an unassigned one; after **adding** an assignment row visibility appears, after **deleting** it visibility disappears, in both cases without editing the query
- [ ] No `qual`/`with_check` in `pg_policies` of the `app` schema contains the literal `'doctor'`, `'patient'`, or `'admin'` — the role decision is made by `TO`, not by comparing values
- [ ] A token with no recognized `cadence_role` is refused by the seam before a transaction is opened; `GET /v1/me` answers with an empty role in that case, because it never touches the database
- [ ] A query that got past the seams fails with `42501` and reads no table
- [ ] An admin reads and writes everything **except `audit_log`: there, read only** — no policies and no `UPDATE`/`DELETE` grants exist for any role
- [ ] `audit_log` rejects `UPDATE`, `DELETE`, and `TRUNCATE` under every request-path and service role; a row with no attribution, with both attribution fields filled, or with an empty `actor_job` is rejected by a constraint
- [ ] An `audit_log` row whose actor does not match the published seam is rejected by policy; in the spec and in the note this is recorded as a "do not forget to name the actor" property, not as protection against forgery
- [ ] The service function creates a patient (four rows) and a `patient.create` audit record in one transaction; a rollback leaves neither rows nor audit; a repeated patient-specialist pair is rejected by `UNIQUE`
- [ ] A nested call of either seam inside the other **refuses** rather than blocking on `Acquire`
- [ ] Verified at startup: the request-path pool is connected as `cadence_app`, the service pool as `cadence_service_app`, neither is a superuser and neither carries `rolbypassrls`; `DATABASE_SERVICE_URL` is mandatory
- [ ] `WithCaller` rejects a subject that is not a UUID before opening a transaction; `app.jwt_subject()` returns `NULL` on an unverified value rather than `22P02`; the role name for `SET LOCAL ROLE` comes from a closed map of constants, never from token data
- [ ] The hook always writes `cadence_role` deterministically: if a profile exists it puts the role in, if not it removes the key; an event arriving with `cadence_role: admin` already substituted does not contain it on the way out; the mandatory claims are not discarded; token issuance does not fail in any case
- [ ] `cadence_gotrue` has neither `USAGE` on the `app` schema nor `SELECT` on `profiles` — only `EXECUTE` on the hook; revoking membership in `cadence_auth_hook` breaks token issuance
- [ ] `go list -deps ./internal/identity | grep -E 'MicahParks|golang-jwt'` returns nothing: the bounded context no longer compiles against the token-verification libraries
- [ ] The gate forbids a non-constant expression in `Exec`/`Query`/`QueryRow`/`SendBatch` calls in `api/internal/**`; `gosec` G201/G202 are enabled as a backstop; a test verifies that neither pool's execution mode is `QueryExecModeSimpleProtocol`
- [ ] `make gate` and `make test-integration` are green, including the edits to existing tests listed in the steps; `openapi.json` is regenerated in the same commit that changes the role field description

## Scope / Non-scope

**In scope.** Six tables with their columns, constraints, and indexes. Seven roles,
six policy shapes, explicit grants per role and per column, two registries in the
tests plus the functions registry. A second pool on `DATABASE_SERVICE_URL` with a
startup check, the `WithService` seam, the mapping of the claim to a Postgres role
in `WithCaller`. The subject-reading function. The patient creation path through the
service function with audit in one transaction. The token issuance hook and a
separate non-superuser GoTrue role locally. The regression suite. The gate on SQL
authorship.

**Out of scope.** `invites`, the GoTrue Admin API, emails, mail configuration. The
`POST /v1/patients` HTTP endpoint: without sending the invitation it is half a
thing, and `openapi.json` is committed and spawns two clients.
`GET /v1/dashboard/overview` v0 — it reads through `WithCaller` and proves the same
policies the suite proves directly. Person seeds. Dashboard sign-in and the KMP
screens. Storage policies — per ADR-008 the files live in S3, and there are no
row-level policies there. A hermetic GoTrue container in the test harness — it
arrives with the invitation integration tests (build plan, IDN-17). The account
deletion procedure with refresh-token revocation — the next block. Exporting
`audit_log` outside the cluster — deployment. The tables of the other ten contexts.

## What already exists (DONE)

- `api/migrations/000001_base.up.sql` — three roles, the `app` schema, default
  privileges on four verbs for `cadence_authenticated`, a guard against a role with
  surplus attributes (hardcoding three names), and a check that `cadence_app` does
  not inherit, with an account of the PG16 trap.
- `database.WithCaller` — the impersonation seam: claims as a bound parameter, then
  `SET LOCAL ROLE`, all for the duration of the transaction. `impersonatedRole` is
  the constant `cadence_authenticated`, which this block replaces with three.
- `internal/platform/auth` — Bearer token verification against JWKS, a `Principal`
  of three fields, a guard on everything except an exhaustive list of open paths.
- `internal/platform/config` — `DatabaseConfig.ServiceURL` already exists with the
  comment "reserved for system jobs" (`config.go:42`), `DATABASE_SERVICE_URL` is in
  `.env.example` and in the tests, but is **not validated**. The second pool is
  built on ground already prepared.
- `internal/platform/testsupport` — a Postgres container per package, `NewDatabase`,
  `NewEmptyDatabase`, a non-superuser bootstrap role, constants for role names.
- `base_migration_integration_test.go` — `pg_class` walks for forced RLS and
  ownership plus their "catching" counterparts, which plant a violating table. The
  walks are capable of failing; what they lack is real tables.
- `TestChainAppliesWithoutSuperuser` — a guard on the premise: the role applying the
  chain is not a superuser. It is **not** proof that `BYPASSRLS` is unnecessary:
  nothing tries to create anything with that attribute.
- `caller_integration_test.go` — eight hostile subjects pass through and come back
  verbatim; among the payloads there is already `'); SET ROLE cadence_owner`.
- The local environment `make dev-up` — Postgres and GoTrue v2.194.0, ES256, JWKS at
  the derived address, registration disabled. GoTrue connects as
  `POSTGRES_USER: cadence`, that is, as the **container's superuser** — which is
  what step 8 is obliged to fix.
- The chain lives in `cadence_schema_migrations` so as not to fight with GoTrue.

## Technical detail

### Roles

Seven. `000001_base` is amended (the decision is in the proposal).

`cadence_owner` — owns the schema, the objects, and the hook function, `NOLOGIN`.
`cadence_app` — LOGIN, `DATABASE_URL`, a member of the three product roles and only
those, each membership with a preceding `REVOKE` and `WITH INHERIT FALSE`.
`cadence_patient`, `cadence_doctor`, `cadence_admin` — `NOLOGIN`, the impersonation
targets of the request path. `cadence_service_app` — LOGIN,
`DATABASE_SERVICE_URL`, a member of `cadence_service` only. `cadence_service` —
`NOLOGIN`, the impersonation target of the service path.

`cadence_authenticated` is **abolished**: an intermediate group role would bring
back exactly the indistinguishability of the product roles from which the second
round's escalation grew. This touches `caller.go` (`impersonatedRole`),
`testsupport/postgres.go` (`AuthenticatedRole`), and the base migration.

`cadence_app`'s membership in `cadence_service` is never granted. The attribute
guard and the `ALTER ROLE … NOLOGIN/NOINHERIT` declarations extend from three names
to all seven — otherwise a role created by hand with `INHERIT` or `BYPASSRLS` slips
past `IF NOT EXISTS` and past the violator walk. The PG16 `inherit_option` check
gains a counterpart for every membership, not just for the original pair.

### Grants

There are no default privileges: `ALTER DEFAULT PRIVILEGES` is removed from the
chain — it does not distinguish product roles and would contradict the empty cells
of the matrix. Nothing is granted on sequences: `audit_log.id` is declared
`GENERATED ALWAYS AS IDENTITY`, and `SELECT` on the sequence would be a volume
oracle bypassing RLS. Every grant appears in its own table's migration.

| table | `cadence_patient` | `cadence_doctor` | `cadence_admin` | `cadence_service` | `cadence_owner` |
|---|---|---|---|---|---|
| `profiles` | SELECT; UPDATE(`full_name`, `timezone`, `locale`) | SELECT | SELECT, INSERT, UPDATE, DELETE | SELECT, INSERT, UPDATE | SELECT |
| `patient_profiles` | SELECT; UPDATE(`target_weight_kg`) | SELECT | SELECT, INSERT, UPDATE, DELETE | SELECT, INSERT | — |
| `provider_profiles` | SELECT | SELECT | SELECT, INSERT, UPDATE, DELETE | SELECT, INSERT | — |
| `care_team_assignments` | SELECT | SELECT | SELECT, INSERT, UPDATE, DELETE | SELECT, INSERT, DELETE | — |
| `user_preferences` | SELECT, UPDATE | — | SELECT, INSERT, UPDATE, DELETE | SELECT, INSERT | — |
| `audit_log` | — | — | SELECT | INSERT | — |

`profiles.role` and `profiles.user_id` are granted to no product role other than
`cadence_admin`; the clinical `dob`, `sex`, and `height_cm` only to the service
path. `cadence_owner`'s `SELECT` on `profiles` exists precisely for the hook's
defining role. `cadence_doctor`'s grants in this block are read-only: edits to the
patient card arrive with the dashboard.

### Policies — six shapes, each with an explicit `TO`

A policy without `TO` applies to `PUBLIC`, that is, to the service path as well, so
`TO` is mandatory and is verified by the registry.

Own rows — `TO cadence_patient, cadence_doctor USING (user_id =
app.jwt_subject())`, per verb; on `user_preferences` only `TO cadence_patient`. The
profiles of one's own specialists — `TO cadence_patient FOR SELECT` on `profiles`
and `provider_profiles`, an `EXISTS` over `care_team_assignments` with
`provider_id = <row>.user_id AND patient_id = app.jwt_subject()`. Assigned patients
— `TO cadence_doctor FOR SELECT`, the same `EXISTS` in the other direction.
Everything for the admin — `TO cadence_admin`, and `FOR SELECT` on `audit_log`. The
service path — `TO cadence_service`, per the verbs in the grants table; on
`audit_log` the `WITH CHECK` additionally requires the actor to match the published
seam. The read for the hook — a narrow `FOR SELECT TO cadence_owner` on `profiles`.

The role decision is made by `TO` rather than by comparing values: `app.jwt_role()`
is unnecessary and is not created. A constraint on the shape: the subquery executes
under the policies of the table being read, and it works because the
`care_team_assignments` policies do not refer back to `profiles`; a back-reference
gives `42P17`. Policies are expressed only through positive equality — negations and
`coalesce` are three-valued and on `NULL` do not do what they look like they do.

### Columns

Closed sets are a `CHECK` on `text`, not `CREATE TYPE`: an added enum value cannot
be rolled back in `.down.sql`.

- `profiles`: `user_id uuid PK` (no FK to `auth.users`), `role text CHECK IN
  (patient, doctor, admin)`, `full_name text CHECK (length between 1 and 200)`,
  `timezone text`, `locale text DEFAULT 'ru'`, `created_at timestamptz DEFAULT
  now()`. **`initials` is not stored** — it is derived from `full_name`, as is the
  avatar colour; both are derived by a single module on each surface (the project
  rule "nothing derived is stored"). The `timezone` value is validated against
  `pg_timezone_names` on the service path in Go: it is a table, and a `CHECK`
  against it is impossible.
- `patient_profiles`: `user_id uuid PK → profiles`, `dob date`, `sex text CHECK IN
  (male, female)` — §03 does not define the set, so we define it here — `height_cm
  numeric`, `target_weight_kg numeric`, `joined_at timestamptz`. `joined_at` is the
  moment of invite acceptance, `profiles.created_at` is the moment the clinic
  created the row: two different facts, not a duplicate.
- `provider_profiles`: `user_id uuid PK → profiles`, `title_ru text`,
  `clinic_name text` (the anchor for future multi-clinic support, §03).
- `care_team_assignments`: `id uuid PK`, `patient_id → profiles`,
  `provider_id → profiles`, `care_role text CHECK IN (endo, dietitian, nurse)`,
  `is_primary boolean`, `since date`, `CHECK (patient_id <> provider_id)`,
  `UNIQUE(patient_id, provider_id)`, a partial unique index `(patient_id)
  WHERE is_primary` — there is one primary specialist — and indexes on
  `(provider_id)` and `(patient_id)`: they are in the subquery of every doctor and
  patient policy, that is, on the hot path of every query. Role consistency — that
  `provider_id` points at a doctor's profile — is provided by the service path and a
  test: a `CHECK` against another table is impossible.
- `user_preferences`: `user_id uuid PK → profiles`, `dose_reminders bool DEFAULT
  true`, `lead_time_min int CHECK IN (15,30,60) DEFAULT 30`, `meal_reminders bool
  DEFAULT false`, `units text CHECK IN (kg,lb) DEFAULT 'kg'`, `time_fmt smallint
  CHECK IN (24,12) DEFAULT 24`, `weekly_report bool DEFAULT true`, `team_messages
  bool DEFAULT true`, `reorder_alerts bool DEFAULT true`.
- `audit_log`: `id bigint GENERATED ALWAYS AS IDENTITY`, `at timestamptz DEFAULT
  now()`, `actor_id uuid NULL`, `actor_job text NULL`,
  `CHECK (num_nonnulls(actor_id, actor_job) = 1 AND coalesce(actor_job,'x') <> '')`,
  `action text`, `entity text`, `entity_id uuid NULL`, `patient_id uuid NULL`,
  `meta jsonb`. The first action literal is `patient.create`.

### The function

`app.jwt_subject()`: `STABLE` (not `IMMUTABLE` — constant folding would give a plan
shared between users), `SET search_path = pg_catalog, pg_temp`, every name
qualified, `nullif(current_setting('request.jwt.claims', true), '')` before the cast
— without `nullif` an empty GUC value gives `22P02` inside a policy — and `NULL`
instead of a cast if the value does not match the UUID format. Immediately after
creation, `REVOKE ALL … FROM PUBLIC`, then `GRANT EXECUTE` to the four
impersonation roles: otherwise `CREATE FUNCTION` grants `EXECUTE` to `PUBLIC`, and
the declared grant would mean nothing.

### Splitting the `auth` package

Pulled into this block from a debt task found by the `golang-standard` audit when
finalizing "API Skeleton": `internal/identity` transitively compiles against
`keyfunc/v3`, `jwkset`, and `golang-jwt/jwt/v5`, because the context needs
`auth.PrincipalFrom(ctx)` and `Principal` sits in the package with the JWKS
verifier. All eleven contexts will go the same way: every endpoint needs the
principal.

This is not cosmetic: ADR-006 counts GoTrue's replaceability as part of the bounded
cost of a migration — "replacing it with our own token issuance is one
implementation, not a sweeping edit". Today that is untrue.

`internal/platform/auth` keeps `Principal`, `WithPrincipal`, `PrincipalFrom` and
zero third-party dependencies — the contexts import it.
`internal/platform/auth/token` takes `Verifier`, `VerifierConfig`, `NewVerifier`,
`ErrTokenRejected`, `ErrKeysUnavailable`, `Middleware` — only the transport and the
composition root import it.

Done here rather than separately, because step 2 edits exactly these files, and
because after the first M2 endpoint it becomes an edit to eleven packages instead of
two.

### The seams

`WithCaller` maps the verified `cadence_role` to a Postgres role name through a
**closed map in Go**; the name is a package constant, never a value from the token.
An unknown or missing claim is a refusal before `Begin`: there is no role to assume.
`Subject` is parsed as a UUID and refused in the same place. Claims are published
under the key `cadence_role`; empty values are not published at all.

`WithService` runs on the `DATABASE_SERVICE_URL` pool: `SET LOCAL ROLE
cadence_service`, claims explicitly cleared, the audit actor published into the
transaction and verified by a policy. Both seams set a marker in the `context` and
**refuse** when nested in either order: a nested call would take a second connection
and an independent transaction, and the outer rollback would leave committed rows.
The cost is named: the combination "check the caller's rights under RLS and write
through the service path in one transaction" is unavailable, and while
`POST /v1/patients` is out of scope this is deferred rather than solved.

`postgres.go` pins the execution mode explicitly — under the simple protocol `pgx`
interpolates arguments on the client, and publishing claims as a bound parameter
would stop being that. We do **not** take `DISCARD ALL` in `AfterRelease`: it
includes `DEALLOCATE ALL`, which conflicts with the prepared statement cache, does
not execute inside a transaction, and has no context. Instead of a reset, a
property: `SET` without `LOCAL` is issued nowhere, and that is verified by a test.

Both pools are checked at startup and by a test: each one's connection role, the
absence of superuser and `rolbypassrls`, and the refusal of `SET ROLE` between the
paths. `DATABASE_SERVICE_URL` becomes mandatory — an empty string in `pgx` falls
through to libpq defaults and in a test container resolves to the superuser, which
would give green tests and different behaviour in production.

### The hook

`SECURITY DEFINER`, owner **`cadence_owner`**, `SET search_path = pg_catalog,
pg_temp`, with `app.profiles` qualified. The owner is not `cadence_service`: the
service pool becomes that role and could `ALTER FUNCTION … RESET search_path` or
`DROP FUNCTION`. No pool belongs to `cadence_owner`. Reading `profiles` as the
defining role is permitted by the narrow `FOR SELECT TO cadence_owner` policy.

`REVOKE ALL … FROM PUBLIC`, then `EXECUTE` and `USAGE ON SCHEMA app` for the
intermediary role `cadence_auth_hook`. The function always writes the key
deterministically: if a profile exists it puts the role in, if not,
`claims - 'cadence_role'`. It never passes claims through as-is: the input is formed
by GoTrue from user data. The mandatory claims (`sub`, `aud`, `exp`, `iss`, `role`)
are not discarded — otherwise GoTrue answers 500.

Locally a `cadence_gotrue` role is created: LOGIN, not a superuser, **owner of the
`auth` schema** — otherwise GoTrue's migrations will not run — with nothing on `app`
beyond `EXECUTE` on the hook. A one-off `auth-schema` step creates the schema
already under its ownership. Today GoTrue connects as the container's superuser, so
any grant check is green regardless of the grants.

### The invariant on SQL authorship

Overwriting `request.jwt.claims` gives **subject substitution within one's own
product role**: the GUC has `USERSET` context and there is nothing to forbid it with
— `SET` is a statement, not a function. But the role is no longer read from the
claims, so that is not a way to become an admin. It is mechanized by the rule "no
non-constant expression reaches `Exec`/`Query`/`QueryRow`/`SendBatch` in
`api/internal/**`": the `gosec` G201/G202 regex goes by SQL keywords and sees
neither `SET`, nor `GRANT`, nor `TRUNCATE` — and `caller.go:127` today is exactly
that shape. `gosec` remains a backstop. Plus a test that **asserts** the residual
substitution.

## Architecture decision

Forced RLS applies to the owner as well, and the chain is applied by a role with
`CREATEROLE` and no superuser, which cannot grant `BYPASSRLS`. So we do not have the
§04 service pool with `BYPASSRLS`, and every write must be permitted by a policy.
This is not a divergence from the partner: their document is correct for Supabase,
where `service_role` is created by the platform. It is an untouched consequence of
ADR-008.

Chosen: the `cadence_service` role without `BYPASSRLS`, with its own LOGIN role and
its own pool (the boundary between the paths runs along session_user rather than
along a single SQL statement, and that is the partner's literal shape — "a
**separate** service-role pool"); and **product roles as Postgres roles**, because
grants are bound to a role: while the admin was a claim, a grant needed by them
reached the patient too, and the escalation could be closed neither by policy nor by
column. The side benefit is substantial — overwriting claims stops yielding an admin
at all.

Rejected: folding the service path into the admin shape via a claim; creating a
`BYPASSRLS` role by hand; closing the escalation with column grants while the role
is a claim. The full account, the three review rounds, and the reversibility of each
decision are in the
[[20-Projects/cadence/architecture/proposals/identity-schema-and-policies|proposal]].

## Component deltas

### data-layer.md
- MODIFIED: invariant 2 — "there are exactly three policy shapes" becomes "exactly six, and each carries an explicit `TO`": own rows, a patient's own specialists' profiles, a doctor's assigned patients, everything for the admin, the service path, and the narrow `FOR SELECT TO cadence_owner` on `profiles` for the hook's defining role
- MODIFIED: invariant 2 — the product role stops being read from claims by policies: the shapes key on `TO`, because Postgres grants are bound to a role, and a claim-role made column grants indistinguishable between patient and admin
- MODIFIED: invariant 2 — the stated reason for rejecting `auth.uid()` is corrected: not "it reads `request.jwt.claim.sub` in the singular" (GoTrue's definition contains a `coalesce` and would have worked), but "the `auth` schema does not exist in the policy-test database", plus our model's independence from a third-party library's migrations
- MODIFIED: invariant 3 — "Three roles" becomes "Seven roles": `cadence_authenticated` is abolished, and `cadence_patient`, `cadence_doctor`, `cadence_admin`, `cadence_service`, and the `cadence_service_app` that connects into it appear; `cadence_app`'s membership in `cadence_service` is never granted
- MODIFIED: invariant 3 — there are no default privileges at all: every grant is issued explicitly in its own table's migration, per role and where needed per column; nothing is granted on sequences
- MODIFIED: invariant 1 — the `pg_class` walk is described as two registries, grants and policies, plus a functions registry over `pg_proc`; the phrase "passes vacuously" is removed
- ADDED: invariant — overwriting `request.jwt.claims` yields subject substitution within one's own product role, because the GUC has `USERSET` context; the role is not elevated by it. Mechanized by forbidding non-constant expressions in `Exec`/`Query` calls in `api/internal/**` and by a test asserting the residual property
- ADDED: a consequence of having no superuser — `BYPASSRLS` is unavailable to the chain, verified by a probing test rather than by documentation
- ADDED: a constraint on policy shape — the subquery executes under the policies of the table being read, and back-references are forbidden as a source of `42P17`; policies are expressed only through positive equality
- ADDED: invariant — both pools are checked at startup: each one's connection role, the absence of superuser and `rolbypassrls`, and the refusal of `SET ROLE` between the paths
- ADDED: to the Auth contract — the `cadence_role` claim is an input for choosing a Postgres role in the seam, not a source of decisions in policies; the stock `role` stays `authenticated`

### identity.md
- MODIFIED: `status: planned` becomes `status: implemented`; six tables exist with their column lists, and `invites` stays planned
- MODIFIED: "Data" — `profiles.user_id` is a `uuid PRIMARY KEY` with no FK to `auth.users`; `initials` is not stored, on a par with the avatar colour, and both values are derived from `full_name` and the identifier by a single module on each surface
- ADDED: invariant — the product role becomes a Postgres role in the seam through a closed map of constants; there is no lookup of `profiles` for the role on the request path, and a token with no recognized role gets a refusal from the seam rather than a default role
- ADDED: invariant — `profiles.role` is writable only by `cadence_admin` and the service path; the patient edits their own rows within a listed set of columns, and the clinical fields of `patient_profiles` are entered by the clinic
- ADDED: invariant — `care_team_assignments` is read-only for the patient: deleting an assignment would be a way to hide from the doctor
- MODIFIED: "Known limitation" — there is no access revocation channel at all: deleting a profile does not interrupt an issued token, the only boundary is the TTL, and this concerns more than demoting a doctor

### api.md
- ADDED: to the wiring — `database.WithService` on a separate `DATABASE_SERVICE_URL` pool, paired with `WithCaller`; both seams refuse when nested inside each other, and the cost of that is the unavailability of atomically combining a caller's rights check with a service write
- MODIFIED: invariant 1 — the request path is `SET LOCAL ROLE` into one of the three product roles through a closed map in Go; the service path is `SET LOCAL ROLE cadence_service` from a separate connection; the role name is always a package constant
- ADDED: to invariant 1 — the query execution mode is pinned explicitly, because under the simple protocol `pgx` interpolates arguments on the client; `SET` without `LOCAL` is issued nowhere, and that is verified by a test instead of resetting the connection
- MODIFIED: "Shape" — the shared wiring is listed as `internal/platform/{auth,auth/token,config,database,httpserver,testsupport}`: `auth` carries only the principal and zero third-party dependencies, `auth/token` carries token verification and the middleware. The contexts import the first, the transport and the composition root import the second, and the replaceability of token issuance from ADR-006 becomes a fact rather than a promise
- REMOVED: from "Open questions" — the item about `Subject` being checked only for non-emptiness
- REMOVED: from "Open questions" — the item about where the product role lives in the token

### audit.md
- MODIFIED: `status: planned` becomes `status: implemented`
- MODIFIED: invariant 1 — "append-only" is a property of the schema against the request and service paths, and **detectable rather than preventable** against the migration chain: the migration role belongs to `cadence_owner` and can drop `FORCE`, delete a policy, or issue a `TRUNCATE`, which RLS does not check
- MODIFIED: invariant 2 — atomicity is provided by forbidding nested seams, otherwise the service transaction commits separately from the outer one
- MODIFIED: invariant 3 — clarified that the service path is authorized in Go rather than by policy: the service-path policy carries no row predicate, and that is named rather than implied
- ADDED: invariant — attribution is mandatory by constraint; the `INSERT` policy reconciling the actor against the published seam adds a "do not forget to name the actor" property and is **not** protection against forgery
- ADDED: to "Data" — the `actor_job` field for named system jobs, which is absent from the source document's model
- ADDED: to contracts — the admin has read-only access to `audit_log`; there is no `FOR ALL` shape on that table

## Decomposition

### step-1: Seven roles, removing default privileges, the SQL authorship gate

Amending `000001_base` up and down: `cadence_patient`, `cadence_doctor`,
`cadence_admin`, `cadence_service`, `cadence_service_app`; abolishing
`cadence_authenticated`; a `REVOKE` before every `GRANT … WITH INHERIT FALSE`;
`USAGE ON SCHEMA app` for all impersonation roles; the attribute guard and the
`ALTER ROLE` declarations extending from three names to seven; the `inherit_option`
check gaining a counterpart for every membership; `ALTER DEFAULT PRIVILEGES` removed
entirely.

The gate: the rule "no non-constant expression reaches `Exec`/`Query`/
`QueryRow`/`SendBatch` in `api/internal/**`", with `gosec` G201/G202 as a backstop.

New tests: `CREATE ROLE … BYPASSRLS` under the migration role fails — a probe of the
assumption the block stands on; `SET ROLE cadence_service` from `cadence_app` fails
with `42501` and works from `cadence_service_app`; `pg_auth_members` contains no
(`cadence_service`, `cadence_app`) pair and no membership with `inherit_option`; no
role named `service_role` — the local environment creates it empty and `NOLOGIN` —
has any grant on the `app` schema.

Existing tests are amended, listed explicitly:
`TestSingleStepRollbackRemovesTheBaseMigration` and `TestDownMigrationIsIdempotent`
count roles against a three-element list and would stay green with the new ones
surviving; `TestChainRefusesAMembershipItCannotRevoke` gains counterparts;
`TestRequestPathReachesTablesOnlyThroughImpersonation` and
`TestRequestPathRoleIsLowPrivilege` move from `cadence_authenticated` to the product
roles; `testsupport/postgres.go` gains constants for all the new roles and loses
`AuthenticatedRole`.
todoist: "6h9HmVPMJQmR8JVq"

### step-2: The product role as a Postgres role, two seams, two pools

`internal/platform/auth` is split into two packages: `auth` itself keeps `Principal`
and zero third-party dependencies, and `auth/token` takes the verifier and the
middleware. This closes the debt task "Separate `Principal` from token
verification": its deadline is "before the first M2 endpoint", and this step edits
exactly those files. Acceptance is `go list -deps ./internal/identity` with no
`MicahParks` and no `golang-jwt`.

The verifier reads `cadence_role`, and a missing claim yields an empty role rather
than a refusal at the token level. `internal/platform/database`:
`WithCaller` maps the claim to a role name through a closed map of constants and
refuses before `Begin` on an unknown role and on a non-UUID subject; claims are
published under the key `cadence_role`, and empty values are not published;
`WithService` on the second pool with claims cleared, the actor published, and
nesting forbidden in both orders; the query execution mode is pinned; `SET` without
`LOCAL` is issued nowhere. `internal/platform/config`: `DATABASE_SERVICE_URL`
becomes mandatory. `cmd/api`: the second pool and the startup check — each pool's
connection role, the absence of superuser and `rolbypassrls`, and the refusal of
`SET ROLE` between the paths.

Edits to existing tests, listed explicitly: in
`TestWithCallerIsNotAnInjectionVector` the eight hostile subjects currently **must
pass** — they move into the role field, which also ends up in the claims and has no
UUID shape, and cases refusing before `Begin` are added for the subjects;
`TestWithCallerPublishesTheClaims` reads the key `role` and moves to
`cadence_role`; `verifier_test.go` — the fixture gains `cadence_role`, the assertion
`Role == "authenticated"` is replaced, and a case is added for a token carrying only
the stock `role`. The role field description in `identity/handler.go` is rewritten,
and `make openapi` regenerates `openapi.json` in the same commit — otherwise the
gate is red.
todoist: "6h9HmVMRQXMfpw7q"

### step-3: The subject-reading function in the `app` schema

`app.jwt_subject()`: `STABLE`, `SET search_path = pg_catalog, pg_temp`, qualified
names, `nullif` before the cast, `NULL` instead of `22P02` on a value that is not
UUID-shaped, `REVOKE ALL … FROM PUBLIC` and `GRANT EXECUTE` to the four
impersonation roles. `app.jwt_role()` is not created: the role decision is made by
`TO`.

Tests: inside `WithCaller` it returns what was published; outside a seam — `NULL`
with no exception; inside `WithService` it is empty; an empty GUC does not fail the
query; `cadence_app` cannot call it. Plus a cheap test of high value: one prepared
statement on one connection under two subjects returns different rows — proof that
the plan is not reused across a change of claims.
todoist: "6h9HmVRx96j8xVgH"

### step-4: The schema migration: six tables, columns, constraints, indexes

The migration opens with `SET ROLE cadence_owner` (`data-layer` invariant 3) and
lays down six tables in foreign-key order with their columns, a `CHECK` on every
closed set and on `audit_log` attribution, `CHECK (patient_id <> provider_id)`,
`UNIQUE(patient_id, provider_id)`, the partial unique index on the primary
specialist, and the indexes supporting the policy subqueries. `ENABLE` and `FORCE
ROW LEVEL SECURITY` on each. There are no policies or grants yet — forced RLS
without a policy forbids everyone, and that is verified by a test as a safe
intermediate state.

Here the `pg_class` walks stop being vacuous. The stale comment in
`coexistence_integration_test.go` saying `profiles` will reference the `auth` schema
is amended: decision D3 cancels that.
todoist: "6h9HmVfP8c2r6VHH"

### step-5: Six policy shapes, explicit grants, and three registries

Policies with an explicit `TO` and explicit grants per role and per column,
following the two tables in the technical detail. The grants registry over
`has_table_privilege` and `has_column_privilege`; the policies registry over the
set, the verbs, and `TO`, requiring `polroles <> '{0}'`; the functions registry over
`pg_proc` — owner, `prosecdef`, the **value** of `proconfig`, ACL. A test for the
absence of the literals `'doctor'`, `'patient'`, and `'admin'` in `qual`/`with_check`.
A test for `42P17`: a deliberately created back-reference produces exactly that code
— otherwise "there is no recursion" is green even in a world where the policy was
forgotten.
todoist: "6h9HmVhxcMxwGfqH"

### step-6: The patient creation path through the service seam, with audit

A service function in `internal/identity`: `profiles`, `patient_profiles`,
`care_team_assignments`, `user_preferences`, and `audit_log` with the action
`patient.create` in one transaction through `WithService`, with `timezone` validated
against `pg_timezone_names` and the assigned specialist's role checked for
consistency. There is no HTTP endpoint. Service-path authorization lives in Go, and
that is written down rather than implied.

It comes before the regression suite deliberately: under forced RLS not even the
owner can create fixtures, so this path is the only way to obtain rows for the
policy tests. Tests: a rollback leaves neither rows nor audit; a repeated pair is
rejected by `UNIQUE`; an actor not matching the published one is rejected by policy;
an empty `actor_job` is rejected by a constraint; `UPDATE`, `DELETE`, and `TRUNCATE`
on `audit_log` are rejected under every impersonation role.
todoist: "6h9HmVvfMh3hmWRH"

### step-7: The regression suite of policy tests

Isolation is proven by behaviour on rows created through step 6's path. The patient:
sees their own, sees the profiles of their own specialists, does not see an
unassigned one, **cannot** change `role`, cannot insert or delete an assignment,
cannot write the clinical fields. The doctor: sees their own profile, sees assigned
patients, does not see an unassigned one, gains visibility when an assignment is
added and **loses it when one is deleted**. The admin: everything except writing to
`audit_log`. The service path: exactly what is listed. A transaction with no
published claims reads zero rows and writes nothing — the policies refuse on a
`NULL` subject, not the privileges. Somebody else's row comes back as an empty
result, not a refusal.

Separately — a test **asserting** the residual property: overwriting
`request.jwt.claims` inside one's own transaction yields subject substitution but not
role elevation, with a reference to the invariant.

This suite is the one every subsequent migration is obliged to extend (`data-layer`
invariant 5), which makes its shape more important than its current size.
todoist: "6h9HmVwrwgc8qcmq"

### step-8: The token issuance hook and the non-superuser GoTrue role

The hook function per decision D4: owner `cadence_owner`, `SECURITY DEFINER`, a
pinned `search_path`, `REVOKE … FROM PUBLIC`, `EXECUTE` and `USAGE ON SCHEMA app`
for the `cadence_auth_hook` role. The `cadence_gotrue` role in `docker-compose`
instead of the container's superuser, the `auth` schema created by a one-off step
already under its ownership, the membership granted by the `dev-up` step, the GoTrue
variables. Refusals are commented on the spot: the same ones will recur at
deployment.

Tests: `cadence_gotrue` has neither `USAGE` on `app` nor `SELECT` on `profiles` —
only `EXECUTE` on the hook; revoking membership breaks token issuance (verifiable
only once GoTrue has stopped being a superuser); an event with `cadence_role: admin`
substituted in does not contain it on the way out; a user with no profile receives a
token without the claim and issuance does not fail; the mandatory claims are not
discarded.

Verification with a real token end to end is done **by hand** against `make dev-up`
and recorded here as a manual step: there is no GoTrue container in the test
harness, it arrives with the invitation integration tests (build plan, IDN-17).
todoist: "6h9HmW56R74QfccH"

## Open questions

> [!question] The attributes of the role issued by Timeweb's managed Postgres
> (`rolcreaterole`, `CREATE` on the database) have not been verified — the cluster
> does not exist yet. The chain is written for `CREATEROLE` without a superuser, and
> the block deliberately adds nothing new to that requirement. Verifying it is a
> deployment task.

> [!question] The exact shape of the pg-function call made by GoTrue v2.194.0 — the
> event and return value shapes, behaviour inside a transaction and on timeout —
> lines up per the documentation, and the variable names and the `pg-functions://`
> format have been verified, but not against the sources. Clarified at step 8 on the
> live environment; nothing depends on it before the policies are written.

> [!question] `GENERATED ALWAYS AS IDENTITY` does still permit `OVERRIDING SYSTEM
> VALUE`, that is, the service path is capable of choosing `audit_log.id` and
> reordering history. Either closed by a policy or deliberately accepted — decided at
> step 4; a narrow question.

> [!question] A `UNIQUE` and partial-unique-index violation reports the existence of
> rows the caller cannot see. Harmless today: the product roles have no `INSERT` on
> `care_team_assignments`. Written down so it surfaces by the time of the first table
> with a unique key and a write from the patient.

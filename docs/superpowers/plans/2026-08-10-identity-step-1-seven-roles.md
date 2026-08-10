# Identity step-1: seven roles, no default privileges, the SQL authorship gate

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this
> plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the base migration's three-role arrangement with the seven roles the access model
needs, remove default privileges entirely, and add a gate that forbids a non-constant expression
from reaching `Exec`/`Query`/`QueryRow`/`SendBatch` in `api/internal/**`.

**Architecture:** `000001_base` is amended in place — it is the base migration and nothing is
deployed from it yet. Product roles become Postgres roles (`cadence_patient`, `cadence_doctor`,
`cadence_admin`), the service path gets its own `NOLOGIN` target (`cadence_service`) and its own
`LOGIN` connection role (`cadence_service_app`), and `cadence_authenticated` is abolished. Grants
stop arriving by `ALTER DEFAULT PRIVILEGES` and start being written per table, which lands in
step-4/step-5; this step only removes the mechanism and fixes the tests that leaned on it.

**Tech Stack:** PostgreSQL 17, golang-migrate, pgx v5, testcontainers-go, golangci-lint v2 (gosec
G201/G202), a dependency-free AST checker in `api/cmd/sqlauthorship`.

## Global Constraints

- Every migration that adds a table must extend the RLS policy test suite (project rule). This step
  adds no table.
- `gofumpt` formatting, errors wrapped with `%w`, `context.Context` first (global Go standards).
- The chain is applied by a `CREATEROLE`, non-superuser bootstrap role. Nothing may require
  superuser.
- Role names reaching `SET ROLE` are package constants, never token data.
- The seam must stay mandatory: connection roles are `NOINHERIT`, and every membership is granted
  `WITH INHERIT FALSE` on PostgreSQL 16+.

---

## File structure

| File | Responsibility after this step |
|---|---|
| `api/migrations/000001_base.up.sql` | seven roles, four memberships, convergence, schema, `USAGE` for the four impersonation roles. No default privileges. |
| `api/migrations/000001_base.down.sql` | removes eight role names (the seven plus the abolished one) and the schema, twice-safe. |
| `api/internal/platform/testsupport/postgres.go` | constants for the seven roles, `ServiceAppURL`, password provisioning for both LOGIN roles. `AuthenticatedRole` gone. |
| `api/internal/platform/database/caller.go` | `WithCaller` picks the Postgres role from a closed map keyed on `Caller.Role`. |
| `api/internal/platform/database/base_migration_integration_test.go` | role-arrangement tests: the `BYPASSRLS` probe, the membership registry, the service-path barrier, the alien-role probe. |
| `api/internal/platform/database/caller_integration_test.go` | seam tests against product roles, with explicit grants instead of default privileges. |
| `api/cmd/sqlauthorship/` | the authorship checker: walks `api/internal/**` and rejects a non-constant SQL argument. |
| `scripts/gate/go.sh` | runs the checker between `go vet` and `golangci-lint`. |
| `api/.golangci.yml` | `gosec` enabled, restricted to G201 and G202. |

## Role arrangement produced by this step

```
cadence_owner        NOLOGIN  owns schema app and every object in it
cadence_app          LOGIN NOINHERIT   DATABASE_URL, member of the three product roles
cadence_patient      NOLOGIN  impersonation target
cadence_doctor       NOLOGIN  impersonation target
cadence_admin        NOLOGIN  impersonation target
cadence_service_app  LOGIN NOINHERIT   DATABASE_SERVICE_URL, member of cadence_service only
cadence_service      NOLOGIN  impersonation target of the service path
```

Expected memberships, exhaustively — `(granted, member)`:

```
(cadence_patient, cadence_app)
(cadence_doctor,  cadence_app)
(cadence_admin,   cadence_app)
(cadence_service, cadence_service_app)
```

`(cadence_service, cadence_app)` is never granted, and none of the four carries `inherit_option`.

---

### Task 1: The role arrangement, proven by tests before it exists

**Files:**
- Modify: `api/internal/platform/testsupport/postgres.go`
- Modify: `api/internal/platform/database/base_migration_integration_test.go`
- Modify: `api/migrations/000001_base.up.sql`
- Modify: `api/migrations/000001_base.down.sql`

**Interfaces:**
- Produces: `testsupport.{OwnerRole,AppRole,PatientRole,DoctorRole,AdminRole,ServiceRole,ServiceAppRole}`,
  `testsupport.Database.ServiceAppURL`, `testsupport.ProductRoles []string`.
- Consumes: nothing from later tasks.

- [ ] **Step 1: Add the constants the new tests will name**

  In `testsupport/postgres.go`, drop `AuthenticatedRole` and add `PatientRole`, `DoctorRole`,
  `AdminRole`, `ServiceRole`, `ServiceAppRole` plus `ProductRoles` (patient, doctor, admin) and
  `ImpersonationRoles` (those three plus service). Add `serviceAppPass` and `ServiceAppURL`, and set
  the password on `cadence_service_app` the same way the harness already does for `cadence_app`.

- [ ] **Step 2: Write the failing tests**

  Four new tests in `base_migration_integration_test.go`:

  - `TestChainCannotCreateABypassRLSRole` — under `db.MigrationURL`,
    `CREATE ROLE cadence_bypass_probe NOBYPASSRLS` succeeds and
    `ALTER ROLE cadence_bypass_probe BYPASSRLS` fails; the assumption the whole block stands on is
    probed rather than quoted. Drop the probe role in `t.Cleanup`.
  - `TestServicePathIsUnreachableFromTheRequestPath` — as `cadence_app`,
    `SET ROLE cadence_service` fails with `42501`; as `cadence_service_app` it succeeds.
  - `TestMembershipsAreExactlyTheFourExpected` — read `pg_auth_members` joined to `pg_roles` twice,
    restricted to the seven names on both sides, and require exactly the four pairs above with
    `inherit_option` false on every one.
  - `TestAlienRolesHoldNothingInTheApplicationSchema` — create `service_role` `NOLOGIN` on the
    cluster (the local stack creates it for GoTrue's Supabase-era grants), apply the chain into a
    fresh database, and require `has_schema_privilege('service_role','app','USAGE')` to be false.

  Amend the existing tests in the same file: the three-element role lists in
  `TestBaseMigrationRollsBackCleanly`, `TestSingleStepRollbackRemovesTheBaseMigration` and
  `TestDownMigrationIsIdempotent` become the seven; `TestChainConvergesPreexistingRoles` and
  `TestChainRefusesAMembershipItCannotRevoke` loosen `cadence_patient` instead of
  `cadence_authenticated`; `TestRequestPathReachesTablesOnlyThroughImpersonation` grants on the
  probe table explicitly — with default privileges gone, the grant is the migration's job and the
  test may no longer rely on one arriving by itself.

- [ ] **Step 3: Run the tests and watch them fail**

  Run: `cd api && go test -tags=integration ./internal/platform/database/... -run 'TestChainCannotCreateABypassRLSRole|TestServicePathIsUnreachableFromTheRequestPath|TestMembershipsAreExactlyTheFourExpected|TestAlienRolesHoldNothingInTheApplicationSchema' -count=1`
  Expected: FAIL — `role "cadence_service" does not exist`.

- [ ] **Step 4: Rewrite the up migration**

  Seven guarded `CREATE ROLE`s; `ALTER ROLE` declarations for all seven; the attribute guard
  widened from three names to seven; convergence that revokes, `DROP OWNED BY`s and drops
  `cadence_authenticated` when a previous version of the chain left it behind; a `REVOKE` before
  every `GRANT … WITH INHERIT FALSE` for the four memberships plus a defensive
  `REVOKE cadence_service FROM cadence_app`; a single check that the membership set is exactly the
  four expected pairs and that none inherits, raising the same shape of exception the current chain
  raises; `GRANT CONNECT` for both LOGIN roles; `SET ROLE cadence_owner` around `CREATE SCHEMA`;
  `GRANT USAGE ON SCHEMA app` to the four impersonation roles only. Both `ALTER DEFAULT PRIVILEGES`
  statements are deleted.

- [ ] **Step 5: Rewrite the down migration**

  One `FOREACH` over the eight names for the `GRANT … TO CURRENT_USER` pass, the schema drop under
  the owner, then per role: revoke database-scoped grants, `DROP OWNED BY`, `DROP ROLE`. Twice-safe,
  as now.

- [ ] **Step 6: Run the database suite**

  Run: `cd api && go test -tags=integration ./internal/platform/database/... -count=1`
  Expected: PASS.

---

### Task 2: The seam impersonates the caller's product role

**Files:**
- Modify: `api/internal/platform/database/caller.go`
- Modify: `api/internal/platform/database/caller_integration_test.go`

**Interfaces:**
- Produces: `database.ErrUnknownRole`; `WithCaller` refusing before `Begin` on a role outside the map.
- Consumes: `testsupport.PatientRole` and friends from Task 1.

> **Deviations from the spec**, both recorded as `[!deviation]` callouts in the spec's step-1:
> the closed map is pulled forward from step-2, because step-1 abolishes `cadence_authenticated`
> and a seam whose only target has been deleted cannot stay green for the length of a step; and
> `SET LOCAL ROLE <name>` is issued as `SELECT set_config('role', $1, true)`, because the
> statement form admits no placeholder and the authorship gate this step introduces forbids a
> statement assembled from a variable.

- [ ] **Step 1: Write the failing tests**

  In `caller_integration_test.go`: `testCaller()` carries `Role: "patient"`;
  `TestWithCallerImpersonates` asserts `current_user == testsupport.PatientRole`; a new
  `TestWithCallerImpersonatesEachProductRole` walks patient/doctor/admin and asserts each lands on
  its own Postgres role; a new `TestWithCallerRefusesAnUnknownRole` passes `Role: "root"` and
  requires `errors.Is(err, database.ErrUnknownRole)` with the closure never running.

- [ ] **Step 2: Run them and watch them fail**

  Run: `cd api && go test -tags=integration ./internal/platform/database/... -run TestWithCaller -count=1`
  Expected: FAIL — impersonation still targets a role that no longer exists.

- [ ] **Step 3: Implement the closed map**

  Replace the `impersonatedRole` constant with `postgresRoleFor(role string) (string, bool)` over a
  package-level `map[string]string` of three entries, each value a package constant. Refuse before
  `pool.Begin` when the lookup misses.

- [ ] **Step 4: Fix `createAppTable`**

  With default privileges gone it must grant explicitly:
  `GRANT SELECT, INSERT, UPDATE, DELETE ON app.<name> TO cadence_patient, cadence_doctor, cadence_admin`.
  The helper's comment changes with it — the privileges are now the test's arrangement, and saying
  otherwise would be a false claim in a comment.

- [ ] **Step 5: Run the whole integration suite**

  Run: `cd api && make test-integration`
  Expected: PASS.

---

### Task 3: The SQL authorship gate

**Files:**
- Create: `api/cmd/sqlauthorship/main.go`
- Create: `api/cmd/sqlauthorship/check.go`
- Create: `api/cmd/sqlauthorship/check_test.go`
- Modify: `scripts/gate/go.sh`
- Modify: `api/.golangci.yml`

**Interfaces:**
- Produces: `Check(dirs []string) ([]Finding, error)` and `Finding{File, Line, Call, Expr}`.

- [ ] **Step 1: Write the failing unit tests**

  Table-driven over source snippets parsed from strings, asserting: a bare string literal is
  accepted; a package constant is accepted; `"SET LOCAL ROLE " + roleConst` is accepted;
  `"SET LOCAL ROLE " + role` where `role` is a parameter is rejected; `fmt.Sprintf(...)` is
  rejected; a local variable holding a literal is rejected (the rule is syntactic on purpose — a
  variable is where the next edit puts input); `.Exec`, `.Query`, `.QueryRow` and `.SendBatch` are
  all covered; a call on a non-SQL receiver method of another name is ignored.

- [ ] **Step 2: Run and watch them fail**

  Run: `cd api && go test ./cmd/sqlauthorship/... -count=1`
  Expected: FAIL — package does not compile.

- [ ] **Step 3: Implement the checker**

  `go/parser` over each package directory, collect `const` identifiers declared in the package,
  then walk for `*ast.CallExpr` whose `Fun` is a `*ast.SelectorExpr` with a name in
  {`Exec`,`Query`,`QueryRow`,`SendBatch`}. The SQL argument is the first `string`-shaped argument
  after an optional `ctx`: accept `*ast.BasicLit` of kind `STRING`, an `*ast.Ident` in the const
  set, an `*ast.SelectorExpr` whose selector is in the const set, and `*ast.BinaryExpr` with `+`
  whose operands are all accepted. Everything else is a finding.

- [ ] **Step 4: Run the unit tests, then the checker against the tree**

  Run: `cd api && go test ./cmd/sqlauthorship/... -count=1 && go run ./cmd/sqlauthorship ./internal`
  Expected: unit tests PASS; the tree scan reports nothing. `caller.go`'s
  `"SET LOCAL ROLE " + impersonatedRole` shape is the one real case in the tree and must be
  accepted.

- [ ] **Step 5: Wire the gate**

  In `scripts/gate/go.sh`, add an `==> sql authorship` stage after `go vet`. In `.golangci.yml`,
  enable `gosec` with `includes: [G201, G202]` as the backstop the spec asks for.

- [ ] **Step 6: Run the whole gate**

  Run: `make -C api gate && make -C api test-integration`
  Expected: both green.

---

## Self-review

- Spec coverage: seven roles ✓, default privileges removed ✓, `USAGE` for impersonation roles ✓,
  attribute guard widened ✓, `inherit_option` counterpart per membership ✓, `BYPASSRLS` probe ✓,
  `SET ROLE cadence_service` barrier ✓, membership registry ✓, alien `service_role` probe ✓, named
  existing-test amendments ✓, authorship gate with gosec backstop ✓.
- Not in this step, by the spec's own decomposition: the `auth`/`auth/token` split, the second pool,
  `WithService`, `DATABASE_SERVICE_URL` validation, the claim rename to `cadence_role` (step-2); the
  subject function (step-3); tables, policies and grants (step-4, step-5).
- Types used in Task 2 (`ErrUnknownRole`, `postgresRoleFor`) are defined in Task 2; constants used
  by Tasks 1 and 2 are defined in Task 1 Step 1.

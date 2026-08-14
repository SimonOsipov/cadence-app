//go:build integration

package database_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// asRole opens a connection and impersonates one of the roles a transaction can
// run as, without going through a seam. It exists for the cases that are about
// the function rather than about the seam: what it answers when nobody
// published anything.
func asRole(t *testing.T, db *testsupport.Database, role string) *pgx.Conn {
	t.Helper()

	conn := testsupport.Connect(t, db.AppURL)
	if _, err := conn.Exec(t.Context(), "SET ROLE "+role); err != nil {
		t.Fatalf("impersonating %s: %v", role, err)
	}

	return conn
}

func TestJWTSubjectReturnsWhatTheSeamPublished(t *testing.T) {
	pool, _ := requestPool(t)

	var subject string
	if err := database.WithCaller(
		t.Context(), pool, testCaller(),
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, "SELECT app.jwt_subject()::text").Scan(&subject)
		},
	); err != nil {
		t.Fatalf("WithCaller: %v", err)
	}

	if subject != testCaller().Subject {
		t.Errorf("app.jwt_subject() = %q, want %q", subject, testCaller().Subject)
	}
}

// Outside a seam there is no caller, and the answer is NULL rather than an
// exception. A function that raised here would turn every policy on a table
// read outside a transaction — a migration, a maintenance query — into an error
// nobody expected, and a policy that raises is a 500 where "no rows" belongs.
func TestJWTSubjectIsNullWithNoClaims(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := asRole(t, db, testsupport.PatientRole)

	var subject *string
	if err := conn.QueryRow(t.Context(), "SELECT app.jwt_subject()::text").Scan(&subject); err != nil {
		t.Fatalf("calling app.jwt_subject() with no claims: %v", err)
	}

	if subject != nil {
		t.Errorf("app.jwt_subject() = %q with nothing published, want NULL", *subject)
	}
}

// The empty string is the shape the service seam leaves behind: set_config has
// no way to unset, so clearing means setting to ”. Without the nullif this is
// where the function would raise 22P02 — inside every policy, in every service
// transaction.
func TestJWTSubjectSurvivesAnEmptyClaimsSetting(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()
	conn := asRole(t, db, testsupport.PatientRole)

	if _, err := conn.Exec(ctx, `SELECT set_config('request.jwt.claims', '', false)`); err != nil {
		t.Fatalf("clearing the claims: %v", err)
	}

	var subject *string
	if err := conn.QueryRow(ctx, "SELECT app.jwt_subject()::text").Scan(&subject); err != nil {
		t.Fatalf("calling app.jwt_subject() with empty claims: %v", err)
	}

	if subject != nil {
		t.Errorf("app.jwt_subject() = %q with empty claims, want NULL", *subject)
	}
}

// A claim that is not UUID-shaped answers NULL rather than raising. The seam
// already refuses such a subject before opening a transaction, so this is the
// second of two guards — and the one that holds when a caller overwrites the
// claims inside their own transaction, which the setting's USERSET context
// permits and nothing can forbid.
func TestJWTSubjectIsNullForAClaimThatIsNotAUUID(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()
	conn := asRole(t, db, testsupport.PatientRole)

	for _, claim := range []string{
		`{"sub":"not-a-uuid"}`,
		`{"sub":""}`,
		`{"sub":"8a1f3b7c$0000$4000$8000$000000000001"}`,
		`{"cadence_role":"patient"}`,
		// Not JSON at all, and not hypothetical: the setting has USERSET
		// context, so a caller can put this there inside their own transaction.
		// Without the IS JSON guard each of these raises 22P02 — from inside
		// every policy, once the policies exist.
		`garbage`,
		`{"sub":`,
		`{"sub":"8a1f3b7c-0000-4000-8000-000000000001"`,
		// Valid JSON that is not an object. These never raised, and they are
		// here so the guard is not mistaken for the reason they answer NULL.
		`[1,2]`,
		`"a string"`,
		`null`,
	} {
		if _, err := conn.Exec(
			ctx, `SELECT set_config('request.jwt.claims', $1, false)`, claim,
		); err != nil {
			t.Fatalf("publishing %s: %v", claim, err)
		}

		var subject *string
		if err := conn.QueryRow(ctx, "SELECT app.jwt_subject()::text").Scan(&subject); err != nil {
			t.Errorf("claims %s raised instead of answering NULL: %v", claim, err)

			continue
		}
		if subject != nil {
			t.Errorf("claims %s gave %q, want NULL", claim, *subject)
		}
	}
}

// Inside the service seam there is no caller by construction, and the function
// says so. A service transaction that could name a subject would let a policy
// written for the request path match rows on it.
//
// The connection is dirtied first, on a pool of one, because on a clean one the
// answer is NULL whatever the seam does — a fresh transaction inherits nothing,
// and the assertion would hold with the seam's clearing deleted entirely.
func TestJWTSubjectIsNullInsideTheServiceSeam(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	pool, err := database.NewPool(ctx, db.ServiceAppURL+"&pool_max_conns=1")
	if err != nil {
		t.Fatalf("opening a single-connection service pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(
		ctx, `SELECT set_config('request.jwt.claims', $1, false)`,
		`{"sub":"8a1f3b7c-0000-4000-8000-000000000009","cadence_role":"admin"}`,
	); err != nil {
		t.Fatalf("dirtying the connection: %v", err)
	}

	// The control: the claims are on the connection, so a NULL below is the
	// seam clearing them rather than their never having been there.
	var planted *string
	if err := pool.QueryRow(
		ctx, `SELECT nullif(current_setting('request.jwt.claims', true), '')`,
	).Scan(&planted); err != nil {
		t.Fatalf("reading the planted claims: %v", err)
	}
	if planted == nil {
		t.Fatal("the claims were not planted, so this test proves nothing")
	}

	var subject *string
	if err := database.WithServiceJob(
		ctx, pool, testProbe,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, "SELECT app.jwt_subject()::text").Scan(&subject)
		},
	); err != nil {
		t.Fatalf("WithServiceJob: %v", err)
	}

	if subject != nil {
		t.Errorf("app.jwt_subject() = %q inside the service seam, want NULL — a caller's "+
			"identity outlived its transaction and was served to a system job", *subject)
	}
}

// The connection roles hold nothing on this schema, and the function is not an
// exception carved out of that.
//
// What refuses them is the schema rather than the function's own ACL — they
// hold no USAGE on app — so this does not witness the REVOKE. The test below,
// over a role that does hold the schema, is the one that does; deleting either
// as a duplicate of the other loses a property.
func TestTheConnectionRolesCannotCallJWTSubject(t *testing.T) {
	db := cluster.NewDatabase(t)

	for role, dsn := range map[string]string{
		testsupport.AppRole:        db.AppURL,
		testsupport.ServiceAppRole: db.ServiceAppURL,
	} {
		t.Run(role, func(t *testing.T) {
			conn := testsupport.Connect(t, dsn)

			var subject *string
			err := conn.QueryRow(t.Context(), "SELECT app.jwt_subject()::text").Scan(&subject)
			if err == nil {
				t.Fatalf("%s called app.jwt_subject() without impersonating", role)
			}

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
				t.Fatalf("the call failed with %v, want SQLSTATE 42501", err)
			}
		})
	}
}

// The cheap test of high value. STABLE rather than IMMUTABLE is what keeps the
// planner from folding this call into a constant at plan time — and pgx caches
// prepared statements per connection, so a folded value would be served to the
// next caller on the same pooled connection.
//
// One connection, one statement text, two subjects. If the plan were reused
// with the first subject baked in, the second answer would be the first
// caller's identity: one patient reading as another, with nothing in any log to
// say so.
func TestJWTSubjectIsNotFoldedIntoACachedPlan(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	// A single connection, so the second transaction is guaranteed to meet the
	// statement the first one prepared. With a larger pool the test would pass
	// on two different connections and prove nothing.
	//
	// Opened through database.NewPool rather than hand-configured, because the
	// premise of this test is pgx's per-connection statement cache, and the
	// execution mode that provides it is pinned there. A pool built beside that
	// pinning would agree with production by coincidence — which is the exact
	// coincidence the pinning was written down to stop relying on.
	pool, err := database.NewPool(ctx, db.AppURL+"&pool_max_conns=1")
	if err != nil {
		t.Fatalf("opening a single-connection pool: %v", err)
	}
	t.Cleanup(pool.Close)

	read := func(subject string) string {
		t.Helper()

		var got string
		if err := database.WithCaller(
			ctx, pool, database.Caller{Subject: subject, Role: "patient"},
			func(ctx context.Context, tx pgx.Tx) error {
				// The same statement text both times, on purpose: that is what
				// makes pgx reuse the cached prepared statement.
				return tx.QueryRow(ctx, "SELECT app.jwt_subject()::text").Scan(&got)
			},
		); err != nil {
			t.Fatalf("WithCaller for %s: %v", subject, err)
		}

		return got
	}

	first := "8a1f3b7c-0000-4000-8000-000000000001"
	second := "8a1f3b7c-0000-4000-8000-000000000002"

	if got := read(first); got != first {
		t.Fatalf("first call gave %q, want %q", got, first)
	}
	if got := read(second); got != second {
		t.Errorf("second call on the same connection gave %q, want %q — the value was folded "+
			"into a cached plan, so one caller reads as another", got, second)
	}
}

// The REVOKE needs a witness of its own. Both connection roles are refused by
// the schema — they hold no USAGE on app — so the test above stays green with
// the revoke deleted and measures the schema grant rather than the function's.
//
// This gives a role the schema and nothing else, which is the state PUBLIC would
// be in for any role somebody adds later: CREATE FUNCTION hands EXECUTE to
// PUBLIC, and without the revoke the explicit grant to the four impersonation
// roles would be a list nobody was bound by.
func TestAnUnnamedRoleWithSchemaAccessStillCannotCallJWTSubject(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	// Opened before the cleanup that uses it: t.Context is already cancelled by
	// the time cleanups run, so a connection dialled in there never gets made.
	admin := testsupport.Connect(t, db.SuperuserURL)

	// Created by the bootstrap role: cadence_owner is NOCREATEROLE, and owning
	// the schema is a different thing from being able to invent roles.
	owner := testsupport.Connect(t, db.MigrationURL)
	if _, err := owner.Exec(ctx, `CREATE ROLE cadence_bystander NOLOGIN`); err != nil {
		t.Fatalf("creating the bystander: %v", err)
	}
	registerBystanderCleanup(t, admin, ctx)

	if _, err := owner.Exec(ctx, `SET ROLE `+testsupport.OwnerRole); err != nil {
		t.Fatalf("becoming the owner: %v", err)
	}
	if _, err := owner.Exec(ctx, `GRANT USAGE ON SCHEMA app TO cadence_bystander`); err != nil {
		t.Fatalf("granting the schema to the bystander: %v", err)
	}

	// The control: the schema really is reachable now, so a refusal below is the
	// function's ACL rather than the schema's.
	var reachable bool
	if err := owner.QueryRow(
		ctx, `SELECT has_schema_privilege('cadence_bystander', 'app', 'USAGE')`,
	).Scan(&reachable); err != nil {
		t.Fatalf("reading the bystander's schema privilege: %v", err)
	}
	if !reachable {
		t.Fatal("the bystander cannot reach the schema, so this test proves nothing")
	}

	var permitted bool
	if err := owner.QueryRow(
		ctx, `SELECT has_function_privilege('cadence_bystander', 'app.jwt_subject()', 'EXECUTE')`,
	).Scan(&permitted); err != nil {
		t.Fatalf("reading the bystander's function privilege: %v", err)
	}
	if permitted {
		t.Error("a role nobody granted can execute app.jwt_subject() — CREATE FUNCTION's grant " +
			"to PUBLIC was never revoked")
	}
}

// registerBystanderCleanup is registered the moment the role exists, not after
// the statements that follow it. Roles are cluster objects: a t.Fatalf in
// between would leave this one standing for every later test in the binary.
func registerBystanderCleanup(t *testing.T, admin *pgx.Conn, ctx context.Context) {
	t.Helper()

	t.Cleanup(func() {
		cleanupCtx := context.WithoutCancel(ctx)
		for _, statement := range []string{
			`REVOKE USAGE ON SCHEMA app FROM cadence_bystander`,
			`DROP ROLE IF EXISTS cadence_bystander`,
		} {
			if _, err := admin.Exec(cleanupCtx, statement); err != nil {
				t.Errorf("cleaning up the bystander (%s): %v", statement, err)
			}
		}
	})
}

// The grant names four roles and two of them were witnessed by the tests above:
// a patient through the request seam, the service role through the service one.
// Striking cadence_doctor or cadence_admin from the list left every test green —
// and a doctor's request would then meet 42501 from inside a policy.
//
// The set, not its size: every impersonation target, and neither connection role.
func TestEveryImpersonationTargetCanCallJWTSubject(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	conn := testsupport.Connect(t, db.SuperuserURL)

	expected := map[string]bool{}
	for _, role := range testsupport.ImpersonationRoles() {
		expected[role] = true
	}
	for _, role := range []string{testsupport.AppRole, testsupport.ServiceAppRole} {
		expected[role] = false
	}

	for role, want := range expected {
		var permitted bool
		if err := conn.QueryRow(
			ctx,
			`SELECT has_function_privilege($1, 'app.jwt_subject()', 'EXECUTE')`, role,
		).Scan(&permitted); err != nil {
			t.Fatalf("reading %s's privilege on the function: %v", role, err)
		}

		if permitted != want {
			t.Errorf("has_function_privilege(%s, app.jwt_subject(), EXECUTE) = %v, want %v",
				role, permitted, want)
		}
	}

	// Behaviour as well as catalogue, for the two the tests above never call.
	for _, role := range []string{testsupport.DoctorRole, testsupport.AdminRole} {
		roleConn := asRole(t, db, role)

		var subject *string
		if err := roleConn.QueryRow(ctx, "SELECT app.jwt_subject()::text").Scan(&subject); err != nil {
			t.Errorf("%s cannot call app.jwt_subject(): %v", role, err)
		}
	}
}

// The function belongs to cadence_owner, like everything else in the schema. The
// pg_class sweep that catches a table created without SET ROLE does not look at
// pg_proc, so without this a migration that forgot the SET ROLE would leave the
// function owned by whoever applied the chain — a role that can then replace its
// body, and will be able to make it SECURITY DEFINER when step-8 needs one.
//
// The pinned search_path is asserted here too. It is not observable in the
// function's behaviour while every name in the body is qualified, so the
// catalogue is the only witness there is.
func TestJWTSubjectIsOwnedByTheOwnerAndPinsItsSearchPath(t *testing.T) {
	db := cluster.NewDatabase(t)

	conn := testsupport.Connect(t, db.SuperuserURL)

	var owner string
	var config []string
	var securityDefiner bool
	if err := conn.QueryRow(t.Context(), `
		SELECT pg_get_userbyid(p.proowner), p.proconfig, p.prosecdef
		FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = $1 AND p.proname = 'jwt_subject'
	`, testsupport.AppSchema).Scan(&owner, &config, &securityDefiner); err != nil {
		t.Fatalf("reading the function from the catalogue: %v", err)
	}

	if owner != testsupport.OwnerRole {
		t.Errorf("app.jwt_subject() is owned by %q, want %q", owner, testsupport.OwnerRole)
	}
	if securityDefiner {
		t.Error("app.jwt_subject() is SECURITY DEFINER; it reads a setting and needs no rights")
	}
	if want := "search_path=pg_catalog, pg_temp"; !slices.Contains(config, want) {
		t.Errorf("proconfig = %v, want it to contain %q", config, want)
	}
}

//go:build integration

package database_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

var cluster *testsupport.Cluster

func TestMain(m *testing.M) {
	ctx := context.Background()

	c, err := testsupport.StartCluster(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting the test cluster: %v\n", err)
		os.Exit(1)
	}
	cluster = c

	code := m.Run()

	if err := cluster.Terminate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "terminating the test cluster: %v\n", err)
	}

	os.Exit(code)
}

// The role under test is the one a deployment actually connects with, read back
// from the connection rather than looked up by name. The first draft of this
// spec asserted the attributes of cadence_authenticated, which is NOLOGIN and
// therefore nobody's connection: the assertion passed while the request path
// ran as the database owner.
func TestRequestPathRoleIsLowPrivilege(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.AppURL)

	var (
		name        string
		super       bool
		bypassRLS   bool
		createRole  bool
		createDB    bool
		inheritance bool
	)
	err := conn.QueryRow(t.Context(), `
		SELECT rolname, rolsuper, rolbypassrls, rolcreaterole, rolcreatedb, rolinherit
		FROM pg_roles
		WHERE rolname = CURRENT_USER
	`).Scan(&name, &super, &bypassRLS, &createRole, &createDB, &inheritance)
	if err != nil {
		t.Fatalf("reading the current role: %v", err)
	}

	if name != testsupport.AppRole {
		t.Errorf("CURRENT_USER = %q, want %q", name, testsupport.AppRole)
	}
	if super {
		t.Error("the request path role is a superuser")
	}
	if bypassRLS {
		t.Error("the request path role can bypass RLS, which makes every policy advisory")
	}
	if createRole {
		t.Error("the request path role can create roles")
	}
	if createDB {
		t.Error("the request path role can create databases")
	}
	// NOINHERIT is what makes the impersonation seam load-bearing: membership in
	// cadence_authenticated is usable only through an explicit SET ROLE, so a
	// query that skips it has no table privileges at all.
	if inheritance {
		t.Error("the request path role inherits its granted roles, so skipping SET ROLE goes unnoticed")
	}
}

func TestRequestPathRoleOwnsNothing(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)

	// Tables, views, sequences, indexes and the schema itself — anything the
	// request path could own is something it could also alter, and altering a
	// table includes turning row level security off on it.
	var owned []string
	rows, err := conn.Query(t.Context(), `
		SELECT n.nspname || '.' || c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND pg_get_userbyid(c.relowner) = $2
		UNION ALL
		SELECT 'schema ' || nspname
		FROM pg_namespace
		WHERE nspname = $1 AND pg_get_userbyid(nspowner) = $2
	`, testsupport.AppSchema, testsupport.AppRole)
	if err != nil {
		t.Fatalf("listing objects owned by the request path role: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		owned = append(owned, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating: %v", err)
	}

	if len(owned) > 0 {
		t.Errorf("the request path role owns %v; ownership belongs to %s alone", owned, testsupport.OwnerRole)
	}
}

func TestApplicationSchemaIsOwnedByTheOwnerRole(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)

	var owner string
	err := conn.QueryRow(t.Context(), `
		SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname = $1
	`, testsupport.AppSchema).Scan(&owner)
	if err != nil {
		t.Fatalf("reading the owner of schema %s: %v", testsupport.AppSchema, err)
	}

	if owner != testsupport.OwnerRole {
		t.Errorf("schema %s is owned by %q, want %q", testsupport.AppSchema, owner, testsupport.OwnerRole)
	}
}

// The chain has to survive without superuser, because it will never have one:
// on Supabase the role that applies migrations is `postgres`, which has
// CREATEROLE and CREATEDB and is not a superuser. Asserted rather than assumed
// so that a future change to the harness cannot quietly hand the chain rights
// the deployment does not have.
func TestChainAppliesWithoutSuperuser(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.MigrationURL)

	var super bool
	if err := conn.QueryRow(
		t.Context(),
		`SELECT rolsuper FROM pg_roles WHERE rolname = CURRENT_USER`,
	).Scan(&super); err != nil {
		t.Fatalf("reading the migration role: %v", err)
	}

	if super {
		t.Fatal("the chain was applied by a superuser, so it proves nothing about Supabase")
	}
}

// The arrangement of the three roles is only worth anything if it actually
// governs access to a table. There are no tables yet, so this test makes one:
// without SET ROLE the request path holds no privileges at all, and with it the
// privileges arrive from the default privileges declared FOR ROLE cadence_owner.
//
// This is the test that fails if FOR ROLE is dropped from the migration.
func TestRequestPathReachesTablesOnlyThroughImpersonation(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	owner := testsupport.Connect(t, db.MigrationURL)
	if _, err := owner.Exec(ctx, `SET ROLE cadence_owner`); err != nil {
		t.Fatalf("becoming the owner: %v", err)
	}
	if _, err := owner.Exec(ctx, `CREATE TABLE app.probe (id integer PRIMARY KEY)`); err != nil {
		t.Fatalf("creating the probe table: %v", err)
	}

	app := testsupport.Connect(t, db.AppURL)

	var id int
	err := app.QueryRow(ctx, `SELECT id FROM app.probe`).Scan(&id)
	if err == nil {
		t.Fatal("the request path read a table without impersonating; membership is being inherited")
	}

	// Any error would satisfy "it failed" — a dropped connection, a typo in the
	// table name, a schema that moved. Only 42501 says the database refused on
	// privileges, which is the single thing this test claims to prove.
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("reading without impersonating failed with %v, want SQLSTATE 42501 (insufficient_privilege)", err)
	}

	if _, err := app.Exec(ctx, `SET ROLE cadence_authenticated`); err != nil {
		t.Fatalf("impersonating: %v", err)
	}

	if _, err := app.Exec(ctx, `SELECT id FROM app.probe`); err != nil {
		t.Fatalf("reading while impersonating: %v", err)
	}
}

// tablesMissingForcedRLS is the invariant itself, kept in one place so that the
// test asserting it holds and the test proving it can fail run the same query.
func tablesMissingForcedRLS(t *testing.T, dsn string) []string {
	t.Helper()

	conn := testsupport.Connect(t, dsn)
	rows, err := conn.Query(t.Context(), `
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1
		  AND c.relkind IN ('r', 'p')
		  AND NOT (c.relrowsecurity AND c.relforcerowsecurity)
		ORDER BY c.relname
	`, testsupport.AppSchema)
	if err != nil {
		t.Fatalf("sweeping pg_class: %v", err)
	}
	defer rows.Close()

	var offenders []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		offenders = append(offenders, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating: %v", err)
	}

	return offenders
}

// Vacuous today — there are no tables. It becomes a gate the moment M2 adds the
// first one, which is the point: the rule arrives before the table, not after.
func TestForcedRowLevelSecurityInvariant(t *testing.T) {
	db := cluster.NewDatabase(t)

	if offenders := tablesMissingForcedRLS(t, db.SuperuserURL); len(offenders) > 0 {
		t.Errorf("tables without forced row level security: %v", offenders)
	}
}

// An invariant that has nothing to check yet is indistinguishable from one that
// cannot check anything. This runs the same sweep against a table that violates
// it and requires the violation to be reported.
func TestForcedRowLevelSecurityInvariantCatchesAnUnprotectedTable(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	owner := testsupport.Connect(t, db.MigrationURL)
	if _, err := owner.Exec(ctx, `SET ROLE cadence_owner`); err != nil {
		t.Fatalf("becoming the owner: %v", err)
	}
	if _, err := owner.Exec(ctx, `CREATE TABLE app.unprotected (id integer PRIMARY KEY)`); err != nil {
		t.Fatalf("creating the unprotected table: %v", err)
	}

	if offenders := tablesMissingForcedRLS(t, db.SuperuserURL); len(offenders) != 1 || offenders[0] != "unprotected" {
		t.Errorf("sweep reported %v, want exactly [unprotected]", offenders)
	}

	// Enabling row level security without FORCE leaves the owner exempt, which
	// is the failure the invariant exists to catch.
	if _, err := owner.Exec(ctx, `ALTER TABLE app.unprotected ENABLE ROW LEVEL SECURITY`); err != nil {
		t.Fatalf("enabling row level security: %v", err)
	}

	if offenders := tablesMissingForcedRLS(t, db.SuperuserURL); len(offenders) != 1 {
		t.Errorf("sweep reported %v after ENABLE without FORCE, want the table still reported", offenders)
	}

	if _, err := owner.Exec(ctx, `ALTER TABLE app.unprotected FORCE ROW LEVEL SECURITY`); err != nil {
		t.Fatalf("forcing row level security: %v", err)
	}

	if offenders := tablesMissingForcedRLS(t, db.SuperuserURL); len(offenders) != 0 {
		t.Errorf("sweep reported %v after FORCE, want none", offenders)
	}
}

// tablesNotOwnedByOwnerRole is the companion invariant: every later migration
// has to open with SET ROLE cadence_owner, and this is what notices when one
// does not.
func tablesNotOwnedByOwnerRole(t *testing.T, dsn string) []string {
	t.Helper()

	conn := testsupport.Connect(t, dsn)
	rows, err := conn.Query(t.Context(), `
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1
		  AND c.relkind IN ('r', 'p')
		  AND pg_get_userbyid(c.relowner) <> $2
		ORDER BY c.relname
	`, testsupport.AppSchema, testsupport.OwnerRole)
	if err != nil {
		t.Fatalf("sweeping pg_class for ownership: %v", err)
	}
	defer rows.Close()

	var offenders []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		offenders = append(offenders, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating: %v", err)
	}

	return offenders
}

func TestEveryApplicationTableIsOwnedByTheOwnerRole(t *testing.T) {
	db := cluster.NewDatabase(t)

	if offenders := tablesNotOwnedByOwnerRole(t, db.SuperuserURL); len(offenders) > 0 {
		t.Errorf("tables not owned by %s: %v", testsupport.OwnerRole, offenders)
	}
}

func TestOwnershipInvariantCatchesATableCreatedWithoutSetRole(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	// Exactly what a migration that forgets to open with SET ROLE produces: a
	// table in the application schema owned by whoever applied the chain.
	bootstrap := testsupport.Connect(t, db.MigrationURL)
	if _, err := bootstrap.Exec(ctx, `CREATE TABLE app.stray (id integer PRIMARY KEY)`); err != nil {
		t.Fatalf("creating the stray table: %v", err)
	}

	if offenders := tablesNotOwnedByOwnerRole(t, db.SuperuserURL); len(offenders) != 1 || offenders[0] != "stray" {
		t.Errorf("sweep reported %v, want exactly [stray]", offenders)
	}
}

// A base migration that cannot be rolled back is a base migration nobody dares
// re-apply. The roles are cluster objects and the grants that depend on them
// are not, so the order of the down migration is the whole difficulty: a
// surviving grant makes DROP ROLE fail, and a surviving role makes the next
// apply a no-op that looks like success.
func TestBaseMigrationRollsBackCleanly(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	if err := database.MigrateDown(db.MigrationURL, testsupport.MigrationsPath(t), 0); err != nil {
		t.Fatalf("rolling the chain back: %v", err)
	}

	admin := testsupport.Connect(t, db.SuperuserURL)

	var schemas int
	if err := admin.QueryRow(
		ctx,
		`SELECT count(*) FROM pg_namespace WHERE nspname = $1`, testsupport.AppSchema,
	).Scan(&schemas); err != nil {
		t.Fatalf("counting the application schema: %v", err)
	}
	if schemas != 0 {
		t.Errorf("schema %s survived the rollback", testsupport.AppSchema)
	}

	for _, role := range []string{testsupport.AppRole, testsupport.OwnerRole, testsupport.AuthenticatedRole} {
		var found int
		if err := admin.QueryRow(
			ctx,
			`SELECT count(*) FROM pg_roles WHERE rolname = $1`, role,
		).Scan(&found); err != nil {
			t.Fatalf("counting role %s: %v", role, err)
		}
		if found != 0 {
			t.Errorf("role %s survived the rollback", role)
		}
	}
}

// The Makefile's migrate-down target passes no argument, which resolves to one
// step — a different code path from the whole-chain rollback above, and the one
// an operator actually runs.
func TestSingleStepRollbackRemovesTheBaseMigration(t *testing.T) {
	db := cluster.NewDatabase(t)

	if err := database.MigrateDown(db.MigrationURL, testsupport.MigrationsPath(t), 1); err != nil {
		t.Fatalf("rolling back one step: %v", err)
	}

	admin := testsupport.Connect(t, db.SuperuserURL)

	var roles int
	if err := admin.QueryRow(
		t.Context(),
		`SELECT count(*) FROM pg_roles WHERE rolname IN ($1, $2, $3)`,
		testsupport.AppRole, testsupport.OwnerRole, testsupport.AuthenticatedRole,
	).Scan(&roles); err != nil {
		t.Fatalf("counting roles: %v", err)
	}
	if roles != 0 {
		t.Errorf("%d of the three roles survived a single-step rollback", roles)
	}
}

// Rolling back twice is something a person does by accident, and it has to be
// dull rather than dangerous.
//
// Going through MigrateDown twice would prove nothing: after the first pass the
// version is nil and golang-migrate returns before it opens the file, so the
// guards in the SQL are never reached. This runs the down migration itself
// twice, which is what the file claims to survive.
func TestDownMigrationIsIdempotent(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	statements, err := os.ReadFile(filepath.Join(testsupport.MigrationsPath(t), "000001_base.down.sql"))
	if err != nil {
		t.Fatalf("reading the down migration: %v", err)
	}

	conn := testsupport.Connect(t, db.MigrationURL)
	for pass := 1; pass <= 2; pass++ {
		if _, err := conn.Exec(ctx, string(statements)); err != nil {
			t.Fatalf("applying the down migration, pass %d: %v", pass, err)
		}
	}

	admin := testsupport.Connect(t, db.SuperuserURL)

	var roles int
	if err := admin.QueryRow(
		ctx,
		`SELECT count(*) FROM pg_roles WHERE rolname IN ($1, $2, $3)`,
		testsupport.AppRole, testsupport.OwnerRole, testsupport.AuthenticatedRole,
	).Scan(&roles); err != nil {
		t.Fatalf("counting roles: %v", err)
	}
	if roles != 0 {
		t.Errorf("%d of the three roles survived two passes of the down migration", roles)
	}
}

// Applying the chain onto a cluster where the roles already exist has to
// converge them, not accept whatever is there. A cadence_app left with INHERIT
// would take the grant and turn impersonation into an option — the seam would
// still be called, and skipping it would stop being visible.
func TestChainConvergesPreexistingRoles(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	admin := testsupport.Connect(t, db.SuperuserURL)
	if _, err := admin.Exec(ctx, `ALTER ROLE cadence_app INHERIT`); err != nil {
		t.Fatalf("loosening the request path role: %v", err)
	}

	// Loosened as a membership too, and issued by the bootstrap role so that the
	// chain's REVOKE can reach it. Loosening only the role attribute would not
	// discriminate: on PostgreSQL 16 an existing membership keeps its own
	// inherit option regardless, so the test would pass with the ALTER removed
	// from the chain — and the ALTER is the only defence on the pre-16 branch.
	bootstrap := testsupport.Connect(t, db.MigrationURL)
	if _, err := bootstrap.Exec(ctx, `GRANT cadence_authenticated TO cadence_app WITH INHERIT TRUE`); err != nil {
		t.Fatalf("loosening the membership: %v", err)
	}

	// A second database in the same cluster finds the roles already there —
	// exactly the state a redeploy against Supabase starts from.
	second := cluster.NewDatabase(t)

	owner := testsupport.Connect(t, second.MigrationURL)
	if _, err := owner.Exec(ctx, `SET ROLE cadence_owner`); err != nil {
		t.Fatalf("becoming the owner: %v", err)
	}
	if _, err := owner.Exec(ctx, `CREATE TABLE app.probe (id integer PRIMARY KEY)`); err != nil {
		t.Fatalf("creating the probe table: %v", err)
	}

	// Behaviour rather than the attribute: the question is whether skipping the
	// seam is still refused, and the attribute and the grant decide that jointly.
	app := testsupport.Connect(t, second.AppURL)

	var id int
	err := app.QueryRow(ctx, `SELECT id FROM app.probe`).Scan(&id)

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("re-applying the chain onto loosened roles left the request path able to read without "+
			"impersonating (error was %v); the chain creates attributes once instead of declaring them", err)
	}

	// The attribute gets its own witness. Behaviour on PostgreSQL 16 is decided
	// by the membership alone, so without this assertion the ALTER could be
	// dropped and everything would stay green — until a deployment on 15, where
	// the attribute is the only thing standing between a query and the
	// privileges it never asked for.
	var inheritance bool
	if err := admin.QueryRow(
		ctx,
		`SELECT rolinherit FROM pg_roles WHERE rolname = $1`, testsupport.AppRole,
	).Scan(&inheritance); err != nil {
		t.Fatalf("reading the request path role: %v", err)
	}
	if inheritance {
		t.Error("re-applying the chain left the request path role with INHERIT")
	}
}

// The guard that reads the applied version before rolling back exists so that a
// rollback cannot report success without doing anything. Both of its arms are
// asserted here: nothing applied is a quiet success, a version whose file is
// missing from the source is an error.
func TestRollbackOnAnEmptyDatabaseSucceedsQuietly(t *testing.T) {
	db := cluster.NewDatabase(t)
	path := testsupport.MigrationsPath(t)

	if err := database.MigrateDown(db.MigrationURL, path, 0); err != nil {
		t.Fatalf("first rollback: %v", err)
	}

	if err := database.MigrateDown(db.MigrationURL, path, 0); err != nil {
		t.Fatalf("rolling back an already-empty database: %v", err)
	}
}

func TestRollbackRefusesWhenTheAppliedVersionIsMissingFromTheSource(t *testing.T) {
	db := cluster.NewDatabase(t)

	// A checkout that predates the applied migration — the ordinary "roll the
	// app back, then roll the schema back" sequence. golang-migrate reports it
	// with the same sentinel it uses for an empty database, and swallowing both
	// is how a rollback prints success and changes nothing.
	source := t.TempDir()
	for _, name := range []string{"000002_later.up.sql", "000002_later.down.sql"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte("SELECT 1;\n"), 0o644); err != nil {
			t.Fatalf("seeding %s: %v", name, err)
		}
	}

	if err := database.MigrateDown(db.MigrationURL, source, 1); err == nil {
		t.Fatal("rolling back a version absent from the source reported success")
	}
}

// force is used once, under pressure, against a half-applied production
// database. Proving it there for the first time is not a plan.
func TestForceClearsTheDirtyFlag(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()
	path := testsupport.MigrationsPath(t)

	// The chain's own bookkeeping, not golang-migrate's default name: GoTrue
	// shares this database and claims `schema_migrations` for its own migrator.
	conn := testsupport.Connect(t, db.MigrationURL)
	tag, err := conn.Exec(ctx, `UPDATE cadence_schema_migrations SET dirty = true`)
	if err != nil {
		t.Fatalf("marking the chain dirty: %v", err)
	}
	// Asserted rather than assumed: an UPDATE that matched nothing would leave
	// the chain clean, and every assertion below would then be testing the
	// happy path under a name that says otherwise.
	if tag.RowsAffected() == 0 {
		t.Fatal("no version row to mark dirty — the chain recorded nothing")
	}

	if err := database.RunMigrations(db.MigrationURL, path); err == nil {
		t.Fatal("a dirty chain applied without complaint")
	}

	if err := database.MigrateForce(db.MigrationURL, path, 1); err != nil {
		t.Fatalf("forcing the version: %v", err)
	}

	if err := database.RunMigrations(db.MigrationURL, path); err != nil {
		t.Fatalf("applying after force: %v", err)
	}
}

// There is one loosening the chain cannot undo: a membership granted by another
// role. A grant only ever updates the row belonging to the role issuing it, and
// revoking reaches only the grants this role has authority over — so a
// superuser's permissive grant survives both.
//
// What the chain must not do is carry on and report a separation it did not
// achieve. This is the failure the whole spec was rewritten around, and the only
// honest answer at this point is to stop.
func TestChainRefusesAMembershipItCannotRevoke(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	admin := testsupport.Connect(t, db.SuperuserURL)
	if _, err := admin.Exec(ctx, `GRANT cadence_authenticated TO cadence_app WITH INHERIT TRUE`); err != nil {
		t.Fatalf("granting as a superuser: %v", err)
	}

	// Databases are per-test; roles and the grants between them are not. This
	// test deliberately creates the one piece of state the chain cannot clear
	// itself, so it has to clear it — otherwise every later test in the binary
	// applies the chain into the refusal it just planted.
	t.Cleanup(func() {
		cleanupCtx := context.WithoutCancel(ctx)
		if _, err := admin.Exec(cleanupCtx, `REVOKE cadence_authenticated FROM cadence_app`); err != nil {
			t.Errorf("revoking the superuser grant: %v", err)
		}
	})

	statements, err := os.ReadFile(filepath.Join(testsupport.MigrationsPath(t), "000001_base.up.sql"))
	if err != nil {
		t.Fatalf("reading the base migration: %v", err)
	}

	conn := testsupport.Connect(t, db.MigrationURL)
	_, err = conn.Exec(ctx, string(statements))
	if err == nil {
		t.Fatal("the chain applied onto an inheriting membership it cannot revoke, and said nothing")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || !strings.Contains(pgErr.Message, "impersonation seam would be optional") {
		t.Fatalf("the chain failed with %v, want the explicit refusal", err)
	}
}

// The chain has to come back after it has gone away: a rollback that leaves the
// cluster in a state the next apply cannot handle is a one-way door.
func TestChainReappliesAfterRollback(t *testing.T) {
	db := cluster.NewDatabase(t)
	path := testsupport.MigrationsPath(t)

	if err := database.MigrateDown(db.MigrationURL, path, 0); err != nil {
		t.Fatalf("rolling back: %v", err)
	}

	if err := database.RunMigrations(db.MigrationURL, path); err != nil {
		t.Fatalf("re-applying: %v", err)
	}

	if offenders := tablesMissingForcedRLS(t, db.SuperuserURL); len(offenders) > 0 {
		t.Errorf("tables without forced row level security after re-apply: %v", offenders)
	}
}

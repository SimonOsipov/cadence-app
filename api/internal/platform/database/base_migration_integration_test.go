//go:build integration

package database_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

var cluster *testsupport.Cluster

// os.Exit runs no deferred function and a panicking test never returns through m.Run, so the teardown lives in a
// function of its own: without it a panic leaves the containers running, and TESTCONTAINERS_RYUK_DISABLED means
// nothing else reaps them.
func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

func runSuite(m *testing.M) int {
	ctx := context.Background()

	c, err := testsupport.StartCluster(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting the test cluster: %v\n", err)

		return 1
	}
	cluster = c

	defer func() {
		if err := cluster.Terminate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "terminating the test cluster: %v\n", err)
		}
	}()

	return m.Run()
}

// The roles under test are the ones a deployment actually connects with, read
// back from the connection rather than looked up by name. The first draft of
// this spec asserted the attributes of cadence_authenticated, which is NOLOGIN
// and therefore nobody's connection: the assertion passed while the request
// path ran as the database owner.
//
// Both paths are covered, because the service path is the one that would be
// tempting to hand a shortcut to: it writes rows no policy lets the request
// path write, and the way to make that easy is exactly the attribute that would
// make every policy advisory.
func TestConnectionRolesAreLowPrivilege(t *testing.T) {
	db := cluster.NewDatabase(t)

	for role, dsn := range map[string]string{
		testsupport.AppRole:        db.AppURL,
		testsupport.ServiceAppRole: db.ServiceAppURL,
	} {
		t.Run(role, func(t *testing.T) {
			conn := testsupport.Connect(t, dsn)

			var (
				name        string
				super       bool
				bypassRLS   bool
				replication bool
				createRole  bool
				createDB    bool
				inheritance bool
			)
			err := conn.QueryRow(t.Context(), `
				SELECT rolname, rolsuper, rolbypassrls, rolreplication,
				       rolcreaterole, rolcreatedb, rolinherit
				FROM pg_roles
				WHERE rolname = CURRENT_USER
			`).Scan(&name, &super, &bypassRLS, &replication, &createRole, &createDB, &inheritance)
			if err != nil {
				t.Fatalf("reading the current role: %v", err)
			}

			if name != role {
				t.Errorf("CURRENT_USER = %q, want %q", name, role)
			}
			if super {
				t.Errorf("%s is a superuser", role)
			}
			if bypassRLS {
				t.Errorf("%s can bypass RLS, which makes every policy advisory", role)
			}
			if replication {
				t.Errorf("%s can stream WAL, which reads every row of every table "+
					"without a policy ever running", role)
			}
			if createRole {
				t.Errorf("%s can create roles", role)
			}
			if createDB {
				t.Errorf("%s can create databases", role)
			}
			// NOINHERIT is what makes the impersonation seam load-bearing:
			// membership in the impersonation targets is usable only through an
			// explicit SET ROLE, so a query that skips it has no table
			// privileges at all.
			if inheritance {
				t.Errorf("%s inherits its granted roles, so skipping SET ROLE goes unnoticed", role)
			}
		})
	}
}

// The whole block stands on one assumption: the chain cannot hand anybody
// BYPASSRLS, so every write — the service path's included — has to be permitted
// by a policy. An assumption a spec quotes is worth less than one a test probes,
// and this one decides whether the §04 service pool exists at all.
func TestChainCannotCreateABypassRLSRole(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	conn := testsupport.Connect(t, db.MigrationURL)

	// Opened before the cleanup that uses it: t.Context is already cancelled by
	// the time cleanups run, so a connection dialled in there never gets made.
	admin := testsupport.Connect(t, db.SuperuserURL)

	// The control. Without it a CREATEROLE attribute quietly lost in the harness
	// would make the assertion below pass for the wrong reason.
	if _, err := conn.Exec(ctx, `CREATE ROLE cadence_bypass_probe NOLOGIN NOBYPASSRLS`); err != nil {
		t.Fatalf("the migration role cannot create an ordinary role at all: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx := context.WithoutCancel(ctx)
		for _, role := range []string{"cadence_bypass_probe", "cadence_bypass_probe_2"} {
			if _, err := admin.Exec(cleanupCtx, `DROP ROLE IF EXISTS `+role); err != nil {
				t.Errorf("dropping %s: %v", role, err)
			}
		}
	})

	// Both spellings, because they are two different code paths in Postgres and
	// the block's assumption covers both: no role the chain can reach ever
	// escapes a policy.
	for name, statement := range map[string]string{
		"altering an existing role": `ALTER ROLE cadence_bypass_probe BYPASSRLS`,
		"creating a new one":        `CREATE ROLE cadence_bypass_probe_2 NOLOGIN BYPASSRLS`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := conn.Exec(ctx, statement)
			if err == nil {
				t.Fatal("the migration role granted BYPASSRLS; the access model is advisory, " +
					"not enforced")
			}

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
				t.Fatalf("granting BYPASSRLS failed with %v, want SQLSTATE 42501 "+
					"(insufficient_privilege)", err)
			}
		})
	}
}

// The two paths are separated by session_user, not by a statement. If the
// request path could become cadence_service it would hold the service path's
// grants — and the service path is the one that writes rows the policies of the
// request path forbid.
func TestServicePathIsUnreachableFromTheRequestPath(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	app := testsupport.Connect(t, db.AppURL)

	_, err := app.Exec(ctx, `SET ROLE `+testsupport.ServiceRole)
	if err == nil {
		t.Fatal("the request path became the service role; the two pools are one privilege domain")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("SET ROLE %s from the request path failed with %v, want SQLSTATE 42501",
			testsupport.ServiceRole, err)
	}

	// The other half: the barrier has to be a barrier and not a role nobody can
	// assume. Without this the test would pass against a chain that granted the
	// membership to nobody at all.
	service := testsupport.Connect(t, db.ServiceAppURL)
	if _, err := service.Exec(ctx, `SET ROLE `+testsupport.ServiceRole); err != nil {
		t.Fatalf("the service path cannot assume %s: %v", testsupport.ServiceRole, err)
	}
}

// The membership graph is small enough to state exhaustively, and stating it
// exhaustively is the point: a surplus edge is how a role quietly acquires
// somebody else's grants, and an edge that inherits is how the seam stops being
// mandatory.
func TestMembershipsAreExactlyTheFourExpected(t *testing.T) {
	db := cluster.NewDatabase(t)

	conn := testsupport.Connect(t, db.SuperuserURL)

	// Filtered on the member side alone. Closing both sides would look
	// exhaustive and see nothing of `pg_execute_server_program -> cadence_app`,
	// which is programs running on the database host, reached from the request
	// path without the seam ever being called.
	rows, err := conn.Query(t.Context(), `
		SELECT granted.rolname, member.rolname, m.inherit_option
		FROM pg_auth_members m
		JOIN pg_roles granted ON granted.oid = m.roleid
		JOIN pg_roles member ON member.oid = m.member
		WHERE member.rolname = ANY($1)
		ORDER BY 1, 2
	`, testsupport.ChainRoles())
	if err != nil {
		t.Fatalf("reading the membership graph: %v", err)
	}
	defer rows.Close()

	var found []string
	for rows.Next() {
		var granted, member string
		var inherits bool
		if err := rows.Scan(&granted, &member, &inherits); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		if inherits {
			t.Errorf("%s is granted to %s with inheritance, so the impersonation seam is optional",
				granted, member)
		}
		found = append(found, granted+" -> "+member)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating: %v", err)
	}

	want := []string{
		testsupport.AdminRole + " -> " + testsupport.AppRole,
		testsupport.DoctorRole + " -> " + testsupport.AppRole,
		testsupport.PatientRole + " -> " + testsupport.AppRole,
		testsupport.ServiceRole + " -> " + testsupport.ServiceAppRole,
	}
	if !slices.Equal(found, want) {
		t.Errorf("membership graph is %v, want exactly %v", found, want)
	}
}

// The role the product roles replaced does not get to survive. On a cluster
// where an earlier version of this chain ran, cadence_authenticated still holds
// USAGE on the schema, still carries default privileges pointing at it, and
// cadence_app is still a member — so SET ROLE cadence_authenticated would keep
// working and keep bypassing the per-role grants the whole block is built on.
//
// This plants exactly that state and re-applies the chain onto it. It is not a
// hypothetical: it is what every machine that ran the previous version starts
// from.
func TestChainAbolishesTheRoleTheProductRolesReplaced(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	admin := testsupport.Connect(t, db.SuperuserURL)

	t.Cleanup(func() {
		cleanupCtx := context.WithoutCancel(ctx)
		if _, err := admin.Exec(cleanupCtx, `DROP ROLE IF EXISTS cadence_authenticated`); err != nil {
			t.Errorf("dropping the abolished role: %v", err)
		}
	})

	bootstrap := testsupport.Connect(t, db.MigrationURL)
	for _, statement := range []string{
		`CREATE ROLE cadence_authenticated NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS`,
		`GRANT cadence_authenticated TO cadence_app WITH INHERIT FALSE`,
		`GRANT USAGE ON SCHEMA app TO cadence_authenticated`,
		`ALTER DEFAULT PRIVILEGES FOR ROLE cadence_owner IN SCHEMA app ` +
			`GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO cadence_authenticated`,
	} {
		if _, err := bootstrap.Exec(ctx, statement); err != nil {
			t.Fatalf("planting the previous arrangement (%s): %v", statement, err)
		}
	}

	statements, err := os.ReadFile(filepath.Join(testsupport.MigrationsPath(t), "000001_base.up.sql"))
	if err != nil {
		t.Fatalf("reading the base migration: %v", err)
	}
	if _, err := bootstrap.Exec(ctx, string(statements)); err != nil {
		t.Fatalf("re-applying the chain onto the previous arrangement: %v", err)
	}

	var survived int
	if err := admin.QueryRow(
		ctx,
		`SELECT count(*) FROM pg_roles WHERE rolname = 'cadence_authenticated'`,
	).Scan(&survived); err != nil {
		t.Fatalf("counting the abolished role: %v", err)
	}
	if survived != 0 {
		t.Fatal("cadence_authenticated survived the chain; a query can still assume it and " +
			"skip the per-role grants entirely")
	}

	// The other direction, and it needs its own arrangement: the rollback names
	// the abolished role too, and after the chain has removed it there is
	// nothing left for that branch to do. Planted again, then rolled back.
	if _, err := bootstrap.Exec(
		ctx, `CREATE ROLE cadence_authenticated NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS`,
	); err != nil {
		t.Fatalf("planting the abolished role for the rollback: %v", err)
	}
	if _, err := bootstrap.Exec(ctx, `GRANT USAGE ON SCHEMA app TO cadence_authenticated`); err != nil {
		t.Fatalf("planting the abolished role's schema privilege: %v", err)
	}

	if err := database.MigrateDown(db.MigrationURL, testsupport.MigrationsPath(t), 0); err != nil {
		t.Fatalf("rolling the chain back: %v", err)
	}

	if err := admin.QueryRow(
		ctx,
		`SELECT count(*) FROM pg_roles WHERE rolname = 'cadence_authenticated'`,
	).Scan(&survived); err != nil {
		t.Fatalf("counting the abolished role after the rollback: %v", err)
	}
	if survived != 0 {
		t.Error("cadence_authenticated survived the rollback; a cluster that ran the previous " +
			"chain is not left clean")
	}
}

// The local stack creates Supabase's role names so GoTrue's own migrations have
// something to grant to. They are empty by construction, and this is what keeps
// them empty: with ALTER DEFAULT PRIVILEGES gone from the chain there is no
// mechanism left that could hand a privilege to a role nobody named.
func TestAlienRolesHoldNothingInTheApplicationSchema(t *testing.T) {
	ctx := t.Context()

	admin := testsupport.Connect(t, cluster.AdminDSN())

	// Roles are cluster objects, and the GoTrue harness creates this same name
	// for the same reason the local stack does. So the role is planted only when
	// it is missing, and taken away only by whoever planted it — a bare CREATE
	// makes this test fail whenever an identity provider ran first in the same
	// binary, and an unconditional DROP would take the role out from under the
	// next one.
	var missing bool
	if err := admin.QueryRow(
		ctx, `SELECT NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'service_role')`,
	).Scan(&missing); err != nil {
		t.Fatalf("looking for the alien role: %v", err)
	}

	if missing {
		if _, err := admin.Exec(ctx, `CREATE ROLE service_role NOLOGIN`); err != nil {
			t.Fatalf("creating the alien role: %v", err)
		}

		t.Cleanup(func() {
			cleanupCtx := context.WithoutCancel(ctx)
			if _, err := admin.Exec(cleanupCtx, `DROP ROLE IF EXISTS service_role`); err != nil {
				t.Errorf("dropping the alien role: %v", err)
			}
		})
	}

	// Created after the role, so the chain runs in a cluster where the alien
	// role already exists — which is the order a real deployment has.
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)

	var usage bool
	if err := conn.QueryRow(
		ctx,
		`SELECT has_schema_privilege('service_role', $1, 'USAGE')`, testsupport.AppSchema,
	).Scan(&usage); err != nil {
		t.Fatalf("reading the alien role's schema privilege: %v", err)
	}
	if usage {
		t.Errorf("service_role holds USAGE on schema %s; the chain grants to roles it did not name",
			testsupport.AppSchema)
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

// The chain has to survive without superuser, because it will never have one —
// see bootstrapRole. Asserted rather than assumed so that a future change to the
// harness cannot quietly hand the chain rights the deployment does not have.
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
		t.Fatal("the chain was applied by a superuser, so it proves nothing about the deployment")
	}
}

// The arrangement of the roles is only worth anything if it actually governs
// access to a table. There are no tables yet, so this test makes one — and it
// has to issue the grant itself, because with ALTER DEFAULT PRIVILEGES gone
// from the chain a privilege arrives only where a migration writes it. What
// remains under test is the seam: without SET ROLE the request path holds
// nothing at all, whatever the grants say.
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
	if _, err := owner.Exec(ctx, `GRANT SELECT ON app.probe TO `+testsupport.PatientRole); err != nil {
		t.Fatalf("granting on the probe table: %v", err)
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

	if _, err := app.Exec(ctx, `SET ROLE `+testsupport.PatientRole); err != nil {
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

	for _, role := range testsupport.ChainRoles() {
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

	// btree_gist lives outside app, so DROP SCHEMA app CASCADE does not reach it
	// and every assertion above is blind to it. A chain that leaves an extension
	// installed is one whose next apply starts from a state nobody described.
	var extensions int
	if err := admin.QueryRow(
		ctx,
		`SELECT count(*) FROM pg_extension WHERE extname <> 'plpgsql'`,
	).Scan(&extensions); err != nil {
		t.Fatalf("counting the extensions: %v", err)
	}
	if extensions != 0 {
		t.Errorf("%d extension(s) survived the rollback", extensions)
	}
}

// The Makefile's migrate-down target passes no argument, which resolves to one
// step — a different code path from the whole-chain rollback, and the one an
// operator actually runs after a bad deploy.
//
// Written against the chain's length rather than against a particular migration,
// because the chain grows: a test naming the latest file has to be edited by
// every step that adds one, and a test edited that often stops being read.
func TestUnwindingTheChainOneStepAtATimeReachesTheBase(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()
	path := testsupport.MigrationsPath(t)

	conn := testsupport.Connect(t, db.MigrationURL)
	admin := testsupport.Connect(t, db.SuperuserURL)

	appliedVersion := func() *int {
		t.Helper()

		var version *int
		if err := conn.QueryRow(ctx, `SELECT version FROM cadence_schema_migrations`).Scan(&version); err != nil {
			t.Fatalf("reading the applied version: %v", err)
		}

		return version
	}

	countBaseRoles := func() int {
		t.Helper()

		var roles int
		if err := admin.QueryRow(
			ctx,
			`SELECT count(*) FROM pg_roles WHERE rolname = ANY($1)`, testsupport.BaseRoles(),
		).Scan(&roles); err != nil {
			t.Fatalf("counting the base roles: %v", err)
		}

		return roles
	}

	countRoles := func() int {
		t.Helper()

		var roles int
		if err := admin.QueryRow(
			ctx,
			`SELECT count(*) FROM pg_roles WHERE rolname = ANY($1)`, testsupport.ChainRoles(),
		).Scan(&roles); err != nil {
			t.Fatalf("counting roles: %v", err)
		}

		return roles
	}

	// Everything the chain can put in the schema, counted together. Naming
	// tables would tie this to whichever migration happens to be last — and the
	// one after it adds policies, the one after that a function.
	countObjects := func() int {
		t.Helper()

		var objects int
		if err := admin.QueryRow(ctx, `
			SELECT (
				SELECT count(*) FROM pg_class c
				JOIN pg_namespace n ON n.oid = c.relnamespace
				WHERE n.nspname = $1 AND c.relkind IN ('r', 'p', 'S', 'v', 'm')
			) + (
				SELECT count(*) FROM pg_proc p
				JOIN pg_namespace n ON n.oid = p.pronamespace
				WHERE n.nspname = $1
			) + (
				SELECT count(*) FROM pg_policies WHERE schemaname = $1
			)
		`, testsupport.AppSchema).Scan(&objects); err != nil {
			t.Fatalf("counting the objects of the schema: %v", err)
		}

		return objects
	}

	// The same objects, plus what a policy actually says, plus what the chain's
	// roles carry as connection defaults. A migration that rewrites a policy
	// rather than adding one leaves the count where it was — 000006 is the first
	// of those — and one that only sets a limit on a role touches the schema not
	// at all, which 000007 is the first of. The witness has to be able to see
	// both, or an empty down file passes.
	describeSchema := func() string {
		t.Helper()

		var description string
		if err := admin.QueryRow(ctx, `
			SELECT coalesce(string_agg(line, E'\n' ORDER BY line), '') FROM (
				SELECT c.relkind::text || ' ' || c.relname AS line
				FROM pg_class c
				JOIN pg_namespace n ON n.oid = c.relnamespace
				WHERE n.nspname = $1 AND c.relkind IN ('r', 'p', 'S', 'v', 'm')
				UNION ALL
				SELECT 'f ' || p.proname || ' ' || pg_get_functiondef(p.oid)
				FROM pg_proc p
				JOIN pg_namespace n ON n.oid = p.pronamespace
				WHERE n.nspname = $1
				UNION ALL
				-- Columns, because without them a migration whose whole content is
				-- ADD COLUMN is invisible here: the relation list is unchanged, and
				-- an empty down file for it would pass the witness below. Measured
				-- on 000010, which adds one column and nothing else.
				-- The collation is in the line because format_type leaves it out: a migration whose
				-- whole content is ALTER COLUMN ... COLLATE moves nothing else here, and an empty down
				-- file for it would pass. Measured on 000012, which changes one column's collation.
				SELECT 'c ' || c.relname || '.' || a.attname || ' ' ||
				       pg_catalog.format_type(a.atttypid, a.atttypmod) ||
				       coalesce(' collate ' || co.collname, '') ||
				       CASE WHEN a.attnotnull THEN ' NOT NULL' ELSE '' END
				FROM pg_attribute a
				JOIN pg_class c ON c.oid = a.attrelid
				JOIN pg_namespace n ON n.oid = c.relnamespace
				LEFT JOIN pg_collation co ON co.oid = a.attcollation
				WHERE n.nspname = $1 AND a.attnum > 0 AND NOT a.attisdropped
				  AND c.relkind IN ('r', 'p', 'v', 'm')
				UNION ALL
				-- Constraints, for the same reason and the next case over: a
				-- migration whose whole content is ADD CONSTRAINT leaves the
				-- relation list and the column list alike untouched.
				SELECT 'k ' || c.relname || ' ' || con.conname || ' ' ||
				       pg_catalog.pg_get_constraintdef(con.oid)
				FROM pg_constraint con
				JOIN pg_class c ON c.oid = con.conrelid
				JOIN pg_namespace n ON n.oid = con.connamespace
				WHERE n.nspname = $1
				UNION ALL
				-- Indexes, for the same reason again and the case after that: a
				-- migration whose whole content is CREATE INDEX moves neither the
				-- relation list, nor the columns, nor the constraints. Measured on
				-- 000011, which adds one index and nothing else.
				SELECT 'i ' || i.indexname || ' ' || i.indexdef
				FROM pg_indexes i
				WHERE i.schemaname = $1
				UNION ALL
				SELECT 'p ' || tablename || ' ' || policyname || ' ' || cmd || ' ' ||
				       coalesce(qual, '-') || ' | ' || coalesce(with_check, '-')
				FROM pg_policies WHERE schemaname = $1
				UNION ALL
				SELECT 'r ' || r.rolname || ' ' || array_to_string(s.setconfig, ',')
				FROM pg_db_role_setting s
				JOIN pg_roles r ON r.oid = s.setrole
				WHERE r.rolname = ANY($2)
				UNION ALL
				-- Extensions, and they are not schema-scoped: 000013 installs
				-- btree_gist outside app, so every line above is blind to it and a
				-- migration whose whole content is CREATE EXTENSION would pass the
				-- witness with an empty down file. plpgsql is excluded because the
				-- cluster arrives with it.
				SELECT 'e ' || e.extname || ' in ' || n.nspname
				FROM pg_extension e
				JOIN pg_namespace n ON n.oid = e.extnamespace
				WHERE e.extname <> 'plpgsql'
			) AS objects
		`, testsupport.AppSchema, testsupport.ChainRoles()).Scan(&description); err != nil {
			t.Fatalf("describing the schema: %v", err)
		}

		return description
	}

	before := appliedVersion()
	if before == nil || *before < 2 {
		t.Fatal("the chain is one migration long, so stepping through it says nothing")
	}
	objectsBefore := countObjects()
	if objectsBefore == 0 {
		t.Fatal("the schema holds nothing, so a rollback has nothing to be seen undoing")
	}
	describedBefore := describeSchema()

	if err := database.MigrateDown(db.MigrationURL, path, 1); err != nil {
		t.Fatalf("rolling back one step: %v", err)
	}

	// One step is one migration. A chain whose steps did not compose would take
	// more than it was asked for.
	after := appliedVersion()
	if after == nil {
		t.Fatal("one step removed the version row entirely")
	}
	if *after != *before-1 {
		t.Fatalf("one step moved the version from %d to %d, want %d", *before, *after, *before-1)
	}

	// And the file it ran actually did something. golang-migrate decrements the
	// version whether or not the SQL inside the down file changed anything, so
	// without a schema-level witness an empty down migration passes — and the
	// whole-chain assertions below cannot supply one, because the base migration
	// drops the schema CASCADE and sweeps up whatever was left behind.
	//
	// The witness is the description rather than the count, because a migration
	// whose whole content is a rewritten policy leaves the count untouched while
	// changing the schema. The count is still checked in the one direction it
	// still means something: rolling back does not add.
	if describeSchema() == describedBefore {
		t.Error("the schema is unchanged by the rollback; the migration's down file did nothing")
	}
	if objectsAfter := countObjects(); objectsAfter > objectsBefore {
		t.Errorf("the schema held %d objects before the rollback and %d after; the migration's "+
			"down file added to it", objectsBefore, objectsAfter)
	}

	// It took the base arrangement nothing, though: the operator undoing the last
	// deploy is not undoing what every environment stands on. A later migration
	// may create a role of its own and take it back, which is why this counts the
	// seven of the base rather than every role the chain has ever made.
	roles := countBaseRoles()
	if want := len(testsupport.BaseRoles()); roles != want {
		t.Errorf("%d of the chain's %d roles survived a single-step rollback, want all of them",
			roles, want)
	}

	for step := *after; step > 0; step-- {
		if err := database.MigrateDown(db.MigrationURL, path, 1); err != nil {
			t.Fatalf("rolling back step %d: %v", step, err)
		}
	}

	if remaining := countRoles(); remaining != 0 {
		t.Errorf("%d of the chain's roles survived unwinding the chain step by step", remaining)
	}

	var schemas int
	if err := admin.QueryRow(
		ctx, `SELECT count(*) FROM pg_namespace WHERE nspname = $1`, testsupport.AppSchema,
	).Scan(&schemas); err != nil {
		t.Fatalf("counting the application schema: %v", err)
	}
	if schemas != 0 {
		t.Errorf("schema %s survived unwinding the chain step by step", testsupport.AppSchema)
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

	conn := testsupport.Connect(t, db.MigrationURL)

	// Every down migration, newest first, each applied twice — because the
	// property belongs to the file rather than to the chain, and a migration
	// whose rollback is not twice-safe is exactly as dangerous as the first one.
	//
	// Read from the directory rather than listed here: a migration added without
	// a twice-safe rollback must fail this test rather than be absent from it.
	entries, err := os.ReadDir(testsupport.MigrationsPath(t))
	if err != nil {
		t.Fatalf("reading the migrations directory: %v", err)
	}

	var downs []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".down.sql") {
			downs = append(downs, entry.Name())
		}
	}
	slices.Sort(downs)
	slices.Reverse(downs)

	if len(downs) < 2 {
		t.Fatal("fewer than two down migrations found; the directory was not read")
	}

	for _, name := range downs {
		statements, err := os.ReadFile(filepath.Join(testsupport.MigrationsPath(t), name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}

		for pass := 1; pass <= 2; pass++ {
			if _, err := conn.Exec(ctx, string(statements)); err != nil {
				t.Fatalf("applying %s, pass %d: %v", name, pass, err)
			}
		}
	}

	admin := testsupport.Connect(t, db.SuperuserURL)

	var roles int
	if err := admin.QueryRow(
		ctx,
		`SELECT count(*) FROM pg_roles WHERE rolname = ANY($1)`, testsupport.ChainRoles(),
	).Scan(&roles); err != nil {
		t.Fatalf("counting roles: %v", err)
	}
	if roles != 0 {
		t.Errorf("%d of the chain's roles survived two passes of the down migration", roles)
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
	if _, err := bootstrap.Exec(
		ctx, `GRANT `+testsupport.PatientRole+` TO cadence_app WITH INHERIT TRUE`,
	); err != nil {
		t.Fatalf("loosening the membership: %v", err)
	}

	// A second database in the same cluster finds the roles already there —
	// exactly the state a redeploy against a managed cluster starts from.
	second := cluster.NewDatabase(t)

	owner := testsupport.Connect(t, second.MigrationURL)
	if _, err := owner.Exec(ctx, `SET ROLE cadence_owner`); err != nil {
		t.Fatalf("becoming the owner: %v", err)
	}
	if _, err := owner.Exec(ctx, `CREATE TABLE app.probe (id integer PRIMARY KEY)`); err != nil {
		t.Fatalf("creating the probe table: %v", err)
	}
	// The grant is what makes the assertion below discriminate. Without it the
	// read fails with 42501 whether or not the membership inherits, and the test
	// would stay green against the very loosening it exists to catch.
	if _, err := owner.Exec(ctx, `GRANT SELECT ON app.probe TO `+testsupport.PatientRole); err != nil {
		t.Fatalf("granting on the probe table: %v", err)
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
	// Numbered above anything the real chain contains, so the applied version is
	// genuinely absent from this source rather than merely renamed in it.
	source := t.TempDir()
	for _, name := range []string{"000900_later.up.sql", "000900_later.down.sql"} {
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

	// Forced to the version that is actually applied, read back rather than
	// hardcoded: forcing to any other number is an operator telling the chain a
	// lie, and the next migration would then be applied on top of a schema that
	// already has it.
	var applied int
	if err := conn.QueryRow(ctx, `SELECT version FROM cadence_schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("reading the applied version: %v", err)
	}

	if err := database.MigrateForce(db.MigrationURL, path, applied); err != nil {
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
	// Every membership, not one of the four. The check in the chain was widened
	// to cover all of them, and a suite that plants only cadence_patient leaves
	// the other three unwatched while looking like it covers them — an inheriting
	// cadence_service on the service path is the seam of that path becoming
	// optional, which is the failure the whole spec was rewritten around.
	for granted, member := range map[string]string{
		testsupport.PatientRole: testsupport.AppRole,
		testsupport.DoctorRole:  testsupport.AppRole,
		testsupport.AdminRole:   testsupport.AppRole,
		testsupport.ServiceRole: testsupport.ServiceAppRole,
	} {
		t.Run(granted+"-to-"+member, func(t *testing.T) {
			assertChainRefusesTheMembership(t, granted, member)
		})
	}
}

func assertChainRefusesTheMembership(t *testing.T, granted, member string) {
	t.Helper()

	db := cluster.NewDatabase(t)
	ctx := t.Context()

	admin := testsupport.Connect(t, db.SuperuserURL)
	if _, err := admin.Exec(
		ctx, `GRANT `+granted+` TO `+member+` WITH INHERIT TRUE`,
	); err != nil {
		t.Fatalf("granting as a superuser: %v", err)
	}

	// Databases are per-test; roles and the grants between them are not. This
	// test deliberately creates the one piece of state the chain cannot clear
	// itself, so it has to clear it — otherwise every later test in the binary
	// applies the chain into the refusal it just planted.
	t.Cleanup(func() {
		cleanupCtx := context.WithoutCancel(ctx)
		if _, err := admin.Exec(cleanupCtx, `REVOKE `+granted+` FROM `+member); err != nil {
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

// The chain refuses a membership it cannot revoke whatever the granted role is,
// and the roles worth worrying about are mostly not this chain's own. A
// predefined role granted to the request path hands it privileges no policy
// governs, and none of the names involved matches cadence_%.
func TestChainRefusesAForeignMembershipIntoAConnectionRole(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	admin := testsupport.Connect(t, db.SuperuserURL)
	if _, err := admin.Exec(ctx, `GRANT pg_read_all_data TO cadence_app`); err != nil {
		t.Fatalf("granting a predefined role as a superuser: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx := context.WithoutCancel(ctx)
		if _, err := admin.Exec(cleanupCtx, `REVOKE pg_read_all_data FROM cadence_app`); err != nil {
			t.Errorf("revoking the predefined role: %v", err)
		}
	})

	statements, err := os.ReadFile(filepath.Join(testsupport.MigrationsPath(t), "000001_base.up.sql"))
	if err != nil {
		t.Fatalf("reading the base migration: %v", err)
	}

	conn := testsupport.Connect(t, db.MigrationURL)
	if _, err := conn.Exec(ctx, string(statements)); err == nil {
		t.Fatal("the chain applied onto a role granted pg_read_all_data and said nothing; " +
			"the request path can read every table with no policy in sight")
	} else {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || !strings.Contains(pgErr.Message, "did not grant") {
			t.Fatalf("the chain failed with %v, want the explicit refusal", err)
		}
	}
}

// IF NOT EXISTS skips, and a skip is not convergence. A schema created by hand
// under another owner would leave the owner of every future table able to turn
// row level security off on it — the one thing the whole arrangement exists to
// prevent — and the only signal Postgres gives is a NOTICE that golang-migrate
// discards.
func TestChainTakesOwnershipOfASchemaSomebodyElseCreated(t *testing.T) {
	// A database whose roles already exist, so the schema can be handed to one
	// of them — and rolled back afterwards so the next test starts from nothing.
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	bootstrap := testsupport.Connect(t, db.MigrationURL)
	if err := database.MigrateDown(db.MigrationURL, testsupport.MigrationsPath(t), 0); err != nil {
		t.Fatalf("clearing the chain: %v", err)
	}

	// Somebody else, literally: created by the applying role but owned by the
	// request path, with the grants a person reaching for `it does not work`
	// would write. This is the state the chain has to converge, and the branch
	// that grants the current owner to CURRENT_USER only runs here.
	if err := database.RunMigrations(db.MigrationURL, testsupport.MigrationsPath(t)); err != nil {
		t.Fatalf("applying the chain to create the roles: %v", err)
	}
	if err := database.MigrateDown(db.MigrationURL, testsupport.MigrationsPath(t), 0); err != nil {
		t.Fatalf("clearing the chain again: %v", err)
	}

	admin := testsupport.Connect(t, db.SuperuserURL)
	for _, statement := range []string{
		`CREATE ROLE app_owner_probe NOLOGIN`,
		`CREATE SCHEMA app AUTHORIZATION app_owner_probe`,
		`GRANT ALL ON SCHEMA app TO PUBLIC`,
	} {
		if _, err := admin.Exec(ctx, statement); err != nil {
			t.Fatalf("planting the foreign schema (%s): %v", statement, err)
		}
	}
	if _, err := admin.Exec(ctx, `GRANT app_owner_probe TO `+testsupport.MigrationRole()); err != nil {
		t.Fatalf("letting the applying role take ownership: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx := context.WithoutCancel(ctx)
		for _, statement := range []string{
			`DROP OWNED BY app_owner_probe CASCADE`,
			`DROP ROLE IF EXISTS app_owner_probe`,
		} {
			if _, err := admin.Exec(cleanupCtx, statement); err != nil {
				t.Errorf("cleaning up the probe owner (%s): %v", statement, err)
			}
		}
	})

	if err := database.RunMigrations(db.MigrationURL, testsupport.MigrationsPath(t)); err != nil {
		t.Fatalf("applying the chain onto a pre-existing schema: %v", err)
	}

	var owner string
	if err := bootstrap.QueryRow(ctx, `
		SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname = $1
	`, testsupport.AppSchema).Scan(&owner); err != nil {
		t.Fatalf("reading the schema owner: %v", err)
	}

	if owner != testsupport.OwnerRole {
		t.Errorf("schema %s is owned by %q after the chain, want %q — a table created in it "+
			"would belong to a role that can disable row level security on it",
			testsupport.AppSchema, owner, testsupport.OwnerRole)
	}

	// Ownership was only half of it. ALTER SCHEMA ... OWNER TO leaves every
	// grant the previous owner issued exactly where it was, so without the
	// sweep the request path comes out of the chain still holding CREATE on the
	// application schema — free to create a table, own it, and turn row level
	// security off on it.
	for role, verbs := range map[string][]string{
		testsupport.AppRole:        {"CREATE", "USAGE"},
		testsupport.ServiceAppRole: {"CREATE", "USAGE"},
		"public":                   {"CREATE", "USAGE"},
	} {
		for _, verb := range verbs {
			var holds bool
			if err := bootstrap.QueryRow(
				ctx,
				`SELECT has_schema_privilege($1, $2, $3)`, role, testsupport.AppSchema, verb,
			).Scan(&holds); err != nil {
				t.Fatalf("reading %s's %s on the schema: %v", role, verb, err)
			}

			if holds {
				t.Errorf("%s still holds %s on schema %s after the chain converged its owner",
					role, verb, testsupport.AppSchema)
			}
		}
	}
}

// A schema that already holds somebody else's table is refused rather than
// adopted. Converging the owner of the schema says nothing about the objects
// inside it, and their owner can disable row level security on them — while the
// pg_class sweep that would notice lives in this suite and never runs against a
// deployed cluster.
func TestChainRefusesASchemaHoldingObjectsItDoesNotOwn(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	bootstrap := testsupport.Connect(t, db.MigrationURL)
	if _, err := bootstrap.Exec(ctx, `CREATE TABLE app.legacy (id integer PRIMARY KEY)`); err != nil {
		t.Fatalf("planting a table owned by the applying role: %v", err)
	}

	statements, err := os.ReadFile(filepath.Join(testsupport.MigrationsPath(t), "000001_base.up.sql"))
	if err != nil {
		t.Fatalf("reading the base migration: %v", err)
	}

	_, err = bootstrap.Exec(ctx, string(statements))
	if err == nil {
		t.Fatal("the chain applied onto a schema holding a table it does not own, and said nothing")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || !strings.Contains(pgErr.Message, "does not own") {
		t.Fatalf("the chain failed with %v, want the explicit refusal", err)
	}
}

// The ALTER ROLE declarations converge the impersonation targets too, and the
// attribute guard does not check rolcanlogin — so a hand-created cadence_patient
// with LOGIN is fixed by that loop alone. Without a witness the loop can be
// dropped and every other test stays green.
func TestChainConvergesAnImpersonationTargetLeftWithLogin(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	admin := testsupport.Connect(t, db.SuperuserURL)
	if _, err := admin.Exec(ctx, `ALTER ROLE `+testsupport.PatientRole+` LOGIN`); err != nil {
		t.Fatalf("loosening the impersonation target: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx := context.WithoutCancel(ctx)
		if _, err := admin.Exec(cleanupCtx, `ALTER ROLE `+testsupport.PatientRole+` NOLOGIN`); err != nil {
			t.Errorf("restoring the impersonation target: %v", err)
		}
	})

	// A second database in the same cluster finds the roles already there.
	second := cluster.NewDatabase(t)
	_ = second

	var canLogin bool
	if err := admin.QueryRow(
		ctx,
		`SELECT rolcanlogin FROM pg_roles WHERE rolname = $1`, testsupport.PatientRole,
	).Scan(&canLogin); err != nil {
		t.Fatalf("reading the impersonation target: %v", err)
	}

	if canLogin {
		t.Errorf("%s can still log in after the chain re-applied; an impersonation target with "+
			"a password is a way into the database that skips the seam entirely",
			testsupport.PatientRole)
	}
}

// The assumable roles are NOLOGIN, so no connection can read their attributes
// back from CURRENT_USER — and they are the roles that actually run: the
// impersonation targets are what the policies are evaluated as, and
// cadence_auth_hook is what the identity provider's own database user becomes.
// BYPASSRLS on cadence_patient makes every policy of M2 decorative while every
// test about connections stays green, and CREATEROLE on cadence_auth_hook hands
// that to whatever is reachable from the internet, with the same silence.
func TestAssumableRolesCarryNoEscapeAttributes(t *testing.T) {
	db := cluster.NewDatabase(t)

	conn := testsupport.Connect(t, db.SuperuserURL)
	rows, err := conn.Query(t.Context(), `
		SELECT rolname
		FROM pg_roles
		WHERE rolname = ANY($1)
		  AND (rolsuper OR rolbypassrls OR rolreplication OR rolcreaterole OR rolcreatedb
		       OR rolcanlogin)
		ORDER BY rolname
	`, append(testsupport.ImpersonationRoles(), testsupport.AuthHookRole))
	if err != nil {
		t.Fatalf("reading the impersonation targets: %v", err)
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

	if len(offenders) > 0 {
		t.Errorf("assumable roles carrying an escape attribute or LOGIN: %v", offenders)
	}
}

// Privileges on the application schema are granted to a closed set of roles and
// to no others. The connection roles holding nothing on the schema is what makes
// a query that skips the seam fail with 42501 rather than merely find no rows.
//
// The set is read out of the schema's ACL rather than asked about role by role.
// The earlier form named six roles and asked `has_schema_privilege` of each,
// which answers "did this role get it" and can never answer "who else did" — the
// migration that added cadence_auth_hook granted USAGE to a seventh role and the
// test stayed green. A list is not a set until something enumerates it.
//
// Two things this cannot see, both covered elsewhere. It reads *direct* grants,
// so a role that reaches the schema through a membership — cadence_migrator
// through cadence_owner, the identity provider's user through cadence_auth_hook
// — does not appear; that is the point of the membership and is enumerated by
// TestMembershipsAreExactlyTheFourExpected. And it says nothing about what the
// connection roles inherit, which the earlier form did assert as a side effect:
// that lives in the NOINHERIT check above and in
// TestRequestPathReachesTablesOnlyThroughImpersonation, which measures it as a
// live refusal.
func TestSchemaPrivilegesAreGrantedToAClosedSet(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)

	// The impersonation targets reach the tables; the owner owns the schema and
	// is the only role that may create in it; the hook role reaches one function
	// and nothing else. The two connection roles are deliberately absent — they
	// arrive at a table only by becoming one of the targets.
	//
	// CREATE is here rather than filtered out because it is the privilege that
	// matters most on a schema: a request path that could create a table in `app`
	// could create one with no row level security on it.
	want := map[string]bool{
		testsupport.OwnerRole + " USAGE":    true,
		testsupport.OwnerRole + " CREATE":   true,
		testsupport.AuthHookRole + " USAGE": true,
	}
	for _, role := range testsupport.ImpersonationRoles() {
		want[role+" USAGE"] = true
	}

	rows, err := conn.Query(t.Context(), `
		SELECT CASE WHEN a.grantee = 0 THEN 'PUBLIC' ELSE a.grantee::regrole::text END
		       || ' ' || a.privilege_type
		FROM pg_namespace n, aclexplode(n.nspacl) a
		WHERE n.nspname = $1
	`, testsupport.AppSchema)
	if err != nil {
		t.Fatalf("reading the schema's ACL: %v", err)
	}
	defer rows.Close()

	got := map[string]bool{}
	for rows.Next() {
		var grant string
		if err := rows.Scan(&grant); err != nil {
			t.Fatalf("scanning a grant: %v", err)
		}
		got[grant] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating: %v", err)
	}

	for grant := range got {
		if !want[grant] {
			t.Errorf("%q on schema %s, and nothing in the chain grants it",
				grant, testsupport.AppSchema)
		}
	}
	for grant := range want {
		if !got[grant] {
			t.Errorf("%q is missing on schema %s", grant, testsupport.AppSchema)
		}
	}
}

// Removing ALTER DEFAULT PRIVILEGES is the point of this step, and a removal
// nothing measures is a removal that comes back. A table created after the chain
// has to reach nobody until its own migration writes a grant, per role and where
// needed per column — which is the matrix default privileges cannot express.
func TestATableCreatedAfterTheChainReachesNobody(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	owner := testsupport.Connect(t, db.MigrationURL)
	if _, err := owner.Exec(ctx, `SET ROLE cadence_owner`); err != nil {
		t.Fatalf("becoming the owner: %v", err)
	}
	if _, err := owner.Exec(ctx, `CREATE TABLE app.ungranted (id integer PRIMARY KEY)`); err != nil {
		t.Fatalf("creating the table: %v", err)
	}

	admin := testsupport.Connect(t, db.SuperuserURL)

	for _, role := range testsupport.ImpersonationRoles() {
		for _, verb := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
			var granted bool
			if err := admin.QueryRow(
				ctx,
				`SELECT has_table_privilege($1, 'app.ungranted', $2)`, role, verb,
			).Scan(&granted); err != nil {
				t.Fatalf("reading %s's %s privilege: %v", role, verb, err)
			}

			if granted {
				t.Errorf("%s holds %s on a table no migration granted it — "+
					"default privileges are back in the chain", role, verb)
			}
		}
	}

	// The mechanism itself, not only its effect: a declaration left behind
	// reaches every table of every later migration, and the check above would
	// only notice once such a table existed.
	var declarations int
	if err := admin.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_default_acl d
		JOIN pg_namespace n ON n.oid = d.defaclnamespace
		WHERE n.nspname = $1
	`, testsupport.AppSchema).Scan(&declarations); err != nil {
		t.Fatalf("counting default privilege declarations: %v", err)
	}
	if declarations != 0 {
		t.Errorf("%d default privilege declaration(s) on schema %s, want none",
			declarations, testsupport.AppSchema)
	}
}

// The attribute guard raises rather than reporting a separation it did not
// achieve. It cannot clear SUPERUSER, BYPASSRLS or REPLICATION — only a role
// holding them can — so stopping is the only honest answer, and REPLICATION is
// on the list because a role that streams WAL reads every row of every table
// without a policy ever running.
func TestChainRefusesARoleCarryingAnEscapeAttribute(t *testing.T) {
	for name, attribute := range map[string]string{
		"replication": "REPLICATION",
		"bypassrls":   "BYPASSRLS",
	} {
		t.Run(name, func(t *testing.T) {
			db := cluster.NewDatabase(t)
			ctx := t.Context()

			admin := testsupport.Connect(t, db.SuperuserURL)
			if _, err := admin.Exec(ctx, `ALTER ROLE cadence_app `+attribute); err != nil {
				t.Fatalf("loosening the request path role: %v", err)
			}

			t.Cleanup(func() {
				cleanupCtx := context.WithoutCancel(ctx)
				if _, err := admin.Exec(cleanupCtx, `ALTER ROLE cadence_app NO`+attribute); err != nil {
					t.Errorf("clearing %s: %v", attribute, err)
				}
			})

			statements, err := os.ReadFile(
				filepath.Join(testsupport.MigrationsPath(t), "000001_base.up.sql"),
			)
			if err != nil {
				t.Fatalf("reading the base migration: %v", err)
			}

			conn := testsupport.Connect(t, db.MigrationURL)
			_, err = conn.Exec(ctx, string(statements))
			if err == nil {
				t.Fatalf("the chain applied onto a role carrying %s and said nothing", attribute)
			}

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) ||
				!strings.Contains(pgErr.Message, "privileges the request path must never hold") {
				t.Fatalf("the chain failed with %v, want the explicit refusal", err)
			}
		})
	}
}

// The other direction of the same audit: our privileges arriving at somebody
// else. A stranger granted a product role does SET ROLE into it and holds every
// grant the product role holds — no cadence_* connection required.
func TestChainRefusesAMembershipOutOfAProductRole(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	admin := testsupport.Connect(t, db.SuperuserURL)
	if _, err := admin.Exec(ctx, `CREATE ROLE analyst_probe NOLOGIN`); err != nil {
		t.Fatalf("creating the outsider: %v", err)
	}
	if _, err := admin.Exec(ctx, `GRANT `+testsupport.PatientRole+` TO analyst_probe`); err != nil {
		t.Fatalf("granting the product role to the outsider: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx := context.WithoutCancel(ctx)
		if _, err := admin.Exec(cleanupCtx, `DROP ROLE IF EXISTS analyst_probe`); err != nil {
			t.Errorf("dropping the outsider: %v", err)
		}
	})

	statements, err := os.ReadFile(filepath.Join(testsupport.MigrationsPath(t), "000001_base.up.sql"))
	if err != nil {
		t.Fatalf("reading the base migration: %v", err)
	}

	conn := testsupport.Connect(t, db.MigrationURL)
	_, err = conn.Exec(ctx, string(statements))
	if err == nil {
		t.Fatal("the chain applied while an outsider held a product role, and said nothing")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || !strings.Contains(pgErr.Message, "did not grant") {
		t.Fatalf("the chain failed with %v, want the explicit refusal", err)
	}
}

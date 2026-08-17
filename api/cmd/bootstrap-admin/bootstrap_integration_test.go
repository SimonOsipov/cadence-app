//go:build integration

// The command against a real chain, under the role a deployment runs it as: the two rows are permitted by 000009's
// owner policies and by nothing else, and the check that keeps a second administrator out is a SELECT the owner can
// only make through profiles_hook_read. None of that is observable against a fake.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// insufficientPrivilege is what a role that may not act at all is refused with, as against a policy finding no row.
const insufficientPrivilege = "42501"

var cluster *testsupport.Cluster

// os.Exit runs no deferred function and a panicking test never returns through m.Run, so the teardown lives in a
// function of its own: without it a panic leaves the containers running, and TESTCONTAINERS_RYUK_DISABLED means
// nothing else reaps them.
func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

func runSuite(m *testing.M) int {
	ctx := context.Background()

	started, err := testsupport.StartCluster(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting the test cluster: %v\n", err)

		return 1
	}
	cluster = started

	defer func() {
		if err := cluster.Terminate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "terminating the test cluster: %v\n", err)
		}
	}()

	return m.Run()
}

// bootstrap runs the command as an operator does: the real writer, the URL in the environment, typed arguments.
func bootstrap(t *testing.T, db *testsupport.Database, userID, fullName string) error {
	t.Helper()

	t.Setenv("DATABASE_MIGRATION_URL", db.MigrationURL)

	return run(t.Context(), []string{userID, fullName}, writeTheFirstAdministrator)
}

// The witness the command cannot be: the owner reads no audit row and sees profiles only through the hook's policy.
func observer(t *testing.T, db *testsupport.Database) *pgx.Conn {
	t.Helper()

	return testsupport.Connect(t, db.SuperuserURL)
}

func TestTheCommandCreatesTheClinicsFirstAdministrator(t *testing.T) {
	db := cluster.NewDatabase(t)

	if err := bootstrap(t, db, adminID, adminName); err != nil {
		t.Fatalf("bootstrapping the first administrator: %v", err)
	}

	var (
		role     string
		name     string
		timezone *string
		locale   string
	)
	if err := observer(t, db).QueryRow(t.Context(), `
		SELECT role, full_name, timezone, locale FROM app.profiles WHERE user_id = $1
	`, adminID).Scan(&role, &name, &timezone, &locale); err != nil {
		t.Fatalf("reading the administrator's profile: %v", err)
	}

	if role != "admin" {
		t.Errorf("the profile is a %s, want an admin", role)
	}
	if name != adminName {
		t.Errorf("the profile is named %q, want %q", name, adminName)
	}
	// Empty until the first sign-in captures it; a value invented here would be a guess at somebody's clock.
	if timezone != nil {
		t.Errorf("the profile carries the timezone %q, want none yet", *timezone)
	}
	if locale != "ru" {
		t.Errorf("the profile is in %q, want the column's own default", locale)
	}
}

// Signed by a job and never by a person — 000009's answer to an author that can TRUNCATE the table.
func TestTheCommandSignsItsAuditRowAsAJob(t *testing.T) {
	db := cluster.NewDatabase(t)

	if err := bootstrap(t, db, adminID, adminName); err != nil {
		t.Fatalf("bootstrapping the first administrator: %v", err)
	}

	var (
		actorID  *string
		actorJob string
		action   string
		entity   string
		entityID string
	)
	// Filtered on the account rather than taken as the only row: unfiltered this is right only while
	// the database is fresh and the command wrote exactly once, neither of which the query states.
	if err := observer(t, db).QueryRow(t.Context(), `
		SELECT actor_id, actor_job, action, entity, entity_id::text
		FROM app.audit_log WHERE entity_id = $1
	`, adminID).Scan(&actorID, &actorJob, &action, &entity, &entityID); err != nil {
		t.Fatalf("reading the audit row: %v", err)
	}

	if actorID != nil {
		t.Errorf("the audit row names the person %q", *actorID)
	}
	// Spelled out rather than compared against the constant that produced it: measured, renaming auditJob left an
	// expectation derived from it green.
	if actorJob != "bootstrap-admin" {
		t.Errorf("the audit row is signed %q, want bootstrap-admin", actorJob)
	}
	if action != "admin.bootstrap" || entity != "profiles" {
		t.Errorf("the audit row says %s on %s", action, entity)
	}
	if entityID != adminID {
		t.Errorf("the audit row is about %s, want the administrator %s", entityID, adminID)
	}
}

// Nothing is unique on role, so a command that skipped the check answers no error and leaves two administrators.
func TestTheCommandRefusesASecondAdministratorAndWritesNothing(t *testing.T) {
	db := cluster.NewDatabase(t)

	if err := bootstrap(t, db, adminID, adminName); err != nil {
		t.Fatalf("bootstrapping the first administrator: %v", err)
	}

	err := bootstrap(t, db, otherAdminID, otherAdminName)
	if err == nil {
		t.Fatal("a second administrator was created")
	}
	if !errors.Is(err, errAdministratorExists) {
		t.Fatalf("the second run failed with %v, want the refusal", err)
	}

	conn := observer(t, db)

	var profiles, audited int
	if err := conn.QueryRow(t.Context(), `
		SELECT (SELECT count(*) FROM app.profiles), (SELECT count(*) FROM app.audit_log)
	`).Scan(&profiles, &audited); err != nil {
		t.Fatalf("counting what the two runs wrote: %v", err)
	}

	if profiles != 1 {
		t.Errorf("the clinic has %d profiles, want the one the first run wrote", profiles)
	}
	if audited != 1 {
		t.Errorf("the audit log holds %d rows, want the one the first run signed", audited)
	}
}

// audit.md's second invariant, measured rather than described: only one transaction over both inserts makes an
// administrator without the row that signs them impossible. The audit insert is failed by taking its policy away —
// committed separately, the profile survives and the clinic has an administrator nobody signed for.
func TestAnAdministratorIsNotCreatedWithoutTheRowThatSignsThem(t *testing.T) {
	db := cluster.NewDatabase(t)
	superuser := observer(t, db)

	if _, err := superuser.Exec(
		t.Context(), `DROP POLICY audit_log_bootstrap_insert ON app.audit_log`,
	); err != nil {
		t.Fatalf("taking the audit policy away: %v", err)
	}

	err := bootstrap(t, db, adminID, adminName)
	if err == nil {
		t.Fatal("the command reported success with no audit row to show for it")
	}
	// Where it failed and not merely that it did: without this the test is green on a command that never got that far.
	if !strings.Contains(err.Error(), "signing the audit row") {
		t.Fatalf("the run failed with %v, want the audit insert to be what refused", err)
	}

	var profiles int
	if err := superuser.QueryRow(
		t.Context(), `SELECT count(*) FROM app.profiles`,
	).Scan(&profiles); err != nil {
		t.Fatalf("counting the profiles: %v", err)
	}
	if profiles != 0 {
		t.Errorf("%d profile(s) survived the failed audit row", profiles)
	}
}

// The shape a migration role may well be provisioned in: the chain never relies on inheritance, opening with SET ROLE.
func noInheritMemberOfTheOwner(t *testing.T, db *testsupport.Database) string {
	t.Helper()

	// Roles are cluster objects, so the per-test database name is what keeps two runs from colliding over this one.
	role := "cadence_bootstrap_" + db.Name
	const password = "cadence_bootstrap"

	superuser := testsupport.Connect(t, db.SuperuserURL)
	for _, statement := range []string{
		fmt.Sprintf(`CREATE ROLE %q LOGIN NOINHERIT NOSUPERUSER NOBYPASSRLS PASSWORD '%s'`, role, password),
		fmt.Sprintf(`GRANT %s TO %q`, testsupport.OwnerRole, role),
	} {
		if _, err := superuser.Exec(t.Context(), statement); err != nil {
			t.Fatalf("arranging the NOINHERIT migration role: %v", err)
		}
	}

	t.Cleanup(func() {
		if _, err := superuser.Exec(
			context.WithoutCancel(t.Context()), fmt.Sprintf(`DROP ROLE %q`, role),
		); err != nil {
			t.Errorf("dropping the NOINHERIT migration role: %v", err)
		}
	})

	parsed, err := url.Parse(db.SuperuserURL)
	if err != nil {
		t.Fatalf("parsing the harness connection string: %v", err)
	}
	parsed.User = url.UserPassword(role, password)

	return parsed.String()
}

// The test that makes the command's SET ROLE load-bearing: measured, removing it leaves every other test here green,
// because the harness's migration role inherits.
func TestTheCommandWorksUnderAMigrationRoleThatInheritsNothing(t *testing.T) {
	db := cluster.NewDatabase(t)

	t.Setenv("DATABASE_MIGRATION_URL", noInheritMemberOfTheOwner(t, db))

	if err := run(t.Context(), []string{adminID, adminName}, writeTheFirstAdministrator); err != nil {
		t.Fatalf("bootstrapping under a migration role that inherits nothing: %v", err)
	}

	var role string
	if err := observer(t, db).QueryRow(
		t.Context(), `SELECT role FROM app.profiles WHERE user_id = $1`, adminID,
	).Scan(&role); err != nil {
		t.Fatalf("reading the administrator's profile: %v", err)
	}
	if role != "admin" {
		t.Errorf("the profile is a %s, want an admin", role)
	}
}

// The refusal must be the privilege one: cadence_app cannot become the owner, the barrier the access model rests on.
func TestTheCommandCannotDoThisFromTheRequestPath(t *testing.T) {
	db := cluster.NewDatabase(t)

	t.Setenv("DATABASE_MIGRATION_URL", db.AppURL)

	err := run(t.Context(), []string{adminID, adminName}, writeTheFirstAdministrator)
	if err == nil {
		t.Fatal("the request path created an administrator")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != insufficientPrivilege {
		t.Fatalf("the request path was refused with %v, want SQLSTATE %s",
			err, insufficientPrivilege)
	}

	var profiles int
	if err := observer(t, db).QueryRow(
		t.Context(), `SELECT count(*) FROM app.profiles`,
	).Scan(&profiles); err != nil {
		t.Fatalf("counting the profiles: %v", err)
	}
	if profiles != 0 {
		t.Errorf("the request path wrote %d profile(s)", profiles)
	}
}

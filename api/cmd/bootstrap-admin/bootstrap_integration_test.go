//go:build integration

// The command against a real chain, under the role a deployment runs it as.
//
// Everything it does is the arrangement's rather than this package's: the two
// rows are permitted by 000009's owner policies and by nothing else, and the
// refusal that keeps a second administrator out is a SELECT the owner is only
// able to make through profiles_hook_read. None of that is observable against a
// fake.
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

// insufficientPrivilege is what Postgres answers when a role may not do
// something at all, as opposed to a policy finding no row for it.
const insufficientPrivilege = "42501"

var cluster *testsupport.Cluster

func TestMain(m *testing.M) {
	ctx := context.Background()

	started, err := testsupport.StartCluster(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting the test cluster: %v\n", err)
		os.Exit(1)
	}
	cluster = started

	code := m.Run()

	if err := cluster.Terminate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "terminating the test cluster: %v\n", err)
	}

	os.Exit(code)
}

// bootstrap runs the command the way an operator does: the real writer, the
// connection string in the environment, and the arguments a person types.
func bootstrap(t *testing.T, db *testsupport.Database, userID, fullName string) error {
	t.Helper()

	t.Setenv("DATABASE_MIGRATION_URL", db.MigrationURL)

	return run(t.Context(), []string{userID, fullName}, writeTheFirstAdministrator)
}

// The witness the command cannot be: the owner reads no audit row at all and
// sees profiles only through the hook's policy, so what actually landed is
// asked of a connection outside the access model.
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
	// Empty until the first sign-in, which is where the block that captures it
	// writes it. A value invented here would be a guess at somebody's clock.
	if timezone != nil {
		t.Errorf("the profile carries the timezone %q, want none yet", *timezone)
	}
	if locale != "ru" {
		t.Errorf("the profile is in %q, want the column's own default", locale)
	}
}

// The row is signed by a job and never by a person. The role that writes it can
// TRUNCATE the table, and 000009's answer to that weaker author is to keep it
// from writing a row that reads as a human's.
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
	if err := observer(t, db).QueryRow(t.Context(), `
		SELECT actor_id, actor_job, action, entity, entity_id::text FROM app.audit_log
	`).Scan(&actorID, &actorJob, &action, &entity, &entityID); err != nil {
		t.Fatalf("reading the audit row: %v", err)
	}

	if actorID != nil {
		t.Errorf("the audit row names the person %q", *actorID)
	}
	// Spelled out rather than compared against the constant that produced it:
	// an expectation derived from the value under test moves with it, and
	// measured — renaming auditJob left this test green.
	if actorJob != "bootstrap-admin" {
		t.Errorf("the audit row is signed %q, want bootstrap-admin", actorJob)
	}
	// The bootstrap's own action rather than the staff route's provider.create:
	// the row is about an administrator, and step-6 writes that other string for
	// every doctor.
	if action != "admin.bootstrap" || entity != "profiles" {
		t.Errorf("the audit row says %s on %s", action, entity)
	}
	if entityID != adminID {
		t.Errorf("the audit row is about %s, want the administrator %s", entityID, adminID)
	}
}

// The refusal, and what makes it worth asserting: nothing on this table is
// unique on role, so a command that skipped the check would answer no error at
// all and leave a clinic with two administrators.
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

// audit.md's second invariant, measured rather than described: an administrator
// without the row that signs them must be impossible, and the only thing making
// it so is that both inserts are one transaction. The audit insert is made to
// fail by taking its policy away — with the two writes committed separately, the
// profile survives this and the clinic has an administrator nobody signed for.
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
	// Where it failed and not merely that it did: without this the test is green
	// against a command that never reached the audit insert at all.
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

// noInheritMemberOfTheOwner returns a connection string for a login role that
// belongs to cadence_owner and inherits nothing from it — the shape a migration
// role may well be provisioned in, since the chain itself never relies on
// inheritance and opens every migration with SET ROLE.
func noInheritMemberOfTheOwner(t *testing.T, db *testsupport.Database) string {
	t.Helper()

	// The database name is unique per test and roles are cluster objects, so
	// two tests running one after the other do not collide over this one.
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

// Why the command assumes the owner role rather than counting on the migration
// role's membership carrying it. Measured: with the SET ROLE removed everything
// else here stays green, because the harness's migration role inherits — and
// this is the arrangement in which that difference is the whole command.
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

// The command belongs to the migration role. Pointed at the request path's
// connection string it must refuse rather than write anything, and the refusal
// has to be the privilege one: cadence_app cannot become the owner, which is the
// barrier the whole access model rests on.
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

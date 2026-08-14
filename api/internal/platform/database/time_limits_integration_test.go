//go:build integration

package database_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// The settings are read back in milliseconds rather than as text: pg_settings
// holds every GUC in its base unit, so 5000 there is the number the migration
// and the connection string both write, while SHOW would answer "5s" and make
// the comparison a question about spelling.
func effectiveMilliseconds(t *testing.T, conn *pgx.Conn, setting string) int64 {
	t.Helper()

	var value int64
	if err := conn.QueryRow(
		t.Context(), `SELECT setting::bigint FROM pg_settings WHERE name = $1`, setting,
	).Scan(&value); err != nil {
		t.Fatalf("reading %s: %v", setting, err)
	}

	return value
}

func roleSettings(t *testing.T, admin *pgx.Conn, role string) []string {
	t.Helper()

	var settings []string
	if err := admin.QueryRow(t.Context(), `
		SELECT coalesce(s.setconfig, '{}')
		FROM pg_roles r
		LEFT JOIN pg_db_role_setting s ON s.setrole = r.oid AND s.setdatabase = 0
		WHERE r.rolname = $1
	`, role).Scan(&settings); err != nil {
		t.Fatalf("reading the settings of %s: %v", role, err)
	}

	return settings
}

// The limits belong to the role that logs in, and the reason is a property of
// Postgres rather than a preference: a role's settings are applied at login,
// from session_user. SET ROLE does not adopt the target's defaults.
//
// Written against cadence_service — the impersonation target, which is the
// tempting place to put them, since it is the role the work actually runs as —
// the two lines would be a limit nothing ever applies: present in the
// catalogue, absent from every session. That half is measured here by planting
// one and watching it not arrive.
func TestTheServicePathLimitsSitOnTheRoleThatLogsIn(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()
	admin := testsupport.Connect(t, db.SuperuserURL)

	expected := map[string]int64{
		"statement_timeout":                   database.ServiceStatementTimeout.Milliseconds(),
		"idle_in_transaction_session_timeout": database.ServiceIdleInTransactionTimeout.Milliseconds(),
	}

	t.Run("the chain sets them on the role the service pool logs in as", func(t *testing.T) {
		settings := roleSettings(t, admin, testsupport.ServiceAppRole)

		for setting, value := range expected {
			entry := fmt.Sprintf("%s=%d", setting, value)
			if !slices.Contains(settings, entry) {
				t.Errorf("%s carries %v, want it to carry %q",
					testsupport.ServiceAppRole, settings, entry)
			}
		}
	})

	t.Run("and not on the role the service path becomes", func(t *testing.T) {
		for _, entry := range roleSettings(t, admin, testsupport.ServiceRole) {
			for setting := range expected {
				if strings.HasPrefix(entry, setting+"=") {
					t.Errorf("%s carries %q, which no session ever reads: SET ROLE does not "+
						"apply the target role's settings", testsupport.ServiceRole, entry)
				}
			}
		}
	})

	t.Run("a session carries them before and after it becomes the service role", func(t *testing.T) {
		// Planted on the impersonation target for the duration of this subtest, so
		// that "SET ROLE does not adopt them" is measured rather than asserted from
		// the documentation. Roles are cluster objects and the tests in this binary
		// share one, hence the cleanup.
		planted := int64(1234)
		if _, err := admin.Exec(
			ctx, fmt.Sprintf(
				`ALTER ROLE %s SET statement_timeout = %d`, testsupport.ServiceRole, planted,
			),
		); err != nil {
			t.Fatalf("planting a setting on %s: %v", testsupport.ServiceRole, err)
		}

		t.Cleanup(func() {
			if _, err := admin.Exec(
				context.WithoutCancel(ctx), fmt.Sprintf(
					`ALTER ROLE %s RESET statement_timeout`, testsupport.ServiceRole,
				),
			); err != nil {
				t.Errorf("taking the planted setting off %s: %v", testsupport.ServiceRole, err)
			}
		})

		// The harness's own connection string, which carries no runtime parameters
		// — so what this session runs under is the role's default and nothing else.
		conn := testsupport.Connect(t, db.ServiceAppURL)

		for setting, want := range expected {
			if got := effectiveMilliseconds(t, conn, setting); got != want {
				t.Errorf("a fresh service session runs under %s = %dms, want %dms", setting, got, want)
			}
		}

		if _, err := conn.Exec(ctx, fmt.Sprintf(`SET ROLE %s`, testsupport.ServiceRole)); err != nil {
			t.Fatalf("becoming %s: %v", testsupport.ServiceRole, err)
		}

		got := effectiveMilliseconds(t, conn, "statement_timeout")
		if got == planted {
			t.Errorf("statement_timeout became %dms on SET ROLE %s: the limits would belong on "+
				"the impersonation target after all, and this whole migration is on the wrong role",
				planted, testsupport.ServiceRole)
		}
		if want := expected["statement_timeout"]; got != want {
			t.Errorf("after SET ROLE %s the session runs under statement_timeout = %dms, want %dms",
				testsupport.ServiceRole, got, want)
		}
	})
}

// The pool's own copy, measured where the role's default is not there to supply
// it: a database the migration never reached — a restored dump, a cluster
// somebody rebuilt — still gets a bounded service path.
func TestTheServicePoolCarriesItsLimitsWhereTheRoleDoesNot(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()
	admin := testsupport.Connect(t, db.SuperuserURL)

	expected := map[string]int64{
		"statement_timeout":                   database.ServiceStatementTimeout.Milliseconds(),
		"idle_in_transaction_session_timeout": database.ServiceIdleInTransactionTimeout.Milliseconds(),
	}

	for setting := range expected {
		if _, err := admin.Exec(
			ctx, fmt.Sprintf(
				`ALTER ROLE %s RESET %s`, testsupport.ServiceAppRole, setting,
			),
		); err != nil {
			t.Fatalf("taking %s off %s: %v", setting, testsupport.ServiceAppRole, err)
		}
	}

	// Put back for the tests that run after this one: the roles are cluster
	// objects, and the next NewDatabase reapplying the chain is not a thing to
	// depend on.
	t.Cleanup(func() {
		cleanupCtx := context.WithoutCancel(ctx)
		for setting, value := range expected {
			if _, err := admin.Exec(
				cleanupCtx, fmt.Sprintf(
					`ALTER ROLE %s SET %s = %d`, testsupport.ServiceAppRole, setting, value,
				),
			); err != nil {
				t.Errorf("restoring %s on %s: %v", setting, testsupport.ServiceAppRole, err)
			}
		}
	})

	// The control: without the role's default, a plain connection is unbounded.
	// Without it a pool that carried nothing would pass on the role setting this
	// test has just removed.
	plain := testsupport.Connect(t, db.ServiceAppURL)
	for setting := range expected {
		if got := effectiveMilliseconds(t, plain, setting); got != 0 {
			t.Fatalf("%s is %dms on a plain connection with the role's default removed, "+
				"want the cluster default of 0 — this test measured the role setting, not the pool",
				setting, got)
		}
	}

	pool, err := database.NewServicePool(ctx, db.ServiceAppURL)
	if err != nil {
		t.Fatalf("opening the service pool: %v", err)
	}
	defer pool.Close()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquiring a connection: %v", err)
	}
	defer conn.Release()

	for setting, want := range expected {
		if got := effectiveMilliseconds(t, conn.Conn(), setting); got != want {
			t.Errorf("the service pool's connection runs under %s = %dms, want %dms",
				setting, got, want)
		}
	}
}

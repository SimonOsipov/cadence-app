package database

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The service path's two limits bound one transaction from different sides, and
// which of them fires first decides what the process loses.
//
// statement_timeout aborts the statement: SQLSTATE 57014, the connection stays,
// and the caller gets an error naming what happened. The idle-in-transaction
// limit terminates the session — the pool loses a connection and the whole
// transaction with it. The recoverable limit has to be the tighter one, or the
// ordinary answer to a slow query is a dropped connection and the operator
// learns nothing about which query it was.
//
// Asserted rather than written in a comment beside the constants, because a
// comment does not fail when somebody raises one number.
func TestTheServicePathLimitsAreOrdered(t *testing.T) {
	if ServiceStatementTimeout >= ServiceIdleInTransactionTimeout {
		t.Errorf("statement_timeout is %s and idle_in_transaction_session_timeout is %s: "+
			"a statement may outlive the limit that terminates the session, so a slow query "+
			"costs a connection rather than an answer",
			ServiceStatementTimeout, ServiceIdleInTransactionTimeout)
	}
}

// The connection string and migration 000007 carry the same two numbers, and
// nothing but this makes them move together.
//
// Both are load-bearing and neither replaces the other: the role setting covers
// every session that logs in as cadence_service_app, including one nobody wrote
// — a psql session, a job somebody ran by hand — while the connection string
// covers this pool even on a database the migration never reached, a restored
// dump among them.
func TestTheMigrationSetsTheNumbersTheServicePoolConnectsUnder(t *testing.T) {
	sql, err := os.ReadFile(filepath.Join(migrationsDir(t), "000007_service_path_time_limits.up.sql"))
	if err != nil {
		t.Fatalf("reading the migration: %v", err)
	}

	params := serviceRuntimeParams()
	if len(params) == 0 {
		t.Fatal("the service pool carries no runtime parameters, so this compared nothing")
	}

	for setting, value := range params {
		want := fmt.Sprintf("ALTER ROLE %s SET %s = %s;", serviceAppRole, setting, value)
		if !strings.Contains(string(sql), want) {
			t.Errorf("the migration does not carry %q, so the role's default and the pool's "+
				"connection string disagree about %s", want, setting)
		}
	}
}

// The pool the service path connects on is not the request path's with another
// URL: it carries both limits and a connection count somebody chose.
func TestTheServicePoolIsBoundedInTimeAndInConnections(t *testing.T) {
	// The URL names a connection count of its own, and it is not decoration:
	// pgxpool's default is max(4, NumCPU), which on a four-core runner is the
	// value asserted below — so with a number nobody set, deleting the assignment
	// would be green in CI and red only on a bigger machine. Seven is nothing this
	// package would ever choose.
	cfg, err := serviceConfig("postgres://cadence_service_app@localhost:5432/cadence?pool_max_conns=7")
	if err != nil {
		t.Fatalf("building the service pool configuration: %v", err)
	}

	for setting, want := range serviceRuntimeParams() {
		if got := cfg.ConnConfig.RuntimeParams[setting]; got != want {
			t.Errorf("the service pool connects with %s = %q, want %q", setting, got, want)
		}
	}

	// pgx defaults MaxConns to max(4, NumCPU): a number nobody chose, which grows
	// with whatever machine the process lands on. The service path is the narrow
	// one — the writes no request may make — and the one whose policies carry no
	// row predicate, so how many of them run at once is a decision.
	if cfg.MaxConns != serviceMaxConns {
		t.Errorf("the service pool holds up to %d connections, want %d", cfg.MaxConns, serviceMaxConns)
	}
}

// migrationsDir finds api/migrations by walking up to the module root, the same
// way the integration harness does. Written out here rather than taken from that
// harness because the harness is behind the integration build tag and this test
// belongs in the fast gate: it reads a file and needs no database.
func migrationsDir(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("reading the working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "migrations")
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found above the working directory")
		}
		dir = parent
	}
}

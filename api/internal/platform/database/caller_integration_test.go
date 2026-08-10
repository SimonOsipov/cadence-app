//go:build integration

package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// requestPool opens the pool a deployment actually serves requests from: the
// low-privilege role in DATABASE_URL. Every assertion below has to be made
// against that role, or it is an assertion about a connection nobody uses.
func requestPool(t *testing.T) (*pgxpool.Pool, *testsupport.Database) {
	t.Helper()

	db := cluster.NewDatabase(t)

	pool, err := database.NewPool(t.Context(), db.AppURL)
	if err != nil {
		t.Fatalf("opening the request-path pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool, db
}

func testCaller() database.Caller {
	return database.Caller{
		Subject: "8a1f3b7c-0000-4000-8000-000000000001",
		Role:    "authenticated",
	}
}

// TestWithCallerImpersonates is the seam's reason to exist. Inside the closure
// the transaction must be running as cadence_authenticated — the role the RLS
// policies of M2 are written for — and not as the role that connected.
func TestWithCallerImpersonates(t *testing.T) {
	pool, _ := requestPool(t)

	var current, session string
	err := database.WithCaller(t.Context(), pool, testCaller(), func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SELECT current_user, session_user").Scan(&current, &session)
	})
	if err != nil {
		t.Fatalf("WithCaller: %v", err)
	}

	if current != testsupport.AuthenticatedRole {
		t.Errorf("current_user inside the seam = %q, want %q", current, testsupport.AuthenticatedRole)
	}
	// session_user does not change with SET ROLE, and saying so here keeps the
	// assertion above honest: it is impersonation, not a different connection.
	if session != testsupport.AppRole {
		t.Errorf("session_user inside the seam = %q, want %q", session, testsupport.AppRole)
	}
}

// TestWithCallerPublishesTheClaims: a policy cannot filter by a caller it
// cannot read. The claims are what auth.uid() and every M2 policy will consult.
func TestWithCallerPublishesTheClaims(t *testing.T) {
	pool, _ := requestPool(t)
	caller := testCaller()

	var subject, role string
	err := database.WithCaller(t.Context(), pool, caller, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT current_setting('request.jwt.claims', true)::json ->> 'sub',
			       current_setting('request.jwt.claims', true)::json ->> 'role'
		`).Scan(&subject, &role)
	})
	if err != nil {
		t.Fatalf("WithCaller: %v", err)
	}

	if subject != caller.Subject {
		t.Errorf("sub claim = %q, want %q", subject, caller.Subject)
	}
	if role != caller.Role {
		t.Errorf("role claim = %q, want %q", role, caller.Role)
	}
}

// TestWithCallerResetsAfterwards is the half that a naive implementation gets
// wrong. Connections are pooled, so a role or a claim that outlives its
// transaction is served to the next request — as somebody else's identity.
func TestWithCallerResetsAfterwards(t *testing.T) {
	pool, _ := requestPool(t)

	var inside uint32
	if err := database.WithCaller(t.Context(), pool, testCaller(), func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&inside)
	}); err != nil {
		t.Fatalf("WithCaller: %v", err)
	}

	assertConnectionIsClean(t, pool, inside)
}

// TestWithCallerResetsAfterAFailure: the reset above must not depend on the
// happy path. A closure that fails is the case where a leaked identity would go
// unnoticed the longest.
func TestWithCallerResetsAfterAFailure(t *testing.T) {
	pool, _ := requestPool(t)
	sentinel := errors.New("the closure refused")

	var inside uint32
	if err := database.WithCaller(t.Context(), pool, testCaller(), func(ctx context.Context, tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&inside); err != nil {
			return err
		}

		return sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatalf("WithCaller = %v, want it to wrap the closure's error", err)
	}

	assertConnectionIsClean(t, pool, inside)
}

// TestWithCallerResetsAfterAPanic. A panicking closure is the only path where
// nothing in the code returns — the guarantee rests entirely on the deferred
// rollback. That is exactly the kind of promise that stops being true when
// somebody restructures the function around it, so it is asserted rather than
// reasoned about.
func TestWithCallerResetsAfterAPanic(t *testing.T) {
	pool, _ := requestPool(t)

	var inside uint32

	func() {
		defer func() {
			if recovered := recover(); recovered == nil {
				t.Error("the panic did not propagate out of WithCaller")
			}
		}()

		_ = database.WithCaller(t.Context(), pool, testCaller(), func(ctx context.Context, tx pgx.Tx) error {
			if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&inside); err != nil {
				return err
			}

			panic("the closure came apart")
		})
	}()

	assertConnectionIsClean(t, pool, inside)
}

// assertConnectionIsClean requires that the connection the seam ran on carries
// neither the impersonated role nor the claims once the seam is over — and that
// it really is the same connection, because a replaced one proves nothing.
func assertConnectionIsClean(t *testing.T, pool *pgxpool.Pool, inside uint32) {
	t.Helper()

	var current string
	var claims *string
	var after uint32
	if err := pool.QueryRow(
		t.Context(),
		"SELECT current_user, current_setting('request.jwt.claims', true), pg_backend_pid()",
	).Scan(&current, &claims, &after); err != nil {
		t.Fatalf("querying after the seam: %v", err)
	}

	if after != inside {
		t.Fatalf("the seam ran on backend %d and the check ran on %d — "+
			"the connection was replaced, so nothing here was proven", inside, after)
	}
	if current != testsupport.AppRole {
		t.Errorf("current_user after the seam = %q, want %q — the role outlived its transaction",
			current, testsupport.AppRole)
	}
	if claims != nil && *claims != "" {
		t.Errorf("claims after the seam = %q, want none — they outlived their transaction", *claims)
	}
}

// TestWithCallerRollsBackOnFailure: the seam is a transaction, so a closure
// that fails half way leaves nothing behind.
func TestWithCallerRollsBackOnFailure(t *testing.T) {
	pool, db := requestPool(t)

	// Owned by the owner, in the application schema, so the impersonated role
	// reaches it exactly the way an M2 table will — through the default
	// privileges the base migration set, not through a grant written here.
	createAppTable(t, db, "rollback_probe")

	sentinel := errors.New("the closure refused")
	err := database.WithCaller(t.Context(), pool, testCaller(), func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "INSERT INTO app.rollback_probe (note) VALUES ('written')"); err != nil {
			return err
		}

		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("WithCaller = %v, want it to wrap the closure's error", err)
	}

	var rows int
	if err := database.WithCaller(t.Context(), pool, testCaller(), func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SELECT count(*) FROM app.rollback_probe").Scan(&rows)
	}); err != nil {
		t.Fatalf("counting after the rollback: %v", err)
	}

	if rows != 0 {
		t.Errorf("%d row(s) survived a failed closure, want 0", rows)
	}
}

// TestWithCallerCommitsOnSuccess is the other direction, and it is not
// symmetry for its own sake: a seam that always rolled back would pass every
// test above.
func TestWithCallerCommitsOnSuccess(t *testing.T) {
	pool, db := requestPool(t)
	createAppTable(t, db, "commit_probe")

	if err := database.WithCaller(t.Context(), pool, testCaller(), func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO app.commit_probe (note) VALUES ('written')")

		return err
	}); err != nil {
		t.Fatalf("WithCaller: %v", err)
	}

	var rows int
	if err := database.WithCaller(t.Context(), pool, testCaller(), func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SELECT count(*) FROM app.commit_probe").Scan(&rows)
	}); err != nil {
		t.Fatalf("counting after the commit: %v", err)
	}

	if rows != 1 {
		t.Errorf("%d row(s) after a successful closure, want 1", rows)
	}
}

// TestWithCallerIsNotAnInjectionVector. The subject arrives from a token — it
// is attacker-controlled input that has already passed a signature check, which
// says nothing about its contents. Composing the SET into a string is the
// obvious implementation and the wrong one.
func TestWithCallerIsNotAnInjectionVector(t *testing.T) {
	pool, db := requestPool(t)
	createAppTable(t, db, "injection_probe")

	hostile := []string{
		`'; DROP TABLE app.injection_probe; --`,
		`" ; RESET ROLE; SELECT '`,
		`'); SET ROLE cadence_owner; SELECT set_config('a','b',true`,
		`\'; SELECT 1; --`,
		`{"sub": "someone-else"}`,
		"line\nbreak\ttab",
		`"quoted"`,
		`наблюдение`,
	}

	for _, subject := range hostile {
		caller := database.Caller{Subject: subject, Role: "authenticated"}

		var readBack, current string
		err := database.WithCaller(t.Context(), pool, caller, func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `
				SELECT current_setting('request.jwt.claims', true)::json ->> 'sub', current_user
			`).Scan(&readBack, &current)
		})
		if err != nil {
			t.Errorf("WithCaller with subject %q: %v", subject, err)

			continue
		}

		// Read back verbatim: the value is data, so it neither executes nor is
		// mangled on the way in. A seam that escaped by rewriting would corrupt
		// the identity a policy compares against.
		if readBack != subject {
			t.Errorf("subject %q came back as %q", subject, readBack)
		}
		if current != testsupport.AuthenticatedRole {
			t.Errorf("subject %q changed the effective role to %q", subject, current)
		}
	}

	// The table the first payload tried to drop.
	var exists bool
	if err := pool.QueryRow(
		t.Context(),
		"SELECT to_regclass('app.injection_probe') IS NOT NULL",
	).Scan(&exists); err != nil {
		t.Fatalf("checking the probe table: %v", err)
	}
	if !exists {
		t.Error("a hostile claim value dropped the probe table")
	}
}

// TestWithCallerRefusesAnEmptySubject: a principal with no identity must not
// reach a policy, where an empty string would simply match nothing — or, in the
// wrong policy, everything.
func TestWithCallerRefusesAnEmptySubject(t *testing.T) {
	pool, _ := requestPool(t)

	err := database.WithCaller(t.Context(), pool, database.Caller{Role: "authenticated"},
		func(context.Context, pgx.Tx) error {
			t.Error("the closure ran for a caller with no subject")

			return nil
		})
	// A named error, not any error: a handler has to tell "this caller has no
	// identity" — which is a 401 — apart from "the database is unhappy", which
	// is a 500.
	if !errors.Is(err, database.ErrNoSubject) {
		t.Fatalf("WithCaller = %v, want it to wrap ErrNoSubject", err)
	}
}

// TestRequestPathCannotReachTablesWithoutTheSeam is what NOINHERIT buys. A
// query that skips the seam holds no table privileges at all and fails loudly,
// instead of running with the union of both roles' rights and no policy in
// sight — which is the failure that looks like everything working.
//
// The SQLSTATE is checked rather than the mere presence of an error, matching
// the rule the rest of this suite already follows: a typo in the table name
// would produce 42P01 and leave a green test that asserts nothing about
// privileges.
func TestRequestPathCannotReachTablesWithoutTheSeam(t *testing.T) {
	pool, db := requestPool(t)
	createAppTable(t, db, "unguarded_probe")

	var rows int
	err := pool.QueryRow(t.Context(), "SELECT count(*) FROM app.unguarded_probe").Scan(&rows)
	if err == nil {
		t.Fatal("the request path read an application table without impersonating")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("reading without the seam failed with %v, want SQLSTATE 42501 (insufficient_privilege)", err)
	}
}

// createAppTable adds a table the way a migration does: through the bootstrap
// role, under SET ROLE cadence_owner, in the application schema. The privileges
// the tests then exercise are therefore the ones the base migration granted by
// default — not ones this file arranged for its own convenience.
func createAppTable(t *testing.T, db *testsupport.Database, name string) {
	t.Helper()

	conn := testsupport.Connect(t, db.MigrationURL)

	if _, err := conn.Exec(t.Context(), "SET ROLE cadence_owner"); err != nil {
		t.Fatalf("becoming the owner: %v", err)
	}

	if _, err := conn.Exec(
		t.Context(),
		"CREATE TABLE app."+pgx.Identifier{name}.Sanitize()+" (id bigserial PRIMARY KEY, note text)",
	); err != nil {
		t.Fatalf("creating app.%s: %v", name, err)
	}
}

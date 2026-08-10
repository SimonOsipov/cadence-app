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
		Role:    "patient",
	}
}

// TestWithCallerImpersonates is the seam's reason to exist. Inside the closure
// the transaction must be running as the caller's product role — the role the
// grants and the policies are written for — and not as the role that connected.
func TestWithCallerImpersonates(t *testing.T) {
	pool, _ := requestPool(t)

	var current, session string
	err := database.WithCaller(t.Context(), pool, testCaller(), func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SELECT current_user, session_user").Scan(&current, &session)
	})
	if err != nil {
		t.Fatalf("WithCaller: %v", err)
	}

	if current != testsupport.PatientRole {
		t.Errorf("current_user inside the seam = %q, want %q", current, testsupport.PatientRole)
	}
	// session_user does not change with SET ROLE, and saying so here keeps the
	// assertion above honest: it is impersonation, not a different connection.
	if session != testsupport.AppRole {
		t.Errorf("session_user inside the seam = %q, want %q", session, testsupport.AppRole)
	}
}

// Each product role lands on its own Postgres role. A seam that mapped every
// caller onto one role would pass the test above and reintroduce exactly the
// indistinguishability the block exists to remove: a grant the admin needs
// would reach the patient.
func TestWithCallerImpersonatesEachProductRole(t *testing.T) {
	pool, _ := requestPool(t)

	for claim, want := range map[string]string{
		"patient": testsupport.PatientRole,
		"doctor":  testsupport.DoctorRole,
		"admin":   testsupport.AdminRole,
	} {
		t.Run(claim, func(t *testing.T) {
			caller := database.Caller{Subject: "8a1f3b7c-0000-4000-8000-000000000001", Role: claim}

			var current string
			if err := database.WithCaller(
				t.Context(), pool, caller,
				func(ctx context.Context, tx pgx.Tx) error {
					return tx.QueryRow(ctx, "SELECT current_user").Scan(&current)
				},
			); err != nil {
				t.Fatalf("WithCaller: %v", err)
			}

			if current != want {
				t.Errorf("claim %q impersonated %q, want %q", claim, current, want)
			}
		})
	}
}

// A role the map does not know is a refusal before the transaction opens. There
// is no default role to fall back to: falling back would mean a token with a
// role nobody recognizes gets served as somebody, and which somebody would be
// decided by whichever entry happened to be first.
func TestWithCallerRefusesAnUnknownRole(t *testing.T) {
	pool, _ := requestPool(t)

	// Four inputs, and none of them is arbitrary. "service" is a token claiming
	// the service path, which must not be reachable from the request pool at
	// all. "cadence_patient" is the Postgres name rather than the product name,
	// so the map is pinned to one vocabulary. The empty string is what a token
	// with no role at all produces, and "root" is the ordinary stranger.
	for _, role := range []string{"service", "cadence_patient", "", "root"} {
		t.Run("role="+role, func(t *testing.T) {
			caller := database.Caller{Subject: "8a1f3b7c-0000-4000-8000-000000000001", Role: role}

			before := pool.Stat().AcquireCount()

			err := database.WithCaller(t.Context(), pool, caller, func(context.Context, pgx.Tx) error {
				t.Error("the closure ran for a caller whose role has no Postgres role")

				return nil
			})
			if !errors.Is(err, database.ErrUnknownRole) {
				t.Fatalf("WithCaller = %v, want it to wrap ErrUnknownRole", err)
			}

			// Refused before Begin, not merely before the closure. Without this
			// the assertion above stays green with the lookup moved after
			// pool.Begin — which would take a connection and open a transaction
			// for a caller there was never a role to serve.
			if after := pool.Stat().AcquireCount(); after != before {
				t.Errorf("the pool was acquired %d time(s) for a caller with no role; "+
					"the refusal happens after Begin", after-before)
			}
		})
	}
}

// The residual property, asserted rather than argued: request.jwt.claims has
// USERSET context, so a query inside the seam can overwrite it — and what that
// buys is a different subject within one's own role, never a different role.
// This is the claim the base migration's preamble makes, and it names this test.
func TestWithCallerKeepsTheRoleWhenTheClaimsAreOverwritten(t *testing.T) {
	pool, _ := requestPool(t)

	var current, subject string
	err := database.WithCaller(t.Context(), pool, testCaller(), func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			SELECT set_config('request.jwt.claims',
			                  '{"sub":"11111111-0000-4000-8000-000000000002","role":"admin"}', true)
		`); err != nil {
			return err
		}

		return tx.QueryRow(ctx, `
			SELECT current_user, current_setting('request.jwt.claims', true)::json ->> 'sub'
		`).Scan(&current, &subject)
	})
	if err != nil {
		t.Fatalf("WithCaller: %v", err)
	}

	if current != testsupport.PatientRole {
		t.Errorf("current_user after overwriting the claims = %q, want %q — the product role is "+
			"read from the claims after all", current, testsupport.PatientRole)
	}

	// The other half, and it is the honest half: the substitution did happen. A
	// test asserting only that the role held would be green in a world where the
	// GUC could not be written at all, and would then be measuring nothing.
	if subject == testCaller().Subject {
		t.Error("the claims could not be overwritten, so this test proves nothing about what " +
			"overwriting them does")
	}
}

// The service path holds nothing without its own seam either. Its grants are
// wider than the request path's by design, which is exactly why a query that
// skips SET ROLE on that pool must fail rather than run as the connection role.
func TestServicePathCannotReachTablesWithoutTheSeam(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	createAppTable(t, db, "service_probe")

	owner := testsupport.Connect(t, db.MigrationURL)
	if _, err := owner.Exec(ctx, `SET ROLE cadence_owner`); err != nil {
		t.Fatalf("becoming the owner: %v", err)
	}
	if _, err := owner.Exec(
		ctx, `GRANT SELECT ON app.service_probe TO `+testsupport.ServiceRole,
	); err != nil {
		t.Fatalf("granting on the probe table: %v", err)
	}

	service := testsupport.Connect(t, db.ServiceAppURL)

	var rows int
	err := service.QueryRow(ctx, `SELECT count(*) FROM app.service_probe`).Scan(&rows)
	if err == nil {
		t.Fatal("the service path read an application table without impersonating")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("reading without the seam failed with %v, want SQLSTATE 42501", err)
	}

	if _, err := service.Exec(ctx, `SET ROLE `+testsupport.ServiceRole); err != nil {
		t.Fatalf("impersonating the service role: %v", err)
	}
	if err := service.QueryRow(ctx, `SELECT count(*) FROM app.service_probe`).Scan(&rows); err != nil {
		t.Fatalf("reading while impersonating the service role: %v", err)
	}
}

// TestWithCallerPublishesTheClaims: a policy cannot filter by a caller it
// cannot read. The claims are what auth.uid() and every M2 policy will consult.
func TestWithCallerPublishesTheClaims(t *testing.T) {
	pool, _ := requestPool(t)
	caller := testCaller()

	var subject, role string
	err := database.WithCaller(t.Context(), pool, caller, func(ctx context.Context, tx pgx.Tx) error {
		// cadence_role, not role. GoTrue writes the literal "authenticated" into
		// the stock claim for every user, so a policy or a seam reading that key
		// would be reading a value that means nothing about the product.
		return tx.QueryRow(ctx, `
			SELECT current_setting('request.jwt.claims', true)::json ->> 'sub',
			       current_setting('request.jwt.claims', true)::json ->> 'cadence_role'
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

	// Owned by the owner, in the application schema, and granted per role — the
	// way a table of M2 will arrive, now that there are no default privileges
	// for one to arrive by.
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

// hostileValues are the payloads a token could carry into a statement. They have
// passed a signature check, which says nothing about their contents.
//
// Neither field of a Caller can carry one any more — the subject must be a UUID
// and the role one of three names — so what they assert here is refusal. The
// pass-through property they used to assert, that a value published as data
// neither executes nor is rewritten, moved to the one field that still carries
// free text: the audit actor of the service seam.
func hostileValues() []string {
	return []string{
		`'; DROP TABLE app.injection_probe; --`,
		`" ; RESET ROLE; SELECT '`,
		`'); SET ROLE cadence_owner; SELECT set_config('a','b',true`,
		`\'; SELECT 1; --`,
		`{"sub": "someone-else"}`,
		"line\nbreak\ttab",
		`"quoted"`,
		`наблюдение`,
	}
}

// Neither field of a caller can carry a payload anywhere any more, and the two
// halves are refused for different reasons — which is the point: the subject is
// refused for its shape, the role for not being one of three names. Both refuse
// before a transaction opens.
func TestWithCallerRefusesHostileValuesInEitherField(t *testing.T) {
	pool, _ := requestPool(t)

	for _, value := range hostileValues() {
		before := pool.Stat().AcquireCount()

		subjectErr := database.WithCaller(
			t.Context(), pool, database.Caller{Subject: value, Role: "patient"},
			func(context.Context, pgx.Tx) error {
				t.Errorf("the closure ran for subject %q", value)

				return nil
			},
		)
		if !errors.Is(subjectErr, database.ErrInvalidSubject) {
			t.Errorf("subject %q: WithCaller = %v, want ErrInvalidSubject", value, subjectErr)
		}

		roleErr := database.WithCaller(
			t.Context(), pool,
			database.Caller{Subject: "8a1f3b7c-0000-4000-8000-000000000001", Role: value},
			func(context.Context, pgx.Tx) error {
				t.Errorf("the closure ran for role %q", value)

				return nil
			},
		)
		if !errors.Is(roleErr, database.ErrUnknownRole) {
			t.Errorf("role %q: WithCaller = %v, want ErrUnknownRole", value, roleErr)
		}

		if after := pool.Stat().AcquireCount(); after != before {
			t.Errorf("value %q took %d connection(s); both refusals belong before Begin",
				value, after-before)
		}
	}
}

// A subject the database would refuse to cast is refused here instead. The
// policies are written with the idiom that casts the claim to uuid, and a cast
// that fails raises 22P02 from inside a policy — a 500 where the honest answer
// is a refusal, and one no handler can attribute.
func TestWithCallerRefusesASubjectThatIsNotAUUID(t *testing.T) {
	pool, _ := requestPool(t)

	for _, subject := range []string{
		"not-a-uuid",
		"8a1f3b7c-0000-4000-8000",
		"8a1f3b7c-0000-4000-8000-0000000000011",
		" 8a1f3b7c-0000-4000-8000-000000000001",
		// The axis every other fixture misses: right length, wrong separators.
		// pgx's parser splices the hex out of positions 8, 13, 18 and 23 without
		// looking at what sits between them, so this parses here and is refused
		// by uuid_in with 22P02 — from inside a policy, which is a 500 for what
		// is really a refusal.
		"8a1f3b7c$0000$4000$8000$000000000001",
		"8a1f3b7c00000-4000-8000-000000000001",
	} {
		err := database.WithCaller(
			t.Context(), pool, database.Caller{Subject: subject, Role: "patient"},
			func(context.Context, pgx.Tx) error { return nil },
		)
		if !errors.Is(err, database.ErrInvalidSubject) {
			t.Errorf("subject %q: WithCaller = %v, want ErrInvalidSubject", subject, err)
		}
	}

	// The control. Without it a validator that refused everything would pass.
	if err := database.WithCaller(
		t.Context(), pool, testCaller(), func(context.Context, pgx.Tx) error { return nil },
	); err != nil {
		t.Fatalf("a well-formed subject was refused: %v", err)
	}
}

// TestWithCallerRefusesAnEmptySubject: a principal with no identity must not
// reach a policy, where an empty string would simply match nothing — or, in the
// wrong policy, everything.
func TestWithCallerRefusesAnEmptySubject(t *testing.T) {
	pool, _ := requestPool(t)

	err := database.WithCaller(t.Context(), pool, database.Caller{Role: "patient"},
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
// role, under SET ROLE cadence_owner, in the application schema — and it issues
// the grants itself, per role, exactly as the table migrations of step-4 and
// step-5 will. There are no default privileges any more, so a table created
// without a grant is reachable by nobody and the tests below would all pass for
// the wrong reason.
func createAppTable(t *testing.T, db *testsupport.Database, name string) {
	t.Helper()

	conn := testsupport.Connect(t, db.MigrationURL)

	if _, err := conn.Exec(t.Context(), "SET ROLE cadence_owner"); err != nil {
		t.Fatalf("becoming the owner: %v", err)
	}

	table := "app." + pgx.Identifier{name}.Sanitize()
	sequence := "app." + pgx.Identifier{name + "_id_seq"}.Sanitize()

	if _, err := conn.Exec(
		t.Context(),
		"CREATE TABLE "+table+" (id bigserial PRIMARY KEY, note text)",
	); err != nil {
		t.Fatalf("creating %s: %v", table, err)
	}

	for _, role := range testsupport.ImpersonationRoles() {
		if _, err := conn.Exec(
			t.Context(),
			"GRANT SELECT, INSERT, UPDATE, DELETE ON "+table+" TO "+role,
		); err != nil {
			t.Fatalf("granting on %s to %s: %v", table, role, err)
		}

		// bigserial owns a sequence, and INSERT without USAGE on it fails with
		// 42501 — which would read as "the seam refused" in every test below.
		if _, err := conn.Exec(
			t.Context(),
			"GRANT USAGE ON SEQUENCE "+sequence+" TO "+role,
		); err != nil {
			t.Fatalf("granting the sequence of %s to %s: %v", table, role, err)
		}
	}
}

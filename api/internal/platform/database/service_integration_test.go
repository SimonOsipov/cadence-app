//go:build integration

package database_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// testProbe stands in for the reminder sweep and the invitation mailer: a path
// with no human behind it, which is the constructor most of the assertions here
// reach for because what they are about is the transaction rather than the actor.
const testProbe = "test.probe"

// servicePool connects as a second role, which is what makes the barrier
// between the paths something the server enforces rather than something the
// code remembers.
func servicePool(t *testing.T, db *testsupport.Database) *pgxpool.Pool {
	t.Helper()

	pool, err := database.NewPool(t.Context(), db.ServiceAppURL)
	if err != nil {
		t.Fatalf("opening the service-path pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

// TestWithServiceImpersonates is the service seam's reason to exist: the writes
// no policy lets a request make run as cadence_service, from a connection the
// request path cannot reach.
func TestWithServiceImpersonates(t *testing.T) {
	db := cluster.NewDatabase(t)
	pool := servicePool(t, db)

	var current, session string
	err := database.WithServiceJob(t.Context(), pool, testProbe, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SELECT current_user, session_user").Scan(&current, &session)
	})
	if err != nil {
		t.Fatalf("WithService: %v", err)
	}

	if current != testsupport.ServiceRole {
		t.Errorf("current_user inside the service seam = %q, want %q", current, testsupport.ServiceRole)
	}
	if session != testsupport.ServiceAppRole {
		t.Errorf("session_user inside the service seam = %q, want %q",
			session, testsupport.ServiceAppRole)
	}
}

// A service transaction has no caller.
//
// The connection is deliberately dirtied first, with a session-level setting on
// a single-connection pool, and that is what makes the assertion mean anything:
// a fresh transaction inherits nothing, so the same test against a clean
// connection stays green with the clearing statement deleted — it would be
// measuring Postgres rather than this seam. A setting that outlived its
// transaction is exactly the state the clearing defends against, and connections
// are pooled.
func TestWithServiceCarriesNoCallerClaims(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	cfg, err := pgxpool.ParseConfig(db.ServiceAppURL)
	if err != nil {
		t.Fatalf("parsing the service URL: %v", err)
	}
	cfg.MaxConns = 1

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("opening a single-connection service pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(
		ctx, `SELECT set_config('request.jwt.claims', $1, false)`,
		`{"sub":"8a1f3b7c-0000-4000-8000-000000000001","cadence_role":"admin"}`,
	); err != nil {
		t.Fatalf("dirtying the connection: %v", err)
	}

	// The control: the claims really are on the connection, so a green result
	// below is the seam clearing them rather than them never having been there.
	var planted *string
	if err := pool.QueryRow(
		ctx, `SELECT nullif(current_setting('request.jwt.claims', true), '')`,
	).Scan(&planted); err != nil {
		t.Fatalf("reading the planted claims: %v", err)
	}
	if planted == nil {
		t.Fatal("the claims were not planted, so this test proves nothing")
	}

	var claims *string
	if err := database.WithServiceJob(
		ctx, pool, testProbe,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(
				ctx, `SELECT nullif(current_setting('request.jwt.claims', true), '')`,
			).Scan(&claims)
		},
	); err != nil {
		t.Fatalf("WithServiceJob: %v", err)
	}

	if claims != nil {
		t.Errorf("the service transaction carries claims %q, want none — a caller's identity "+
			"outlived its transaction and was served to a system job", *claims)
	}
}

// Exactly one of the two attribution settings, always. audit_log's constraint
// requires it — a row with neither is unattributed and a row with both names two
// actors for one action — and the seam is where that becomes impossible rather
// than merely required.
func TestWithServicePublishesExactlyOneActor(t *testing.T) {
	const person = "8a1f3b7c-0000-4000-8000-000000000001"

	db := cluster.NewDatabase(t)
	pool := servicePool(t, db)

	read := func(ctx context.Context, tx pgx.Tx, id, job *string) error {
		// coalesce, because an unpublished setting reads as NULL rather than as
		// the empty string — and NULL is exactly what the audit_log constraint
		// wants to see in the other column.
		return tx.QueryRow(ctx, `
			SELECT coalesce(current_setting('app.actor_id', true), ''),
			       coalesce(current_setting('app.actor_job', true), '')
		`).Scan(id, job)
	}

	for name, run := range map[string]struct {
		open    func(*testing.T, *string, *string) error
		wantID  string
		wantJob string
	}{
		"a person": {
			open: func(t *testing.T, id, job *string) error {
				t.Helper()

				return database.WithService(
					requestOf(t, person), pool,
					func(ctx context.Context, tx pgx.Tx) error { return read(ctx, tx, id, job) },
				)
			},
			wantID: person,
		},
		"a job": {
			open: func(t *testing.T, id, job *string) error {
				t.Helper()

				return database.WithServiceJob(
					t.Context(), pool, testProbe,
					func(ctx context.Context, tx pgx.Tx) error { return read(ctx, tx, id, job) },
				)
			},
			wantJob: testProbe,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var id, job string
			if err := run.open(t, &id, &job); err != nil {
				t.Fatalf("opening the service seam: %v", err)
			}

			if id != run.wantID {
				t.Errorf("app.actor_id = %q, want %q", id, run.wantID)
			}
			if job != run.wantJob {
				t.Errorf("app.actor_job = %q, want %q", job, run.wantJob)
			}
		})
	}
}

// The one field of a service transaction that still carries free text is the
// job name, and it travels as a bound parameter. The payloads are the ones a
// token could have carried before both caller fields were validated; they are
// data here too, and data neither executes nor is rewritten on the way in.
func TestWithServiceIsNotAnInjectionVector(t *testing.T) {
	db := cluster.NewDatabase(t)
	pool := servicePool(t, db)

	createAppTable(t, db, "injection_probe")

	for _, name := range hostileValues() {
		var readBack, current string
		if err := database.WithServiceJob(
			t.Context(), pool, name,
			func(ctx context.Context, tx pgx.Tx) error {
				return tx.QueryRow(ctx, `
					SELECT current_setting('app.actor_job', true), current_user
				`).Scan(&readBack, &current)
			},
		); err != nil {
			t.Errorf("WithService with job %q: %v", name, err)

			continue
		}

		if readBack != name {
			t.Errorf("job %q came back as %q", name, readBack)
		}
		if current != testsupport.ServiceRole {
			t.Errorf("job %q changed the effective role to %q", name, current)
		}
	}

	var exists bool
	if err := pool.QueryRow(t.Context(), `
		SELECT EXISTS (
			SELECT FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = 'app' AND c.relname = 'injection_probe'
		)
	`).Scan(&exists); err != nil {
		t.Fatalf("checking the probe table: %v", err)
	}
	if !exists {
		t.Error("a hostile actor name dropped the probe table")
	}
}

// An actor is not optional, and a blank name is not one. Every service write is
// audited, and an audit row nobody signed answers "who changed this" with
// silence.
//
// The person-shaped half of this — no principal, a malformed subject — moved to
// TestWithServiceRefusesARequestWithNoVerifiedCaller when the subject stopped
// being an argument. What is left here is the job name, the one thing about an
// actor a caller still supplies.
func TestWithServiceRefusesAnActorNobodyChose(t *testing.T) {
	db := cluster.NewDatabase(t)
	pool := servicePool(t, db)

	// Whitespace is not a job name either. It passes the database's constraint —
	// coalesce(actor_job,'x') <> '' is true for a space — and it is the audit
	// log's grouping key, where " sweep" beside "sweep" is two jobs in every
	// report anybody runs.
	for _, blank := range []string{"", "   ", "\t\n"} {
		before := pool.Stat().AcquireCount()

		err := database.WithServiceJob(
			t.Context(), pool, blank, func(context.Context, pgx.Tx) error {
				t.Errorf("the closure ran for the job name %q", blank)

				return nil
			},
		)
		if !errors.Is(err, database.ErrNoActor) {
			t.Errorf("WithServiceJob with the name %q = %v, want ErrNoActor", blank, err)
		}

		if after := pool.Stat().AcquireCount(); after != before {
			t.Errorf("the pool was acquired %d time(s) for the job name %q", after-before, blank)
		}
	}

	// Padding is not a second job. The name is trimmed on the way in, and the
	// witness is the value the transaction actually publishes rather than two
	// values compared to each other.
	var published string
	if err := database.WithServiceJob(
		t.Context(), pool, "  reminder-sweep  ",
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT current_setting('app.actor_job', true)`).Scan(&published)
		},
	); err != nil {
		t.Fatalf("WithServiceJob with a padded name: %v", err)
	}
	if published != "reminder-sweep" {
		t.Errorf("a padded job name was published as %q", published)
	}
}

// Nesting is refused in both orders, and the refusal is what keeps a rollback
// meaningful: a nested seam takes a second connection and commits independently,
// so the outer rollback would leave its rows behind.
func TestSeamsRefuseToNestInEitherOrder(t *testing.T) {
	db := cluster.NewDatabase(t)
	request, err := database.NewPool(t.Context(), db.AppURL)
	if err != nil {
		t.Fatalf("opening the request-path pool: %v", err)
	}
	t.Cleanup(request.Close)

	service := servicePool(t, db)

	t.Run("service inside caller", func(t *testing.T) {
		err := database.WithCaller(t.Context(), request, testCaller(), func(ctx context.Context, _ pgx.Tx) error {
			return database.WithServiceJob(ctx, service, testProbe, func(context.Context, pgx.Tx) error {
				t.Error("the nested closure ran")

				return nil
			})
		})
		if !errors.Is(err, database.ErrNestedSeam) {
			t.Fatalf("WithCaller = %v, want it to wrap ErrNestedSeam", err)
		}
	})

	t.Run("caller inside service", func(t *testing.T) {
		err := database.WithServiceJob(t.Context(), service, testProbe, func(ctx context.Context, _ pgx.Tx) error {
			return database.WithCaller(ctx, request, testCaller(), func(context.Context, pgx.Tx) error {
				t.Error("the nested closure ran")

				return nil
			})
		})
		if !errors.Is(err, database.ErrNestedSeam) {
			t.Fatalf("WithService = %v, want it to wrap ErrNestedSeam", err)
		}
	})

	t.Run("caller inside caller", func(t *testing.T) {
		err := database.WithCaller(t.Context(), request, testCaller(), func(ctx context.Context, _ pgx.Tx) error {
			return database.WithCaller(ctx, request, testCaller(), func(context.Context, pgx.Tx) error {
				t.Error("the nested closure ran")

				return nil
			})
		})
		if !errors.Is(err, database.ErrNestedSeam) {
			t.Fatalf("WithCaller = %v, want it to wrap ErrNestedSeam", err)
		}
	})
}

// The service seam is a transaction like the other one, and a seam that always
// rolled back would pass every assertion above.
func TestWithServiceCommitsAndRollsBack(t *testing.T) {
	db := cluster.NewDatabase(t)
	pool := servicePool(t, db)
	createAppTable(t, db, "service_write_probe")

	sentinel := errors.New("the closure refused")
	if err := database.WithServiceJob(
		t.Context(), pool, testProbe,
		func(ctx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "INSERT INTO app.service_write_probe (note) VALUES ('a')"); err != nil {
				return err
			}

			return sentinel
		},
	); !errors.Is(err, sentinel) {
		t.Fatalf("WithService = %v, want it to wrap the closure's error", err)
	}

	if err := database.WithServiceJob(
		t.Context(), pool, testProbe,
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO app.service_write_probe (note) VALUES ('b')")

			return err
		},
	); err != nil {
		t.Fatalf("WithService: %v", err)
	}

	var rows int
	if err := database.WithServiceJob(
		t.Context(), pool, testProbe,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, "SELECT count(*) FROM app.service_write_probe").Scan(&rows)
		},
	); err != nil {
		t.Fatalf("counting: %v", err)
	}

	if rows != 1 {
		t.Errorf("%d row(s) after one failed and one successful closure, want 1", rows)
	}
}

// VerifyPools is what stands between a swapped pair of connection strings and a
// deployment nobody notices is wrong.
func TestVerifyPoolsAcceptsTheArrangementAndRefusesTheRest(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	open := func(url string) *pgxpool.Pool {
		pool, err := database.NewPool(ctx, url)
		if err != nil {
			t.Fatalf("opening a pool: %v", err)
		}
		t.Cleanup(pool.Close)

		return pool
	}

	request := open(db.AppURL)
	service := open(db.ServiceAppURL)
	superuser := open(db.SuperuserURL)

	if err := database.VerifyPools(ctx, request, service); err != nil {
		t.Fatalf("VerifyPools refused the arrangement the chain builds: %v", err)
	}

	for name, pools := range map[string][2]*pgxpool.Pool{
		"the pools swapped":               {service, request},
		"one pool serving both paths":     {request, request},
		"a superuser on the request path": {superuser, service},
		"a superuser on the service path": {request, superuser},
	} {
		t.Run(name, func(t *testing.T) {
			if err := database.VerifyPools(ctx, pools[0], pools[1]); err == nil {
				t.Error("VerifyPools accepted it")
			}
		})
	}
}

// Under the simple protocol pgx interpolates arguments into the statement on
// the client, which would turn the bound parameters both seams rest on into
// string concatenation nobody is looking at.
func TestPoolsDoNotUseTheSimpleProtocol(t *testing.T) {
	db := cluster.NewDatabase(t)

	for name, url := range map[string]string{
		"request path": db.AppURL,
		"service path": db.ServiceAppURL,
	} {
		t.Run(name, func(t *testing.T) {
			pool, err := database.NewPool(t.Context(), url)
			if err != nil {
				t.Fatalf("opening the pool: %v", err)
			}
			t.Cleanup(pool.Close)

			if mode := pool.Config().ConnConfig.DefaultQueryExecMode; mode == pgx.QueryExecModeSimpleProtocol {
				t.Errorf("the %s pool uses the simple protocol, so its arguments are "+
					"interpolated on the client", name)
			}
		})
	}
}

// Every setting either seam publishes is transaction-scoped, and this is the
// witness for "SET without LOCAL is issued nowhere". A behavioural test proves
// the connection is clean afterwards; this proves there is no second statement
// anywhere in the package that would not be.
func TestThePackageIssuesNoSettingThatOutlivesItsTransaction(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		source, err := os.ReadFile(filepath.Join(".", entry.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", entry.Name(), err)
		}

		for number, line := range strings.Split(string(source), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}

			if strings.Contains(line, "set_config(") && !strings.Contains(line, ", true)") {
				t.Errorf("%s:%d publishes a setting that is not transaction-local: %s",
					entry.Name(), number+1, trimmed)
			}

			// The statement form has no is_local argument to get wrong: SET
			// without LOCAL outlives the transaction, and connections are pooled.
			if strings.Contains(line, `"SET `) && !strings.Contains(line, `"SET LOCAL `) {
				t.Errorf("%s:%d issues SET without LOCAL: %s", entry.Name(), number+1, trimmed)
			}
		}
	}
}

// "Exactly one" is a property of the transaction rather than of the actor
// value: a leftover on the connection makes it two at the moment the row is
// written, so the seam clears the setting it is not publishing.
//
// The connection is dirtied session-level on a single-connection pool for the
// same reason as the claims test: on a clean connection the assertion is green
// with both clearing statements deleted.
func TestWithServiceClearsTheActorSettingItDoesNotPublish(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	cfg, err := pgxpool.ParseConfig(db.ServiceAppURL)
	if err != nil {
		t.Fatalf("parsing the service URL: %v", err)
	}
	cfg.MaxConns = 1

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("opening a single-connection service pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(
		ctx, `SELECT set_config('app.actor_job', 'left-behind', false)`,
	); err != nil {
		t.Fatalf("dirtying the connection: %v", err)
	}

	var id, job string
	if err := database.WithService(
		requestOf(t, "8a1f3b7c-0000-4000-8000-000000000001"), pool,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `
				SELECT coalesce(nullif(current_setting('app.actor_id', true), ''), ''),
				       coalesce(nullif(current_setting('app.actor_job', true), ''), '')
			`).Scan(&id, &job)
		},
	); err != nil {
		t.Fatalf("WithService: %v", err)
	}

	if id == "" {
		t.Error("the actor was not published")
	}
	if job != "" {
		t.Errorf("app.actor_job = %q, want cleared — the audit row would name two actors", job)
	}
}

// The barrier probe has to tell "the database refused this" from "the probe did
// not find out". A cluster with no migration chain applied has no
// cadence_service to assume, so an unconditional error check reads that as a
// barrier holding and lets the process start having measured nothing.
func TestVerifyPoolsRefusesToGuessWhenTheRolesDoNotExist(t *testing.T) {
	ctx := t.Context()

	db := cluster.NewDatabase(t)
	request, err := database.NewPool(ctx, db.AppURL)
	if err != nil {
		t.Fatalf("opening the request-path pool: %v", err)
	}
	t.Cleanup(request.Close)

	service, err := database.NewPool(ctx, db.ServiceAppURL)
	if err != nil {
		t.Fatalf("opening the service-path pool: %v", err)
	}
	t.Cleanup(service.Close)

	// The control: it passes while the arrangement is there.
	if err := database.VerifyPools(ctx, request, service); err != nil {
		t.Fatalf("VerifyPools refused the arrangement the chain builds: %v", err)
	}

	// Renamed rather than dropped: the role owns grants and a membership, so
	// dropping it needs a cleanup of its own, and what the probe cares about is
	// only that the name it asks for is not there.
	admin := testsupport.Connect(t, db.SuperuserURL)
	if _, err := admin.Exec(
		ctx, `ALTER ROLE `+testsupport.ServiceRole+` RENAME TO cadence_service_renamed`,
	); err != nil {
		t.Fatalf("renaming the service role: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.WithoutCancel(ctx)
		if _, err := admin.Exec(
			cleanupCtx, `ALTER ROLE cadence_service_renamed RENAME TO `+testsupport.ServiceRole,
		); err != nil {
			t.Errorf("restoring the service role: %v", err)
		}
	})

	if err := database.VerifyPools(ctx, request, service); err == nil {
		t.Error("VerifyPools reported the barrier holds against a role that does not exist")
	}
}

// The separation is claimed to be symmetric, so it is probed symmetrically. A
// service path able to assume a product role would read every patient's rows
// through policies written for somebody else.
func TestVerifyPoolsRefusesAServicePathThatCanAssumeAProductRole(t *testing.T) {
	ctx := t.Context()

	db := cluster.NewDatabase(t)
	request, err := database.NewPool(ctx, db.AppURL)
	if err != nil {
		t.Fatalf("opening the request-path pool: %v", err)
	}
	t.Cleanup(request.Close)

	service, err := database.NewPool(ctx, db.ServiceAppURL)
	if err != nil {
		t.Fatalf("opening the service-path pool: %v", err)
	}
	t.Cleanup(service.Close)

	admin := testsupport.Connect(t, db.SuperuserURL)
	if _, err := admin.Exec(
		ctx, `GRANT `+testsupport.DoctorRole+` TO `+testsupport.ServiceAppRole,
	); err != nil {
		t.Fatalf("granting a product role to the service path: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.WithoutCancel(ctx)
		if _, err := admin.Exec(
			cleanupCtx, `REVOKE `+testsupport.DoctorRole+` FROM `+testsupport.ServiceAppRole,
		); err != nil {
			t.Errorf("revoking the product role: %v", err)
		}
	})

	if err := database.VerifyPools(ctx, request, service); err == nil {
		t.Error("VerifyPools accepted a service path holding a product role")
	}
}

// The attribute guard has to catch a role altered after the chain built it, not
// only one the chain never built. BYPASSRLS on either connection role makes
// every policy in the system advisory while every test about policies stays
// green — which is why it is checked at startup and why the check is measured.
func TestVerifyPoolsRefusesAConnectionThatBypassesRowLevelSecurity(t *testing.T) {
	ctx := t.Context()

	db := cluster.NewDatabase(t)
	admin := testsupport.Connect(t, db.SuperuserURL)

	for _, role := range []string{testsupport.AppRole, testsupport.ServiceAppRole} {
		t.Run(role, func(t *testing.T) {
			if _, err := admin.Exec(ctx, `ALTER ROLE `+role+` BYPASSRLS`); err != nil {
				t.Fatalf("loosening %s: %v", role, err)
			}
			t.Cleanup(func() {
				cleanupCtx := context.WithoutCancel(ctx)
				if _, err := admin.Exec(cleanupCtx, `ALTER ROLE `+role+` NOBYPASSRLS`); err != nil {
					t.Errorf("restoring %s: %v", role, err)
				}
			})

			request, err := database.NewPool(ctx, db.AppURL)
			if err != nil {
				t.Fatalf("opening the request-path pool: %v", err)
			}
			t.Cleanup(request.Close)

			service, err := database.NewPool(ctx, db.ServiceAppURL)
			if err != nil {
				t.Fatalf("opening the service-path pool: %v", err)
			}
			t.Cleanup(service.Close)

			if err := database.VerifyPools(ctx, request, service); err == nil {
				t.Errorf("VerifyPools accepted a deployment where %s bypasses row level security",
					role)
			}
		})
	}
}

// "The two roles differ" would pass for any two low-privilege roles, so the
// names are pinned against the ones the chain creates. A DATABASE_URL pointing
// at a third role nobody wrote a grant for holds nothing, cannot assume anything,
// and therefore satisfies every other check in VerifyPools — while failing every
// request once the deploy has gone out.
func TestVerifyPoolsRefusesARoleTheChainDidNotBuild(t *testing.T) {
	ctx := t.Context()

	db := cluster.NewDatabase(t)
	admin := testsupport.Connect(t, db.SuperuserURL)

	if _, err := admin.Exec(
		ctx, `CREATE ROLE cadence_impostor LOGIN NOINHERIT PASSWORD 'cadence_impostor'`,
	); err != nil {
		t.Fatalf("creating the impostor role: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.WithoutCancel(ctx)
		// The CONNECT grant below is an object depending on the role, and it
		// lives in a database this cleanup is not connected to — so it is
		// revoked before the drop rather than left to it.
		for _, statement := range []string{
			`REVOKE CONNECT ON DATABASE ` + pgx.Identifier{db.Name}.Sanitize() +
				` FROM cadence_impostor`,
			`DROP ROLE IF EXISTS cadence_impostor`,
		} {
			if _, err := admin.Exec(cleanupCtx, statement); err != nil {
				t.Errorf("cleaning up the impostor role (%s): %v", statement, err)
			}
		}
	})
	if _, err := admin.Exec(
		ctx, `GRANT CONNECT ON DATABASE `+pgx.Identifier{db.Name}.Sanitize()+` TO cadence_impostor`,
	); err != nil {
		t.Fatalf("letting the impostor connect: %v", err)
	}

	impostor, err := database.NewPool(ctx, cluster.DSN("cadence_impostor", "cadence_impostor", db.Name))
	if err != nil {
		t.Fatalf("opening the impostor pool: %v", err)
	}
	t.Cleanup(impostor.Close)

	service, err := database.NewPool(ctx, db.ServiceAppURL)
	if err != nil {
		t.Fatalf("opening the service-path pool: %v", err)
	}
	t.Cleanup(service.Close)

	if err := database.VerifyPools(ctx, impostor, service); err == nil {
		t.Error("VerifyPools accepted a request path connecting as a role the chain never built")
	}
}

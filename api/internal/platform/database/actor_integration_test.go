//go:build integration

package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// The subject the middleware verified, and one it did not. The second exists so
// the assertions below can say "this uuid and not that one" rather than "some
// uuid": an actor read from anywhere other than the principal would still be
// UUID-shaped, and a test comparing shapes would not notice.
const (
	verifiedSubject = "8a1f3b7c-0000-4000-8000-0000000000b1"
	somebodyElse    = "8a1f3b7c-0000-4000-8000-0000000000b2"
)

func requestOf(t *testing.T, subject string) context.Context {
	t.Helper()

	return auth.WithPrincipal(t.Context(), auth.Principal{Subject: subject, Role: "doctor"})
}

// The audit actor is the caller the request was verified as, and there is no
// argument by which it could be anybody else.
//
// Forging the actor survived two drafts of the proposal, and it survived them
// because it was the value of a parameter: every call site looked right, and
// one that named a different uuid would have looked right too. What closes it
// is not a check — it is that the subject now has one source, and no exported
// function of this package takes it. The witness for the second half is
// TestTheExportedSurfaceIsTheOneDeclared, which needs no database.
func TestTheServiceActorIsTheCallerTheRequestWasVerifiedAs(t *testing.T) {
	db := cluster.NewDatabase(t)
	pool := servicePool(t, db)

	// Two callers on one pool, because one would be satisfied by an actor read
	// from anywhere that happens to hold a uuid — a constant, the first principal
	// the process ever saw, a value left on the connection. The actor has to
	// move with the principal.
	for _, subject := range []string{verifiedSubject, somebodyElse} {
		t.Run(subject, func(t *testing.T) {
			var id string
			if err := database.WithService(
				requestOf(t, subject),
				pool,
				func(ctx context.Context, tx pgx.Tx) error {
					return tx.QueryRow(ctx, `
						SELECT coalesce(nullif(current_setting('app.actor_id', true), ''), '')
					`).Scan(&id)
				},
			); err != nil {
				t.Fatalf("WithService: %v", err)
			}

			if id != subject {
				t.Errorf("app.actor_id = %q, want the principal's subject %q", id, subject)
			}
		})
	}
}

// A service transaction on a request that carries no verified caller is refused.
//
// Not defaulted to a job, and not left unattributed: the two are the states the
// audit log has no way to tell apart afterwards. The pool is not touched either,
// so a refusal costs no connection.
func TestWithServiceRefusesARequestWithNoVerifiedCaller(t *testing.T) {
	db := cluster.NewDatabase(t)
	pool := servicePool(t, db)

	before := pool.Stat().AcquireCount()

	err := database.WithService(t.Context(), pool, func(context.Context, pgx.Tx) error {
		t.Error("the closure ran for a request with no verified caller")

		return nil
	})
	if !errors.Is(err, database.ErrNoPrincipal) {
		t.Fatalf("WithService = %v, want ErrNoPrincipal", err)
	}

	if after := pool.Stat().AcquireCount(); after != before {
		t.Errorf("the pool was acquired %d time(s) for a request with no verified caller",
			after-before)
	}

	// A principal is not enough on its own, and the two ways it can be useless
	// are different errors on purpose. audit_log.actor_id is a uuid column, and a
	// cast failing inside a policy is a 500 where a refusal belongs — so a subject
	// that is not UUID-shaped is refused here rather than at the insert. A
	// principal with no subject at all is not malformed, it is absent, and it
	// arrives the same way: the verifier refuses a token with no sub, so this is
	// the branch that holds if it ever stops.
	for _, refusal := range []struct {
		name    string
		subject string
		want    error
	}{
		{"a malformed subject", "not-a-uuid", database.ErrInvalidSubject},
		{"no subject at all", "", database.ErrNoActor},
	} {
		t.Run(refusal.name, func(t *testing.T) {
			ctx := auth.WithPrincipal(
				t.Context(), auth.Principal{Subject: refusal.subject, Role: "doctor"},
			)

			if err := database.WithService(ctx, pool, func(context.Context, pgx.Tx) error {
				t.Errorf("the closure ran for %s", refusal.name)

				return nil
			}); !errors.Is(err, refusal.want) {
				t.Errorf("WithService with %s = %v, want %v", refusal.name, err, refusal.want)
			}
		})
	}
}

// The reminder sweep, the invitation mailer, the push fan-out: real writes with
// no human behind them. They are a different constructor rather than a different
// argument, which is what makes "this path has no actor to derive" a decision
// somebody made once instead of a nil check at every call site.
//
// It writes actor_job and leaves actor_id cleared even when a principal happens
// to be on the context: what is recorded is what the constructor said, and the
// audit log's "exactly one of the two" is the seam's property rather than the
// caller's.
func TestAPathWithNoHumanIsAttributedToItsJob(t *testing.T) {
	db := cluster.NewDatabase(t)
	pool := servicePool(t, db)

	for name, ctx := range map[string]context.Context{
		"with no principal on the context": t.Context(),
		"with one":                         requestOf(t, verifiedSubject),
	} {
		t.Run(name, func(t *testing.T) {
			var id, job string
			if err := database.WithServiceJob(
				ctx, pool, "reminder-sweep",
				func(ctx context.Context, tx pgx.Tx) error {
					return tx.QueryRow(ctx, `
						SELECT coalesce(nullif(current_setting('app.actor_id', true), ''), ''),
						       coalesce(nullif(current_setting('app.actor_job', true), ''), '')
					`).Scan(&id, &job)
				},
			); err != nil {
				t.Fatalf("WithServiceJob: %v", err)
			}

			if job != "reminder-sweep" {
				t.Errorf("app.actor_job = %q, want the job's name", job)
			}
			if id != "" {
				t.Errorf("app.actor_id = %q, want cleared", id)
			}
		})
	}
}

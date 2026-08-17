//go:build integration

package database_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// The second holder must not enter until the first has left. Asserted by order rather than by a duration: under load
// a timing assertion says what the machine was doing.
func TestTheSecondHolderOfOneKeyWaitsForTheFirst(t *testing.T) {
	pool := lockPool(t)

	const key = "anna@clinic.example"

	var (
		mu    sync.Mutex
		order []string
	)

	record := func(what string) {
		mu.Lock()
		defer mu.Unlock()

		order = append(order, what)
	}

	first := make(chan struct{})
	firstMayLeave := make(chan struct{})

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		err := database.WithAdvisoryLock(t.Context(), pool, database.OnboardingLock, key,
			func(context.Context) error {
				record("first in")
				close(first)
				<-firstMayLeave
				record("first out")

				return nil
			})
		if err != nil {
			t.Errorf("the first holder: %v", err)
		}
	}()

	<-first

	second := make(chan struct{})

	wg.Add(1)

	go func() {
		defer wg.Done()

		// Bounded well above the 250ms the first holder stays inside, and for the failure rather than the wait:
		// unbounded, a lock the first holder never releases would take the suite's whole timeout to report.
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()

		err := database.WithAdvisoryLock(ctx, pool, database.OnboardingLock, key,
			func(context.Context) error {
				record("second in")
				close(second)

				return nil
			})
		if err != nil {
			t.Errorf("the second holder: %v", err)
		}
	}()

	// Long enough that a lock which does not block would have let the second holder in. It cannot fail falsely: the
	// assertion below is on the order, and a slow machine only makes the second holder later.
	select {
	case <-second:
		t.Fatal("the second holder entered while the first was inside")
	case <-time.After(250 * time.Millisecond):
	}

	close(firstMayLeave)
	wg.Wait()

	want := []string{"first in", "first out", "second in"}
	if len(order) != len(want) {
		t.Fatalf("the holders recorded %v, want %v", order, want)
	}

	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("the holders recorded %v, want %v", order, want)
		}
	}
}

// Two addresses are two locks: without this the test above passes against a lock taken on a constant.
func TestTwoKeysAreTwoLocks(t *testing.T) {
	pool := lockPool(t)

	inside := make(chan struct{})
	mayLeave := make(chan struct{})

	done := make(chan error, 1)

	go func() {
		done <- database.WithAdvisoryLock(t.Context(), pool, database.OnboardingLock,
			"anna@clinic.example", func(context.Context) error {
				close(inside)
				<-mayLeave

				return nil
			})
	}()

	<-inside

	entered := make(chan struct{})

	// Bounded: with both keys hashing to one number the call below never returns, and a wait is the failure here.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	err := database.WithAdvisoryLock(ctx, pool, database.OnboardingLock,
		"boris@clinic.example", func(context.Context) error {
			close(entered)

			return nil
		})
	if err != nil {
		t.Fatalf("holding the second key: %v", err)
	}

	select {
	case <-entered:
	default:
		t.Fatal("the holder of the second key never ran")
	}

	close(mayLeave)

	if err := <-done; err != nil {
		t.Fatalf("holding the first key: %v", err)
	}
}

// A failing closure still releases, and the second holder is on a pool of its own: an advisory lock belongs to the
// session, so measured, a test that re-locks through the same pool passes with the release deleted.
//
// The failure the release prevents shows up in production only: the first patient whose creation is refused leaves
// the lock behind, and every later request for that address waits on a connection that is back in the pool.
func TestAFailingHolderReleasesTheLockForEverybodyElse(t *testing.T) {
	held, others := twoPoolsOnOneDatabase(t)

	const key = "carried@clinic.example"

	refused := errors.New("the closure refused")

	if err := database.WithAdvisoryLock(t.Context(), held, database.OnboardingLock, key,
		func(context.Context) error {
			return refused
		}); !errors.Is(err, refused) {
		t.Fatalf("the closure's error came back as %v, want %v", err, refused)
	}

	// Bounded, because the failure mode is a wait that never ends.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	if err := database.WithAdvisoryLock(ctx, others, database.OnboardingLock, key,
		func(context.Context) error {
			return nil
		}); err != nil {
		t.Fatalf("another connection taking the lock after a failed holder: %v", err)
	}
}

// The release runs on a context detached from the request's, and this is what that detachment buys: a request
// cancelled while it holds the lock hands its connection back alive.
//
// Named for the mutation it exists to fail against — context.WithoutCancel(ctx) replaced by ctx in the deferred
// release. The lock itself comes free either way, which is why the suite stayed green under that mutation: the unlock
// statement is refused before it leaves the process, the branch below it closes the connection, and Postgres drops the
// advisory locks of a session that has died. What is lost is the connection, so that is what is measured — and the
// pool holds one, so a replacement cannot hide behind a spare.
func TestALockReleasedAfterCancellationKeepsItsConnection(t *testing.T) {
	pool := oneConnectionPool(t)

	before := backendPID(t, pool)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := database.WithAdvisoryLock(ctx, pool, database.OnboardingLock, "cancelled@clinic.example",
		func(inside context.Context) error {
			// From within, so that the release below is the first thing to run on an ended context: the
			// request whose caller hung up while its invitation was in flight.
			cancel()

			return inside.Err()
		})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("the cancelled holder came back with %v, want %v", err, context.Canceled)
	}

	if after := backendPID(t, pool); after != before {
		t.Errorf("the pool's backend went from %d to %d: the release ran on the request's own cancelled "+
			"context, so the connection was closed instead of unlocked and every cancelled request costs one",
			before, after)
	}
}

// The request path's pool, under cadence_app: the role the lock is taken as in production.
func lockPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	return poolOn(t, cluster.NewDatabase(t).AppURL)
}

// Advisory locks are scoped to a database: two pools on two databases never contend and would pass with no lock.
func twoPoolsOnOneDatabase(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool) {
	t.Helper()

	url := cluster.NewDatabase(t).AppURL

	return poolOn(t, url), poolOn(t, url)
}

// One connection, so that a connection the release destroyed is one the next acquire has to replace with a new
// backend rather than take from a spare.
func oneConnectionPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	cfg, err := pgxpool.ParseConfig(cluster.NewDatabase(t).AppURL)
	if err != nil {
		t.Fatalf("parsing the database URL: %v", err)
	}

	cfg.MaxConns = 1

	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatalf("opening the pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

func backendPID(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()

	var pid int
	if err := pool.QueryRow(t.Context(), `SELECT pg_catalog.pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatalf("reading the backend pid: %v", err)
	}

	return pid
}

func poolOn(t *testing.T, url string) *pgxpool.Pool {
	t.Helper()

	pool, err := database.NewPool(t.Context(), url)
	if err != nil {
		t.Fatalf("opening the pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

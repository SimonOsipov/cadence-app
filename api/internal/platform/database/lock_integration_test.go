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

// The lock is what makes two requests for one address run one after the other,
// so the property under test is that the second holder does not enter until the
// first has left. Asserted by order rather than by a duration: a machine under
// load makes a timing assertion say what the machine was doing.
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

		// Bounded well above the 250ms the first holder stays inside, and for
		// the failure rather than the wait: a lock the first holder never
		// releases is what this test is against, and unbounded it would take
		// the whole suite's timeout to report it.
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

	// Long enough that a lock which does not block would have let the second
	// holder in and short enough not to dominate the suite. It cannot produce a
	// false failure: the assertion below is on the order, and a slow machine only
	// makes the second holder arrive later than the first, which is the state the
	// test wants.
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

// Two addresses are two locks. Without this the test above would also pass
// against a lock taken on a constant, which would serialise the whole clinic.
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

	// Bounded, because the failure this test exists for is a wait rather than a
	// wrong answer: with both keys hashing to one number the call below never
	// returns, and an unbounded test would take the suite's whole timeout to say so.
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

// A failing closure still releases, and the second holder is on a pool of its
// own. That is not decoration: an advisory lock belongs to the session, so a
// session that took it can take it again — measured, and it means a test that
// re-locks through the same pool passes with the release deleted, since pgxpool
// hands the connection straight back.
//
// What the release buys is that everybody else can proceed. The failure it
// prevents shows up in production only: the first patient whose creation is
// refused leaves the lock behind, and every later request for that address
// waits on a connection that is back in the pool and never coming out of it.
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

// The pool is the request path's, under cadence_app: that is the pool the lock
// is taken on in production, and the role holds no privilege on the app schema
// beyond its policies.
func lockPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	return poolOn(t, cluster.NewDatabase(t).AppURL)
}

// Two pools on one database, because advisory locks are scoped to a database:
// two pools on two databases would never contend, and a test built on them
// would pass with the lock deleted.
func twoPoolsOnOneDatabase(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool) {
	t.Helper()

	url := cluster.NewDatabase(t).AppURL

	return poolOn(t, url), poolOn(t, url)
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

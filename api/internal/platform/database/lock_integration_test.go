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

func poolOn(t *testing.T, url string) *pgxpool.Pool {
	t.Helper()

	pool, err := database.NewPool(t.Context(), url)
	if err != nil {
		t.Fatalf("opening the pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

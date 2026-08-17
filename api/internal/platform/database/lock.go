package database

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LockClass separates one kind of advisory lock from another.
//
// Postgres has one advisory lock space per database, so classes are allocated here rather than by the contexts that
// take them: two contexts picking a number of their own eventually pick the same one.
type LockClass int32

// OnboardingLock covers one address for the whole of a person's creation: lookup, invitation and both transactions.
const OnboardingLock LockClass = 1

// unlockTimeout keeps an already-cancelled request from falling to the branch below and destroying a connection.
const unlockTimeout = 5 * time.Second

// ErrLockUnavailable is the lock's own failure, named so a transport can answer «busy, try again» rather than
// «internal error»: no connection to take the lock on and a refused statement are both ordinary under load.
var ErrLockUnavailable = errors.New("the advisory lock could not be taken")

// WithAdvisoryLock runs fn while holding the advisory lock named by class and key, and releases it however fn ends.
//
// The lock is session-level and held on a connection of its own, because what it has to cover is wider than a
// transaction: the call that invites somebody is made outside one.
//
// pool must not be the pool fn writes on. Each holder occupies one connection for as long as fn runs, so holders
// taken from the pool fn then asks a second connection of can fill it between them — every one holding a lock and
// every one waiting for a connection only another holder can release.
//
// The key is hashed into the lock space, so two keys can collide and serialise two operations that had no reason to
// wait for each other. That costs concurrency and never correctness, which is the right way round.
func WithAdvisoryLock(
	ctx context.Context, pool *pgxpool.Pool, class LockClass, key string, fn func(context.Context) error,
) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("taking a connection to lock on: %w: %w", err, ErrLockUnavailable)
	}
	defer conn.Release()

	hashed := lockKey(key)

	if _, err := conn.Exec(
		ctx,
		`SELECT pg_catalog.pg_advisory_lock($1, $2)`, int32(class), hashed,
	); err != nil {
		// Closed rather than released: the statement may have been granted the lock and then errored — a cancel
		// racing the grant — and a connection carrying a session lock back into the pool blocks that key for every
		// later request. Today pgx kills a cancelled connection itself, but that is a default nothing here pins.
		_ = conn.Conn().Close(ctx)

		return fmt.Errorf("taking the advisory lock: %w: %w", err, ErrLockUnavailable)
	}

	defer func() {
		// A release that inherits an already-ended context never reaches the server.
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), unlockTimeout)
		defer cancel()

		if _, err := conn.Exec(
			releaseCtx,
			`SELECT pg_catalog.pg_advisory_unlock($1, $2)`, int32(class), hashed,
		); err != nil {
			// Closed rather than handed back: a pooled connection still holding the lock blocks that key for the
			// lifetime of the process.
			_ = conn.Conn().Close(releaseCtx)
		}
	}()

	return fn(ctx)
}

// lockKey folds a key into the lock space's integer; deciding that two spellings are one person is the caller's.
func lockKey(key string) int32 {
	digest := fnv.New32a()
	_, _ = digest.Write([]byte(key))

	// The wrap is deliberate: the lock space is signed and every bit of the digest is worth keeping.
	return int32(digest.Sum32())
}

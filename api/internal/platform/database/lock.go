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
// Postgres has one advisory lock space for the whole database, so the classes
// are allocated here rather than by the contexts that take them: two contexts
// that pick a number of their own eventually pick the same one, and the symptom
// is two unrelated operations serialising each other under load.
type LockClass int32

// OnboardingLock covers everything done for one email address while a person is
// created — a patient or a member of staff: the lookup, the invitation and the
// two transactions that record them.
const OnboardingLock LockClass = 1

// unlockTimeout bounds the release of a lock whose context is already finished,
// so the ordinary cancelled request releases cleanly instead of falling to the
// branch below and destroying a connection.
const unlockTimeout = 5 * time.Second

// WithAdvisoryLock runs fn while holding the advisory lock named by class and
// key, and releases it afterwards however fn ends.
//
// The lock is session-level and held on a connection of its own, because what it
// has to cover is wider than a transaction: the call that invites somebody is
// made outside one, and a transaction-scoped lock would end at the first commit
// and leave the retry unserialised.
//
// pool must not be the pool fn writes on. Each holder occupies one connection
// for as long as fn runs, so holders taken from the same pool fn then asks for a
// second connection from can fill it between them — every one holding a lock and
// every one waiting for a connection that only another holder can release. The
// request pool is the one this is taken on; the writes go to the service pool.
//
// The key is hashed into the lock space, so two keys can collide and serialise
// two operations that had no reason to wait for each other. That costs
// concurrency and never correctness, which is the right way round: the
// alternative is a lock table with rows to insert, delete and clean up after a
// process that died.
// ErrLockUnavailable is the lock's own failure — no connection to take it on,
// or the statement itself refused. Named so a transport can answer «busy, try
// again» rather than «internal error»: both causes are ordinary under load.
var ErrLockUnavailable = errors.New("the advisory lock could not be taken")

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
		// Closed rather than released: the statement may have been granted the
		// lock and then errored — a cancel racing the grant — and a connection
		// carrying a session lock back into the pool blocks that key for every
		// later request. Today pgx kills a cancelled connection itself, but that
		// is a default nothing here pins, and the release path below already
		// makes the conservative move.
		_ = conn.Conn().Close(ctx)

		return fmt.Errorf("taking the advisory lock: %w: %w", err, ErrLockUnavailable)
	}

	defer func() {
		// The usual reason to be here is a context that has just ended, and a
		// release that inherits it never reaches the server.
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), unlockTimeout)
		defer cancel()

		if _, err := conn.Exec(
			releaseCtx,
			`SELECT pg_catalog.pg_advisory_unlock($1, $2)`, int32(class), hashed,
		); err != nil {
			// Closed rather than handed back: a pooled connection still holding
			// the lock blocks that key for the lifetime of the process.
			_ = conn.Conn().Close(releaseCtx)
		}
	}()

	return fn(ctx)
}

// lockKey folds a key into the integer the lock space is addressed by. The
// caller normalises first — the fold that decides two spellings are one person
// belongs to the context that owns the address, not to a hash function.
func lockKey(key string) int32 {
	digest := fnv.New32a()
	_, _ = digest.Write([]byte(key))

	// The wrap is deliberate: the lock space is signed and every bit of the
	// digest is worth keeping.
	return int32(digest.Sum32())
}

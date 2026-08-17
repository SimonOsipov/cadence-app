package database

import (
	"context"
	"errors"
	"net"

	"github.com/jackc/pgx/v5/pgconn"
)

// notNow is what a server that is up answers when it cannot serve this request yet.
//
// One code at a time rather than a class prefix: 08P01 protocol_violation is permanent, is a driver-level fault,
// and is inside class 08, so a prefix match would tell a client to retry it forever. Counted over the .c files of
// src/backend in the PostgreSQL 17.5 tree on 2026-08-17 — the basis for every number here — it has 111 raise sites
// against 31 for 08006, so the mistake would not be a rare one.
//
// Codes this server cannot send are left out on the same basis: 08004 and 08007 have no raise site anywhere in that
// tree, and 08000 and 08001 have none in src/backend — they belong to postgres_fdw and dblink, which this API does
// not use. pg_hba refusing a connection is 28000, outside the class entirely.
var notNow = map[string]struct{}{
	"08003": {}, // connection_does_not_exist
	"08006": {}, // connection_failure
	"40001": {}, // serialization_failure
	"40P01": {}, // deadlock_detected
	"53200": {}, // out_of_memory
	"53300": {}, // too_many_connections
	"55P03": {}, // lock_not_available
	// Not only statement_timeout: of the four sites in postgres.c the others are an authentication timeout, an
	// autovacuum task and a cancel request. A pgx caller that gives up returns context.Canceled and no SQLSTATE.
	"57014": {}, // query_canceled
	"57P01": {}, // admin_shutdown
	"57P02": {}, // crash_shutdown
	"57P03": {}, // cannot_connect_now
}

// IsUnavailable reports whether err means «not now» rather than «broken»: the difference between telling a caller to
// repeat the request and telling them to quote a request id.
//
// A caller that repeats must be idempotent for that advice to be sound: 40001 and 40P01 are raised after work may
// have begun, and this function cannot know what the caller was doing.
//
// The structural arm is first and is the one that matters most: a Postgres that is not accepting connections sends
// no SQLSTATE, so a classifier reading only codes misses it. Measured on pgx v5.10.0 against a refused port on
// 2026-08-17: errors.As finds *pgconn.ConnectError, finds no *pgconn.PgError, and returns in under a millisecond
// rather than at the deadline. A host that drops packets instead of refusing blocks until the deadline, and is
// caught by the arm above.
func IsUnavailable(err error) bool {
	if err == nil {
		return false
	}

	// The server's own word first, even inside a ConnectError. pgx wraps a server error raised during the
	// handshake — a rotated password, an edited pg_hba, a database renamed by a restore — in one of those, so a
	// structural arm read before this would answer «repeat in a few minutes» to a deployment that will never
	// succeed again. Measured on pgx v5.10.0: such a refusal satisfies errors.As for both types.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		_, transient := notNow[pgErr.Code]

		return transient
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Nothing came back carrying a SQLSTATE: the connection itself is what failed.
	var connErr *pgconn.ConnectError
	if errors.As(err, &connErr) {
		return true
	}

	var netErr net.Error

	return errors.As(err, &netErr)
}

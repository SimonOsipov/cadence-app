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
// and is inside class 08, so a prefix match would tell a client to retry it forever. Counted in the PostgreSQL 17.5
// tree on 2026-08-17, it has 111 raise sites, against 31 for 08006 — the mistake would not be a rare one.
//
// Codes the server never raises are left out, and that is measured rather than assumed: 08000, 08004 and 08007 have
// zero raise sites in that tree, and pg_hba rejecting a connection is 28000, outside this class entirely.
var notNow = map[string]struct{}{
	"08001": {}, // sqlclient_unable_to_establish_sqlconnection
	"08003": {}, // connection_does_not_exist
	"08006": {}, // connection_failure
	"40001": {}, // serialization_failure
	"40P01": {}, // deadlock_detected
	"53200": {}, // out_of_memory
	"53300": {}, // too_many_connections
	"55P03": {}, // lock_not_available
	// Not only statement_timeout: postgres.c raises it for pg_cancel_backend and for a recovery conflict too. A
	// pgx caller that gives up never arrives here — that returns context.Canceled with no SQLSTATE at all.
	"57014": {}, // query_canceled
	"57P01": {}, // admin_shutdown
	"57P02": {}, // crash_shutdown
	"57P03": {}, // cannot_connect_now
}

// IsUnavailable reports whether err means «not now» rather than «broken»: the difference between telling a caller to
// repeat the request and telling them to quote a request id.
//
// **A caller that repeats must be idempotent for that advice to be sound.** Two of the codes below — 40001 and
// 40P01 — are raised after work may have begun, and this function cannot know what the caller was doing.
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

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var connErr *pgconn.ConnectError
	if errors.As(err, &connErr) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		_, transient := notNow[pgErr.Code]

		return transient
	}

	return false
}

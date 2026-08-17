package database

import (
	"context"
	"errors"
	"net"

	"github.com/jackc/pgx/v5/pgconn"
)

// notNow is what the server says when it is up and refusing for a reason that passes.
//
// Chosen one code at a time rather than by class prefix, because the classes are not uniform: 08004 is pg_hba
// rejecting this deployment and 08P01 is a protocol violation — both permanent, both inside class 08, and both
// answered «repeat in a few minutes» by a prefix match. 08007 is here on the strength of the caller's idempotence,
// which is the one property that makes «unknown whether it committed» safe to retry.
var notNow = map[string]struct{}{
	"08000": {}, // connection_exception
	"08001": {}, // sqlclient_unable_to_establish_sqlconnection
	"08003": {}, // connection_does_not_exist
	"08006": {}, // connection_failure
	"08007": {}, // transaction_resolution_unknown
	"40001": {}, // serialization_failure
	"40P01": {}, // deadlock_detected
	"53200": {}, // out_of_memory
	"53300": {}, // too_many_connections
	"55P03": {}, // lock_not_available
	"57014": {}, // query_canceled — statement_timeout, not the caller giving up
	"57P01": {}, // admin_shutdown
	"57P02": {}, // crash_shutdown
	"57P03": {}, // cannot_connect_now
}

// IsUnavailable reports whether err means «not now» rather than «broken»: the difference between telling a caller to
// repeat the request and telling them to quote a request id.
//
// The structural arm is first and is the one that matters most: a Postgres that is down answers nothing at all, so
// there is no SQLSTATE to read — pgx returns a *pgconn.ConnectError wrapping a network error, and it returns it
// immediately rather than at the deadline. Measured on pgx v5.10.0 against a closed port on 2026-08-17: errors.As
// finds ConnectError and finds no PgError.
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

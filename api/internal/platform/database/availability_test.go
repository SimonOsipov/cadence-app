package database

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestWhatCountsAsTheDatabaseNotAnswering(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		// The structural arm. A server that is down sends no SQLSTATE, so a classifier that reads only codes
		// misses the one failure the whole distinction exists for.
		{"the connection was refused", &pgconn.ConnectError{Config: &pgconn.Config{}}, true},
		{"the network went away", &net.OpError{Op: "dial", Err: errors.New("no route to host")}, true},
		{"the request ran out of time", context.DeadlineExceeded, true},

		{"the server is shutting down", &pgconn.PgError{Code: "57P01"}, true},
		{"the server crashed", &pgconn.PgError{Code: "57P02"}, true},
		{"the database is not accepting connections", &pgconn.PgError{Code: "57P03"}, true},
		{"there are no connections left", &pgconn.PgError{Code: "53300"}, true},
		{"the server ran out of memory", &pgconn.PgError{Code: "53200"}, true},
		{"the transaction lost a race", &pgconn.PgError{Code: "40001"}, true},
		{"the transaction deadlocked", &pgconn.PgError{Code: "40P01"}, true},
		{"the statement ran out of time", &pgconn.PgError{Code: "57014"}, true},
		{"the connection failed", &pgconn.PgError{Code: "08006"}, true},

		// Permanent, and inside class 08 — the two a prefix match would sell as temporary.
		{"pg_hba refused this deployment", &pgconn.PgError{Code: "08004"}, false},
		{"the protocol was violated", &pgconn.PgError{Code: "08P01"}, false},

		// Permanent, and outside class 08 by one digit: the negative that fails a prefix widened to "0".
		{"the feature is not supported", &pgconn.PgError{Code: "0A000"}, false},
		{"the value was not a uuid", &pgconn.PgError{Code: "22P02"}, false},
		{"a grant is missing", &pgconn.PgError{Code: "42501"}, false},
		{"the row already exists", &pgconn.PgError{Code: "23505"}, false},
		{"the password was wrong", &pgconn.PgError{Code: "28P01"}, false},

		// The caller gave up. Nothing is wrong with the database, and the answer goes to a socket nobody is
		// reading; calling it unavailable would put client behaviour into the server's availability signal.
		{"the caller gave up", context.Canceled, false},

		{"nothing went wrong", nil, false},
		{"something nobody classified", errors.New("the database went away mid-statement"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsUnavailable(tc.err); got != tc.want {
				t.Errorf("IsUnavailable(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// Wrapped, because every caller wraps: a classifier that only reads the outermost error is one fmt.Errorf away from
// answering the wrong thing.
func TestTheClassificationSurvivesWrapping(t *testing.T) {
	wrapped := fmt.Errorf("running as cadence_patient: %w",
		fmt.Errorf("beginning the transaction: %w", &pgconn.PgError{Code: "57P03"}))

	if !IsUnavailable(wrapped) {
		t.Error("a wrapped 57P03 is not read as unavailable")
	}
}

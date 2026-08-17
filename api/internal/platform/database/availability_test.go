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
		// A server that is down sends no SQLSTATE, which is the failure the whole distinction exists for.
		{"the connection was refused", &pgconn.ConnectError{Config: &pgconn.Config{}}, true},
		{"the network went away", &net.OpError{Op: "dial", Err: errors.New("no route to host")}, true},
		{"the request ran out of time", context.DeadlineExceeded, true},

		{"the connection does not exist", &pgconn.PgError{Code: "08003"}, true},
		{"the lock was not available", &pgconn.PgError{Code: "55P03"}, true},
		{"the server is shutting down", &pgconn.PgError{Code: "57P01"}, true},
		{"the server crashed", &pgconn.PgError{Code: "57P02"}, true},
		{"the database is not accepting connections", &pgconn.PgError{Code: "57P03"}, true},
		{"there are no connections left", &pgconn.PgError{Code: "53300"}, true},
		{"the server ran out of memory", &pgconn.PgError{Code: "53200"}, true},
		{"the transaction lost a race", &pgconn.PgError{Code: "40001"}, true},
		{"the transaction deadlocked", &pgconn.PgError{Code: "40P01"}, true},
		{"the statement ran out of time", &pgconn.PgError{Code: "57014"}, true},
		{"the connection failed", &pgconn.PgError{Code: "08006"}, true},

		// Permanent and inside class 08: the code a prefix match would sell as temporary.
		{"the protocol was violated", &pgconn.PgError{Code: "08P01"}, false},

		// Inside class 08 and out of the map because this server cannot send them: 08004 and 08007 have no raise
		// site in the tree at all, 08000 and 08001 none outside postgres_fdw and dblink.
		{"a connection exception nothing raises", &pgconn.PgError{Code: "08000"}, false},
		{"a client that could not establish a connection", &pgconn.PgError{Code: "08001"}, false},
		{"a server that rejected the connection", &pgconn.PgError{Code: "08004"}, false},
		{"a transaction whose resolution is unknown", &pgconn.PgError{Code: "08007"}, false},

		// pg_hba refusing this deployment. Outside class 08 — the code that looks like it belongs there, 08004,
		// has no raise site in the server at all.
		{"the authorization was not valid", &pgconn.PgError{Code: "28000"}, false},

		// Outside class 08 by one character: the negative that fails a prefix widened to "0".
		{"the feature is not supported", &pgconn.PgError{Code: "0A000"}, false},

		{"the value was not a uuid", &pgconn.PgError{Code: "22P02"}, false},
		{"a grant is missing", &pgconn.PgError{Code: "42501"}, false},
		{"the row already exists", &pgconn.PgError{Code: "23505"}, false},
		{"the password was wrong", &pgconn.PgError{Code: "28P01"}, false},

		// Client behaviour, not the server's availability.
		{"the caller gave up", context.Canceled, false},

		{"nothing went wrong", nil, false},
		{"something nobody classified", errors.New("the database went away mid-statement"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// By name: a ConnectError with no inner error panics in its own Error method, when this is read.
			if got := IsUnavailable(tc.err); got != tc.want {
				t.Errorf("IsUnavailable(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// Every caller wraps, and a classifier reading only the outermost error is one fmt.Errorf from the wrong answer.
func TestTheClassificationSurvivesWrapping(t *testing.T) {
	wrapped := fmt.Errorf("running as cadence_patient: %w",
		fmt.Errorf("beginning the transaction: %w", &pgconn.PgError{Code: "57P03"}))

	if !IsUnavailable(wrapped) {
		t.Error("a wrapped 57P03 is not read as unavailable")
	}
}

//go:build integration

package database_test

import (
	"errors"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// The one shape a synthetic fixture cannot carry: pgx wraps a server error raised during the handshake inside a
// *pgconn.ConnectError whose inner error is unexported, so only a real refusal from a real server builds one.
//
// It decides whether a rotated password or an edited pg_hba is answered «repeat in a few minutes» forever. Both
// types match on this value, so this also pins which of the two arms is read first.
func TestAnAuthenticationFailureIsNotUnavailability(t *testing.T) {
	db := cluster.NewDatabase(t)

	parsed, err := url.Parse(db.AppURL)
	if err != nil {
		t.Fatalf("reading the connection string: %v", err)
	}
	parsed.User = url.UserPassword(parsed.User.Username(), "not-the-password")

	_, err = pgconn.Connect(t.Context(), parsed.String())
	if err == nil {
		t.Fatal("the server accepted a password it should not have")
	}

	var (
		pgErr   *pgconn.PgError
		connErr *pgconn.ConnectError
	)

	if !errors.As(err, &pgErr) || !errors.As(err, &connErr) {
		t.Fatalf("the refusal is not the shape this test exists for: %T — %v", err, err)
	}

	if database.IsUnavailable(err) {
		t.Errorf("SQLSTATE %s is reported as «repeat the request», which it will never succeed at", pgErr.Code)
	}
}

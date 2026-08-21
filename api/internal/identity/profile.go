package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// ProfileReader answers what the caller's own row says about them.
//
// An interface and not the struct below, because the route's two refusals — the
// database that will not answer, the read that fails — are otherwise reachable
// only with a database in front of them, and they are the answers a client has
// to be able to tell apart.
type ProfileReader interface {
	NameOf(ctx context.Context, caller database.Caller) (string, error)
}

// Profiles reads a caller's own profile row.
type Profiles struct {
	pool *pgxpool.Pool
}

// NewProfiles builds the service over the request pool; a nil pool yields a nil service, which the
// handler refuses on.
func NewProfiles(pool *pgxpool.Pool) *Profiles {
	if pool == nil {
		return nil
	}

	return &Profiles{pool: pool}
}

// NameOf answers the caller's own name, or the empty string when the clinic holds no profile for
// them — an account an invitation reached and provisioning did not.
//
// The predicate is not what keeps this to one row: profiles_own_select does for a patient and a
// doctor, and an administrator reads every profile through profiles_admin. Under an admin token the
// statement is the only thing choosing whose name comes back.
func (p *Profiles) NameOf(ctx context.Context, caller database.Caller) (string, error) {
	var name string

	err := database.WithCaller(ctx, p.pool, caller, func(ctx context.Context, tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT full_name FROM app.profiles WHERE user_id = $1
		`, caller.Subject).Scan(&name)

		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("reading the profile: %w", err)
		}

		return nil
	})
	if err != nil {
		if database.IsUnavailable(err) {
			return "", fmt.Errorf("reading the profile of %s: %w: %w", caller.Subject, ErrDatabaseUnavailable, err)
		}

		return "", fmt.Errorf("reading the profile of %s: %w", caller.Subject, err)
	}

	return name, nil
}

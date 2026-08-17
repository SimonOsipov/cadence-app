package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// ErrNoProfileToRecordAgainst: the hook issues cadence_role only for an account that has a profile, so no row is a
// broken invariant rather than a miss.
var ErrNoProfileToRecordAgainst = errors.New("the caller has no profile to record against")

// ErrNoTimezone: requireKnownTimezone accepts the empty string as «not measured yet», which a device's report is not.
var ErrNoTimezone = errors.New("the report carries no timezone")

// ErrDatabaseUnavailable is «repeat the request» rather than «quote the request id» — the two 5xx shapes of the api
// note. Not «did not answer»: a deadlock answers promptly and still means «not now».
var ErrDatabaseUnavailable = errors.New("the database could not serve the request")

// ErrNoRole is the account an invitation reached and provisioning has not: the seam has no role to assume for one.
var ErrNoRole = errors.New("the caller has no product role")

// The Russian the device reads. Neither names the offending zone: it came from the platform, not from the person.
const (
	detailNotATimezone = "Часовой пояс не распознан."
	detailNoRole       = "Аккаунт ещё не заведён в клинике."
)

const adminRole = "admin"

// Sessions writes what a device reports about itself on launch.
type Sessions struct {
	pool *pgxpool.Pool
}

// NewSessions builds the service over the request pool; a nil pool yields a nil service, which the handler refuses on.
func NewSessions(pool *pgxpool.Pool) *Sessions {
	if pool == nil {
		return nil
	}

	return &Sessions{pool: pool}
}

// RecordTimezone writes the zone against the caller's own profile row.
//
// A named weakness, which is why the predicate below is not a redundant copy of profiles_own_update: that policy
// narrows the statement for cadence_patient only. Under cadence_admin the profiles policy is USING (true) over a
// table-wide grant and cadence_app holds membership of that role, so without WHERE user_id = $2 a caller with an
// admin token rewrites the timezone of every profile in the clinic and is answered 204.
func (s *Sessions) RecordTimezone(ctx context.Context, caller database.Caller, timezone string) error {
	if timezone == "" {
		return fmt.Errorf("recording the timezone of %s: %w", caller.Subject, ErrNoTimezone)
	}

	err := database.WithCaller(ctx, s.pool, caller, func(ctx context.Context, tx pgx.Tx) error {
		if err := requireKnownTimezone(ctx, tx, timezone); err != nil {
			return err
		}

		// nullif as the patient's creation writes it: an empty string here is a zone nothing reports as missing.
		tag, err := tx.Exec(ctx, `
			UPDATE app.profiles SET timezone = nullif($1, '') WHERE user_id = $2
		`, timezone, caller.Subject)
		if err != nil {
			return fmt.Errorf("recording the timezone: %w", err)
		}

		// user_id is the primary key, so the count is 0 or 1, and a nil error is not a witness for a write.
		if tag.RowsAffected() != 1 {
			return ErrNoProfileToRecordAgainst
		}

		return nil
	})
	if err != nil {
		if database.IsUnavailable(err) {
			return fmt.Errorf("recording the timezone of %s: %w: %w", caller.Subject, ErrDatabaseUnavailable, err)
		}

		return fmt.Errorf("recording the timezone of %s: %w", caller.Subject, err)
	}

	return nil
}

// SessionInput is the request body of POST /v1/me/session.
type SessionInput struct {
	Body SessionBody
}

type SessionBody struct {
	Timezone string `json:"timezone" minLength:"1" maxLength:"64" doc:"The device's IANA timezone as pg_timezone_names spells it: Europe/Moscow, Asia/Tbilisi."`
}

// SessionOutput carries no body; 204 is the whole answer.
type SessionOutput struct{}

func (s *Service) recordSession(ctx context.Context, input *SessionInput) (*SessionOutput, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("no verified principal on the request context")
	}

	// Ahead of the assembly check unlike the onboarding routes: what staff are answered is the contract itself.
	switch principal.Role {
	case patientRole:
	case providerRole, adminRole:
		return &SessionOutput{}, nil
	default:
		// Invited and not provisioned is ordinary, and still an account no write can name: 204 would claim one.
		return nil, refusalForSession(fmt.Errorf("%q: %w", principal.Role, ErrNoRole))
	}

	if s.sessions == nil {
		return nil, huma.Error500InternalServerError("this API was assembled without a sessions service")
	}

	caller := database.Caller{Subject: principal.Subject, Role: principal.Role}
	if err := s.sessions.RecordTimezone(ctx, caller, input.Body.Timezone); err != nil {
		return nil, refusalForSession(err)
	}

	return &SessionOutput{}, nil
}

func refusalForSession(err error) error {
	switch {
	case errors.Is(err, ErrUnknownTimezone), errors.Is(err, ErrNoTimezone):
		return huma.Error400BadRequest(detailNotATimezone)

	case errors.Is(err, ErrNoRole):
		return huma.Error403Forbidden(detailNoRole)

	case errors.Is(err, ErrDatabaseUnavailable):
		return huma.Error503ServiceUnavailable("the database could not serve the request", err)

	default:
		return huma.Error500InternalServerError("recording the timezone", err)
	}
}

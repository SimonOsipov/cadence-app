package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// ErrNoProfileToRecordAgainst is returned when the write matched no row: the hook issues cadence_role only for an
// account that has a profile, so this is a broken invariant rather than a miss.
var ErrNoProfileToRecordAgainst = errors.New("the caller has no profile to record against")

// ErrNoTimezone is returned for an empty zone. requireKnownTimezone accepts one — absence is the ordinary state of a
// freshly created patient — and this is the one caller for which it is a malformed report rather than a state.
var ErrNoTimezone = errors.New("the report carries no timezone")

// ErrDatabaseUnavailable separates «repeat the request» from «quote the request id»: see the two 5xx shapes in the
// api component note.
var ErrDatabaseUnavailable = errors.New("the database did not answer")

// The Russian the device reads. Neither names the zone it was given: the value came from the platform rather than
// from the person holding the phone.
const (
	detailNotATimezone = "Часовой пояс не распознан."
	detailNoRole       = "Аккаунт ещё не заведён в клинике."
)

const adminRole = "admin"

// Sessions writes what a device reports about itself on launch.
type Sessions struct {
	pool *pgxpool.Pool
}

// NewSessions builds the service over the **request** pool: the row is the policy's choice and the column the
// grant's, and through the service path both decisions would move into Go. A nil pool yields a nil service rather
// than one that panics on first use.
func NewSessions(pool *pgxpool.Pool) *Sessions {
	if pool == nil {
		return nil
	}

	return &Sessions{pool: pool}
}

// RecordTimezone writes the zone against the caller's own profile row.
//
// The predicate is not a copy of profiles_own_update. That policy narrows this statement for cadence_patient only;
// under cadence_admin it is USING (true) over a table-wide grant, and cadence_app holds membership of that role — so
// without the predicate a caller with an admin token rewrites the timezone of every profile in the clinic and is
// answered 204.
func (s *Sessions) RecordTimezone(ctx context.Context, caller database.Caller, timezone string) error {
	if timezone == "" {
		return fmt.Errorf("recording the timezone of %s: %w", caller.Subject, ErrNoTimezone)
	}

	err := database.WithCaller(ctx, s.pool, caller, func(ctx context.Context, tx pgx.Tx) error {
		if err := requireKnownTimezone(ctx, tx, timezone); err != nil {
			return err
		}

		tag, err := tx.Exec(ctx, `
			UPDATE app.profiles SET timezone = $1 WHERE user_id = $2
		`, timezone, caller.Subject)
		if err != nil {
			return fmt.Errorf("recording the timezone: %w", err)
		}

		// user_id is the primary key, so the count is 0 or 1: a nil error from an UPDATE that matched nothing is
		// not a witness for a write.
		if tag.RowsAffected() != 1 {
			return ErrNoProfileToRecordAgainst
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("recording the timezone of %s: %w", caller.Subject, classifyAvailability(err))
	}

	return nil
}

// classifyAvailability names the failures that mean «not now» rather than «broken».
func classifyAvailability(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && isUnavailable(pgErr.Code) {
		return fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
	}

	return err
}

// isUnavailable: class 08 is connection failure, 57P01-03 are shutdown, termination, and a database that stopped
// accepting connections.
func isUnavailable(code string) bool {
	return strings.HasPrefix(code, "08") ||
		code == "57P01" || code == "57P02" || code == "57P03"
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

	// A closed set rather than «anything that is not a patient»: a token whose role claim went missing is a
	// regression of the issuance hook, and 204 would make it indistinguishable from a write on the one route every
	// device calls on every launch.
	//
	// Ahead of the assembly check on purpose, unlike the onboarding routes: what staff are answered is a statement
	// of the contract rather than of what this process was built with.
	switch principal.Role {
	case patientRole:
	case providerRole, adminRole:
		return &SessionOutput{}, nil
	default:
		return nil, huma.Error403Forbidden(detailNoRole)
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

	case errors.Is(err, ErrDatabaseUnavailable):
		return huma.Error503ServiceUnavailable("the database did not answer", err)

	// Named rather than left to the default, so «this account has no profile» is one line in the log and not an
	// anonymous wrapped driver error.
	case errors.Is(err, ErrNoProfileToRecordAgainst):
		return huma.Error500InternalServerError("no profile to record the timezone against", err)

	default:
		return huma.Error500InternalServerError("recording the timezone", err)
	}
}

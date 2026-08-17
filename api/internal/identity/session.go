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

// ErrNotATimezone is returned for a zone Postgres does not know.
//
// Checked against pg_timezone_names rather than a list of our own: the set is
// the server's, it moves with its tzdata, and a copy here would start refusing
// zones the database would have accepted.
var ErrNotATimezone = errors.New("no such timezone")

// ErrNoProfileToRecordAgainst is returned when the write changed no row.
//
// With cadence_role: patient the issuance hook has already found a profile, so
// no row is a broken invariant rather than an ordinary miss — and a nil error
// from an UPDATE that matched nothing is not a witness for a write.
var ErrNoProfileToRecordAgainst = errors.New("the caller has no profile to record against")

// The Russian a patient's device reads when it reports a zone the server does
// not know. It names the field rather than the value: the value came from the
// platform, and the person holding the phone did not type it.
const detailNotATimezone = "Часовой пояс не распознан."

// Sessions records what the caller's device reports about itself on launch.
type Sessions struct {
	pool *pgxpool.Pool
}

// NewSessions builds the service over the request pool.
//
// The request pool rather than the service one, and that is the whole design:
// the patient writes their own row under their own identity, so the policy
// decides which row and the column grant decides which column. Through the
// service path both decisions would move into Go.
func NewSessions(pool *pgxpool.Pool) *Sessions {
	return &Sessions{pool: pool}
}

// RecordTimezone writes the zone against the caller's own profile.
//
// Validation and write share one transaction: the zone is checked against the
// server that is about to store it, not against a second one.
func (s *Sessions) RecordTimezone(ctx context.Context, caller database.Caller, timezone string) error {
	err := database.WithCaller(ctx, s.pool, caller, func(ctx context.Context, tx pgx.Tx) error {
		var known bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_timezone_names WHERE name = $1)
		`, timezone).Scan(&known); err != nil {
			return fmt.Errorf("asking whether the zone is one: %w", err)
		}

		if !known {
			return fmt.Errorf("%q: %w", timezone, ErrNotATimezone)
		}

		// No predicate on the subject: profiles_own_update is
		// USING (user_id = app.jwt_subject()), so the row is the policy's
		// choice. A WHERE here would be a second copy of that rule, and the two
		// would drift the first time one of them was edited.
		tag, err := tx.Exec(ctx, `UPDATE app.profiles SET timezone = $1`, timezone)
		if err != nil {
			return fmt.Errorf("recording the timezone: %w", err)
		}

		if tag.RowsAffected() == 0 {
			return ErrNoProfileToRecordAgainst
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("recording the timezone of %s: %w", caller.Subject, err)
	}

	return nil
}

// SessionInput is the request body of POST /v1/me/session.
type SessionInput struct {
	Body SessionBody
}

// SessionBody is what a device reports about itself.
//
// One field, and everything else a launch could report is deliberately absent:
// this contract is generated into two client surfaces, and a field added here
// is a field neither of them can drop again.
type SessionBody struct {
	Timezone string `json:"timezone" minLength:"1" maxLength:"64" doc:"The device's IANA timezone, as the server knows it in pg_timezone_names: Europe/Moscow, Asia/Tbilisi. Stored for the patient only — the schedule, the reminders and the server-side sweep all read it."`
}

// SessionOutput carries no body: the caller learns the outcome from the status.
type SessionOutput struct{}

// recordSession takes the timezone of whoever is holding the device.
//
// A doctor and an administrator are answered rather than refused, and the
// decision is made here rather than by reading the database's refusal: only
// cadence_patient holds UPDATE on the column, so their write is a 42501 — and a
// handler that read "permission denied" as success would answer the same way on
// the day the patient's own grant went missing.
func (s *Service) recordSession(ctx context.Context, input *SessionInput) (*SessionOutput, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("no verified principal on the request context")
	}

	if principal.Role != patientRole {
		return &SessionOutput{}, nil
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

// refusalForSession answers what a report of a timezone is refused for.
func refusalForSession(err error) error {
	if errors.Is(err, ErrNotATimezone) {
		return huma.Error400BadRequest(detailNotATimezone)
	}

	return huma.Error500InternalServerError("recording the timezone", err)
}

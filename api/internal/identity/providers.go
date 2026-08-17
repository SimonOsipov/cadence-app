package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// Migration 000006 speaking: the service policies on profiles admit 'patient' and 'doctor' and no third value. Its
// own header is careful about the limit of that — an admin token transitions the request path into cadence_admin,
// whose policy admits any role.
var ErrServicePathMakesNoAdmins = errors.New("the service path may not write this role")

// What Postgres answers to a missing grant and to a row a policy would not admit, without distinguishing them.
const insufficientPrivilege = "42501"

// The two halves of the message that separate a policy's refusal from a grant's, and the table it has to be about:
// the audit row's actor is checked by a policy on the same transaction, and that refusal says nothing about a role.
//
// A named weakness: this is Postgres' English, and nothing in this repository pins the server's lc_messages. On a
// cluster initialised with a translated locale the reading falls through and the refusal is the 500 this route
// exists to remove.
const (
	rowLevelSecurityRefusal = "row-level security policy"
	profilesTable           = `"profiles"`
)

// NewProvider is everything the clinic states when it takes on a member of staff. No timezone and no locale, for
// the reason a new patient has none: nothing has reported one until the person's own device does.
type NewProvider struct {
	UserID string

	// Data rather than a literal inside the INSERT: which roles the service path may bring into being is
	// migration 000006's decision, and a literal would leave that policy's refusal unreachable and unmeasured.
	Role string

	FullName string

	// What a patient sees beside a name on their care team. Absent when the clinic did not state one.
	TitleRU *string
}

// CreateProvider writes a member of staff and the card that describes them through the service seam: two rows and
// an audit record, or none of them. It is the second of the two transactions of a staff creation — the invitation
// goes out first, under the lock Onboarding.InviteProvider takes.
func CreateProvider(ctx context.Context, pool *pgxpool.Pool, provider NewProvider) error {
	if !database.IsUUIDShaped(provider.UserID) {
		return fmt.Errorf("creating provider %q: %w", provider.UserID, ErrMalformedIdentifier)
	}

	err := database.WithService(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		return writeProvider(ctx, tx, provider)
	})
	if err != nil {
		return fmt.Errorf("creating provider %s: %w", provider.UserID, classifyStaff(err))
	}

	return nil
}

func writeProvider(ctx context.Context, tx pgx.Tx, provider NewProvider) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.profiles (user_id, role, full_name) VALUES ($1, $2, $3)
	`, provider.UserID, provider.Role, provider.FullName); err != nil {
		return fmt.Errorf("writing the profile: %w", err)
	}

	// clinic_name is left to whoever adds the second clinic: with one, no name distinguishes anybody from anybody.
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.provider_profiles (user_id, title_ru) VALUES ($1, $2)
	`, provider.UserID, provider.TitleRU); err != nil {
		return fmt.Errorf("writing the provider card: %w", err)
	}

	// No patient: the one difference from the patient's row, and see writeAudit for what that column costs when it
	// is filled in anyway.
	return writeAudit(ctx, tx, providerCreated, provider.UserID, nil)
}

// classifyStaff turns the database's answer into one of this package's refusals, where there is one to turn it into.
//
// The second arm is what this route rests on. Postgres gives a missing grant, a missing column grant and a row no
// policy would admit the same 42501, so which refusal was heard is decided on the message and on nothing else — and
// only the last of the three is a statement about the role that was written.
func classifyStaff(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}

	// The database's own error stays in the chain rather than being replaced by its message: a caller — the log,
	// or a test — that wants to check the reading has to reach the refusal it was made from.
	switch {
	case pgErr.Code == uniqueViolation:
		return fmt.Errorf("%w: %w", ErrAlreadyExists, pgErr)

	case pgErr.Code == insufficientPrivilege &&
		strings.Contains(pgErr.Message, rowLevelSecurityRefusal) &&
		strings.Contains(pgErr.Message, profilesTable):
		return fmt.Errorf("%w: %w", ErrServicePathMakesNoAdmins, pgErr)
	}

	return err
}

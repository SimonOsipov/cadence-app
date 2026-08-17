package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// The reasons a patient is not created, each named so a transport can map them to different answers: one error string
// for all of them is how they become a 500.
var (
	// pg_timezone_names is a table rather than a fixed list, so a CHECK cannot consult it and the check lives here.
	ErrUnknownTimezone = errors.New("the timezone is not one this server knows")

	// A foreign key cannot express it: care_team_assignments references profiles, and a CHECK across tables does not
	// exist in Postgres.
	ErrNotAProvider = errors.New("the assigned specialist is not a provider")

	// Caught before anything leaves the process: the database answers it with the same UNIQUE violation as a patient
	// who already exists, and by then the invitation has gone out.
	ErrSpecialistNamedTwice = errors.New("the same specialist is named twice")

	// The profiles primary key speaking, not a lookup this code did — a check-then-insert is two statements with a
	// gap between them.
	ErrAlreadyExists = errors.New("the patient already exists")

	// Should be unreachable: check() refuses a repeated specialist first, so reaching it means a shape check missed.
	ErrAssignmentCollided = errors.New("an assignment already exists")

	// A patient with no care team is invisible to every doctor, which is a state the product has no screen for.
	ErrNoSpecialist = errors.New("a patient needs at least one specialist")

	// The partial unique index refuses it too, but with the same 23505 a duplicate patient produces.
	ErrTwoPrimarySpecialists = errors.New("a patient has one primary specialist")

	// Refused before the database is touched: a cast that fails inside a policy is a 500 where a refusal belongs.
	ErrMalformedIdentifier = errors.New("the identifier is not a UUID")
)

// uniqueViolation is what Postgres answers when a UNIQUE or an exclusion constraint refuses a row.
const uniqueViolation = "23505"

// Assignment is one specialist on a patient's care team.
type Assignment struct {
	// Checked for being a doctor in the same transaction that writes the row.
	ProviderID string

	// What they do for this patient: endo, dietitian or nurse.
	CareRole string

	// At most one per patient, held by a partial unique index.
	Primary bool
}

// NewPatient is everything the clinic knows when it creates a patient. The clinical fields are pointers because
// "not measured yet" and "measured as zero" are different facts about a person.
type NewPatient struct {
	UserID   string
	FullName string
	Timezone string
	Locale   string

	DateOfBirth    *string
	Sex            *string
	HeightCM       *float64
	TargetWeightKG *float64

	Specialists []Assignment
}

// CreatePatient writes a patient and everything that describes them, in one transaction, through the service seam.
//
// Four rows and an audit record, or none of them: a patient with a profile and no care team is invisible to every
// doctor, and one created without an audit row is a change nobody signed.
//
// No policy lets a request do this, so its authorization is in Go; what the database still holds is attribution — the
// audit policy reconciles the row's actor against the one the seam published.
//
// It is the second of a creation's two transactions and not an operation anybody calls on its own: the invitation
// goes out first, under the lock Onboarding.InvitePatient takes.
func CreatePatient(ctx context.Context, pool *pgxpool.Pool, patient NewPatient) error {
	if err := check(patient); err != nil {
		return fmt.Errorf("creating patient %s: %w", patient.UserID, err)
	}

	err := database.WithService(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := requireKnownTimezone(ctx, tx, patient.Timezone); err != nil {
			return err
		}

		for _, assignment := range patient.Specialists {
			if err := requireProvider(ctx, tx, assignment.ProviderID); err != nil {
				return err
			}
		}

		return writePatient(ctx, tx, patient)
	})
	if err != nil {
		return fmt.Errorf("creating patient %s: %w", patient.UserID, classify(err))
	}

	return nil
}

// check is everything refusable without the database, so a malformed caller never takes a connection.
func check(patient NewPatient) error {
	if !database.IsUUIDShaped(patient.UserID) {
		return fmt.Errorf("%q: %w", patient.UserID, ErrMalformedIdentifier)
	}

	return checkBody(patient)
}

// checkBody is the half of check that does not need the account to exist yet: the onboarding flow has to refuse a
// malformed body BEFORE it invites, or every form mistake costs an address — the invitation has gone out and the
// invitee holds a link to an account that will never get a profile.
func checkBody(patient NewPatient) error {
	if len(patient.Specialists) == 0 {
		return ErrNoSpecialist
	}

	primaries := 0
	for _, assignment := range patient.Specialists {
		if !database.IsUUIDShaped(assignment.ProviderID) {
			return fmt.Errorf("%q: %w", assignment.ProviderID, ErrMalformedIdentifier)
		}
		if assignment.Primary {
			primaries++
		}
	}

	if primaries > 1 {
		return fmt.Errorf("%d of them: %w", primaries, ErrTwoPrimarySpecialists)
	}

	return nil
}

// refuseRepeatedSpecialist is the care team's UNIQUE, asked before the write rather than after it.
//
// CreatePatient does not call it, deliberately: the constraint is the authority, and a late refusal is correct for
// every caller that can afford one. The onboarding flow cannot — by the time its transaction opens the invitation has
// gone out, so a refusal there spends the address and tells the doctor it is taken rather than that the form names
// one specialist twice.
func refuseRepeatedSpecialist(patient NewPatient) error {
	named := make(map[string]struct{}, len(patient.Specialists))

	for _, assignment := range patient.Specialists {
		if _, twice := named[assignment.ProviderID]; twice {
			return fmt.Errorf("%q: %w", assignment.ProviderID, ErrSpecialistNamedTwice)
		}
		named[assignment.ProviderID] = struct{}{}
	}

	return nil
}

// requireKnownTimezone asks the server rather than a list: the set changes with the tzdata the server was built
// against, and a copy here would drift silently.
//
// Absence is accepted, and it is the ordinary state of a patient the clinic has just created; the guard below would
// read an empty string as an unknown zone, so the amendment is here rather than at the insert.
func requireKnownTimezone(ctx context.Context, tx pgx.Tx, timezone string) error {
	if timezone == "" {
		return nil
	}

	var known bool
	if err := tx.QueryRow(
		// Qualified: pg_temp is searched before pg_catalog for a relation name, so a temporary table called
		// pg_timezone_names would make this guard pass on anything.
		ctx, `SELECT EXISTS (SELECT FROM pg_catalog.pg_timezone_names WHERE name = $1)`, timezone,
	).Scan(&known); err != nil {
		return fmt.Errorf("checking the timezone: %w", err)
	}

	if !known {
		return fmt.Errorf("%q: %w", timezone, ErrUnknownTimezone)
	}

	return nil
}

// requireProvider runs inside the transaction: a check made outside answers about a row that can change before the
// insert lands.
func requireProvider(ctx context.Context, tx pgx.Tx, providerID string) error {
	var role string
	err := tx.QueryRow(ctx, `SELECT role FROM app.profiles WHERE user_id = $1`, providerID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%q: %w", providerID, ErrNotAProvider)
	}
	if err != nil {
		return fmt.Errorf("reading the specialist's profile: %w", err)
	}

	if role != "doctor" {
		return fmt.Errorf("%q is a %s: %w", providerID, role, ErrNotAProvider)
	}

	return nil
}

func writePatient(ctx context.Context, tx pgx.Tx, patient NewPatient) error {
	// locale is left to the column default when the caller names none, and two statements rather than one with a
	// conditional because DEFAULT is not an expression in Postgres. nullif on the timezone for the other direction:
	// empty is how a patient is created, and an empty string stored there is a zone nothing reports as missing.
	var err error
	if patient.Locale == "" {
		_, err = tx.Exec(ctx, `
			INSERT INTO app.profiles (user_id, role, full_name, timezone)
			VALUES ($1, 'patient', $2, nullif($3, ''))
		`, patient.UserID, patient.FullName, patient.Timezone)
	} else {
		_, err = tx.Exec(ctx, `
			INSERT INTO app.profiles (user_id, role, full_name, timezone, locale)
			VALUES ($1, 'patient', $2, nullif($3, ''), $4)
		`, patient.UserID, patient.FullName, patient.Timezone, patient.Locale)
	}
	if err != nil {
		return fmt.Errorf("writing the profile: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO app.patient_profiles (user_id, dob, sex, height_cm, target_weight_kg, joined_at)
		VALUES ($1, $2, $3, $4, $5, NULL)
	`, patient.UserID, patient.DateOfBirth, patient.Sex,
		patient.HeightCM, patient.TargetWeightKG); err != nil {
		return fmt.Errorf("writing the patient card: %w", err)
	}

	// joined_at stays NULL above on purpose: it is the moment the invitation is accepted, where profiles.created_at
	// is the moment the clinic wrote the row.

	for _, assignment := range patient.Specialists {
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.care_team_assignments (patient_id, provider_id, care_role, is_primary)
			VALUES ($1, $2, $3, $4)
		`, patient.UserID, assignment.ProviderID, assignment.CareRole, assignment.Primary); err != nil {
			return fmt.Errorf("assigning %s: %w", assignment.ProviderID, err)
		}
	}

	// NUT-01 is to extend this transaction with the patient's nutrition_targets row — part of creating a patient
	// rather than a second action behind an endpoint of its own. No migration creates that table yet.

	// The reminder defaults live in the schema; naming none of them here is what keeps them from being duplicated.
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.user_preferences (user_id) VALUES ($1)
	`, patient.UserID); err != nil {
		return fmt.Errorf("writing the preferences: %w", err)
	}

	// Last, and in the same transaction: a rollback takes the audit row with it, and there is nothing to have signed.
	return writeAudit(ctx, tx, patientCreated, patient.UserID, &patient.UserID)
}

// classify turns the database's answer into one of this package's refusals, where there is one to turn it into.
//
// Only UNIQUE: everything else keeps its own error, because a refusal this package does not recognise is a failure
// rather than a rule. The database's own error stays in the chain — a caller reading it is what a wrapped `%s` would
// leave with a string and no error to inspect, which is the divergence classifyStaff did not have.
func classify(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != uniqueViolation {
		return err
	}

	// Which key spoke decides the answer. Collapsed into one they all read as «this address is taken», which is a
	// fact about the address for two refusals that are facts about the form — and the doctor, told the patient
	// exists, has no reason to correct it and retry.
	switch pgErr.ConstraintName {
	case careTeamPairKey:
		return fmt.Errorf("%w: %w", ErrAssignmentCollided, pgErr)

	case careTeamPrimaryKey:
		return fmt.Errorf("%w: %w", ErrTwoPrimarySpecialists, pgErr)
	}

	return fmt.Errorf("%w: %w", ErrAlreadyExists, pgErr)
}

// The two care-team keys that answer 23505 for something the form got wrong rather than for an address already taken.
// checkBody and refuseRepeatedSpecialist refuse both before an invitation goes out; these arms are what the answer is
// when one of those checks is removed or a race gets past it.
const (
	careTeamPairKey    = "care_team_assignments_unique_pair"
	careTeamPrimaryKey = "care_team_assignments_one_primary"
)

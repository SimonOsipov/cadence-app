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

// The reasons a patient is not created. Each is a refusal rather than a failure,
// and each is named so a caller can tell them apart — the transport that will
// eventually sit in front of this has to map them to different answers, and a
// single error string is how that becomes a 500 for all of them.
var (
	// ErrUnknownTimezone means the timezone is not one this server knows.
	// pg_timezone_names is a table rather than a fixed list, so a CHECK cannot
	// consult it and this is where the check lives.
	ErrUnknownTimezone = errors.New("the timezone is not one this server knows")

	// ErrNotAProvider means an assignment names somebody who is not a doctor.
	// A foreign key cannot express it: care_team_assignments references
	// profiles, and a CHECK against another table is not a thing Postgres has.
	ErrNotAProvider = errors.New("the assigned specialist is not a provider")

	// ErrAlreadyExists means the patient, or one of the assignments, is already
	// there. It is the database's UNIQUE speaking, not a lookup this code did:
	// a check-then-insert is two statements with a gap between them.
	ErrAlreadyExists = errors.New("the patient or an assignment already exists")

	// ErrNoSpecialist means nobody was named to look after the patient. A
	// patient with no care team is invisible to every doctor, which is a state
	// the product has no screen for.
	ErrNoSpecialist = errors.New("a patient needs at least one specialist")

	// ErrTwoPrimarySpecialists means more than one specialist was marked as
	// leading. The database refuses it too, through the partial unique index —
	// but it refuses with the same 23505 a duplicate patient produces, and a
	// transport answering "this patient already exists" to somebody who ticked
	// two boxes is answering the wrong question.
	ErrTwoPrimarySpecialists = errors.New("a patient has one primary specialist")

	// ErrMalformedIdentifier means an identifier is not UUID-shaped. Refused
	// before the database is touched, for the same reason the seam refuses a
	// malformed subject: a cast that fails inside a policy is a 500 where a
	// refusal belongs, and it names nothing.
	ErrMalformedIdentifier = errors.New("the identifier is not a UUID")
)

// uniqueViolation is what Postgres answers when a UNIQUE or an exclusion
// constraint refuses a row.
const uniqueViolation = "23505"

// Assignment is one specialist on a patient's care team.
type Assignment struct {
	// ProviderID is the specialist's profile. Checked for being a doctor in the
	// same transaction that writes the row.
	ProviderID string

	// CareRole is what they do for this patient: endo, dietitian or nurse.
	CareRole string

	// Primary marks the one specialist who leads. At most one per patient, and
	// the database holds that with a partial unique index.
	Primary bool
}

// NewPatient is everything the clinic knows when it creates a patient.
//
// The clinical fields are pointers because "not measured yet" and "measured as
// zero" are different facts about a person, and a height of zero is not a
// height. The rest are required.
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

// CreatePatient writes a patient and everything that describes them, in one
// transaction, through the service seam.
//
// Four rows and an audit record, or none of them. That is the whole reason this
// is one function rather than four calls: a patient with a profile and no care
// team is invisible to every doctor, and a patient created without an audit row
// is a change nobody signed.
//
// It runs on the service path because no policy lets a request do this. The
// authorization is therefore in Go rather than in the database, and that is
// written here rather than implied — it is the one place in this system where
// the database is not the last word. What the database still holds is
// attribution: the audit policy reconciles the row's actor against the one the
// seam published, so a write that forgets to name an actor is refused rather
// than recorded anonymously.
//
// There is no HTTP endpoint. Sending the invitation belongs to the same action
// and does not exist yet, and half of an action behind a URL is worse than none
// of it: openapi.json is committed and generates two client surfaces.
//
// Who the patient is created *by* is not a parameter. It is the caller the
// request was verified as, read off the context by the seam — so this function
// has no way of signing the audit row with somebody else's name, and neither has
// whatever eventually calls it.
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

// check is everything that can be refused without asking the database. It runs
// before the transaction opens: a caller whose input is malformed never takes a
// connection, and never gets an answer that came out of a policy.
func check(patient NewPatient) error {
	if !database.IsUUIDShaped(patient.UserID) {
		return fmt.Errorf("%q: %w", patient.UserID, ErrMalformedIdentifier)
	}

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

// requireKnownTimezone asks the server rather than a list. The set is the
// server's, it changes with the tzdata it was built against, and a copy of it
// here would be a second source of truth that drifts silently.
func requireKnownTimezone(ctx context.Context, tx pgx.Tx, timezone string) error {
	var known bool
	if err := tx.QueryRow(
		// Qualified, like every other relation this package names. With the
		// default search_path, pg_temp is searched before pg_catalog for a
		// relation name, so a temporary table called pg_timezone_names would
		// make this guard pass on anything.
		ctx, `SELECT EXISTS (SELECT FROM pg_catalog.pg_timezone_names WHERE name = $1)`, timezone,
	).Scan(&known); err != nil {
		return fmt.Errorf("checking the timezone: %w", err)
	}

	if !known {
		return fmt.Errorf("%q: %w", timezone, ErrUnknownTimezone)
	}

	return nil
}

// requireProvider checks that an assignment names a doctor.
//
// Inside the transaction, not before it: a check made outside would be answering
// about a row that can change before the insert lands.
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
	// locale is left to the column default when the caller names none, rather
	// than defaulted here. The schema already says 'ru', and two copies of a
	// default drift the first time one of them is edited — the same argument the
	// preferences row below rests on.
	//
	// Two statements rather than one with a conditional expression, because
	// DEFAULT is not an expression in Postgres: it may only appear as a whole
	// value in a VALUES list. Each of them is a constant, which is what the
	// authorship gate requires.
	var err error
	if patient.Locale == "" {
		_, err = tx.Exec(ctx, `
			INSERT INTO app.profiles (user_id, role, full_name, timezone)
			VALUES ($1, 'patient', $2, $3)
		`, patient.UserID, patient.FullName, patient.Timezone)
	} else {
		_, err = tx.Exec(ctx, `
			INSERT INTO app.profiles (user_id, role, full_name, timezone, locale)
			VALUES ($1, 'patient', $2, $3, $4)
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

	// joined_at stays NULL above on purpose: it is the moment the invitation is
	// accepted, which has not happened. profiles.created_at is the moment the
	// clinic created the row, and the two are different facts about two
	// different people's actions.

	for _, assignment := range patient.Specialists {
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.care_team_assignments (patient_id, provider_id, care_role, is_primary)
			VALUES ($1, $2, $3, $4)
		`, patient.UserID, assignment.ProviderID, assignment.CareRole, assignment.Primary); err != nil {
			return fmt.Errorf("assigning %s: %w", assignment.ProviderID, err)
		}
	}

	// The defaults are the product's answer to "what should a new patient's
	// reminders be", and they live in the schema. Inserting the row with none of
	// them named is what makes that true rather than duplicated here.
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.user_preferences (user_id) VALUES ($1)
	`, patient.UserID); err != nil {
		return fmt.Errorf("writing the preferences: %w", err)
	}

	// Last, and in the same transaction. A rollback takes the audit row with it,
	// which is right: there is nothing to have signed.
	//
	// The setting names travel as bound parameters rather than being composed
	// into the statement — current_setting takes its name as an argument, so
	// there is no identifier to concatenate. That keeps the statement a constant,
	// which the authorship gate requires, while the names themselves stay the
	// seam's own constants: a copy of the two strings here would be a contract
	// with no compile-time link, and renaming them there would leave this package
	// compiling and failing at runtime.
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.audit_log (actor_id, actor_job, action, entity, entity_id, patient_id)
		VALUES (
			nullif(current_setting($2, true), '')::uuid,
			nullif(current_setting($3, true), ''),
			'patient.create', 'profiles', $1, $1
		)
	`, patient.UserID, database.ActorIDSetting, database.ActorJobSetting); err != nil {
		return fmt.Errorf("writing the audit record: %w", err)
	}

	return nil
}

// classify turns the database's answer into one of this package's refusals,
// where there is one to turn it into.
//
// Only UNIQUE, and deliberately: a check-then-insert would be two statements
// with a gap, and the gap is where two clinics adding the same patient at the
// same time both find nothing and both proceed. Everything else keeps its own
// error, because a refusal this package does not recognise is a failure rather
// than a rule.
func classify(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return fmt.Errorf("%w: %s", ErrAlreadyExists, pgErr.ConstraintName)
	}

	return err
}

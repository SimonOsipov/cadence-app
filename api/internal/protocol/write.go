package protocol

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

var (
	// The authorization no policy holds. The service path's predicates are USING (true),
	// so a doctor writing a course for somebody who is not their patient is refused here
	// or nowhere — this is the one place in this context where the database is not the
	// last word, and it is written down rather than implied.
	ErrNotYourPatient = errors.New("the patient is not on this caller's care team")

	// A patient reaching this path at all is a wiring mistake: they have no route to it,
	// and the check exists so that a future one cannot arrive without a decision.
	ErrNotAPrescriber = errors.New("only a doctor or an admin prescribes")

	// The partial unique index speaking. Ending a course is a status change, so this is
	// «finish the one that is running» rather than a retry.
	ErrAlreadyRunning = errors.New("the patient already has a course running")

	ErrNoSuchProtocol = errors.New("no such course")

	// Refused before the seam opens: a cast that fails inside a transaction is a 500
	// where a refusal belongs. It lives here rather than in Draft.Check because the
	// generator may not import the database package, and IsUUIDShaped is its.
	ErrMalformedIdentifier = errors.New("the identifier is not a UUID")
)

// Written is what the caller gets back: the identifiers the database chose, in the order the
// items and phases were sent, so a client can match its own form rows to them.
type Written struct {
	ProtocolID ProtocolID
	ItemIDs    []ProtocolItemID
}

// Create writes a course, its items and their phases in one transaction, through the service
// seam, with an audit row.
//
// The service seam and not the request one, because this is a cross-actor write: the doctor
// prescribes for somebody else, and `cadence_doctor` holds no INSERT on any of these tables.
// What that costs is that RLS answers nothing here, which is why the care-team check above
// runs first and why it is the subject of its own test.
func Create(ctx context.Context, pool *pgxpool.Pool, draft Draft) (Written, error) {
	if err := draft.Check(); err != nil {
		return Written{}, err
	}
	if !database.IsUUIDShaped(string(draft.PatientID)) {
		return Written{}, fmt.Errorf("%q: %w", draft.PatientID, ErrMalformedIdentifier)
	}

	var written Written
	err := database.WithService(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := requireCaresFor(ctx, tx, draft.PatientID); err != nil {
			return err
		}

		var err error
		written, err = write(ctx, tx, draft)
		if err != nil {
			return err
		}

		return writeAudit(ctx, tx, "protocol.create", string(written.ProtocolID), string(draft.PatientID))
	})
	if err != nil {
		return Written{}, fmt.Errorf("prescribing for %s: %w", draft.PatientID, classify(err))
	}

	return written, nil
}

// Replace rewrites a course's items and phases, keeping the course row and its identifier.
//
// Delete and insert rather than a diff. §03 gives an item no identity a client holds across
// an edit — the form sends a list, not a patch — and the generator reads items and phases
// together, so a half-applied edit is a schedule nobody prescribed. The dose events that
// answered a removed item hold it in place: 000019 makes that reference RESTRICT, so an item
// somebody has injected cannot be dropped, and the caller is told which.
func Replace(ctx context.Context, pool *pgxpool.Pool, id ProtocolID, draft Draft) (Written, error) {
	if err := draft.Check(); err != nil {
		return Written{}, err
	}
	for _, identifier := range []string{string(draft.PatientID), string(id)} {
		if !database.IsUUIDShaped(identifier) {
			return Written{}, fmt.Errorf("%q: %w", identifier, ErrMalformedIdentifier)
		}
	}

	var written Written
	err := database.WithService(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		// Authorized on the path's patient before a row is read, and the order is the
		// finding rather than a style: reading the owner first made the reply an
		// oracle. An unassigned doctor holding a course id got 403 where the course
		// existed and belonged to the named patient, and 404 otherwise — one bit of
		// protocols.patient_id per request, to a caller RLS would show nothing.
		// identity's own write path authorizes before it reads for the same reason.
		if err := requireCaresFor(ctx, tx, draft.PatientID); err != nil {
			return err
		}

		owner, err := ownerOf(ctx, tx, id)
		if err != nil {
			return err
		}
		// The owner is not named in the refusal: it is the one thing this caller must
		// not learn, and the course id alone identifies the refusal in a log.
		if owner != draft.PatientID {
			return fmt.Errorf("course %s: %w", id, ErrNoSuchProtocol)
		}

		if _, err := tx.Exec(ctx, `
			UPDATE app.protocols
			SET start_date = $2::date, duration_weeks = $3, status = $4, notes = $5
			WHERE id = $1
		`, string(id), draft.StartDate.String(), draft.Weeks, string(draft.Status), draft.Notes); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM app.protocol_items WHERE protocol_id = $1`, string(id)); err != nil {
			return err
		}

		written.ProtocolID = id
		if err := writeItems(ctx, tx, &written, draft); err != nil {
			return err
		}

		return writeAudit(ctx, tx, "protocol.replace", string(id), string(draft.PatientID))
	})
	if err != nil {
		return Written{}, fmt.Errorf("rewriting course %s: %w", id, classify(err))
	}

	return written, nil
}

func write(ctx context.Context, tx pgx.Tx, draft Draft) (Written, error) {
	var written Written
	if err := tx.QueryRow(ctx, `
		INSERT INTO app.protocols (patient_id, start_date, duration_weeks, status, created_by, notes)
		VALUES ($1, $2::date, $3, $4, nullif(current_setting($6, true), '')::uuid, $5)
		RETURNING id::text
	`, string(draft.PatientID), draft.StartDate.String(), draft.Weeks, string(draft.Status),
		draft.Notes, database.ActorIDSetting).Scan(&written.ProtocolID); err != nil {
		return Written{}, err
	}

	return written, writeItems(ctx, tx, &written, draft)
}

func writeItems(ctx context.Context, tx pgx.Tx, written *Written, draft Draft) error {
	written.ItemIDs = make([]ProtocolItemID, 0, len(draft.Items))

	for n, item := range draft.Items {
		var compound *CompoundID
		if item.Kind == KindInjection {
			resolved, err := ResolveCompound(ctx, tx, item.Compound)
			if err != nil {
				return fmt.Errorf("item %d: %w", n+1, err)
			}
			compound = &resolved
		}

		days := make([]int16, len(item.DaysOfWeek))
		for i, day := range item.DaysOfWeek {
			days[i] = int16(civil.ISOWeekday(day))
		}
		times := make([]string, len(item.Times))
		for i, slot := range item.Times {
			times[i] = slot.String()
		}

		var itemID ProtocolItemID
		if err := tx.QueryRow(ctx, `
			INSERT INTO app.protocol_items
			    (protocol_id, kind, compound_id, cadence, days_of_week, times, loggable)
			VALUES ($1, $2, $3, $4, $5::smallint[], $6::time[], $7)
			RETURNING id::text
		`, string(written.ProtocolID), string(item.Kind), compound, string(item.Cadence),
			days, times, item.Loggable).Scan(&itemID); err != nil {
			return fmt.Errorf("item %d: %w", n+1, err)
		}
		written.ItemIDs = append(written.ItemIDs, itemID)

		for p, phase := range item.Phases {
			if _, err := tx.Exec(ctx, `
				INSERT INTO app.protocol_phases
				    (protocol_item_id, from_week, to_week, dose_value, dose_unit)
				VALUES ($1, $2, $3, $4, $5)
			`, string(itemID), phase.FromWeek, phase.ToWeek,
				phase.Dose.Value, string(phase.Dose.Unit)); err != nil {
				return fmt.Errorf("item %d, phase %d: %w", n+1, p+1, err)
			}
		}
	}

	return nil
}

// requireCaresFor is the whole of this path's authorization, and it reads care_team_assignments
// directly rather than through a policy: inside the service seam there is no policy to read
// through. An admin is not on anybody's care team and is allowed regardless — they are not a
// provider, and no assignment would ever name them.
func requireCaresFor(ctx context.Context, tx pgx.Tx, patient civil.UserID) error {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return ErrNotAPrescriber
	}

	switch principal.Role {
	case "admin":
		return nil
	case "doctor":
	default:
		return fmt.Errorf("%s: %w", principal.Role, ErrNotAPrescriber)
	}

	var assigned bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM app.care_team_assignments
			WHERE patient_id = $1 AND provider_id = $2
		)
	`, string(patient), principal.Subject).Scan(&assigned); err != nil {
		return fmt.Errorf("reading the care team: %w", err)
	}
	if !assigned {
		return fmt.Errorf("%s: %w", patient, ErrNotYourPatient)
	}

	return nil
}

func ownerOf(ctx context.Context, tx pgx.Tx, id ProtocolID) (civil.UserID, error) {
	var owner civil.UserID
	err := tx.QueryRow(ctx,
		`SELECT patient_id::text FROM app.protocols WHERE id = $1`, string(id)).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("%s: %w", id, ErrNoSuchProtocol)
	}
	if err != nil {
		return "", fmt.Errorf("reading course %s: %w", id, err)
	}

	return owner, nil
}

// classify turns the constraints that answer a legitimate request into named errors. Only
// those: everything else keeps its own error, because a catch-all here is how a bug becomes a
// 409 that the doctor retries forever.
//
// Draft.Check refuses the shape rules before a connection is taken, so what reaches here is
// the race it cannot see — two doctors prescribing at once — and the one rule the schema
// holds alone.
func classify(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}

	switch {
	case pgErr.Code == uniqueViolation && pgErr.ConstraintName == oneActivePerPatient:
		return ErrAlreadyRunning
	case pgErr.Code == foreignKeyViolation && pgErr.ConstraintName == doseEventsHoldTheirItem:
		return ErrItemHasBeenInjected
	default:
		return err
	}
}

const (
	uniqueViolation     = "23505"
	foreignKeyViolation = "23503"

	// A partial unique index, so its name is what pg reports rather than a constraint's.
	oneActivePerPatient = "protocols_one_active_per_patient"

	// 000019's RESTRICT: an item somebody has injected is held in place by the doses that
	// answered it, which is what stops an edit of the course from erasing them.
	doseEventsHoldTheirItem = "dose_events_answer_an_item_of_that_course"
)

// ErrItemHasBeenInjected is a refusal with a remedy, not a failure: the course keeps its
// history, and the item is stopped by clearing `loggable` rather than by removal.
var ErrItemHasBeenInjected = errors.New("an item the patient has injected cannot be removed")

func writeAudit(ctx context.Context, tx pgx.Tx, action, entityID, patientID string) error {
	// The setting names travel as bound parameters, which keeps the statement the
	// constant the authorship gate requires.
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.audit_log (actor_id, actor_job, action, entity, entity_id, patient_id)
		VALUES (
			nullif(current_setting($3, true), '')::uuid,
			nullif(current_setting($4, true), ''),
			$2, 'protocols', $1, $5
		)
	`, entityID, action, database.ActorIDSetting, database.ActorJobSetting, patientID); err != nil {
		return fmt.Errorf("writing the audit record: %w", err)
	}

	return nil
}

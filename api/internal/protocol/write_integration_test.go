//go:build integration

package protocol_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

const (
	writePatientA = "5d4f3b7c-0000-4000-8000-0000000000a1"
	writePatientB = "5d4f3b7c-0000-4000-8000-0000000000b1"
	writeDoctorA  = "5d4f3b7c-0000-4000-8000-0000000000a2"
	writeDoctorB  = "5d4f3b7c-0000-4000-8000-0000000000b2"
	writeAdmin    = "5d4f3b7c-0000-4000-8000-0000000000c1"
	writeJob      = "test.protocol.write"
)

// Two patients and two doctors assigned crosswise: «only their own patients» is not a
// statement one patient can carry.
func prescribing(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()

	db := cluster.NewDatabase(t)
	pool, err := database.NewPool(t.Context(), db.ServiceAppURL)
	if err != nil {
		t.Fatalf("opening the service pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := database.WithServiceJob(
		t.Context(), pool, writeJob,
		func(ctx context.Context, tx pgx.Tx) error {
			for id, name := range map[string]string{writeDoctorA: "Марина", writeDoctorB: "Ольга"} {
				if _, err := tx.Exec(ctx, `
					INSERT INTO app.profiles (user_id, role, full_name, timezone)
					VALUES ($1, 'doctor', $2, 'Europe/Moscow')
				`, id, name); err != nil {
					return err
				}
			}
			for patient, doctor := range map[string]string{
				writePatientA: writeDoctorA, writePatientB: writeDoctorB,
			} {
				if _, err := tx.Exec(ctx, `
					INSERT INTO app.profiles (user_id, role, full_name, timezone)
					VALUES ($1, 'patient', 'Пациент', 'Asia/Yekaterinburg')
				`, patient); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx,
					`INSERT INTO app.patient_profiles (user_id) VALUES ($1)`, patient); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `
					INSERT INTO app.care_team_assignments (patient_id, provider_id, care_role, is_primary)
					VALUES ($1, $2, 'endo', true)
				`, patient, doctor); err != nil {
					return err
				}
			}
			return nil
		},
	); err != nil {
		t.Fatalf("seeding the clinic: %v", err)
	}

	// The admin is seeded as the superuser: 000006 refuses one on the service path, and
	// that refusal is a rule of the schema rather than an inconvenience of this test.
	superuser := testsupport.Connect(t, db.SuperuserURL)
	if _, err := superuser.Exec(t.Context(), `
		INSERT INTO app.profiles (user_id, role, full_name, timezone)
		VALUES ($1, 'admin', 'Пётр', 'Europe/Moscow')
	`, writeAdmin); err != nil {
		t.Fatalf("seeding the admin: %v", err)
	}

	return pool, db.SuperuserURL
}

func as(t *testing.T, subject, role string) context.Context {
	t.Helper()

	return auth.WithPrincipal(t.Context(), auth.Principal{Subject: subject, Role: role})
}

func aCourse(patient string) protocol.Draft {
	draft := protocol.Draft{
		PatientID: civil.UserID(patient),
		StartDate: civil.NewDate(2026, time.May, 4),
		Weeks:     12,
		Status:    protocol.StatusActive,
		Items: []protocol.DraftItem{{
			Kind: protocol.KindInjection,
			Compound: protocol.CompoundRef{New: &protocol.NewCompound{
				NameRU: "Семаглутид", DefaultUnit: protocol.MG, Route: "sc", Icon: "syringe",
			}},
			Cadence:    protocol.CadenceWeekly,
			DaysOfWeek: []time.Weekday{time.Sunday},
			Times:      []civil.Slot{{Hour: 8}},
			Loggable:   true,
			Phases: []protocol.ProtocolPhase{
				{FromWeek: 1, ToWeek: 4, Dose: protocol.Dose{Value: 0.25, Unit: protocol.MG}},
				{FromWeek: 5, ToWeek: 12, Dose: protocol.Dose{Value: 0.5, Unit: protocol.MG}},
			},
		}},
	}
	return draft
}

// The authorization no policy holds. Inside the service seam every predicate is USING (true),
// so a doctor prescribing for somebody who is not their patient is refused in Go or nowhere.
func TestOnlyTheCareTeamPrescribes(t *testing.T) {
	pool, _ := prescribing(t)
	draft := aCourse(writePatientA)

	for _, caller := range []struct {
		name    string
		subject string
		role    string
		want    error
	}{
		{"the assigned doctor", writeDoctorA, "doctor", nil},
		{"the other doctor", writeDoctorB, "doctor", protocol.ErrNotYourPatient},
		{"the patient themselves", writePatientA, "patient", protocol.ErrNotAPrescriber},
		{"a doctor of nobody", "5d4f3b7c-0000-4000-8000-00000000dead", "doctor", protocol.ErrNotYourPatient},
	} {
		t.Run(caller.name, func(t *testing.T) {
			_, err := protocol.Create(as(t, caller.subject, caller.role), pool, draft)
			if !errors.Is(err, caller.want) {
				t.Errorf("got %v, want %v", err, caller.want)
			}
		})
	}

	// The admin, who is on nobody's care team and prescribes anyway — and last, because
	// the assigned doctor's course above is the one the partial index counts.
	if _, err := protocol.Create(as(t, writeAdmin, "admin"), pool, draft); !errors.Is(
		err, protocol.ErrAlreadyRunning,
	) {
		t.Errorf("the admin got %v, want the running-course refusal rather than an authorization one", err)
	}

	// And with no principal at all, which is a wiring mistake rather than a caller.
	if _, err := protocol.Create(t.Context(), pool, draft); err == nil {
		t.Error("a request with no verified caller wrote a course")
	}
}

// One transaction: the course, its items, its phases and the audit row, or none of them.
func TestACourseArrivesWholeAndSigned(t *testing.T) {
	pool, superuser := prescribing(t)
	draft := aCourse(writePatientA)

	written, err := protocol.Create(as(t, writeDoctorA, "doctor"), pool, draft)
	if err != nil {
		t.Fatalf("prescribing: %v", err)
	}
	if len(written.ItemIDs) != 1 {
		t.Fatalf("wrote %d items, want 1", len(written.ItemIDs))
	}

	rows := countRows(t, pool, string(written.ProtocolID))
	if rows.items != 1 || rows.phases != 2 {
		t.Errorf("wrote %d items and %d phases, want 1 and 2", rows.items, rows.phases)
	}

	// The audit row, and who it names: the doctor, not a job. The seam derives the actor
	// from the principal, so a course signed by nobody is a course nobody prescribed.
	actor, action, patient := auditFor(t, superuser, string(written.ProtocolID))
	if actor != writeDoctorA || action != "protocol.create" || patient != writePatientA {
		t.Errorf("the audit row reads %s/%s/%s", actor, action, patient)
	}

	// created_by is the prescriber too, and it is the column that survives them leaving.
	if by := createdBy(t, pool, string(written.ProtocolID)); by != writeDoctorA {
		t.Errorf("the course was created by %s, want the doctor", by)
	}

	// Nothing is written when the draft is refused, and the refusal comes before a
	// connection is taken: a malformed course must not spend a transaction.
	bad := aCourse(writePatientB)
	bad.Items[0].Phases[1].FromWeek = 2
	if _, err := protocol.Create(as(t, writeDoctorB, "doctor"), pool, bad); !errors.Is(
		err, protocol.ErrPhasesOverlap,
	) {
		t.Errorf("overlapping phases gave %v", err)
	}
	if courses := coursesOf(t, pool, writePatientB); courses != 0 {
		t.Errorf("the other patient has %d courses after a refusal", courses)
	}
}

// An edit replaces the items rather than patching them — but not the ones a patient has
// already injected: 000019 holds those in place, and the caller is told why.
func TestAnEditKeepsWhatThePatientHasAlreadyInjected(t *testing.T) {
	pool, superuser := prescribing(t)
	draft := aCourse(writePatientA)

	written, err := protocol.Create(as(t, writeDoctorA, "doctor"), pool, draft)
	if err != nil {
		t.Fatalf("prescribing: %v", err)
	}

	logDose(t, pool, writePatientA, string(written.ProtocolID), string(written.ItemIDs[0]))

	// Titration during a course, which is this product's main clinical loop and was
	// impossible: the first version of Replace deleted every item unconditionally, so
	// one logged dose made the whole course uneditable — its status included, and the
	// `loggable` remedy the refusal names needed the very statement being refused.
	kept := written.ItemIDs[0]
	draft.Items[0].ID = &kept
	draft.Weeks = 16
	draft.Items[0].Phases[1].ToWeek = 16
	draft.Items[0].Times = []civil.Slot{{Hour: 20}}

	edited, err := protocol.Replace(as(t, writeDoctorA, "doctor"), pool, written.ProtocolID, draft)
	if err != nil {
		t.Fatalf("titrating a course with a dose logged against it: %v", err)
	}
	if edited.ItemIDs[0] != kept {
		t.Errorf("the item became %s, want the one the dose answers", edited.ItemIDs[0])
	}
	if rows := countRows(t, pool, string(written.ProtocolID)); rows.items != 1 || rows.phases != 2 {
		t.Errorf("after the edit: %d items and %d phases", rows.items, rows.phases)
	}
	if doses := dosesOn(t, pool, string(kept)); doses != 1 {
		t.Errorf("the item now answers %d doses, want the one that was logged", doses)
	}
	if slot := slotOf(t, pool, string(kept)); slot != "20:00:00" {
		t.Errorf("the item's slot is %s, want the edit to have landed", slot)
	}

	if actor, action, patient := auditFor(t, superuser, "protocol.replace"); actor != writeDoctorA ||
		action != "protocol.replace" || patient != writePatientA {
		t.Errorf("the edit's audit row reads %s/%s/%s", actor, action, patient)
	}

	// Cancelling it, which is the other statement the unconditional delete foreclosed.
	draft.Status = protocol.StatusCancelled
	if _, err := protocol.Replace(
		as(t, writeDoctorA, "doctor"), pool, written.ProtocolID, draft,
	); err != nil {
		t.Fatalf("cancelling a course with a logged dose: %v", err)
	}

	// And the item itself still cannot be dropped: the doses that answered it are the
	// clinic's record, and the way to stop it is to clear loggable.
	draft.Items[0].ID = nil
	if _, err := protocol.Replace(
		as(t, writeDoctorA, "doctor"), pool, written.ProtocolID, draft,
	); !errors.Is(err, protocol.ErrItemHasBeenInjected) {
		t.Errorf("dropping an injected item gave %v, want ErrItemHasBeenInjected", err)
	}
	if rows := countRows(t, pool, string(written.ProtocolID)); rows.items != 1 {
		t.Errorf("the refused edit left %d items", rows.items)
	}
	if doses := dosesOn(t, pool, string(kept)); doses != 1 {
		t.Errorf("the refused edit left %d doses", doses)
	}
}

// An item of another course may not be rewritten by naming it: the service seam carries no
// row predicate, so the UPDATE is scoped to the course as well as to the row and the caller
// is told the course does not hold it.
func TestAnItemOfAnotherCourseCannotBeNamed(t *testing.T) {
	pool, _ := prescribing(t)

	mine, err := protocol.Create(as(t, writeDoctorA, "doctor"), pool, aCourse(writePatientA))
	if err != nil {
		t.Fatalf("prescribing: %v", err)
	}
	theirs, err := protocol.Create(as(t, writeDoctorB, "doctor"), pool, aCourse(writePatientB))
	if err != nil {
		t.Fatalf("prescribing for the other patient: %v", err)
	}

	draft := aCourse(writePatientA)
	borrowed := theirs.ItemIDs[0]
	draft.Items[0].ID = &borrowed

	if _, err := protocol.Replace(
		as(t, writeDoctorA, "doctor"), pool, mine.ProtocolID, draft,
	); !errors.Is(err, protocol.ErrNoSuchItem) {
		t.Errorf("naming another course's item gave %v, want ErrNoSuchItem", err)
	}
	if slot := slotOf(t, pool, string(borrowed)); slot != "08:00:00" {
		t.Errorf("the other patient's item now reads %s", slot)
	}
}

// Another doctor's course is not theirs to rewrite, and the refusal must not tell them it
// exists: the reply is the same one an identifier nobody holds gets.
func TestAnEditReachesOnlyTheCallersOwnPatients(t *testing.T) {
	pool, _ := prescribing(t)
	draft := aCourse(writePatientA)

	written, err := protocol.Create(as(t, writeDoctorA, "doctor"), pool, draft)
	if err != nil {
		t.Fatalf("prescribing: %v", err)
	}

	// The same answer for a course that exists and for an identifier nobody holds,
	// and this is the axis the first version of this test left open: reading the
	// owner before authorizing made the reply 403 where the course existed and
	// belonged to the named patient, and 404 otherwise. One bit of protocols.
	// patient_id per request, to a caller RLS would show nothing.
	for _, course := range []protocol.ProtocolID{
		written.ProtocolID,
		"5d4f3b7c-0000-4000-8000-00000000dead",
	} {
		if _, err := protocol.Replace(
			as(t, writeDoctorB, "doctor"), pool, course, draft,
		); !errors.Is(err, protocol.ErrNotYourPatient) {
			t.Errorf("the other doctor got %v for course %s", err, course)
		}
	}

	// And a course claimed for a patient it does not belong to, which is the same shape
	// as step 2's reparenting: the identifier is real and the patient is the caller's.
	stolen := aCourse(writePatientB)
	if _, err := protocol.Replace(
		as(t, writeDoctorB, "doctor"), pool, written.ProtocolID, stolen,
	); !errors.Is(err, protocol.ErrNoSuchProtocol) {
		t.Errorf("rewriting one patient's course as another's gave %v", err)
	}
	if owner := ownerOfCourse(t, pool, string(written.ProtocolID)); owner != writePatientA {
		t.Errorf("the course now belongs to %s", owner)
	}

	// And the refusal names no owner: the course id identifies it in a log, and the
	// patient it belongs to is the one thing this caller must not learn.
	_, err = protocol.Replace(as(t, writeDoctorB, "doctor"), pool, written.ProtocolID, stolen)
	if err != nil && strings.Contains(err.Error(), writePatientA) {
		t.Errorf("the refusal names the owner: %q", err)
	}
}

// A malformed identifier is refused before a connection is taken: a cast that fails inside a
// policy is a 500 where a refusal belongs. Unreachable over HTTP, where huma validates the
// path parameter — this guards the in-process callers, cmd/seed among them.
func TestAMalformedIdentifierIsRefusedBeforeTheSeamOpens(t *testing.T) {
	pool, _ := prescribing(t)

	malformed := aCourse("not-a-uuid")
	if _, err := protocol.Create(
		as(t, writeDoctorA, "doctor"), pool, malformed,
	); !errors.Is(err, protocol.ErrMalformedIdentifier) {
		t.Errorf("creating for a malformed patient gave %v", err)
	}

	good := aCourse(writePatientA)
	if _, err := protocol.Replace(
		as(t, writeDoctorA, "doctor"), pool, "not-a-uuid", good,
	); !errors.Is(err, protocol.ErrMalformedIdentifier) {
		t.Errorf("rewriting a malformed course gave %v", err)
	}
}

type rowCounts struct{ items, phases int }

func countRows(t *testing.T, pool *pgxpool.Pool, id string) rowCounts {
	t.Helper()

	var counted rowCounts
	ask(t, pool, `
		SELECT
		    (SELECT count(*) FROM app.protocol_items WHERE protocol_id = $1),
		    (SELECT count(*) FROM app.protocol_phases p
		       JOIN app.protocol_items i ON i.id = p.protocol_item_id
		      WHERE i.protocol_id = $1)
	`, []any{id}, &counted.items, &counted.phases)

	return counted
}

// The service role holds INSERT on audit_log and deliberately not SELECT — the path that
// signs an act cannot read the trail back — so this reads as the superuser.
func auditFor(t *testing.T, superuserURL, of string) (string, string, string) {
	t.Helper()

	conn := testsupport.Connect(t, superuserURL)

	// By entity id or by action, because both questions arise: «what did this course
	// collect» and «was the edit signed at all».
	var actor, action, patient string
	if err := conn.QueryRow(t.Context(), `
		SELECT coalesce(actor_id::text, ''), action, coalesce(patient_id::text, '')
		FROM app.audit_log WHERE entity_id::text = $1 OR action = $1
	`, of).Scan(&actor, &action, &patient); err != nil {
		t.Fatalf("reading the audit trail: %v", err)
	}

	return actor, action, patient
}

func createdBy(t *testing.T, pool *pgxpool.Pool, id string) string {
	t.Helper()

	var by string
	ask(t, pool, `SELECT coalesce(created_by::text, '') FROM app.protocols WHERE id = $1`, []any{id}, &by)

	return by
}

func ownerOfCourse(t *testing.T, pool *pgxpool.Pool, id string) string {
	t.Helper()

	var owner string
	ask(t, pool, `SELECT patient_id::text FROM app.protocols WHERE id = $1`, []any{id}, &owner)

	return owner
}

func coursesOf(t *testing.T, pool *pgxpool.Pool, patient string) int {
	t.Helper()

	var count int
	ask(t, pool, `SELECT count(*) FROM app.protocols WHERE patient_id = $1`, []any{patient}, &count)

	return count
}

// logDose puts a dose event against the item, which is what makes the item unremovable.
func logDose(t *testing.T, pool *pgxpool.Pool, patient, course, item string) {
	t.Helper()

	if err := database.WithServiceJob(
		t.Context(), pool, writeJob,
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.dose_events
				    (patient_id, protocol_id, protocol_item_id, scheduled_for_date,
				     injected_at, dose_value, dose_unit, client_request_id)
				VALUES ($1, $2, $3, DATE '2026-05-10',
				        TIMESTAMPTZ '2026-05-10 08:00:00+05', 0.25, 'мг', 'edit-guard-01')
			`, patient, course, item)

			return err
		},
	); err != nil {
		t.Fatalf("logging a dose: %v", err)
	}
}

func dosesOn(t *testing.T, pool *pgxpool.Pool, item string) int {
	t.Helper()

	var doses int
	ask(t, pool, `SELECT count(*) FROM app.dose_events WHERE protocol_item_id = $1`, []any{item}, &doses)

	return doses
}

func slotOf(t *testing.T, pool *pgxpool.Pool, item string) string {
	t.Helper()

	var slot string
	ask(t, pool, `SELECT times[1]::text FROM app.protocol_items WHERE id = $1`, []any{item}, &slot)

	return slot
}

func ask(t *testing.T, pool *pgxpool.Pool, query string, args []any, into ...any) {
	t.Helper()

	if err := database.WithServiceJob(
		t.Context(), pool, writeJob,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, query, args...).Scan(into...)
		},
	); err != nil {
		t.Fatalf("reading: %v", err)
	}
}

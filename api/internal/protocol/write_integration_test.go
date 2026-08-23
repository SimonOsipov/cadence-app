//go:build integration

package protocol_test

import (
	"context"
	"errors"
	"slices"
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

// theRequestURL is the last clinic's request-seam URL. A package-level handoff rather than a
// second return value, because only one test needs it and every other call site would carry
// a parameter it ignores.
var theRequestURL string

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

	theRequestURL = db.AppURL

	return pool, db.SuperuserURL
}

// prescribingWithRequests is the same clinic with the request pool as well, for the reads
// that run under the caller's own identity rather than through the service seam.
func prescribingWithRequests(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool) {
	t.Helper()

	service, _ := prescribing(t)
	requests, err := database.NewPool(t.Context(), theRequestURL)
	if err != nil {
		t.Fatalf("opening the request pool: %v", err)
	}
	t.Cleanup(requests.Close)

	return service, requests
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
	// The seam refuses it before requireCaresFor is reached, and naming that is the
	// point: by here a course is already running, so «some error» is what the partial
	// index would answer with every guard gone.
	if _, err := protocol.Create(t.Context(), pool, draft); !errors.Is(err, database.ErrNoPrincipal) {
		t.Errorf("a request with no verified caller gave %v", err)
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

// The three kinds §03 gives a course, and the mixed edit rewriteItems exists for: one item
// named and rewritten, one unnamed and added, one omitted and dropped, in the same call.
//
// A one-item fixture collapsed two axes into one — with a single item, «what was kept» and
// «what was written» are the same set, so a `kept` built from updates alone, or a delete run
// before the loop, was indistinguishable. It also meant «врач заводит протокол с позициями
// трёх видов» — an acceptance criterion — never reached the database at all, and the branch
// that leaves compound_id NULL for a kind that is not a drug ran only in unit tests.
func TestACourseOfThreeKindsIsWrittenAndThenEditedInPlace(t *testing.T) {
	pool, _ := prescribing(t)

	draft := aCourse(writePatientA)
	draft.Items = append(draft.Items,
		aKind(protocol.KindSupplement, civil.Slot{Hour: 9}),
		aKind(protocol.KindWeighIn, civil.Slot{Hour: 7}),
	)

	written, err := protocol.Create(as(t, writeDoctorA, "doctor"), pool, draft)
	if err != nil {
		t.Fatalf("prescribing three kinds: %v", err)
	}
	if len(written.ItemIDs) != 3 {
		t.Fatalf("wrote %d items, want 3", len(written.ItemIDs))
	}
	if kinds := kindsOf(t, pool, string(written.ProtocolID)); kinds != "injection,supplement,weigh_in" {
		t.Errorf("the course holds %s", kinds)
	}
	// The branch that leaves the column NULL: only an injection names a drug.
	if named := itemsNamingADrug(t, pool, string(written.ProtocolID)); named != 1 {
		t.Errorf("%d items name a drug, want the injection alone", named)
	}

	// A second patient with a course of their own, because every fixture here has held
	// exactly one: with one course in the database, `DELETE … WHERE protocol_id = $1
	// AND NOT (id = ANY(…))` is indistinguishable from the same statement without its
	// first predicate — and without it, an edit deletes every protocol item in the
	// database that this request did not name.
	theirs, err := protocol.Create(as(t, writeDoctorB, "doctor"), pool, aCourse(writePatientB))
	if err != nil {
		t.Fatalf("prescribing for the other patient: %v", err)
	}
	before := countRows(t, pool, string(theirs.ProtocolID))

	// The mixed edit: keep the supplement and rewrite it, add a second injection, drop
	// the weigh-in and the first injection.
	kept := written.ItemIDs[1]
	edited := aCourse(writePatientA)
	edited.Items = []protocol.DraftItem{
		func() protocol.DraftItem {
			item := aKind(protocol.KindSupplement, civil.Slot{Hour: 21})
			item.ID = &kept

			return item
		}(),
		aCourse(writePatientA).Items[0],
	}

	after, err := protocol.Replace(as(t, writeDoctorA, "doctor"), pool, written.ProtocolID, edited)
	if err != nil {
		t.Fatalf("the mixed edit: %v", err)
	}
	if len(after.ItemIDs) != 2 || after.ItemIDs[0] != kept {
		t.Errorf("the reply carries %v, want the kept item first", after.ItemIDs)
	}
	if after.ItemIDs[1] == written.ItemIDs[0] || after.ItemIDs[1] == written.ItemIDs[2] {
		t.Errorf("the added item reused a dropped identifier: %s", after.ItemIDs[1])
	}
	if rows := countRows(t, pool, string(written.ProtocolID)); rows.items != 2 {
		t.Errorf("the course now holds %d items, want 2", rows.items)
	}
	if slot := slotOf(t, pool, string(kept)); slot != "21:00:00" {
		t.Errorf("the kept item reads %s, want the rewrite to have landed", slot)
	}
	for _, dropped := range []protocol.ProtocolItemID{written.ItemIDs[0], written.ItemIDs[2]} {
		if stillThere(t, pool, string(dropped)) {
			t.Errorf("item %s outlived an edit that omitted it", dropped)
		}
	}
	if after := countRows(t, pool, string(theirs.ProtocolID)); after != before {
		t.Errorf("the other patient's course went from %+v to %+v", before, after)
	}
}

// The caller's own spelling of an identifier is not the database's. Both IsUUIDShaped and
// huma accept an uppercase uuid, and the sweep used to compare `id::text` — always canonical
// lowercase — against what the caller sent: the item was rewritten in place by one statement
// and deleted by the next, with its phases, having been named to keep it.
func TestAnItemKeptUnderAnotherSpellingIsStillKept(t *testing.T) {
	pool, _ := prescribing(t)

	written, err := protocol.Create(as(t, writeDoctorA, "doctor"), pool, aCourse(writePatientA))
	if err != nil {
		t.Fatalf("prescribing: %v", err)
	}

	shouted := protocol.ProtocolItemID(strings.ToUpper(string(written.ItemIDs[0])))
	draft := aCourse(writePatientA)
	draft.Items[0].ID = &shouted
	draft.Items[0].Times = []civil.Slot{{Hour: 20}}

	if _, err := protocol.Replace(as(t, writeDoctorA, "doctor"), pool, written.ProtocolID, draft); err != nil {
		t.Fatalf("editing under an uppercase identifier: %v", err)
	}
	if !stillThere(t, pool, string(written.ItemIDs[0])) {
		t.Fatal("the item the request named to keep was deleted")
	}
	if slot := slotOf(t, pool, string(written.ItemIDs[0])); slot != "20:00:00" {
		t.Errorf("the kept item reads %s", slot)
	}
}

// Two items naming one row: both writes land on it, the second wins, and the reply says the
// course has two. Refused before a connection is taken.
func TestAnItemNamedTwiceIsRefused(t *testing.T) {
	pool, _ := prescribing(t)

	written, err := protocol.Create(as(t, writeDoctorA, "doctor"), pool, aCourse(writePatientA))
	if err != nil {
		t.Fatalf("prescribing: %v", err)
	}

	twice := aCourse(writePatientA)
	kept := written.ItemIDs[0]
	twice.Items[0].ID = &kept
	second := aCourse(writePatientA).Items[0]
	second.ID = &kept
	twice.Items = append(twice.Items, second)

	if _, err := protocol.Replace(
		as(t, writeDoctorA, "doctor"), pool, written.ProtocolID, twice,
	); !errors.Is(err, protocol.ErrItemNamedTwice) {
		t.Errorf("naming one item twice gave %v", err)
	}
	if rows := countRows(t, pool, string(written.ProtocolID)); rows.items != 1 {
		t.Errorf("the refused edit left %d items", rows.items)
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

	// The course row itself, read back column by column. «The call did not error» was
	// the whole assertion here, and it was already green before the fix — reducing the
	// UPDATE to `SET notes = $5` left the suite green, because nothing read the other
	// three back and no constraint ties to_week to duration_weeks.
	if course := courseRow(t, pool, string(written.ProtocolID)); course.weeks != 16 ||
		course.status != "active" || course.start != "2026-05-04" {
		t.Errorf("the course reads %+v after the titration", course)
	}

	// Cancelling it, which is the other statement the unconditional delete foreclosed.
	draft.Status = protocol.StatusCancelled
	draft.StartDate = civil.NewDate(2026, time.May, 11)
	note := "остановлен по просьбе пациента"
	draft.Notes = &note
	if _, err := protocol.Replace(
		as(t, writeDoctorA, "doctor"), pool, written.ProtocolID, draft,
	); err != nil {
		t.Fatalf("cancelling a course with a logged dose: %v", err)
	}
	if course := courseRow(t, pool, string(written.ProtocolID)); course.status != "cancelled" ||
		course.start != "2026-05-11" || course.notes != note || course.weeks != 16 {
		t.Errorf("the cancelled course reads %+v", course)
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

	// The borrowing draft has to differ from the victim's, or the witness cannot fail:
	// both courses come from aCourse and carry the same slot and the same phases, so
	// «the row still reads 08:00» was green whether the row was written or not — the
	// unscoped write would have produced that very value.
	draft := aCourse(writePatientA)
	draft.Items[0].Times = []civil.Slot{{Hour: 20}}
	draft.Items[0].Phases = []protocol.ProtocolPhase{
		{FromWeek: 1, ToWeek: 12, Dose: protocol.Dose{Value: 1.5, Unit: protocol.MG}},
	}
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
	// And their phases, which are the rows the unscoped delete would have taken: two
	// bands at their own doses, not one at the caller's.
	if bands := phasesOn(t, pool, string(borrowed)); bands != "0.25,0.5" {
		t.Errorf("the other patient's bands now read %s", bands)
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

func aKind(kind protocol.ItemKind, at civil.Slot) protocol.DraftItem {
	return protocol.DraftItem{
		Kind:     kind,
		Cadence:  protocol.CadenceDaily,
		Times:    []civil.Slot{at},
		Loggable: false,
		Phases:   []protocol.ProtocolPhase{{FromWeek: 1, ToWeek: 12, Dose: protocol.Dose{Value: 1, Unit: protocol.MG}}},
	}
}

type course struct {
	start  string
	weeks  int
	status string
	notes  string
}

func courseRow(t *testing.T, pool *pgxpool.Pool, id string) course {
	t.Helper()

	var read course
	ask(t, pool, `
		SELECT start_date::text, duration_weeks, status, coalesce(notes, '')
		FROM app.protocols WHERE id = $1
	`, []any{id}, &read.start, &read.weeks, &read.status, &read.notes)

	return read
}

func kindsOf(t *testing.T, pool *pgxpool.Pool, course string) string {
	t.Helper()

	var kinds string
	ask(t, pool, `
		SELECT string_agg(kind, ',' ORDER BY kind) FROM app.protocol_items WHERE protocol_id = $1
	`, []any{course}, &kinds)

	return kinds
}

func itemsNamingADrug(t *testing.T, pool *pgxpool.Pool, course string) int {
	t.Helper()

	var named int
	ask(t, pool, `
		SELECT count(*) FROM app.protocol_items WHERE protocol_id = $1 AND compound_id IS NOT NULL
	`, []any{course}, &named)

	return named
}

func stillThere(t *testing.T, pool *pgxpool.Pool, item string) bool {
	t.Helper()

	var there bool
	ask(t, pool, `SELECT EXISTS (SELECT FROM app.protocol_items WHERE id = $1)`, []any{item}, &there)

	return there
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

func phasesOn(t *testing.T, pool *pgxpool.Pool, item string) string {
	t.Helper()

	var bands string
	ask(t, pool, `
		SELECT coalesce(string_agg(dose_value::text, ',' ORDER BY from_week), '')
		FROM app.protocol_phases WHERE protocol_item_id = $1
	`, []any{item}, &bands)

	return bands
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

// The plan as the generator wants it, read in one place. dosing needs it to decide which
// occurrence a dose answers, and a bounded context does not read its neighbour's tables —
// so the read lives here, with the schema it belongs to.
func TestTheActivePlanIsReadWholeOrNotAtAll(t *testing.T) {
	pool, _ := prescribing(t)

	draft := aCourse(writePatientA)
	draft.Items = append(draft.Items, protocol.DraftItem{
		Kind:    protocol.KindSupplement,
		Cadence: protocol.CadenceDaily,
		// Descending on purpose: nothing orders this column, so a read taking it as
		// stored would put the morning dose against the evening occurrence.
		Times:    []civil.Slot{{Hour: 20}, {Hour: 8}},
		Loggable: true,
		Phases: []protocol.ProtocolPhase{
			{FromWeek: 1, ToWeek: 6, Dose: protocol.Dose{Value: 250, Unit: protocol.MCG}},
			{FromWeek: 7, ToWeek: 12, Dose: protocol.Dose{Value: 500, Unit: protocol.MCG}},
		},
	})

	written, err := protocol.Create(as(t, writeDoctorA, "doctor"), pool, draft)
	if err != nil {
		t.Fatalf("prescribing: %v", err)
	}

	// A second patient with a course of their own, because with one course in the clinic
	// `WHERE patient_id = $1` is indistinguishable from its absence — and that predicate
	// is the whole tenant boundary of this read on the service seam, where every policy
	// is USING (true). Everything below inherits its tenancy from the row it returns.
	theirs, err := protocol.Create(as(t, writeDoctorB, "doctor"), pool, aCourse(writePatientB))
	if err != nil {
		t.Fatalf("prescribing for the other patient: %v", err)
	}

	plan := readPlan(t, pool, writePatientA)

	if plan.Protocol.ID == theirs.ProtocolID {
		t.Fatal("the read answered the other patient's course")
	}
	if plan.Protocol.ID != written.ProtocolID || plan.Protocol.PatientID != writePatientA {
		t.Errorf("the plan is %s for %s", plan.Protocol.ID, plan.Protocol.PatientID)
	}
	if plan.Protocol.StartDate != civil.NewDate(2026, time.May, 4) || plan.Protocol.Weeks != 12 {
		t.Errorf("the course reads %v for %d weeks", plan.Protocol.StartDate, plan.Protocol.Weeks)
	}
	if plan.Protocol.Status != protocol.StatusActive {
		t.Errorf("the course is %q", plan.Protocol.Status)
	}

	if len(plan.Items) != 2 {
		t.Fatalf("the plan holds %d items, want 2", len(plan.Items))
	}
	// The set, not the order: protocol_items carries no position, so the order the course
	// was written in is not recoverable and the read's is stable-but-arbitrary. Asserting
	// the write's order here would pin a property the schema does not hold.
	read := []protocol.ProtocolItemID{plan.Items[0].ID, plan.Items[1].ID}
	slices.Sort(read)
	wrote := slices.Clone(written.ItemIDs)
	slices.Sort(wrote)
	if !slices.Equal(read, wrote) {
		t.Errorf("the plan holds %v, the write answered %v", read, wrote)
	}
	// The identifiers and not merely the count: two courses of two items each would
	// satisfy «two items» with the other patient's rows.
	for _, mine := range read {
		if slices.Contains(theirs.ItemIDs, mine) {
			t.Errorf("item %s belongs to the other patient's course", mine)
		}
	}

	injection := byKind(t, plan, protocol.KindInjection)
	if injection.Kind != protocol.KindInjection || injection.CompoundID == nil {
		t.Errorf("the injection reads %+v", injection)
	}
	if len(injection.DaysOfWeek) != 1 || injection.DaysOfWeek[0] != time.Sunday {
		t.Errorf("the injection falls on %v, want Sunday", injection.DaysOfWeek)
	}
	if len(injection.Times) != 1 || injection.Times[0] != (civil.Slot{Hour: 8}) {
		t.Errorf("the injection is at %v", injection.Times)
	}
	if !injection.Loggable {
		t.Error("the injection is not loggable")
	}

	supplement := byKind(t, plan, protocol.KindSupplement)
	if supplement.CompoundID != nil {
		t.Errorf("the supplement names a drug: %v", *supplement.CompoundID)
	}
	// In clock order, whatever order the form sent. The fixture below sends them
	// descending, so an implementation reading the column as stored fails.
	if len(supplement.Times) != 2 ||
		supplement.Times[0] != (civil.Slot{Hour: 8}) || supplement.Times[1] != (civil.Slot{Hour: 20}) {
		t.Errorf("the supplement is at %v, want 08:00 then 20:00", supplement.Times)
	}
	if len(supplement.DaysOfWeek) != 0 {
		t.Errorf("a daily item names weekdays: %v", supplement.DaysOfWeek)
	}

	// The whole key set, not only the two entries read below: every assertion here
	// indexes by an item id that came from the scoped item read, so another patient's
	// phases would sit in the map unseen. That is the deputy — «every caller happens to
	// index by a scoped key» — and step 9's aggregates need not.
	if len(plan.Phases) != 2 {
		t.Errorf("the plan carries phases for %d items, want 2", len(plan.Phases))
	}
	for keyed := range plan.Phases {
		if keyed != injection.ID && keyed != supplement.ID {
			t.Errorf("the plan carries phases for %s, which is not this course's", keyed)
		}
	}

	// And in week order — the generator reads them by week, and a map keyed wrongly
	// would give one item another's titration.
	if bands := plan.Phases[supplement.ID]; len(bands) != 2 ||
		bands[0].FromWeek != 1 || bands[1].Dose.Value != 500 || bands[1].Dose.Unit != protocol.MCG {
		t.Errorf("the supplement's phases read %+v", bands)
	}
	if bands := plan.Phases[injection.ID]; len(bands) != 2 || bands[0].Dose.Value != 0.25 {
		t.Errorf("the injection's phases read %+v", bands)
	}
}

// A patient with no course at all, and one whose course was cancelled: both answer «no plan»
// rather than an empty one, because an empty plan generates an empty schedule and reads as a
// patient who is prescribed nothing.
func TestAPatientWithNoRunningCourseHasNoPlan(t *testing.T) {
	pool, _ := prescribing(t)

	if _, found, err := planFor(t, pool, writePatientB); err != nil || found {
		t.Errorf("a patient with no course: found=%v, err=%v", found, err)
	}

	// The argument and not the caller. This is the case that fails when the patient
	// predicate is dropped: on the service seam every policy is USING (true), so with
	// one active course in the clinic an unscoped read answers it to whoever asks —
	// and the round-one repair, which gave the second patient a course, could not
	// catch it, because A's course is created first and the query has no ORDER BY.
	//
	// Step 9's missed-dose sweep is the next caller and runs on that seam.
	if _, err := protocol.Create(as(t, writeDoctorA, "doctor"), pool, aCourse(writePatientA)); err != nil {
		t.Fatalf("prescribing for the first patient: %v", err)
	}
	if _, found, err := planFor(t, pool, writePatientB); err != nil || found {
		t.Errorf("the other patient's course answered for %s: found=%v, err=%v", writePatientB, found, err)
	}

	written, found, err := planFor(t, pool, writePatientA)
	if err != nil || !found {
		t.Fatalf("reading the course just written: found=%v, err=%v", found, err)
	}

	cancelled := aCourse(writePatientA)
	cancelled.Status = protocol.StatusCancelled
	kept := written.Items[0].ID
	cancelled.Items[0].ID = &kept
	if _, err := protocol.Replace(
		as(t, writeDoctorA, "doctor"), pool, written.Protocol.ID, cancelled,
	); err != nil {
		t.Fatalf("cancelling: %v", err)
	}

	if _, found, err := planFor(t, pool, writePatientA); err != nil || found {
		t.Errorf("a cancelled course: found=%v, err=%v", found, err)
	}
}

// And the boundary: one patient's plan is not another's. The read runs on the request seam,
// so RLS answers — which is the whole reason it is a read and not a service call.
func TestThePlanIsReadUnderTheCallersOwnIdentity(t *testing.T) {
	pool, requests := prescribingWithRequests(t)

	if _, err := protocol.Create(as(t, writeDoctorA, "doctor"), pool, aCourse(writePatientA)); err != nil {
		t.Fatalf("prescribing: %v", err)
	}

	for _, caller := range []struct {
		name    string
		subject string
		role    string
		found   bool
	}{
		{"the patient themselves", writePatientA, "patient", true},
		{"their own doctor", writeDoctorA, "doctor", true},
		{"the other patient", writePatientB, "patient", false},
		{"the other patient's doctor", writeDoctorB, "doctor", false},
		{"a doctor of nobody", "5d4f3b7c-0000-4000-8000-00000000dead", "doctor", false},
	} {
		t.Run(caller.name, func(t *testing.T) {
			var found bool
			if err := database.WithCaller(
				t.Context(), requests,
				database.Caller{Subject: caller.subject, Role: caller.role},
				func(ctx context.Context, tx pgx.Tx) error {
					var err error
					_, found, err = protocol.ActivePlanFor(ctx, tx, civil.UserID(writePatientA))

					return err
				},
			); err != nil {
				t.Fatalf("reading: %v", err)
			}
			if found != caller.found {
				t.Errorf("found=%v, want %v", found, caller.found)
			}
		})
	}
}

func byKind(t *testing.T, plan protocol.Plan, kind protocol.ItemKind) protocol.ProtocolItem {
	t.Helper()

	for _, item := range plan.Items {
		if item.Kind == kind {
			return item
		}
	}
	t.Fatalf("the plan holds no %s", kind)

	return protocol.ProtocolItem{}
}

func readPlan(t *testing.T, pool *pgxpool.Pool, patient string) protocol.Plan {
	t.Helper()

	plan, found, err := planFor(t, pool, patient)
	if err != nil {
		t.Fatalf("reading the plan: %v", err)
	}
	if !found {
		t.Fatal("no running course")
	}

	return plan
}

func planFor(t *testing.T, pool *pgxpool.Pool, patient string) (protocol.Plan, bool, error) {
	t.Helper()

	var (
		plan  protocol.Plan
		found bool
	)
	err := database.WithServiceJob(
		t.Context(), pool, writeJob,
		func(ctx context.Context, tx pgx.Tx) error {
			var err error
			plan, found, err = protocol.ActivePlanFor(ctx, tx, civil.UserID(patient))

			return err
		},
	)

	return plan, found, err
}

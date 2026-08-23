//go:build integration

package dosing_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/dosing"
	"github.com/SimonOsipov/cadence-app/api/internal/journal"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// The one instant the two zones disagree about, which is the whole of the timezone test:
// in Yekaterinburg, where the seeded patients live, it is 00:30 on Sunday 10 May — the day
// the weekly injection falls on — and in Moscow it is 22:30 on Saturday the 9th.
//
// Chosen by measurement after the first attempt picked 03:30 UTC, where both zones are
// already Sunday and the test passed for the wrong reason.
var theMoment = time.Date(2026, time.May, 9, 19, 30, 0, 0, time.UTC)

func logging(t *testing.T) (clinic, *pgxpool.Pool) {
	t.Helper()

	c := newClinic(t)

	// The seeded item is a weekly Sunday injection; its course starts 4 May 2026.
	return c, c.request
}

func draftFor(c clinic, patient string, edit func(*dosing.Draft)) dosing.Draft {
	dose := protocol.Dose{Value: 0.25, Unit: protocol.MG}
	draft := dosing.Draft{
		ItemID:          protocol.ProtocolItemID(c.item[patient]),
		Dose:            &dose,
		ClientRequestID: "wizard-" + patient[:8],
	}
	if edit != nil {
		edit(&draft)
	}

	return draft
}

func caller(patient string) database.Caller {
	return database.Caller{Subject: patient, Role: "patient"}
}

// One action, two facts, and the whole point of one call: the dose and the day are written
// together or not at all.
func TestOneActionWritesBothFacts(t *testing.T) {
	c, pool := logging(t)

	logged, err := dosing.Log(t.Context(), pool, caller(patientA), theMoment,
		draftFor(c, patientA, func(d *dosing.Draft) {
			mood := 4
			note := "спокойно"
			d.Mood = &mood
			d.Note = &note
			d.Sides = []dosing.SideEffect{"nausea"}
			d.Site = siteOf("l-abdomen")
		}))
	if err != nil {
		t.Fatalf("logging: %v", err)
	}
	if logged.Outcome != dosing.Written {
		t.Fatalf("the outcome is %q", logged.Outcome)
	}
	if logged.JournalDate != civil.NewDate(2026, time.May, 10) {
		t.Errorf("the day is %v", logged.JournalDate)
	}
	if logged.Dose.Value != 0.25 || logged.Dose.Unit != protocol.MG {
		t.Errorf("the dose reads %+v", logged.Dose)
	}

	event := eventRow(t, c, logged.EventID)
	if event.scheduledDate != "2026-05-10" || event.scheduledTime != "08:00:00" {
		t.Errorf("the event answers %s %s", event.scheduledDate, event.scheduledTime)
	}
	// injected_at is the instant and never the occurrence it answers, which is the guard
	// step 7 handed here when its own copy of it became structural. The two differ in this
	// fixture by construction: the moment is 19:30 UTC on the 9th and the slot is 08:00 on
	// the 10th, so a write storing one for the other cannot pass.
	if !event.injectedAt.Equal(theMoment) {
		t.Errorf("the dose was injected at %v, want %v", event.injectedAt, theMoment)
	}
	// The compound is the item's, and the draft has no field to carry one — the guard is
	// the shape of the type, so this asserts the resolution rather than a refusal.
	if event.compound != compoundID {
		t.Errorf("the dose is attributed to %s", event.compound)
	}
	if event.site != "l-abdomen" || event.mood != 4 {
		t.Errorf("the event reads site %q mood %d", event.site, event.mood)
	}

	day := dayRow(t, c, patientA, "2026-05-10")
	if day.source != "dose" {
		t.Errorf("the day's provenance is %q", day.source)
	}
	if day.mood != 4 || day.note != "спокойно" || day.tags != "{nausea}" {
		t.Errorf("the day reads %+v", day)
	}
}

// The dose recorded is the one the patient entered, not the one the course prescribes. The
// wizard's stepper is editable — the KMP's own note says the prototype silently resets a
// stepped-down dose and that this does not port it — so a patient who took 0,125 мг has a
// row saying 0,125 мг. Writing the prescription over it would make the difference unsayable
// to the doctor's history, to adherence and to the titration view.
func TestTheDoseRecordedIsTheOneThePatientTook(t *testing.T) {
	c, pool := logging(t)

	steppedDown := draftFor(c, patientA, func(d *dosing.Draft) {
		d.Dose = &protocol.Dose{Value: 0.125, Unit: protocol.MG}
	})

	logged, err := dosing.Log(t.Context(), pool, caller(patientA), theMoment, steppedDown)
	if err != nil {
		t.Fatalf("logging: %v", err)
	}
	if logged.Dose.Value != 0.125 {
		t.Errorf("the reply says %v", logged.Dose.Value)
	}

	var value float64
	ask(t, c, `SELECT dose_value FROM app.dose_events WHERE id = $1`, []any{logged.EventID}, &value)
	if value != 0.125 {
		t.Errorf("the record says the patient took %v, and they took 0,125", value)
	}
}

// An injection with the check-in skipped still writes its day: the KMP pins that, and the
// schema exempts the dose path from «a day says something» for it.
func TestAnInjectionWithNoCheckInStillWritesItsDay(t *testing.T) {
	c, pool := logging(t)

	logged, err := dosing.Log(t.Context(), pool, caller(patientA), theMoment,
		draftFor(c, patientA, nil))
	if err != nil {
		t.Fatalf("logging: %v", err)
	}
	if logged.Outcome != dosing.Written {
		t.Fatalf("the outcome is %q", logged.Outcome)
	}

	day := dayRow(t, c, patientA, "2026-05-10")
	if day.source != "dose" || day.mood != 0 || day.tags != "{}" {
		t.Errorf("the day reads %+v, want an empty day born of a dose", day)
	}
}

// The idempotency §03 asks for: the repeat answers what the first answered and writes nothing.
func TestARepeatOfOneRequestIsOneDose(t *testing.T) {
	c, pool := logging(t)
	draft := draftFor(c, patientA, nil)

	first, err := dosing.Log(t.Context(), pool, caller(patientA), theMoment, draft)
	if err != nil {
		t.Fatalf("the first attempt: %v", err)
	}

	// An hour later, from the retry queue, with the same key.
	again, err := dosing.Log(t.Context(), pool, caller(patientA), theMoment.Add(time.Hour), draft)
	if err != nil {
		t.Fatalf("the retry: %v", err)
	}

	if again.EventID != first.EventID || again.Outcome != dosing.Written {
		t.Errorf("the retry answered %+v, want %+v", again, first)
	}
	if again.JournalDate != first.JournalDate || again.Dose != first.Dose {
		t.Errorf("the retry answered a different row: %+v against %+v", again, first)
	}
	// Counted on the day, because the clinic's seed puts one dose on another day and
	// «the patient has one dose» would then be false whatever the retry did.
	if doses := dosesOn(t, c, patientA, "2026-05-10"); doses != 1 {
		t.Errorf("the day carries %d doses, want 1", doses)
	}
}

// The day is the patient's own, computed in their zone. The seeded patients live in
// Yekaterinburg, where the instant is Sunday morning — in Moscow it is still Saturday, and a
// server reading its own clock would put the dose on a day the course does not prescribe.
func TestTheDayIsThePatientsOwn(t *testing.T) {
	c, pool := logging(t)

	moveTo(t, c, patientA, "Europe/Moscow")

	if _, err := dosing.Log(t.Context(), pool, caller(patientA), theMoment,
		draftFor(c, patientA, nil)); err != nil {
		t.Fatalf("logging: %v", err)
	}
	// Saturday in Moscow: the weekly item falls on Sundays, so there is no occurrence.
	logged, _ := dosing.Log(t.Context(), pool, caller(patientA), theMoment,
		draftFor(c, patientA, func(d *dosing.Draft) { d.ClientRequestID = "second-key" }))
	if logged.Outcome != dosing.NotScheduledToday {
		t.Errorf("in Moscow the outcome is %q, want not_scheduled_today", logged.Outcome)
	}

	// And back in Yekaterinburg the same instant is Sunday.
	moveTo(t, c, patientA, "Asia/Yekaterinburg")
	logged, err := dosing.Log(t.Context(), pool, caller(patientA), theMoment,
		draftFor(c, patientA, func(d *dosing.Draft) { d.ClientRequestID = "third-key" }))
	if err != nil {
		t.Fatalf("logging: %v", err)
	}
	if logged.Outcome != dosing.Written || logged.JournalDate != civil.NewDate(2026, time.May, 10) {
		t.Errorf("in Yekaterinburg the outcome is %q on %v", logged.Outcome, logged.JournalDate)
	}
}

// A patient whose profile carries no zone. There is no safe default — the server's would put
// a dose on the wrong date for half a clinic — so it is refused and says which patient.
func TestAPatientWithNoZoneIsRefusedRatherThanGuessed(t *testing.T) {
	c, pool := logging(t)

	moveTo(t, c, patientA, "")

	if _, err := dosing.Log(t.Context(), pool, caller(patientA), theMoment,
		draftFor(c, patientA, nil)); !errors.Is(err, dosing.ErrNoTimezone) {
		t.Errorf("a patient with no zone gave %v, want ErrNoTimezone", err)
	}
}

func siteOf(code string) *dosing.Site {
	site := dosing.Site(code)

	return &site
}

type storedEvent struct {
	scheduledDate string
	scheduledTime string
	injectedAt    time.Time
	compound      string
	site          string
	mood          int
}

func eventRow(t *testing.T, c clinic, id string) storedEvent {
	t.Helper()

	var read storedEvent
	ask(t, c, `
		SELECT scheduled_for_date::text, coalesce(scheduled_for_time::text, ''), injected_at,
		       coalesce(compound_id::text, ''), coalesce(site_code, ''), coalesce(mood, 0)
		FROM app.dose_events WHERE id = $1
	`, []any{id}, &read.scheduledDate, &read.scheduledTime, &read.injectedAt,
		&read.compound, &read.site, &read.mood)

	return read
}

type storedDay struct {
	source string
	mood   int
	energy int
	sleep  int
	note   string
	tags   string
}

func dayRow(t *testing.T, c clinic, patient, day string) storedDay {
	t.Helper()

	var read storedDay
	ask(t, c, `
		SELECT source, coalesce(mood, 0), coalesce(energy, 0), coalesce(sleep, 0),
		       coalesce(note, ''), tags::text
		FROM app.journal_entries WHERE patient_id = $1 AND entry_date = $2::date
	`, []any{patient, day}, &read.source, &read.mood, &read.energy, &read.sleep,
		&read.note, &read.tags)

	return read
}

func dosesOn(t *testing.T, c clinic, patient, day string) int {
	t.Helper()

	var doses int
	ask(t, c, `
		SELECT count(*) FROM app.dose_events
		WHERE patient_id = $1 AND scheduled_for_date = $2::date
	`, []any{patient, day}, &doses)

	return doses
}

// moveTo rewrites the patient's zone. The superuser, because the service path holds UPDATE on
// profiles but the column list is provisioning's and this is a fixture, not a flow.
func moveTo(t *testing.T, c clinic, patient, zone string) {
	t.Helper()

	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx,
				`UPDATE app.profiles SET timezone = nullif($2, '') WHERE user_id = $1`, patient, zone)

			return err
		},
	); err != nil {
		t.Fatalf("moving %s to %q: %v", patient, zone, err)
	}
}

func ask(t *testing.T, c clinic, query string, args []any, into ...any) {
	t.Helper()

	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, query, args...).Scan(into...)
		},
	); err != nil {
		t.Fatalf("reading: %v", err)
	}
}

func daysOn(t *testing.T, c clinic, patient, day string) int {
	t.Helper()

	var days int
	ask(t, c, `
		SELECT count(*) FROM app.journal_entries
		WHERE patient_id = $1 AND entry_date = $2::date
	`, []any{patient, day}, &days)

	return days
}

// The vial is resolved and not chosen, which is the recorded invariant: exactly one open,
// undisposed vial of the item's compound is the answer, and with two the server leaves it
// empty rather than guessing. The KMP's mock takes the fullest of the two and admits in its
// own comment that this is «at most wrong by one vial» — and a wrong vial is a wrong
// remaining count, which is the number the patient reorders on.
func TestTheVialIsResolvedRatherThanChosen(t *testing.T) {
	for _, cabinet := range []struct {
		name  string
		open  int
		named bool
	}{
		{"no open vial at all", 0, false},
		{"one open vial", 1, true},
		{"two open vials of the same compound", 2, false},
	} {
		t.Run(cabinet.name, func(t *testing.T) {
			c, pool := logging(t)
			openVials(t, c, patientA, cabinet.open)

			logged, err := dosing.Log(t.Context(), pool, caller(patientA), theMoment,
				draftFor(c, patientA, nil))
			if err != nil {
				t.Fatalf("logging: %v", err)
			}
			if (logged.VialID != nil) != cabinet.named {
				t.Errorf("the dose names vial %v, want named=%v", logged.VialID, cabinet.named)
			}
			if cabinet.named && *logged.VialID != c.vial[patientA] {
				t.Errorf("the dose names %s, want the patient's own open vial", *logged.VialID)
			}
		})
	}
}

// A disposed vial is still on the shelf as history, and drawing from it would put a dose into
// one the patient threw away.
func TestADisposedVialIsNotDrawnFrom(t *testing.T) {
	c, pool := logging(t)
	openVials(t, c, patientA, 1)
	disposeOf(t, c, c.vial[patientA])

	logged, err := dosing.Log(t.Context(), pool, caller(patientA), theMoment,
		draftFor(c, patientA, nil))
	if err != nil {
		t.Fatalf("logging: %v", err)
	}
	if logged.VialID != nil {
		t.Errorf("the dose was drawn from %s, which was thrown away", *logged.VialID)
	}
}

// And the patient's own choice wins over the resolution when the picker did make one.
func TestAVialTheAppNamedIsTheOneUsed(t *testing.T) {
	c, pool := logging(t)
	second := openVials(t, c, patientA, 2)

	named := second
	logged, err := dosing.Log(t.Context(), pool, caller(patientA), theMoment,
		draftFor(c, patientA, func(d *dosing.Draft) { d.VialID = &named }))
	if err != nil {
		t.Fatalf("logging: %v", err)
	}
	if logged.VialID == nil || *logged.VialID != second {
		t.Errorf("the dose names %v, want the one the app chose", logged.VialID)
	}
}

// openVials opens `count` vials of the seeded compound and answers the last one's id. The
// clinic seeds one sealed vial per patient; a second is added when two are wanted.
func openVials(t *testing.T, c clinic, patient string, count int) string {
	t.Helper()

	if count == 0 {
		return ""
	}

	// As the patient, because opening a vial is the patient's own act: step 3 gave the
	// service path SELECT and INSERT on vials and deliberately no UPDATE.
	var last string
	if err := database.WithCaller(
		t.Context(), c.request, database.Caller{Subject: patient, Role: "patient"},
		func(ctx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `
				UPDATE app.vials SET opened_at = DATE '2026-05-01' WHERE id = $1
			`, c.vial[patient]); err != nil {
				return err
			}
			last = c.vial[patient]
			if count < 2 {
				return nil
			}

			return tx.QueryRow(ctx, `
				INSERT INTO app.vials
				    (patient_id, compound_id, concentration_label, total_doses,
				     opened_at, expires_on)
				VALUES ($1, $2, '2,4 мг/0,75 мл', 4, DATE '2026-05-02', DATE '2026-12-31')
				RETURNING id::text
			`, patient, compoundID).Scan(&last)
		},
	); err != nil {
		t.Fatalf("opening %d vials: %v", count, err)
	}

	return last
}

func disposeOf(t *testing.T, c clinic, vial string) {
	t.Helper()

	if err := database.WithCaller(
		t.Context(), c.request, database.Caller{Subject: patientA, Role: "patient"},
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx,
				`UPDATE app.vials SET disposed_at = DATE '2026-05-05' WHERE id = $1`, vial)

			return err
		},
	); err != nil {
		t.Fatalf("disposing of %s: %v", vial, err)
	}
}

// The ordinary sequence: a check-in in the morning, a dose in the evening. Nothing in this
// step logged a dose onto a day that already had an entry, so the upsert's conflict branch
// was held by nothing — replacing DO UPDATE with DO NOTHING left the package green — and
// with it the three rules journal.Merge holds and the schema cannot express.
//
// Step 4 left this measurement here by name: «если шаг 8 станет апсертить по другому
// арбитру, контроль надо переписывать тогда же — здесь этого не заметит ничто».
func TestADoseMergesIntoTheDayTheCheckInAlreadyWrote(t *testing.T) {
	c, pool := logging(t)

	morning := journal.CheckInDraft{
		EntryDate: civil.NewDate(2026, time.May, 10),
		Mood:      pointTo(2),
		Energy:    pointTo(3),
		Sleep:     pointTo(4),
		Tags:      []journal.Tag{journal.TagNausea},
		Note:      pointTo("утром так себе"),
	}
	writeCheckIn(t, c, patientA, morning)

	evening := 5
	if _, err := dosing.Log(t.Context(), pool, caller(patientA), theMoment,
		draftFor(c, patientA, func(d *dosing.Draft) {
			d.Mood = &evening
			d.Sides = []dosing.SideEffect{journal.TagNausea, journal.TagSite}
		})); err != nil {
		t.Fatalf("logging: %v", err)
	}

	day := dayRow(t, c, patientA, "2026-05-10")

	// Provenance is set once: the day was born of a hand-written check-in, and a dose
	// logged against it later does not make that untrue.
	if day.source != "manual" {
		t.Errorf("the day's provenance is now %q", day.source)
	}
	// A named reading overrides, an unnamed one keeps what was there.
	if day.mood != 5 {
		t.Errorf("the mood is %d, want the evening's", day.mood)
	}
	if day.energy != 3 || day.sleep != 4 {
		t.Errorf("energy and sleep read %d and %d, want the morning's", day.energy, day.sleep)
	}
	// A blank note is a skipped note, not an erased one.
	if day.note != "утром так себе" {
		t.Errorf("the note is %q", day.note)
	}
	// Tags accumulate without duplicates: nausea was reported twice and is one tag.
	if day.tags != "{nausea,site}" {
		t.Errorf("the tags read %s", day.tags)
	}
	if days := daysOn(t, c, patientA, "2026-05-10"); days != 1 {
		t.Errorf("the day has %d rows", days)
	}
}

func pointTo[T any](v T) *T { return &v }

// writeCheckIn puts a hand-written day in, as the patient — which is the seam the diary's
// own write path uses.
func writeCheckIn(t *testing.T, c clinic, patient string, draft journal.CheckInDraft) {
	t.Helper()

	entry, err := journal.Merge(nil, civil.UserID(patient), draft, journal.SourceManual)
	if err != nil {
		t.Fatalf("merging: %v", err)
	}

	tags := make([]string, 0, len(entry.Tags))
	for _, tag := range entry.Tags {
		tags = append(tags, string(tag))
	}

	if err := database.WithCaller(
		t.Context(), c.request, database.Caller{Subject: patient, Role: "patient"},
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.journal_entries
				    (patient_id, entry_date, mood, energy, sleep, tags, note, source)
				VALUES ($1, $2::date, $3, $4, $5, $6::text[], $7, $8)
			`, patient, entry.EntryDate.String(), entry.Mood, entry.Energy, entry.Sleep,
				tags, entry.Note, string(entry.Source))

			return err
		},
	); err != nil {
		t.Fatalf("writing the check-in: %v", err)
	}
}

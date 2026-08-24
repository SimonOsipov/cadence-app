//go:build integration

package main

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// thePersona reads back the identifier of the patient the mobile screens are drawn
// around, so the assertions below ask about her rows and nobody else's.
func thePersona(t *testing.T, on deps) civil.UserID {
	t.Helper()

	var userID string
	if err := database.WithServiceJob(t.Context(), on.writes, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `
				SELECT user_id::text FROM app.profiles WHERE full_name = 'Марина Волкова'
			`).Scan(&userID)
		}); err != nil {
		t.Fatalf("looking for the persona: %v", err)
	}

	return civil.UserID(userID)
}

// planOf reads the course back through the context that owns it, on the service
// path — which is what the seed itself wrote through, and what says the rows are
// shaped the way a read expects rather than merely present.
func planOf(t *testing.T, on deps, patient civil.UserID) protocol.Plan {
	t.Helper()

	var plan protocol.Plan
	if err := database.WithServiceJob(t.Context(), on.writes, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			found, running, err := protocol.ActivePlanFor(ctx, tx, patient)
			if err != nil {
				return err
			}
			if !running {
				t.Fatal("the persona holds no active course")
			}
			plan = found

			return nil
		}); err != nil {
		t.Fatalf("reading the course: %v", err)
	}

	return plan
}

// The whole point of the step, asked of the database rather than of the literal:
// the seeded stand shows every category of occurrence, and the day it is seeded on
// is a day inside the course.
func TestTheSeededStandShowsEveryKindOfOccurrence(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	if err := seed(t.Context(), theClinic(), on); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	plan := planOf(t, on, thePersona(t, on))

	week, ok := protocol.CycleWeek(plan.Protocol, theSeededDay)
	if !ok {
		t.Fatalf("the seeded day is outside the course that was just written")
	}
	// Four, written out. Against demoWeek this expectation moves with the constant
	// it is meant to pin: the prototype's header says «4-я неделя» and the first
	// titration band is what that week shows, so the number is the claim.
	if week != 4 {
		t.Errorf("the seeded day is week %d, want 4", week)
	}

	kinds := map[protocol.ItemKind]int{}
	for _, item := range plan.Items {
		kinds[item.Kind]++
	}
	for _, kind := range []protocol.ItemKind{
		protocol.KindInjection, protocol.KindSupplement, protocol.KindWeighIn,
	} {
		if kinds[kind] == 0 {
			t.Errorf("the seeded course holds no %s", kind)
		}
	}

	// The supplement's drug is on its row, which is what the strip draws it from.
	// It was not, and nothing here saw it: the writer keyed on «injection» and
	// stored NULL, leaving the drug in the directory with no item pointing at it —
	// an em-dash under a beaker where «Глицин + магний» under a moon belongs.
	var supplement protocol.ProtocolItem
	for _, item := range plan.Items {
		if item.Kind == protocol.KindSupplement {
			supplement = item
		}
		if item.Kind == protocol.KindWeighIn && item.CompoundID != nil {
			t.Error("the weigh-in prescribes a drug")
		}
	}
	if supplement.CompoundID == nil {
		t.Fatal("the supplement names no drug, so its row draws blank")
	}

	drugs, err := compoundsOf(t, on, plan)
	if err != nil {
		t.Fatalf("reading the directory: %v", err)
	}
	named, ok := drugs[*supplement.CompoundID]
	if !ok {
		t.Fatalf("the supplement points at a drug the directory does not hold")
	}
	if named.NameRU != "Глицин + магний" || named.Icon != "moon" {
		t.Errorf("the supplement's drug is %q under %q", named.NameRU, named.Icon)
	}
}

func compoundsOf(t *testing.T, on deps, plan protocol.Plan) (map[protocol.CompoundID]protocol.Compound, error) {
	t.Helper()

	byID := map[protocol.CompoundID]protocol.Compound{}
	err := database.WithServiceJob(t.Context(), on.writes, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			found, err := protocol.CompoundsFor(ctx, tx, plan)
			if err != nil {
				return err
			}
			for _, drug := range found {
				byID[drug.ID] = drug
			}

			return nil
		})

	return byID, err
}

// A day's occurrences, generated from what the seed actually wrote. This is the
// half a fixture test cannot answer: that the rows round-trip through the schema
// into a plan the generator can read.
func TestTheSeededCourseGeneratesADaysOccurrences(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	if err := seed(t.Context(), theClinic(), on); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	patient := thePersona(t, on)
	plan := planOf(t, on, patient)

	// A Sunday inside the course: the weekly injection and the weekly weigh-in
	// both fall, and the daily pair falls on every day.
	sunday := courseStart(theSeededDay).AddDays(7 * (demoWeek - 1))
	if sunday.Weekday() != time.Sunday {
		t.Fatalf("%s is not a Sunday", sunday)
	}

	// Written out as triples rather than counted. Counts are the same for three
	// wrong courses this suite used to pass: a demo week of six, titration bands
	// shifted to 1..2/3..4/5..12, and the supplement's 21:30 swapped with the
	// weigh-in's 07:30. What each occurrence is, at what time, at what dose is the
	// axis a cardinality cannot see.
	//
	// The doses are literals and not read back from the course: an expectation
	// derived from the constant under test moves with it.
	assertOccurrences(t, "the Sunday", protocol.OccurrencesFor(plan, nil, sunday, theSeededDay), []triple{
		{protocol.KindInjection, civil.Slot{Hour: 7}, "0.25 мг"},
		{protocol.KindWeighIn, civil.Slot{Hour: 7, Minute: 30}, ""},
		{protocol.KindInjection, civil.Slot{Hour: 8}, "250 мкг"},
		{protocol.KindInjection, civil.Slot{Hour: 20}, "250 мкг"},
		{protocol.KindSupplement, civil.Slot{Hour: 21, Minute: 30}, ""},
	})

	// The weekly pair is absent, which is the assertion: a generator ignoring
	// days_of_week would answer the Sunday's five here.
	assertOccurrences(t, "the Monday", protocol.OccurrencesFor(plan, nil, sunday.AddDays(1), theSeededDay), []triple{
		{protocol.KindInjection, civil.Slot{Hour: 8}, "250 мкг"},
		{protocol.KindInjection, civil.Slot{Hour: 20}, "250 мкг"},
		{protocol.KindSupplement, civil.Slot{Hour: 21, Minute: 30}, ""},
	})
}

// triple is what an occurrence is, when, and at what dose — the empty string
// meaning none, which is what a supplement and a weigh-in carry.
type triple struct {
	kind protocol.ItemKind
	at   civil.Slot
	dose string
}

func assertOccurrences(t *testing.T, what string, got []protocol.Occurrence, want []triple) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s holds %d occurrences, want %d: %+v", what, len(got), len(want), got)
	}

	// Sorted by slot, because the generator's order is its own business and this is
	// a statement about the day. Kind breaks the tie so that two items sharing a
	// slot — §03 allows it — are ordered by what they are rather than by the order
	// the generator happened to emit them in.
	slices.SortStableFunc(got, func(a, b protocol.Occurrence) int {
		if by := a.Time.Hour - b.Time.Hour; by != 0 {
			return by
		}
		if by := a.Time.Minute - b.Time.Minute; by != 0 {
			return by
		}

		return strings.Compare(string(a.Kind), string(b.Kind))
	})

	for i, occurrence := range got {
		dose := ""
		if occurrence.Dose != nil {
			dose = strconv.FormatFloat(occurrence.Dose.Value, 'g', -1, 64) + " " + string(occurrence.Dose.Unit)
		}
		if occurrence.Kind != want[i].kind || occurrence.Time != want[i].at || dose != want[i].dose {
			t.Errorf("%s, occurrence %d: %s at %s dosed %q, want %s at %s dosed %q",
				what, i+1, occurrence.Kind, occurrence.Time, dose,
				want[i].kind, want[i].at, want[i].dose)
		}
	}
}

// The seed is re-run against a stand somebody is using, and a second run must not
// give the persona a second course: every read answers the first it finds.
func TestRunningTheSeedTwicePrescribesOneCourse(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	for range 2 {
		if err := seed(t.Context(), theClinic(), on); err != nil {
			t.Fatalf("seeding: %v", err)
		}
	}

	if got := countOf(t, on.writes, `SELECT count(*) FROM app.protocols`); got != 1 {
		t.Errorf("the stand holds %d courses, want one", got)
	}
	if got := countOf(t, on.writes, `SELECT count(*) FROM app.protocol_items`); got != 4 {
		t.Errorf("the stand holds %d prescribed items, want four", got)
	}
	// Three semaglutide bands and one for BPC-157. The supplement and the weigh-in
	// carry none, so a run that dosed them would show eight.
	if got := countOf(t, on.writes, `SELECT count(*) FROM app.protocol_phases`); got != 4 {
		t.Errorf("the stand holds %d dose bands, want four", got)
	}
}

// Two patients, each holding their own course, because with one the predicate in
// holdsACourse is a no-op: the service seam's policies are USING (true), so
// `WHERE patient_id = $1` is the only thing making that read a question about one
// person. Measured — without it the second patient's prescription is silently
// suppressed by the first patient's row, and the seed prints one course.
func TestEachPrescribedPatientHoldsTheirOwnCourse(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	both := theClinic()
	second := -1
	for i := range both.patients {
		if both.patients[i].prescribed {
			continue
		}
		both.patients[i].prescribed = true
		second = i

		break
	}
	if second < 0 {
		t.Fatal("every seeded patient is already prescribed; this test measures nothing")
	}

	// Twice, so the second run has to leave both alone rather than only the first.
	for range 2 {
		if err := seed(t.Context(), both, on); err != nil {
			t.Fatalf("seeding: %v", err)
		}
	}

	for _, who := range []string{"Марина Волкова", both.patients[second].fullName} {
		held := countOf(t, on.writes, `
			SELECT count(*) FROM app.protocols p
			JOIN app.profiles who ON who.user_id = p.patient_id
			WHERE who.full_name = $1
		`, who)
		if held != 1 {
			t.Errorf("%s holds %d courses, want one", who, held)
		}
	}
}

// The roster is a name and an age; a course invented for each of them is treatment
// nobody prescribed. Only the persona holds one.
func TestOnlyThePersonaIsPrescribedACourse(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	if err := seed(t.Context(), theClinic(), on); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	held := countOf(t, on.writes, `
		SELECT count(*) FROM app.protocols p
		JOIN app.profiles who ON who.user_id = p.patient_id
		WHERE who.full_name <> 'Марина Волкова'
	`)
	if held != 0 {
		t.Errorf("%d patients besides the persona were prescribed a course", held)
	}
}

// The directory is filled by prescribing rather than by a list of its own, and a
// second run adds nothing — it returns at holdsACourse before Create is reached.
//
// That is all this measures. «Повтор по регистру возвращает существующий
// compoundId» is asked by TestANameAlreadyInTheDirectoryResolvesToTheRowItAlreadyHas
// in internal/protocol; no name in this seed is ever spelled in two cases.
func TestTheDirectoryHoldsEachDrugOnce(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	for range 2 {
		if err := seed(t.Context(), theClinic(), on); err != nil {
			t.Fatalf("seeding: %v", err)
		}
	}

	if got := countOf(t, on.writes, `SELECT count(*) FROM app.compounds`); got != 3 {
		t.Errorf("the directory holds %d drugs, want three", got)
	}
}

// The one thing standing between a re-used address and the persona's course
// landing on somebody else's record, and until round two nothing measured it.
//
// requireCaresFor cannot discriminate here: every seeded patient names the same
// primary specialist, so a course written for the wrong one of them passes. The
// address is the seed's key, and on a re-run the identifier behind it comes from
// the provider — so who that identifier turns out to be is what has to be asked.
func TestASeedRefusesToPrescribeForSomebodyElse(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	if err := seed(t.Context(), theClinic(), on); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	persona := thePersona(t, on)

	if err := isWhoWeMeant(t.Context(), on, string(persona), "Марина Волкова"); err != nil {
		t.Errorf("the persona is refused as herself: %v", err)
	}

	err := isWhoWeMeant(t.Context(), on, string(persona), "Кто-то Другой")
	if !errors.Is(err, errNotWhoWeMeant) {
		t.Errorf("prescribing for the wrong person answered %v, want %v", err, errNotWhoWeMeant)
	}
	// The refusal names both, because an operator meeting it has to be able to tell
	// a re-used address from a patient who renamed themselves.
	if err != nil && !strings.Contains(err.Error(), "Марина Волкова") {
		t.Errorf("the refusal does not say who the account is: %v", err)
	}
}

// An account nobody holds is a refusal too, not a course written blind. The error
// is named rather than merely required: any broken query answers non-nil.
func TestASeedRefusesToPrescribeForAnAccountWithNoProfile(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	err := isWhoWeMeant(t.Context(), on, "5d4f3b7c-0000-4000-8000-00000000ffff", "Марина Волкова")
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("an account with no profile answered %v, want no rows", err)
	}
}

// And the guard measured where it is called rather than as a function, because a
// call site is what an extracted one leaves untested — the same survivor this step
// already met once over the seeded day's timezone.
//
// The scenario is the recorded limitation as well: full_name is patient-writable,
// so a patient who renames themselves stops the next run. It stops with a refusal
// naming both, which is the point — the alternative was writing the persona's
// twelve-week course onto whoever now holds that address.
func TestARenamedPersonaStopsTheNextRunRatherThanBeingPrescribedFor(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	// The first run creates her and prescribes nothing, so that the second run's
	// refusal is measurable as an absence. With a course already on the row, the
	// count below is one whatever happens — including when the guard runs after the
	// write it protects, which is the ordering this is here to pin.
	unprescribed := theClinic()
	unprescribed.patients[0].prescribed = false
	if err := seed(t.Context(), unprescribed, on); err != nil {
		t.Fatalf("the first run: %v", err)
	}

	if err := database.WithServiceJob(t.Context(), on.writes, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx,
				`UPDATE app.profiles SET full_name = $1 WHERE full_name = $2`,
				"Кто-то Другой", "Марина Волкова")
			return err
		}); err != nil {
		t.Fatalf("renaming the persona: %v", err)
	}

	err := seed(t.Context(), theClinic(), on)
	if !errors.Is(err, errNotWhoWeMeant) {
		t.Fatalf("the second run answered %v, want %v", err, errNotWhoWeMeant)
	}
	if !strings.Contains(err.Error(), "Кто-то Другой") {
		t.Errorf("the refusal does not say who holds the account: %v", err)
	}

	// Refused before the write and not after it: a course on the stranger's record
	// plus an error afterwards is the harm, not a lesser version of it.
	held := countOf(t, on.writes, `
		SELECT count(*) FROM app.protocols p
		JOIN app.profiles who ON who.user_id = p.patient_id
		WHERE who.full_name = 'Кто-то Другой'
	`)
	if held != 0 {
		t.Errorf("the stranger's record holds %d courses", held)
	}
}

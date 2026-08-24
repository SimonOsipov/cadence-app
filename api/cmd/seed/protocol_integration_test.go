//go:build integration

package main

import (
	"context"
	"sort"
	"strconv"
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
	// a statement about the day rather than about the list.
	sort.Slice(got, func(a, b int) bool {
		if got[a].Time.Hour != got[b].Time.Hour {
			return got[a].Time.Hour < got[b].Time.Hour
		}
		return got[a].Time.Minute < got[b].Time.Minute
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

// The drug directory is filled by prescribing rather than by a list of its own, and
// three drugs prescribed twice are still three rows: «повтор по регистру возвращает
// существующий compoundId» is an acceptance criterion of this feature, and a second
// seed run is the cheapest way to ask it.
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

//go:build integration

package main

import (
	"context"
	"testing"

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
	if week != demoWeek {
		t.Errorf("the seeded day is week %d, want %d", week, demoWeek)
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
	if sunday.Weekday() != 0 {
		t.Fatalf("%s is not a Sunday", sunday)
	}

	onSunday := protocol.OccurrencesFor(plan, nil, sunday, theSeededDay)
	// Semaglutide at 07:00, the weigh-in at 07:30, BPC-157 at 08:00 and 20:00, and
	// the supplement at 21:30: five, and the count is what says the weekly items
	// are not being generated daily.
	if len(onSunday) != 5 {
		t.Errorf("the Sunday holds %d occurrences, want 5: %+v", len(onSunday), onSunday)
	}

	monday := sunday.AddDays(1)
	onMonday := protocol.OccurrencesFor(plan, nil, monday, theSeededDay)
	// The two BPC-157 slots and the supplement. The weekly pair is absent, which
	// is the assertion: a generator ignoring days_of_week would answer five here.
	if len(onMonday) != 3 {
		t.Errorf("the Monday holds %d occurrences, want 3: %+v", len(onMonday), onMonday)
	}

	// The titration reaches the generated day: week 4 is the first band.
	dosed := 0
	for _, occurrence := range onSunday {
		if occurrence.Dose == nil {
			continue
		}
		dosed++
		if occurrence.Dose.Value == 0.25 && occurrence.Dose.Unit != protocol.MG {
			t.Errorf("the semaglutide is dosed in %q", occurrence.Dose.Unit)
		}
	}
	// Three of the five carry a dose: the supplement and the weigh-in carry none,
	// which is what makes the strip's dose column meaningfully optional.
	if dosed != 3 {
		t.Errorf("%d of the Sunday's occurrences carry a dose, want 3", dosed)
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

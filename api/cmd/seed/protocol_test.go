package main

import (
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

const seededPatientID = "9b2f3b7c-0000-4000-8000-0000000000a1"

// A year of days, because the incident courseStart records is one a single fixture
// date cannot catch.
func TestTheSeededPatientIsInsideTheirCourseOnAnyDay(t *testing.T) {
	day := civil.NewDate(2026, time.January, 1)

	for range 366 {
		course := theCourse(seededPatientID, day)
		plan := protocol.Plan{Protocol: protocol.Protocol{
			StartDate: course.StartDate,
			Weeks:     course.Weeks,
			Status:    course.Status,
		}}

		week, ok := protocol.CycleWeek(plan.Protocol, day)
		if !ok {
			t.Fatalf("seeded on %s, the patient is outside their own course", day)
		}
		// Four, written out rather than compared against demoWeek: an expectation
		// derived from the constant under test moves with it, and the week is the
		// claim — it is what the prototype's header and the first band both say.
		if week != 4 {
			t.Errorf("seeded on %s, the patient is in week %d, want 4", day, week)
		}

		day = day.AddDays(1)
	}
}

// The weekly items are prescribed on Sundays, so a course opening mid-week would
// put its first injection six days after the patient was told it started.
func TestTheCourseOpensOnASunday(t *testing.T) {
	day := civil.NewDate(2026, time.January, 1)

	for range 366 {
		if start := courseStart(day); start.Weekday() != time.Sunday {
			t.Fatalf("seeded on %s, the course opens on %s, a %s", day, start, start.Weekday())
		}

		day = day.AddDays(1)
	}
}

// Every category of occurrence, which is the whole point of the step: a stand
// showing only injections cannot be held up against a screen that draws three
// kinds of row.
func TestTheCourseCoversEveryKindOfOccurrence(t *testing.T) {
	course := theCourse(seededPatientID, civil.NewDate(2026, time.May, 31))

	kinds := map[protocol.ItemKind]int{}
	cadences := map[protocol.Cadence]int{}
	slots := 0
	for _, item := range course.Items {
		kinds[item.Kind]++
		cadences[item.Cadence]++
		slots += len(item.Times)
	}

	for _, kind := range []protocol.ItemKind{
		protocol.KindInjection, protocol.KindSupplement, protocol.KindWeighIn,
	} {
		if kinds[kind] == 0 {
			t.Errorf("nothing in the course is a %s", kind)
		}
	}
	for _, cadence := range []protocol.Cadence{protocol.CadenceWeekly, protocol.CadenceDaily} {
		if cadences[cadence] == 0 {
			t.Errorf("nothing in the course is %s", cadence)
		}
	}
	// Four items, five slots: BPC-157 carries two.
	if slots != len(course.Items)+1 {
		t.Errorf("%d items carry %d slots", len(course.Items), slots)
	}
}

// The course the seed writes has to be one the API would accept, and Check is what
// says so — a seed that writes past its own validator is a stand the product could
// not have produced.
func TestTheSeededCourseIsOneTheAPIWouldAccept(t *testing.T) {
	if err := theCourse(seededPatientID, civil.NewDate(2026, time.May, 31)).Check(); err != nil {
		t.Errorf("the seeded course is refused: %v", err)
	}
}

// The titration is the reason the course is twelve weeks: three bands of four, and
// the strip shows a different dose in week 1, week 5 and week 9.
func TestTheSemaglutideIsTitratedInThreeBands(t *testing.T) {
	course := theCourse(seededPatientID, civil.NewDate(2026, time.May, 31))

	sema := course.Items[0]
	if sema.Compound.New == nil || sema.Compound.New.NameRU != "Семаглутид" {
		t.Fatalf("the first item is %+v", sema.Compound)
	}

	// The boundaries and not only the doses. Widths summing to twelve is true of
	// 1..2 / 3..4 / 5..12 as well, and under that the seeded day — week four —
	// would show 0,5 мг where the prototype draws 0,25.
	want := []protocol.ProtocolPhase{
		{FromWeek: 1, ToWeek: 4, Dose: protocol.Dose{Value: 0.25, Unit: protocol.MG}},
		{FromWeek: 5, ToWeek: 8, Dose: protocol.Dose{Value: 0.5, Unit: protocol.MG}},
		{FromWeek: 9, ToWeek: 12, Dose: protocol.Dose{Value: 1.0, Unit: protocol.MG}},
	}
	if len(sema.Phases) != len(want) {
		t.Fatalf("the titration has %d bands", len(sema.Phases))
	}
	for i, phase := range sema.Phases {
		if phase != want[i] {
			t.Errorf("band %d is %+v, want %+v", i+1, phase, want[i])
		}
	}
}

// The two rows that carry no dose, and why each one does not.
func TestWhatCarriesNoDoseCarriesNone(t *testing.T) {
	course := theCourse(seededPatientID, civil.NewDate(2026, time.May, 31))

	for _, item := range course.Items {
		if item.Kind == protocol.KindInjection {
			continue
		}
		if len(item.Phases) != 0 {
			t.Errorf("the %s is dosed: %+v", item.Kind, item.Phases)
		}
		// Neither is loggable: the supplement is tracked without asking, and
		// nothing records a weight until the measurements context exists.
		if item.Loggable {
			t.Errorf("the %s offers to be recorded, and nothing records one", item.Kind)
		}
	}
}

// The supplement names a drug and the weigh-in does not. The strip draws the
// supplement's glyph and name from that compound — «Глицин + магний» under a moon —
// while a weigh-in is not a prescription of anything.
func TestTheSupplementNamesADrugAndTheWeighInDoesNot(t *testing.T) {
	course := theCourse(seededPatientID, civil.NewDate(2026, time.May, 31))

	for _, item := range course.Items {
		named := item.Compound.ID != nil || item.Compound.New != nil

		switch item.Kind {
		case protocol.KindWeighIn:
			if named {
				t.Error("the weigh-in prescribes a drug")
			}
		case protocol.KindSupplement:
			if !named {
				t.Error("the supplement names no drug, so the strip draws it blank")
			}
			if item.Compound.New == nil || item.Compound.New.Icon != "moon" {
				t.Errorf("the supplement's glyph is %+v", item.Compound.New)
			}
		case protocol.KindInjection:
			if !named {
				t.Error("an injection prescribes no drug")
			}
		}
	}
}

// The seeded people live in Europe/Moscow and the reads resolve a patient's day in
// their own zone, so the course is counted in that zone rather than the host's.
//
// The failure this closes is one day wide and lands exactly on the boundary the
// course is aligned to: a run at 23:30 UTC on a Saturday counts back from that
// Saturday, while in Moscow it is already Sunday — twenty-eight days after the
// start rather than twenty-one, so the stand opens on week five at 0,5 мг where
// the prototype draws week four at 0,25.
func TestTheSeededDayIsThePatientsAndNotTheHosts(t *testing.T) {
	// 23:30 UTC on Saturday 30 May 2026 is 02:30 on Sunday 31 May in Moscow.
	saturdayNight := time.Date(2026, time.May, 30, 23, 30, 0, 0, time.UTC)

	// Through the function the command actually calls, not through todayIn with a
	// zone written out here: the zone named at the call site is the thing that can
	// be wrong, and an argument the test supplies pins nothing about it.
	today := seededToday(saturdayNight)
	if want := civil.NewDate(2026, time.May, 31); today != want {
		t.Fatalf("the seeded day is %s, want %s", today, want)
	}

	course := theCourse(seededPatientID, today)
	week, ok := protocol.CycleWeek(protocol.Protocol{
		StartDate: course.StartDate, Weeks: course.Weeks, Status: course.Status,
	}, today)
	if !ok || week != 4 {
		t.Errorf("the patient is in week %d (inside: %v), want 4", week, ok)
	}
}

// A zone the host cannot load must not stop a development command; the day falls
// back to the instant's own rather than to the zero date, which no course covers.
func TestAZoneTheHostCannotLoadStillAnswersADay(t *testing.T) {
	at := time.Date(2026, time.May, 27, 12, 0, 0, 0, time.UTC)

	if got := todayIn("Mars/Olympus_Mons", at); got != civil.NewDate(2026, time.May, 27) {
		t.Errorf("an unknown zone answered %s", got)
	}
}

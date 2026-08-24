package main

import (
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

const seededPatientID = "9b2f3b7c-0000-4000-8000-0000000000a1"

// The failure this exists to prevent has already happened once in this repository:
// MockSeed's course was written down as 10 May 2026, ran out on 1 August, and every
// screen went blank on the second with the whole suite green. So the start is
// counted back from the day the seed runs, and this asks a year of days whether the
// patient is inside their course on each of them.
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
		if week != demoWeek {
			t.Errorf("seeded on %s, the patient is in week %d, want %d", day, week, demoWeek)
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
	// Four items, five slots: BPC-157 carries two, which is what makes an
	// occurrence keyed by (item, date, time) rather than by (item, date).
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

	want := []protocol.Dose{
		{Value: 0.25, Unit: protocol.MG},
		{Value: 0.5, Unit: protocol.MG},
		{Value: 1.0, Unit: protocol.MG},
	}
	if len(sema.Phases) != len(want) {
		t.Fatalf("the titration has %d bands", len(sema.Phases))
	}
	covered := 0
	for i, phase := range sema.Phases {
		if phase.Dose != want[i] {
			t.Errorf("band %d doses %+v, want %+v", i+1, phase.Dose, want[i])
		}
		covered += phase.ToWeek - phase.FromWeek + 1
	}
	if covered != course.Weeks {
		t.Errorf("the bands cover %d weeks of a %d-week course", covered, course.Weeks)
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

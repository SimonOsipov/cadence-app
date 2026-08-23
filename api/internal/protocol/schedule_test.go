package protocol

import (
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

// A weekly Sunday injection and a twice-daily supplement, on a course that starts Monday
// 4 May 2026 and runs twelve weeks. The two kinds together are what tell the injection dot
// from «something is prescribed».
func aSchedulePlan() Plan {
	compound := CompoundID("3c1f3b7c-0000-4000-8000-00000000000d")
	dose := Dose{Value: 0.25, Unit: MG}

	return Plan{
		Protocol: Protocol{
			PatientID: "3c1f3b7c-0000-4000-8000-0000000000a1",
			StartDate: civil.NewDate(2026, time.May, 4),
			Weeks:     12,
			Status:    StatusActive,
		},
		Items: []ProtocolItem{
			{
				ID: "item-injection", Kind: KindInjection, CompoundID: &compound,
				Cadence: CadenceWeekly, DaysOfWeek: []time.Weekday{time.Sunday},
				Times: []civil.Slot{{Hour: 8}}, Loggable: true,
			},
			{
				ID: "item-supplement", Kind: KindSupplement,
				Cadence: CadenceDaily,
				Times:   []civil.Slot{{Hour: 8}, {Hour: 20}}, Loggable: true,
			},
		},
		Phases: map[ProtocolItemID][]ProtocolPhase{
			"item-injection":  {{FromWeek: 1, ToWeek: 12, Dose: dose}},
			"item-supplement": {{FromWeek: 1, ToWeek: 12, Dose: dose}},
		},
	}
}

func week(from, through int) civil.Range {
	window, ok := civil.NewRange(civil.NewDate(2026, time.May, from), civil.NewDate(2026, time.May, through))
	if !ok {
		panic("the fixture's window runs backwards")
	}

	return window
}

// Every day of the window is a cell, including the ones prescribing nothing: the calendar
// draws a month, and a client filling the gaps would have to know arithmetic that is the
// server's, because the window came from the patient's own zone.
func TestEveryDayOfTheWindowIsACell(t *testing.T) {
	days := ScheduleFor(aSchedulePlan(), nil, week(4, 10), civil.NewDate(2026, time.May, 4))

	if len(days) != 7 {
		t.Fatalf("the window is %d cells, want seven", len(days))
	}
	for i, day := range days {
		if want := civil.NewDate(2026, time.May, 4+i); day.Date != want {
			t.Errorf("cell %d is %v, want %v", i, day.Date, want)
		}
	}
}

// The dot is about injections. A day carrying only the supplement has occurrences and no dot,
// which is the pair a single «anything prescribed» flag could not say.
func TestTheInjectionDotIsAboutInjectionsAndNotAboutAnything(t *testing.T) {
	days := ScheduleFor(aSchedulePlan(), nil, week(4, 10), civil.NewDate(2026, time.May, 4))

	// 10 May is the Sunday; the rest carry the daily supplement alone.
	for _, day := range days {
		wanted := day.Date == civil.NewDate(2026, time.May, 10)
		if day.HasInjection != wanted {
			t.Errorf("%v: injection dot %v, want %v", day.Date, day.HasInjection, wanted)
		}
	}
}

// Pending and done are two questions, and a day prescribing nothing answers no to both.
func TestADayWithNothingPrescribedIsNeitherPendingNorDone(t *testing.T) {
	// The week before the course starts.
	before, ok := civil.NewRange(civil.NewDate(2026, time.April, 27), civil.NewDate(2026, time.May, 3))
	if !ok {
		t.Fatal("the window runs backwards")
	}

	for _, day := range ScheduleFor(aSchedulePlan(), nil, before, civil.NewDate(2026, time.May, 4)) {
		if day.AnyPending || day.AllDone || day.HasInjection || day.CycleWeek != nil {
			t.Errorf("%v reads %+v, want an empty cell", day.Date, day)
		}
	}
}

// A day every one of whose occurrences is logged is done and not pending; one with some of
// them logged is both not-done and pending. The twice-daily item is what tells them apart —
// with a single occurrence the two rules give the same answer.
func TestADayIsDoneOnlyWhenEveryOccurrenceIs(t *testing.T) {
	day := civil.NewDate(2026, time.May, 6)
	morning := LoggedSlot{ItemID: "item-supplement", Date: day, Time: &civil.Slot{Hour: 8}}
	evening := LoggedSlot{ItemID: "item-supplement", Date: day, Time: &civil.Slot{Hour: 20}}

	for _, state := range []struct {
		name    string
		logged  []LoggedSlot
		pending bool
		done    bool
	}{
		{"nothing logged", nil, true, false},
		{"the morning only", []LoggedSlot{morning}, true, false},
		{"both", []LoggedSlot{morning, evening}, false, true},
	} {
		t.Run(state.name, func(t *testing.T) {
			// The window is that one day, and today is that day: a day in the past
			// with nothing logged is missed rather than pending, which is a different
			// question and has its own case below.
			window, _ := civil.NewRange(day, day)
			cells := ScheduleFor(aSchedulePlan(), state.logged, window, day)

			if cells[0].AnyPending != state.pending || cells[0].AllDone != state.done {
				t.Errorf("pending=%v done=%v, want %v and %v",
					cells[0].AnyPending, cells[0].AllDone, state.pending, state.done)
			}
		})
	}
}

// A day in the past with nothing logged is missed: not pending, and not done either. Both
// flags false, which is the same shape as a day prescribing nothing and a different fact —
// the client tells them apart by the occurrences, not by these.
func TestAMissedDayIsNeitherPendingNorDone(t *testing.T) {
	past := civil.NewDate(2026, time.May, 6)
	window, _ := civil.NewRange(past, past)

	cells := ScheduleFor(aSchedulePlan(), nil, window, civil.NewDate(2026, time.May, 20))
	if cells[0].AnyPending || cells[0].AllDone {
		t.Errorf("a missed day reads pending=%v done=%v", cells[0].AnyPending, cells[0].AllDone)
	}
}

// The week number is the course's own, and absent outside it.
func TestTheWeekNumberIsTheCoursesAndAbsentOutsideIt(t *testing.T) {
	// The course starts 4 May, so 4–10 May is week one and 11 May opens week two.
	window, _ := civil.NewRange(civil.NewDate(2026, time.May, 3), civil.NewDate(2026, time.May, 11))
	cells := ScheduleFor(aSchedulePlan(), nil, window, civil.NewDate(2026, time.May, 4))

	if cells[0].CycleWeek != nil {
		t.Errorf("the day before the course is week %d", *cells[0].CycleWeek)
	}
	if cells[1].CycleWeek == nil || *cells[1].CycleWeek != 1 {
		t.Errorf("the first day is %v, want week one", cells[1].CycleWeek)
	}
	if cells[8].CycleWeek == nil || *cells[8].CycleWeek != 2 {
		t.Errorf("the eighth day is %v, want week two", cells[8].CycleWeek)
	}
}

// A cancelled course draws nothing at all — no weeks, no dots, no occurrences. «Неделя 4»
// over an empty calendar is worse than saying nothing.
func TestACancelledCourseDrawsAnEmptyCalendar(t *testing.T) {
	plan := aSchedulePlan()
	plan.Protocol.Status = StatusCancelled

	for _, day := range ScheduleFor(plan, nil, week(4, 10), civil.NewDate(2026, time.May, 4)) {
		if day.HasInjection || day.AnyPending || day.AllDone {
			t.Errorf("%v reads %+v", day.Date, day)
		}
	}
}

// A future day of a running course: prescribed, dotted, and neither pending nor done.
//
// Pending is today's word. The KMP computes it from PENDING alone, and statusOf gives that to
// today only — so counting a scheduled occurrence as pending makes every prescribed day
// between tomorrow and the end of a twelve-week course report it. Nothing measured this until
// review found it: the suite had today, the past and days outside the course, and the one
// case where the two implementations disagree was the one missing.
func TestAFutureDayIsNeitherPendingNorDone(t *testing.T) {
	future := civil.NewDate(2026, time.May, 17)
	window, _ := civil.NewRange(future, future)

	cells := ScheduleFor(aSchedulePlan(), nil, window, civil.NewDate(2026, time.May, 10))

	if len(cells) != 1 {
		t.Fatalf("the window is %d cells", len(cells))
	}
	if cells[0].AnyPending {
		t.Error("a day a week away reports something pending")
	}
	if cells[0].AllDone {
		t.Error("a day a week away reports everything done")
	}
	// And it is still a prescribed day: the dot is what the calendar draws, and it does
	// not come from either flag.
	if !cells[0].HasInjection {
		t.Error("the Sunday a week away carries no injection")
	}
}

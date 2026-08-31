package measurements

import (
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// Both edges are in the window, so today is one of the days it counts: «за 7 дней» ending
// today is the six before it and today.
const (
	weekDays        = 7
	fourWeeksDays   = 28
	threeMonthsDays = 84
)

// RangeOn is the days a window covers, or false when it covers none.
//
// `today` is a parameter and never the clock: it is the patient's own day, resolved in the
// patient's zone by the caller, and a window counted from the server's day is a day off at
// both edges for half a clinic. The course is the patient's last one, already chosen and read
// by protocol.LastPlanFor — nil when they have none.
//
// Three of the four are lengths and always answer. The cycle is geometry and answers false in
// two cases: no course at all, and a course that starts after today.
func RangeOn(window Window, course *protocol.Protocol, today civil.Date) (civil.Range, bool) {
	switch window {
	case WindowWeek:
		return endingOn(today, weekDays)
	case WindowFourWeeks:
		return endingOn(today, fourWeeksDays)
	case WindowThreeMonths:
		return endingOn(today, threeMonthsDays)
	case WindowCycle:
		return cycleOn(course, today)
	}

	// No default inside the switch, and nothing but empty out here: a window off the set
	// must not inherit its neighbour's span. ParseWindow is what keeps one from arriving.
	return civil.Range{}, false
}

func endingOn(today civil.Date, days int) (civil.Range, bool) {
	return civil.NewRange(today.AddDays(-(days - 1)), today)
}

// The course's own start through today or its last prescribed day, whichever is earlier —
// and status is not asked. A course the doctor closed yesterday still happened, and a trend
// that filtered by status would blank the screen of a patient between courses. **The calendar
// decides the other way**: `protocol.RowFor` draws nothing for a cancelled course. Two
// surfaces, two rules, deliberately — one answers «what is prescribed now», this one «what
// was measured».
//
// A course that has not started yet is refused by NewRange rather than by a check of its own:
// its start is after its through, and «a window that runs backwards is not a window» is that
// constructor's whole job. A second check here would be a second definition of it.
func cycleOn(course *protocol.Protocol, today civil.Date) (civil.Range, bool) {
	if course == nil {
		return civil.Range{}, false
	}

	// LastPrescribedDay, not the same arithmetic written again: the dose bands are
	// clipped by it too, so an axis cannot outlive the strip beneath it.
	return civil.NewRange(course.StartDate, civil.MinDate(today, course.LastPrescribedDay()))
}

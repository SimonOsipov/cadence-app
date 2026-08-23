package protocol

import "github.com/SimonOsipov/cadence-app/api/internal/platform/civil"

// ScheduleDay is one cell of the calendar: enough to draw a day's dots without opening it.
//
// The three flags are computed, never stored — «nothing derived is stored» is this project's
// rule, and a day's state is a question about generated occurrences and logged doses that has
// a different answer tomorrow.
type ScheduleDay struct {
	Date civil.Date

	// Absent outside the course: a week number over an empty calendar says less than
	// nothing, and a cancelled course has no weeks at all.
	CycleWeek *int

	// Whether any of the day's occurrences is an injection — the calendar's dot is about
	// injections, and a supplement does not earn one.
	HasInjection bool

	// Anything still to do today, and everything already done. Both, and not one: a day
	// with nothing prescribed is neither pending nor done, and a single flag would have
	// to lie about one of them.
	AnyPending bool
	AllDone    bool
}

// ScheduleFor is every day of a window, dots included.
//
// Days with nothing prescribed are in the list rather than left out: the calendar draws a
// month, and a client filling gaps itself would have to know the window's arithmetic — which
// is the server's, because the window came from the patient's own zone.
func ScheduleFor(plan Plan, logged []LoggedSlot, window civil.Range, today civil.Date) []ScheduleDay {
	days := make([]ScheduleDay, 0, window.Days())

	window.Each(func(d civil.Date) {
		day := ScheduleDay{Date: d}
		if week, inCourse := CycleWeek(plan.Protocol, d); inCourse {
			day.CycleWeek = &week
		}

		occurrences := OccurrencesFor(plan, logged, d, today)
		day.AllDone = len(occurrences) > 0
		for _, occurrence := range occurrences {
			if occurrence.Kind == KindInjection {
				day.HasInjection = true
			}
			switch occurrence.Status {
			case StatusDone:
			case StatusPending, StatusScheduled:
				day.AnyPending = true
				day.AllDone = false
			case StatusMissed:
				day.AllDone = false
			}
		}

		days = append(days, day)
	})

	return days
}

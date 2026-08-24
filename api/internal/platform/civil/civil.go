// Package civil holds the calendar vocabulary every bounded context carries, so that
// none of them imports another for a date.
//
// UserID sits here despite the name: a patient id is the one identifier all four
// clinical contexts hold, and identity exports no type for it. If it ever does, this
// is the first thing to move.
package civil

import (
	"fmt"
	"time"
)

// Date is a calendar day with no location: a schedule is the patient's own, and carrying a
// timestamp would make "the same day" a question about a timezone. Comparable with ==,
// which every match in this package depends on.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

func NewDate(year int, month time.Month, day int) Date {
	return Date{Year: year, Month: month, Day: day}
}

func (d Date) time() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

func fromTime(t time.Time) Date {
	return Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}
}

// Valid reports whether the calendar has this day.
//
// The type admits days it does not: Date{2026, February, 30} is representable, and that is
// worse than merely wrong — == compares the fields and says «a different day», while Before
// and After go through time and say «the same day as 2 March». A dose logged against such a
// date closes no occurrence, and its two readers disagree without either being able to say so.
//
// Measured rather than tabulated: time normalises an out-of-range day instead of refusing it,
// so a round trip through it is the check, and February is right in a leap year for free.
func (d Date) Valid() bool { return fromTime(d.time()) == d }

// String is the ISO form, which is what both the wire and the database want. Fixed width:
// a year under four digits or a month under two is a date PostgreSQL parses as another one.
func (d Date) String() string { return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day) }

func (d Date) AddDays(days int) Date { return fromTime(d.time().AddDate(0, 0, days)) }

func (d Date) DaysUntil(other Date) int {
	return int(other.time().Sub(d.time()).Hours()) / 24
}

func (d Date) Before(other Date) bool { return d.time().Before(other.time()) }

func (d Date) After(other Date) bool { return d.time().After(other.time()) }

func (d Date) Weekday() time.Weekday { return d.time().Weekday() }

func MinDate(a, b Date) Date {
	if a.Before(b) {
		return a
	}
	return b
}

func MaxDate(a, b Date) Date {
	if a.After(b) {
		return a
	}
	return b
}

// Slot is a time of day to the minute: §03's times[] is what makes BPC-157 at 08:00 and at
// 20:00 two occurrences that are logged apart.
type Slot struct {
	Hour   int
	Minute int
}

// Valid reports whether the clock has this time, for the reason Date.Valid exists: the type
// admits Slot{Hour: 99}, which renders as «99:00» and reaches a time column as a 22007 that
// no caller classifies. Written out rather than round-tripped — unlike a date, a time has no
// normalising constructor to compare against.
func (s Slot) Valid() bool {
	return s.Hour >= 0 && s.Hour <= 23 && s.Minute >= 0 && s.Minute <= 59
}

// String is the wire and database form. Fixed width for the same reason a date is.
func (s Slot) String() string { return fmt.Sprintf("%02d:%02d", s.Hour, s.Minute) }

// Range includes both edges, because a window is a set of days and not a subtraction.
//
// An inverted one is not harmless: Contains answers false for every day, but Days() returns a
// negative count — measured, −8 for a window inverted by nine days — so a caller dividing by
// it or iterating over it is asking a question the value cannot answer. Kotlin's TrendRange
// refused one in its constructor (`require(from <= through)`), and NewRange below is that
// refusal returned, which is what step 1 recorded as owed to the step that builds a window
// from a parameter.
//
// The struct stays constructible without it: a literal is how the suites write one, and a
// type whose fields are public cannot promise more than its constructor is asked for. What
// the constructor gives is one place for a caller taking a month from the wire to find out.
type Range struct {
	From    Date
	Through Date
}

// NewRange is the checked constructor: it refuses a window that runs backwards, and a window
// whose edges are not days the calendar has.
func NewRange(from, through Date) (Range, bool) {
	if !from.Valid() || !through.Valid() || through.Before(from) {
		return Range{}, false
	}

	return Range{From: from, Through: through}, true
}

// MonthOf is the window a month parameter means: the first of that month through its last
// day. The last day is found by stepping back from the first of the next one, so February
// and the leap year come out right without a table.
func MonthOf(year int, month time.Month) (Range, bool) {
	first := NewDate(year, month, 1)
	if !first.Valid() {
		return Range{}, false
	}

	return NewRange(first, fromTime(first.time().AddDate(0, 1, -1)))
}

func (r Range) Contains(d Date) bool { return !d.Before(r.From) && !d.After(r.Through) }

func (r Range) Days() int { return r.From.DaysUntil(r.Through) + 1 }

// Each walks the window a day at a time, from its first to its last. A method rather than
// arithmetic at each call site: «from, through, inclusive» written out is where one caller
// writes `<` and stops a day early.
func (r Range) Each(visit func(Date)) {
	for d := r.From; !d.After(r.Through); d = d.AddDays(1) {
		visit(d)
	}
}

// ISOWeekday converts Go's Sunday-from-zero weekday to the 1..7 the schema stores.
func ISOWeekday(day time.Weekday) int {
	if day == time.Sunday {
		return 7
	}
	return int(day)
}

func WeekdayFromISO(iso int) (time.Weekday, bool) {
	switch {
	case iso == 7:
		return time.Sunday, true
	case iso >= 1 && iso <= 6:
		return time.Weekday(iso), true
	default:
		return time.Sunday, false
	}
}

// UserID identifies a person the clinic knows about — a patient, a doctor or an
// admin. Typed for the reason every id in this codebase is: eleven contexts hang off
// the same handful of foreign keys, and a parameter list of strings is where a
// patient id ends up in a vial id's place with everything still compiling.
type UserID string

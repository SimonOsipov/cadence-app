package protocol

import "time"

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

// Range includes both edges, because a window is a set of days and not a subtraction. It
// does not enforce that it runs forwards — Kotlin's TrendRange did, and the aggregates of
// step 9, which build a window from a month parameter, are what need the checked
// constructor back. Until then an inverted Range answers «no days» rather than refusing.
type Range struct {
	From    Date
	Through Date
}

func (r Range) Contains(d Date) bool { return !d.Before(r.From) && !d.After(r.Through) }

func (r Range) Days() int { return r.From.DaysUntil(r.Through) + 1 }

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

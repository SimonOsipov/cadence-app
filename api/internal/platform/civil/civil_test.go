package civil

import (
	"testing"
	"time"
)

func TestDaysUntilCountsForwardsAndBackwardsAcrossAMonth(t *testing.T) {
	from := NewDate(2026, time.May, 10)

	if got := from.DaysUntil(NewDate(2026, time.June, 7)); got != 28 {
		t.Fatalf("28 days to 7 June, got %d", got)
	}
	if got := from.DaysUntil(NewDate(2026, time.May, 9)); got != -1 {
		t.Fatalf("the day before is -1, got %d", got)
	}
}

func TestAddDaysCrossesALeapFebruary(t *testing.T) {
	// 2028 is a leap year: an implementation stepping by 365-day years lands a day early.
	if got := NewDate(2028, time.February, 14).AddDays(27); got != NewDate(2028, time.March, 12) {
		t.Fatalf("day 27 after 14 February 2028 is 12 March, got %v", got)
	}
}

func TestISOWeekdayPutsMondayFirstAndSundaySeventh(t *testing.T) {
	// time.Weekday counts Sunday from zero; the column is ISO. Both ends, not the middle.
	if got := ISOWeekday(time.Monday); got != 1 {
		t.Fatalf("Monday is 1, got %d", got)
	}
	if got := ISOWeekday(time.Sunday); got != 7 {
		t.Fatalf("Sunday is 7, got %d", got)
	}
	for iso := 1; iso <= 7; iso++ {
		day, ok := WeekdayFromISO(iso)
		if !ok || ISOWeekday(day) != iso {
			t.Fatalf("iso %d did not round-trip: %v %v", iso, day, ok)
		}
	}
	if _, ok := WeekdayFromISO(0); ok {
		t.Fatal("0 is not an ISO weekday")
	}
	if _, ok := WeekdayFromISO(8); ok {
		t.Fatal("8 is not an ISO weekday")
	}
}

func TestARangeOfOneDayIsOneDayLongAndContainsIt(t *testing.T) {
	// A one-day window is what «весь цикл» asks for on the day a course begins.
	day := NewDate(2026, time.May, 10)
	r := Range{From: day, Through: day}

	if r.Days() != 1 {
		t.Fatalf("both edges included, got %d", r.Days())
	}
	if !r.Contains(day) {
		t.Fatal("a range holds its own single day")
	}
	if r.Contains(day.AddDays(1)) || r.Contains(day.AddDays(-1)) {
		t.Fatal("and holds nothing either side")
	}
}

func TestTheWeekdayIsTheCalendarsAndNotAnOffset(t *testing.T) {
	if got := NewDate(2026, time.May, 10).Weekday(); got != time.Sunday {
		t.Fatalf("10 May 2026 is a Sunday, got %v", got)
	}
}

func TestMinAndMaxPickTheEarlierAndTheLater(t *testing.T) {
	early := NewDate(2026, time.May, 10)
	late := NewDate(2026, time.August, 1)

	if MinDate(late, early) != early || MinDate(early, late) != early {
		t.Error("MinDate is the earlier one whichever side it arrives on")
	}
	if MaxDate(late, early) != late || MaxDate(early, late) != late {
		t.Error("MaxDate is the later one whichever side it arrives on")
	}
	if MinDate(early, early) != early || MaxDate(early, early) != early {
		t.Error("two equal days are their own min and max")
	}
}

func TestADayAndASlotRenderFixedWidth(t *testing.T) {
	// The year and the month are what a variable width breaks: «26-5-4» is a date
	// PostgreSQL parses, and not the one meant.
	for _, day := range []struct {
		date Date
		want string
	}{
		{NewDate(2026, time.May, 4), "2026-05-04"},
		{NewDate(926, time.December, 31), "0926-12-31"},
		{NewDate(2026, time.January, 1), "2026-01-01"},
	} {
		if got := day.date.String(); got != day.want {
			t.Errorf("%v rendered %q, want %q", day.date, got, day.want)
		}
	}

	for _, slot := range []struct {
		at   Slot
		want string
	}{
		{Slot{Hour: 8}, "08:00"},
		{Slot{Hour: 20, Minute: 30}, "20:30"},
		{Slot{}, "00:00"},
	} {
		if got := slot.at.String(); got != slot.want {
			t.Errorf("%v rendered %q, want %q", slot.at, got, slot.want)
		}
	}
}

// A day the type admits and the calendar does not. §03's calendar arithmetic runs through
// time, and == runs over the fields, so an impossible date makes the two disagree: they call
// it a different day and the same day at once, and a dose logged against it closes no
// occurrence.
func TestADayTheCalendarDoesNotHaveIsRefused(t *testing.T) {
	for _, refused := range []Date{
		{Year: 2026, Month: time.February, Day: 30},
		{Year: 2026, Month: time.April, Day: 31},
		{Year: 2026, Month: time.January, Day: 0},
		{Year: 2026, Month: time.January, Day: 32},
		{Year: 2026, Month: time.Month(0), Day: 1},
		{Year: 2026, Month: time.Month(13), Day: 1},
	} {
		if refused.Valid() {
			t.Errorf("%v passed as a day", refused)
		}
	}

	// The leap day, which is the case a naive month-length table gets wrong.
	for _, accepted := range []Date{
		{Year: 2024, Month: time.February, Day: 29},
		{Year: 2026, Month: time.February, Day: 28},
		{Year: 2026, Month: time.December, Day: 31},
		{Year: 2026, Month: time.January, Day: 1},
	} {
		if !accepted.Valid() {
			t.Errorf("%v was refused as a day", accepted)
		}
	}
}

func TestATimeTheClockDoesNotHaveIsRefused(t *testing.T) {
	for _, refused := range []Slot{
		{Hour: 24}, {Hour: -1}, {Hour: 99}, {Hour: 8, Minute: 60}, {Hour: 8, Minute: -1},
	} {
		if refused.Valid() {
			t.Errorf("%v passed as a time", refused)
		}
	}

	// Both ends, and midnight, which a bound written as «> 0» would refuse.
	for _, accepted := range []Slot{{}, {Hour: 23, Minute: 59}, {Hour: 8}, {Hour: 20, Minute: 30}} {
		if !accepted.Valid() {
			t.Errorf("%v was refused as a time", accepted)
		}
	}
}

// The constructor step 1 recorded as owed to step 9, and the reason: an inverted window
// answers Contains false for every day and Days() a negative number, so a caller dividing by
// it or iterating over it asks a question the value cannot answer.
func TestAWindowThatRunsBackwardsIsRefused(t *testing.T) {
	for _, refused := range []struct{ from, through Date }{
		{NewDate(2026, time.May, 10), NewDate(2026, time.May, 1)},
		{NewDate(2026, time.May, 10), NewDate(2025, time.May, 10)},
		// An edge the calendar does not have, which Contains and Days would answer
		// about as though it did.
		{Date{Year: 2026, Month: time.February, Day: 30}, NewDate(2026, time.March, 5)},
		{NewDate(2026, time.March, 1), Date{Year: 2026, Month: time.April, Day: 31}},
	} {
		if got, ok := NewRange(refused.from, refused.through); ok {
			t.Errorf("NewRange(%v, %v) gave %v", refused.from, refused.through, got)
		}
	}

	// One day is a window of one day, not an empty one — both edges are included.
	one := NewDate(2026, time.May, 10)
	if window, ok := NewRange(one, one); !ok || window.Days() != 1 {
		t.Errorf("a single day gave %v, %v", window, ok)
	}
}

// The window a month parameter means, and February is where a month-length table gets it
// wrong: this steps back from the first of the next month instead.
func TestTheWindowOfAMonthEndsOnItsLastDay(t *testing.T) {
	for _, month := range []struct {
		year int
		of   time.Month
		last int
	}{
		{2026, time.February, 28},
		{2024, time.February, 29},
		{2026, time.April, 30},
		{2026, time.December, 31},
		{2026, time.January, 31},
	} {
		window, ok := MonthOf(month.year, month.of)
		if !ok {
			t.Errorf("%v %d was refused", month.of, month.year)

			continue
		}
		if window.From != NewDate(month.year, month.of, 1) {
			t.Errorf("%v %d opens on %v", month.of, month.year, window.From)
		}
		if window.Through != NewDate(month.year, month.of, month.last) {
			t.Errorf("%v %d ends on %v, want the %dth", month.of, month.year, window.Through, month.last)
		}
		if window.Days() != month.last {
			t.Errorf("%v %d is %d days", month.of, month.year, window.Days())
		}
	}

	if _, ok := MonthOf(2026, time.Month(13)); ok {
		t.Error("a thirteenth month was accepted")
	}
}

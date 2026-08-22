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

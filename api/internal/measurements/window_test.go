package measurements

import (
	"slices"
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// A Sunday in the middle of the seeded course, far enough from any month edge that the
// arithmetic is not read off a calendar boundary by accident.
var trendsToday = civil.NewDate(2026, time.May, 10)

func aCourseFrom(start civil.Date, weeks int, status protocol.ProtocolStatus) *protocol.Protocol {
	return &protocol.Protocol{
		ID:        protocol.ProtocolID("pr"),
		PatientID: civil.UserID("p"),
		StartDate: start,
		Weeks:     weeks,
		Status:    status,
	}
}

// Both edges included, and today is one of the days: «за 7 дней» ending today is the six days
// before it and today, not seven days before it.
func TestTheFixedWindowsCountBackFromTheDayTheyAreGiven(t *testing.T) {
	cases := []struct {
		window Window
		from   civil.Date
		days   int
	}{
		{WindowWeek, civil.NewDate(2026, time.May, 4), 7},
		{WindowFourWeeks, civil.NewDate(2026, time.April, 13), 28},
		{WindowThreeMonths, civil.NewDate(2026, time.February, 16), 84},
	}

	for _, c := range cases {
		t.Run(string(c.window), func(t *testing.T) {
			// No course at all, and the three fixed windows still answer: their
			// length is the whole of their definition.
			got, ok := RangeOn(c.window, nil, trendsToday)
			if !ok {
				t.Fatal("answered no window")
			}
			if got.From != c.from || got.Through != trendsToday {
				t.Errorf("window is %s..%s, not %s..%s",
					got.From, got.Through, c.from, trendsToday)
			}
			if got.Days() != c.days {
				t.Errorf("window is %d days, not %d", got.Days(), c.days)
			}
		})
	}
}

// The cycle is geometry: the course's own start, and today or the last day it prescribes,
// whichever comes first. Status is not asked, which is what keeps the axis on the screen of a
// patient whose course the doctor closed yesterday.
func TestTheCycleWindowIsTheGeometryOfTheCourse(t *testing.T) {
	// The same geometry under each of the three statuses, and the same window expected of
	// all three: «status is not asked» is a claim a fixture of running courses cannot make,
	// and a cycle that filtered on `active` would blank the axis of every patient whose
	// course the doctor closed — with the whole suite still green.
	running := civil.NewDate(2026, time.April, 5)

	cases := []struct {
		name    string
		course  *protocol.Protocol
		today   civil.Date
		from    civil.Date
		through civil.Date
	}{
		{
			name:    "a course still running is bounded by today",
			course:  aCourseFrom(running, 12, protocol.StatusActive),
			today:   trendsToday,
			from:    running,
			through: trendsToday,
		},
		{
			name:    "a cancelled course of the same shape gives the same window",
			course:  aCourseFrom(running, 12, protocol.StatusCancelled),
			today:   trendsToday,
			from:    running,
			through: trendsToday,
		},
		{
			// Twelve weeks from 4 January is 84 days, the last of them 28 March.
			name:    "a completed course is bounded by its last prescribed day",
			course:  aCourseFrom(civil.NewDate(2026, time.January, 4), 12, protocol.StatusCompleted),
			today:   trendsToday,
			from:    civil.NewDate(2026, time.January, 4),
			through: civil.NewDate(2026, time.March, 28),
		},
		{
			name:    "the day the course ends, the two bounds meet",
			course:  aCourseFrom(civil.NewDate(2026, time.February, 16), 12, protocol.StatusActive),
			today:   civil.NewDate(2026, time.May, 10),
			from:    civil.NewDate(2026, time.February, 16),
			through: civil.NewDate(2026, time.May, 10),
		},
		{
			name:    "a course starting today is one day wide",
			course:  aCourseFrom(trendsToday, 12, protocol.StatusActive),
			today:   trendsToday,
			from:    trendsToday,
			through: trendsToday,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := RangeOn(WindowCycle, c.course, c.today)
			if !ok {
				t.Fatal("answered no window")
			}
			if got.From != c.from || got.Through != c.through {
				t.Errorf("window is %s..%s, not %s..%s",
					got.From, got.Through, c.from, c.through)
			}
		})
	}
}

// Both empty cases, and they are two rather than one: a patient who has never been prescribed
// anything, and one whose course starts next week. The screen says «no data» for the same
// reason in both, but a single test would leave whichever of the two it did not write
// answering with an inverted range.
func TestTheCycleWindowIsEmptyWithoutACourseAndBeforeItStarts(t *testing.T) {
	t.Run("no course at all", func(t *testing.T) {
		if got, ok := RangeOn(WindowCycle, nil, trendsToday); ok {
			t.Errorf("answered %s..%s for a patient with no course", got.From, got.Through)
		}
	})

	t.Run("a course that has not started", func(t *testing.T) {
		tomorrow := trendsToday.AddDays(1)
		course := aCourseFrom(tomorrow, 12, protocol.StatusActive)
		if got, ok := RangeOn(WindowCycle, course, trendsToday); ok {
			t.Errorf("answered %s..%s for a course starting tomorrow", got.From, got.Through)
		}
	})
}

// The parser is the gate, so this cannot arrive from the wire — but a window that is not a
// member must not fall through to somebody else's answer. Empty, and not the cycle.
func TestAWindowOffTheSetAnswersNothing(t *testing.T) {
	course := aCourseFrom(trendsToday, 12, protocol.StatusActive)
	if got, ok := RangeOn(Window("year"), course, trendsToday); ok {
		t.Errorf("answered %s..%s for a window off the set", got.From, got.Through)
	}
}

func TestTheWindowCodesOnTheWireAreTheWrittenOutOnes(t *testing.T) {
	if want := []Window{"7d", "4w", "3m", "cycle"}; !slices.Equal(Windows(), want) {
		t.Errorf("the windows are %v, not %v", Windows(), want)
	}

	for _, window := range Windows() {
		if parsed, ok := ParseWindow(string(window)); !ok || parsed != window {
			t.Errorf("%q parses to (%q, %v)", window, parsed, ok)
		}
	}

	for _, off := range []string{"", "7D", " 7d", "week", "cycle "} {
		if parsed, ok := ParseWindow(off); ok {
			t.Errorf("%q parsed as %q", off, parsed)
		}
	}
}

// The day is the patient's, and the moment is not it. At 00:20 in Yekaterinburg it is still
// the previous day in UTC, so a window counted from the server's day is a day short at both
// edges — the reading taken this morning would draw outside the window that is meant to hold
// it. The windows themselves take a date, which is what makes this the caller's obligation and
// this test the record of it.
func TestTheWindowFollowsThePatientsDayAcrossLocalMidnight(t *testing.T) {
	where, err := time.LoadLocation("Asia/Yekaterinburg")
	if err != nil {
		t.Fatalf("loading the patient's zone: %v", err)
	}

	// 19:20 UTC on 9 May is 00:20 on 10 May in a zone five hours ahead.
	moment := time.Date(2026, time.May, 9, 19, 20, 0, 0, time.UTC)
	local := moment.In(where)
	patients := civil.NewDate(local.Year(), local.Month(), local.Day())
	servers := civil.NewDate(moment.Year(), moment.Month(), moment.Day())

	if patients == servers {
		t.Fatalf("the fixture is not on the boundary: both zones say %s", patients)
	}

	mine, ok := RangeOn(WindowWeek, nil, patients)
	if !ok {
		t.Fatal("answered no window")
	}
	if want := civil.NewDate(2026, time.May, 4); mine.From != want || mine.Through != patients {
		t.Errorf("the patient's week is %s..%s, not %s..%s",
			mine.From, mine.Through, want, patients)
	}

	theirs, ok := RangeOn(WindowWeek, nil, servers)
	if !ok {
		t.Fatal("answered no window")
	}
	if theirs.Through == mine.Through {
		t.Errorf("the two zones answered the same window ending %s", theirs.Through)
	}

	// The minute before, and the local day is the earlier one: the boundary is crossed
	// between these two moments and nowhere else.
	before := moment.Add(-21 * time.Minute).In(where)
	if got := civil.NewDate(before.Year(), before.Month(), before.Day()); got != servers {
		t.Errorf("23:59 in the patient's zone is %s, not %s", got, servers)
	}
}

// The cycle window at the same boundary: the day after a course's last prescribed day, the
// patient's zone has already turned over while the server's has not, and the axis must stop
// at the course rather than run a day past it.
func TestTheCycleWindowClampsInThePatientsZone(t *testing.T) {
	where, err := time.LoadLocation("Asia/Yekaterinburg")
	if err != nil {
		t.Fatalf("loading the patient's zone: %v", err)
	}

	course := aCourseFrom(civil.NewDate(2026, time.February, 16), 12, protocol.StatusCompleted)
	last := course.LastPrescribedDay()

	// 00:20 in the patient's zone on the day after the course's last day.
	moment := time.Date(last.Year, last.Month, last.Day, 19, 20, 0, 0, time.UTC)
	local := moment.In(where)
	patients := civil.NewDate(local.Year(), local.Month(), local.Day())

	got, ok := RangeOn(WindowCycle, course, patients)
	if !ok {
		t.Fatal("answered no window")
	}
	if got.Through != last {
		t.Errorf("the cycle runs through %s, not the last prescribed day %s", got.Through, last)
	}
}

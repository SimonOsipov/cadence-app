package protocol

import (
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

var (
	january  = time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC)
	february = time.Date(2026, time.February, 1, 9, 0, 0, 0, time.UTC)
	may      = civil.NewDate(2026, time.May, 1)
	june     = civil.NewDate(2026, time.June, 1)
)

func courseRow(id string, status ProtocolStatus, start civil.Date, created time.Time) Protocol {
	return Protocol{
		ID:        ProtocolID(id),
		PatientID: civil.UserID("p"),
		StartDate: start,
		Weeks:     12,
		Status:    status,
		CreatedAt: created,
	}
}

// Every rung of the key decides in one case and points the wrong way in another: an id that
// already agrees with the answer leaves the rung above it unmeasured, which is how a course
// picked by «the greatest id» would pass a suite written the obvious way.
func TestTheLastCourseIsTheOneTheKeyNames(t *testing.T) {
	cases := []struct {
		name    string
		courses []Protocol
		want    ProtocolID
	}{
		{
			// Deliberately first in the slice, so «take the last one» fails here while
			// «take the first» fails in every case below.
			name: "the running course wins over one that starts later",
			courses: []Protocol{
				courseRow("b7", StatusActive, may, january),
				courseRow("f0", StatusCompleted, june, february),
			},
			want: "b7",
		},
		{
			name: "with none running, the latest start wins over a later created_at and a greater id",
			courses: []Protocol{
				courseRow("z9", StatusCancelled, may, february),
				courseRow("a1", StatusCompleted, june, january),
			},
			want: "a1",
		},
		{
			name: "on the same start, the later created_at wins over a greater id",
			courses: []Protocol{
				courseRow("z9", StatusCompleted, may, january),
				courseRow("a1", StatusCompleted, may, february),
			},
			want: "a1",
		},
		{
			// The same instant is what two rows written in one transaction carry:
			// now() is the transaction's, so created_at cannot separate them and the
			// id is the whole of the answer.
			name: "on the same start and the same instant, the greater id wins",
			courses: []Protocol{
				courseRow("a1", StatusCompleted, may, january),
				courseRow("z9", StatusCompleted, may, january),
			},
			want: "z9",
		},
		{
			// Geometry, not status: the cycle window of a patient between courses is
			// the last one they were on, and asking about its status would blank the
			// screen the day the doctor closes it.
			name: "a cancelled course is still the last course",
			courses: []Protocol{
				courseRow("a1", StatusCancelled, may, january),
			},
			want: "a1",
		},
	}

	// Each case both ways round, and that is what measures the rungs rather than half of
	// them: the walk compares each candidate against the standing winner, so one order only
	// ever asks «is the candidate greater» and a rung answering «lesser» is never reached.
	// Reversed, the same case asks the other question of the same rung.
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, order := range []struct {
				name    string
				courses []Protocol
			}{
				{"as they arrived", c.courses},
				{"reversed", reverse(c.courses)},
			} {
				t.Run(order.name, func(t *testing.T) {
					got, ok := latestCourse(order.courses)
					if !ok {
						t.Fatalf("answered no course out of %d", len(order.courses))
					}
					if got.ID != c.want {
						t.Errorf("chose %s, not %s", got.ID, c.want)
					}
				})
			}
		})
	}
}

func reverse(courses []Protocol) []Protocol {
	backwards := make([]Protocol, 0, len(courses))
	for i := len(courses) - 1; i >= 0; i-- {
		backwards = append(backwards, courses[i])
	}

	return backwards
}

func TestAPatientWithNoCourseHasNoLastOne(t *testing.T) {
	if _, ok := latestCourse(nil); ok {
		t.Error("answered a course out of none")
	}
}

// The answer does not depend on the order the rows arrived in, and that is the whole reason
// the key is written out: `protocols` has no order of its own, and a SELECT without one is
// free to hand the same three rows back in any order.
//
// No running course in the fixture on purpose: the status rung decides in both directions at
// once, so a fixture holding one is settled before the dates are ever compared and cannot fail
// for the reason this test names.
func TestTheOrderTheRowsArriveInDoesNotDecide(t *testing.T) {
	courses := []Protocol{
		courseRow("f0", StatusCompleted, june, february),
		courseRow("b7", StatusCancelled, may, january),
		courseRow("z9", StatusCompleted, may, february),
	}

	forwards, ok := latestCourse(courses)
	if !ok {
		t.Fatal("answered no course")
	}

	backwards, ok := latestCourse(reverse(courses))
	if !ok {
		t.Fatal("answered no course")
	}

	if forwards.ID != backwards.ID {
		t.Errorf("the order decided: %s forwards, %s backwards", forwards.ID, backwards.ID)
	}
	if forwards.ID != "f0" {
		t.Errorf("chose %s, not the course that started latest", forwards.ID)
	}
}

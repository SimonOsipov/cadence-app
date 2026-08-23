package protocol

import (
	"testing"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

// Every boundary, from both sides. The words are Russian's own and not even quarters, so the
// hours are the thing to pin: a rule rounded to six-hour blocks agrees with this one on
// midnight and on noon and disagrees on five, eighteen and twenty-three.
func TestEachBoundaryOfTheDayIsWhereRussianPutsIt(t *testing.T) {
	for _, at := range []struct {
		hour int
		want PartOfDay
	}{
		{0, Night},
		{4, Night},
		{5, Morning},
		{11, Morning},
		{12, Afternoon},
		{17, Afternoon},
		{18, Evening},
		{23, Evening},
	} {
		if got := PartOfDayAt(civil.Slot{Hour: at.hour}); got != at.want {
			t.Errorf("%02d:00 is %q, want %q", at.hour, got, at.want)
		}
	}

	// The minute does not move the word, at either edge of an hour.
	for _, minute := range []int{0, 59} {
		if got := PartOfDayAt(civil.Slot{Hour: 11, Minute: minute}); got != Morning {
			t.Errorf("11:%02d is %q", minute, got)
		}
	}
}

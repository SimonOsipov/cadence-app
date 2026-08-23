package protocol

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

// The neighbours as this package sees them: three answers and nothing else. Stubbing them is
// the whole point of the interfaces living here — the aggregate's rules are testable without
// a database, and without dosing or inventory existing at all.
type stubNeighbours struct {
	site    string
	logged  []LoggedSlot
	left    *int
	reorder *ReorderHint

	askedFor []ProtocolItemID
	windows  []civil.Range
	fail     error
}

func (s *stubNeighbours) SuggestNextSite(context.Context, pgx.Tx, civil.UserID) (string, error) {
	return s.site, s.fail
}

func (s *stubNeighbours) LoggedSlotsIn(
	_ context.Context, _ pgx.Tx, _ civil.UserID, window civil.Range,
) ([]LoggedSlot, error) {
	s.windows = append(s.windows, window)

	return s.logged, s.fail
}

func (s *stubNeighbours) SupplyFor(
	_ context.Context, _ pgx.Tx, _ civil.UserID, item ProtocolItem, _ civil.Date,
) (*int, *ReorderHint, error) {
	s.askedFor = append(s.askedFor, item.ID)

	return s.left, s.reorder, s.fail
}

var theCompound = Compound{
	ID: "3c1f3b7c-0000-4000-8000-00000000000d", NameRU: "Семаглутид",
	DefaultUnit: MG, Route: "sc", Icon: "syringe",
}

func todayFor(t *testing.T, plan Plan, running bool, at civil.Date, clock civil.Slot, n *stubNeighbours) Today {
	t.Helper()

	today, err := TodayFor(t.Context(), nil, "3c1f3b7c-0000-4000-8000-0000000000a1",
		plan, []Compound{theCompound}, running, at, clock, n, n, n)
	if err != nil {
		t.Fatalf("assembling: %v", err)
	}

	return today
}

// A patient with no running course still has a screen: the day, the part of it, and the zone
// the wizard would open on. Everything else is a fact about a course, and inventing zeros for
// them would draw a card about a prescription that does not exist.
func TestAPatientWithNoCourseStillHasADay(t *testing.T) {
	n := &stubNeighbours{site: "l-abdomen"}

	today := todayFor(t, Plan{}, false, civil.NewDate(2026, time.May, 10), civil.Slot{Hour: 9}, n)

	if today.Date != civil.NewDate(2026, time.May, 10) || today.PartOfDay != Morning {
		t.Errorf("the day reads %v %q", today.Date, today.PartOfDay)
	}
	if today.SuggestedSite != "l-abdomen" {
		t.Errorf("the zone is %q", today.SuggestedSite)
	}
	if today.CycleWeek != nil || today.NextDose != nil || today.WeekProtocol != nil {
		t.Errorf("a course's facts are answered without one: %+v", today)
	}
	if today.VialDosesLeft != nil || today.Reorder != nil || today.NextTitration != nil {
		t.Errorf("the cabinet answered for a patient with no course: %+v", today)
	}
	if len(n.askedFor) != 0 {
		t.Errorf("the cabinet was asked about %v", n.askedFor)
	}
}

// The next dose is the next *injection* still open, and the strip is every item. A supplement
// is on the strip and is never the hero card's dose — the two readers disagree by design, and
// a single «next occurrence» would put a vitamin in the hero card.
func TestTheNextDoseIsAnInjectionAndTheStripIsEverything(t *testing.T) {
	n := &stubNeighbours{site: "r-glute"}

	// 10 May is the Sunday the injection falls on; the supplement is daily at 08:00 and
	// 20:00, so its morning slot comes first in the day.
	today := todayFor(t, aSchedulePlan(), true, civil.NewDate(2026, time.May, 10), civil.Slot{Hour: 7}, n)

	if today.NextDose == nil {
		t.Fatal("nothing is next")
	}
	if today.NextDose.Kind != KindInjection {
		t.Errorf("the hero card's dose is a %s", today.NextDose.Kind)
	}
	if len(today.WeekProtocol) != 2 {
		t.Errorf("the strip holds %d rows, want both items", len(today.WeekProtocol))
	}
	if today.NextDoseCompound == nil || today.NextDoseCompound.NameRU != "Семаглутид" {
		t.Errorf("the hero card names %v", today.NextDoseCompound)
	}
	if today.CycleWeek == nil || *today.CycleWeek != 1 {
		t.Errorf("the week is %v", today.CycleWeek)
	}
}

// Logged today, and by the injection: the supplement's slots closing does not make «доза
// записана» true, and the hero card would say so if the flag were about any occurrence.
func TestTheDoseLoggedFlagIsAboutTheInjection(t *testing.T) {
	day := civil.NewDate(2026, time.May, 10)

	for _, state := range []struct {
		name   string
		logged []LoggedSlot
		want   bool
	}{
		{"nothing logged", nil, false},
		{
			"both of the supplement's slots", []LoggedSlot{
				{ItemID: "item-supplement", Date: day, Time: &civil.Slot{Hour: 8}},
				{ItemID: "item-supplement", Date: day, Time: &civil.Slot{Hour: 20}},
			}, false,
		},
		{
			"the injection", []LoggedSlot{
				{ItemID: "item-injection", Date: day, Time: &civil.Slot{Hour: 8}},
			}, true,
		},
	} {
		t.Run(state.name, func(t *testing.T) {
			n := &stubNeighbours{site: "r-glute", logged: state.logged}
			today := todayFor(t, aSchedulePlan(), true, day, civil.Slot{Hour: 21}, n)

			if today.DoseLoggedToday != state.want {
				t.Errorf("logged=%v, want %v", today.DoseLoggedToday, state.want)
			}
			// And once it is logged there is no next dose to offer.
			if state.want && today.NextDose != nil {
				t.Errorf("a dose is offered after one was logged: %+v", today.NextDose)
			}
		})
	}
}

// The cabinet is asked about the item the hero card names, and about no other. Asking per
// item would put a supplement's supply under an injection's card.
func TestTheCabinetIsAskedAboutTheDoseBeingOffered(t *testing.T) {
	left := 3
	n := &stubNeighbours{
		site: "r-glute", left: &left,
		reorder: &ReorderHint{CompoundID: theCompound.ID, WeeksLeft: 2},
	}

	today := todayFor(t, aSchedulePlan(), true, civil.NewDate(2026, time.May, 10), civil.Slot{Hour: 7}, n)

	if len(n.askedFor) != 1 || n.askedFor[0] != "item-injection" {
		t.Errorf("the cabinet was asked about %v", n.askedFor)
	}
	if today.VialDosesLeft == nil || *today.VialDosesLeft != 3 {
		t.Errorf("the vial has %v doses left", today.VialDosesLeft)
	}
	if today.Reorder == nil || today.Reorder.WeeksLeft != 2 {
		t.Errorf("the reorder hint reads %+v", today.Reorder)
	}
}

// The history is read for the day the summary is about, and once. A window of a week would be
// read and then filtered to the same day, and a window of the wrong day would answer about
// somebody's yesterday.
func TestTheHistoryIsReadForThatDayAndOnce(t *testing.T) {
	n := &stubNeighbours{site: "r-glute"}
	day := civil.NewDate(2026, time.May, 10)

	todayFor(t, aSchedulePlan(), true, day, civil.Slot{Hour: 7}, n)

	if len(n.windows) != 1 {
		t.Fatalf("the history was read %d times", len(n.windows))
	}
	if n.windows[0].From != day || n.windows[0].Through != day {
		t.Errorf("the window is %v..%v, want that day", n.windows[0].From, n.windows[0].Through)
	}
}

// A neighbour that fails fails the whole answer: half a hero card is a screen that says
// something untrue about a patient's own treatment.
func TestANeighbourThatFailsFailsTheAnswer(t *testing.T) {
	n := &stubNeighbours{site: "r-glute", fail: errors.New("the cabinet is shut")}

	if _, err := TodayFor(t.Context(), nil, "3c1f3b7c-0000-4000-8000-0000000000a1",
		aSchedulePlan(), nil, true, civil.NewDate(2026, time.May, 10), civil.Slot{Hour: 7},
		n, n, n); err == nil {
		t.Error("a failing neighbour was answered around")
	}
}

// Every field of a context that is not built is absent, and none of them is a zero. «0 из 4
// приёмов» over a nutrition context that does not exist is a lie the client cannot detect.
func TestTheFieldsOfContextsThatDoNotExistAreAbsent(t *testing.T) {
	n := &stubNeighbours{site: "r-glute"}

	today := todayFor(t, aSchedulePlan(), true, civil.NewDate(2026, time.May, 10), civil.Slot{Hour: 7}, n)

	if today.MealCount != nil || today.MealMacros != nil || today.Targets != nil {
		t.Errorf("nutrition answered %+v", today)
	}
	if today.WeightKG != nil || today.TargetWeightKG != nil || today.WeightSeries != nil {
		t.Errorf("measurements answered %+v", today)
	}
}

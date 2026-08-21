package inventory

import (
	"fmt"
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

var (
	today   = protocol.NewDate(2026, time.May, 31)
	patient = protocol.UserID("p-1")
	sema    = protocol.CompoundID("sema")
	nextID  int
)

func vial(totalDoses int, openedAt *protocol.Date, expiresOn protocol.Date) Vial {
	nextID++

	return Vial{
		ID:                 VialID(fmt.Sprintf("v-%d", nextID)),
		PatientID:          patient,
		CompoundID:         sema,
		ConcentrationLabel: "1 мг/мл",
		TotalDoses:         totalDoses,
		OpenedAt:           openedAt,
		ExpiresOn:          expiresOn,
	}
}

func day(year int, month time.Month, d int) *protocol.Date {
	date := protocol.NewDate(year, month, d)

	return &date
}

var farOff = protocol.NewDate(2026, time.December, 31)

// An item whose cadence gives the weekly rate each case needs.
func weeklyItem(compound protocol.CompoundID, daysAWeek int) protocol.ProtocolItem {
	days := []time.Weekday{
		time.Monday, time.Tuesday, time.Wednesday, time.Thursday,
		time.Friday, time.Saturday, time.Sunday,
	}[:daysAWeek]

	return protocol.ProtocolItem{
		ID:         protocol.ProtocolItemID("item-" + string(compound)),
		ProtocolID: protocol.ProtocolID("pr"),
		Kind:       protocol.KindInjection,
		CompoundID: &compound,
		Cadence:    protocol.CadenceWeekly,
		DaysOfWeek: days,
		Times:      []protocol.Slot{{Hour: 7}},
		Loggable:   true,
	}
}

// drawn is the narrow shape RemainingDoses takes: which vial each logged dose came
// out of, and nothing else about it.
func drawn(v Vial, times int) []VialID {
	out := make([]VialID, times)
	for i := range out {
		out[i] = v.ID
	}

	return out
}

func TestLoggingADoseDecrementsTheVialItCameFrom(t *testing.T) {
	v := vial(4, nil, farOff)

	for _, c := range []struct{ logged, remaining int }{{0, 4}, {2, 2}, {4, 0}} {
		if got := RemainingDoses(v, drawn(v, c.logged)); got != c.remaining {
			t.Errorf("%d logged: want %d remaining, got %d", c.logged, c.remaining, got)
		}
	}
}

func TestDosesFromOtherVialsDoNotCount(t *testing.T) {
	// A subtraction ignoring the vial would drain both at once.
	mine := vial(4, nil, farOff)
	other := vial(4, nil, farOff)

	if got := RemainingDoses(mine, drawn(other, 3)); got != 4 {
		t.Errorf("another vial's doses counted: got %d, want 4", got)
	}
}

func TestRemainingNeverGoesNegative(t *testing.T) {
	v := vial(2, nil, farOff)

	if got := RemainingDoses(v, drawn(v, 5)); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestAnUnopenedVialIsSealedAndAnOpenedOneIsActive(t *testing.T) {
	if got := StatusOf(vial(4, nil, farOff), nil, today); got != StatusSealed {
		t.Errorf("unopened: got %v, want sealed", got)
	}
	if got := StatusOf(vial(4, day(2026, time.May, 1), farOff), nil, today); got != StatusActive {
		t.Errorf("opened: got %v, want active", got)
	}
}

func TestADisposedVialSaysSoWhateverElseIsTrueOfIt(t *testing.T) {
	v := vial(4, day(2026, time.May, 1), farOff)
	v.DisposedAt = day(2026, time.May, 20)

	if got := StatusOf(v, nil, today); got != StatusDisposed {
		t.Errorf("got %v, want disposed", got)
	}
}

func TestAVialBelowAQuarterIsLow(t *testing.T) {
	// §03's «low <25%», strict: exactly a quarter is not low. Eight doses, not four:
	// four's remainders land on 25% and 0% with nothing in between.
	v := vial(8, day(2026, time.May, 1), farOff)

	for _, c := range []struct {
		logged int
		want   VialStatus
		why    string
	}{
		{5, StatusActive, "three of eight"},
		{6, StatusActive, "exactly a quarter"},
		{7, StatusLow, "one of eight"},
	} {
		if got := StatusOf(v, drawn(v, c.logged), today); got != c.want {
			t.Errorf("%s: got %v, want %v", c.why, got, c.want)
		}
	}
}

func TestExpiryOutranksLowStock(t *testing.T) {
	soon := vial(8, day(2026, time.May, 1), protocol.NewDate(2026, time.June, 10))
	later := vial(8, day(2026, time.May, 1), protocol.NewDate(2026, time.July, 20))

	if got := StatusOf(soon, drawn(soon, 7), today); got != StatusExpiring {
		t.Errorf("expiring soon: got %v, want expiring", got)
	}
	if got := StatusOf(later, drawn(later, 7), today); got != StatusLow {
		t.Errorf("expiring later: got %v, want low", got)
	}
}

func TestASealedVialAboutToExpireSaysSo(t *testing.T) {
	// Measured: it read SEALED. Unopened stock about to be wasted is exactly the vial
	// worth warning about.
	v := vial(4, nil, protocol.NewDate(2026, time.June, 3))

	if got := StatusOf(v, nil, today); got != StatusExpiring {
		t.Errorf("got %v, want expiring", got)
	}
}

func TestAHintIsAboutOneCompoundAndCountsOnlyItsVials(t *testing.T) {
	// Measured: an unopened BPC vial once suppressed the semaglutide hint entirely.
	mine := vial(4, day(2026, time.May, 1), farOff)
	bpcSpare := vial(30, nil, farOff)
	bpcSpare.CompoundID = protocol.CompoundID("bpc")

	alone := ReorderHintFor(weeklyItem(sema, 1), []Vial{mine}, nil, today)
	withOtherCompound := ReorderHintFor(weeklyItem(sema, 1), []Vial{mine, bpcSpare}, nil, today)

	if alone == nil || withOtherCompound == nil || *alone != *withOtherCompound {
		t.Fatalf("a vial of another compound changed the answer: %v vs %v", alone, withOtherCompound)
	}
	if alone.CompoundID != sema {
		t.Errorf("the hint is about %v, want %v", alone.CompoundID, sema)
	}
}

func TestTheReorderHintNeedsBothNoSparesAndLittleSupply(t *testing.T) {
	// §03: «0 sealed spares & ≤4 weeks supply». Either alone is not a reason to tell a
	// patient to order more.
	open := vial(4, day(2026, time.May, 1), farOff)
	// A *small* spare, deliberately: a full one would put total supply over four weeks
	// anyway, passing without the spare check ever running.
	spare := vial(1, nil, farOff)

	if got := ReorderHintFor(weeklyItem(sema, 1), []Vial{open, spare}, drawn(open, 3), today); got != nil {
		t.Errorf("a sealed spare is exactly what makes reordering unnecessary: got %v", got)
	}
	if got := ReorderHintFor(weeklyItem(sema, 0), []Vial{open}, nil, today); got != nil {
		t.Errorf("an item prescribed on no day has no weekly rate: got %v", got)
	}

	hint := ReorderHintFor(weeklyItem(sema, 1), []Vial{open}, drawn(open, 1), today)

	if hint == nil {
		t.Fatal("three doses at one a week is three weeks of supply and no spare")
	}
	if hint.WeeksLeft != 3 || hint.CompoundID != sema {
		t.Errorf("got %v, want 3 weeks of %v", hint, sema)
	}
}

func TestAnExpiredVialIsNeitherASpareNorSupply(t *testing.T) {
	// Measured: an expired sealed vial made the spare check true, so a patient two doses
	// from nothing saw no reorder card at all.
	open := vial(4, day(2026, time.May, 1), farOff)
	dead := vial(4, nil, today.AddDays(-1))

	hint := ReorderHintFor(weeklyItem(sema, 1), []Vial{open, dead}, drawn(open, 2), today)

	if hint == nil || hint.WeeksLeft != 2 {
		t.Errorf("the expired vial is not two more weeks of supply: got %v", hint)
	}

	// Boundary: a vial expiring *today* is still usable and still suppresses the hint.
	expiringToday := vial(4, nil, today)
	if got := ReorderHintFor(
		weeklyItem(sema, 1), []Vial{open, expiringToday}, drawn(open, 2), today,
	); got != nil {
		t.Errorf("stock that expires today has not expired yet: got %v", got)
	}
}

func TestADisposedVialIsNeitherASpareNorSupply(t *testing.T) {
	open := vial(4, day(2026, time.May, 1), farOff)
	binned := vial(4, nil, farOff)
	binned.DisposedAt = day(2026, time.May, 2)

	hint := ReorderHintFor(weeklyItem(sema, 1), []Vial{open, binned}, drawn(open, 1), today)

	if hint == nil || hint.WeeksLeft != 3 {
		t.Errorf("got %v, want 3 weeks", hint)
	}
}

func TestFourWeeksOfSupplyIsAHintAndFiveIsNot(t *testing.T) {
	// §03 sets the threshold at «≤4 weeks», and no other fixture lands on it: moving
	// the comparison to five weeks left the whole suite green. The Kotlin suite has
	// the same hole.
	//
	// One dose a week, so a vial's total is its weeks of supply outright.
	for _, c := range []struct {
		doses int
		want  int
		hint  bool
	}{
		{3, 3, true},
		{4, 4, true},
		{5, 0, false},
	} {
		open := vial(c.doses, day(2026, time.May, 1), farOff)

		got := ReorderHintFor(weeklyItem(sema, 1), []Vial{open}, nil, today)

		switch {
		case c.hint && got == nil:
			t.Errorf("%d weeks of supply: got no hint, want one", c.doses)
		case c.hint && got.WeeksLeft != c.want:
			t.Errorf("%d weeks of supply: got %d weeks left, want %d", c.doses, got.WeeksLeft, c.want)
		case !c.hint && got != nil:
			t.Errorf("%d weeks of supply is more than the threshold: got %v", c.doses, got)
		}
	}
}

func TestAnItemWithNoWeeklyRateGetsNoHintEvenWhenTheVialIsEmpty(t *testing.T) {
	// The guard on the rate is load-bearing, and measured: without it the division
	// is by zero. A vial with doses left gives +Inf, which converts to a number
	// larger than the threshold and happens to answer «no hint» by accident — but a
	// drained one gives NaN, which converts to 0, and 0 is not more than four weeks.
	// The patient would be told to reorder a drug prescribed on no day at all, with
	// «0 weeks left» beside it.
	drained := vial(4, day(2026, time.May, 1), farOff)

	if got := ReorderHintFor(weeklyItem(sema, 0), []Vial{drained}, drawn(drained, 4), today); got != nil {
		t.Errorf("an item prescribed on no day: got %v, want no hint", got)
	}
}

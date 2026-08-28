package inventory

import (
	"fmt"
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

var (
	today   = civil.NewDate(2026, time.May, 31)
	patient = civil.UserID("p-1")
	sema    = protocol.CompoundID("sema")
	nextID  int
)

// step is the dose every case in this file is written against: the cabinet counts
// substance, and a case that says «eight doses» means eight of these.
var step = protocol.Dose{Value: 0.25, Unit: protocol.MG}

func vial(doses int, openedAt *civil.Date, expiresOn civil.Date) Vial {
	nextID++

	return Vial{
		ID:                 VialID(fmt.Sprintf("v-%d", nextID)),
		PatientID:          patient,
		CompoundID:         sema,
		ConcentrationLabel: "1 мг/мл",
		TotalAmount:        Amount(doses) * 250,
		AmountUnit:         protocol.MG,
		OpenedAt:           openedAt,
		ExpiresOn:          expiresOn,
	}
}

// dosesLeft is RemainingDoses at the standard step, dereferenced. Every case here
// prescribes a dose, so a nil is a broken fixture rather than a case.
func dosesLeft(t *testing.T, v Vial, draws []Draw) int {
	t.Helper()

	left := RemainingDoses(v, draws, &step)
	if left == nil {
		t.Fatal("the fixture prescribes a dose and RemainingDoses answered nothing")
	}

	return *left
}

func day(year int, month time.Month, d int) *civil.Date {
	date := civil.NewDate(year, month, d)

	return &date
}

var farOff = civil.NewDate(2026, time.December, 31)

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
		Times:      []civil.Slot{{Hour: 7}},
		Loggable:   true,
	}
}

// drawn is the narrow shape RemainingDoses takes: which vial each logged dose came
// out of, and nothing else about it.
func drawn(v Vial, times int) []Draw {
	out := make([]Draw, times)
	for i := range out {
		out[i] = Draw{VialID: v.ID, Amount: 250}
	}

	return out
}

func TestLoggingADoseDecrementsTheVialItCameFrom(t *testing.T) {
	v := vial(4, nil, farOff)

	for _, c := range []struct{ logged, remaining int }{{0, 4}, {2, 2}, {4, 0}} {
		if got := dosesLeft(t, v, drawn(v, c.logged)); got != c.remaining {
			t.Errorf("%d logged: want %d remaining, got %d", c.logged, c.remaining, got)
		}
	}
}

func TestDosesFromOtherVialsDoNotCount(t *testing.T) {
	// A subtraction ignoring the vial would drain both at once.
	mine := vial(4, nil, farOff)
	other := vial(4, nil, farOff)

	if got := dosesLeft(t, mine, drawn(other, 3)); got != 4 {
		t.Errorf("another vial's doses counted: got %d, want 4", got)
	}
}

func TestRemainingNeverGoesNegative(t *testing.T) {
	v := vial(2, nil, farOff)

	if got := dosesLeft(t, v, drawn(v, 5)); got != 0 {
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
	soon := vial(8, day(2026, time.May, 1), civil.NewDate(2026, time.June, 10))
	later := vial(8, day(2026, time.May, 1), civil.NewDate(2026, time.July, 20))

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
	v := vial(4, nil, civil.NewDate(2026, time.June, 3))

	if got := StatusOf(v, nil, today); got != StatusExpiring {
		t.Errorf("got %v, want expiring", got)
	}
}

func TestAHintIsAboutOneCompoundAndCountsOnlyItsVials(t *testing.T) {
	// Measured: an unopened BPC vial once suppressed the semaglutide hint entirely.
	mine := vial(4, day(2026, time.May, 1), farOff)
	bpcSpare := vial(30, nil, farOff)
	bpcSpare.CompoundID = protocol.CompoundID("bpc")

	alone := ReorderHintFor(weeklyItem(sema, 1), CabinetOf(patient, []Vial{mine}), nil, &step, today)
	withOtherCompound := ReorderHintFor(weeklyItem(sema, 1), CabinetOf(patient, []Vial{mine, bpcSpare}), nil, &step, today)

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

	if got := ReorderHintFor(weeklyItem(sema, 1), CabinetOf(patient, []Vial{open, spare}), drawn(open, 3), &step, today); got != nil {
		t.Errorf("a sealed spare is exactly what makes reordering unnecessary: got %v", got)
	}
	if got := ReorderHintFor(weeklyItem(sema, 0), CabinetOf(patient, []Vial{open}), nil, &step, today); got != nil {
		t.Errorf("an item prescribed on no day has no weekly rate: got %v", got)
	}

	hint := ReorderHintFor(weeklyItem(sema, 1), CabinetOf(patient, []Vial{open}), drawn(open, 1), &step, today)

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

	hint := ReorderHintFor(weeklyItem(sema, 1), CabinetOf(patient, []Vial{open, dead}), drawn(open, 2), &step, today)

	if hint == nil || hint.WeeksLeft != 2 {
		t.Errorf("the expired vial is not two more weeks of supply: got %v", hint)
	}

	// Boundary: a vial expiring *today* is still usable and still suppresses the hint.
	expiringToday := vial(4, nil, today)
	if got := ReorderHintFor(
		weeklyItem(sema, 1), CabinetOf(patient, []Vial{open, expiringToday}), drawn(open, 2), &step, today,
	); got != nil {
		t.Errorf("stock that expires today has not expired yet: got %v", got)
	}
}

func TestADisposedVialIsNeitherASpareNorSupply(t *testing.T) {
	open := vial(4, day(2026, time.May, 1), farOff)
	binned := vial(4, nil, farOff)
	binned.DisposedAt = day(2026, time.May, 2)

	hint := ReorderHintFor(weeklyItem(sema, 1), CabinetOf(patient, []Vial{open, binned}), drawn(open, 1), &step, today)

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

		got := ReorderHintFor(weeklyItem(sema, 1), CabinetOf(patient, []Vial{open}), nil, &step, today)

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
	// The guard on the rate is load-bearing, and it sits before the division rather
	// than after it because Go leaves the conversion undefined: converting ±Inf or
	// NaN to an int is not specified, and the two architectures this project runs on
	// disagree. Measured on both, 2026-08-21:
	//
	//	arm64 (this laptop):   int(4/0) = maxint,  int(0/0) = 0
	//	amd64 (CI, Timeweb):   int(4/0) = minint,  int(0/0) = minint
	//
	// So without the guard, amd64 tells a patient to reorder a drug prescribed on no
	// day at all — for a full vial as readily as an empty one — with a nonsense
	// number of weeks beside it. On arm64 only the drained case shows it. This test
	// uses the drained vial because that is the case both architectures fail.
	drained := vial(4, day(2026, time.May, 1), farOff)

	if got := ReorderHintFor(weeklyItem(sema, 0), CabinetOf(patient, []Vial{drained}), drawn(drained, 4), &step, today); got != nil {
		t.Errorf("an item prescribed on no day: got %v, want no hint", got)
	}
}

func TestAnotherPatientsVialsChangeNothing(t *testing.T) {
	// The axis the fixture never varied: every vial in this file belongs to one
	// patient, so the function would behave identically with PatientID deleted. A
	// doctor-side caller reads several cabinets in one result set, and the database
	// cannot catch the mixing — every row is one they are entitled to.
	other := civil.UserID("p-2")
	mine := vial(4, day(2026, time.May, 1), farOff)

	// Theirs is open and full, deliberately. A sealed one would be caught by the
	// spare check before the sum is ever reached, so the half about inflating the
	// weeks left would never run: unfiltered, thirty-four doses at one a week is
	// thirty-four weeks and no hint at all.
	theirs := vial(30, day(2026, time.May, 1), farOff)
	theirs.PatientID = other

	alone := ReorderHintFor(weeklyItem(sema, 1), CabinetOf(patient, []Vial{mine}), nil, &step, today)
	mixed := ReorderHintFor(
		weeklyItem(sema, 1), CabinetOf(patient, []Vial{mine, theirs}), nil, &step, today,
	)

	if alone == nil || mixed == nil || *alone != *mixed {
		t.Fatalf("another patient's vials changed the answer: %v vs %v", alone, mixed)
	}

	// And their cabinet holds only their own row, with their own number of weeks —
	// different from the mixed answer, so this cannot pass on the same arithmetic.
	if got := ReorderHintFor(
		weeklyItem(sema, 1), CabinetOf(other, []Vial{mine, theirs}), nil, &step, today,
	); got != nil {
		t.Errorf("thirty weeks of supply is no reorder hint: got %v", got)
	}
}

// BG-001, and the whole reason the vial stopped counting injections.
//
// One vial of 2 мг against a course that titrates 0,25 → 0,5 → 1,0. The count model
// stored eight injections and kept answering eight at every dose, so the hint stayed
// silent through the phase where supply actually halved and then halved again. The
// numbers below are the proposal's measured table, and the weeks are asserted rather
// than the presence of a hint: «выдаётся» and «выдаётся with the right number» differ
// by exactly the defect this replaces.
func TestTheReorderHintFollowsTheTitration(t *testing.T) {
	v := vial(8, day(2026, time.May, 1), farOff)
	weekly := weeklyItem(sema, 1)
	cabinet := CabinetOf(patient, []Vial{v})

	for _, phase := range []struct {
		name  string
		dose  protocol.Dose
		weeks *int
	}{
		{"the starting dose", protocol.Dose{Value: 0.25, Unit: protocol.MG}, nil},
		{"after the first step up", protocol.Dose{Value: 0.5, Unit: protocol.MG}, ptr(4)},
		{"at the full dose", protocol.Dose{Value: 1, Unit: protocol.MG}, ptr(2)},
	} {
		t.Run(phase.name, func(t *testing.T) {
			got := ReorderHintFor(weekly, cabinet, nil, &phase.dose, today)

			switch {
			case phase.weeks == nil && got != nil:
				t.Errorf("eight weeks of supply asked for a reorder: %+v", got)
			case phase.weeks != nil && got == nil:
				t.Error("the hint stayed silent past the threshold — this is BG-001")
			case phase.weeks != nil && got.WeeksLeft != *phase.weeks:
				t.Errorf("the hint says %d weeks, want %d", got.WeeksLeft, *phase.weeks)
			}
		})
	}
}

// The division floors, and the boundary is where it shows. Two milligrams at 0,3 is six
// injections and a remainder that buys nothing: rounding up would promise a dose the
// vial cannot give.
func TestARemainderTooSmallForADoseBuysNothing(t *testing.T) {
	v := vial(8, day(2026, time.May, 1), farOff)

	for _, c := range []struct {
		name string
		dose protocol.Dose
		want int
	}{
		{"a dose that divides evenly", protocol.Dose{Value: 0.5, Unit: protocol.MG}, 4},
		{"a dose that leaves a remainder", protocol.Dose{Value: 0.3, Unit: protocol.MG}, 6},
		{"a dose larger than the vial", protocol.Dose{Value: 3, Unit: protocol.MG}, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := dosesLeftAt(t, v, nil, c.dose); got != c.want {
				t.Errorf("2 мг at %v gives %d injections, want %d", c.dose.Value, got, c.want)
			}
		})
	}
}

// A course that has ended prescribes nothing, and a drug the doctor typed has no
// reference dose. Both answer the same way: the substance is known, the number of
// injections is not, and inventing one is what invariant 3 refuses.
func TestWithNoDoseThereIsNoCountOfInjections(t *testing.T) {
	v := vial(8, day(2026, time.May, 1), farOff)

	if got := RemainingDoses(v, nil, nil); got != nil {
		t.Errorf("with no dose the vial claims %d injections", *got)
	}
	if got := RemainingAmount(v, nil); got != 2_000 {
		t.Errorf("the substance is %d мкг, want 2000 — that part is always answerable", got)
	}
	if got := StatusOf(v, nil, today); got != StatusActive {
		t.Errorf("with no dose the status is %v, want active — «low» is a fraction, not a count", got)
	}
}

func ptr(n int) *int { return &n }

func dosesLeftAt(t *testing.T, v Vial, draws []Draw, dose protocol.Dose) int {
	t.Helper()

	left := RemainingDoses(v, draws, &dose)
	if left == nil {
		t.Fatalf("no count at %v %s", dose.Value, dose.Unit)
	}

	return *left
}

// Weeks are not doses, and in every other fixture here they are the same number: the
// cadence is one injection a week, so eight doses read as eight weeks whichever the
// arithmetic returns. Twice a week is where they part.
func TestTheHintCountsWeeksAndNotDoses(t *testing.T) {
	v := vial(8, day(2026, time.May, 1), farOff)
	cabinet := CabinetOf(patient, []Vial{v})
	dose := protocol.Dose{Value: 0.25, Unit: protocol.MG}

	got := ReorderHintFor(weeklyItem(sema, 2), cabinet, nil, &dose, today)
	if got == nil {
		t.Fatal("eight doses at two a week is four weeks, which is inside the threshold")
	}
	if got.WeeksLeft != 4 {
		t.Errorf("the hint says %d, want 4 — eight doses, two a week", got.WeeksLeft)
	}
}

// The quarter is a quarter, not «some fraction». Every other case here divides evenly
// into thousands of micrograms, so the threshold's magnitude is free to drift.
func TestTheLowThresholdIsAQuarterAndNotSomeOtherFraction(t *testing.T) {
	v := Vial{
		ID: "odd", PatientID: patient, CompoundID: sema,
		TotalAmount: 1_000, AmountUnit: protocol.MG,
		OpenedAt: day(2026, time.May, 1), ExpiresOn: farOff,
	}

	// 220 of 1000 is below a quarter; 260 is above. A threshold of a fifth would call
	// the first one active, and one of a third would call the second one low.
	for _, c := range []struct {
		drawn Amount
		want  VialStatus
	}{{780, StatusLow}, {740, StatusActive}} {
		draws := []Draw{{VialID: v.ID, Amount: c.drawn}}
		if got := StatusOf(v, draws, today); got != c.want {
			t.Errorf("%d мкг drawn of 1000 reads %v, want %v", c.drawn, got, c.want)
		}
	}
}

// The microgram dose, which no other case in this file prescribes.
//
// Every fixture here is milligrams, so AmountOf's мкг arm is reached by the conversion
// table and never through the divisor — the one place a wrong arm turns into a number on
// the day card. BPC-157 is the compound it exists for: 250 мкг out of a 5 мг vial.
func TestAMicrogramDoseDividesTheVialItIsPrescribedFrom(t *testing.T) {
	bpc := protocol.CompoundID("bpc")
	v := Vial{
		ID: "v-bpc", PatientID: patient, CompoundID: bpc,
		ConcentrationLabel: "5 мг/мл",
		TotalAmount:        5_000, AmountUnit: protocol.MCG,
		OpenedAt: day(2026, time.May, 1), ExpiresOn: farOff,
	}
	dose := protocol.Dose{Value: 250, Unit: protocol.MCG}

	left := RemainingDoses(v, []Draw{{VialID: v.ID, Amount: 1_000}}, &dose)

	if left == nil {
		t.Fatal("a dose of 250 мкг bought no count at all")
	}
	// 4000 мкг at 250 apiece. Read as milligrams the dose would be 250 000 мкг and the
	// answer none, which is the arm this case is here to hold.
	if *left != 16 {
		t.Errorf("4000 мкг at 250 мкг apiece is %d injections, want 16", *left)
	}
}

// A vial the patient set aside takes no part in the hint, on either half of the rule.
//
// The two halves fail differently, so both are measured: as a sealed spare it would suppress
// the hint outright — the patient is told nothing while the only substance they have is the
// one they deliberately shelved — and as an open vial its contents would be summed into the
// weeks left, so the hint arrives late by however much it holds.
func TestAHeldBackVialNeitherSuppressesTheHintNorFeedsIt(t *testing.T) {
	open := vial(4, day(2026, time.May, 1), farOff)
	shelved := vial(1, nil, farOff)
	shelved.HeldBackAt = day(2026, time.May, 3)

	suppressing := ReorderHintFor(
		weeklyItem(sema, 1), CabinetOf(patient, []Vial{open, shelved}), drawn(open, 1), &step, today,
	)
	if suppressing == nil {
		t.Fatal("a held-back spare suppressed the hint, and it is not supply the patient can use")
	}
	// The same three weeks the case without a spare answers: the shelved vial's own
	// doses must not have been added to them.
	if suppressing.WeeksLeft != 3 {
		t.Errorf("the hint says %d weeks, want 3 — the held-back vial was counted", suppressing.WeeksLeft)
	}

	openShelved := vial(4, day(2026, time.May, 1), farOff)
	openShelved.HeldBackAt = day(2026, time.May, 3)
	fed := ReorderHintFor(
		weeklyItem(sema, 1), CabinetOf(patient, []Vial{open, openShelved}), drawn(open, 1), &step, today,
	)
	if fed == nil {
		t.Fatal("the hint disappeared with a held-back vial in the cabinet")
	}
	if fed.WeeksLeft != 3 {
		t.Errorf("an open held-back vial added %d weeks to the hint", fed.WeeksLeft-3)
	}
}

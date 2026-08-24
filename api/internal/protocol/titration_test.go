package protocol

import (
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

var titrationPlan = Plan{
	Protocol: Protocol{
		ID:        ProtocolID("pr"),
		PatientID: civil.UserID("p"),
		StartDate: civil.NewDate(2026, time.May, 10),
		Weeks:     12,
		Status:    StatusActive,
	},
	Items: []ProtocolItem{
		{
			ID:         sema,
			ProtocolID: ProtocolID("pr"),
			Kind:       KindInjection,
			CompoundID: compoundPtr("sema"),
			Cadence:    CadenceWeekly,
			DaysOfWeek: []time.Weekday{time.Sunday},
			Times:      []civil.Slot{{Hour: 7}},
			Loggable:   true,
		},
	},
	// Deliberately out of order: a sorted fixture lets a missing sort through.
	Phases: map[ProtocolItemID][]ProtocolPhase{
		sema: {
			{FromWeek: 5, ToWeek: 8, Dose: Dose{Value: 0.5, Unit: MG}},
			{FromWeek: 9, ToWeek: 12, Dose: Dose{Value: 1.0, Unit: MG}},
			{FromWeek: 1, ToWeek: 4, Dose: Dose{Value: 0.25, Unit: MG}},
		},
	},
}

func TestTheNextStepIsTheOneAfterTheWeekYouAreIn(t *testing.T) {
	step := TitrationStepAfter(titrationPlan, sema, civil.NewDate(2026, time.May, 31))

	if step == nil {
		t.Fatal("week 4 has a step ahead of it")
	}
	if step.Week != 5 {
		t.Errorf("the week the new band begins: got %d", step.Week)
	}
	if step.From != (Dose{Value: 0.25, Unit: MG}) || step.To != (Dose{Value: 0.5, Unit: MG}) {
		t.Errorf("0,25 → 0,5: got %v → %v", step.From, step.To)
	}
	// Week 5 begins seven days after week 4's Sunday.
	if step.Date != civil.NewDate(2026, time.June, 7) {
		t.Errorf("7 June: got %v", step.Date)
	}
}

func TestTheStepMovesOnOnceItIsPassed(t *testing.T) {
	step := TitrationStepAfter(titrationPlan, sema, civil.NewDate(2026, time.June, 10))

	if step == nil {
		t.Fatal("week 5 still has week 9 ahead of it")
	}
	if step.From != (Dose{Value: 0.5, Unit: MG}) || step.To != (Dose{Value: 1.0, Unit: MG}) {
		t.Errorf("0,5 → 1,0: got %v → %v", step.From, step.To)
	}
	if step.Date != civil.NewDate(2026, time.July, 5) {
		t.Errorf("5 July: got %v", step.Date)
	}
}

func TestTheLastBandHasNoNextStep(t *testing.T) {
	if step := TitrationStepAfter(titrationPlan, sema, civil.NewDate(2026, time.July, 20)); step != nil {
		t.Errorf("nothing after the last band, got %v", step)
	}
}

func TestAnItemWithOnePhaseNeverTitrates(t *testing.T) {
	flat := titrationPlan
	flat.Phases = map[ProtocolItemID][]ProtocolPhase{
		sema: {{FromWeek: 1, ToWeek: 12, Dose: Dose{Value: 250.0, Unit: MCG}}},
	}

	if step := TitrationStepAfter(flat, sema, civil.NewDate(2026, time.May, 31)); step != nil {
		t.Errorf("one band never steps, got %v", step)
	}
}

func TestADateOutsideTheCycleHasNoStep(t *testing.T) {
	if step := TitrationStepAfter(titrationPlan, sema, civil.NewDate(2026, time.May, 9)); step != nil {
		t.Errorf("bounded by the cycle, not the calendar, got %v", step)
	}
}

func TestSortingThePhasesLeavesTheCallersOrderAlone(t *testing.T) {
	// PhaseDose reads the phases as they arrive, so a sort in place here would change what
	// dose another reader of the same plan sees.
	before := append([]ProtocolPhase(nil), titrationPlan.Phases[sema]...)

	TitrationSteps(titrationPlan, sema)

	after := titrationPlan.Phases[sema]
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("phase %d moved: was %v, now %v", i, before[i], after[i])
		}
	}
}

func TestAChangeOfUnitAtTheSameNumberIsStillAStep(t *testing.T) {
	// Measured: weakening the comparison in TitrationSteps to `from.Dose.Value ==
	// to.Dose.Value` left the whole suite green, because no fixture ever varies the unit
	// inside one item's phase list. «250 мкг → 250 мг» is a thousandfold rise that would read as
	// a dose held and lose its mark.
	unitChange := titrationPlan
	unitChange.Phases = map[ProtocolItemID][]ProtocolPhase{
		sema: {
			{FromWeek: 1, ToWeek: 4, Dose: Dose{Value: 250.0, Unit: MCG}},
			{FromWeek: 5, ToWeek: 12, Dose: Dose{Value: 250.0, Unit: MG}},
		},
	}

	steps := TitrationSteps(unitChange, sema)

	if len(steps) != 1 {
		t.Fatalf("one step, got %d: %v", len(steps), steps)
	}
	if steps[0].From.Unit != MCG || steps[0].To.Unit != MG {
		t.Errorf("мкг → мг, got %v → %v", steps[0].From, steps[0].To)
	}
}

// Built at run time from variables, not from literals: Go folds untyped constant arithmetic
// at arbitrary precision, so `0.1 + 0.2` written inline equals `0.3` exactly and would prove
// nothing.
var (
	firstTenth   = 0.1
	secondFifth  = 0.2
	summedThird  = Dose{Value: firstTenth + secondFifth, Unit: MG}
	literalThird = Dose{Value: 0.3, Unit: MG}
)

func TestADoseThatArrivedThroughArithmeticReadsAsAChangeOfDose(t *testing.T) {
	// The invariant the Dose doc names — doses are never summed — is one a later step can
	// break silently, and a comment cannot fail. Two phases both meant to hold 0,3 мг, one
	// value computed rather than parsed: without the invariant this draws «Доза растёт:
	// 0,3 мг → 0,3 мг», a change from a value to itself, rendered identically on both sides.
	// Fatal, not Skip: Go pins float64 to IEEE-754 binary64, so this cannot legitimately
	// happen. A Skip here would turn a drifting fixture into silence — falsify the premise and
	// the suite reports ok, which is the failure mode this whole test exists to refuse.
	if summedThird == literalThird {
		t.Fatal("0.1+0.2 must not equal 0.3 — the hazard this pins is gone, or the fixture drifted")
	}

	drifted := titrationPlan
	drifted.Phases = map[ProtocolItemID][]ProtocolPhase{
		sema: {
			{FromWeek: 1, ToWeek: 4, Dose: summedThird},
			{FromWeek: 5, ToWeek: 12, Dose: literalThird},
		},
	}

	steps := TitrationSteps(drifted, sema)

	// Pinned as it behaves, not as one would wish: == is the right operator while doses only
	// ever arrive parsed, and this fails the day somebody swaps it for a tolerance without
	// deciding that summing is now allowed.
	if len(steps) != 1 {
		t.Fatalf("exact comparison makes two doses that differ a step: got %d", len(steps))
	}
	// Named rather than asserted away: this is the failure the Dose doc warns about, and it
	// is here so that the day someone computes a dose, this test says what changed.
	t.Logf("a computed 0,3 мг and a parsed one differ: %.20f vs %.20f", steps[0].From.Value, steps[0].To.Value)
}

func TestTwoPhasesOpeningOnTheSameWeekKeepTheOrderTheyArrivedIn(t *testing.T) {
	// What this pins is the outcome, and what it does NOT pin is the choice of sort — said
	// plainly because the first version of this test claimed otherwise. Measured against
	// go 1.26: swapping sort.SliceStable for sort.Slice changes nothing here, and nothing at
	// any size probed — all-tied inputs up to 40 elements and paired ties up to 100 both keep
	// their arrival order, because pdqsort has a fast path for equal keys. The stable sort is
	// chosen for the guarantee, not because a test can tell the two apart.
	//
	// Ties are representable here and refused by the schema and by Check, which is the same
	// window PhaseDose's comment names.
	duplicate := titrationPlan
	duplicate.Phases = map[ProtocolItemID][]ProtocolPhase{
		sema: {
			{FromWeek: 1, ToWeek: 4, Dose: Dose{Value: 0.25, Unit: MG}},
			{FromWeek: 5, ToWeek: 8, Dose: Dose{Value: 0.5, Unit: MG}},
			{FromWeek: 5, ToWeek: 8, Dose: Dose{Value: 0.75, Unit: MG}},
		},
	}

	steps := TitrationSteps(duplicate, sema)

	if len(steps) != 2 {
		t.Fatalf("two boundaries, got %d: %v", len(steps), steps)
	}
	// The first of the tied pair is the one that arrived first — 0,5 before 0,75.
	if steps[0].To != (Dose{Value: 0.5, Unit: MG}) || steps[1].From != (Dose{Value: 0.5, Unit: MG}) {
		t.Fatalf("the arrival order of the tied phases moved: %v", steps)
	}
}

package inventory_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/SimonOsipov/cadence-app/api/internal/inventory"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

func TestAQuantityConvertsToWholeMicrograms(t *testing.T) {
	for _, conversion := range []struct {
		name  string
		value float64
		unit  protocol.DoseUnit
		want  inventory.Amount
	}{
		{"a milligram", 1, protocol.MG, 1_000},
		{"a microgram", 1, protocol.MCG, 1},
		{"a titration step", 0.25, protocol.MG, 250},
		{"how BPC-157 comes", 250, protocol.MCG, 250},
		{"a whole vial", 2, protocol.MG, 2_000},
		// The schema's finest milligram value. One microgram, and not zero.
		{"three decimals of a milligram", 0.001, protocol.MG, 1},
		// Measured: 1.005 × 1000 is 1004.9999999999998863 in binary64, so truncation
		// gives 1004 and rounding gives 1005. Most three-decimal values multiply
		// exactly — 0,29 and 0,07 among them — so a case has to be chosen for this
		// rather than assumed, and this is the one the schema's scale bound admits.
		{"a value whose product lands just under", 1.005, protocol.MG, 1_005},
		{"one that multiplies exactly", 0.29, protocol.MG, 290},
	} {
		t.Run(conversion.name, func(t *testing.T) {
			got, err := inventory.AmountOf(conversion.value, conversion.unit)
			if err != nil {
				t.Fatalf("converting %v %s: %v", conversion.value, conversion.unit, err)
			}
			if got != conversion.want {
				t.Errorf("%v %s is %d мкг, want %d", conversion.value, conversion.unit, got, conversion.want)
			}
		})
	}
}

// A dose finer than the atom converts to nothing, and nothing is not a divisor.
//
// The schema refuses such a phase since 000023, and the writer refuses it before that —
// but the arithmetic is the last place it would surface, and there it must answer «no
// count» rather than divide by zero. Measured: without the guard this panics.
func TestADoseThatConvertsToNothingIsNotADivisor(t *testing.T) {
	v := inventory.Vial{ID: "v", TotalAmount: 2_000, AmountUnit: protocol.MG}

	tooFine := protocol.Dose{Value: 0.0001, Unit: protocol.MG}
	if got, err := inventory.AmountOf(tooFine.Value, tooFine.Unit); err != nil || got != 0 {
		t.Fatalf("0,0001 мг is %d мкг (err %v); the case this pins needs it to be zero", got, err)
	}
	if got := inventory.RemainingDoses(v, nil, &tooFine); got != nil {
		t.Errorf("a dose of nothing bought %d injections", *got)
	}
}

// The sum is why this type exists: protocol.Dose says a float64 is safe only while
// doses are never added, and a vial's remainder now adds them.
func TestTwelveTitrationStepsAreExactlyThreeMilligrams(t *testing.T) {
	step, err := inventory.AmountOf(0.25, protocol.MG)
	if err != nil {
		t.Fatalf("converting the step: %v", err)
	}

	var summed inventory.Amount
	for range 12 {
		summed += step
	}

	whole, err := inventory.AmountOf(3, protocol.MG)
	if err != nil {
		t.Fatalf("converting the whole: %v", err)
	}
	if summed != whole {
		t.Errorf("twelve steps are %d мкг, three milligrams are %d", summed, whole)
	}

	// And the same sum in the type doses are stored in, so the test names the hazard
	// rather than asserting a bare number. Not 0,25 — it is a binary fraction and twelve
	// of them come to exactly 3,0, which is why the first version of this block skipped
	// on every platform. Measured: twelve of 0,07 do not come to 0,84.
	var drifted float64
	for range 12 {
		drifted += 0.07
	}
	if drifted == 0.84 {
		t.Fatal("twelve sevenths of a milligram summed exactly in float64 — " +
			"the hazard inventory.Amount exists for is gone, or this fixture drifted")
	}

	steps, err := inventory.AmountOf(0.07, protocol.MG)
	if err != nil {
		t.Fatalf("converting: %v", err)
	}
	var exact inventory.Amount
	for range 12 {
		exact += steps
	}
	if want, _ := inventory.AmountOf(0.84, protocol.MG); exact != want {
		t.Errorf("twelve of 0,07 мг are %d мкг, want %d", exact, want)
	}
}

// A unit the cabinet cannot count in is refused rather than silently taken as one of
// the two: the closed set lives in protocol, and a third member added there must
// reach a decision here rather than a zero.
func TestAUnitWithNoAtomIsRefused(t *testing.T) {
	if _, err := inventory.AmountOf(1, protocol.DoseUnit("ед")); !errors.Is(err, inventory.ErrUnitHasNoAtom) {
		t.Errorf("an unknown unit gave %v, want ErrUnitHasNoAtom", err)
	}

	for _, unit := range protocol.DoseUnits() {
		if _, err := inventory.AmountOf(1, unit); err != nil {
			t.Errorf("%s is a prescribed unit and the cabinet refused it: %v", unit, err)
		}
	}
}

// The conversion back into the unit the clinic wrote, which is what the wire carries.
//
// Both arms, because only the milligram one is reached by any fixture in this package: BPC-157
// comes in micrograms, and swapping the arms renders its 250 мкг vial as 0,25 мкг on the
// patient's own cabinet — a thousandfold error on the one number that screen is for.
//
// The claim is the round trip and not the quotient: 290 мкг is 0,28999999999999998 in binary64
// and 1 мкг is 0,001, neither exact, both correctly rounded and both recovered by AmountOf.
func TestAQuantityComesBackInTheUnitItWasWrittenIn(t *testing.T) {
	for _, carried := range []struct {
		amount inventory.Amount
		unit   protocol.DoseUnit
		want   float64
	}{
		{2_000, protocol.MG, 2},
		{250, protocol.MG, 0.25},
		{1, protocol.MG, 0.001},
		{290, protocol.MG, 0.29},
		{250, protocol.MCG, 250},
		{1, protocol.MCG, 1},
		// The ceilings 000024 holds, in both units.
		{100_000_000, protocol.MG, 100_000},
		{100_000_000, protocol.MCG, 100_000_000},
	} {
		t.Run(fmt.Sprintf("%d мкг as %s", carried.amount, carried.unit), func(t *testing.T) {
			got := inventory.AmountIn(carried.amount, carried.unit)
			if got != carried.want {
				t.Errorf("%d мкг reads %v %s, want %v", carried.amount, got, carried.unit, carried.want)
			}
			// And back, which is the property the wire depends on: the number the
			// patient sees converts to the micrograms the cabinet subtracts.
			back, err := inventory.AmountOf(got, carried.unit)
			if err != nil || back != carried.amount {
				t.Errorf("%v %s converts back to %d мкг (err %v), want %d",
					got, carried.unit, back, err, carried.amount)
			}
		})
	}
}

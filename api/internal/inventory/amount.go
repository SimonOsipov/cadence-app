package inventory

import (
	"errors"
	"math"

	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// ErrUnitHasNoAtom is a unit the cabinet cannot count in.
var ErrUnitHasNoAtom = errors.New("a quantity is counted in micrograms, and this unit is neither мг nor мкг")

// Amount is a quantity of substance in whole micrograms.
//
// Integer, and that is the whole design. protocol.Dose carries a float64 and says in
// its own doc that this is safe only because doses are never summed — a vial's
// remainder is now a subtraction of quantities, so the sum lives here instead, in a
// type where twelve doses of 0,25 мг are exactly 3 мг rather than nearly.
//
// A microgram is the atom because it is the finest thing the clinic prescribes. All
// three operands are bounded to it in the schema: a vial's amount and a logged dose by
// 000021, and a prescribed phase dose — the divisor — by 000023.
type Amount int64

const microgramsPerMilligram = 1000

// AmountOf converts a stored quantity and its unit to micrograms.
//
// math.Round, not a conversion: Go truncates toward zero, and measured, 1.005 × 1000 is
// 1004.9999999999998863 in binary64 — truncation there is a microgram lost on every
// read. Most three-decimal values multiply exactly, 0,29 among them, so the case has to
// be chosen rather than assumed.
//
// Magnitude is not bounded here and the schema bounds only scale: truncation of a value
// out of range saturates on arm64 and wraps on amd64, so an absurd amount would answer
// differently by machine. Unreachable while nothing writes the column — the endpoint of
// step 6 is where the bound belongs.
func AmountOf(value float64, unit protocol.DoseUnit) (Amount, error) {
	switch unit {
	case protocol.MG:
		return Amount(math.Round(value * microgramsPerMilligram)), nil
	case protocol.MCG:
		return Amount(math.Round(value)), nil
	default:
		return 0, ErrUnitHasNoAtom
	}
}

// AmountOfDose is the same conversion for a prescribed dose.
func AmountOfDose(dose protocol.Dose) (Amount, error) {
	return AmountOf(dose.Value, dose.Unit)
}

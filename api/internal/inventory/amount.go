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
// A microgram is the atom because it is the finest thing the clinic prescribes; the
// schema bounds scale per unit so nothing finer can be stored to be lost here.
type Amount int64

const microgramsPerMilligram = 1000

// AmountOf converts a stored quantity and its unit to micrograms.
//
// math.Round, not a conversion: Go truncates toward zero, so int64(0.29 * 1e6) is
// 289 999. And truncation of a value out of range differs by architecture — on arm64
// it saturates, on amd64 it wraps — so a conversion measured on one machine lies
// about the other.
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

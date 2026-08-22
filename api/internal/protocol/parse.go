package protocol

import "slices"

// The closed sets, each declared once and read by both the parser and the integration test
// that reconciles it against the CHECK the schema carries. Sorted, because that comparison is
// element by element.

func Statuses() []ProtocolStatus {
	return []ProtocolStatus{StatusActive, StatusCancelled, StatusCompleted}
}

func Cadences() []Cadence {
	return []Cadence{CadenceDaily, CadenceNPerWeek, CadenceWeekly}
}

func Kinds() []ItemKind {
	return []ItemKind{KindInjection, KindSupplement, KindWeighIn}
}

func DoseUnits() []DoseUnit {
	return []DoseUnit{MG, MCG}
}

// ParseStatus and its three neighbours are the one seam where a string becomes a value of a
// closed set: the transport of this step and the seed of step 11.
//
// `(T, bool)` and not an error: the caller knows which field it was reading and the name of
// the field is the whole of the message. Returning the zero value on failure would make an
// unparsed status read as the empty string, which compares unequal to every member — safe
// here, and not something to rely on, so the boolean is the answer.
func ParseStatus(s string) (ProtocolStatus, bool) { return parse(s, Statuses()) }

func ParseCadence(s string) (Cadence, bool) { return parse(s, Cadences()) }

func ParseKind(s string) (ItemKind, bool) { return parse(s, Kinds()) }

func ParseDoseUnit(s string) (DoseUnit, bool) { return parse(s, DoseUnits()) }

func parse[T ~string](s string, set []T) (T, bool) {
	if i := slices.Index(set, T(s)); i >= 0 {
		return set[i], true
	}

	var none T

	return none, false
}

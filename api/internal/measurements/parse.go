package measurements

import "slices"

// The two closed sets, each declared once and read by the parser, the meta table and the tests
// that reconcile them against the schema and against the KMP enum.
//
// The metrics are in enumeration order and deliberately not sorted, which is where they differ
// from protocol's sets and journal's: the overview of step 4 answers in it, so it is data.

func Metrics() []Metric {
	return []Metric{
		MetricWeight, MetricHRV, MetricRHR, MetricSleep,
		MetricBodyFat, MetricWaist, MetricHip, MetricChest,
	}
}

func Sources() []Source {
	return []Source{SourceManual, SourceHealthKit, SourceHealthConnect}
}

// ParseMetric and ParseSource are the seam where a string becomes a member of a set: the
// transport of steps 7 and 8, and the seed. `(T, bool)` and not an error — the caller knows
// which field it was reading, and the field's name is the whole of the message.
func ParseMetric(s string) (Metric, bool) { return parse(s, Metrics()) }

func ParseSource(s string) (Source, bool) { return parse(s, Sources()) }

func parse[T ~string](s string, set []T) (T, bool) {
	if i := slices.Index(set, T(s)); i >= 0 {
		return set[i], true
	}

	var none T

	return none, false
}

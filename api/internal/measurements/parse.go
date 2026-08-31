package measurements

import "slices"

// The three closed sets, each declared once and read by the parser and by the tests. The
// metrics and the sources are reconciled there against the schema's CHECKs and the KMP enums;
// the windows are pinned by written-out literals instead, because the codes exist in one place
// only — the frozen prototype — and this is where they become the server's.
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

// In the order the picker draws them, shortest first, which is the order the prototype's two
// trends screens list and the order TrendWindow declares.
func Windows() []Window {
	return []Window{WindowWeek, WindowFourWeeks, WindowThreeMonths, WindowCycle}
}

// The seam where a string becomes a member of a set: the transport, the row scan and the write.
// `(T, bool)` and not an error — the caller knows which field it was reading, and the field's
// name is the whole of the message.
func ParseMetric(s string) (Metric, bool) { return parse(s, Metrics()) }

func ParseSource(s string) (Source, bool) { return parse(s, Sources()) }

func ParseWindow(s string) (Window, bool) { return parse(s, Windows()) }

func parse[T ~string](s string, set []T) (T, bool) {
	if i := slices.Index(set, T(s)); i >= 0 {
		return set[i], true
	}

	var none T

	return none, false
}

// WritableMetrics is what a patient can type in: the eight less `sleep`, which the API derives
// from imported sessions — «a derived score computed by the API … (v1: duration-based formula,
// constants module)», source/architecture-overview-v1.1.md:182, so the formula lands in this
// module when the importer does. There is no number on a watch face to read off and enter.
//
// Deliberately not the frozen prototype's five. `mobile/src/features/body/data.ts` marks
// weight, bodyfat, waist, hip and chest `editable`, and that is the Body screen's add sheet
// rather than this API's set: an HRV and a resting pulse are numbers a watch displays.
func WritableMetrics() []Metric {
	return []Metric{
		MetricWeight, MetricHRV, MetricRHR,
		MetricBodyFat, MetricWaist, MetricHip, MetricChest,
	}
}

// writable is the guard below the schema's enum, for a caller that reaches Record without one.
func writable(m Metric) bool { return slices.Contains(WritableMetrics(), m) }

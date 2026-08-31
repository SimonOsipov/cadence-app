package measurements

// Meta is the table itself, MetricMeta a row of it. Three of the eight carry a bound and five
// carry none — absent, and not zero.
//
// The switch carries no default, so a ninth metric answers false instead of inheriting its
// neighbour's unit. It is a wrong answer and not a build error: the compiler does not check a
// switch for exhaustiveness, and the `exhaustive` linter that would is not enabled in this
// repository — a separate decision, not one to make as a side effect of this step. Until it
// is, TestEveryMetricCarriesItsMeta is what closes the gap.
//
// The three bounds are the architecture overview's, written «HRV ≥58 · RHR ≤60 · Sleep ≥75»
// (source/architecture-overview-v1.1.md, the sentence that asks for this module). Each
// comparator agrees with the metric's own direction, so the bound travels as a number and
// Direction says which side of it is well.
func Meta(m Metric) (MetricMeta, bool) {
	switch m {
	case MetricWeight:
		return MetricMeta{Unit: "kg", Direction: DirectionDown}, true
	case MetricHRV:
		return MetricMeta{Unit: "ms", Direction: DirectionUp, Threshold: &Threshold{Value: 58}}, true
	case MetricRHR:
		return MetricMeta{Unit: "bpm", Direction: DirectionDown, Threshold: &Threshold{Value: 60}}, true
	case MetricSleep:
		return MetricMeta{Unit: "/100", Direction: DirectionUp, Threshold: &Threshold{Value: 75}}, true
	case MetricBodyFat:
		return MetricMeta{Unit: "%", Direction: DirectionDown}, true
	case MetricWaist:
		return MetricMeta{Unit: "cm", Direction: DirectionDown}, true
	case MetricHip:
		return MetricMeta{Unit: "cm", Direction: DirectionDown}, true
	case MetricChest:
		return MetricMeta{Unit: "cm", Direction: DirectionDown}, true
	}

	return MetricMeta{}, false
}

// Bounds is what a reading of the metric may plausibly be, or false for a metric off the set.
//
// 000025 constrains `value` to being finite and nothing more, and names this module as where
// the per-metric range belongs, so this is the second half of that decision rather than a
// second definition of it.
//
// The numbers have no canonical source: neither §03 nor the architecture overview gives one.
// The frozen prototype's `min`/`max` (mobile/src/features/body/data.ts) are not it — read at
// their call site they are the travel of a stepper in the add sheet, cut for the one seeded
// patient that screen draws, and its weight range of 80–140 would refuse a patient of 70. So
// these are chosen here, wide enough that no living patient is refused and narrow enough that
// a slipped decimal point is. Sleep's are the scale its own unit names.
func Bounds(m Metric) (Bound, bool) {
	switch m {
	case MetricWeight:
		return Bound{Low: 20, High: 400}, true
	case MetricHRV:
		return Bound{Low: 1, High: 400}, true
	case MetricRHR:
		return Bound{Low: 20, High: 250}, true
	case MetricSleep:
		return Bound{Low: 0, High: 100}, true
	case MetricBodyFat:
		return Bound{Low: 1, High: 75}, true
	case MetricWaist:
		return Bound{Low: 30, High: 250}, true
	case MetricHip:
		return Bound{Low: 30, High: 250}, true
	case MetricChest:
		return Bound{Low: 30, High: 250}, true
	}

	return Bound{}, false
}

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

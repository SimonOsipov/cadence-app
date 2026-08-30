package measurements

import (
	"slices"
	"testing"
)

// The switch carries no default, so a ninth metric added to the set answers false here rather
// than falling through to somebody else's unit. Go will not refuse to build it, which is why
// this test and not the compiler is what says so.
func TestEveryMetricCarriesItsMeta(t *testing.T) {
	for _, metric := range Metrics() {
		meta, ok := Meta(metric)
		if !ok {
			t.Errorf("Meta(%q) has no row", metric)

			continue
		}
		if meta.Unit == "" {
			t.Errorf("Meta(%q) carries no unit", metric)
		}
		if meta.Direction != DirectionUp && meta.Direction != DirectionDown {
			t.Errorf("Meta(%q) points %q", metric, meta.Direction)
		}
	}

	if _, ok := Meta("thigh"); ok {
		t.Error("Meta answered for a metric off the set")
	}
}

// Named metrics on both sides, not a count: «three carry one» is satisfied by three wrong
// ones, and the five without a threshold have to be measured for absence rather than for a
// zero — a zero HRV bound reads as «any reading clears it».
func TestExactlyThreeMetricsCarryAThreshold(t *testing.T) {
	var carried []Metric
	for _, metric := range Metrics() {
		meta, ok := Meta(metric)
		if !ok {
			t.Fatalf("Meta(%q) has no row", metric)
		}
		if meta.Threshold != nil {
			carried = append(carried, metric)
		}
	}

	if !slices.Equal(carried, []Metric{MetricHRV, MetricRHR, MetricSleep}) {
		t.Errorf("the thresholds sit on %v", carried)
	}
	for _, metric := range []Metric{
		MetricWeight, MetricBodyFat, MetricWaist, MetricHip, MetricChest,
	} {
		if meta, _ := Meta(metric); meta.Threshold != nil {
			t.Errorf("%q carries a threshold of %v", metric, meta.Threshold.Value)
		}
	}
}

// The three canonical numbers and their comparators, written out rather than read off the
// constants they check: an expectation derived from the thing under test moves with it. Where
// the numbers come from is recorded beside them, in meta.go.
func TestTheThresholdsAreTheCanonicalNumbers(t *testing.T) {
	for _, want := range []struct {
		metric    Metric
		value     float64
		direction Direction
	}{
		{MetricHRV, 58, DirectionUp},
		{MetricRHR, 60, DirectionDown},
		{MetricSleep, 75, DirectionUp},
	} {
		t.Run(string(want.metric), func(t *testing.T) {
			meta, ok := Meta(want.metric)
			if !ok || meta.Threshold == nil {
				t.Fatalf("Meta(%q) carries no threshold", want.metric)
			}
			if meta.Threshold.Value != want.value {
				t.Errorf("the bound is %v, canonically %v", meta.Threshold.Value, want.value)
			}
			if meta.Direction != want.direction {
				t.Errorf("it points %q, canonically %q", meta.Direction, want.direction)
			}
		})
	}
}

// Direction is a clinical fact, and every one of the eight is «lower is better» except HRV and
// sleep. Six carry it in the frozen trends prototype; hip and chest are absent from that module
// and the body screen that does draw them has no direction field, so theirs is inferred from
// waist — as MetricMeta.kt does, corroborating chest against that screen's seeded 112 → 105.
// Step 10 records the inference as a deviation.
func TestEachMetricPointsTheWayItsSourceDoes(t *testing.T) {
	for metric, want := range map[Metric]Direction{
		MetricWeight:  DirectionDown,
		MetricHRV:     DirectionUp,
		MetricRHR:     DirectionDown,
		MetricSleep:   DirectionUp,
		MetricBodyFat: DirectionDown,
		MetricWaist:   DirectionDown,
		MetricHip:     DirectionDown,
		MetricChest:   DirectionDown,
	} {
		if meta, _ := Meta(metric); meta.Direction != want {
			t.Errorf("%q points %q, not %q", metric, meta.Direction, want)
		}
	}
}

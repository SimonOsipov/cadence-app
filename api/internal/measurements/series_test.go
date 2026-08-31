package measurements

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

func at(day, hour int) time.Time {
	return time.Date(2026, time.May, day, hour, 0, 0, 0, time.UTC)
}

// One variable and not a call per reading: time.FixedZone hands back a fresh *Location
// every time, and two instants built through two calls compare unequal under == although
// they are the same moment.
var plusThree = time.FixedZone("+03", 3*60*60)

// The instant a reading imported from a device three hours east would carry: 09:00 there is
// 06:00 UTC.
func atPlusThree(day, hour int) time.Time {
	return time.Date(2026, time.May, day, hour, 0, 0, 0, plusThree)
}

// Four shapes in one fixture, because one shape hides a whole axis: three readings on one
// instant («a», «d» and «e»), one of them recorded in a zone three hours east and none of
// them ordered by value the way its id is; two readings on one day («y» and «x»); and
// outside that instant every id runs against the clock, in both metrics and within the
// shared day too. Ids ascending with the clock anywhere would let a comparator that never
// looks at the clock answer correctly and never fail — «x» after «y» is what refuses one
// that orders by calendar day and splits the day by id.
func trendRows() []Reading {
	return []Reading{
		{ID: "e", Metric: MetricWeight, Value: 80.1, MeasuredAt: at(5, 6)},
		{ID: "d", Metric: MetricWeight, Value: 80.7, MeasuredAt: at(5, 6)},
		{ID: "a", Metric: MetricWeight, Value: 80.4, MeasuredAt: atPlusThree(5, 9)},
		{ID: "n", Metric: MetricHRV, Value: 61, MeasuredAt: at(4, 5)},
		{ID: "x", Metric: MetricWeight, Value: 81.0, MeasuredAt: at(3, 21)},
		{ID: "y", Metric: MetricWeight, Value: 81.5, MeasuredAt: at(3, 7)},
		{ID: "p", Metric: MetricHRV, Value: 55, MeasuredAt: at(2, 5)},
		{ID: "z", Metric: MetricWeight, Value: 82.0, MeasuredAt: at(1, 8)},
	}
}

func reversed(readings []Reading) []Reading {
	back := slices.Clone(readings)
	slices.Reverse(back)

	return back
}

func ids(points []Point) []ReadingID {
	got := make([]ReadingID, 0, len(points))
	for _, point := range points {
		got = append(got, point.ID)
	}

	return got
}

// Both arrival orders, and each kills a mutant the other survives — measured, not assumed.
// Newest-first is what refuses a comparator with no id tie-break: reversed survives that
// one, because at n≤12 slices.SortFunc is an insertion sort (maxInsertion, GOROOT/src/
// slices/zsortanyfunc.go) and leaves the tied pair in the order it arrived. Reversed is
// what refuses «hand the rows back turned around», which newest-first cannot see, the read
// being `measured_at DESC`.
func TestThePointOrderDoesNotDependOnTheOrderTheRowsArrivedIn(t *testing.T) {
	arrivals := map[string][]Reading{
		"as they arrived": trendRows(),
		"reversed":        reversed(trendRows()),
	}
	// Written out rather than derived from the fixture: ordering by id would answer «a»
	// first, where the clock answers «z». The three sharing an instant are «a», «d», «e»,
	// the first of them written in a zone three hours ahead, where the same moment reads
	// as a later wall clock.
	want := map[Metric][]ReadingID{
		MetricWeight: {"z", "y", "x", "a", "d", "e"},
		MetricHRV:    {"p", "n"},
	}

	for arrival, readings := range arrivals {
		for metric, order := range want {
			t.Run(arrival+"/"+string(metric), func(t *testing.T) {
				got := ids(SeriesOf(metric, readings).Points)
				if !slices.Equal(got, order) {
					t.Errorf("the points arrived as %v, want %v", got, order)
				}
			})
		}
	}
}

// The values travel with the ids they were read under: a series that carried the right
// order and somebody else's numbers would pass the test above.
func TestAPointCarriesItsOwnReadingsValueAndInstant(t *testing.T) {
	points := SeriesOf(MetricWeight, trendRows()).Points

	// The whole point, location included: a series that normalised every instant to UTC
	// would answer the same moments and lose the zone the reading was taken in.
	want := []Point{
		{ID: "z", Value: 82.0, MeasuredAt: at(1, 8)},
		{ID: "y", Value: 81.5, MeasuredAt: at(3, 7)},
		{ID: "x", Value: 81.0, MeasuredAt: at(3, 21)},
		{ID: "a", Value: 80.4, MeasuredAt: atPlusThree(5, 9)},
		{ID: "d", Value: 80.7, MeasuredAt: at(5, 6)},
		{ID: "e", Value: 80.1, MeasuredAt: at(5, 6)},
	}
	if !slices.Equal(points, want) {
		t.Errorf("the series is %v, want %v", points, want)
	}
}

// The rows are one read shared by eight series, so a sort in place would leave the second
// metric reading a slice the first one rearranged.
func TestBuildingTheSeriesLeavesTheRowsAsTheyArrived(t *testing.T) {
	readings := trendRows()
	before := slices.Clone(readings)

	Overview(readings)

	if !slices.Equal(readings, before) {
		t.Errorf("the rows were rearranged: %v, want %v", readings, before)
	}
}

// The set, in enumeration order, written out — a mutant answering an empty list is as
// well-formed as this one, and a counted assertion admits an invented metric.
func TestTheOverviewCarriesEveryMetricInEnumerationOrder(t *testing.T) {
	var got []Metric
	for _, series := range Overview(trendRows()) {
		got = append(got, series.Metric)
	}

	want := []Metric{"weight", "hrv", "rhr", "sleep", "bodyfat", "waist", "hip", "chest"}
	if !slices.Equal(got, want) {
		t.Fatalf("the overview carries %v, want %v", got, want)
	}
}

// The overview is not eight empty axes: the measured metrics keep their points where the
// unmeasured ones keep none.
func TestTheOverviewKeepsThePointsOfTheMetricsThatWereMeasured(t *testing.T) {
	want := map[Metric][]ReadingID{
		MetricWeight:  {"z", "y", "x", "a", "d", "e"},
		MetricHRV:     {"p", "n"},
		MetricRHR:     {},
		MetricSleep:   {},
		MetricBodyFat: {},
		MetricWaist:   {},
		MetricHip:     {},
		MetricChest:   {},
	}

	walked := 0
	for _, series := range Overview(trendRows()) {
		walked++
		got := ids(series.Points)
		if !slices.Equal(got, want[series.Metric]) {
			t.Errorf("%s carries %v, want %v", series.Metric, got, want[series.Metric])
		}
	}
	// An overview that answered nothing at all would satisfy the loop above, the way the
	// package's other guards would pass an empty directory.
	if walked != len(want) {
		t.Errorf("walked %d series, the overview owes %d", walked, len(want))
	}
}

// Non-nil and not merely empty: a nil slice renders as `null` on the wire, and «нет данных»
// arriving as null instead of [] is a client crash rather than an empty chart.
func TestAMetricWithNoReadingsCarriesAnEmptyListAndNotNil(t *testing.T) {
	walked := 0
	for _, series := range Overview(nil) {
		walked++
		if series.Points == nil {
			t.Errorf("%s carries a nil list of points", series.Metric)
		}
		if len(series.Points) != 0 {
			t.Errorf("%s carries %d points off no readings", series.Metric, len(series.Points))
		}
	}
	// Eight written out: Metrics() is what Overview walks, so a count taken from it would
	// move together with the mutation it is supposed to refuse.
	if walked != 8 {
		t.Errorf("walked %d series, the overview owes 8", walked)
	}
}

// Base, latest, delta, average, minimum, maximum and movement are the client's arithmetic
// over the points — KMP computes all seven as getters over this same list. A second
// definition of the same truth is the divergence class this project has paid for twice, so
// the field set is pinned rather than trusted.
func TestNothingDerivedTravelsOnASeries(t *testing.T) {
	for _, shape := range []struct {
		of    any
		field []string
	}{
		{of: Series{}, field: []string{"Metric", "Points"}},
		{of: Point{}, field: []string{"ID", "Value", "MeasuredAt"}},
	} {
		typ := reflect.TypeOf(shape.of)

		var got []string
		for i := range typ.NumField() {
			got = append(got, typ.Field(i).Name)
		}
		if !slices.Equal(got, shape.field) {
			t.Errorf("%s carries %v, want %v", typ.Name(), got, shape.field)
		}
	}
}

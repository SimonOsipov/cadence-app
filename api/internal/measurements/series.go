package measurements

import (
	"slices"
	"strings"
	"time"
)

// ReadingID is the row's own id. It travels to the client because the row a patient wants
// deleted is the point they are looking at.
type ReadingID string

// Reading is a row of `measurements` as the arithmetic needs it: no unit, which belongs to
// the metric, and no patient, which belongs to the request.
type Reading struct {
	ID         ReadingID
	Metric     Metric
	Value      float64
	MeasuredAt time.Time
}

// Point is one reading on one metric's axis, with nothing derived on it — base, latest,
// delta, average, minimum, maximum and movement are the client's arithmetic over the list.
type Point struct {
	ID         ReadingID
	Value      float64
	MeasuredAt time.Time
}

// Series is one metric's points, oldest first.
type Series struct {
	Metric Metric
	Points []Point
}

// SeriesOf is the readings of one metric, ordered by the clock and never by the order the
// database handed them over: the index is `measured_at DESC` and no read declares an
// ordering the client can rely on. Rows of other metrics are dropped — the overview reads
// one window's rows once and splits them here.
func SeriesOf(metric Metric, readings []Reading) Series {
	points := []Point{}
	for _, reading := range readings {
		if reading.Metric != metric {
			continue
		}
		points = append(points, Point{ID: reading.ID, Value: reading.Value, MeasuredAt: reading.MeasuredAt})
	}
	// The copy above is what keeps this from rearranging the caller's rows: eight series
	// come off one slice, and a sort in place would hand the second metric a shuffled read.
	slices.SortFunc(points, comparePoints)

	return Series{Metric: metric, Points: points}
}

// Overview is every metric in enumeration order, the unmeasured ones included: dropping one
// would leave a patient unable to find out that it is unmeasured.
func Overview(readings []Reading) []Series {
	metrics := Metrics()

	series := make([]Series, 0, len(metrics))
	for _, metric := range metrics {
		series = append(series, SeriesOf(metric, readings))
	}

	return series
}

// Total, because the last rung is the primary key: two readings can share an instant — a
// watch writes a batch on one timestamp — and «whichever the database listed first» is not
// an order two reads of the same window agree on.
func comparePoints(a, b Point) int {
	// Before both ways round and never ==: two instants can be the same moment in two
	// zones, and == on a time.Time compares the wall clock, the monotonic reading and the
	// location instead of the moment.
	if a.MeasuredAt.Before(b.MeasuredAt) {
		return -1
	}
	if b.MeasuredAt.Before(a.MeasuredAt) {
		return 1
	}

	return strings.Compare(string(a.ID), string(b.ID))
}

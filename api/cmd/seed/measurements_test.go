package main

import (
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/measurements"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// A history written out by hand is a history with a typo in it, and the write path refuses one
// row into the run with everything before it already written: a metric no patient types, a
// value a decimal point slipped in, a clock the column has no hour for.
func TestTheSeededReadingsAreOnesThePatientCouldHaveTyped(t *testing.T) {
	for _, reading := range theReadings() {
		if !slices.Contains(measurements.WritableMetrics(), reading.metric) {
			t.Errorf("a %s is seeded, and no patient records that metric by hand", reading.metric)

			continue
		}

		bound, bounded := measurements.Bounds(reading.metric)
		if !bounded {
			t.Errorf("%s carries no interval to check %v against", reading.metric, reading.value)

			continue
		}
		if !bound.Contains(reading.value) {
			t.Errorf("a %s of %v is outside the %v..%v the write path admits",
				reading.metric, reading.value, bound.Low, bound.High)
		}
		if !reading.at.Valid() {
			t.Errorf("a %s is stamped at %v, which is not a time of day", reading.metric, reading.at)
		}
		if reading.daysBack < 0 {
			t.Errorf("a %s is measured %d days back, which is a day that has not happened",
				reading.metric, reading.daysBack)
		}
	}
}

// seededMetrics is every metric the fixture records, which is what the set below pins.
func seededMetrics() []measurements.Metric {
	seeded := map[measurements.Metric]int{}
	for _, reading := range theReadings() {
		seeded[reading.metric]++
	}

	return slices.Sorted(maps.Keys(seeded))
}

// The set and not its size: a fixture that lost the hip and gained the chest is exactly as
// long. Chest is the deliberate absence — the screen has to be able to say «не измерялось» —
// and sleep is the impossible one, a score the API derives from imported sessions.
func TestTheSeededMetricsAreTheSixTheScreensDraw(t *testing.T) {
	want := []measurements.Metric{
		measurements.MetricWeight, measurements.MetricHRV, measurements.MetricRHR,
		measurements.MetricBodyFat, measurements.MetricWaist, measurements.MetricHip,
	}
	slices.Sort(want)

	if got := seededMetrics(); !slices.Equal(got, want) {
		t.Errorf("the seed records %v, want %v", got, want)
	}
}

// The helper the tape series are built with: the ends are the samples' own, nothing between them
// leaves the interval its neighbours set, and a falling series does not rise anywhere.
func TestStretchingASeriesKeepsItsEndsAndItsDirection(t *testing.T) {
	samples := []float64{44.0, 42.6, 41.4, 40.5}

	over := stretched(samples, tapeWeeks)
	if len(over) != tapeWeeks {
		t.Fatalf("four samples stretched to %d readings, want %d", len(over), tapeWeeks)
	}
	if over[0] != samples[0] || over[len(over)-1] != samples[len(samples)-1] {
		t.Errorf("it runs %v → %v, want %v → %v",
			over[0], over[len(over)-1], samples[0], samples[len(samples)-1])
	}
	for i, value := range over[1:] {
		if value > over[i] {
			t.Errorf("reading %d rises to %v from %v, and the samples only fall", i+1, value, over[i])
		}
	}

	// The two samples between the ends are what the Body screen was taken for: a line drawn from
	// the first reading to the last passes them by, and this one does not.
	if straight := stretched([]float64{samples[0], samples[len(samples)-1]}, tapeWeeks); slices.Equal(over, straight) {
		t.Error("the stretched series is the straight line between its ends, and the samples between them are lost")
	}
}

// The head property of the pass without a database: every window the picker offers reaches a
// reading of every metric the seed records. The live read over real rows is what the step asks
// for and it is behind Docker; this is the same claim as arithmetic, which is all three of the
// lengths are, and the fourth is geometry over a course this package builds itself.
//
// Two, and not one: a window holding a single point draws no line, and the delta, the average and
// the movement the client computes off it are all arithmetic over a pair. The week is the one
// exception, and it is the tape's — nobody measures a waist daily.
//
// Two days, because the cycle is the only window whose width moves: it opens on the Sunday three
// weeks before the run, so a run on a Sunday is the narrowest it gets.
func TestEverySeededMetricReachesIntoEveryWindow(t *testing.T) {
	for _, today := range []civil.Date{
		civil.NewDate(2026, time.May, 27),
		civil.NewDate(2026, time.May, 31),
	} {
		drafted := theCourse(seededPatientID, today)
		course := protocol.Protocol{
			StartDate: drafted.StartDate, Weeks: drafted.Weeks, Status: drafted.Status,
		}

		for _, window := range measurements.Windows() {
			covered, drawn := measurements.RangeOn(window, &course, today)
			if !drawn {
				t.Errorf("the %s window of a stand seeded on %s covers no days", window, today)

				continue
			}

			reached := map[measurements.Metric]int{}
			for _, reading := range theReadings() {
				if covered.Contains(today.AddDays(-reading.daysBack)) {
					reached[reading.metric]++
				}
			}

			want := 2
			if window == measurements.WindowWeek {
				want = 1
			}
			for _, metric := range seededMetrics() {
				if reached[metric] < want {
					t.Errorf("the %s window of a stand seeded on %s (%s) draws %d %s readings, want %d",
						window, today, today.Weekday(), reached[metric], metric, want)
				}
			}
		}
	}
}

// The spread against the window that holds it, at both ends. Past the widest, a row is one no
// read can be asked about; pulled inside a fortnight, the history satisfies every window above
// and still draws four charts of a single week. Ten weeks is written out rather than derived from
// the cadences, which are what it is here to hold still.
func TestTheHistoryFillsTheWidestWindowWithoutLeavingIt(t *testing.T) {
	today := civil.NewDate(2026, time.May, 27)

	// nil, because the three lengths need no course and the widest of them is a length.
	widest, drawn := measurements.RangeOn(measurements.WindowThreeMonths, nil, today)
	if !drawn {
		t.Fatal("the widest window covers no days")
	}

	for _, reading := range theReadings() {
		if day := today.AddDays(-reading.daysBack); !widest.Contains(day) {
			t.Errorf("a %s of %v sits on %s, outside the %s..%s the widest window covers",
				reading.metric, reading.value, day, widest.From, widest.Through)
		}
	}

	// Off the series and not off every row: the catch-up weigh-in reaches eighty-two days back on
	// its own, and a threshold it satisfies says nothing about the cadence it was written to hold.
	reach := map[measurements.Metric]int{}
	for _, series := range theHistory {
		for _, reading := range series.readings() {
			reach[reading.metric] = max(reach[reading.metric], reading.daysBack)
		}
	}

	for _, metric := range seededMetrics() {
		if reach[metric] < 70 {
			t.Errorf("the %s series reaches %d days back, want at least seventy",
				metric, reach[metric])
		}
	}
}

// The row written last is the one measured first, and that is the whole arrangement: written in
// order, no read could tell an answer sorted by the clock from one handed back in the order the
// rows went in.
func TestTheOldestReadingIsTheLastOneWritten(t *testing.T) {
	written := theReadings()
	if len(written) < 2 {
		t.Fatalf("the seed writes %d readings, and an order needs two", len(written))
	}

	last := written[len(written)-1]
	for _, reading := range written[:len(written)-1] {
		if reading.daysBack >= last.daysBack {
			t.Fatalf("the last row written is measured %d days back and a %s before it %d, "+
				"so the write order and the clock agree", last.daysBack, reading.metric, reading.daysBack)
		}
	}
}

// One day carrying two readings of one metric is the only arrangement that tells a series
// ordered by the clock from one ordered by the day.
func TestOneSeededDayCarriesTwoReadingsOfOneMetric(t *testing.T) {
	type when struct {
		metric   measurements.Metric
		daysBack int
	}

	byDay := map[when][]civil.Slot{}
	for _, reading := range theReadings() {
		byDay[when{reading.metric, reading.daysBack}] = append(
			byDay[when{reading.metric, reading.daysBack}], reading.at,
		)
	}

	twice := 0
	for day, at := range byDay {
		if len(at) == 1 {
			continue
		}
		twice++

		if len(at) != 2 {
			t.Errorf("%d %s readings sit %d days back", len(at), day.metric, day.daysBack)
		}
		if slices.Contains(at[1:], at[0]) {
			t.Errorf("two %s readings %d days back share the instant, and a point is not a point",
				day.metric, day.daysBack)
		}
	}
	if twice != 1 {
		t.Errorf("%d days carry more than one reading of a metric, want exactly one", twice)
	}
}

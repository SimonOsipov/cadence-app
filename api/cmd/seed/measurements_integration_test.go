//go:build integration

package main

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/measurements"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// theSeededClinic runs the whole command and answers the persona every assertion below is about.
func theSeededClinic(t *testing.T) (deps, civil.UserID) {
	t.Helper()

	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	if err := seed(t.Context(), theClinic(), on); err != nil {
		t.Fatalf("seeding the clinic: %v", err)
	}

	return on, thePersona(t, on)
}

// noonOn is an instant inside the seeded day in the seeded people's own zone, which is what the
// read resolves their day from.
func noonOn(day civil.Date) time.Time {
	return time.Date(day.Year, day.Month, day.Day, 12, 0, 0, 0, seededPlace())
}

func theTrends(
	t *testing.T, on deps, patient civil.UserID, window measurements.Window,
) measurements.Trends {
	t.Helper()

	var trends measurements.Trends
	if err := database.WithCaller(t.Context(), on.requests,
		database.Caller{Subject: string(patient), Role: "patient"},
		func(ctx context.Context, tx pgx.Tx) error {
			var err error
			trends, err = measurements.TrendsFor(ctx, tx, patient, noonOn(on.today), window)

			return err
		}); err != nil {
		t.Fatalf("asking for the %s window: %v", window, err)
	}

	return trends
}

func theTrend(
	t *testing.T, on deps, patient civil.UserID,
	window measurements.Window, metric measurements.Metric,
) measurements.Trend {
	t.Helper()

	var trend measurements.Trend
	if err := database.WithCaller(t.Context(), on.requests,
		database.Caller{Subject: string(patient), Role: "patient"},
		func(ctx context.Context, tx pgx.Tx) error {
			var err error
			trend, err = measurements.TrendFor(
				ctx, tx, patient, noonOn(on.today), window, metric,
			)

			return err
		}); err != nil {
		t.Fatalf("asking for the %s of the %s window: %v", metric, window, err)
	}

	return trend
}

// The point of the pass, asked the way a screen asks it: as the patient, through the request
// pool, of the read the handlers call. Ninety-two inserts say nothing about what any of the four
// windows draws — three of them are lengths counted back from the patient's own day, and the
// fourth is the geometry of her course.
func TestEveryWindowTheSeededPatientCanAskForDrawsSomething(t *testing.T) {
	on, patient := theSeededClinic(t)

	for _, window := range measurements.Windows() {
		drawsEverySeededMetric(t, window, theTrends(t, on, patient, window))
	}
}

// The cycle window is at its narrowest on a Sunday: the course opens on the Sunday three weeks
// back, so its far edge is that day exactly and the weekly series land on it. The Wednesday the
// other stands are seeded on gives them three days of slack and hides it.
func TestTheCycleWindowOfAStandSeededOnASundayDrawsToo(t *testing.T) {
	on, db := seedStand(t)
	on.today = civil.NewDate(2026, time.May, 31)
	if weekday := on.today.Weekday(); weekday != time.Sunday {
		t.Fatalf("the narrow stand is seeded on a %s", weekday)
	}
	theFirstAdministrator(t, db)

	if err := seed(t.Context(), theClinic(), on); err != nil {
		t.Fatalf("seeding the clinic: %v", err)
	}

	patient := thePersona(t, on)
	drawsEverySeededMetric(t, measurements.WindowCycle,
		theTrends(t, on, patient, measurements.WindowCycle))

	// The assertion the narrow stand is for, and it is about the far edge rather than about
	// emptiness: every seeded series carries a reading on the day of the run, so a window
	// opening a day late is invisible to the helper above. Here the weekly weight lands on that
	// edge exactly — three weeks back is the day the course opened — and a day late drops it.
	weight := theTrend(t, on, patient, measurements.WindowCycle, measurements.MetricWeight)
	if len(weight.Series.Points) == 0 {
		t.Fatal("the cycle draws no weight at all")
	}
	if oldest, want := dayOf(t, weight.Series.Points[0].MeasuredAt, weight.Timezone),
		on.today.AddDays(-21); oldest != want {
		t.Errorf("its oldest weight is %s, want %s — the day the course opened", oldest, want)
	}
}

// The answer's eight series: six the seed records and the two it leaves alone, which answer
// empty rather than being missing.
func drawsEverySeededMetric(t *testing.T, window measurements.Window, trends measurements.Trends) {
	t.Helper()

	if trends.Range == nil {
		t.Fatalf("the %s window covers no days at all", window)
	}

	// The set first, because every assertion below is conditional on the metric being answered
	// at all: a read that dropped the two unmeasured ones would satisfy each of them silently.
	answered := make([]measurements.Metric, 0, len(trends.Series))
	for _, series := range trends.Series {
		answered = append(answered, series.Metric)
	}
	if want := measurements.Metrics(); !slices.Equal(answered, want) {
		t.Fatalf("the %s window answers %v, want %v", window, answered, want)
	}

	for _, series := range trends.Series {
		switch series.Metric {
		// Deliberately unmeasured, and the assertion is what keeps it deliberate: the screen
		// has to be able to say «не измерялось», and the sleep score is one the API derives
		// from imported sessions rather than one anybody types.
		case measurements.MetricChest, measurements.MetricSleep:
			if len(series.Points) != 0 {
				t.Errorf("the %s window answers %d %s readings, want none",
					window, len(series.Points), series.Metric)
			}
		default:
			if len(series.Points) == 0 {
				t.Errorf("the %s window answers no %s at all", window, series.Metric)
			}
		}
	}
}

// The catch-up weigh-in is written last and measured before every other reading, so a read
// handing rows back in the order they went in answers it last. Asked over the widest window,
// which is the only one that reaches back far enough to hold it.
func TestTheSeededWeightAnswersInTheOrderItWasMeasured(t *testing.T) {
	on, patient := theSeededClinic(t)

	points := theTrend(t, on, patient,
		measurements.WindowThreeMonths, measurements.MetricWeight).Series.Points
	if len(points) < 2 {
		t.Fatalf("the widest window answers %d weights, and an order needs two", len(points))
	}

	for i, point := range points[1:] {
		if point.MeasuredAt.Before(points[i].MeasuredAt) {
			t.Errorf("the point at %s comes after the one at %s",
				points[i].MeasuredAt, point.MeasuredAt)
		}
	}

	// The catch-up's own value, written out rather than read off the fixture: an expectation
	// taken from the row under test moves with it and pins nothing.
	if points[0].Value != 117.8 {
		t.Errorf("the widest window opens on %v кг, want the catch-up weigh-in at 117,8",
			points[0].Value)
	}
	if want := on.today.AddDays(-82); dayOf(t, points[0].MeasuredAt, seededZone) != want {
		t.Errorf("it is dated %s, want %s", points[0].MeasuredAt, want)
	}
}

// One day carrying two weighings is the arrangement that tells a series ordered by the clock
// from one ordered by the day. Binned by the zone the answer itself publishes, and the clock each
// point reads there is pinned too: stamped in UTC the pair comes apart across midnight, and in a
// zone an hour or two off it holds together while three days back becomes two and a morning
// weigh-in reads 10:10.
func TestTheSeededWeekCarriesADayWithTwoWeighings(t *testing.T) {
	on, patient := theSeededClinic(t)

	trend := theTrend(t, on, patient, measurements.WindowWeek, measurements.MetricWeight)
	where, err := time.LoadLocation(trend.Timezone)
	if err != nil {
		t.Fatalf("the answer is dated in %q: %v", trend.Timezone, err)
	}

	perDay := map[civil.Date][]string{}
	for _, point := range trend.Series.Points {
		local := point.MeasuredAt.In(where)
		day := civil.NewDate(local.Date())
		perDay[day] = append(perDay[day], local.Format("15:04"))
	}

	twice := 0
	for day, at := range perDay {
		if len(at) == 1 {
			continue
		}
		twice++

		if want := on.today.AddDays(-3); day != want {
			t.Errorf("the day with two weighings is %s, want %s", day, want)
		}
		if want := []string{"07:10", "21:40"}; !slices.Equal(at, want) {
			t.Errorf("%s carries weighings at %v, want %v", day, at, want)
		}
	}
	if twice != 1 {
		t.Errorf("%d days of the week carry two weighings, want exactly one", twice)
	}
}

// A seed run again against a stand somebody is already using: a doubled history is two points on
// every day of every chart, and every delta the client computes off them is halved.
func TestRunningTheSeedTwiceRecordsOneHistory(t *testing.T) {
	on, db := seedStand(t)
	theFirstAdministrator(t, db)

	for range 2 {
		if err := seed(t.Context(), theClinic(), on); err != nil {
			t.Fatalf("seeding: %v", err)
		}
	}

	if got := countOf(t, on.writes, `SELECT count(*) FROM app.measurements`); got != len(theReadings()) {
		t.Errorf("the stand holds %d readings, want the %d the seed writes", got, len(theReadings()))
	}
}

func dayOf(t *testing.T, at time.Time, zone string) civil.Date {
	t.Helper()

	where, err := time.LoadLocation(zone)
	if err != nil {
		t.Fatalf("loading %s: %v", zone, err)
	}

	return civil.NewDate(at.In(where).Date())
}

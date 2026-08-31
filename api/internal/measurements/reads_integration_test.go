//go:build integration

package measurements_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/measurements"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// 09:00 on 5 August 2026 in Yekaterinburg, where every seeded patient lives, and 04:00 UTC.
// The two disagree about which day it is for five hours a day, which is what every window
// edge below is cut against.
var theMorning = time.Date(2026, time.August, 5, 4, 0, 0, 0, time.UTC)

const (
	// The zone of the seeded patients, and the whole reason the edges are worth a test:
	// midnight there is 19:00 UTC of the day before.
	zoneOfThePatients = "Asia/Yekaterinburg"

	theDayTheCourseBegan = "2026-07-20"
)

// A course with two injectables, one of them titrating, so the detail read has a strip to draw
// and the position it draws is decided rather than assumed. The supplement carries no phases;
// the overlay's choice is measured by its own suite, and what this fixture is for is that the
// choice happens at all on the read path.
type course struct {
	protocolID  string
	titrating   string
	flat        string
	supplement  string
	firstMark   civil.Date
	secondMark  civil.Date
	lastDayOfIt civil.Date
}

func (c clinic) prescribe(t *testing.T, patient, status string) course {
	t.Helper()

	var prescribed course
	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			var compound string
			if err := tx.QueryRow(ctx, `
				INSERT INTO app.compounds (name_ru, default_unit, route, icon)
				VALUES ('Семаглутид', 'мг', 'sc', 'syringe')
				RETURNING id::text
			`).Scan(&compound); err != nil {
				return fmt.Errorf("the compound: %w", err)
			}

			if err := tx.QueryRow(ctx, `
				INSERT INTO app.protocols (patient_id, start_date, duration_weeks, status)
				VALUES ($1, DATE '`+theDayTheCourseBegan+`', 12, $2)
				RETURNING id::text
			`, patient, status).Scan(&prescribed.protocolID); err != nil {
				return fmt.Errorf("the course: %w", err)
			}

			for _, item := range []struct {
				into   *string
				kind   string
				phases [][3]any
			}{
				{&prescribed.titrating, "injection", [][3]any{{1, 2, 0.25}, {3, 12, 0.5}}},
				{&prescribed.flat, "injection", [][3]any{{1, 12, 1.0}}},
				{&prescribed.supplement, "supplement", nil},
			} {
				if err := tx.QueryRow(ctx, `
					INSERT INTO app.protocol_items
					    (protocol_id, kind, compound_id, cadence, days_of_week, times, loggable)
					VALUES ($1, $2, $3, 'weekly', ARRAY[1]::smallint[], ARRAY['08:00']::time[], true)
					RETURNING id::text
				`, prescribed.protocolID, item.kind, compound).Scan(item.into); err != nil {
					return fmt.Errorf("the %s item: %w", item.kind, err)
				}
				for _, phase := range item.phases {
					if _, err := tx.Exec(ctx, `
						INSERT INTO app.protocol_phases
						    (protocol_item_id, from_week, to_week, dose_value, dose_unit)
						VALUES ($1, $2, $3, $4, 'мг')
					`, *item.into, phase[0], phase[1], phase[2]); err != nil {
						return fmt.Errorf("a phase: %w", err)
					}
				}
			}

			return nil
		},
	); err != nil {
		t.Fatalf("prescribing for %s: %v", patient, err)
	}

	// Week 1 opens on the day the course began and week 3 seven times two days later; the
	// marks are what ProtocolMarks draws, and they are named here so the assertions do not
	// recompute the arithmetic they are checking.
	began := civil.NewDate(2026, time.July, 20)
	prescribed.firstMark = began
	prescribed.secondMark = began.AddDays(14)
	prescribed.lastDayOfIt = began.AddDays(12*7 - 1)

	return prescribed
}

// reading writes one row under the service role, which is the only path that can choose the
// source. The instant is given as a literal with its offset so that the edge cases below say
// which local midnight they are a hair either side of.
func (c clinic) reading(t *testing.T, patient, metric string, value float64, at, source string) string {
	t.Helper()

	unit := map[string]string{
		"weight": "kg", "hrv": "ms", "rhr": "bpm", "sleep": "/100",
		"bodyfat": "%", "waist": "cm", "hip": "cm", "chest": "cm",
	}[metric]

	var id string
	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `
				INSERT INTO app.measurements
				    (patient_id, metric, value, unit, measured_at, source)
				VALUES ($1, $2, $3, $4, $5::timestamptz, $6)
				RETURNING id::text
			`, patient, metric, value, unit, at, source).Scan(&id)
		},
	); err != nil {
		t.Fatalf("seeding a %s of %s: %v", metric, patient, err)
	}

	return id
}

// reads runs fn as the patient, through the request pool, which is the seam the read handlers
// use: RLS answers, and nothing here is privileged.
func (c clinic) reads(t *testing.T, patient string, fn func(context.Context, pgx.Tx) error) {
	t.Helper()

	if err := c.as(t, patient, "patient", fn); err != nil {
		t.Fatalf("reading as %s: %v", patient, err)
	}
}

func (c clinic) trends(t *testing.T, patient string, window measurements.Window) measurements.Trends {
	t.Helper()

	var trends measurements.Trends
	c.reads(t, patient, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		trends, err = measurements.TrendsFor(
			ctx, tx, civil.UserID(patient), theMorning, window,
		)

		return err
	})

	return trends
}

func (c clinic) trend(
	t *testing.T, patient string, window measurements.Window, metric measurements.Metric,
) measurements.Trend {
	t.Helper()

	var trend measurements.Trend
	c.reads(t, patient, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		trend, err = measurements.TrendFor(
			ctx, tx, civil.UserID(patient), theMorning, window, metric,
		)

		return err
	})

	return trend
}

func pointsOf(series measurements.Series) []float64 {
	values := make([]float64, 0, len(series.Points))
	for _, point := range series.Points {
		values = append(values, point.Value)
	}

	return values
}

func spanOf(t *testing.T, span measurements.Span) string {
	t.Helper()

	if span.Range == nil {
		return "no window"
	}

	return span.Range.From.String() + ".." + span.Range.Through.String()
}

// Every metric, in the order the module enumerates them, and the answer is checked as a set of
// codes rather than by its length: a read that dropped the unmeasured five is exactly as
// well-formed as one that did not.
func TestTheOverviewCarriesEveryMetricInEnumerationOrder(t *testing.T) {
	c := newClinic(t)

	trends := c.trends(t, patientA, measurements.WindowWeek)

	var got []measurements.Metric
	for _, series := range trends.Series {
		got = append(got, series.Metric)
	}
	want := []measurements.Metric{
		measurements.MetricWeight, measurements.MetricHRV, measurements.MetricRHR,
		measurements.MetricSleep, measurements.MetricBodyFat, measurements.MetricWaist,
		measurements.MetricHip, measurements.MetricChest,
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("the overview answers %v", got)
	}

	// The positive control: without it every assertion above holds over eight empty lists.
	if values := pointsOf(trends.Series[0]); len(values) != 1 || values[0] != 82.4 {
		t.Errorf("the weight series carries %v", values)
	}
	if values := pointsOf(trends.Series[4]); len(values) != 0 {
		t.Errorf("the unmeasured bodyfat series carries %v", values)
	}
}

// The span the answer is dated in, and the zone that makes those dates mean anything: without
// it the client bins the points into its own days and a reading admitted at the window's edge
// is drawn beyond it.
func TestTheAnswerCarriesThePatientsZoneAndTheWindowAsDates(t *testing.T) {
	c := newClinic(t)

	trends := c.trends(t, patientA, measurements.WindowWeek)

	if trends.Timezone != zoneOfThePatients {
		t.Errorf("the answer is dated in %q", trends.Timezone)
	}
	// 7 days ending on the patient's own day, both edges counted.
	if got := spanOf(t, trends.Span); got != "2026-07-30..2026-08-05" {
		t.Errorf("the week window is %s", got)
	}
	if trends.Window != measurements.WindowWeek {
		t.Errorf("the answer names the window %q", trends.Window)
	}
}

// The window's edges are local midnights, and this is the assertion the whole zone apparatus
// exists for: at 19:00 UTC on 29 July it is already the thirtieth in Yekaterinburg, so a
// reading at that instant is the first day's and one a second earlier is not. Cut in UTC the
// two would swap sides, and the seven-day window would quietly hold five hours of an eighth day.
func TestTheWindowsEdgesAreMidnightsInThePatientsOwnZone(t *testing.T) {
	c := newClinic(t)

	// Written with the offset the zone has on those dates rather than as a local time, so
	// that the instant is unambiguous and the test is about the server's arithmetic.
	c.reading(t, patientA, "waist", 91, "2026-07-29 19:00:00+00", "manual")  // 00:00 on the 30th
	c.reading(t, patientA, "waist", 999, "2026-07-29 18:59:59+00", "manual") // 23:59:59 on the 29th
	c.reading(t, patientA, "waist", 89, "2026-08-05 18:59:59+00", "manual")  // 23:59:59 on the 5th
	c.reading(t, patientA, "waist", 998, "2026-08-05 19:00:00+00", "manual") // 00:00 on the 6th

	trend := c.trend(t, patientA, measurements.WindowWeek, measurements.MetricWaist)

	// 91 then 89: oldest first, and neither sentinel is among them. The values are what say
	// which row it is — a count would pass on two rows of the wrong pair.
	if got := fmt.Sprint(pointsOf(trend.Series)); got != "[91 89]" {
		t.Errorf("the waist series over %s is %s", spanOf(t, trend.Span), got)
	}
}

// The patient's own day and not the server's, which is a different assertion from the window
// edges above: this one moves «today» itself. At 20:00 UTC on the fourth it is already one in
// the morning of the fifth in Yekaterinburg, so a week counted from the server's day would be
// the thirty-ninth of July through the fourth — a day short at both ends.
func TestTodayIsThePatientsDayAndNotTheServersOwn(t *testing.T) {
	c := newClinic(t)

	theNightBefore := time.Date(2026, time.August, 4, 20, 0, 0, 0, time.UTC)

	var trends measurements.Trends
	c.reads(t, patientA, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		trends, err = measurements.TrendsFor(
			ctx, tx, civil.UserID(patientA), theNightBefore, measurements.WindowWeek,
		)

		return err
	})

	if got := spanOf(t, trends.Span); got != "2026-07-30..2026-08-05" {
		t.Errorf("the week window is %s", got)
	}
}

// The cycle window is the course's own geometry, and status is not asked: the patient whose
// course the doctor cancelled still has an axis, because the days happened.
func TestTheCycleWindowIsTheGeometryOfTheLastCourseWhateverItsStatus(t *testing.T) {
	for _, status := range []string{"active", "completed", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			c := newClinic(t)
			c.prescribe(t, patientA, status)

			trends := c.trends(t, patientA, measurements.WindowCycle)

			// [start, min(today, the last prescribed day)] — the course runs to
			// October, so today is what closes it.
			if got := spanOf(t, trends.Span); got != theDayTheCourseBegan+"..2026-08-05" {
				t.Errorf("the cycle window is %s", got)
			}
			if values := pointsOf(trends.Series[0]); len(values) != 1 {
				t.Errorf("the weight series carries %v", values)
			}
		})
	}
}

// The two empty cases, and eight metrics in both: a patient with no window still has to be
// able to find out that a metric is unmeasured.
func TestTheCycleWindowIsAbsentWithNoCourseAndWithOneThatHasNotStarted(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*testing.T, clinic)
	}{
		{"no course at all", func(*testing.T, clinic) {}},
		{"a course that starts tomorrow", func(t *testing.T, c clinic) {
			if err := database.WithServiceJob(
				t.Context(), c.service, seedJob,
				func(ctx context.Context, tx pgx.Tx) error {
					_, err := tx.Exec(ctx, `
						INSERT INTO app.protocols
						    (patient_id, start_date, duration_weeks, status)
						VALUES ($1, DATE '2026-08-06', 12, 'active')
					`, patientA)

					return err
				},
			); err != nil {
				t.Fatalf("prescribing: %v", err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newClinic(t)
			tc.prepare(t, c)

			trends := c.trends(t, patientA, measurements.WindowCycle)

			if trends.Range != nil {
				t.Errorf("the cycle window is %s", spanOf(t, trends.Span))
			}
			if len(trends.Series) != 8 {
				t.Errorf("the answer carries %d metrics", len(trends.Series))
			}
			// The zone is still the patient's: the response is dated even when it
			// covers nothing, or a client cannot say what it is looking at.
			if trends.Timezone != zoneOfThePatients {
				t.Errorf("the empty answer is dated in %q", trends.Timezone)
			}
		})
	}
}

// The detail read: one metric's points, and the prescription drawn under them. The strip is
// the titrating position's, which is the overlay's choice — measured here only in that it is made
// at all and that the marks land on the days the phases open.
func TestTheDetailCarriesOneMetricAndTheStripUnderIt(t *testing.T) {
	c := newClinic(t)
	prescribed := c.prescribe(t, patientA, "active")

	trend := c.trend(t, patientA, measurements.WindowCycle, measurements.MetricWeight)

	if trend.Series.Metric != measurements.MetricWeight {
		t.Errorf("the detail answers about %q", trend.Series.Metric)
	}
	if values := pointsOf(trend.Series); fmt.Sprint(values) != "[82.4]" {
		t.Errorf("the weight series carries %v", values)
	}
	if len(trend.Overlay.Bands) != 2 {
		t.Errorf("the strip has %d bands: %+v", len(trend.Overlay.Bands), trend.Overlay.Bands)
	}
	var marked []string
	for _, mark := range trend.Overlay.Marks {
		marked = append(marked, mark.Date.String())
	}
	if fmt.Sprint(marked) != fmt.Sprint([]string{
		prescribed.firstMark.String(), prescribed.secondMark.String(),
	}) {
		t.Errorf("the marks sit on %v", marked)
	}
}

// A cancelled course draws an axis with points and no strip, and that is the pair this test
// exists for: the window is geometry and survives the cancellation, the strip is prescription
// and does not. Both lists are empty rather than nil — the client's own fields are not nullable.
func TestACancelledCourseGivesAnAxisWithPointsAndNoStrip(t *testing.T) {
	c := newClinic(t)
	c.prescribe(t, patientA, "cancelled")

	trend := c.trend(t, patientA, measurements.WindowCycle, measurements.MetricWeight)

	if got := spanOf(t, trend.Span); got != theDayTheCourseBegan+"..2026-08-05" {
		t.Errorf("the cycle window is %s", got)
	}
	if values := pointsOf(trend.Series); fmt.Sprint(values) != "[82.4]" {
		t.Errorf("the weight series carries %v", values)
	}
	if trend.Overlay.Bands == nil || len(trend.Overlay.Bands) != 0 {
		t.Errorf("the strip has bands: %+v", trend.Overlay.Bands)
	}
	if trend.Overlay.Marks == nil || len(trend.Overlay.Marks) != 0 {
		t.Errorf("the strip has marks: %+v", trend.Overlay.Marks)
	}
}

// counting is a transaction that keeps a tally of the statements naming a table. The seam it
// wraps is pgx.Tx, which is an interface, so the count is of the statements the read actually
// sent rather than of what its structure suggests it sends.
type counting struct {
	pgx.Tx

	table string
	reads int
}

func (c *counting) count(sql string) {
	// `app.protocols` and not `app.protocol_items`: neither string is a substring of the
	// other, so the plain match says exactly «the course row was read».
	if strings.Contains(sql, c.table) {
		c.reads++
	}
}

func (c *counting) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	c.count(sql)

	return c.Tx.Query(ctx, sql, args...)
}

func (c *counting) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	c.count(sql)

	return c.Tx.QueryRow(ctx, sql, args...)
}

// The plan is read at most once per request, counted rather than asserted in prose — and the
// upper bound is not the whole claim: the three windows that are lengths read it not at all,
// because they are counted back from the patient's day and a course they never look at is a
// query nobody needs. The detail reads it once and hands the one value to both readers of it,
// the cycle window's geometry and the overlay's choice of position.
func TestTheCoursePlanIsReadAtMostOncePerRequest(t *testing.T) {
	c := newClinic(t)
	c.prescribe(t, patientA, "active")

	for _, want := range []struct {
		name  string
		reads int
		ask   func(context.Context, pgx.Tx) error
	}{
		{"the overview over a length", 0, func(ctx context.Context, tx pgx.Tx) error {
			_, err := measurements.TrendsFor(
				ctx, tx, civil.UserID(patientA), theMorning, measurements.WindowWeek,
			)

			return err
		}},
		{"the overview over the cycle", 1, func(ctx context.Context, tx pgx.Tx) error {
			_, err := measurements.TrendsFor(
				ctx, tx, civil.UserID(patientA), theMorning, measurements.WindowCycle,
			)

			return err
		}},
		{"the detail over a length", 1, func(ctx context.Context, tx pgx.Tx) error {
			_, err := measurements.TrendFor(
				ctx, tx, civil.UserID(patientA), theMorning,
				measurements.WindowWeek, measurements.MetricWeight,
			)

			return err
		}},
		{"the detail over the cycle", 1, func(ctx context.Context, tx pgx.Tx) error {
			_, err := measurements.TrendFor(
				ctx, tx, civil.UserID(patientA), theMorning,
				measurements.WindowCycle, measurements.MetricWeight,
			)

			return err
		}},
	} {
		t.Run(want.name, func(t *testing.T) {
			counter := &counting{table: "app.protocols"}
			c.reads(t, patientA, func(ctx context.Context, tx pgx.Tx) error {
				counter.Tx = tx

				return want.ask(ctx, counter)
			})

			if counter.reads != want.reads {
				t.Errorf("the course was read %d times, want %d", counter.reads, want.reads)
			}
		})
	}
}

// The counter is a test instrument, and an instrument that cannot report a fault measures
// nothing: two reads of the course in one request have to come out as two.
func TestTheCounterSeesASecondReadOfTheCourse(t *testing.T) {
	c := newClinic(t)
	c.prescribe(t, patientA, "active")

	counter := &counting{table: "app.protocols"}
	c.reads(t, patientA, func(ctx context.Context, tx pgx.Tx) error {
		counter.Tx = tx
		for range 2 {
			if _, _, err := protocol.LastPlanFor(ctx, counter, civil.UserID(patientA)); err != nil {
				return err
			}
		}

		return nil
	})

	if counter.reads != 2 {
		t.Errorf("two reads of the course counted as %d", counter.reads)
	}
}

// The predicates written out beside the policies that say the same thing, measured on the one
// path where the policy is not their deputy: the service role reads every patient's rows, so a
// read leaning on row security alone answers somebody else's history here. Every other call in
// this suite runs as cadence_patient, where dropping any of them changes nothing at all.
//
// The delete is the sharpest of the three. Without its predicate another patient's imported row
// is found, and the refusal becomes «yours, but imported» — which is the 404/409 split reporting
// one patient's history to another, one status code at a time.
func TestThePredicatesHoldWhereRowSecurityIsNotTheirDeputy(t *testing.T) {
	c := newClinic(t)

	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			trends, err := measurements.TrendsFor(
				ctx, tx, civil.UserID(patientA), theMorning, measurements.WindowWeek,
			)
			if err != nil {
				return err
			}
			if got := fmt.Sprint(pointsOf(trends.Series[0])); got != "[82.4]" {
				t.Errorf("the overview reaches %s", got)
			}
			if got := fmt.Sprint(pointsOf(trends.Series[1])); got != "[61]" {
				t.Errorf("the overview reaches %s of hrv", got)
			}

			trend, err := measurements.TrendFor(
				ctx, tx, civil.UserID(patientA), theMorning,
				measurements.WindowWeek, measurements.MetricWeight,
			)
			if err != nil {
				return err
			}
			if got := fmt.Sprint(pointsOf(trend.Series)); got != "[82.4]" {
				t.Errorf("the detail reaches %s", got)
			}

			if err := measurements.Delete(
				ctx, tx, civil.UserID(patientA), measurements.ReadingID(c.imported[patientB]),
			); !errors.Is(err, measurements.ErrNoSuchReading) {
				t.Errorf("deleting another patient's imported row answered %v", err)
			}

			return nil
		},
	); err != nil {
		t.Fatalf("reading as the service role: %v", err)
	}
}

// A patient whose profile carries no zone is refused rather than answered in the server's:
// every window edge is a local midnight, and the server's would move the axis by a day for
// half a clinic. Named so that provisioning learns which patient to fix.
//
// Both spellings of «no zone», because the column admits both — it is nullable text with no
// CHECK — and they fail differently: `time.LoadLocation("")` answers UTC and no error, so a
// guard that only looks for NULL draws the axis in the wrong zone silently, which is worse
// than the refusal it was supposed to be.
func TestAPatientWithNoTimezoneIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name string
		zone *string
	}{
		{"no zone at all", nil},
		{"a zone of nothing", new(string)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newClinic(t)

			if err := database.WithServiceJob(
				t.Context(), c.service, seedJob,
				func(ctx context.Context, tx pgx.Tx) error {
					_, err := tx.Exec(ctx,
						`UPDATE app.profiles SET timezone = $2 WHERE user_id = $1`,
						patientA, tc.zone)

					return err
				},
			); err != nil {
				t.Fatalf("clearing the zone: %v", err)
			}

			err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
				_, err := measurements.TrendsFor(
					ctx, tx, civil.UserID(patientA), theMorning, measurements.WindowWeek,
				)

				return err
			})
			if !errors.Is(err, measurements.ErrNoTimezone) {
				t.Fatalf("the read answered %v", err)
			}
		})
	}
}

// Nobody else's points, and both reads asked: the predicate is written out beside the policy
// rather than left to it, which is this schema's doctrine, and a read that trusted RLS alone
// would answer another patient's history the day this function is called under a role that
// can see more than one.
func TestAPatientReadsNobodyElsesPoints(t *testing.T) {
	c := newClinic(t)
	c.reading(t, patientB, "weight", 60.5, "2026-08-02 07:00:00+05", "manual")

	trends := c.trends(t, patientA, measurements.WindowWeek)
	if got := fmt.Sprint(pointsOf(trends.Series[0])); got != "[82.4]" {
		t.Errorf("the overview of A carries %s", got)
	}

	trend := c.trend(t, patientA, measurements.WindowWeek, measurements.MetricWeight)
	if got := fmt.Sprint(pointsOf(trend.Series)); got != "[82.4]" {
		t.Errorf("the detail of A carries %s", got)
	}
}

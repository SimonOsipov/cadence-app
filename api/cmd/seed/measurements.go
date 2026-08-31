package main

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/measurements"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// The two times of day a reading is stamped with, in the patient's own zone. The minutes are
// this seed's own — the prototype describes a morning weighing («Утром · натощак») and
// prescribes its weekly weigh-in at 07:30, and neither of these two hours is written anywhere in
// it. Two things hang off them: they are hours apart on one day, and both are written out in the
// test that reads them back through the patient's zone — stamped in UTC the evening one lands on
// the next day and the morning one reads 10:10.
var (
	morning = civil.Slot{Hour: 7, Minute: 10}
	evening = civil.Slot{Hour: 21, Minute: 40}
)

// seededReading is one row of the persona's history. The day is counted back from the day of
// the run and never written down: a series with an end date in it expires, and this project
// has already watched every screen go blank the morning after one did.
type seededReading struct {
	metric   measurements.Metric
	daysBack int
	at       civil.Slot
	value    float64
}

// metricSeries is one metric measured at a fixed cadence: `values` oldest first, `every` days
// apart, the last of them `until` days back from the day of the run.
type metricSeries struct {
	metric measurements.Metric
	every  int
	until  int
	values []float64
}

// The persona's history as the frozen prototype draws it.
//
// Weight, HRV and the resting pulse are the trends screen's own series
// (mobile/src/features/trends/data.ts), which gives each of them twelve weeks a week apart and
// the last week a day at a time — so they arrive here as two rows of this table rather than
// one. Its widest series is cut at both ends: the oldest point sits 84 days back and the widest
// window opens 83 days back, and the newest is the day of the run, which the daily week carries.
//
// Body fat and waist are the Body screen's (mobile/src/features/body/data.ts) and deliberately
// not the trends module's, which runs body fat 22,4 → 19,0 and the waist 37,5 → 35,0 «см» —
// nineteen per cent of fat and a thirty-five-centimetre waist on a patient of 110 кг at 188 см.
// The Body screen's numbers are the ones her profile agrees with. The hip is only there: the
// trends module carries a thigh instead, which is a different circumference.
//
// Its four samples are labelled «Нед 1..4» and are stretched over the same twelve weeks as the
// weight, because the two modules disagree about how long the story takes and one calendar has
// to hold both: at the Body screen's cadence the persona loses nine centimetres of waist and
// three and a half points of fat against one kilogram. Stretched, the same numbers read as seven
// kilograms of fat and no lean mass — which is what the trends module's own copy says is
// happening («Идут вместе — это жир, не вода», weight ↔ waist).
//
// Sleep is absent because no patient types one: it is the score the API derives from imported
// sessions, and there is no importer. Chest is absent the way KMP's MockSeed leaves it absent
// («Chest is left unmeasured on purpose», MockSeed.kt:337) and not the way the Body screen's
// fixture does, which measures it: a screen has to be able to say «не измерялось».
var theHistory = []metricSeries{
	{
		metric: measurements.MetricWeight, every: 7, until: 7,
		values: []float64{
			117.4, 116.6, 115.7, 114.8, 114.0, 113.2, 112.4, 111.6, 111.0, 110.5, 110.2,
		},
	},
	{
		metric: measurements.MetricWeight, every: 1, until: 0,
		values: []float64{110.6, 110.4, 110.3, 110.4, 110.2, 110.1, 110.0},
	},
	{
		metric: measurements.MetricHRV, every: 7, until: 7,
		values: []float64{55, 58, 56, 59, 61, 60, 63, 65, 67, 69, 70},
	},
	{
		metric: measurements.MetricHRV, every: 1, until: 0,
		values: []float64{66, 68, 67, 70, 69, 71, 71},
	},
	{
		metric: measurements.MetricRHR, every: 7, until: 7,
		values: []float64{65, 65, 64, 64, 63, 62, 62, 60, 59, 58, 58},
	},
	{
		metric: measurements.MetricRHR, every: 1, until: 0,
		values: []float64{60, 59, 60, 58, 58, 58, 58},
	},
	{
		metric: measurements.MetricBodyFat, every: 7, until: 0,
		values: stretched([]float64{44.0, 42.6, 41.4, 40.5}, tapeWeeks),
	},
	{
		metric: measurements.MetricWaist, every: 7, until: 0,
		values: stretched([]float64{101, 97, 94, 92}, tapeWeeks),
	},
	{
		metric: measurements.MetricHip, every: 7, until: 0,
		values: stretched([]float64{116, 112, 110, 108}, tapeWeeks),
	},
}

// tapeWeeks is how many weekly readings the four samples become. Twelve, because the tape has to
// reach as far back as the weight it is read beside — and a window drawing one point draws no
// line, which is what four samples a month apart leave in the cycle. The week keeps its single
// point either way: a tape is not read daily.
const tapeWeeks = 12

// stretched reads `points` readings off the line the samples anchor, at even steps along it: the
// two ends are the samples' own exactly, and every sample between them bends the line rather than
// being passed by. Rounded to the tenth the metrics are recorded in.
func stretched(samples []float64, points int) []float64 {
	over := make([]float64, 0, points)
	for i := range points {
		at := float64(i) * float64(len(samples)-1) / float64(points-1)
		below := int(at)
		if below >= len(samples)-1 {
			over = append(over, samples[len(samples)-1])

			continue
		}
		between := samples[below] + (samples[below+1]-samples[below])*(at-float64(below))
		over = append(over, math.Round(between*10)/10)
	}

	return over
}

// The two rows the table above cannot hold, and both are there to be read back rather than to
// be looked at.
//
// The evening weighing is a second reading on a day that already has one: a series ordered by
// the clock and one ordered by the day are the same answer until a day carries two.
//
// The catch-up is a weigh-in typed in late. It is written last and measured before every other
// row, so the order the answer comes back in cannot be the order the rows went in — and it sits
// 82 days back: older than the oldest series row, and inside the widest window, which opens 83
// days back. A row outside all four windows is a row no read can be asked about.
var theArrangedReadings = []seededReading{
	{metric: measurements.MetricWeight, daysBack: 3, at: evening, value: 110.7},
	{metric: measurements.MetricWeight, daysBack: 82, at: morning, value: 117.8},
}

// theReadings is every row this pass writes, in the order it writes them.
func theReadings() []seededReading {
	var written []seededReading
	for _, series := range theHistory {
		written = append(written, series.readings()...)
	}

	return append(written, theArrangedReadings...)
}

func (s metricSeries) readings() []seededReading {
	readings := make([]seededReading, 0, len(s.values))
	for i, value := range s.values {
		readings = append(readings, seededReading{
			metric:   s.metric,
			daysBack: s.until + (len(s.values)-1-i)*s.every,
			at:       morning,
			value:    value,
		})
	}

	return readings
}

// recordTheReadings gives the persona the history the trends screens draw, once.
//
// Through measurements.Record and not an INSERT of its own, for the reason this command creates
// people through identity.Onboarding: the unit is a function of the metric, the plausible
// interval belongs to that module, and a second copy of either here is the copy that drifts.
//
// It writes no audit row, and that is the table's arrangement rather than this command's
// shortcut: 000026 gives measurements the diary's — request seam, RLS, no audit row — so no
// path into this table writes one, for the patient or for anybody. What the seed does not
// inherit is the invariant's reading of the service role (architecture/overview.md, «every such
// path writes to the audit log»), and the two disagree here as they already do over the vials
// and the doses fillTheCabinet writes. Recorded rather than settled: making it true is four
// passes and a decision about what actor a seeded clinic acts as.
//
// Re-running the seed is ordinary, so a patient who already holds a reading is left with the
// history they have: a second run would double every point on every chart.
func recordTheReadings(
	ctx context.Context, writes *pgxpool.Pool, patient civil.UserID, today civil.Date,
) (bool, error) {
	recorded := false
	err := database.WithServiceJob(ctx, writes, seedJob, func(ctx context.Context, tx pgx.Tx) error {
		var held bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM app.measurements WHERE patient_id = $1)`,
			string(patient)).Scan(&held); err != nil {
			return fmt.Errorf("looking for a history already recorded: %w", err)
		}
		if held {
			return nil
		}

		where := seededPlace()
		// The instant the seeded day ends, which is what Record refuses a reading after. The
		// command's own calendar and not the host's clock, like every other pass. The day's end
		// and not an hour inside it because what the history may not run past is the day of the
		// run, not some time of day within it: no reading is stamped after 07:10 on it today,
		// and a fixture moved to the evening must not start being refused. The price is named
		// rather than hidden — a stand seeded before 07:10 holds one point on every chart
		// measured ahead of its own created_at, which is the state ErrMeasuredInTheFuture
		// exists to refuse a patient. Nothing reads the pair, and 000025 constrains neither
		// against the other.
		now := time.Date(today.Year, today.Month, today.Day+1, 0, 0, 0, 0, where)

		for _, reading := range theReadings() {
			day := today.AddDays(-reading.daysBack)
			if _, err := measurements.Record(ctx, tx, patient, now, measurements.Draft{
				Metric: reading.metric,
				Value:  reading.value,
				MeasuredAt: time.Date(day.Year, day.Month, day.Day,
					reading.at.Hour, reading.at.Minute, 0, 0, where),
			}); err != nil {
				return fmt.Errorf("recording a %s of %v on %s: %w",
					reading.metric, reading.value, day, err)
			}
		}
		recorded = true

		return nil
	})
	if err != nil {
		return false, err
	}

	return recorded, nil
}

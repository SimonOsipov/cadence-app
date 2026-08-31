package measurements

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// ErrNoTimezone is a patient whose profile carries no zone. Every window edge is a midnight
// in the patient's own day, so there is no safe default: the server's zone would move the
// axis by a day for half a clinic, and refusing says which patient to fix.
var ErrNoTimezone = errors.New("the patient's timezone is not recorded")

// Span is what an answer is dated in: the window that was asked for, the days it covers, and
// the zone those days are the patient's own in. The zone travels because without it a client
// bins the points into its own days, and a reading admitted at the window's edge is drawn
// beyond it.
//
// Range is absent rather than empty in the two cases the cycle window has none — no course at
// all, and a course that has not started. It is the absence KMP's own `TrendWindow.rangeOn`
// answers with, its constructor refusing a range that runs backwards; the field it lands in,
// `TrendsOverview.range`, does not admit it — recorded in docs/prototype-divergences.md.
type Span struct {
	Window   Window
	Range    *civil.Range
	Timezone string
}

// Trends is the overview: one window, all eight metrics, the unmeasured ones included.
type Trends struct {
	Span

	Series []Series
}

// Trend is one metric over one window with the prescription drawn under it.
type Trend struct {
	Span

	Series  Series
	Overlay Overlay
}

// TrendsFor is the overview read.
//
// The course is read only for the cycle window: the other three are lengths counted back from
// the patient's own day, and a plan nothing asks about is a query nobody needs.
func TrendsFor(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, now time.Time, window Window,
) (Trends, error) {
	zone, err := zoneOf(ctx, tx, patient, now)
	if err != nil {
		return Trends{}, err
	}

	var course *protocol.Protocol
	if window == WindowCycle {
		plan, found, err := protocol.LastPlanFor(ctx, tx, patient)
		if err != nil {
			return Trends{}, err
		}
		if found {
			course = &plan.Protocol
		}
	}

	// Eight empty series and not none: a window covering no days is still a window, and a
	// patient has to be able to find out that a metric is unmeasured.
	trends := Trends{
		Span:   Span{Window: window, Timezone: zone.Name},
		Series: Overview(nil),
	}

	r, covered := RangeOn(window, course, zone.Today)
	if !covered {
		return trends, nil
	}
	trends.Range = &r

	readings, err := readingsIn(ctx, tx, patient, r, zone.Where)
	if err != nil {
		return Trends{}, err
	}
	trends.Series = Overview(readings)

	return trends, nil
}

// TrendFor is the detail read: one metric's points and the strip of prescription under them.
//
// The course is read once, here, and the same value answers both readers of it — the cycle
// window's geometry and the overlay's choice of position. Asked where each is needed it would
// be two queries for one fact.
func TrendFor(
	ctx context.Context, tx pgx.Tx, patient civil.UserID,
	now time.Time, window Window, metric Metric,
) (Trend, error) {
	zone, err := zoneOf(ctx, tx, patient, now)
	if err != nil {
		return Trend{}, err
	}

	plan, found, err := protocol.LastPlanFor(ctx, tx, patient)
	if err != nil {
		return Trend{}, err
	}
	var (
		last   *protocol.Plan
		course *protocol.Protocol
	)
	if found {
		last = &plan
		course = &plan.Protocol
	}

	trend := Trend{
		Span:   Span{Window: window, Timezone: zone.Name},
		Series: SeriesOf(metric, nil),
		// No window, no strip — and taken from the function that owns the empty overlay's
		// shape rather than written out again, because neither of its lists may be nil.
		Overlay: OverlayOn(nil, civil.Range{}),
	}

	r, covered := RangeOn(window, course, zone.Today)
	if !covered {
		return trend, nil
	}
	trend.Range = &r

	readings, err := readingsOf(ctx, tx, patient, metric, r, zone.Where)
	if err != nil {
		return Trend{}, err
	}
	trend.Series = SeriesOf(metric, readings)
	trend.Overlay = OverlayOn(last, r)

	return trend, nil
}

// patientZone is the patient's zone under its three names: the one that travels on the wire,
// the location the window is cut with, and the day they are living in.
type patientZone struct {
	Name  string
	Where *time.Location
	Today civil.Date
}

// zoneOf is the third copy of this read in the API — protocol.DayOf and dosing.todayFor are
// the other two — and it is recorded as debt rather than extracted, because moving them into
// a shared place would touch two shipped packages.
//
// It is not a call of either: both answer the patient's day and neither answers the zone's
// name, which is what a trend response carries so that the client bins its points into the
// days this window was cut in.
func zoneOf(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, now time.Time,
) (patientZone, error) {
	var name *string
	if err := tx.QueryRow(ctx,
		`SELECT timezone FROM app.profiles WHERE user_id = $1`, string(patient)).Scan(&name); err != nil {
		return patientZone{}, fmt.Errorf("reading the timezone of %s: %w", patient, err)
	}
	if name == nil || *name == "" {
		return patientZone{}, fmt.Errorf("%s: %w", patient, ErrNoTimezone)
	}

	where, err := time.LoadLocation(*name)
	if err != nil {
		return patientZone{}, fmt.Errorf("the zone %q of %s: %w", *name, patient, err)
	}

	local := now.In(where)

	return patientZone{
		Name:  *name,
		Where: where,
		Today: civil.NewDate(local.Year(), local.Month(), local.Day()),
	}, nil
}

// The two reads of the table, and there are two rather than nine: the overview asks for the
// window once and splits the rows into eight series in Go, while the detail asks for the one
// metric it draws.
//
// Neither declares an order. SeriesOf imposes a total one — the clock, then the row's own id —
// and its suite runs both arrival orders for that reason, so ordering here would be a second
// answer to a question already settled.
const readingColumns = `id::text, metric, value, measured_at`

func readingsIn(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, r civil.Range, where *time.Location,
) ([]Reading, error) {
	from, until := instantsOf(r, where)

	rows, err := tx.Query(ctx, `
		SELECT `+readingColumns+`
		FROM app.measurements
		WHERE patient_id = $1 AND measured_at >= $2 AND measured_at < $3
	`, string(patient), from, until)
	if err != nil {
		return nil, fmt.Errorf("reading the measurements of %s: %w", patient, err)
	}

	return scanReadings(rows)
}

func readingsOf(
	ctx context.Context, tx pgx.Tx, patient civil.UserID,
	metric Metric, r civil.Range, where *time.Location,
) ([]Reading, error) {
	from, until := instantsOf(r, where)

	rows, err := tx.Query(ctx, `
		SELECT `+readingColumns+`
		FROM app.measurements
		WHERE patient_id = $1 AND metric = $2 AND measured_at >= $3 AND measured_at < $4
	`, string(patient), string(metric), from, until)
	if err != nil {
		return nil, fmt.Errorf("reading the %s of %s: %w", metric, patient, err)
	}

	return scanReadings(rows)
}

func scanReadings(rows pgx.Rows) ([]Reading, error) {
	defer rows.Close()

	var readings []Reading
	for rows.Next() {
		var (
			reading Reading
			metric  string
		)
		if err := rows.Scan(&reading.ID, &metric, &reading.Value, &reading.MeasuredAt); err != nil {
			return nil, err
		}

		// Parsed and not cast, as journal's tags and dosing's zones are: a value the schema
		// admits and Go does not is a set that drifted, and casting it would put the row on
		// an axis of its own that no overview ever asks for.
		parsed, ok := ParseMetric(metric)
		if !ok {
			return nil, fmt.Errorf("the reading %s names the metric %q", reading.ID, metric)
		}
		reading.Metric = parsed

		readings = append(readings, reading)
	}

	return readings, rows.Err()
}

// instantsOf is the window's days as the half-open interval of instants that holds them.
//
// The edges are local midnights, because a day is a day in the patient's own zone: the first
// of August in Yekaterinburg opens at 19:00 UTC on the thirty-first of July, and an interval
// cut at UTC midnight would take five hours of the wrong day at each end.
//
// Half-open at the top rather than closed on the last day's final instant: the closing edge
// is the next day's midnight, and a reading landing exactly there belongs to that next day.
func instantsOf(r civil.Range, where *time.Location) (from, until time.Time) {
	return time.Date(r.From.Year, r.From.Month, r.From.Day, 0, 0, 0, 0, where),
		time.Date(r.Through.Year, r.Through.Month, r.Through.Day+1, 0, 0, 0, 0, where)
}

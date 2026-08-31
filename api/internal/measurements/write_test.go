package measurements

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// A transaction that counts what reached it. pgx.Tx is an interface, so «the guard ran above
// the statement» is a measurement here rather than an inference — the embedded nil would
// panic on anything this does not override, which is itself the assertion that nothing else
// is called.
type countingTx struct {
	pgx.Tx

	statements int
}

func (tx *countingTx) QueryRow(context.Context, string, ...any) pgx.Row {
	tx.statements++

	return reachedRow{}
}

type reachedRow struct{}

func (reachedRow) Scan(...any) error { return errors.New("the statement was reached") }

var theInstant = time.Date(2026, time.August, 5, 4, 0, 0, 0, time.UTC)

// The sleep score is computed from imported sessions and there is no number for a patient to
// type, so the refusal is the server's own and not the schema's alone: a caller that reaches
// this package with no document — the seed, a job, a client generated from an older contract
// — is refused here too, and refused before any statement.
func TestTheSleepScoreIsNotAReadingAPatientTypes(t *testing.T) {
	tx := &countingTx{}

	_, err := Record(t.Context(), tx, "7c4d1a90-0000-4000-8000-0000000000a1", theInstant, Draft{
		Metric: MetricSleep, Value: 82, MeasuredAt: theInstant.Add(-time.Hour),
	})
	if !errors.Is(err, ErrMetricNotWritable) {
		t.Errorf("writing a sleep score answered %v", err)
	}
	if tx.statements != 0 {
		t.Errorf("the refusal reached %d statements", tx.statements)
	}
}

// The positive control, and the reason the assertion above is not satisfied by a Record that
// refuses everything: the seven do reach a statement, and each of them separately, so a guard
// widened to one metric too many goes red here rather than in the suite behind Docker.
func TestEveryWritableMetricReachesItsStatement(t *testing.T) {
	for _, metric := range WritableMetrics() {
		t.Run(string(metric), func(t *testing.T) {
			tx := &countingTx{}

			bound, ok := Bounds(metric)
			if !ok {
				t.Fatalf("%q carries no bound", metric)
			}
			_, err := Record(t.Context(), tx, "7c4d1a90-0000-4000-8000-0000000000a1", theInstant, Draft{
				Metric: metric, Value: (bound.Low + bound.High) / 2, MeasuredAt: theInstant.Add(-time.Hour),
			})
			if errors.Is(err, ErrMetricNotWritable) {
				t.Errorf("%q was refused as unwritable", metric)
			}
			if tx.statements != 1 {
				t.Errorf("%q reached %d statements", metric, tx.statements)
			}
		})
	}
}

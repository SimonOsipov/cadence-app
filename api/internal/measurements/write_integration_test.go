//go:build integration

package measurements_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/measurements"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

// One hand-typed row and one imported one, which is what newClinic gives every patient.
const seededReadings = 2

// A quarter of an hour before theMorning, and it has to differ from it: the row's instant is
// read back below, and against a draft measured at `now` a write that stored the recording
// instant instead of the measured one would be indistinguishable from a correct one.
var theQuarterHourBefore = theMorning.Add(-15 * time.Minute)

func (c clinic) record(t *testing.T, patient string, draft measurements.Draft) (measurements.Recorded, error) {
	t.Helper()

	var recorded measurements.Recorded
	err := c.as(t, patient, "patient", func(ctx context.Context, tx pgx.Tx) error {
		var err error
		recorded, err = measurements.Record(
			ctx, tx, civil.UserID(patient), theMorning, draft,
		)

		return err
	})

	return recorded, err
}

func (c clinic) delete(t *testing.T, patient string, id measurements.ReadingID) error {
	t.Helper()

	return c.as(t, patient, "patient", func(ctx context.Context, tx pgx.Tx) error {
		return measurements.Delete(ctx, tx, civil.UserID(patient), id)
	})
}

// The row as the privileged role reads it back, which is the witness a request-path error
// cannot be: a statement the policy filtered away answers success and zero rows.
type row struct {
	patient    string
	metric     string
	value      float64
	unit       string
	measuredAt time.Time
	source     string
	note       *string
}

func (c clinic) row(t *testing.T, id measurements.ReadingID) row {
	t.Helper()

	var stored row
	if err := c.superuser.QueryRow(t.Context(), `
		SELECT patient_id::text, metric, value, unit, measured_at, source, note
		FROM app.measurements WHERE id = $1
	`, string(id)).Scan(
		&stored.patient, &stored.metric, &stored.value, &stored.unit,
		&stored.measuredAt, &stored.source, &stored.note,
	); err != nil {
		t.Fatalf("re-reading %s as the superuser: %v", id, err)
	}

	return stored
}

// held is every row the patient holds, counted as the privileged role: what a request answered
// is not a witness for what a request wrote.
func (c clinic) held(t *testing.T, patient string) int {
	t.Helper()

	var held int
	if err := c.superuser.QueryRow(t.Context(), `
		SELECT count(*) FROM app.measurements WHERE patient_id = $1
	`, patient).Scan(&held); err != nil {
		t.Fatalf("counting %s's readings: %v", patient, err)
	}

	return held
}

func draft(edit func(*measurements.Draft)) measurements.Draft {
	d := measurements.Draft{
		Metric:     measurements.MetricWeight,
		Value:      81.2,
		MeasuredAt: theQuarterHourBefore,
	}
	if edit != nil {
		edit(&d)
	}

	return d
}

// The unit is the server's and never the request's: Draft carries no field for one, and the
// row is re-read to say which unit landed. Source too — the column is defaulted and the
// patient holds no grant on it, so a hand-typed row cannot claim to have come off a watch.
func TestAHandTypedReadingIsWrittenWithTheUnitAndSourceTheServerChose(t *testing.T) {
	c := newClinic(t)

	note := "после зала"
	recorded, err := c.record(t, patientA, draft(func(d *measurements.Draft) { d.Note = &note }))
	if err != nil {
		t.Fatalf("recording: %v", err)
	}

	if recorded.Unit != "kg" || recorded.Source != measurements.SourceManual {
		t.Errorf("the reply says %q from %q", recorded.Unit, recorded.Source)
	}

	stored := c.row(t, recorded.ID)
	if stored.patient != patientA || stored.metric != "weight" || stored.value != 81.2 {
		t.Errorf("the row reads %+v", stored)
	}
	if stored.unit != "kg" || stored.source != "manual" {
		t.Errorf("the row is %q from %q", stored.unit, stored.source)
	}
	// The column the axis is drawn along, and the one the reply cannot witness: Recorded's own
	// MeasuredAt is copied from the draft, so comparing the two compares a value to itself.
	// Replace the parameter in the INSERT with `now` and only this line goes red.
	if !stored.measuredAt.Equal(theQuarterHourBefore) {
		t.Errorf("the row is measured at %v, not %v", stored.measuredAt, theQuarterHourBefore)
	}
	if stored.note == nil || *stored.note != note {
		t.Errorf("the row's note is %v", stored.note)
	}
}

// The four refusals the server makes for itself, each named rather than answered as a 500, and
// each proved to have reached no statement.
//
// The count is taken on the same transaction and not afterwards as the superuser: WithCaller
// commits only when the closure returns nil, so a count taken outside would read the seeded two
// however late in Record the check sat, and the assertion could not fail. Inside, a value or date
// check moved below the INSERT reads three. The unknown metric goes red elsewhere in this test —
// it is refused by the value check first, because Bounds answers a zero interval for a metric it
// has no row for — so that row is pinned by its errors.Is and not by the count.
func TestAReadingTheServerCannotBelieveNeverReachesAStatement(t *testing.T) {
	for _, tc := range []struct {
		name  string
		draft measurements.Draft
		want  error
	}{
		{"measured after it was recorded", draft(func(d *measurements.Draft) {
			d.MeasuredAt = theMorning.Add(time.Second)
		}), measurements.ErrMeasuredInTheFuture},
		{"a metric off the set", draft(func(d *measurements.Draft) {
			d.Metric = "thigh"
		}), measurements.ErrUnknownMetric},
		{"a weight below what a patient can be", draft(func(d *measurements.Draft) {
			d.Value = 19.9
		}), measurements.ErrValueImplausible},
		{"a weight with a slipped decimal point", draft(func(d *measurements.Draft) {
			d.Value = 812
		}), measurements.ErrValueImplausible},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newClinic(t)

			var (
				refusal error
				held    int
			)
			if err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
				_, refusal = measurements.Record(
					ctx, tx, civil.UserID(patientA), theMorning, tc.draft,
				)

				return tx.QueryRow(ctx, `
					SELECT count(*) FROM app.measurements WHERE patient_id = $1
				`, patientA).Scan(&held)
			}); err != nil {
				t.Fatalf("counting inside the write's own transaction: %v", err)
			}

			if !errors.Is(refusal, tc.want) {
				t.Errorf("the write answered %v", refusal)
			}
			// Every row of theirs, and not the subset a value filter would leave: an
			// assertion narrowed to «not 82,4» is satisfied by a write that stored some
			// other number, which is one of the mutations it is here to catch.
			if held != seededReadings {
				t.Errorf("the patient holds %d readings, not the %d seeded", held, seededReadings)
			}
		})
	}
}

// The fifth refusal is the table's own, and it cannot be measured the way the four above are: a
// failed statement aborts the transaction it ran in, so a count taken after it is an error
// rather than a zero. That abort IS the witness the others need a count for — what this pins is
// that the 23514 is classified rather than reaching the patient as a 500 about a field they
// filled in themselves.
func TestANoteOfNothingIsRefusedByTheTableAndSaidInWords(t *testing.T) {
	c := newClinic(t)

	blank := "   "
	_, err := c.record(t, patientA, draft(func(d *measurements.Draft) { d.Note = &blank }))
	if !errors.Is(err, measurements.ErrNoteSaysNothing) {
		t.Fatalf("the write answered %v", err)
	}
}

// Both edges of the interval are admitted, which is the other half of the refusals above: a
// check written with `>` and `<` refuses a reading the clinic considers ordinary and leaves
// every test above green.
func TestTheEdgesOfTheIntervalAreAdmitted(t *testing.T) {
	c := newClinic(t)

	for _, value := range []float64{20, 400} {
		if _, err := c.record(t, patientA, draft(func(d *measurements.Draft) {
			d.Value = value
		})); err != nil {
			t.Errorf("a weight of %v was refused: %v", value, err)
		}
	}
}

// A reading measured now is not a reading measured in the future. Without this the refusal
// above is satisfied by one that refuses everything.
func TestAReadingMeasuredAtThisVeryInstantIsAdmitted(t *testing.T) {
	c := newClinic(t)

	if _, err := c.record(t, patientA, draft(func(d *measurements.Draft) {
		d.MeasuredAt = theMorning
	})); err != nil {
		t.Fatalf("recording: %v", err)
	}
}

// Somebody else's row is invisible, and 404 is the only honest answer: admitting it exists
// would tell one patient about another's history through the status code alone.
func TestDeletingSomebodyElsesReadingAnswersNoSuchReading(t *testing.T) {
	c := newClinic(t)

	theirs := measurements.ReadingID(c.manual[patientB])
	if err := c.delete(t, patientA, theirs); !errors.Is(err, measurements.ErrNoSuchReading) {
		t.Fatalf("the delete answered %v", err)
	}
	if !c.survives(t, string(theirs)) {
		t.Error("the other patient's row was deleted")
	}
}

// An identifier nobody holds answers exactly what somebody else's row answers, which is what
// keeps the status code from reporting whether a row exists.
func TestDeletingAReadingNobodyHoldsAnswersNoSuchReading(t *testing.T) {
	c := newClinic(t)

	for _, id := range []measurements.ReadingID{
		"7c4d1a90-0000-4000-8000-00000000dead",
		// Not a uuid at all: `WHERE id = $1` on one raises 22P02, and a 500 for what
		// is a 404 tells the caller their identifier reached the database.
		"the-one-i-deleted",
	} {
		if err := c.delete(t, patientA, id); !errors.Is(err, measurements.ErrNoSuchReading) {
			t.Errorf("deleting %q answered %v", id, err)
		}
	}
}

// Their own imported row is a different refusal, and the reason is that it is a different
// truth: the row is on their screen, so «no such reading» would be a lie, and the sample
// would return on the next sync anyway. The two are told apart by a read before the delete —
// the statement itself answers zero rows and nil in both cases.
func TestDeletingOnesOwnImportedReadingIsRefusedWithItsReason(t *testing.T) {
	c := newClinic(t)

	imported := measurements.ReadingID(c.imported[patientA])
	err := c.delete(t, patientA, imported)
	if !errors.Is(err, measurements.ErrReadingWasImported) {
		t.Fatalf("the delete answered %v", err)
	}
	if errors.Is(err, measurements.ErrNoSuchReading) {
		t.Error("a row the patient can see was reported as absent")
	}
	if !c.survives(t, string(imported)) {
		t.Error("the imported row was deleted")
	}
}

// The positive control for both refusals above: without it they hold over a delete that never
// deletes anything at all.
func TestDeletingOnesOwnHandTypedReadingRemovesIt(t *testing.T) {
	c := newClinic(t)

	mine := measurements.ReadingID(c.manual[patientA])
	if err := c.delete(t, patientA, mine); err != nil {
		t.Fatalf("the delete answered %v", err)
	}
	if c.survives(t, string(mine)) {
		t.Error("the row is still there")
	}
	// And the neighbour's row of the same shape is untouched, so the delete was bounded
	// rather than merely successful.
	if !c.survives(t, c.manual[patientB]) {
		t.Error("the other patient's row went with it")
	}
}

// A row written through Record can be deleted through Delete: the source the write leaves is
// the one the delete's predicate admits, and the two halves are only bound together by that
// column's default.
func TestAReadingJustWrittenCanBeDeletedByThePatientWhoWroteIt(t *testing.T) {
	c := newClinic(t)

	recorded, err := c.record(t, patientA, draft(nil))
	if err != nil {
		t.Fatalf("recording: %v", err)
	}
	if err := c.delete(t, patientA, recorded.ID); err != nil {
		t.Fatalf("deleting: %v", err)
	}
	if c.survives(t, string(recorded.ID)) {
		t.Error("the row is still there")
	}
}

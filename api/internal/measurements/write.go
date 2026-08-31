package measurements

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

var (
	// ErrUnknownMetric is a metric this module has no row for. Named rather than left to
	// the schema's CHECK, because a 23514 from a closed set the server also declares is a
	// refusal the caller can be told about by field name.
	ErrUnknownMetric = errors.New("no such metric")

	// ErrMetricNotWritable is a metric this API records and no patient types. Only the sleep
	// score is one: it is computed from imported sessions, so there is nothing to read off a
	// watch face — and it is refused here as well as by the request schema, because a caller
	// reaching this package without a document would otherwise write a number nobody can
	// produce. Not by the table: 000025 admits all eight metrics, and has to, because the
	// import path writes the sleep score on the service role.
	ErrMetricNotWritable = errors.New("this metric is not one a patient records by hand")

	// ErrMeasuredInTheFuture is a reading taken after the instant it was recorded. It is a
	// wrong clock or a mistyped date, and admitting it would put a point past the right
	// edge of every window the patient can ask for.
	ErrMeasuredInTheFuture = errors.New("a reading is measured before it is recorded")

	// ErrValueImplausible is a value outside what the metric can read at all — a slipped
	// decimal point, not a number the clinic would worry about. The threshold is what says
	// the latter, and a reading that fails it is exactly the reading the screen is for.
	ErrValueImplausible = errors.New("the value is outside what the metric can plausibly read")

	// ErrNoteSaysNothing is a note of whitespace. Named for dosing's reason: the schema
	// refuses it, and unclassified that refusal reaches the patient as a 500 on a field
	// they filled in themselves.
	ErrNoteSaysNothing = errors.New("a note is either absent or says something")

	// ErrNoSuchReading is a reading this patient does not hold — and one nobody holds, told
	// apart from nothing, because a status that distinguished them would report another
	// patient's history one bit at a time.
	ErrNoSuchReading = errors.New("no such reading")

	// ErrReadingWasImported is their own row, and visible on their screen, which is why it
	// is not the error above: the sample is the health platform's fact and returns on the
	// next sync, so the refusal has to say why rather than deny the row exists.
	ErrReadingWasImported = errors.New("an imported reading is not the patient's to delete")
)

// Draft is a reading as the patient types it.
//
// No unit and no source: the unit is a function of the metric and the server sets it, and the
// source is defaulted by the schema on a column the patient holds no grant on — so a
// hand-typed row cannot claim to have come off a watch.
type Draft struct {
	Metric     Metric
	Value      float64
	MeasuredAt time.Time
	Note       *string
}

// Recorded is the row that was written, as the reply carries it.
type Recorded struct {
	ID         ReadingID
	Metric     Metric
	Value      float64
	Unit       string
	MeasuredAt time.Time
	Source     Source
}

// Record writes one hand-typed reading.
//
// `now` is a parameter and never the clock, like everything else in this package: the future
// the reading is refused for is the server's own instant, which is the only one both the
// patient and the row's `created_at` are measured against.
func Record(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, now time.Time, draft Draft,
) (Recorded, error) {
	meta, known := Meta(draft.Metric)
	bound, bounded := Bounds(draft.Metric)
	// Both, and not just the first: a metric carrying a unit and no interval would
	// otherwise be written with nothing checking its value at all.
	if !known || !bounded {
		return Recorded{}, fmt.Errorf("%q: %w", draft.Metric, ErrUnknownMetric)
	}
	if !writable(draft.Metric) {
		return Recorded{}, fmt.Errorf("%q: %w", draft.Metric, ErrMetricNotWritable)
	}
	if draft.MeasuredAt.After(now) {
		return Recorded{}, fmt.Errorf("%s: %w", draft.MeasuredAt, ErrMeasuredInTheFuture)
	}
	if !bound.Contains(draft.Value) {
		return Recorded{}, fmt.Errorf(
			"%v %s: %w", draft.Value, meta.Unit, ErrValueImplausible,
		)
	}

	recorded := Recorded{
		Metric:     draft.Metric,
		Value:      draft.Value,
		Unit:       meta.Unit,
		MeasuredAt: draft.MeasuredAt,
	}

	// The source is read back rather than assumed: it is the schema's default on a column
	// this statement does not name, so what the reply says about it is a fact about the row
	// and not about this function's expectations.
	var source string
	if err := tx.QueryRow(ctx, `
		INSERT INTO app.measurements (patient_id, metric, value, unit, measured_at, note)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id::text, source
	`, string(patient), string(draft.Metric), draft.Value, meta.Unit,
		draft.MeasuredAt, draft.Note).Scan(&recorded.ID, &source); err != nil {
		return Recorded{}, classify(err)
	}

	parsed, ok := ParseSource(source)
	if !ok {
		return Recorded{}, fmt.Errorf("the reading %s was born as %q", recorded.ID, source)
	}
	recorded.Source = parsed

	return recorded, nil
}

// Delete removes one of the patient's own hand-typed readings.
//
// The row is read before it is deleted, and that read is the whole design. A DELETE the policy
// filtered away answers success and zero rows, and so does one against a row nobody holds — so
// the statement's own outcome cannot tell «not yours» from «yours, but imported». The first is
// ErrNoSuchReading because a row the patient cannot see must not be admitted to exist; the
// second says why, because that row IS on their screen.
func Delete(ctx context.Context, tx pgx.Tx, patient civil.UserID, id ReadingID) error {
	// An identifier arrives from a path, and `WHERE id = $1` on one that is not a uuid
	// raises 22P02 — a 500 for what is a 404, and one that tells the caller their string
	// reached the database. IsUUIDShaped is the shape this project already declared;
	// parsing it again here would be a second definition of it.
	if !database.IsUUIDShaped(string(id)) {
		return fmt.Errorf("%q: %w", id, ErrNoSuchReading)
	}

	// The patient written out beside the policy that says the same thing, which is this
	// schema's doctrine: the predicate holds whatever role the transaction took on.
	var source string
	err := tx.QueryRow(ctx, `
		SELECT source FROM app.measurements WHERE id = $1 AND patient_id = $2
	`, string(id), string(patient)).Scan(&source)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", id, ErrNoSuchReading)
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", id, err)
	}
	if source != string(SourceManual) {
		return fmt.Errorf("%s came from %s: %w", id, source, ErrReadingWasImported)
	}

	tag, err := tx.Exec(ctx, `
		DELETE FROM app.measurements WHERE id = $1 AND patient_id = $2
	`, string(id), string(patient))
	if err != nil {
		return fmt.Errorf("deleting %s: %w", id, err)
	}
	// Not the policy: a statement ago the same transaction read this row under the same
	// identity and found it manual. Zero rows here is another request having deleted it in
	// between, and «no such reading» is what that request already made true.
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", id, ErrNoSuchReading)
	}

	return nil
}

// classify turns the one constraint that answers a legitimate request into its named error,
// and only that one: a catch-all here is how a bug becomes a refusal the client retries.
func classify(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	if pgErr.Code == checkViolation && pgErr.ConstraintName == noteSaysSomething {
		return ErrNoteSaysNothing
	}

	return err
}

const (
	checkViolation    = "23514"
	noteSaysSomething = "measurements_note_check"
)

//go:build integration

package journal_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/journal"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// The policy regression suite for the table 000017 adds. Extending it is an
// acceptance rule of the data layer, not a nicety.

var cluster *testsupport.Cluster

func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

func runSuite(m *testing.M) int {
	ctx := context.Background()

	c, err := testsupport.StartCluster(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting the test cluster: %v\n", err)

		return 1
	}
	cluster = c

	defer func() {
		if err := cluster.Terminate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "terminating the test cluster: %v\n", err)
		}
	}()

	return m.Run()
}

const (
	patientA = "8a1f3b7c-0000-4000-8000-0000000000a1"
	patientB = "8a1f3b7c-0000-4000-8000-0000000000b1"
	doctorA  = "8a1f3b7c-0000-4000-8000-0000000000a2"
	doctorB  = "8a1f3b7c-0000-4000-8000-0000000000b2"
	adminID  = "8a1f3b7c-0000-4000-8000-0000000000c1"

	seedJob = "test.journal"
	dayA    = "2026-05-31"
)

type clinic struct {
	service *pgxpool.Pool
	request *pgxpool.Pool
}

func newClinic(t *testing.T) clinic {
	t.Helper()

	db := cluster.NewDatabase(t)

	service, err := database.NewPool(t.Context(), db.ServiceAppURL)
	if err != nil {
		t.Fatalf("opening the service pool: %v", err)
	}
	t.Cleanup(service.Close)

	request, err := database.NewPool(t.Context(), db.AppURL)
	if err != nil {
		t.Fatalf("opening the request pool: %v", err)
	}
	t.Cleanup(request.Close)

	superuser := testsupport.Connect(t, db.SuperuserURL)
	if _, err := superuser.Exec(t.Context(), `
		INSERT INTO app.profiles (user_id, role, full_name, timezone)
		VALUES ($1, 'admin', 'Пётр Аверин', 'Europe/Moscow')
	`, adminID); err != nil {
		t.Fatalf("seeding the admin: %v", err)
	}

	c := clinic{service: service, request: request}
	if err := database.WithServiceJob(
		t.Context(), service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error { return seed(ctx, tx) },
	); err != nil {
		t.Fatalf("seeding the clinic: %v", err)
	}

	return c
}

// Two patients and two doctors assigned crosswise, and a day each. With one patient
// every row a caller could read is one they are entitled to.
func seed(ctx context.Context, tx pgx.Tx) error {
	for id, name := range map[string]string{doctorA: "Марина Крылова", doctorB: "Ольга Ветрова"} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.profiles (user_id, role, full_name, timezone)
			VALUES ($1, 'doctor', $2, 'Europe/Moscow')
		`, id, name); err != nil {
			return fmt.Errorf("doctor %s: %w", id, err)
		}
	}

	for patient, doctor := range map[string]string{patientA: doctorA, patientB: doctorB} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.profiles (user_id, role, full_name, timezone)
			VALUES ($1, 'patient', 'Пациент', 'Asia/Yekaterinburg')
		`, patient); err != nil {
			return fmt.Errorf("patient %s: %w", patient, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.patient_profiles (user_id) VALUES ($1)
		`, patient); err != nil {
			return fmt.Errorf("card %s: %w", patient, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.care_team_assignments (patient_id, provider_id, care_role, is_primary)
			VALUES ($1, $2, 'endo', true)
		`, patient, doctor); err != nil {
			return fmt.Errorf("assignment %s: %w", patient, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.journal_entries (patient_id, entry_date, mood, source)
			VALUES ($1, $2::date, 3, 'manual')
		`, patient, dayA); err != nil {
			return fmt.Errorf("entry for %s: %w", patient, err)
		}
	}

	return nil
}

func (c clinic) as(t *testing.T, subject, role string, fn func(context.Context, pgx.Tx) error) error {
	t.Helper()

	return database.WithCaller(
		t.Context(), c.request, database.Caller{Subject: subject, Role: role}, fn,
	)
}

func (c clinic) visible(t *testing.T, subject, role, query string) []string {
	t.Helper()

	var seen []string
	if err := c.as(t, subject, role, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			seen = append(seen, id)
		}

		return rows.Err()
	}); err != nil {
		t.Fatalf("reading as %s: %v", role, err)
	}

	return seen
}

// changed reports how many rows a write touched. A count is what a filtered write
// cannot fake: it returns success and zero, which data-layer invariant 5 names.
func (c clinic) changed(t *testing.T, subject, role, sql string, args ...any) int64 {
	t.Helper()

	var affected int64
	if err := c.as(t, subject, role, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, sql, args...)
		affected = tag.RowsAffected()

		return err
	}); err != nil {
		t.Fatalf("writing as %s: %v", role, err)
	}

	return affected
}

func (c clinic) moodOf(t *testing.T, patient, entryDate string) int {
	t.Helper()

	var mood int
	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`SELECT mood FROM app.journal_entries WHERE patient_id = $1 AND entry_date = $2::date`,
				patient, entryDate).Scan(&mood)
		},
	); err != nil {
		t.Fatalf("reading the day's mood: %v", err)
	}

	return mood
}

func (c clinic) ownerOfTheDay(t *testing.T, entryDate string) []string {
	t.Helper()

	var owners []string
	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			rows, err := tx.Query(ctx,
				`SELECT patient_id::text FROM app.journal_entries WHERE entry_date = $1::date ORDER BY patient_id`,
				entryDate)
			if err != nil {
				return err
			}
			defer rows.Close()

			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					return err
				}
				owners = append(owners, id)
			}

			return rows.Err()
		},
	); err != nil {
		t.Fatalf("reading the day's owners: %v", err)
	}

	return owners
}

func TestEachSideSeesItsOwnDayAndNobodyElses(t *testing.T) {
	c := newClinic(t)

	for _, caller := range []struct {
		name    string
		subject string
		role    string
		owner   string
	}{
		{"the patient", patientA, "patient", patientA},
		{"the assigned doctor", doctorA, "doctor", patientA},
		{"the other patient", patientB, "patient", patientB},
		{"the other doctor", doctorB, "doctor", patientB},
	} {
		seen := c.visible(t, caller.subject, caller.role,
			`SELECT patient_id::text FROM app.journal_entries`)
		if len(seen) != 1 || seen[0] != caller.owner {
			t.Errorf("%s reads %v, want exactly [%s]", caller.name, seen, caller.owner)
		}

		// By key rather than by listing: a policy that filters a scan and not a
		// lookup passes the assertion above and fails this one. The key is
		// (patient_id, entry_date), so naming both is the point-lookup form.
		other := patientB
		if caller.owner == patientB {
			other = patientA
		}
		if got := c.visible(t, caller.subject, caller.role, fmt.Sprintf(
			`SELECT patient_id::text FROM app.journal_entries
			 WHERE patient_id = '%s' AND entry_date = DATE '%s'`, other, dayA,
		)); len(got) != 0 {
			t.Errorf("%s fetched another patient's day by key: %v", caller.name, got)
		}
	}
}

func TestAPatientMayNotWriteADayOntoAnotherPatient(t *testing.T) {
	c := newClinic(t)

	for _, attempt := range []struct {
		name    string
		sql     string
		args    []any
		because string
	}{
		{
			"creating one for another patient",
			`INSERT INTO app.journal_entries (patient_id, entry_date, mood, source)
			 VALUES ($1, DATE '2026-06-01', 4, 'manual')`,
			[]any{patientB},
			// The policy, not the grant: patient_id is insertable, and WITH CHECK is
			// what refuses the value.
			"row-level security",
		},
		{
			// The same statement on a day the other patient DOES own, and it has to
			// be refused the same way. This is the first table here whose patient
			// INSERT grant covers the whole primary key — it must, the key is
			// (patient_id, entry_date) — and a uniqueness check bypasses row
			// security by design. If 23505 could answer before WITH CHECK, telling
			// the two dates apart would read one bit of another patient's health
			// record per probe: did they write a diary entry on this day. Measured:
			// the check runs after, and 42501 answers both.
			"probing whether another patient wrote a day",
			`INSERT INTO app.journal_entries (patient_id, entry_date, mood, source)
			 VALUES ($1, $2::date, 4, 'manual')`,
			[]any{patientB, dayA},
			"row-level security",
		},
		{
			// patient_id is not in the patient's UPDATE grant — half the primary key,
			// and moving a day between patients is a different row rather than an
			// edit of this one. The grant refuses before the policy is consulted.
			"handing their own away",
			`UPDATE app.journal_entries SET patient_id = $1 WHERE patient_id = $2`,
			[]any{patientB, patientA},
			// The grant, before any policy: patient_id is not in the patient's UPDATE
			// column list.
			"permission denied",
		},
	} {
		err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, attempt.sql, attempt.args...)

			return err
		})

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("%s: got %v, want SQLSTATE 42501", attempt.name, err)

			continue
		}
		// 42501 has three causes here — a missing grant, a missing policy, forced
		// RLS — and data-layer invariant 5 asks a refusal to name which. It matters
		// on the second case especially: granting patient_id back on UPDATE keeps it
		// green, because WITH CHECK refuses instead, so the column narrowing this
		// step made deliberately would not be measured at all.
		if !strings.Contains(pgErr.Message, attempt.because) {
			t.Errorf("%s: refused with %q, want it to name %q", attempt.name, pgErr.Message, attempt.because)
		}
	}

	// The upsert of step 8, which no shape above reaches: RLS treats ON CONFLICT DO
	// UPDATE unlike anything else — the arbiter lookup is not filtered, and the
	// update's USING raises rather than filtering. It is safe here because
	// patient_id is half the conflict target, so every row an upsert can meet is
	// the caller's own. A later migration swapping the key for a surrogate would
	// delete that property, and this is what would notice.
	err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO app.journal_entries (patient_id, entry_date, mood, source)
			VALUES ($1, $2::date, 4, 'manual')
			ON CONFLICT (patient_id, entry_date) DO UPDATE SET mood = 1
		`, patientB, dayA)

		return err
	})

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Errorf("upserting onto another patient's day: got %v, want SQLSTATE 42501", err)
	}
	if mood := c.moodOf(t, patientB, dayA); mood != 3 {
		t.Errorf("the other patient's day now reads mood %d, want the seeded 3", mood)
	}

	// And the positive control for the same statement, on their own day.
	if affected := c.changed(t, patientA, "patient", `
		INSERT INTO app.journal_entries (patient_id, entry_date, mood, source)
		VALUES ($1, $2::date, 4, 'manual')
		ON CONFLICT (patient_id, entry_date) DO UPDATE SET mood = 1
	`, patientA, dayA); affected != 1 {
		t.Errorf("upserting their own day touched %d rows, want 1", affected)
	}
	if mood := c.moodOf(t, patientA, dayA); mood != 1 {
		t.Errorf("their own day reads mood %d after the upsert, want 1", mood)
	}

	// Taking another patient's day, in the three forms the vials suite measured: the
	// one that names a row and the two that read no column, where the SELECT policies
	// are never added and the update's USING is the only guard.
	for _, theft := range []struct {
		name string
		sql  string
		want int64
	}{
		{"naming the row", fmt.Sprintf(
			`UPDATE app.journal_entries SET mood = 1 WHERE patient_id = '%s'`, patientB), 0},
		{"sweeping the table", `UPDATE app.journal_entries SET mood = 1`, 1},
		{
			"with a predicate that reads nothing",
			`UPDATE app.journal_entries SET mood = 1 WHERE true`, 1,
		},
	} {
		if affected := c.changed(t, patientA, "patient", theft.sql); affected != theft.want {
			t.Errorf("%s: touched %d rows, want %d", theft.name, affected, theft.want)
		}
		// After each form and not once at the end: the thefts write a value, and a
		// count of rows is exactly what a changed value cannot fail.
		if mood := c.moodOf(t, patientB, dayA); mood != 3 {
			t.Errorf("%s: the other patient's mood is now %d, want the seeded 3", theft.name, mood)
		}
	}

	if owners := c.ownerOfTheDay(t, dayA); len(owners) != 2 {
		t.Errorf("the day is owned by %v, want both patients", owners)
	}

	// The positive control the refusals need: without it they would say «cannot
	// write» rather than «cannot write onto somebody else». The seed inserts on the
	// service path, so it cannot supply this.
	if affected := c.changed(t, patientA, "patient", `
		INSERT INTO app.journal_entries (patient_id, entry_date, mood, note, tags, source)
		VALUES ($1, DATE '2026-06-01', 4, 'ровно', ARRAY['nausea']::text[], 'manual')
	`, patientA); affected != 1 {
		t.Errorf("the patient wrote %d days of their own, want 1", affected)
	}
	if owners := c.ownerOfTheDay(t, "2026-06-01"); len(owners) != 1 || owners[0] != patientA {
		t.Errorf("the new day is owned by %v, want just the patient", owners)
	}
}

// The closed set of tags is written twice — once in 000017 and once as constants in
// Go — and this is what keeps the two the same. A fact written twice is otherwise
// fixed once.
func TestTheTagsTheSchemaAcceptsAreTheOnesGoDeclares(t *testing.T) {
	c := newClinic(t)

	declared := []journal.Tag{
		journal.TagNausea, journal.TagFatigue, journal.TagHeadache, journal.TagBloating,
		journal.TagInsomnia, journal.TagSite, journal.TagAppetite,
	}

	// Every one of them, in one row: a CHECK that refused a member would fail here,
	// and one that accepted anything would fail below.
	tags := make([]string, len(declared))
	for i, tag := range declared {
		tags[i] = string(tag)
	}
	if err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO app.journal_entries (patient_id, entry_date, tags, source)
			VALUES ($1, DATE '2026-06-02', $2::text[], 'manual')
		`, patientA, tags)

		return err
	}); err != nil {
		// Asserted here rather than through c.changed, which fatals on error and
		// would report the refusal as «writing as patient» instead of as itself.
		t.Fatalf("the schema refused a tag Go declares: %v", err)
	}

	err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO app.journal_entries (patient_id, entry_date, tags, source)
			VALUES ($1, DATE '2026-06-03', ARRAY['nausia']::text[], 'manual')
		`, patientA)

		return err
	})

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "journal_entries_tags_are_a_flat_named_list" {
		t.Errorf("a tag outside the set: got %v, want the tag constraint", err)
	}

	// And the sets are the same size as well as compatible: a schema that grew an
	// eighth value would pass both assertions above.
	var accepted []string
	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			// Every single-quoted word in the constraint's own definition. Parsing it
			// is the only way to ask «what else does it name» — a behavioural probe
			// can confirm the seven Go knows and can never discover an eighth.
			return tx.QueryRow(ctx, `
				SELECT array_agg(literal[1] ORDER BY literal[1])
				FROM pg_constraint,
				     LATERAL regexp_matches(pg_get_constraintdef(oid), '''([^'']*)''', 'g') AS literal
				WHERE conname = 'journal_entries_tags_are_a_flat_named_list'
			`).Scan(&accepted)
		},
	); err != nil {
		t.Fatalf("reading the constraint: %v", err)
	}
	// Element by element, not by length: a set the same size but differently spelled
	// is exactly what a count cannot see.
	want := make([]string, len(declared))
	for i, tag := range declared {
		want[i] = string(tag)
	}
	slices.Sort(want)
	if !slices.Equal(accepted, want) {
		t.Errorf("the schema names %v, Go declares %v", accepted, want)
	}
}

func TestADayThatSaysNothingIsRefusedByTheSchemaToo(t *testing.T) {
	// The rule lives in Go, where a sheet can be told which of the two refusals it
	// hit. It lives here as well because the request path is not the only writer:
	// the service path inserts, and a seed with an empty day would put a day nobody
	// filled into the feed.
	c := newClinic(t)

	err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO app.journal_entries (patient_id, entry_date, source)
			VALUES ($1, DATE '2026-06-04', 'manual')
		`, patientA)

		return err
	})

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "journal_entries_say_something" {
		t.Errorf("an empty day: got %v, want the say-something constraint", err)
	}

	// Each of the three things that count as saying something, on its own: the
	// constraint is a disjunction, and a suite that only ever names all three could
	// not tell which arm of it works.
	for i, says := range []struct{ column, value string }{
		{"mood", "2"},
		{"tags", "ARRAY['fatigue']::text[]"},
		{"note", "'ровно'"},
	} {
		if affected := c.changed(t, patientA, "patient", fmt.Sprintf(`
			INSERT INTO app.journal_entries (patient_id, entry_date, source, %s)
			VALUES ($1, DATE '2026-07-0%d', 'manual', %s)
		`, says.column, i+1, says.value), patientA); affected != 1 {
			t.Errorf("a day whose only content is %s was refused", says.column)
		}
	}
}

// refuse runs one statement on the service path and returns the SQLSTATE and the
// constraint that refused it. The name and not only the code: two constraints on one
// column refuse the same row, and this schema has had one dead behind another.
func (c clinic) refuse(t *testing.T, sql string, args ...any) (string, string) {
	t.Helper()

	err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, sql, args...)

			return err
		},
	)
	if err == nil {
		t.Fatal("the statement was accepted")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("refused by something that is not the database: %v", err)
	}

	return pgErr.Code, pgErr.ConstraintName
}

// Every row-shape rule this migration adds, named by the constraint that has to
// fire. Six of them were exercised by nothing, and each could have been deleted from
// the migration with the whole gate green — the primary key among them, which is
// this step's headline decision and the component's first invariant.
func TestEachRowShapeRuleOnADayFires(t *testing.T) {
	c := newClinic(t)

	for _, rule := range []struct {
		name       string
		columns    string
		values     string
		constraint string
		code       string
	}{
		{
			// «One entry per day, always». Nothing wrote the same key twice, so a
			// migration that dropped the uniqueness would have shipped.
			"a second entry for a day already written",
			"entry_date, mood, source", "$2::date, 4, 'manual'",
			"journal_entries_one_per_day", "23505",
		},
		{
			"a mood below the scale", "entry_date, mood, source",
			"DATE '2026-06-10', 0, 'manual'", "journal_entries_mood_check", "23514",
		},
		{
			"an energy above the scale", "entry_date, energy, source",
			"DATE '2026-06-11', 6, 'manual'", "journal_entries_energy_check", "23514",
		},
		{
			"a sleep off the scale", "entry_date, sleep, source",
			"DATE '2026-06-12', 9, 'manual'", "journal_entries_sleep_check", "23514",
		},
		{
			"a note past its bound", "entry_date, note, source",
			"DATE '2026-06-13', pg_catalog.repeat('x', 2001), 'manual'",
			"journal_entries_note_check", "23514",
		},
		{
			// A note of spaces is «nothing said» in Go, and the schema has to agree:
			// the service path is the one writer Go does not stand in front of.
			"a note of nothing but spaces", "entry_date, note, source",
			"DATE '2026-06-14', '   ', 'manual'", "journal_entries_note_check", "23514",
		},
		{
			// The second closed set written twice — here and as Go constants — and
			// the one nothing reconciled.
			"a source outside the set", "entry_date, mood, source",
			"DATE '2026-06-15', 3, 'imported'", "journal_entries_source_check", "23514",
		},
	} {
		t.Run(rule.name, func(t *testing.T) {
			sql := fmt.Sprintf(`
				INSERT INTO app.journal_entries (patient_id, %s) VALUES ($1, %s)
			`, rule.columns, rule.values)

			var code, name string
			if rule.constraint == "journal_entries_one_per_day" {
				code, name = c.refuse(t, sql, patientA, dayA)
			} else {
				code, name = c.refuse(t, sql, patientA)
			}
			if code != rule.code || name != rule.constraint {
				t.Errorf("refused by %s/%s, want %s/%s", code, name, rule.code, rule.constraint)
			}
		})
	}

	// The accept side, which a suite of refusals cannot supply: a CHECK refusing
	// everything would pass every case above. Both ends of every scale and not one
	// row holding a mixture — with sleep at 3 the narrowing of its lower bound to 2
	// survived, because the refusal case uses 9 and stays refused either way.
	for _, day := range []struct {
		name  string
		date  string
		value int
	}{
		{"the low end of every scale", "2026-06-20", 1},
		{"the high end of every scale", "2026-06-21", 5},
	} {
		t.Run(day.name+" and a note at its bound", func(t *testing.T) {
			if err := database.WithServiceJob(
				t.Context(), c.service, seedJob,
				func(ctx context.Context, tx pgx.Tx) error {
					_, err := tx.Exec(ctx, `
						INSERT INTO app.journal_entries
						    (patient_id, entry_date, mood, energy, sleep, note, source)
						VALUES ($1, $2::date, $3, $3, $3, pg_catalog.repeat('x', 2000), 'dose')
					`, patientA, day.date, day.value)

					return err
				},
			); err != nil {
				t.Fatalf("a day at the bounds was refused: %v", err)
			}
		})
	}
}

// The other closed set written twice. Unlike the tags nothing reconciled it, so a
// typo in a Go constant compared itself to itself and would first have failed as a
// raw 23514 in production.
func TestTheSourcesTheSchemaAcceptsAreTheOnesGoDeclares(t *testing.T) {
	c := newClinic(t)

	declared := []journal.Source{journal.SourceManual, journal.SourceDose}
	for i, source := range declared {
		if err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.journal_entries (patient_id, entry_date, mood, source)
				VALUES ($1, $2::date, 3, $3)
			`, patientA, fmt.Sprintf("2026-08-0%d", i+1), string(source))

			return err
		}); err != nil {
			t.Fatalf("the schema refused the source %q Go declares: %v", source, err)
		}
	}

	var accepted []string
	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `
				SELECT array_agg(literal[1] ORDER BY literal[1])
				FROM pg_constraint,
				     LATERAL regexp_matches(pg_get_constraintdef(oid), '''([^'']*)''', 'g') AS literal
				WHERE conname = 'journal_entries_source_check'
			`).Scan(&accepted)
		},
	); err != nil {
		t.Fatalf("reading the constraint: %v", err)
	}

	want := []string{string(journal.SourceDose), string(journal.SourceManual)}
	slices.Sort(want)
	if !slices.Equal(accepted, want) {
		t.Errorf("the schema names %v, Go declares %v", accepted, want)
	}
}

// The four controls the cabinet suite carries and this table had none of. The
// crosswise half is measured above; this is the half a test asserting only the gain
// would miss, plus the three policies nothing but the deparse registry exercised.
//
// A clinic each, because the first subtest reassigns a patient and the rest would
// then be reading a world they did not set up.
func TestTheReachOfEachRoleOverTheDiary(t *testing.T) {
	t.Run("reassignment moves the doctor's reach both ways", func(t *testing.T) {
		c := newClinic(t)

		before := c.visible(t, doctorB, "doctor", `SELECT patient_id::text FROM app.journal_entries`)
		if len(before) != 1 || before[0] != patientB {
			t.Fatalf("the other doctor reads %v before the move, want [%s]", before, patientB)
		}

		if err := database.WithServiceJob(
			t.Context(), c.service, seedJob,
			func(ctx context.Context, tx pgx.Tx) error {
				// Struck and reissued: the service path holds INSERT and DELETE on
				// assignments and deliberately not UPDATE.
				if _, err := tx.Exec(ctx,
					`DELETE FROM app.care_team_assignments WHERE patient_id = $1`, patientA); err != nil {
					return err
				}
				_, err := tx.Exec(ctx, `
					INSERT INTO app.care_team_assignments (patient_id, provider_id, care_role, is_primary)
					VALUES ($1, $2, 'endo', true)
				`, patientA, doctorB)

				return err
			},
		); err != nil {
			t.Fatalf("reassigning: %v", err)
		}

		if after := c.visible(t, doctorB, "doctor",
			`SELECT patient_id::text FROM app.journal_entries`); len(after) != 2 {
			t.Errorf("the reassigned doctor reads %v, want both diaries", after)
		}
		if orphaned := c.visible(t, doctorA, "doctor",
			`SELECT patient_id::text FROM app.journal_entries`); len(orphaned) != 0 {
			t.Errorf("the former doctor still reads %v", orphaned)
		}
	})

	t.Run("a doctor reads a day and may not write one", func(t *testing.T) {
		c := newClinic(t)

		// The registry pins journal_entries/cadence_doctor to {SELECT}, so a widened
		// grant is caught — but nothing measured the refusal, and the cabinet suite
		// does. A doctor writing a symptom into a patient's own diary is the one
		// authority this table deliberately withholds from them.
		for _, forbidden := range []struct{ name, sql string }{
			{"rewriting a day", `UPDATE app.journal_entries SET mood = 1 WHERE patient_id = $1`},
			{"writing one", `INSERT INTO app.journal_entries (patient_id, entry_date, mood, source)
			 VALUES ($1, DATE '2026-07-01', 4, 'manual')`},
		} {
			err := c.as(t, doctorA, "doctor", func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, forbidden.sql, patientA)

				return err
			})

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
				t.Errorf("the doctor succeeded at %s: got %v, want 42501", forbidden.name, err)
			}
		}
	})

	t.Run("the admin reaches every diary and may write one", func(t *testing.T) {
		c := newClinic(t)

		if seen := c.visible(t, adminID, "admin",
			`SELECT patient_id::text FROM app.journal_entries`); len(seen) != 2 {
			t.Errorf("the admin reads %v, want both", seen)
		}
		// The WITH CHECK half of FOR ALL, which only an INSERT reaches: on an UPDATE
		// both halves are `true` and removing one is indistinguishable from removing
		// the other.
		if err := c.as(t, adminID, "admin", func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.journal_entries (patient_id, entry_date, mood, source)
				VALUES ($1, DATE '2026-09-01', 4, 'manual')
			`, patientB)

			return err
		}); err != nil {
			t.Fatalf("the admin could not write a day for an unassigned patient: %v", err)
		}

		// And the USING half, on a row the admin is assigned to nobody through: a
		// count is what a filtered UPDATE cannot fake — it returns success and zero.
		if affected := c.changed(t, adminID, "admin",
			`UPDATE app.journal_entries SET mood = 2 WHERE patient_id = $1 AND entry_date = $2::date`,
			patientA, dayA); affected != 1 {
			t.Errorf("the admin's update touched %d rows, want 1", affected)
		}
		if mood := c.moodOf(t, patientA, dayA); mood != 2 {
			t.Errorf("the day reads mood %d after the admin's update, want 2", mood)
		}
	})

	t.Run("a caller with no identity reaches nothing", func(t *testing.T) {
		c := newClinic(t)

		for subject, want := range map[string]error{
			"":           database.ErrNoSubject,
			"not-a-uuid": database.ErrInvalidSubject,
		} {
			err := c.as(t, subject, "patient", func(context.Context, pgx.Tx) error { return nil })
			if !errors.Is(err, want) {
				t.Errorf("subject %q: got %v, want %v", subject, err, want)
			}
		}

		stranger := "8a1f3b7c-0000-4000-8000-00000000dead"
		for _, role := range []string{"patient", "doctor"} {
			if seen := c.visible(t, stranger, role,
				`SELECT patient_id::text FROM app.journal_entries`); len(seen) != 0 {
				t.Errorf("a stranger as %s read %v", role, seen)
			}
		}

		// And the case the seam cannot be asked for: a real caller blanking their own
		// claims inside their own transaction, which the USERSET context permits.
		if err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
			var before int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.journal_entries`).Scan(&before); err != nil {
				return err
			}
			if before == 0 {
				t.Fatal("the caller read nothing before blanking, so this measures nothing")
			}
			if _, err := tx.Exec(ctx, `SELECT set_config('request.jwt.claims', '', true)`); err != nil {
				return err
			}

			var subjectIsNull bool
			if err := tx.QueryRow(ctx, `SELECT app.jwt_subject() IS NULL`).Scan(&subjectIsNull); err != nil {
				return err
			}
			if !subjectIsNull {
				t.Fatal("blanking left a subject in place, so the zero below is not the NULL case")
			}

			var after int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.journal_entries`).Scan(&after); err != nil {
				return err
			}
			if after != 0 {
				t.Errorf("claims blanked: the diary still returns %d row(s)", after)
			}

			return nil
		}); err != nil {
			t.Fatalf("blanking the claims: %v", err)
		}
	})

	t.Run("the service path reads and inserts and does not rewrite or delete", func(t *testing.T) {
		c := newClinic(t)

		for _, forbidden := range []struct{ name, sql string }{
			{"rewriting a day", `UPDATE app.journal_entries SET mood = 1 WHERE patient_id = $1`},
			{"unwriting a day", `DELETE FROM app.journal_entries WHERE patient_id = $1`},
		} {
			err := database.WithServiceJob(
				t.Context(), c.service, seedJob,
				func(ctx context.Context, tx pgx.Tx) error {
					_, err := tx.Exec(ctx, forbidden.sql, patientA)

					return err
				},
			)

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
				t.Errorf("the service path succeeded at %s: got %v, want 42501", forbidden.name, err)
			}
		}

		// And neither may the patient or their doctor unwrite one: a day is edited,
		// not deleted, and the side-effect flag is computed off this history.
		for _, caller := range []struct{ subject, role string }{
			{patientA, "patient"},
			{doctorA, "doctor"},
		} {
			err := c.as(t, caller.subject, caller.role, func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `DELETE FROM app.journal_entries WHERE patient_id = $1`, patientA)

				return err
			})

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
				t.Errorf("%s deleted a day: got %v, want 42501", caller.role, err)
			}
		}
	})
}

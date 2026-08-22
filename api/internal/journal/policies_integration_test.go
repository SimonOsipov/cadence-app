//go:build integration

package journal_test

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	dayB    = "2026-05-30"
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
			VALUES ($1, DATE '2026-05-31', 3, 'manual')
		`, patient); err != nil {
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
		name string
		sql  string
		args []any
	}{
		{
			"creating one for another patient",
			`INSERT INTO app.journal_entries (patient_id, entry_date, mood, source)
			 VALUES ($1, DATE '2026-06-01', 4, 'manual')`,
			[]any{patientB},
		},
		{
			// patient_id is not in the patient's UPDATE grant — half the primary key,
			// and moving a day between patients is a different row rather than an
			// edit of this one. The grant refuses before the policy is consulted.
			"handing their own away",
			`UPDATE app.journal_entries SET patient_id = $1 WHERE patient_id = $2`,
			[]any{patientB, patientA},
		},
	} {
		err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, attempt.sql, attempt.args...)

			return err
		})

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("%s: got %v, want SQLSTATE 42501", attempt.name, err)
		}
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
	}

	// And the other patient's day is untouched, read back on the service path.
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
	if affected := c.changed(t, patientA, "patient", `
		INSERT INTO app.journal_entries (patient_id, entry_date, tags, source)
		VALUES ($1, DATE '2026-06-02', $2::text[], 'manual')
	`, patientA, tags); affected != 1 {
		t.Errorf("the schema refused a tag Go declares: wrote %d rows", affected)
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
				     LATERAL regexp_matches(pg_get_constraintdef(oid), '''([a-z]+)''', 'g') AS literal
				WHERE conname = 'journal_entries_tags_are_a_flat_named_list'
			`).Scan(&accepted)
		},
	); err != nil {
		t.Fatalf("reading the constraint: %v", err)
	}
	if len(accepted) != len(declared) {
		t.Errorf("the schema names %d tags (%v), Go declares %d", len(accepted), accepted, len(declared))
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

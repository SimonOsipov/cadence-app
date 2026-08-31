//go:build integration

package measurements_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// The policy regression suite for the table 000025 adds. Extending it is an
// acceptance rule of the data layer, not a nicety.
//
// The table brings the schema's first patient-held DELETE, and a delete is the
// verb whose refusals are hardest to see: a statement filtered away by a policy
// returns success and zero rows, exactly like one that found nothing. Every
// assertion about it here reads the count of rows affected and then re-reads the
// table as the superuser — twelve of sixteen policy mutations survived a suite
// that took err == nil for a witness.

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
	patientA = "7c4d1a90-0000-4000-8000-0000000000a1"
	patientB = "7c4d1a90-0000-4000-8000-0000000000b1"
	doctorA  = "7c4d1a90-0000-4000-8000-0000000000a2"
	doctorB  = "7c4d1a90-0000-4000-8000-0000000000b2"
	adminID  = "7c4d1a90-0000-4000-8000-0000000000c1"

	seedJob = "test.measurements"
)

// Two patients, and each carries both kinds of row: one they typed in and one
// that arrived from a watch. A fixture with only manual rows cannot tell «their
// own» from «their own and manual», which is the whole of the delete boundary.
type clinic struct {
	service   *pgxpool.Pool
	request   *pgxpool.Pool
	superuser *pgx.Conn

	manual   map[string]string
	imported map[string]string
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

	c := clinic{
		service:   service,
		request:   request,
		superuser: superuser,
		manual:    map[string]string{},
		imported:  map[string]string{},
	}
	if err := database.WithServiceJob(
		t.Context(), service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error { return c.seed(ctx, tx) },
	); err != nil {
		t.Fatalf("seeding the clinic: %v", err)
	}

	return c
}

func (c clinic) seed(ctx context.Context, tx pgx.Tx) error {
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

		var manual string
		if err := tx.QueryRow(ctx, `
			INSERT INTO app.measurements
			    (patient_id, metric, value, unit, measured_at, source)
			VALUES ($1, 'weight', 82.4, 'kg', TIMESTAMPTZ '2026-08-01 07:30:00+05', 'manual')
			RETURNING id::text
		`, patient).Scan(&manual); err != nil {
			return fmt.Errorf("manual reading %s: %w", patient, err)
		}
		c.manual[patient] = manual

		var imported string
		if err := tx.QueryRow(ctx, `
			INSERT INTO app.measurements
			    (patient_id, metric, value, unit, measured_at, source, external_id)
			VALUES ($1, 'hrv', 61, 'ms', TIMESTAMPTZ '2026-08-01 06:00:00+05', 'healthkit', $2)
			RETURNING id::text
		`, patient, "hk-"+patient).Scan(&imported); err != nil {
			return fmt.Errorf("imported reading %s: %w", patient, err)
		}
		c.imported[patient] = imported
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

func (c clinic) refuse(t *testing.T, subject, role, sql string, args ...any) (string, string) {
	t.Helper()

	err := c.as(t, subject, role, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, sql, args...)

		return err
	})
	if err == nil {
		t.Fatal("the statement was accepted")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("refused by something that is not the database: %v", err)
	}

	return pgErr.Code, pgErr.ConstraintName
}

// The witness the request path cannot be: the caller refused a delete cannot see
// the row it did not delete either.
func (c clinic) survives(t *testing.T, id string) bool {
	t.Helper()

	var count int
	if err := c.superuser.QueryRow(
		t.Context(), `SELECT count(*) FROM app.measurements WHERE id = $1`, id,
	).Scan(&count); err != nil {
		t.Fatalf("re-reading %s as the superuser: %v", id, err)
	}

	return count == 1
}

func TestEachSideSeesItsOwnMeasurementsAndNobodyElses(t *testing.T) {
	c := newClinic(t)

	for _, caller := range []struct{ name, subject, role, wants string }{
		{"the patient", patientA, "patient", patientA},
		{"the other patient", patientB, "patient", patientB},
		{"the assigned doctor", doctorA, "doctor", patientA},
		{"the other doctor", doctorB, "doctor", patientB},
	} {
		seen := c.visible(t, caller.subject, caller.role,
			`SELECT DISTINCT patient_id::text FROM app.measurements`)
		if len(seen) != 1 || seen[0] != caller.wants {
			t.Errorf("%s reads %v, want just %s", caller.name, seen, caller.wants)
		}
	}

	// And by key, not only by scan: a policy that filtered a listing but not a
	// point lookup would pass the loop above.
	for _, caller := range []struct{ name, subject, role, other string }{
		{"the patient", patientA, "patient", patientB},
		{"the assigned doctor", doctorA, "doctor", patientB},
	} {
		for what, id := range map[string]string{
			"a hand-typed row": c.manual[caller.other],
			"an imported row":  c.imported[caller.other],
		} {
			seen := c.visible(t, caller.subject, caller.role, fmt.Sprintf(
				`SELECT id::text FROM app.measurements WHERE id = '%s'`, id,
			))
			if len(seen) != 0 {
				t.Errorf("%s reached %s of somebody else by key: %v", caller.name, what, seen)
			}
		}
	}

	// The positive control: without it every refusal above holds against a table
	// nobody can read at all.
	if seen := c.visible(t, patientA, "patient",
		`SELECT id::text FROM app.measurements ORDER BY measured_at`); len(seen) != 2 {
		t.Errorf("the patient reads %v of their own readings, want both", seen)
	}
}

// The indexes, asserted as a set rather than counted: a count is satisfied by two
// indexes on the wrong columns, and the two here answer two different questions —
// the access path every read takes, and the key space the importer will land in.
func TestTheReadingsCarryTheIndexesTheyWereGiven(t *testing.T) {
	c := newClinic(t)

	rows, err := c.superuser.Query(t.Context(), `
		SELECT indexdef FROM pg_indexes
		WHERE schemaname = 'app' AND tablename = 'measurements'
		ORDER BY indexname
	`)
	if err != nil {
		t.Fatalf("reading the indexes: %v", err)
	}
	defer rows.Close()

	var found []string
	for rows.Next() {
		var definition string
		if err := rows.Scan(&definition); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		found = append(found, definition)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating: %v", err)
	}

	// The primary key's is here too, and declared rather than filtered out: it is
	// an index the table carries, and a reader counting «exactly two» should meet
	// the third one here rather than wonder where it went.
	want := []string{
		"CREATE INDEX measurements_by_patient_and_metric ON app.measurements " +
			"USING btree (patient_id, metric, measured_at DESC)",
		"CREATE UNIQUE INDEX measurements_one_per_external_sample ON app.measurements " +
			"USING btree (patient_id, metric, external_id) WHERE (external_id IS NOT NULL)",
		"CREATE UNIQUE INDEX measurements_pkey ON app.measurements USING btree (id)",
	}
	if !slices.Equal(found, want) {
		t.Errorf("indexes:\n got %v\nwant %v", found, want)
	}
}

// The step's own down files, applied to a migrated database.
//
// The chain's schema witness proves the head migration's down file and nothing
// below it, so with the policies on top 000025's rollback is unmeasured: an empty
// down file for it passes the whole gate today, because the unwinding test's final
// assertion is satisfied by the base migration's DROP SCHEMA CASCADE. The files are
// run directly rather than through a step count, which would name the wrong pair
// the moment a 000027 exists.
func TestTheStepsDownFilesRemoveWhatItsUpFilesCreated(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.MigrationURL)
	ctx := t.Context()

	tables := func() int {
		t.Helper()

		var count int
		if err := conn.QueryRow(ctx, `
			SELECT count(*) FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = 'app' AND c.relname = 'measurements' AND c.relkind = 'r'
		`).Scan(&count); err != nil {
			t.Fatalf("looking for the table: %v", err)
		}

		return count
	}
	policies := func() int {
		t.Helper()

		var count int
		if err := conn.QueryRow(ctx, `
			SELECT count(*) FROM pg_policies WHERE schemaname = 'app' AND tablename = 'measurements'
		`).Scan(&count); err != nil {
			t.Fatalf("counting the policies: %v", err)
		}

		return count
	}

	// The controls: without them both assertions below hold against a database the
	// migrations never reached.
	if tables() != 1 {
		t.Fatal("the table is absent before the rollback, so this measures nothing")
	}
	if policies() == 0 {
		t.Fatal("the table carries no policies before the rollback, so this measures nothing")
	}

	apply := func(name string) {
		t.Helper()

		statements, err := os.ReadFile(filepath.Join(testsupport.MigrationsPath(t), name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if _, err := conn.Exec(ctx, string(statements)); err != nil {
			t.Fatalf("applying %s: %v", name, err)
		}
	}

	// All three verbs the patient actually holds, each asked where it was granted:
	// INSERT is by column, so a table-level question answers false for it before the
	// rollback as well as after — and a REVOKE narrowed to SELECT and DELETE would
	// pass a table-only check while leaving a column grant standing on a forced
	// table with no policies. DELETE cannot be asked of a column at all: PostgreSQL
	// has column privileges for SELECT, INSERT, UPDATE and REFERENCES only, and
	// answers 22023 for the rest (measured).
	held := func(verb string) bool {
		t.Helper()

		question := `SELECT has_table_privilege($1, 'app.measurements', $2)`
		if verb == "INSERT" {
			question = `SELECT has_column_privilege($1, 'app.measurements', 'value', $2)`
		}

		var granted bool
		if err := conn.QueryRow(ctx, question, testsupport.PatientRole, verb).Scan(&granted); err != nil {
			t.Fatalf("reading the patient's %s: %v", verb, err)
		}

		return granted
	}
	for _, verb := range []string{"SELECT", "INSERT", "DELETE"} {
		if !held(verb) {
			t.Fatalf("the patient holds no %s before the rollback, so this measures nothing", verb)
		}
	}

	apply("000026_measurements_policies.down.sql")
	if left := policies(); left != 0 {
		t.Errorf("%d policies survived the rollback of the migration that created them", left)
	}
	// The grants and not only the policies, because this is the state an operator
	// undoing one step is left in: a role holding a verb on a forced table with no
	// policy is deny-all in effect and a lie in the registry. 000025's DROP TABLE
	// takes the grants with it, so nothing after this line can see them.
	for _, verb := range []string{"SELECT", "INSERT", "DELETE"} {
		if held(verb) {
			t.Errorf("the patient still holds %s after the rollback of the migration "+
				"that granted it", verb)
		}
	}

	apply("000025_measurements_tables.down.sql")
	if tables() != 0 {
		t.Error("app.measurements survived the rollback of the migration that created it")
	}
}

// The patient has no UPDATE grant at all — asserted rather than assumed, and in
// both places it could hide: the table and every column of it.
func TestAPatientHasNoUpdateGrantOnAMeasurement(t *testing.T) {
	c := newClinic(t)
	ctx := t.Context()

	var onTheTable bool
	if err := c.superuser.QueryRow(
		ctx, `SELECT has_table_privilege($1, 'app.measurements', 'UPDATE')`, testsupport.PatientRole,
	).Scan(&onTheTable); err != nil {
		t.Fatalf("reading the patient's table privilege: %v", err)
	}
	if onTheTable {
		t.Error("the patient holds UPDATE on app.measurements at table level")
	}

	var columns []string
	if err := c.superuser.QueryRow(ctx, `
		SELECT coalesce(array_agg(a.attname ORDER BY a.attname), '{}')
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'app' AND c.relname = 'measurements'
		  AND a.attnum > 0 AND NOT a.attisdropped
		  AND has_column_privilege($1, c.oid, a.attnum, 'UPDATE')
	`, testsupport.PatientRole).Scan(&columns); err != nil {
		t.Fatalf("reading the patient's column privileges: %v", err)
	}
	if len(columns) != 0 {
		t.Errorf("the patient may write %v of a measurement", columns)
	}

	// The control on that query: the same shape against INSERT, where the patient
	// does hold a column list, so that an empty answer above means «no UPDATE»
	// rather than «this query finds nothing». Which columns exactly is the column
	// grant registry's declaration and is not restated here.
	var insertable []string
	if err := c.superuser.QueryRow(ctx, `
		SELECT coalesce(array_agg(a.attname ORDER BY a.attname), '{}')
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'app' AND c.relname = 'measurements'
		  AND a.attnum > 0 AND NOT a.attisdropped
		  AND has_column_privilege($1, c.oid, a.attnum, 'INSERT')
	`, testsupport.PatientRole).Scan(&insertable); err != nil {
		t.Fatalf("reading the patient's insert privileges: %v", err)
	}
	if len(insertable) == 0 {
		t.Fatal("the patient holds no column INSERT either, so the empty UPDATE list " +
			"above measured nothing")
	}

	// And behaviourally, on their own row, which is where a grant would show.
	code, _ := c.refuse(t, patientA, "patient",
		`UPDATE app.measurements SET value = 1 WHERE id = $1`, c.manual[patientA])
	if code != "42501" {
		t.Errorf("rewriting their own reading was refused with %s, want 42501", code)
	}
}

// Both boundaries of the schema's first patient-held DELETE: their own, and the
// ones they typed in.
func TestAPatientDeletesTheirOwnHandTypedReadingAndNothingElse(t *testing.T) {
	t.Run("their own hand-typed row goes", func(t *testing.T) {
		c := newClinic(t)

		if affected := c.changed(t, patientA, "patient",
			`DELETE FROM app.measurements WHERE id = $1`, c.manual[patientA]); affected != 1 {
			t.Errorf("deleting their own hand-typed reading touched %d rows, want 1", affected)
		}
		if c.survives(t, c.manual[patientA]) {
			t.Error("the row the patient deleted is still there")
		}
		if !c.survives(t, c.imported[patientA]) {
			t.Error("their imported reading went with it")
		}
	})

	// The two refusals, and neither can be read off the error: a DELETE the policy
	// filtered away returns success and zero rows in both cases. Naming a row reads
	// a column, so PostgreSQL adds the SELECT policy and the other patient's rows
	// are refused by that rather than by the delete's own USING — the two forms
	// below that read no column are where the USING stands alone, and only they
	// fail when its ownership half is removed (measured).
	for _, refused := range []struct {
		name string
		id   func(clinic) string
	}{
		{"their own imported row", func(c clinic) string { return c.imported[patientA] }},
		{"another patient's hand-typed row", func(c clinic) string { return c.manual[patientB] }},
		{"another patient's imported row", func(c clinic) string { return c.imported[patientB] }},
	} {
		t.Run(refused.name+" stays", func(t *testing.T) {
			c := newClinic(t)
			id := refused.id(c)

			if affected := c.changed(t, patientA, "patient",
				`DELETE FROM app.measurements WHERE id = $1`, id); affected != 0 {
				t.Errorf("deleting %s touched %d rows, want 0", refused.name, affected)
			}
			if !c.survives(t, id) {
				t.Errorf("%s was deleted", refused.name)
			}
		})
	}

	// The three statement forms the vials suite measured: the one that names a row,
	// and the two that read no column, where PostgreSQL adds no SELECT policy and the
	// DELETE's own USING is the only guard.
	for _, form := range []struct {
		name string
		sql  string
		want int64
	}{
		{"naming the other patient", fmt.Sprintf(
			`DELETE FROM app.measurements WHERE patient_id = '%s'`, patientB,
		), 0},
		{"sweeping the table", `DELETE FROM app.measurements`, 1},
		{"with a predicate that reads nothing", `DELETE FROM app.measurements WHERE true`, 1},
	} {
		t.Run(form.name, func(t *testing.T) {
			c := newClinic(t)

			if affected := c.changed(t, patientA, "patient", form.sql); affected != form.want {
				t.Errorf("%s touched %d rows, want %d", form.name, affected, form.want)
			}
			// Every row but the caller's own hand-typed one survives, and the one
			// case that reaches a row reaches exactly that one.
			for what, id := range map[string]string{
				"their own imported row":             c.imported[patientA],
				"the other patient's hand-typed row": c.manual[patientB],
				"the other patient's imported row":   c.imported[patientB],
			} {
				if !c.survives(t, id) {
					t.Errorf("%s: %s was deleted", form.name, what)
				}
			}
			if survived := c.survives(t, c.manual[patientA]); survived == (form.want == 1) {
				t.Errorf("%s: their own hand-typed row survived=%v with %d rows affected",
					form.name, survived, form.want)
			}
		})
	}
}

// The referential action, which runs as the table's owner and consults neither
// grant nor policy nor FORCE. It is CASCADE here on purpose — a person deleted
// takes their readings with them — and the assertion that it is not filtered is
// that the imported row goes too, the one the patient's own DELETE cannot touch.
func TestReadingsGoWhenThePatientDoes(t *testing.T) {
	c := newClinic(t)

	if affected := c.changed(t, adminID, "admin",
		`DELETE FROM app.profiles WHERE user_id = $1`, patientA); affected != 1 {
		t.Fatalf("deleting the profile touched %d rows, want 1", affected)
	}

	for what, id := range map[string]string{
		"the hand-typed reading": c.manual[patientA],
		"the imported reading":   c.imported[patientA],
	} {
		if c.survives(t, id) {
			t.Errorf("%s outlived the patient it belongs to", what)
		}
	}
	for what, id := range map[string]string{
		"the other patient's hand-typed reading": c.manual[patientB],
		"the other patient's imported reading":   c.imported[patientB],
	} {
		if !c.survives(t, id) {
			t.Errorf("%s went with somebody else's profile", what)
		}
	}
}

// Every closed set on the row, in both directions. A CHECK narrower than §03 —
// seven metrics, or a unit set that admits any of the six spellings against any
// metric — refuses nothing a refusal test tries and ships green.
func TestEachRowShapeRuleOnAReadingFires(t *testing.T) {
	c := newClinic(t)

	for _, rule := range []struct{ name, columns, values, constraint string }{
		{
			"a metric outside the eight", "metric, value, unit",
			"'thigh', 54, 'cm'", "measurements_metric_check",
		},
		{
			"a source outside the three", "metric, value, unit, source",
			"'weight', 82, 'kg', 'fitbit'", "measurements_source_check",
		},
		{
			// The pair, and this is the case a unit column with a closed set of its
			// own would accept: 'cm' is a unit this table uses, just not this
			// metric's.
			"a metric in another metric's unit", "metric, value, unit",
			"'weight', 82, 'cm'", "measurements_unit_belongs_to_its_metric",
		},
		{
			"a unit no metric uses", "metric, value, unit",
			"'weight', 82, 'lb'", "measurements_unit_belongs_to_its_metric",
		},
		{
			// Each half of the predicate independently: NaN sorts above Infinity in
			// numeric, so it is the upper comparison that refuses it.
			"a value that is not a number", "metric, value, unit",
			"'weight', 'NaN'::numeric, 'kg'", "measurements_value_is_finite",
		},
		{
			"a value of infinity", "metric, value, unit",
			"'weight', 'Infinity'::numeric, 'kg'", "measurements_value_is_finite",
		},
		{
			"a value of minus infinity", "metric, value, unit",
			"'weight', '-Infinity'::numeric, 'kg'", "measurements_value_is_finite",
		},
		{
			"a note of nothing but spaces", "metric, value, unit, note",
			"'weight', 82, 'kg', '   '", "measurements_note_check",
		},
		{
			"a note past its bound", "metric, value, unit, note",
			"'weight', 82, 'kg', pg_catalog.repeat('x', 2001)", "measurements_note_check",
		},
	} {
		t.Run(rule.name, func(t *testing.T) {
			code, name := c.refuse(t, adminID, "admin", fmt.Sprintf(`
				INSERT INTO app.measurements (patient_id, measured_at, %s)
				VALUES ('%s', TIMESTAMPTZ '2026-08-02 09:00:00+05', %s)
			`, rule.columns, patientA, rule.values))

			if code != "23514" {
				t.Errorf("refused with %s, want 23514 (check_violation)", code)
			}
			if name != rule.constraint {
				t.Errorf("refused by %q, want %q", name, rule.constraint)
			}
		})
	}

	// The other half of every set, driven from the lists rather than sampled: the
	// eight pairs are the wire contract, and the codes are cross-checked against
	// the KMP enumeration in the next step.
	for metric, unit := range map[string]string{
		"weight": "kg", "hrv": "ms", "rhr": "bpm", "sleep": "/100",
		"bodyfat": "%", "waist": "cm", "hip": "cm", "chest": "cm",
	} {
		t.Run("the pair "+metric+"/"+unit+" is accepted", func(t *testing.T) {
			if affected := c.changed(t, adminID, "admin", `
				INSERT INTO app.measurements (patient_id, metric, value, unit, measured_at)
				VALUES ($1, $2, 42, $3, TIMESTAMPTZ '2026-08-03 09:00:00+05')
			`, patientA, metric, unit); affected != 1 {
				t.Errorf("the pair was refused")
			}
		})
	}

	for _, source := range []string{"manual", "healthkit", "health_connect"} {
		t.Run("the source "+source+" is accepted", func(t *testing.T) {
			if affected := c.changed(t, adminID, "admin", `
				INSERT INTO app.measurements
				    (patient_id, metric, value, unit, measured_at, source)
				VALUES ($1, 'waist', 84, 'cm', TIMESTAMPTZ '2026-08-04 09:00:00+05', $2)
			`, patientA, source); affected != 1 {
				t.Errorf("the source was refused")
			}
		})
	}

	// The other half of the finiteness rule, and the half nothing else measures:
	// what it does not refuse. Narrowed to a plausible range — the per-metric bound
	// the migration declines to write — the CHECK still refuses all three cases
	// above, and without these two the package stays green.
	for name, value := range map[string]string{
		"a reading of zero":      "0",
		"a reading of a million": "1000000",
	} {
		t.Run(name+" is accepted", func(t *testing.T) {
			if affected := c.changed(t, adminID, "admin", `
				INSERT INTO app.measurements (patient_id, metric, value, unit, measured_at)
				VALUES ($1, 'weight', $2::numeric, 'kg', TIMESTAMPTZ '2026-08-08 09:00:00+05')
			`, patientA, value); affected != 1 {
				t.Error("a finite value outside any plausible range was refused")
			}
		})
	}

	// The default, which is what makes withholding the column grant work: a row the
	// patient writes without naming a source is theirs to delete.
	var source string
	if err := c.superuser.QueryRow(t.Context(), `
		INSERT INTO app.measurements (patient_id, metric, value, unit, measured_at)
		VALUES ($1, 'chest', 101, 'cm', TIMESTAMPTZ '2026-08-05 09:00:00+05')
		RETURNING source
	`, patientA).Scan(&source); err != nil {
		t.Fatalf("writing a reading that names no source: %v", err)
	}
	if source != "manual" {
		t.Errorf("a reading that names no source reads %q, want manual", source)
	}
}

func TestTheReachOfEachRoleOverTheReadings(t *testing.T) {
	// The doctor's reach is the assignment and not the role, so it has to move when
	// the assignment does — in both directions, because a policy that only ever
	// widens is the one that leaks a discharged patient.
	t.Run("reassignment moves the doctor's reach both ways", func(t *testing.T) {
		c := newClinic(t)

		before := c.visible(t, doctorB, "doctor",
			`SELECT DISTINCT patient_id::text FROM app.measurements`)
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
			`SELECT DISTINCT patient_id::text FROM app.measurements`); len(after) != 2 {
			t.Errorf("the reassigned doctor reads %v, want both", after)
		}
		if orphaned := c.visible(t, doctorA, "doctor",
			`SELECT patient_id::text FROM app.measurements`); len(orphaned) != 0 {
			t.Errorf("the former doctor still reads %v", orphaned)
		}
	})

	t.Run("a doctor reads the readings and may not write one", func(t *testing.T) {
		c := newClinic(t)

		for _, forbidden := range []struct{ name, sql string }{
			{"rewriting a reading", `UPDATE app.measurements SET value = 1 WHERE patient_id = $1`},
			{"removing one", `DELETE FROM app.measurements WHERE patient_id = $1`},
			{"recording one", `INSERT INTO app.measurements
			     (patient_id, metric, value, unit, measured_at)
			     VALUES ($1, 'weight', 80, 'kg', TIMESTAMPTZ '2026-08-06 09:00:00+05')`},
		} {
			code, _ := c.refuse(t, doctorA, "doctor", forbidden.sql, patientA)
			if code != "42501" {
				t.Errorf("the doctor at %s: got %s, want 42501", forbidden.name, code)
			}
		}
	})

	t.Run("a patient records their own reading and nobody else's", func(t *testing.T) {
		c := newClinic(t)

		if affected := c.changed(t, patientA, "patient", `
			INSERT INTO app.measurements (patient_id, metric, value, unit, measured_at)
			VALUES ($1, 'waist', 88.5, 'cm', TIMESTAMPTZ '2026-08-07 09:00:00+05')
		`, patientA); affected != 1 {
			t.Errorf("the patient wrote %d readings of their own, want 1", affected)
		}

		// Only WITH CHECK stands here: every column is legal and the row is simply
		// not the caller's.
		code, _ := c.refuse(t, patientA, "patient", `
			INSERT INTO app.measurements (patient_id, metric, value, unit, measured_at)
			VALUES ($1, 'waist', 88.5, 'cm', TIMESTAMPTZ '2026-08-07 09:00:00+05')
		`, patientB)
		if code != "42501" {
			t.Errorf("recording a reading as somebody else: got %s, want 42501", code)
		}

		// What withholding the column grant is for, measured rather than left to
		// the registry: a row the patient marks as imported would be one they
		// could then never delete, and the delete boundary would mean nothing.
		code, _ = c.refuse(t, patientA, "patient", `
			INSERT INTO app.measurements
			    (patient_id, metric, value, unit, measured_at, source)
			VALUES ($1, 'waist', 88.5, 'cm', TIMESTAMPTZ '2026-08-07 09:00:00+05', 'healthkit')
		`, patientA)
		if code != "42501" {
			t.Errorf("recording a reading as imported: got %s, want 42501", code)
		}

		var forOther int
		if err := c.superuser.QueryRow(t.Context(), `
			SELECT count(*) FROM app.measurements WHERE patient_id = $1
		`, patientB).Scan(&forOther); err != nil {
			t.Fatalf("counting the other patient's readings: %v", err)
		}
		if forOther != 2 {
			t.Errorf("the other patient now has %d readings, want the seeded 2", forOther)
		}
	})

	t.Run("the admin reaches every reading and may write one", func(t *testing.T) {
		c := newClinic(t)

		if seen := c.visible(t, adminID, "admin",
			`SELECT DISTINCT patient_id::text FROM app.measurements`); len(seen) != 2 {
			t.Errorf("the admin reads %v, want both", seen)
		}
		// The USING half of the admin's ALL, on a patient the admin is assigned to
		// through nobody: a count is what a filtered statement cannot fake.
		if affected := c.changed(t, adminID, "admin",
			`UPDATE app.measurements SET note = 'сверено' WHERE id = $1`,
			c.imported[patientB]); affected != 1 {
			t.Errorf("the admin's update touched %d rows, want 1", affected)
		}
		if affected := c.changed(t, adminID, "admin",
			`DELETE FROM app.measurements WHERE id = $1`, c.imported[patientB]); affected != 1 {
			t.Errorf("the admin's delete touched %d rows, want 1", affected)
		}
		if c.survives(t, c.imported[patientB]) {
			t.Error("the row the admin deleted is still there")
		}
	})

	t.Run("the service path reads and inserts and does not rewrite or delete", func(t *testing.T) {
		c := newClinic(t)

		for _, forbidden := range []struct{ name, sql string }{
			{"rewriting a reading", `UPDATE app.measurements SET value = 1 WHERE patient_id = $1`},
			{"removing one", `DELETE FROM app.measurements WHERE patient_id = $1`},
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
				t.Errorf("the service path at %s: got %v, want 42501", forbidden.name, err)
			}
		}
		if !c.survives(t, c.manual[patientA]) {
			t.Error("the service path removed a reading")
		}
	})

	t.Run("a caller with no identity reaches nothing", func(t *testing.T) {
		c := newClinic(t)

		stranger := "7c4d1a90-0000-4000-8000-00000000dead"
		for _, role := range []string{"patient", "doctor"} {
			if seen := c.visible(t, stranger, role,
				`SELECT id::text FROM app.measurements`); len(seen) != 0 {
				t.Errorf("a stranger as %s read %v", role, seen)
			}
		}

		// And the case the seam cannot be asked for: a real caller blanking their
		// own claims inside their own transaction, which the USERSET context allows.
		if err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
			var before int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.measurements`).Scan(&before); err != nil {
				return err
			}
			if before == 0 {
				t.Fatal("the caller read nothing before blanking, so this measures nothing")
			}
			if _, err := tx.Exec(ctx, `SELECT set_config('request.jwt.claims', '', true)`); err != nil {
				return err
			}

			// Without this the zero below is green for a subject that survived as
			// some other value matching no row — the assertion would pass for a
			// reason other than the one it is named for.
			var subjectIsNull bool
			if err := tx.QueryRow(ctx, `SELECT app.jwt_subject() IS NULL`).Scan(&subjectIsNull); err != nil {
				return err
			}
			if !subjectIsNull {
				t.Fatal("blanking left a subject in place, so the zero below is not the NULL case")
			}

			var after int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.measurements`).Scan(&after); err != nil {
				return err
			}
			if after != 0 {
				t.Errorf("claims blanked: the table still returns %d row(s)", after)
			}

			return nil
		}); err != nil {
			t.Fatalf("blanking the claims: %v", err)
		}
	})
}

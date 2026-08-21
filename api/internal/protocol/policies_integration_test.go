//go:build integration

package protocol_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// The policy regression suite for the four tables 000013 adds. Extending it is an
// acceptance rule of the data layer rather than a nicety: a migration that adds a
// table and not these is not finished.
//
// The rows are written through the service seam, because under forced row level
// security there is no other way they can exist — the owner is refused too. The
// seam is used directly rather than through identity.CreatePatient so that this
// suite measures the protocol policies and not another context's write path.

var cluster *testsupport.Cluster

func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

// os.Exit runs no deferred function and a panicking test never returns through
// m.Run, so the teardown lives in a function of its own.
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

// Two patients and two doctors, assigned crosswise. That shape is what makes «a
// doctor does not see an unassigned patient's course» mean anything: with one
// patient, every row a doctor could read is one they are entitled to.
const (
	patientA = "8a1f3b7c-0000-4000-8000-0000000000a1"
	patientB = "8a1f3b7c-0000-4000-8000-0000000000b1"
	doctorA  = "8a1f3b7c-0000-4000-8000-0000000000a2"
	doctorB  = "8a1f3b7c-0000-4000-8000-0000000000b2"
	adminID  = "8a1f3b7c-0000-4000-8000-0000000000c1"

	seedJob = "test.protocol"
)

type clinic struct {
	service *pgxpool.Pool
	request *pgxpool.Pool
	// The course of each patient, so a visibility assertion names a row rather
	// than counting them.
	courseA string
	courseB string
	itemA   string
	itemB   string
	phaseA  string
	phaseB  string
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

	// The admin is written by the superuser, and it is an exception since 000006
	// rather than a shortcut: the service policies on profiles refuse the value.
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
		func(ctx context.Context, tx pgx.Tx) error { return c.seed(ctx, tx) },
	); err != nil {
		t.Fatalf("seeding the clinic: %v", err)
	}

	return c
}

func (c *clinic) seed(ctx context.Context, tx pgx.Tx) error {
	for id, name := range map[string]string{
		doctorA: "Марина Крылова",
		doctorB: "Ольга Ветрова",
	} {
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
	}

	var compound string
	if err := tx.QueryRow(ctx, `
		INSERT INTO app.compounds (code, name_ru, default_unit, route, icon)
		VALUES ('semaglutide', 'Семаглутид', 'мг', 'п/к', 'syringe')
		RETURNING id::text
	`).Scan(&compound); err != nil {
		return fmt.Errorf("compound: %w", err)
	}

	for _, seeded := range []struct {
		patient string
		course  *string
		item    *string
		phase   *string
	}{
		{patientA, &c.courseA, &c.itemA, &c.phaseA},
		{patientB, &c.courseB, &c.itemB, &c.phaseB},
	} {
		if err := tx.QueryRow(ctx, `
			INSERT INTO app.protocols (patient_id, start_date, duration_weeks, status)
			VALUES ($1, DATE '2026-05-10', 12, 'active')
			RETURNING id::text
		`, seeded.patient).Scan(seeded.course); err != nil {
			return fmt.Errorf("course for %s: %w", seeded.patient, err)
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO app.protocol_items
			    (protocol_id, kind, compound_id, cadence, days_of_week, times, loggable)
			VALUES ($1, 'injection', $2, 'weekly', ARRAY[7]::smallint[], ARRAY['07:00'::time], true)
			RETURNING id::text
		`, *seeded.course, compound).Scan(seeded.item); err != nil {
			return fmt.Errorf("item for %s: %w", seeded.patient, err)
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO app.protocol_phases
			    (protocol_item_id, from_week, to_week, dose_value, dose_unit)
			VALUES ($1, 1, 4, 0.25, 'мг')
			RETURNING id::text
		`, *seeded.item).Scan(seeded.phase); err != nil {
			return fmt.Errorf("phase for %s: %w", seeded.patient, err)
		}
	}

	return nil
}

// as runs fn through the request seam — the only way these policies are ever
// consulted in production.
func (c clinic) as(t *testing.T, subject, role string, fn func(context.Context, pgx.Tx) error) error {
	t.Helper()

	return database.WithCaller(
		t.Context(), c.request, database.Caller{Subject: subject, Role: role}, fn,
	)
}

// visible reads one column as one caller, so a «cannot see» claim is a set rather
// than a count: a count of zero is also what a broken query returns.
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

func assertExactly(t *testing.T, got []string, want, what string) {
	t.Helper()

	if len(got) != 1 || got[0] != want {
		t.Errorf("%s: got %v, want exactly [%s]", what, got, want)
	}
}

// The four tables crosswise, in one test so that a partial arrangement — a
// doctor's course visible but their patient's phases not — cannot pass by being
// split across two.
func TestEachSideSeesItsOwnCourseAndNobodyElses(t *testing.T) {
	c := newClinic(t)

	for _, caller := range []struct {
		name    string
		subject string
		role    string
		course  string
		item    string
		phase   string
	}{
		{"the patient", patientA, "patient", c.courseA, c.itemA, c.phaseA},
		{"the assigned doctor", doctorA, "doctor", c.courseA, c.itemA, c.phaseA},
		{"the other patient", patientB, "patient", c.courseB, c.itemB, c.phaseB},
		{"the other doctor", doctorB, "doctor", c.courseB, c.itemB, c.phaseB},
	} {
		assertExactly(t,
			c.visible(t, caller.subject, caller.role, `SELECT id::text FROM app.protocols`),
			caller.course, caller.name+" reads courses")
		assertExactly(t,
			c.visible(t, caller.subject, caller.role, `SELECT id::text FROM app.protocol_items`),
			caller.item, caller.name+" reads items")
		assertExactly(t,
			c.visible(t, caller.subject, caller.role, `SELECT id::text FROM app.protocol_phases`),
			caller.phase, caller.name+" reads phases")
	}
}

// The relation is the only thing that grants the reach, so moving it moves the
// reach — with no query edited anywhere.
func TestReassigningAPatientMovesTheCourseWithThem(t *testing.T) {
	c := newClinic(t)

	before := c.visible(t, doctorB, "doctor", `SELECT id::text FROM app.protocols`)
	assertExactly(t, before, c.courseB, "the other doctor before the move")

	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			// Struck and reissued rather than updated: the service path holds
			// INSERT and DELETE on assignments and deliberately not UPDATE, so
			// moving a patient is two facts and not an edit to one.
			if _, err := tx.Exec(ctx, `
				DELETE FROM app.care_team_assignments WHERE patient_id = $1
			`, patientA); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `
				INSERT INTO app.care_team_assignments
				    (patient_id, provider_id, care_role, is_primary)
				VALUES ($1, $2, 'endo', true)
			`, patientA, doctorB)

			return err
		},
	); err != nil {
		t.Fatalf("reassigning: %v", err)
	}

	after := c.visible(t, doctorB, "doctor", `SELECT id::text FROM app.protocols ORDER BY id`)
	if len(after) != 2 {
		t.Fatalf("the reassigned doctor reads %v, want both courses", after)
	}

	// And the first doctor lost it in the same movement, which is the half a test
	// asserting only the gain would miss.
	if orphaned := c.visible(t, doctorA, "doctor", `SELECT id::text FROM app.protocols`); len(orphaned) != 0 {
		t.Errorf("the former doctor still reads %v", orphaned)
	}
}

// The request path reads and never writes here: a course is a cross-actor write
// and travels the service seam, which is where the audit row is written too.
func TestNeitherRequestRoleMayWriteAProtocol(t *testing.T) {
	c := newClinic(t)

	for _, caller := range []struct {
		name    string
		subject string
		role    string
	}{
		{"the patient", patientA, "patient"},
		{"the doctor", doctorA, "doctor"},
	} {
		err := c.as(t, caller.subject, caller.role, func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				UPDATE app.protocols SET duration_weeks = 99 WHERE id = $1
			`, c.courseA)

			return err
		})

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("%s rewrote a course: got %v, want SQLSTATE 42501", caller.name, err)
		}
	}
}

// A patient reaching for another patient's row by naming it, rather than by
// listing: a policy that filters a scan and not a lookup would pass the test
// above and fail here.
func TestNamingAnotherPatientsCourseDoesNotFetchIt(t *testing.T) {
	c := newClinic(t)

	// All three tables, not just the shallow one. The deepest is where it matters
	// most: the plan takes the primary key as an index condition and hangs the RLS
	// predicate above it as a filter, so «filters a scan» and «filters a lookup»
	// are different code paths and only one of them was measured.
	for _, reach := range []struct {
		table string
		id    string
	}{
		{"protocols", c.courseB},
		{"protocol_items", c.itemB},
		{"protocol_phases", c.phaseB},
	} {
		for _, caller := range []struct{ subject, role string }{
			{patientA, "patient"},
			{doctorA, "doctor"},
		} {
			seen := c.visible(t, caller.subject, caller.role,
				fmt.Sprintf(`SELECT id::text FROM app.%s WHERE id = '%s'`, reach.table, reach.id))
			if len(seen) != 0 {
				t.Errorf("the %s fetched another patient's %s by id: %v", caller.role, reach.table, seen)
			}
		}
	}
}

// refuse runs one statement on the service path and returns the SQLSTATE and the
// constraint that refused it.
//
// The name is not decoration. Two constraints on one column refuse the same row,
// and which of them fired is the difference between a rule that works and a rule
// that is dead behind a neighbour — measured here once already: array_position
// raised 0A000 before the dimension guard was reached, and a code-only assertion
// could not see it. data-layer invariant 5 asks the same of 42501, which has
// three possible causes.
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

// The rule that has to be the database's, because Go cannot see the race: two
// transactions can each check for an overlap and each find none.
func TestPhasesOfOneItemMayNotOverlap(t *testing.T) {
	c := newClinic(t)

	// The seeded phase is weeks 1–4. Every shape of overlap with it, and each is
	// asserted separately: an EXCLUDE written with a half-open range accepts the
	// touching case and would pass a test that only tried the contained one.
	for _, overlap := range []struct {
		name     string
		from, to int
	}{
		{"identical", 1, 4},
		{"contained", 2, 3},
		{"straddling the close", 4, 8},
		{"straddling the open", 1, 1},
		{"enclosing", 1, 12},
	} {
		code, name := c.refuse(t, `
			INSERT INTO app.protocol_phases (protocol_item_id, from_week, to_week, dose_value, dose_unit)
			VALUES ($1, $2, $3, 0.5, 'мг')
		`, c.itemA, overlap.from, overlap.to)
		if code != "23P01" || name != "protocol_phases_do_not_overlap" {
			t.Errorf("%s overlap refused by %s/%s, want 23P01/protocol_phases_do_not_overlap",
				overlap.name, code, name)
		}
	}
}

// The other half of the same rule, and the one a constraint written as «phases
// tile the course» would break: a wash-out is a prescription, not an omission.
func TestAGapBetweenPhasesIsAccepted(t *testing.T) {
	c := newClinic(t)

	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			// Weeks 5–8 are left prescribed nothing, deliberately.
			_, err := tx.Exec(ctx, `
				INSERT INTO app.protocol_phases (protocol_item_id, from_week, to_week, dose_value, dose_unit)
				VALUES ($1, 9, 12, 1.0, 'мг')
			`, c.itemA)

			return err
		},
	); err != nil {
		t.Fatalf("a wash-out was refused: %v", err)
	}
}

// The invariant the client's ProtocolPlan rests on: it takes exactly one course,
// so two active ones would make its shape wrong rather than merely surprising.
func TestAPatientHasOneActiveProtocol(t *testing.T) {
	c := newClinic(t)

	code, name := c.refuse(t, `
		INSERT INTO app.protocols (patient_id, start_date, duration_weeks, status)
		VALUES ($1, DATE '2026-09-01', 8, 'active')
	`, patientA)
	if code != "23505" || name != "protocols_one_active_per_patient" {
		t.Errorf("a second active course refused by %s/%s, want 23505/protocols_one_active_per_patient",
			code, name)
	}

	// Partial, and this is what makes that word mean something: the history of
	// finished courses is not bounded at one.
	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			for _, status := range []string{"completed", "cancelled"} {
				if _, err := tx.Exec(ctx, `
					INSERT INTO app.protocols (patient_id, start_date, duration_weeks, status)
					VALUES ($1, DATE '2025-01-01', 8, $2)
				`, patientA, status); err != nil {
					return err
				}
			}

			return nil
		},
	); err != nil {
		t.Fatalf("a finished course was refused: %v", err)
	}
}

// Each of the row-shape rules, named by the constraint that has to fire — the
// name and not only the code, because two constraints on one column refuse the
// same row and only the name says which. That is not hypothetical here: a
// dimension guard on `times` was written as its own constraint and was dead
// behind its neighbour, sorted first by name, raising 0A000 before it.
func TestTheRowShapeRulesEachFire(t *testing.T) {
	c := newClinic(t)

	for _, rule := range []struct {
		name       string
		sql        string
		arg        string
		constraint string
	}{
		{
			// The generator answers «not prescribed» for a day before the course,
			// so a band opening at week 0 would be drawn over days the schedule
			// denies.
			"a phase before week one", `
			INSERT INTO app.protocol_phases (protocol_item_id, from_week, to_week, dose_value, dose_unit)
			VALUES ($1, 0, 0, 0.25, 'мг')`, "item", "protocol_phases_from_week_check",
		},
		{
			"a phase running backwards", `
			INSERT INTO app.protocol_phases (protocol_item_id, from_week, to_week, dose_value, dose_unit)
			VALUES ($1, 8, 5, 0.25, 'мг')`, "item", "protocol_phases_runs_forwards",
		},
		{
			"a daily item that also names weekdays", `
			INSERT INTO app.protocol_items (protocol_id, kind, cadence, days_of_week, times)
			VALUES ($1, 'supplement', 'daily', ARRAY[1,3]::smallint[], ARRAY['21:30'::time])`,
			"course", "protocol_items_cadence_matches_days",
		},
		{
			"a weekly item that names none", `
			INSERT INTO app.protocol_items (protocol_id, kind, cadence, days_of_week, times)
			VALUES ($1, 'weigh_in', 'weekly', '{}'::smallint[], ARRAY['09:00'::time])`,
			"course", "protocol_items_cadence_matches_days",
		},
		{
			// Zero is what Go's time.Weekday gives for Sunday, which is exactly the
			// mistake the ISO helper in calendar.go exists to prevent.
			"a weekday outside the ISO range", `
			INSERT INTO app.protocol_items (protocol_id, kind, cadence, days_of_week, times)
			VALUES ($1, 'weigh_in', 'weekly', ARRAY[0]::smallint[], ARRAY['09:00'::time])`,
			"course", "protocol_items_days_are_iso",
		},
		{
			"an item with no slot", `
			INSERT INTO app.protocol_items (protocol_id, kind, cadence, days_of_week, times)
			VALUES ($1, 'weigh_in', 'weekly', ARRAY[7]::smallint[], '{}'::time[])`,
			"course", "protocol_items_has_a_slot",
		},
		{
			"two compounds whose names differ only in case", `
			INSERT INTO app.compounds (name_ru, default_unit, route, icon)
			VALUES ('семаглутид', 'мг', 'п/к', 'syringe')`, "", "compounds_one_row_per_name",
		},
	} {
		t.Run(rule.name, func(t *testing.T) {
			code, name := c.refuse(t, rule.sql, c.argsFor(rule.arg)...)
			want := "23514"
			if rule.constraint == "compounds_one_row_per_name" {
				want = "23505"
			}
			if code != want || name != rule.constraint {
				t.Errorf("refused by %s/%s, want %s/%s", code, name, want, rule.constraint)
			}
		})
	}
}

// argsFor turns the fixture's name for a parent row into the id, so a refusal
// table stays a table of statements rather than of plumbing.
func (c clinic) argsFor(which string) []any {
	switch which {
	case "course":
		return []any{c.courseB}
	case "item":
		return []any{c.itemB}
	case "":
		return nil
	default:
		return []any{which}
	}
}

// btree_gist is what lets a GiST index compare uuids for equality — the built-in
// GiST covers ranges and geometry and not ordinary scalars. Trusted since
// PostgreSQL 13, which is what lets the chain install it without a superuser, and
// managed Postgres hands nobody one. Measured against the pinned image rather
// than read out of the documentation.
func TestTheExclusionConstraintsExtensionNeedsNoSuperuser(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)

	var trusted bool
	if err := conn.QueryRow(t.Context(), `
		SELECT trusted FROM pg_available_extension_versions
		WHERE name = 'btree_gist' AND installed
	`).Scan(&trusted); err != nil {
		t.Fatalf("reading the extension: %v", err)
	}
	if !trusted {
		t.Error("btree_gist is not trusted on this image: the chain would need a superuser")
	}
}

// The service path carries no row predicate — that authorization lives in Go —
// so what stops it moving a row across a patient boundary in place is the grant,
// and a grant is the only tool that can: WITH CHECK sees no OLD row, and
// «patient_id did not change» is therefore unsayable as a policy.
//
// Measured before it was closed: one UPDATE moved another patient's item onto
// this patient's course, and the patient then read the moved row through their
// own policy, which is exactly correct policy behaviour on a row that now
// belongs to them.
//
// In place, and not more than that. Copy-and-delete reaches the same end state
// with the verbs the write path needs, and no grant can narrow DELETE — it has no
// column form. That half belongs to Go in step 6, and this test does not claim it.
func TestTheServicePathMayNotMoveARowInPlaceToAnotherPatient(t *testing.T) {
	c := newClinic(t)

	for _, move := range []struct {
		name string
		sql  string
		args []any
	}{
		{
			"a course to another patient",
			`UPDATE app.protocols SET patient_id = $1 WHERE id = $2`,
			[]any{patientA, c.courseB},
		},
		{
			"an item to another patient's course",
			`UPDATE app.protocol_items SET protocol_id = $1 WHERE id = $2`,
			[]any{c.courseA, c.itemB},
		},
		{
			"a phase to another patient's item",
			`UPDATE app.protocol_phases SET protocol_item_id = $1 WHERE id = $2`,
			[]any{c.itemA, c.phaseB},
		},
	} {
		if code, _ := c.refuse(t, move.sql, move.args...); code != "42501" {
			t.Errorf("%s: refused with %s, want 42501", move.name, code)
		}
	}

	// The control: the columns the write path actually needs are still writable,
	// so the refusals above are the ownership columns and not a lost grant.
	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `
				UPDATE app.protocols SET status = 'completed', notes = 'закончен' WHERE id = $1
			`, c.courseA); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				UPDATE app.protocol_items SET loggable = false WHERE id = $1
			`, c.itemA); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `
				UPDATE app.protocol_phases SET dose_value = 0.5 WHERE id = $1
			`, c.phaseA)

			return err
		},
	); err != nil {
		t.Fatalf("the write path lost a column it needs: %v", err)
	}
}

// Every closed set is closed, and every value inside it is accepted. The second
// half is what a suite trying only bad values cannot say: a CHECK that refused
// everything would pass one, and so would a CHECK on the wrong column — the
// «daily item that also names weekdays» case reports 23514 whether the cadence
// set is spelled right or not.
//
// The accepted values are the ones internal/protocol/types.go declares, which is
// the pairing between the Go constants and the schema that nothing else asserts.
func TestEveryClosedSetAcceptsItsOwnValuesAndRefusesOthers(t *testing.T) {
	c := newClinic(t)

	t.Run("every kind and cadence and unit is accepted", func(t *testing.T) {
		if err := database.WithServiceJob(
			t.Context(), c.service, seedJob,
			func(ctx context.Context, tx pgx.Tx) error { return acceptEveryValue(ctx, tx, c) },
		); err != nil {
			t.Fatalf("a value the schema declares was refused: %v", err)
		}
	})

	for _, refused := range []struct {
		name       string
		sql        string
		arg        string
		constraint string
	}{
		{"a compound unit outside the set", `
			INSERT INTO app.compounds (name_ru, default_unit, route, icon)
			VALUES ('Тирзепатид', 'ml', 'п/к', 'vial')`, "", "compounds_default_unit_check"},
		{"a course status outside the set", `
			INSERT INTO app.protocols (patient_id, start_date, duration_weeks, status)
			VALUES ($1, DATE '2026-01-01', 8, 'paused')`, patientB, "protocols_status_check"},
		{"a course longer than the bound", `
			INSERT INTO app.protocols (patient_id, start_date, duration_weeks, status)
			VALUES ($1, DATE '2026-01-01', 105, 'completed')`, patientB, "protocols_duration_weeks_check"},
		{"a course of no weeks at all", `
			INSERT INTO app.protocols (patient_id, start_date, duration_weeks, status)
			VALUES ($1, DATE '2026-01-01', 0, 'completed')`, patientB, "protocols_duration_weeks_check"},
		{"an item kind outside the set", `
			INSERT INTO app.protocol_items (protocol_id, kind, cadence, days_of_week, times)
			VALUES ($1, 'infusion', 'weekly', ARRAY[7]::smallint[], ARRAY['09:00'::time])`, "course", "protocol_items_kind_check"},
		{"a cadence outside the set", `
			INSERT INTO app.protocol_items (protocol_id, kind, cadence, days_of_week, times)
			VALUES ($1, 'weigh_in', 'fortnightly', ARRAY[7]::smallint[], ARRAY['09:00'::time])`, "course", "protocol_items_cadence_check"},
		{"a dose unit outside the set", `
			INSERT INTO app.protocol_phases (protocol_item_id, from_week, to_week, dose_value, dose_unit)
			VALUES ($1, 5, 8, 1, 'iu')`, "item", "protocol_phases_dose_unit_check"},
		{"a dose of nothing", `
			INSERT INTO app.protocol_phases (protocol_item_id, from_week, to_week, dose_value, dose_unit)
			VALUES ($1, 5, 8, 0, 'мг')`, "item", "protocol_phases_dose_value_check"},
		{"a slot that is NULL", `
			INSERT INTO app.protocol_items (protocol_id, kind, cadence, days_of_week, times)
			VALUES ($1, 'weigh_in', 'weekly', ARRAY[7]::smallint[], ARRAY[NULL]::time[])`, "course", "protocol_items_slots_are_a_named_flat_list"},
		{"a weekday list of two dimensions", `
			INSERT INTO app.protocol_items (protocol_id, kind, cadence, days_of_week, times)
			VALUES ($1, 'weigh_in', 'weekly', ARRAY[ARRAY[1,2],ARRAY[3,4]]::smallint[],
			        ARRAY['09:00'::time])`, "course", "protocol_items_days_are_a_flat_list"},
		{"a slot list of two dimensions", `
			INSERT INTO app.protocol_items (protocol_id, kind, cadence, days_of_week, times)
			VALUES ($1, 'weigh_in', 'weekly', ARRAY[7]::smallint[],
			        ARRAY[ARRAY['08:00'::time],ARRAY['09:00'::time]])`, "course", "protocol_items_slots_are_a_named_flat_list"},
		{"a compound with an empty name", `
			INSERT INTO app.compounds (name_ru, default_unit, route, icon)
			VALUES ('', 'мг', 'п/к', 'vial')`, "", "compounds_name_ru_check"},
	} {
		t.Run(refused.name, func(t *testing.T) {
			code, name := c.refuse(t, refused.sql, c.argsFor(refused.arg)...)
			if code != "23514" || name != refused.constraint {
				t.Errorf("refused by %s/%s, want 23514/%s", code, name, refused.constraint)
			}
		})
	}
}

func acceptEveryValue(ctx context.Context, tx pgx.Tx, c clinic) error {
	var mcg string
	if err := tx.QueryRow(ctx, `
		INSERT INTO app.compounds (name_ru, default_unit, route, icon)
		VALUES ('BPC-157', 'мкг', 'п/к', 'vial')
		RETURNING id::text
	`).Scan(&mcg); err != nil {
		return fmt.Errorf("мкг as a default unit: %w", err)
	}

	for _, shape := range []struct {
		kind    string
		cadence string
		days    string
	}{
		{"injection", "daily", `'{}'::smallint[]`},
		{"supplement", "n_per_week", `ARRAY[1,3,5]::smallint[]`},
		{"weigh_in", "weekly", `ARRAY[7]::smallint[]`},
	} {
		var item string
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO app.protocol_items
			    (protocol_id, kind, compound_id, cadence, days_of_week, times)
			VALUES ($1, $2, $3, $4, %s, ARRAY['08:00'::time, '20:00'::time])
			RETURNING id::text
		`, shape.days), c.courseB, shape.kind, mcg, shape.cadence).Scan(&item); err != nil {
			return fmt.Errorf("%s / %s: %w", shape.kind, shape.cadence, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.protocol_phases
			    (protocol_item_id, from_week, to_week, dose_value, dose_unit)
			VALUES ($1, 1, 12, 250, 'мкг')
		`, item); err != nil {
			return fmt.Errorf("мкг as a dose unit on %s: %w", shape.kind, err)
		}
	}

	// The two finished statuses, on a patient whose active course the fixture
	// already holds, and the far end of the length bound — otherwise always 12.
	for _, course := range []struct {
		status string
		weeks  int
	}{{"completed", 1}, {"cancelled", 8}, {"completed", 104}} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.protocols (patient_id, start_date, duration_weeks, status)
			VALUES ($1, DATE '2024-01-01', $2, $3)
		`, patientA, course.weeks, course.status); err != nil {
			return fmt.Errorf("status %s over %d weeks: %w", course.status, course.weeks, err)
		}
	}

	return nil
}

// The fourth table, which the step names and the crosswise test above cannot
// cover: a reference has no row predicate to get wrong, so what is worth
// measuring is the column list and the refusal to write.
func TestTheReferenceIsReadableByBothRolesAndWritableByNeither(t *testing.T) {
	c := newClinic(t)

	for _, caller := range []struct{ subject, role string }{
		{patientA, "patient"},
		{doctorA, "doctor"},
	} {
		names := c.visible(t, caller.subject, caller.role, `SELECT name_ru FROM app.compounds`)
		if len(names) != 1 || names[0] != "Семаглутид" {
			t.Errorf("%s reads %v, want the seeded compound", caller.role, names)
		}

		// code and created_at are withheld by a column grant, so asking for
		// either is a privilege refusal and not an empty answer.
		for _, column := range []string{"code", "created_at"} {
			err := c.as(t, caller.subject, caller.role, func(ctx context.Context, tx pgx.Tx) error {
				var value *string
				return tx.QueryRow(ctx, `SELECT `+column+`::text FROM app.compounds`).Scan(&value)
			})

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
				t.Errorf("%s read compounds.%s: got %v, want SQLSTATE 42501", caller.role, column, err)
			}
		}

		err := c.as(t, caller.subject, caller.role, func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.compounds (name_ru, default_unit, route, icon)
				VALUES ('Тирзепатид', 'мг', 'п/к', 'vial')
			`)

			return err
		})

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("%s wrote the reference: got %v, want SQLSTATE 42501", caller.role, err)
		}
	}
}

// The guarantee on protocol_phases is entirely transitive — both its request-role
// bodies mention no subject at all — so what happens in front of a NULL subject is
// the axis that would notice the day protocol_items stops depending on one.
//
// Three cases, and the first version of this test reached only the middle one: it
// passed an empty and an unparseable subject to the seam, which refuses both
// before a transaction opens, and then skipped the error. Twelve of eighteen
// iterations asserted nothing, and the day the third subject stopped being
// accepted the whole test would have gone green having measured nothing at all.
func TestWhatACallerWithoutAnIdentityCanReach(t *testing.T) {
	c := newClinic(t)
	tables := []string{"protocols", "protocol_items", "protocol_phases"}

	t.Run("the seam refuses a subject it cannot use", func(t *testing.T) {
		// Asserted rather than skipped: this is why the cases below cannot be
		// written as callers, and if it stopped being true the cases below would
		// silently change meaning.
		for subject, want := range map[string]error{
			"":           database.ErrNoSubject,
			"not-a-uuid": database.ErrInvalidSubject,
		} {
			err := c.as(t, subject, "patient", func(context.Context, pgx.Tx) error { return nil })
			if !errors.Is(err, want) {
				t.Errorf("subject %q: got %v, want %v", subject, err, want)
			}
		}
	})

	t.Run("a stranger with a well-formed subject reads nothing", func(t *testing.T) {
		stranger := "8a1f3b7c-0000-4000-8000-00000000dead"

		for _, role := range []string{"patient", "doctor"} {
			for _, table := range tables {
				if seen := c.visible(t, stranger, role, `SELECT id::text FROM app.`+table); len(seen) != 0 {
					t.Errorf("a stranger as %s read %s: %v", role, table, seen)
				}
			}
			// The control: zero is also what a broken query returns, so the same
			// call with a subject the clinic knows has to be non-empty.
			if mine := c.visible(t, patientA, "patient", `SELECT id::text FROM app.protocols`); len(mine) == 0 {
				t.Fatal("the control read nothing, so the zeroes above measure nothing")
			}
		}
	})

	// The case the seam cannot be asked for and the schema is built against:
	// request.jwt.claims has USERSET context, so a caller who can execute SQL can
	// overwrite it inside their own transaction. What that yields is a NULL
	// subject, and every predicate here is positive equality — a negation would be
	// three-valued and match everything instead of nothing.
	t.Run("a caller who blanks their own claims reads nothing", func(t *testing.T) {
		for _, blanked := range []string{"", "not-json-at-all"} {
			if err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
				var before int
				if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.protocol_phases`).Scan(&before); err != nil {
					return err
				}
				if before == 0 {
					t.Fatal("the caller read nothing before blanking, so this measures nothing")
				}

				if _, err := tx.Exec(ctx, `SELECT set_config('request.jwt.claims', $1, true)`, blanked); err != nil {
					return err
				}

				for _, table := range tables {
					var after int
					if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.`+table).Scan(&after); err != nil {
						return err
					}
					if after != 0 {
						t.Errorf("claims blanked to %q: %s still returns %d row(s)", blanked, table, after)
					}
				}

				return nil
			}); err != nil {
				t.Fatalf("blanking the claims: %v", err)
			}
		}
	})
}

// The admin's four FOR ALL policies, which the registries declare and nothing
// exercised: USING (true) WITH CHECK (true) is also what a policy looks like when
// somebody meant to write a predicate and did not.
//
// All four tables and both halves. The WITH CHECK half is the one a read-only
// test cannot reach at all, and it is the half that decides whether an admin may
// write a row about somebody — which is the whole of what FOR ALL adds.
func TestTheAdminReachesEveryPatientsCourse(t *testing.T) {
	c := newClinic(t)

	for _, table := range []string{"protocols", "protocol_items", "protocol_phases"} {
		seen := c.visible(t, adminID, "admin", `SELECT id::text FROM app.`+table+` ORDER BY id`)
		if len(seen) != 2 {
			t.Errorf("the admin reads %d row(s) of %s, want both patients'", len(seen), table)
		}
		// The control that makes «reaches everyone» mean it: the same call as a
		// patient sees one, so two is the policy and not the fixture.
		if mine := c.visible(t, patientA, "patient", `SELECT id::text FROM app.`+table); len(mine) != 1 {
			t.Errorf("the patient reads %v of %s, want exactly their own", mine, table)
		}
	}

	if names := c.visible(t, adminID, "admin", `SELECT name_ru FROM app.compounds`); len(names) != 1 {
		t.Errorf("the admin reads %v of compounds, want the seeded one", names)
	}

	// The WITH CHECK half, on every one of the four. An admin writing about a
	// patient they have no assignment to is precisely what FOR ALL permits and
	// what nothing here asked for.
	if err := c.as(t, adminID, "admin", func(ctx context.Context, tx pgx.Tx) error {
		var compound string
		if err := tx.QueryRow(ctx, `
			INSERT INTO app.compounds (name_ru, default_unit, route, icon)
			VALUES ('Ретатрутид', 'мг', 'п/к', 'vial') RETURNING id::text
		`).Scan(&compound); err != nil {
			return fmt.Errorf("compounds: %w", err)
		}

		var course string
		if err := tx.QueryRow(ctx, `
			INSERT INTO app.protocols (patient_id, start_date, duration_weeks, status)
			VALUES ($1, DATE '2026-06-01', 8, 'completed') RETURNING id::text
		`, patientB).Scan(&course); err != nil {
			return fmt.Errorf("protocols: %w", err)
		}

		var item string
		if err := tx.QueryRow(ctx, `
			INSERT INTO app.protocol_items (protocol_id, kind, compound_id, cadence, days_of_week, times)
			VALUES ($1, 'injection', $2, 'weekly', ARRAY[3]::smallint[], ARRAY['11:00'::time])
			RETURNING id::text
		`, course, compound).Scan(&item); err != nil {
			return fmt.Errorf("protocol_items: %w", err)
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO app.protocol_phases (protocol_item_id, from_week, to_week, dose_value, dose_unit)
			VALUES ($1, 1, 8, 2, 'мг')
		`, item)

		return err
	}); err != nil {
		t.Fatalf("the admin could not write a course for an unassigned patient: %v", err)
	}
}

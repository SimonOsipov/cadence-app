//go:build integration

package inventory_test

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

// The policy regression suite for the table 000015 adds. Extending it is an
// acceptance rule of the data layer, not a nicety.
//
// This one differs from the protocol suite in the thing worth measuring: a vial is
// the patient writing about themselves, so the request path writes it, and the
// patient's own predicate is the write rule as well as the read filter. What has to
// be proved is therefore not only «cannot read another patient's» but «cannot make
// a row that is somebody else's, and cannot hand their own away».

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

	seedJob = "test.inventory"
)

type clinic struct {
	service  *pgxpool.Pool
	request  *pgxpool.Pool
	compound string
	vialA    string
	vialB    string
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

	c := clinic{service: service, request: request}
	if err := database.WithServiceJob(
		t.Context(), service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error { return c.seed(ctx, tx) },
	); err != nil {
		t.Fatalf("seeding the clinic: %v", err)
	}

	return c
}

// Two patients and two doctors assigned crosswise, and a vial each. With one
// patient every row a caller could read is one they are entitled to.
func (c *clinic) seed(ctx context.Context, tx pgx.Tx) error {
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
	}

	if err := tx.QueryRow(ctx, `
		INSERT INTO app.compounds (code, name_ru, default_unit, route, icon)
		VALUES ('semaglutide', 'Семаглутид', 'мг', 'п/к', 'syringe')
		RETURNING id::text
	`).Scan(&c.compound); err != nil {
		return fmt.Errorf("compound: %w", err)
	}

	for _, seeded := range []struct {
		patient string
		into    *string
	}{{patientA, &c.vialA}, {patientB, &c.vialB}} {
		if err := tx.QueryRow(ctx, `
			INSERT INTO app.vials
			    (patient_id, compound_id, concentration_label, total_doses, expires_on)
			VALUES ($1, $2, '1 мг/мл', 4, DATE '2026-12-31')
			RETURNING id::text
		`, seeded.patient, c.compound).Scan(seeded.into); err != nil {
			return fmt.Errorf("vial for %s: %w", seeded.patient, err)
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

func TestEachSideSeesItsOwnVialAndNobodyElses(t *testing.T) {
	c := newClinic(t)

	for _, caller := range []struct {
		name    string
		subject string
		role    string
		vial    string
	}{
		{"the patient", patientA, "patient", c.vialA},
		{"the assigned doctor", doctorA, "doctor", c.vialA},
		{"the other patient", patientB, "patient", c.vialB},
		{"the other doctor", doctorB, "doctor", c.vialB},
	} {
		seen := c.visible(t, caller.subject, caller.role, `SELECT id::text FROM app.vials`)
		if len(seen) != 1 || seen[0] != caller.vial {
			t.Errorf("%s reads %v, want exactly [%s]", caller.name, seen, caller.vial)
		}

		// By name rather than by listing: a policy that filters a scan and not a
		// lookup passes the assertion above and fails this one.
		other := c.vialB
		if caller.vial == c.vialB {
			other = c.vialA
		}
		if got := c.visible(t, caller.subject, caller.role,
			fmt.Sprintf(`SELECT id::text FROM app.vials WHERE id = '%s'`, other),
		); len(got) != 0 {
			t.Errorf("%s fetched another patient's vial by id: %v", caller.name, got)
		}
	}
}

// The half this table has and the protocol tables do not: the patient writes here,
// so the predicate is the write rule too. Both directions of it — a row may not be
// created as somebody else's, and an owned row may not be handed away.
func TestAPatientMayNotWriteAVialOntoAnotherPatient(t *testing.T) {
	c := newClinic(t)

	for _, attempt := range []struct {
		name string
		sql  string
		args []any
	}{
		{
			"creating one for another patient",
			`INSERT INTO app.vials (patient_id, compound_id, concentration_label, total_doses, expires_on)
			 VALUES ($1, $2, '1 мг/мл', 4, DATE '2026-12-31')`,
			[]any{patientB, c.compound},
		},
		{
			"handing their own away",
			`UPDATE app.vials SET patient_id = $1 WHERE id = $2`,
			[]any{patientB, c.vialA},
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

	// The control: the columns a patient does own are writable, so the refusals
	// above are the predicate and not a lost grant.
	if err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE app.vials SET opened_at = DATE '2026-05-01', location_ru = 'холодильник'
			WHERE id = $1
		`, c.vialA)

		return err
	}); err != nil {
		t.Fatalf("the patient could not open their own vial: %v", err)
	}

	// And a doctor reads their patient's cabinet without writing it: reordering is
	// the patient's own act.
	err := c.as(t, doctorA, "doctor", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE app.vials SET total_doses = 99 WHERE id = $1`, c.vialA)

		return err
	})

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Errorf("the doctor rewrote a vial: got %v, want SQLSTATE 42501", err)
	}
}

// Nobody but the admin may delete one. A vial is disposed of by setting
// disposed_at, because the dose events drawn from it keep pointing at it.
func TestAVialIsDisposedOfRatherThanDeleted(t *testing.T) {
	c := newClinic(t)

	for _, caller := range []struct{ subject, role string }{
		{patientA, "patient"},
		{doctorA, "doctor"},
	} {
		err := c.as(t, caller.subject, caller.role, func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `DELETE FROM app.vials WHERE id = $1`, c.vialA)

			return err
		})

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("%s deleted a vial: got %v, want SQLSTATE 42501", caller.role, err)
		}
	}

	if err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE app.vials SET opened_at = DATE '2026-05-01', disposed_at = DATE '2026-05-20'
			WHERE id = $1
		`, c.vialA)

		return err
	}); err != nil {
		t.Fatalf("the patient could not dispose of their own vial: %v", err)
	}
}

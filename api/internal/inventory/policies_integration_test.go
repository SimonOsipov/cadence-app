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
	adminID  = "8a1f3b7c-0000-4000-8000-0000000000c1"

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

	// The admin is written by the superuser: since 000006 the service policies on
	// profiles refuse the value outright.
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

	// The third form, and the one the two above cannot reach: *taking* somebody
	// else's row. WITH CHECK passes — the row would be A's — so what refuses it is a
	// USING clause, and a USING clause refuses by filtering: a nil error and zero
	// rows. That is why this asserts on the count and on the row read back rather
	// than on an error.
	//
	// Measured, and the conclusion is not the obvious one. PostgreSQL adds the
	// SELECT policies to an UPDATE only when the statement reads a column of the
	// table — a WHERE that names one, or a RETURNING. So for the first form below
	// the two clauses deputise for each other, and for the second and third they do
	// not: nothing is read, vials_own_select never runs, and vials_own_update's
	// USING is the only guard there is. Widening it alone moves every row in the
	// table at once — measured at two, which is every vial the fixture has.
	for _, theft := range []struct {
		name string
		sql  string
		args []any
	}{
		{"naming the row", `UPDATE app.vials SET patient_id = $1 WHERE id = $2`, []any{patientA, c.vialB}},
		// No WHERE at all, and it is the form that matters most: it reads no
		// column, so the SELECT policies are never added to the statement and
		// vials_own_update's USING is the only thing standing in front of it. It is
		// also a sweep rather than a theft — every vial in the table at once.
		{"sweeping the table", `UPDATE app.vials SET patient_id = $1`, []any{patientA}},
		{"with a predicate that reads nothing", `UPDATE app.vials SET patient_id = $1 WHERE true`, []any{patientA}},
	} {
		var affected int64
		if err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
			tag, err := tx.Exec(ctx, theft.sql, theft.args...)
			affected = tag.RowsAffected()

			return err
		}); err != nil {
			t.Fatalf("%s: %v", theft.name, err)
		}
		if affected > 1 {
			t.Errorf("%s: touched %d rows", theft.name, affected)
		}
		if owner := c.ownerOf(t, c.vialB); owner != patientB {
			t.Errorf("%s: the vial now belongs to %s, want %s", theft.name, owner, patientB)
		}
	}

	// The same shape through ON CONFLICT, and it has to name the id or the DO UPDATE
	// arm is unreachable: without it the conflict target is a fresh
	// gen_random_uuid() that collides with nothing, so the statement quietly inserts
	// a third vial and proves nothing. The first version of this test did exactly
	// that.
	//
	// Named, it is refused by the column grant before any policy is consulted —
	// `id` is not the patient's to write — which is a stronger answer than a
	// filtered update and the one worth pinning.
	err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO app.vials (id, patient_id, compound_id, concentration_label, total_doses, expires_on)
			VALUES ($1, $2, $3, '1 мг/мл', 4, DATE '2026-12-31')
			ON CONFLICT (id) DO UPDATE SET patient_id = $2
		`, c.vialB, patientA, c.compound)

		return err
	})

	var conflictErr *pgconn.PgError
	if !errors.As(err, &conflictErr) || conflictErr.Code != "42501" {
		t.Errorf("ON CONFLICT naming another patient's row: got %v, want SQLSTATE 42501", err)
	}
	if owner := c.ownerOf(t, c.vialB); owner != patientB {
		t.Errorf("after ON CONFLICT the vial belongs to %s, want %s", owner, patientB)
	}

	// The positive control the refusals need, and it is not the seed: the seed
	// inserts on the service path, so without this nothing witnesses that the
	// patient's own INSERT policy lets anything through at all. Drop that policy
	// and every refusal above still passes, saying «cannot insert» rather than
	// «cannot insert onto somebody else».
	var mine string
	if err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO app.vials (patient_id, compound_id, concentration_label, total_doses, expires_on)
			VALUES ($1, $2, '2,4 мг/мл', 6, DATE '2027-01-31')
			RETURNING id::text
		`, patientA, c.compound).Scan(&mine)
	}); err != nil {
		t.Fatalf("the patient could not add a vial of their own: %v", err)
	}

	for _, caller := range []struct {
		name    string
		subject string
		role    string
		sees    bool
	}{
		{"its owner", patientA, "patient", true},
		{"their doctor", doctorA, "doctor", true},
		{"the other patient", patientB, "patient", false},
		{"the other doctor", doctorB, "doctor", false},
	} {
		got := c.visible(t, caller.subject, caller.role,
			fmt.Sprintf(`SELECT id::text FROM app.vials WHERE id = '%s'`, mine))
		if caller.sees != (len(got) == 1) {
			t.Errorf("%s sees the new vial: %v, want %v", caller.name, len(got) == 1, caller.sees)
		}
	}

	// And the columns a patient does own are writable, so the refusals above are
	// the predicate and not a lost grant.
	//
	// Asserted on rows affected and on the value read back, because err == nil is
	// not a witness for a write: an UPDATE a policy filters away returns success and
	// touches nothing. data-layer invariant 5 names that hole and requires both
	// halves of the answer, and this is the first request-path write with a row
	// predicate since the identity block — the first place it is live again.
	if affected := c.changed(t, patientA, "patient", `
		UPDATE app.vials SET opened_at = DATE '2026-05-01', location_ru = 'холодильник'
		WHERE id = $1
	`, c.vialA); affected != 1 {
		t.Errorf("the patient opened %d of their own vials, want 1", affected)
	}
	if got := c.locationOf(t, c.vialA); got != "холодильник" {
		t.Errorf("the vial is stored in %q, want «холодильник»", got)
	}

	// The columns that are not theirs to write, refused by the grant rather than by
	// the predicate: the row is their own. Both verbs, because M4 writes a vial at
	// creation and a grant narrowed on one verb only is a grant that is open on the
	// other.
	for _, withheld := range []struct {
		name string
		sql  string
		args []any
	}{
		{
			"rewriting their own key",
			`UPDATE app.vials SET id = pg_catalog.gen_random_uuid() WHERE id = $1`,
			[]any{c.vialA},
		},
		{
			"choosing a key at creation",
			`INSERT INTO app.vials (id, patient_id, compound_id, concentration_label, total_doses, expires_on)
			 VALUES (pg_catalog.gen_random_uuid(), $1, $2, '1 мг/мл', 4, DATE '2026-12-31')`,
			[]any{patientA, c.compound},
		},
		{
			"backdating their own row",
			`UPDATE app.vials SET created_at = pg_catalog.now() WHERE id = $1`,
			[]any{c.vialA},
		},
		{
			"backdating a row at creation",
			`INSERT INTO app.vials (created_at, patient_id, compound_id, concentration_label, total_doses, expires_on)
			 VALUES (pg_catalog.now(), $1, $2, '1 мг/мл', 4, DATE '2026-12-31')`,
			[]any{patientA, c.compound},
		},
	} {
		err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, withheld.sql, withheld.args...)

			return err
		})

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("%s: got %v, want SQLSTATE 42501", withheld.name, err)
		}
	}

	// The photo key is writable — withholding it left it unwritable on every product
	// path — and what stops it pointing at somebody else's object is the schema, not
	// the grant. Both verbs again, and the accept side beside them so that «refused»
	// does not turn out to mean «refused always».
	for _, key := range []struct {
		name    string
		sql     string
		args    []any
		allowed bool
	}{
		{
			"a key under another patient's prefix, at creation",
			`INSERT INTO app.vials
			   (patient_id, compound_id, concentration_label, total_doses, expires_on, label_photo_path)
			 VALUES ($1, $2, '1 мг/мл', 4, DATE '2026-12-31', $3)`,
			[]any{patientA, c.compound, patientB + "/vials/label.jpg"},
			false,
		},
		{
			"a key under another patient's prefix, by edit",
			`UPDATE app.vials SET label_photo_path = $1 WHERE id = $2`,
			[]any{patientB + "/vials/label.jpg", c.vialA},
			false,
		},
		{
			"a key under their own prefix",
			`UPDATE app.vials SET label_photo_path = $1 WHERE id = $2`,
			[]any{patientA + "/vials/label.jpg", c.vialA},
			true,
		},
	} {
		err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, key.sql, key.args...)

			return err
		})

		if key.allowed {
			if err != nil {
				t.Errorf("%s was refused: %v", key.name, err)
			}

			continue
		}

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" ||
			pgErr.ConstraintName != "vials_photo_key_is_under_its_own_prefix" {
			t.Errorf("%s: got %v, want 23514/vials_photo_key_is_under_its_own_prefix", key.name, err)
		}
	}

	// And a doctor reads their patient's cabinet without writing it: reordering is
	// the patient's own act.
	err = c.as(t, doctorA, "doctor", func(ctx context.Context, tx pgx.Tx) error {
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

	if affected := c.changed(t, patientA, "patient", `
		UPDATE app.vials SET opened_at = DATE '2026-05-01', disposed_at = DATE '2026-05-20'
		WHERE id = $1
	`, c.vialA); affected != 1 {
		t.Errorf("the patient disposed of %d of their own vials, want 1", affected)
	}
	if disposed := c.disposedOn(t, c.vialA); disposed != "2026-05-20" {
		t.Errorf("the vial was disposed on %q, want 2026-05-20", disposed)
	}
}

// ownerOf reads a row on the service path, which carries no row predicate — so a
// «the row did not move» assertion is about the row and not about what the caller
// is allowed to see.
func (c clinic) ownerOf(t *testing.T, vial string) string {
	t.Helper()

	var owner string
	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT patient_id::text FROM app.vials WHERE id = $1`, vial).Scan(&owner)
		},
	); err != nil {
		t.Fatalf("reading the vial's owner: %v", err)
	}

	return owner
}

// changed runs one statement as one caller and reports how many rows it touched.
// A count is what a filtered write cannot fake: it returns success and zero.
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

// Read back on the service path, which has no row predicate: «the value is there»
// is then about the row rather than about what the caller may see.
func (c clinic) locationOf(t *testing.T, vial string) string {
	t.Helper()

	return c.serviceString(t, `SELECT coalesce(location_ru, '') FROM app.vials WHERE id = $1`, vial)
}

func (c clinic) disposedOn(t *testing.T, vial string) string {
	t.Helper()

	return c.serviceString(t,
		`SELECT coalesce(to_char(disposed_at, 'YYYY-MM-DD'), '') FROM app.vials WHERE id = $1`, vial)
}

func (c clinic) serviceString(t *testing.T, sql, arg string) string {
	t.Helper()

	var value string
	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, sql, arg).Scan(&value)
		},
	); err != nil {
		t.Fatalf("reading the vial back: %v", err)
	}

	return value
}

// refuse runs one statement on the service path and returns the SQLSTATE and the
// constraint that refused it. The name and not only the code, for the reason the
// protocol suite records: two constraints on one column refuse the same row, and a
// code-only assertion cannot see that one of them is dead behind the other.
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

func (c clinic) insertVial(columns, values string, args ...any) string {
	return fmt.Sprintf(`
		INSERT INTO app.vials (patient_id, compound_id, %s)
		VALUES ($1, $2, %s)
	`, columns, values)
}

func TestEachRowShapeRuleOnAVialFires(t *testing.T) {
	c := newClinic(t)

	for _, rule := range []struct {
		name       string
		columns    string
		values     string
		constraint string
	}{
		{
			"a vial of no doses at all",
			"concentration_label, total_doses, expires_on",
			"'1 мг/мл', 0, DATE '2026-12-31'", "vials_total_doses_check",
		},
		{
			"a vial of a negative number of doses",
			"concentration_label, total_doses, expires_on",
			"'1 мг/мл', -1, DATE '2026-12-31'", "vials_total_doses_check",
		},
		{
			"a concentration with no label on it",
			"concentration_label, total_doses, expires_on",
			"'', 4, DATE '2026-12-31'", "vials_concentration_label_check",
		},
		{
			"an empty lot number",
			"concentration_label, total_doses, expires_on, lot",
			"'1 мг/мл', 4, DATE '2026-12-31', ''", "vials_lot_check",
		},
		{
			"an empty storage location",
			"concentration_label, total_doses, expires_on, location_ru",
			"'1 мг/мл', 4, DATE '2026-12-31', ''", "vials_location_ru_check",
		},
		{
			"a photo key of nothing",
			"concentration_label, total_doses, expires_on, label_photo_path",
			"'1 мг/мл', 4, DATE '2026-12-31', ''", "vials_photo_key_is_under_its_own_prefix",
		},
		{
			"a lot number past its bound",
			"concentration_label, total_doses, expires_on, lot",
			"'1 мг/мл', 4, DATE '2026-12-31', pg_catalog.repeat('x', 51)", "vials_lot_check",
		},
		{
			"a storage location past its bound",
			"concentration_label, total_doses, expires_on, location_ru",
			"'1 мг/мл', 4, DATE '2026-12-31', pg_catalog.repeat('x', 101)", "vials_location_ru_check",
		},
		{
			"a concentration label past its bound",
			"concentration_label, total_doses, expires_on",
			"pg_catalog.repeat('x', 51), 4, DATE '2026-12-31'", "vials_concentration_label_check",
		},
		{
			"thrown away before it was opened",
			"concentration_label, total_doses, expires_on, opened_at, disposed_at",
			"'1 мг/мл', 4, DATE '2026-12-31', DATE '2026-05-20', DATE '2026-05-01'",
			"vials_disposed_after_opening",
		},
	} {
		t.Run(rule.name, func(t *testing.T) {
			code, name := c.refuse(t, c.insertVial(rule.columns, rule.values), patientA, c.compound)
			if code != "23514" || name != rule.constraint {
				t.Errorf("refused by %s/%s, want 23514/%s", code, name, rule.constraint)
			}
		})
	}

	// The exact bounds, on the accept side: without these, every «past its bound»
	// case above passes just as well against a limit of five thousand.
	t.Run("each length bound accepts its own last character", func(t *testing.T) {
		if err := database.WithServiceJob(
			t.Context(), c.service, seedJob,
			func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, c.insertVial(
					"concentration_label, total_doses, expires_on, lot, location_ru",
					"pg_catalog.repeat('x', 50), 4, DATE '2026-12-31', "+
						"pg_catalog.repeat('y', 50), pg_catalog.repeat('z', 100)",
				), patientA, c.compound)

				return err
			},
		); err != nil {
			t.Fatalf("a value exactly at its bound was refused: %v", err)
		}
	})

	// The accept side, which a suite of refusals cannot supply: a CHECK that refused
	// everything would pass every case above. Disposing of a vial that was never
	// opened is legal — stock can expire in its box.
	t.Run("a sealed vial thrown away unopened", func(t *testing.T) {
		if err := database.WithServiceJob(
			t.Context(), c.service, seedJob,
			func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, c.insertVial(
					"concentration_label, total_doses, expires_on, disposed_at, lot, location_ru, label_photo_path",
					fmt.Sprintf("'2,4 мг/мл', 1, DATE '2026-06-01', DATE '2026-05-20', 'L-77', 'холодильник', '%s/b.jpg'", patientA),
				), patientA, c.compound)

				return err
			},
		); err != nil {
			t.Fatalf("a legal vial was refused: %v", err)
		}
	})
}

// The four controls the protocol suite carries and this table needs for the same
// reasons: its doctor predicate is textually the same, and its write surface is
// wider.
func TestTheReachOfEachRoleOverTheCabinet(t *testing.T) {
	// A clinic each, because the first subtest reassigns a patient and the rest
	// would then be reading a world they did not set up.
	t.Run("reassignment moves the doctor's reach both ways", func(t *testing.T) {
		c := newClinic(t)
		before := c.visible(t, doctorB, "doctor", `SELECT id::text FROM app.vials`)
		if len(before) != 1 || before[0] != c.vialB {
			t.Fatalf("the other doctor reads %v before the move, want [%s]", before, c.vialB)
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

		if after := c.visible(t, doctorB, "doctor", `SELECT id::text FROM app.vials`); len(after) != 2 {
			t.Errorf("the reassigned doctor reads %v, want both cabinets", after)
		}
		// The half a test asserting only the gain would miss.
		if orphaned := c.visible(t, doctorA, "doctor", `SELECT id::text FROM app.vials`); len(orphaned) != 0 {
			t.Errorf("the former doctor still reads %v", orphaned)
		}
	})

	t.Run("the admin reaches every cabinet and may write one", func(t *testing.T) {
		c := newClinic(t)
		if seen := c.visible(t, adminID, "admin", `SELECT id::text FROM app.vials`); len(seen) != 2 {
			t.Errorf("the admin reads %v, want both", seen)
		}
		// The WITH CHECK half, and it takes an INSERT to reach it: on an UPDATE,
		// vials_admin's WITH CHECK is `true` and so is its USING, so removing one is
		// indistinguishable from removing the other. Creating a vial for a patient
		// the admin is not assigned to is the half only WITH CHECK decides.
		if err := c.as(t, adminID, "admin", func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.vials (patient_id, compound_id, concentration_label, total_doses, expires_on)
				VALUES ($1, $2, '5 мг/мл', 2, DATE '2027-03-01')
			`, patientB, c.compound)

			return err
		}); err != nil {
			t.Fatalf("the admin could not write a vial for an unassigned patient: %v", err)
		}
		if affected := c.changed(t, adminID, "admin",
			`UPDATE app.vials SET location_ru = 'сейф' WHERE id = $1`, c.vialB); affected != 1 {
			t.Errorf("the admin wrote %d rows of a patient they are unassigned to, want 1", affected)
		}
	})

	t.Run("a caller with no identity reaches nothing", func(t *testing.T) {
		c := newClinic(t)

		// The two forms the seam refuses before a transaction opens, asserted rather
		// than skipped: they are why the cases below have to be written as a
		// stranger and as a blanking, and if they stopped being refused the cases
		// below would silently change meaning.
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
			if seen := c.visible(t, stranger, role, `SELECT id::text FROM app.vials`); len(seen) != 0 {
				t.Errorf("a stranger as %s read %v", role, seen)
			}
		}

		// And the case the seam cannot be asked for: a real caller blanking their own
		// claims inside their own transaction, which the USERSET context of
		// request.jwt.claims permits.
		if err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
			var before int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.vials`).Scan(&before); err != nil {
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
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.vials`).Scan(&after); err != nil {
				return err
			}
			if after != 0 {
				t.Errorf("claims blanked: the cabinet still returns %d row(s)", after)
			}

			return nil
		}); err != nil {
			t.Fatalf("blanking the claims: %v", err)
		}
	})

	t.Run("the service path reads and inserts and does not rewrite", func(t *testing.T) {
		c := newClinic(t)
		// A vial is the patient's own record; nothing on the service path has
		// business editing one, and the grant is what says so.
		err := database.WithServiceJob(
			t.Context(), c.service, seedJob,
			func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `UPDATE app.vials SET total_doses = 99 WHERE id = $1`, c.vialA)

				return err
			},
		)

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("the service path rewrote a vial: got %v, want SQLSTATE 42501", err)
		}
	})
}

//go:build integration

package dosing_test

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

	"github.com/SimonOsipov/cadence-app/api/internal/dosing"
	"github.com/SimonOsipov/cadence-app/api/internal/journal"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// The policy regression suite for the table 000019 adds. Extending it is an
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
	patientA = "9b2f3b7c-0000-4000-8000-0000000000a1"
	patientB = "9b2f3b7c-0000-4000-8000-0000000000b1"
	doctorA  = "9b2f3b7c-0000-4000-8000-0000000000a2"
	doctorB  = "9b2f3b7c-0000-4000-8000-0000000000b2"
	adminID  = "9b2f3b7c-0000-4000-8000-0000000000c1"

	compoundID = "9b2f3b7c-0000-4000-8000-0000000000d1"

	seedJob = "test.dosing"
	dayA    = "2026-05-31"
)

// Each patient's own course, item and vial, so that every reference a dose event
// carries has a twin belonging to somebody else — which is what a test of «may I
// point at that» needs and a one-patient fixture cannot supply.
type clinic struct {
	service *pgxpool.Pool
	request *pgxpool.Pool

	protocol map[string]string
	item     map[string]string
	vial     map[string]string
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
		service:  service,
		request:  request,
		protocol: map[string]string{},
		item:     map[string]string{},
		vial:     map[string]string{},
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

	if _, err := tx.Exec(ctx, `
		INSERT INTO app.compounds (id, name_ru, default_unit, route, icon)
		VALUES ($1, 'Семаглутид', 'мг', 'sc', 'syringe')
	`, compoundID); err != nil {
		return fmt.Errorf("compound: %w", err)
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

		var protocolID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO app.protocols (patient_id, start_date, duration_weeks, status)
			VALUES ($1, DATE '2026-05-04', 12, 'active')
			RETURNING id::text
		`, patient).Scan(&protocolID); err != nil {
			return fmt.Errorf("protocol %s: %w", patient, err)
		}
		c.protocol[patient] = protocolID

		var itemID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO app.protocol_items
			    (protocol_id, kind, compound_id, cadence, days_of_week, times, loggable)
			VALUES ($1, 'injection', $2, 'weekly', ARRAY[7]::smallint[], ARRAY['08:00']::time[], true)
			RETURNING id::text
		`, protocolID, compoundID).Scan(&itemID); err != nil {
			return fmt.Errorf("item %s: %w", patient, err)
		}
		c.item[patient] = itemID

		if _, err := tx.Exec(ctx, `
			INSERT INTO app.protocol_phases (protocol_item_id, from_week, to_week, dose_value, dose_unit)
			VALUES ($1, 1, 12, 0.25, 'мг')
		`, itemID); err != nil {
			return fmt.Errorf("phase %s: %w", patient, err)
		}

		var vialID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO app.vials (patient_id, compound_id, concentration_label, total_doses, expires_on)
			VALUES ($1, $2, '2,4 мг/0,75 мл', 4, DATE '2026-12-31')
			RETURNING id::text
		`, patient, compoundID).Scan(&vialID); err != nil {
			return fmt.Errorf("vial %s: %w", patient, err)
		}
		c.vial[patient] = vialID

		if _, err := tx.Exec(ctx, `
			INSERT INTO app.dose_events
			    (patient_id, protocol_id, protocol_item_id, vial_id, compound_id,
			     scheduled_for_date, scheduled_for_time, injected_at,
			     dose_value, dose_unit, site_code, client_request_id)
			VALUES ($1, $2, $3, $4, $5, $6::date, TIME '08:00',
			        TIMESTAMPTZ '2026-05-31 08:12:00+05',
			        0.25, 'мг', 'l-abdomen', $7)
		`, patient, protocolID, itemID, vialID, compoundID, dayA, "seed-"+patient); err != nil {
			return fmt.Errorf("dose event %s: %w", patient, err)
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

// refuse runs one statement and returns the SQLSTATE and the constraint that
// refused it — the name and not only the code, because two constraints on one
// column refuse the same row and this schema has had one dead behind another.
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

func TestEachSideSeesItsOwnDosesAndNobodyElses(t *testing.T) {
	c := newClinic(t)

	for _, caller := range []struct{ name, subject, role, wants string }{
		{"the patient", patientA, "patient", patientA},
		{"the other patient", patientB, "patient", patientB},
		{"the assigned doctor", doctorA, "doctor", patientA},
		{"the other doctor", doctorB, "doctor", patientB},
	} {
		seen := c.visible(t, caller.subject, caller.role,
			`SELECT patient_id::text FROM app.dose_events`)
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
		if got := c.visible(t, caller.subject, caller.role, fmt.Sprintf(
			`SELECT patient_id::text FROM app.dose_events WHERE patient_id = '%s'`, caller.other,
		)); len(got) != 0 {
			t.Errorf("%s fetched another patient's dose by key: %v", caller.name, got)
		}
	}
}

// The reason the foreign keys are composite. A dose event pointing into another
// patient's course, item or vial would be a row legitimately its owner's — the
// policies accept it, the doctor reads it as their patient's history, and the vial
// it draws down belongs to somebody else. This is step 2's reparenting in a new
// place, and here it is refused by a key rather than by a predicate, which is why
// the service path is measured too: a policy would not be asked there at all.
func TestADoseMayNotPointIntoAnotherPatientsCourse(t *testing.T) {
	c := newClinic(t)

	for _, borrowed := range []struct {
		name       string
		protocol   string
		item       string
		vial       string
		constraint string
	}{
		{
			"another patient's course", c.protocol[patientB], c.item[patientB], "",
			"dose_events_belong_to_their_course",
		},
		{
			// The pair that a single-column key would wave through: this patient's
			// own course named beside another patient's item.
			"an item of another patient's course", c.protocol[patientA], c.item[patientB], "",
			"dose_events_answer_an_item_of_that_course",
		},
		{
			"another patient's vial", c.protocol[patientA], c.item[patientA], c.vial[patientB],
			"dose_events_drawn_from_their_own_vial",
		},
	} {
		insert := `
			INSERT INTO app.dose_events
			    (patient_id, protocol_id, protocol_item_id, vial_id,
			     scheduled_for_date, injected_at, dose_value, dose_unit, client_request_id)
			VALUES ($1, $2, $3, NULLIF($4, '')::uuid, DATE '2026-06-07',
			        TIMESTAMPTZ '2026-06-07 08:00:00+05', 0.25, 'мг', $5)
		`

		t.Run(borrowed.name+", as the patient", func(t *testing.T) {
			code, name := c.refuse(t, patientA, "patient", insert,
				patientA, borrowed.protocol, borrowed.item, borrowed.vial, "borrow-request-1")
			if code != "23503" || name != borrowed.constraint {
				t.Errorf("refused by %s/%s, want 23503/%s", code, name, borrowed.constraint)
			}
		})

		// The whole point of putting this in the schema: the service path has no row
		// predicate at all, so a policy could not have refused this.
		t.Run(borrowed.name+", on the service path", func(t *testing.T) {
			err := database.WithServiceJob(
				t.Context(), c.service, seedJob,
				func(ctx context.Context, tx pgx.Tx) error {
					_, err := tx.Exec(ctx, insert,
						patientA, borrowed.protocol, borrowed.item, borrowed.vial, "borrow-request-2")

					return err
				},
			)

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23503" ||
				pgErr.ConstraintName != borrowed.constraint {
				t.Errorf("the service path: got %v, want 23503/%s", err, borrowed.constraint)
			}
		})
	}

	// The positive control the refusals need: without it they would say «cannot
	// write» rather than «cannot write pointing there».
	if affected := c.changed(t, patientA, "patient", `
		INSERT INTO app.dose_events
		    (patient_id, protocol_id, protocol_item_id, vial_id,
		     scheduled_for_date, injected_at, dose_value, dose_unit, client_request_id)
		VALUES ($1, $2, $3, $4, DATE '2026-06-07',
		        TIMESTAMPTZ '2026-06-07 08:00:00+05', 0.25, 'мг', 'own-request-1')
	`, patientA, c.protocol[patientA], c.item[patientA], c.vial[patientA]); affected != 1 {
		t.Errorf("the patient logged %d doses of their own, want 1", affected)
	}
}

// The idempotency §03 asks for, and its scope. The key is the client's own, so it
// is unique per patient and not globally: two patients whose devices generate the
// same key must not collide, or one of them silently loses a dose.
func TestTheClientKeyIsWhatMakesARepeatOneDose(t *testing.T) {
	c := newClinic(t)

	log := func(patient, protocol, item, key string) error {
		return c.as(t, patient, "patient", func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.dose_events
				    (patient_id, protocol_id, protocol_item_id,
				     scheduled_for_date, injected_at, dose_value, dose_unit, client_request_id)
				VALUES ($1, $2, $3, DATE '2026-06-14',
				        TIMESTAMPTZ '2026-06-14 08:00:00+05', 0.25, 'мг', $4)
			`, patient, protocol, item, key)

			return err
		})
	}

	const key = "retry-queue-42"
	if err := log(patientA, c.protocol[patientA], c.item[patientA], key); err != nil {
		t.Fatalf("the first attempt: %v", err)
	}

	err := log(patientA, c.protocol[patientA], c.item[patientA], key)

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" ||
		pgErr.ConstraintName != "dose_events_one_per_client_key" {
		t.Errorf("the retry: got %v, want 23505/dose_events_one_per_client_key", err)
	}

	// The same key from the other patient's device, which must not be a collision.
	if err := log(patientB, c.protocol[patientB], c.item[patientB], key); err != nil {
		t.Errorf("the other patient's identical key was refused: %v", err)
	}
}

// The race the client key cannot see: two different keys aiming at one slot, which
// is what saving on two devices produces — and the slotless day, which must stay
// open, because two doses of a twice-daily item on one day is a real day.
//
// The slotless half is held by NULL distinctness in a unique index rather than by
// the index's WHERE clause, so removing the clause leaves both halves green.
// Measured, and said here because the two look like the same rule.
func TestTwoKeysAimingAtOneSlotAreOneDose(t *testing.T) {
	c := newClinic(t)

	log := func(key, slot string) error {
		return c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.dose_events
				    (patient_id, protocol_id, protocol_item_id, scheduled_for_date,
				     scheduled_for_time, injected_at, dose_value, dose_unit, client_request_id)
				VALUES ($1, $2, $3, DATE '2026-06-21', NULLIF($4, '')::time,
				        TIMESTAMPTZ '2026-06-21 08:00:00+05', 0.25, 'мг', $5)
			`, patientA, c.protocol[patientA], c.item[patientA], slot, key)

			return err
		})
	}

	if err := log("device-one-0001", "08:00"); err != nil {
		t.Fatalf("the first device: %v", err)
	}
	if err := log("device-two-0002", "08:00"); err == nil {
		t.Error("a second key aimed at the same slot was accepted")
	} else {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != "dose_events_one_per_slot" {
			t.Errorf("the second device: got %v, want dose_events_one_per_slot", err)
		}
	}

	// And the same pair with no slot named, which must go through.
	if err := log("device-one-0003", ""); err != nil {
		t.Fatalf("a slotless dose: %v", err)
	}
	if err := log("device-two-0004", ""); err != nil {
		t.Errorf("a second slotless dose on the same day was refused: %v", err)
	}
}

// The one uniqueness surface here whose key carries no tenant column — and a
// uniqueness check bypasses row security by design, reached at tuple insertion,
// before the referential checks, which are AFTER ROW triggers. So with the slot key
// as it first stood, the two refusals were distinguishable: 23505 meant the other
// patient had logged a dose in that slot, 23503 meant they had not. One bit of
// another patient's injection history per probe, iterable over dates. The diary met
// the same shape one step earlier, through its primary key.
//
// Possessing the other patient's protocol_item_id is the precondition, and RLS never
// discloses it — which is what kept this from being the whole boundary, not what
// closed it. Closed by putting the caller in the key: the probe row then lands in a
// key space of its own and cannot collide, so the ordering stops mattering rather
// than being relied on.
func TestAPatientMayNotProbeAnotherPatientsSlots(t *testing.T) {
	c := newClinic(t)

	// A slot the other patient has taken, and one nobody has. Both must be refused
	// the same way, by the chain, for naming an item that is not the caller's.
	for _, probe := range []struct{ name, date string }{
		{"a slot the other patient took", dayA},
		{"a slot nobody took", "2026-08-09"},
	} {
		t.Run(probe.name, func(t *testing.T) {
			code, name := c.refuse(t, patientA, "patient", `
				INSERT INTO app.dose_events
				    (patient_id, protocol_id, protocol_item_id, scheduled_for_date,
				     scheduled_for_time, injected_at, dose_value, dose_unit, client_request_id)
				VALUES ($1, $2, $3, $4::date, TIME '08:00',
				        TIMESTAMPTZ '2026-08-09 08:00:00+05', 0.25, 'мг', $5)
			`, patientA, c.protocol[patientA], c.item[patientB], probe.date,
				"probe-"+probe.date)
			if code != "23503" || name != "dose_events_answer_an_item_of_that_course" {
				t.Errorf("refused by %s/%s, want 23503/dose_events_answer_an_item_of_that_course",
					code, name)
			}
		})
	}
}

// Every row-shape rule this migration adds, named by the constraint that has to
// fire. The name and not only the SQLSTATE: two constraints on one column refuse
// the same row, and in this schema one has already hidden behind another.
func TestEachRowShapeRuleOnADoseFires(t *testing.T) {
	c := newClinic(t)

	for shape, rule := range []struct {
		name       string
		columns    string
		values     string
		constraint string
		code       string
	}{
		{
			"a dose of nothing", "dose_value, dose_unit",
			"0, 'мг'", "dose_events_dose_value_check", "23514",
		},
		{
			"a dose in a unit nobody prescribes", "dose_value, dose_unit",
			"0.25, 'ме'", "dose_events_dose_unit_check", "23514",
		},
		{
			// The zone list is §03's three plus the body map's seven, and this is
			// what stops an eighth from being invented at the transport.
			"a zone the body map cannot draw", "dose_value, dose_unit, site_code",
			"0.25, 'мг', 'l-flank'", "dose_events_site_code_check", "23514",
		},
		{
			"a mood below the scale", "dose_value, dose_unit, mood",
			"0.25, 'мг', 0", "dose_events_mood_check", "23514",
		},
		{
			"a mood above the scale", "dose_value, dose_unit, mood",
			"0.25, 'мг', 6", "dose_events_mood_check", "23514",
		},
		{
			"a side effect outside the seven", "dose_value, dose_unit, side_effects",
			"0.25, 'мг', ARRAY['dizziness']::text[]",
			"dose_events_side_effects_are_a_flat_named_list", "23514",
		},
		{
			// The flat half of the same constraint, which on flat input is
			// indistinguishable from a bare <@ — measured on the diary, where a
			// nested array of legal names was accepted without this arm.
			"a nest of side effects rather than a list", "dose_value, dose_unit, side_effects",
			"0.25, 'мг', ARRAY[['nausea'],['fatigue']]::text[]",
			"dose_events_side_effects_are_a_flat_named_list", "23514",
		},
		{
			"a note of nothing but spaces", "dose_value, dose_unit, note",
			"0.25, 'мг', '   '", "dose_events_note_check", "23514",
		},
		{
			"a note past its bound", "dose_value, dose_unit, note",
			"0.25, 'мг', pg_catalog.repeat('x', 2001)", "dose_events_note_check", "23514",
		},
		{
			// Shaped and not prefixed: this key starts with the right characters
			// and resolves under path.Join to another patient's object.
			"a photo key that climbs out of its own prefix", "dose_value, dose_unit, photo_path",
			"0.25, 'мг', '" + patientA + "/../" + patientB + "/site.jpg'",
			"dose_events_photo_key_is_under_its_own_prefix", "23514",
		},
		{
			// The half the constraint is named for, and the half nothing measured:
			// every other photo case is decided by the segment shape alone, so a
			// pattern binding the key to any uuid rather than to this patient's
			// agreed with the real one on all of them.
			"a photo key under another patient's prefix", "dose_value, dose_unit, photo_path",
			"0.25, 'мг', '" + patientB + "/site.jpg'",
			"dose_events_photo_key_is_under_its_own_prefix", "23514",
		},
		{
			"a photo key one segment too deep", "dose_value, dose_unit, photo_path",
			"0.25, 'мг', '" + patientA + "/a/b/c/d/e'",
			"dose_events_photo_key_is_under_its_own_prefix", "23514",
		},
		{
			"a photo key with a segment past its bound", "dose_value, dose_unit, photo_path",
			"0.25, 'мг', '" + patientA + "/' || pg_catalog.repeat('x', 102)",
			"dose_events_photo_key_is_under_its_own_prefix", "23514",
		},
		{
			"a client key one character past its bound", "dose_value, dose_unit, client_request_id",
			"0.25, 'мг', pg_catalog.repeat('k', 129)",
			"dose_events_client_request_id_check", "23514",
		},
		{
			"a client key one character short of its bound", "dose_value, dose_unit, client_request_id",
			"0.25, 'мг', pg_catalog.repeat('k', 7)",
			"dose_events_client_request_id_check", "23514",
		},
		{
			"a client key too short to be one", "dose_value, dose_unit, client_request_id",
			"0.25, 'мг', 'abc'", "dose_events_client_request_id_check", "23514",
		},
		{
			"a client key carrying a path", "dose_value, dose_unit, client_request_id",
			"0.25, 'мг', '../secrets'", "dose_events_client_request_id_check", "23514",
		},
	} {
		t.Run(rule.name, func(t *testing.T) {
			// client_request_id is NOT NULL with no default, so every case that does
			// not set it needs one, and it must differ per case or the uniqueness
			// fires first and the rule under test is never reached.
			columns, values := rule.columns, rule.values
			if !strings.Contains(columns, "client_request_id") {
				columns += ", client_request_id"
				values += fmt.Sprintf(", 'shape-%03d'", shape)
			}

			code, name := c.refuse(t, patientA, "patient", fmt.Sprintf(`
				INSERT INTO app.dose_events
				    (patient_id, protocol_id, protocol_item_id, scheduled_for_date,
				     injected_at, %s)
				VALUES ('%s', '%s', '%s', DATE '2026-06-28',
				        TIMESTAMPTZ '2026-06-28 08:00:00+05', %s)
			`, columns, patientA, c.protocol[patientA], c.item[patientA], values))
			if code != rule.code || name != rule.constraint {
				t.Errorf("refused by %s/%s, want %s/%s", code, name, rule.code, rule.constraint)
			}
		})
	}

	// The accept side, at both ends of everything bounded: a CHECK that refused
	// every row would pass every case above.
	for _, day := range []struct {
		name  string
		date  string
		mood  int
		note  int
		key   string
		photo string
	}{
		{"the low end of everything", "2026-06-29", 1, 1, "12345678", "/a.jpg"},
		{
			// 128 exactly, and the segment 101 characters: a bound is only pinned by
			// the value that sits on it. This key was 127 and the refusal was 3, so
			// BETWEEN 4 AND 127 passed the whole suite.
			"the high end of everything", "2026-06-30", 5, 2000,
			"0123456789012345678901234567890123456789012345678901234567890123" +
				"4567890123456789012345678901234567890123456789012345678901234567",
			"/a/b/c/" + strings.Repeat("d", 101),
		},
	} {
		t.Run(day.name, func(t *testing.T) {
			if affected := c.changed(t, patientA, "patient", `
				INSERT INTO app.dose_events
				    (patient_id, protocol_id, protocol_item_id, scheduled_for_date,
				     injected_at, dose_value, dose_unit, site_code, mood, side_effects,
				     note, photo_path, client_request_id)
				VALUES ($1::uuid, $2, $3, $4::date, TIMESTAMPTZ '2026-06-29 08:00:00+05',
				        0.25, 'мг', 'r-lback', $5, ARRAY['nausea','site']::text[],
				        pg_catalog.repeat('x', $6), $7, $8)
			`, patientA, c.protocol[patientA], c.item[patientA], day.date,
				day.mood, day.note, patientA+day.photo, day.key); affected != 1 {
				t.Errorf("a dose at the bounds was refused")
			}
		})
	}
}

// acceptedBy reads the literals a CHECK names, in order. The widest possible
// pattern between the quotes: '([a-z]+)' would silently skip a value with a dash
// in it and report agreement it had not measured.
func (c clinic) acceptedBy(t *testing.T, constraint string) []string {
	t.Helper()

	var accepted []string
	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `
				SELECT array_agg(literal[1] ORDER BY literal[1])
				FROM pg_constraint,
				     LATERAL regexp_matches(pg_get_constraintdef(oid), '''([^'']*)''', 'g') AS literal
				WHERE conname = $1
			`, constraint).Scan(&accepted)
		},
	); err != nil {
		t.Fatalf("reading %s: %v", constraint, err)
	}

	return accepted
}

// The seven are now written in three places — this migration, 000017, and Go — and
// a fact written three times is fixed once unless something ties them. §03 makes
// them one set: the wizard's check-in writes a dose event and a diary entry from one
// action, and a side effect that is a tag in one and not the other is a bug in the
// feed nobody would find by reading either file.
func TestTheSideEffectsAreOneSetAcrossBothTablesAndGo(t *testing.T) {
	c := newClinic(t)

	declared := []string{}
	for _, tag := range []journal.Tag{
		journal.TagNausea, journal.TagFatigue, journal.TagHeadache, journal.TagBloating,
		journal.TagInsomnia, journal.TagSite, journal.TagAppetite,
	} {
		declared = append(declared, string(tag))
	}
	slices.Sort(declared)

	for _, constraint := range []string{
		"dose_events_side_effects_are_a_flat_named_list",
		"journal_entries_tags_are_a_flat_named_list",
	} {
		if accepted := c.acceptedBy(t, constraint); !slices.Equal(accepted, declared) {
			t.Errorf("%s names %v, Go declares %v", constraint, accepted, declared)
		}
	}

	// And the schema takes every one of them here, in one row: a constraint that
	// refused a member would fail this, and one that took anything fails above.
	if affected := c.changed(t, patientA, "patient", `
		INSERT INTO app.dose_events
		    (patient_id, protocol_id, protocol_item_id, scheduled_for_date,
		     injected_at, dose_value, dose_unit, side_effects, client_request_id)
		VALUES ($1, $2, $3, DATE '2026-07-05', TIMESTAMPTZ '2026-07-05 08:00:00+05',
		        0.25, 'мг', $4::text[], 'every-side-effect')
	`, patientA, c.protocol[patientA], c.item[patientA], declared); affected != 1 {
		t.Error("the schema refused a side effect Go declares")
	}
}

// The zone list §03 leaves open with «…». It is the one closed set here whose
// members were read off a frozen prototype rather than a specification, which is
// exactly the kind this project has twice invented values for.
func TestTheZonesTheSchemaAcceptsAreTheOnesGoDeclares(t *testing.T) {
	c := newClinic(t)

	declared := []string{}
	for _, site := range dosing.Sites() {
		declared = append(declared, string(site))
	}
	if len(declared) != 10 {
		t.Fatalf("Go declares %d zones, and §03 says ten", len(declared))
	}
	slices.Sort(declared)

	if accepted := c.acceptedBy(t, "dose_events_site_code_check"); !slices.Equal(accepted, declared) {
		t.Errorf("the schema names %v, Go declares %v", accepted, declared)
	}

	// Every one of them accepted, one row each: the reconciliation above compares
	// text, and this is what says the text means what it looks like.
	for i, site := range dosing.Sites() {
		if affected := c.changed(t, patientA, "patient", `
			INSERT INTO app.dose_events
			    (patient_id, protocol_id, protocol_item_id, scheduled_for_date,
			     injected_at, dose_value, dose_unit, site_code, client_request_id)
			VALUES ($1, $2, $3, DATE '2026-07-06', TIMESTAMPTZ '2026-07-06 08:00:00+05',
			        0.25, 'мг', $4, $5)
		`, patientA, c.protocol[patientA], c.item[patientA], string(site),
			fmt.Sprintf("zone-%03d", i)); affected != 1 {
			t.Errorf("the schema refused the zone %q Go declares", site)
		}
	}
}

// Step 2 left this decision here, and this is the measurement that makes it one.
// protocol_items.compound_id stays editable — a doctor who picked the wrong drug
// fixes the item rather than losing the course — and the history stops riding on it.
func TestADoseKeepsTheDrugItWasGivenAsWhenTheCourseIsEdited(t *testing.T) {
	c := newClinic(t)

	var other string
	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			if err := tx.QueryRow(ctx, `
				INSERT INTO app.compounds (name_ru, default_unit, route, icon)
				VALUES ('Тирзепатид', 'мг', 'sc', 'syringe') RETURNING id::text
			`).Scan(&other); err != nil {
				return err
			}
			_, err := tx.Exec(ctx,
				`UPDATE app.protocol_items SET compound_id = $1 WHERE id = $2`, other, c.item[patientA])

			return err
		},
	); err != nil {
		t.Fatalf("re-prescribing the item: %v", err)
	}

	attributed := c.visible(t, patientA, "patient", `
		SELECT compound_id::text FROM app.dose_events WHERE client_request_id LIKE 'seed-%'`)
	if len(attributed) != 1 || attributed[0] != compoundID {
		t.Errorf("the logged dose is now attributed to %v, want the compound it was given as", attributed)
	}
	if attributed[0] == other {
		t.Error("editing the course rewrote what the patient is recorded as having injected")
	}
}

// The reach of each role over the stream, in the shape the cabinet and the diary
// suites set. A clinic each, because the first subtest reassigns a patient and the
// rest would then be reading a world they did not set up.
func TestTheReachOfEachRoleOverTheDoseStream(t *testing.T) {
	t.Run("reassignment moves the doctor's reach both ways", func(t *testing.T) {
		c := newClinic(t)

		before := c.visible(t, doctorB, "doctor", `SELECT patient_id::text FROM app.dose_events`)
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
			`SELECT patient_id::text FROM app.dose_events`); len(after) != 2 {
			t.Errorf("the reassigned doctor reads %v, want both streams", after)
		}
		if orphaned := c.visible(t, doctorA, "doctor",
			`SELECT patient_id::text FROM app.dose_events`); len(orphaned) != 0 {
			t.Errorf("the former doctor still reads %v", orphaned)
		}
	})

	t.Run("a doctor reads the stream and may not write it", func(t *testing.T) {
		c := newClinic(t)

		for _, forbidden := range []struct{ name, sql string }{
			{"rewriting a dose", `UPDATE app.dose_events SET mood = 1 WHERE patient_id = $1`},
			{"unlogging one", `DELETE FROM app.dose_events WHERE patient_id = $1`},
			{"logging one", `INSERT INTO app.dose_events
			     (patient_id, protocol_id, protocol_item_id, scheduled_for_date,
			      injected_at, dose_value, dose_unit, client_request_id)
			     VALUES ($1, '` + `00000000-0000-4000-8000-000000000000', ` +
				`'00000000-0000-4000-8000-000000000000', DATE '2026-07-12',
			      TIMESTAMPTZ '2026-07-12 08:00:00+05', 0.25, 'мг', 'doctor-write')`},
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

	t.Run("the patient logs and does not unlog or rewrite", func(t *testing.T) {
		c := newClinic(t)

		// A logged dose is a fact: the vial's remaining count is a subtraction over
		// these rows and adherence is read off them, so a mistake is corrected by
		// the clinic rather than by the row leaving.
		for _, forbidden := range []struct{ name, sql string }{
			{"rewriting their own dose", `UPDATE app.dose_events SET mood = 1 WHERE patient_id = $1`},
			{"unlogging their own dose", `DELETE FROM app.dose_events WHERE patient_id = $1`},
		} {
			err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, forbidden.sql, patientA)

				return err
			})

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
				t.Errorf("the patient succeeded at %s: got %v, want 42501", forbidden.name, err)
			}
		}
	})

	t.Run("the admin reaches every stream and may write one", func(t *testing.T) {
		c := newClinic(t)

		if seen := c.visible(t, adminID, "admin",
			`SELECT patient_id::text FROM app.dose_events`); len(seen) != 2 {
			t.Errorf("the admin reads %v, want both", seen)
		}
		if affected := c.changed(t, adminID, "admin", `
			INSERT INTO app.dose_events
			    (patient_id, protocol_id, protocol_item_id, scheduled_for_date,
			     injected_at, dose_value, dose_unit, client_request_id)
			VALUES ($1, $2, $3, DATE '2026-07-19', TIMESTAMPTZ '2026-07-19 08:00:00+05',
			        0.25, 'мг', 'admin-written')
		`, patientB, c.protocol[patientB], c.item[patientB]); affected != 1 {
			t.Error("the admin could not log a dose for an unassigned patient")
		}
		// The USING half, on a stream the admin is assigned to nobody through: a
		// count is what a filtered UPDATE cannot fake — success and zero.
		if affected := c.changed(
			t, adminID, "admin",
			`UPDATE app.dose_events SET mood = 2 WHERE client_request_id = 'admin-written'`,
		); affected != 1 {
			t.Errorf("the admin's update touched %d rows, want 1", affected)
		}
	})

	t.Run("a caller with no identity reaches nothing", func(t *testing.T) {
		c := newClinic(t)

		for subject, want := range map[string]error{
			"":           database.ErrNoSubject,
			"not-a-uuid": database.ErrInvalidSubject,
		} {
			if err := c.as(t, subject, "patient", func(context.Context, pgx.Tx) error {
				return nil
			}); !errors.Is(err, want) {
				t.Errorf("subject %q: got %v, want %v", subject, err, want)
			}
		}

		stranger := "9b2f3b7c-0000-4000-8000-00000000dead"
		for _, role := range []string{"patient", "doctor"} {
			if seen := c.visible(t, stranger, role,
				`SELECT patient_id::text FROM app.dose_events`); len(seen) != 0 {
				t.Errorf("a stranger as %s read %v", role, seen)
			}
		}

		// And the case the seam cannot be asked for: a real caller blanking their
		// own claims inside their own transaction, which the USERSET context allows.
		if err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
			var before int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.dose_events`).Scan(&before); err != nil {
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
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.dose_events`).Scan(&after); err != nil {
				return err
			}
			if after != 0 {
				t.Errorf("claims blanked: the stream still returns %d row(s)", after)
			}

			return nil
		}); err != nil {
			t.Fatalf("blanking the claims: %v", err)
		}
	})

	t.Run("the service path reads and inserts and does not rewrite or delete", func(t *testing.T) {
		c := newClinic(t)

		for _, forbidden := range []struct{ name, sql string }{
			{"rewriting a dose", `UPDATE app.dose_events SET mood = 1 WHERE patient_id = $1`},
			{"unlogging one", `DELETE FROM app.dose_events WHERE patient_id = $1`},
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
	})
}

// The theft in the three statement forms the vials suite measured: the one that
// names a row, and the two that read no column, where PostgreSQL never adds the
// SELECT policies and an UPDATE's own USING is the only guard. Here there is no
// patient UPDATE grant at all, so all three are refused as a permission — and that
// is what the assertion says, rather than «42501» which has three causes.
func TestAPatientMayNotReachAnotherPatientsDose(t *testing.T) {
	c := newClinic(t)

	// Logging a dose as somebody else, with that somebody's own course and vial
	// named so the composite keys are all satisfied. This is the one shape the keys
	// cannot refuse — every reference is internally consistent, and the row is
	// simply not the caller's. Only WITH CHECK stands here, and until this test
	// existed only the predicate registry noticed when it was widened.
	// Once with a key nobody owns, and once with the key the other patient's seed
	// row already carries: 000020 claims a borrowed key «collides with nothing and
	// reveals nothing», and only the second case measures it. A uniqueness check
	// bypasses row security, so if it answered first, 23505 would say «that patient
	// used this key» — the shape closed on the slot index above.
	for _, attempt := range []struct{ name, key string }{
		{"with a key nobody owns", "onto-another-1"},
		{"with the key that patient already used", "seed-" + patientB},
	} {
		err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.dose_events
				    (patient_id, protocol_id, protocol_item_id, vial_id,
				     scheduled_for_date, injected_at, dose_value, dose_unit, client_request_id)
				VALUES ($1, $2, $3, $4, DATE '2026-07-26',
				        TIMESTAMPTZ '2026-07-26 08:00:00+05', 0.25, 'мг', $5)
			`, patientB, c.protocol[patientB], c.item[patientB], c.vial[patientB], attempt.key)

			return err
		})

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("logging onto another patient %s: got %v, want 42501", attempt.name, err)

			continue
		}
		// Which of 42501's three causes: the policy, not a missing grant. Granting
		// the column list away would keep a bare code assertion green.
		if !strings.Contains(pgErr.Message, "row-level security") {
			t.Errorf("%s: refused with %q, want the policy to be what refused", attempt.name, pgErr.Message)
		}
	}

	// And the statement form the retry path will actually use, aimed at somebody
	// else: an upsert is the one shape whose arbiter lookup is not RLS-filtered.
	err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO app.dose_events
			    (patient_id, protocol_id, protocol_item_id,
			     scheduled_for_date, injected_at, dose_value, dose_unit, client_request_id)
			VALUES ($1, $2, $3, DATE '2026-07-26',
			        TIMESTAMPTZ '2026-07-26 08:00:00+05', 0.25, 'мг', $4)
			ON CONFLICT (patient_id, client_request_id) DO NOTHING
		`, patientB, c.protocol[patientB], c.item[patientB], "seed-"+patientB)

		return err
	})

	var upsert *pgconn.PgError
	if !errors.As(err, &upsert) || upsert.Code != "42501" {
		t.Errorf("upserting onto another patient: got %v, want 42501", err)
	}

	if seen := c.visible(t, patientB, "patient",
		`SELECT id::text FROM app.dose_events WHERE client_request_id = 'onto-another-1'`,
	); len(seen) != 0 {
		t.Errorf("the other patient's stream gained %v", seen)
	}

	for _, theft := range []struct{ name, sql string }{
		{"naming the row", `UPDATE app.dose_events SET mood = 1 WHERE patient_id = '` + patientB + `'`},
		{"sweeping the table", `UPDATE app.dose_events SET mood = 1`},
		{"with a predicate that reads nothing", `UPDATE app.dose_events SET mood = 1 WHERE true`},
	} {
		err := c.as(t, patientA, "patient", func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, theft.sql)

			return err
		})

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("%s: got %v, want 42501", theft.name, err)

			continue
		}
		if !strings.Contains(pgErr.Message, "permission denied") {
			t.Errorf("%s: refused with %q, want it to name the missing grant", theft.name, pgErr.Message)
		}
	}
}

// What a delete of each parent does to a logged dose. Nothing exercised a
// referential action at all, which is how the item key came to say CASCADE while
// this table's own header said a dose is never deleted by anybody but the admin —
// and a referential action runs as the owner, consulting neither grant nor policy.
func TestALoggedDoseOutlivesTheEditOfItsCourse(t *testing.T) {
	c := newClinic(t)

	// Step 6's course editor is exactly this statement: cadence_service holds DELETE
	// on protocol_items with USING (true).
	for _, edit := range []struct{ name, sql string }{
		{"removing the item it answers", `DELETE FROM app.protocol_items WHERE id = $1`},
	} {
		err := database.WithServiceJob(
			t.Context(), c.service, seedJob,
			func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, edit.sql, c.item[patientA])

				return err
			},
		)

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
			t.Errorf("%s: got %v, want 23503 — the dose history holds the line", edit.name, err)
		}
	}

	if seen := c.visible(t, patientA, "patient",
		`SELECT id::text FROM app.dose_events`); len(seen) != 1 {
		t.Errorf("the patient's stream is %v after the edit, want their logged dose", seen)
	}

	// And the course itself, which only the admin can delete. The grant registry
	// already says why it should not: «a course ends by becoming completed or
	// cancelled, and the dose history hangs off it». Until this case existed the
	// key's action was held by nothing — the item's reach is wider, so the item
	// mutation killed itself and left this one alive.
	err := c.as(t, adminID, "admin", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM app.protocols WHERE id = $1`, c.protocol[patientA])

		return err
	})

	var onCourse *pgconn.PgError
	if !errors.As(err, &onCourse) || onCourse.Code != "23503" ||
		onCourse.ConstraintName != "dose_events_belong_to_their_course" {
		t.Errorf("deleting an injected course: got %v, want 23503/dose_events_belong_to_their_course", err)
	}

	// And the person: deleting a patient must still take their course and their doses
	// with them, which RESTRICT could have blocked — profiles cascades to protocols,
	// protocols would cascade to items, and an item RESTRICTed by a dose event would
	// abort the whole delete. Measured rather than reasoned: which of a referenced
	// table's referential triggers fires first is not something to argue about.
	before := c.countingDoses(t)
	if err := c.as(t, adminID, "admin", func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM app.profiles WHERE user_id = $1`, patientB)

		return err
	}); err != nil {
		t.Fatalf("deleting a patient: %v", err)
	}
	if after := c.countingDoses(t); before != 2 || after != 1 {
		t.Errorf("the stream went from %d doses to %d, want 2 to 1", before, after)
	}
}

func (c clinic) countingDoses(t *testing.T) int {
	t.Helper()

	var doses int
	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT count(*) FROM app.dose_events`).Scan(&doses)
		},
	); err != nil {
		t.Fatalf("counting the doses: %v", err)
	}

	return doses
}

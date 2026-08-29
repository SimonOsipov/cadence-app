//go:build integration

package inventory_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// The units a vial's amount may carry are the dose units, and the schema has to accept
// exactly those. Driven from DoseUnits() rather than a literal pair: a set written twice
// is a set that drifts.
//
// Textual, and the behavioural half lives in the refusal table beside the other row-shape
// rules — reading the definition proves the literals, not that the predicate admits them.
func TestTheAmountUnitsTheSchemaAcceptsAreTheOnesGoDeclares(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)

	var accepted []string
	// conrelid as well as the name: a constraint of the same name on another table
	// would otherwise satisfy this.
	if err := conn.QueryRow(t.Context(), `
		SELECT array_agg(literal[1] ORDER BY literal[1])
		FROM pg_constraint,
		     LATERAL regexp_matches(
		         pg_get_constraintdef(oid), '''([^'']*)''', 'g') AS literal
		WHERE conname = 'vials_amount_unit_check'
		  AND conrelid = 'app.vials'::regclass
	`).Scan(&accepted); err != nil {
		t.Fatalf("reading vials_amount_unit_check: %v", err)
	}

	declared := make([]string, 0, len(protocol.DoseUnits()))
	for _, unit := range protocol.DoseUnits() {
		declared = append(declared, string(unit))
	}
	slices.Sort(declared)

	if !slices.Equal(accepted, declared) {
		t.Errorf("the schema accepts %v, Go declares %v", accepted, declared)
	}
}

// The down migration against rows that use the columns it drops, and against the
// question the chain-level test cannot ask: are the columns actually gone.
//
// TestDownMigrationIsIdempotent walks the whole chain on a base with no data, so it
// proves the statements parse and repeat. It does not prove they remove anything, and
// an empty down file passes it.
func TestTheDownMigrationRemovesTheColumnsAndSparesTheRows(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)
	// Applied through the migration role, like the backfill witness beside it: the two
	// tests in this file should not disagree about who runs a migration, and the first
	// statement over data in either file would be filtered silently if they did.
	migrator := testsupport.Connect(t, db.MigrationURL)
	ctx := t.Context()

	// 000022 sits on top and took total_doses away; the fixture below is the shape the
	// chain had when 000021 was the last migration.
	applyMigration(t, migrator, "000022_the_vial_stops_counting_doses.down.sql")

	patient := seedPatient(t, conn, "Europe/Moscow")
	compound := seedCompound(t, conn, "Семаглутид", "мг")

	// Every new column carries a value: the point is that the drops run over data,
	// which the chain-level test never sees.
	var vial string
	if err := conn.QueryRow(ctx, `
		INSERT INTO app.vials (
			patient_id, compound_id, concentration_label, total_doses,
			total_amount, amount_unit, held_back_at, opened_at, expires_on)
		VALUES ($1, $2, '1 мг/мл', 8, 2.0, 'мг', DATE '2026-08-20', DATE '2026-08-01',
		        DATE '2026-12-01')
		RETURNING id::text
	`, patient, compound).Scan(&vial); err != nil {
		t.Fatalf("seeding the vial: %v", err)
	}

	applyMigration(t, migrator, "000021_vial_holds_an_amount.down.sql")

	for _, column := range []string{"total_amount", "amount_unit", "held_back_at"} {
		var present int
		if err := conn.QueryRow(ctx, `
			SELECT count(*) FROM information_schema.columns
			WHERE table_schema = $1 AND table_name = 'vials' AND column_name = $2
		`, testsupport.AppSchema, column).Scan(&present); err != nil {
			t.Fatalf("reading the columns back: %v", err)
		}
		if present != 0 {
			t.Errorf("%s survived the rollback", column)
		}
	}

	// The one line of the down file that DROP COLUMN does not do for it: the column it
	// constrains stays, so forgetting the line leaves the constraint behind.
	var lingering int
	if err := conn.QueryRow(ctx, `
		SELECT count(*) FROM pg_constraint
		WHERE conname = 'dose_events_dose_value_scale_check'
		  AND conrelid = 'app.dose_events'::regclass
	`).Scan(&lingering); err != nil {
		t.Fatalf("reading the constraint back: %v", err)
	}
	if lingering != 0 {
		t.Error("dose_events_dose_value_scale_check survived the rollback")
	}

	// opened_at included deliberately: the down file decides in prose to leave it
	// alone, because the value it holds was derived from dose events that are still
	// there. Without this column in the read, a rollback blanking every opened_at
	// passes — and after step 3 that turns every started vial back into a sealed one.
	var doses int
	var label, opened string
	if err := conn.QueryRow(ctx, `
		SELECT total_doses, concentration_label, opened_at::text
		FROM app.vials WHERE id = $1
	`, vial).Scan(&doses, &label, &opened); err != nil {
		t.Fatalf("reading the vial back: %v", err)
	}
	if doses != 8 || label != "1 мг/мл" || opened != "2026-08-01" {
		t.Errorf("the rollback changed the row: %d doses, %q, opened %q", doses, label, opened)
	}
}

// The backfills, over rows and under the role that applies them.
//
// Two axes, and both were blind. Every other suite migrates a base with nothing in it,
// so the two UPDATEs always touch zero rows. And FORCE row security makes the result
// role-dependent: the superuser is the one role that never applies the chain and the
// only one for which the statements worked. Applied through MigrationURL, this went red
// against a migration that measured green through SuperuserURL.
func TestTheBackfillsFillWhatTheyClaimAndInventNothing(t *testing.T) {
	db := cluster.NewDatabase(t)
	// Seeded and read through the superuser, because the rows the migration writes are
	// ones the migration role cannot select back; applied through the migration role,
	// because that is the one whose privileges decide whether it writes them at all.
	conn := testsupport.Connect(t, db.SuperuserURL)
	migrator := testsupport.Connect(t, db.MigrationURL)
	ctx := t.Context()

	// Down through both: 000022 dropped total_doses, and the fixtures below are the
	// rows the chain held before 000021 converted them.
	applyMigration(t, migrator, "000022_the_vial_stops_counting_doses.down.sql")
	applyMigration(t, migrator, "000021_vial_holds_an_amount.down.sql")

	// Moscow, because the day of the first draw is the patient's and an injection at
	// 01:00 local is the previous day in UTC — the difference this pins.
	patient := seedPatient(t, conn, "Europe/Moscow")
	semaglutide := seedCompound(t, conn, "Семаглутид", "мг")
	bpc := seedCompound(t, conn, "BPC-157", "мкг")
	course, item := seedCourse(t, conn, patient)

	drawn := seedVial(t, conn, patient, semaglutide, 8)
	untouched := seedVial(t, conn, patient, semaglutide, 4)
	mismatched := seedVial(t, conn, patient, bpc, 10)
	discarded := seedVial(t, conn, patient, semaglutide, 6)
	if _, err := conn.Exec(ctx, `
		UPDATE app.vials SET disposed_at = DATE '2026-05-01' WHERE id = $1
	`, discarded); err != nil {
		t.Fatalf("disposing of a vial: %v", err)
	}

	// 01:00 Moscow on 2 June is 22:00 UTC on 1 June: a UTC day would say the first.
	seedDose(t, conn, patient, course, item, drawn, "2026-06-02 01:00:00+03", "0.25", "мг")
	seedDose(t, conn, patient, course, item, drawn, "2026-06-09 08:00:00+03", "0.5", "мг")
	// A µg compound drawn in мг: no same-unit dose, so no multiplier exists.
	seedDose(t, conn, patient, course, item, mismatched, "2026-06-03 08:00:00+03", "0.25", "мг")
	// Drawn after it was thrown away — the guard the migration needs to survive.
	seedDose(t, conn, patient, course, item, discarded, "2026-06-04 08:00:00+03", "0.25", "мг")

	// Two doses sharing an instant, inserted so that heap order and created_at order
	// disagree: without the second sort key the multiplier is whichever came back first.
	tied := seedVial(t, conn, patient, semaglutide, 8)
	seedDose(t, conn, patient, course, item, tied, "2026-06-05 08:00:00+03", "0.25", "мг")
	seedDose(t, conn, patient, course, item, tied, "2026-06-05 08:00:00+03", "0.5", "мг")
	// The two keys are made to disagree: 0,5 wins on created_at, 0,25 wins on id. So
	// dropping created_at from the order is a deterministic red rather than a coin
	// flip, and dropping id alone leaves the answer unchanged — which is what the
	// third key being a tail-guard means.
	if _, err := conn.Exec(ctx, `
		UPDATE app.dose_events SET
			created_at = CASE dose_value WHEN 0.5 THEN TIMESTAMPTZ '2026-06-05 08:00:00+03'
			                             ELSE          TIMESTAMPTZ '2026-06-05 09:00:00+03' END,
			id = CASE dose_value WHEN 0.5 THEN '00000000-0000-4000-8000-0000000000ff'::uuid
			                     ELSE          '00000000-0000-4000-8000-000000000001'::uuid END
		WHERE vial_id = $1
	`, tied); err != nil {
		t.Fatalf("ordering the tied doses: %v", err)
	}

	applyMigration(t, migrator, "000021_vial_holds_an_amount.up.sql")

	for _, want := range []struct {
		name     string
		vial     string
		amount   string
		unit     string
		openedAt string
	}{
		// 8 × 0,25 мг, and the day is Moscow's.
		{"a vial with same-unit draws", drawn, "2.00", "мг", "2026-06-02"},
		// No draws at all: nothing to multiply by, and a fallback of one would write the
		// injection count into the amount column. The unit is still filled — it comes
		// from the compound, not from a dose — and asserting it kills a backfill
		// narrowed to vials that have events.
		{"a vial nothing was drawn from", untouched, "", "мг", ""},
		// The compound is µg and the only dose is мг — a multiplier exists in the
		// wrong unit, and taking it would be out by a factor of a thousand. It was
		// still drawn from, so it still opened: the two backfills are independent and
		// a missing amount does not make a vial unopened.
		{"a vial whose draws are in another unit", mismatched, "", "мкг", "2026-06-03"},
		// Disposed before the first draw: opened_at must stay empty or
		// vials_disposed_after_opening fails the whole migration.
		{"a vial thrown away before its first dose", discarded, "1.50", "мг", ""},
		// The earlier created_at carries 0,5 — 8 × 0,5, not 8 × 0,25.
		{"two draws sharing an instant", tied, "4.0", "мг", "2026-06-05"},
	} {
		t.Run(want.name, func(t *testing.T) {
			var amount, unit, opened *string
			if err := conn.QueryRow(ctx, `
				SELECT total_amount::text, amount_unit, opened_at::text
				FROM app.vials WHERE id = $1
			`, want.vial).Scan(&amount, &unit, &opened); err != nil {
				t.Fatalf("reading the vial: %v", err)
			}

			if got := deref(amount); got != want.amount {
				t.Errorf("total_amount is %q, want %q", got, want.amount)
			}
			if deref(unit) != want.unit {
				t.Errorf("amount_unit is %q, want %q", deref(unit), want.unit)
			}
			if got := deref(opened); got != want.openedAt {
				t.Errorf("opened_at is %q, want %q", got, want.openedAt)
			}
		})
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

func applyMigration(t *testing.T, conn *pgx.Conn, name string) {
	t.Helper()

	statements, err := os.ReadFile(filepath.Join(testsupport.MigrationsPath(t), name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	if _, err := conn.Exec(t.Context(), string(statements)); err != nil {
		t.Fatalf("applying %s: %v", name, err)
	}
}

func seedPatient(t *testing.T, conn *pgx.Conn, zone string) string {
	t.Helper()

	id := uuid.NewString()
	if _, err := conn.Exec(t.Context(), `
		INSERT INTO app.profiles (user_id, role, full_name, timezone)
		VALUES ($1, 'patient', 'Ирина Соколова', $2)
	`, id, zone); err != nil {
		t.Fatalf("seeding the patient: %v", err)
	}

	return id
}

func seedCompound(t *testing.T, conn *pgx.Conn, name, unit string) string {
	t.Helper()

	var id string
	if err := conn.QueryRow(t.Context(), `
		INSERT INTO app.compounds (name_ru, default_unit, route, icon)
		VALUES ($1, $2, 'подкожно', 'syringe')
		RETURNING id::text
	`, name, unit).Scan(&id); err != nil {
		t.Fatalf("seeding %s: %v", name, err)
	}

	return id
}

func seedCourse(t *testing.T, conn *pgx.Conn, patient string) (string, string) {
	t.Helper()

	ctx := t.Context()

	var course string
	if err := conn.QueryRow(ctx, `
		INSERT INTO app.protocols (patient_id, start_date, duration_weeks, status)
		VALUES ($1, DATE '2026-06-01', 12, 'active')
		RETURNING id::text
	`, patient).Scan(&course); err != nil {
		t.Fatalf("seeding the course: %v", err)
	}

	var item string
	if err := conn.QueryRow(ctx, `
		INSERT INTO app.protocol_items (protocol_id, kind, cadence, days_of_week, times)
		VALUES ($1, 'injection', 'weekly', '{7}', '{08:00}')
		RETURNING id::text
	`, course).Scan(&item); err != nil {
		t.Fatalf("seeding the course item: %v", err)
	}

	return course, item
}

func seedVial(t *testing.T, conn *pgx.Conn, patient, compound string, doses int) string {
	t.Helper()

	var id string
	if err := conn.QueryRow(t.Context(), `
		INSERT INTO app.vials (patient_id, compound_id, concentration_label,
		                       total_doses, expires_on)
		VALUES ($1, $2, '1 мг/мл', $3, DATE '2026-12-01')
		RETURNING id::text
	`, patient, compound, doses).Scan(&id); err != nil {
		t.Fatalf("seeding the vial: %v", err)
	}

	return id
}

func seedDose(
	t *testing.T,
	conn *pgx.Conn,
	patient, course, item, vial, at, value, unit string,
) {
	t.Helper()

	if _, err := conn.Exec(t.Context(), `
		INSERT INTO app.dose_events
		    (patient_id, protocol_id, protocol_item_id, vial_id,
		     scheduled_for_date, injected_at, dose_value, dose_unit, client_request_id)
		VALUES ($1, $2, $3, $4, DATE '2026-06-01', $5::timestamptz, $6::numeric, $7, $8)
	`, patient, course, item, vial, at, value, unit, uuid.NewString()); err != nil {
		t.Fatalf("seeding a dose at %s: %v", at, err)
	}
}

// 000022's rollback, over rows.
//
// The reconstruction is not the inverse of the drop — the number a clinic wrote on a box
// is gone — so it re-derives the count from the first dose drawn in the compound's own
// unit and falls back to one. Both branches run here; without rows the UPDATE touches
// nothing and reports success, which is how the whole statement stayed unmeasured.
func TestRollingBackTheCountReconstructsItFromTheDoses(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)
	migrator := testsupport.Connect(t, db.MigrationURL)
	ctx := t.Context()

	patient := seedPatient(t, conn, "Europe/Moscow")
	semaglutide := seedCompound(t, conn, "Семаглутид", "мг")
	course, item := seedCourse(t, conn, patient)

	bpc := seedCompound(t, conn, "BPC-157", "мкг")

	drawn := seedVialOfAmount(t, conn, patient, semaglutide, "2.0")
	untouched := seedVialOfAmount(t, conn, patient, semaglutide, "1.5")
	mismatched := seedVialOfAmount(t, conn, patient, bpc, "1.5")
	short := seedVialOfAmount(t, conn, patient, semaglutide, "0.1")
	seedDose(t, conn, patient, course, item, drawn, "2026-06-02 08:00:00+03", "0.25", "мг")
	seedDose(t, conn, patient, course, item, short, "2026-06-04 08:00:00+03", "0.25", "мг")
	// A µg compound drawn in мг: the divisor exists but is in the wrong unit, and taking
	// it would reconstruct six doses out of a vial that holds one and a half micrograms.
	seedDose(t, conn, patient, course, item, mismatched, "2026-06-03 08:00:00+03", "0.25", "мг")

	applyMigration(t, migrator, "000022_the_vial_stops_counting_doses.down.sql")

	for _, want := range []struct {
		name  string
		vial  string
		doses int
	}{
		// 2 мг drawn at 0,25 is eight injections — the count the clinic would have
		// written, recovered from the one dose that names the size.
		{"a vial something was drawn from", drawn, 8},
		// Nothing to divide by, so the fallback: one injection, not zero and not the
		// milligrams read as a count.
		{"a vial nothing was drawn from", untouched, 1},
		// The filter is what makes this one fall back rather than divide by a
		// milligram figure: 1.5 / 0.25 would be six.
		{"a vial whose draws are in another unit", mismatched, 1},
		// The clamp, and the only case that reaches it: 0,1 мг at 0,25 rounds to no
		// doses at all, and vials_total_doses_check fails the rollback mid-flight.
		{"a vial holding less than the dose drawn from it", short, 1},
	} {
		t.Run(want.name, func(t *testing.T) {
			var doses int
			if err := conn.QueryRow(ctx,
				`SELECT total_doses FROM app.vials WHERE id = $1`, want.vial).Scan(&doses); err != nil {
				t.Fatalf("reading the count back: %v", err)
			}
			if doses != want.doses {
				t.Errorf("the rollback reconstructed %d doses, want %d", doses, want.doses)
			}
		})
	}

	// The schema half, which the data half above cannot see: the one-step rollback
	// witness in platform/database unwinds the head of the chain, and the head is 000023
	// now — so every line here is unmeasured elsewhere. NOT NULL travels in both
	// directions: the column comes back required and the two 000021 added stop being so.
	for _, restored := range []struct {
		what     string
		question string
	}{
		{
			"total_doses comes back as a required integer",
			`SELECT count(*) = 1 FROM information_schema.columns
			  WHERE table_schema = 'app' AND table_name = 'vials'
			    AND column_name = 'total_doses' AND data_type = 'integer'
			    AND is_nullable = 'NO'`,
		},
		{
			"its CHECK comes back with it",
			`SELECT count(*) = 1 FROM pg_constraint
			  WHERE conname = 'vials_total_doses_check' AND conrelid = 'app.vials'::regclass`,
		},
		{
			"the amount stops being required",
			`SELECT bool_and(is_nullable = 'YES') FROM information_schema.columns
			  WHERE table_schema = 'app' AND table_name = 'vials'
			    AND column_name IN ('total_amount', 'amount_unit')`,
		},
		{
			"the patient may write the count again",
			`SELECT count(*) = 2 FROM information_schema.column_privileges
			  WHERE table_schema = 'app' AND table_name = 'vials'
			    AND column_name = 'total_doses' AND grantee = 'cadence_patient'
			    AND privilege_type IN ('INSERT', 'UPDATE')`,
		},
	} {
		t.Run(restored.what, func(t *testing.T) {
			var holds bool
			if err := conn.QueryRow(ctx, restored.question).Scan(&holds); err != nil {
				t.Fatalf("asking the schema: %v", err)
			}
			if !holds {
				t.Errorf("the rollback left the schema without it")
			}
		})
	}
}

// The tightening is allowed to fail, and that is the design rather than an accident.
//
// 000021 leaves total_amount empty on a vial it could not convert. 000022 then refuses to
// move, which is the signal a human should read — the alternative was inventing a
// multiplier nothing downstream could detect. Without this, the next person to meet the
// red migration «fixes» it with a COALESCE and no test objects.
func TestTheTighteningRefusesAVialItCouldNotConvert(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)
	migrator := testsupport.Connect(t, db.MigrationURL)

	applyMigration(t, migrator, "000022_the_vial_stops_counting_doses.down.sql")
	applyMigration(t, migrator, "000021_vial_holds_an_amount.down.sql")

	patient := seedPatient(t, conn, "Europe/Moscow")
	bpc := seedCompound(t, conn, "BPC-157", "мкг")
	// A µg compound with no dose ever drawn in µg: 000021 has no multiplier for it and
	// deliberately leaves the amount empty.
	seedVial(t, conn, patient, bpc, 10)

	applyMigration(t, migrator, "000021_vial_holds_an_amount.up.sql")

	statements, err := os.ReadFile(filepath.Join(
		testsupport.MigrationsPath(t), "000022_the_vial_stops_counting_doses.up.sql",
	))
	if err != nil {
		t.Fatalf("reading the migration: %v", err)
	}
	if _, err := migrator.Exec(t.Context(), string(statements)); err == nil {
		t.Error("the chain moved on over a vial whose amount nobody could work out")
	}
}

func seedVialOfAmount(t *testing.T, conn *pgx.Conn, patient, compound, amount string) string {
	t.Helper()

	var id string
	if err := conn.QueryRow(t.Context(), `
		INSERT INTO app.vials (patient_id, compound_id, concentration_label,
		                       total_amount, amount_unit, opened_at, expires_on)
		VALUES ($1, $2, '1 мг/мл', $3::numeric, 'мг', DATE '2026-06-01', DATE '2026-12-01')
		RETURNING id::text
	`, patient, compound, amount).Scan(&id); err != nil {
		t.Fatalf("seeding a vial of %s мг: %v", amount, err)
	}

	return id
}

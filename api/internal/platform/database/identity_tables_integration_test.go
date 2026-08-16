//go:build integration

package database_test

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// identityTables is every table the chain lays down, stated once so that a
// table added to the schema and forgotten by a test is a missing entry rather
// than a missing file. Six of them are this migration's; invites arrives with
// 000008 and joins the same sweeps.
func identityTables() []string {
	return []string{
		"audit_log",
		"care_team_assignments",
		"invites",
		"patient_profiles",
		"profiles",
		"provider_profiles",
		"user_preferences",
	}
}

// The pg_class sweeps have been in the suite since the first migration and have
// been passing vacuously ever since — there were no tables to walk. This is the
// step where that stops, and the emptiness is worth asserting against: a sweep
// over nothing and a sweep over a set of clean tables are the same green.
func TestTheSchemaHoldsExactlyTheTablesDeclared(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)

	rows, err := conn.Query(t.Context(), `
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind IN ('r', 'p')
		ORDER BY c.relname
	`, testsupport.AppSchema)
	if err != nil {
		t.Fatalf("listing the tables: %v", err)
	}
	defer rows.Close()

	var found []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		found = append(found, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating: %v", err)
	}

	if !slices.Equal(found, identityTables()) {
		t.Errorf("schema %s holds %v, want exactly %v", testsupport.AppSchema, found, identityTables())
	}
}

// FORCE is what extends row level security to a table's owner, and the owner is
// the role every migration runs as. Without it the whole arrangement is advisory
// for exactly the role that can also turn it off.
//
// profiles is the sharpest place to measure it. The owner holds every privilege
// on its own tables by virtue of owning them — the grants registry declares
// exactly that — so a refused INSERT cannot be a missing grant. What it can be,
// and is, is FORCE: the policies permitting the owner anything on this table are
// the hook's read and, since 000009, an INSERT of an admin, and the row below is
// a patient.
func TestForceAppliesToTheOwnerToo(t *testing.T) {
	db := cluster.NewDatabase(t)
	ctx := t.Context()

	owner := testsupport.Connect(t, db.MigrationURL)
	if _, err := owner.Exec(ctx, `SET ROLE `+testsupport.OwnerRole); err != nil {
		t.Fatalf("becoming the owner: %v", err)
	}

	// The control: the owner really can reach this table, so the refusal below
	// is not the schema or a missing grant.
	var rows int
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM app.profiles`).Scan(&rows); err != nil {
		t.Fatalf("the owner cannot read profiles at all: %v", err)
	}

	_, err := owner.Exec(ctx, `
		INSERT INTO app.profiles (user_id, role, full_name)
		VALUES ('8a1f3b7c-0000-4000-8000-000000000001', 'patient', 'Ирина Соколова')
	`)
	if err == nil {
		t.Fatal("the owner wrote a row into a table it is forced by")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("the insert failed with %v, want SQLSTATE 42501", err)
	}
}

// unforced hands back a connection that can write to the tables, by taking FORCE
// off them as the owner.
//
// It exists because the constraints have to be exercised and the rows to
// exercise them with cannot be created: forced row level security with no policy
// refuses every write, and the path that will create rows properly — the service
// seam with its policies — arrives two steps later. Taking FORCE off in a
// throwaway database is the smaller lie than asserting the constraints from the
// catalogue and calling that a test.
//
// It is for constraint-level assertions only. The connection it returns runs as
// cadence_owner, which without FORCE is exempt from row level security — so
// anything written on it about row *visibility* would be green because the owner
// is exempt, indistinguishable from green because a policy works.
func unforced(t *testing.T, db *testsupport.Database) *pgx.Conn {
	t.Helper()

	conn := testsupport.Connect(t, db.MigrationURL)
	if _, err := conn.Exec(t.Context(), `SET ROLE `+testsupport.OwnerRole); err != nil {
		t.Fatalf("becoming the owner: %v", err)
	}

	for _, table := range identityTables() {
		if _, err := conn.Exec(
			t.Context(), `ALTER TABLE app.`+table+` NO FORCE ROW LEVEL SECURITY`,
		); err != nil {
			t.Fatalf("lifting FORCE from %s: %v", table, err)
		}
	}

	return conn
}

func insertProfile(t *testing.T, conn *pgx.Conn, id, role, name string) {
	t.Helper()

	if _, err := conn.Exec(t.Context(), `
		INSERT INTO app.profiles (user_id, role, full_name) VALUES ($1, $2, $3)
	`, id, role, name); err != nil {
		t.Fatalf("seeding profile %s: %v", id, err)
	}
}

// Every closed set is closed, and every value inside it is accepted. A CHECK
// that refused everything would pass a test that only tried bad values.
func TestTheClosedSetsAreClosed(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := unforced(t, db)
	ctx := t.Context()

	patient := "8a1f3b7c-0000-4000-8000-000000000001"
	doctor := "8a1f3b7c-0000-4000-8000-000000000002"
	insertProfile(t, conn, patient, "patient", "Ирина Соколова")
	insertProfile(t, conn, doctor, "doctor", "Марина Крылова")

	if _, err := conn.Exec(ctx, `
		INSERT INTO app.patient_profiles (user_id, sex) VALUES ($1, 'female')
	`, patient); err != nil {
		t.Fatalf("seeding the patient card: %v", err)
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO app.user_preferences (user_id) VALUES ($1)
	`, patient); err != nil {
		t.Fatalf("seeding the preferences: %v", err)
	}

	refused := map[string]struct {
		statement string
		args      []any
	}{
		"a role outside the three": {
			`INSERT INTO app.profiles (user_id, role, full_name) VALUES ($1, 'nurse', 'Х')`,
			[]any{"8a1f3b7c-0000-4000-8000-00000000000a"},
		},
		"an empty name": {
			`INSERT INTO app.profiles (user_id, role, full_name) VALUES ($1, 'patient', '')`,
			[]any{"8a1f3b7c-0000-4000-8000-00000000000b"},
		},
		"a sex outside the two": {
			`UPDATE app.patient_profiles SET sex = 'other' WHERE user_id = $1`, []any{patient},
		},
		"a care role outside the three": {
			`INSERT INTO app.care_team_assignments (patient_id, provider_id, care_role)
			 VALUES ($1, $2, 'physio')`, []any{patient, doctor},
		},
		"a lead time outside the three": {
			`UPDATE app.user_preferences SET lead_time_min = 45 WHERE user_id = $1`, []any{patient},
		},
		"units outside the two": {
			`UPDATE app.user_preferences SET units = 'st' WHERE user_id = $1`, []any{patient},
		},
		"a time format outside the two": {
			`UPDATE app.user_preferences SET time_fmt = 36 WHERE user_id = $1`, []any{patient},
		},
		"an assignment of somebody to themselves": {
			`INSERT INTO app.care_team_assignments (patient_id, provider_id, care_role)
			 VALUES ($1, $1, 'endo')`, []any{patient},
		},
		"an audit row nobody signed": {
			`INSERT INTO app.audit_log (action, entity) VALUES ('patient.create', 'profiles')`, nil,
		},
		"an audit row with two actors": {
			`INSERT INTO app.audit_log (actor_id, actor_job, action, entity)
			 VALUES ($1, 'seed', 'patient.create', 'profiles')`, []any{patient},
		},
		"an audit row signed by an empty job": {
			`INSERT INTO app.audit_log (actor_job, action, entity)
			 VALUES ('', 'patient.create', 'profiles')`, nil,
		},
		"a name past the upper bound": {
			`INSERT INTO app.profiles (user_id, role, full_name)
			 VALUES ($1, 'patient', repeat('я', 201))`,
			[]any{"8a1f3b7c-0000-4000-8000-00000000000c"},
		},
	}

	for name, refusal := range refused {
		t.Run(name, func(t *testing.T) {
			_, err := conn.Exec(ctx, refusal.statement, refusal.args...)
			if err == nil {
				t.Fatal("accepted")
			}

			// 23514 is a check constraint. Any error would satisfy "refused" —
			// a typo in a column name is 42703 and would pass a weaker test.
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
				t.Fatalf("refused with %v, want SQLSTATE 23514 (check_violation)", err)
			}
		})
	}

	// The other direction. Without it every assertion above would hold against a
	// schema that refused everything.
	accepted := map[string]struct {
		statement string
		args      []any
	}{
		"each care role in turn": {
			`INSERT INTO app.care_team_assignments (patient_id, provider_id, care_role)
			 VALUES ($1, $2, 'endo')`, []any{patient, doctor},
		},
		"an audit row signed by a person": {
			`INSERT INTO app.audit_log (actor_id, action, entity)
			 VALUES ($1, 'patient.create', 'profiles')`, []any{patient},
		},
		"an audit row signed by a job": {
			`INSERT INTO app.audit_log (actor_job, action, entity)
			 VALUES ('reminder-sweep', 'reminder.send', 'protocols')`, nil,
		},
	}

	for name, statement := range accepted {
		t.Run(name, func(t *testing.T) {
			if _, err := conn.Exec(ctx, statement.statement, statement.args...); err != nil {
				t.Fatalf("refused a value the set contains: %v", err)
			}
		})
	}

	// is_primary is NOT NULL for a named reason — the partial unique index is
	// WHERE is_primary, and a NULL is not true, so a row carrying one would slip
	// past "there is one primary specialist" without anybody deciding it should.
	// 23502 rather than 23514: it is a null violation, not a check.
	t.Run("an assignment with no answer about being primary", func(t *testing.T) {
		_, err := conn.Exec(ctx, `
			INSERT INTO app.care_team_assignments (patient_id, provider_id, care_role, is_primary)
			VALUES ($1, $2, 'nurse', NULL)
		`, patient, doctor)
		if err == nil {
			t.Fatal("accepted")
		}

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23502" {
			t.Fatalf("refused with %v, want SQLSTATE 23502 (not_null_violation)", err)
		}
	})
}

// The other half of every closed set, and the half a refusal test cannot see. A
// list narrower than the spec's — role without admin, care_role without nurse,
// units without lb — refuses nothing a refusal test tries, ships green, and
// first fails when the service path cannot create an admin or assign a nurse.
//
// Driven from the value lists rather than sampled, because sampling is how the
// previous version of this file passed while six of the sets were half
// unmeasured.
func TestEveryValueTheClosedSetsContainIsAccepted(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := unforced(t, db)
	ctx := t.Context()

	for index, role := range []string{"patient", "doctor", "admin"} {
		id := fmt.Sprintf("8a1f3b7c-0000-4000-8000-00000000001%d", index)
		if _, err := conn.Exec(ctx, `
			INSERT INTO app.profiles (user_id, role, full_name) VALUES ($1, $2, $3)
		`, id, role, "Имя "+role); err != nil {
			t.Errorf("profiles.role refused %q: %v", role, err)
		}
	}

	patient := "8a1f3b7c-0000-4000-8000-000000000010"

	for index, sex := range []string{"male", "female"} {
		if _, err := conn.Exec(ctx, `
			INSERT INTO app.patient_profiles (user_id, sex) VALUES ($1, $2)
			ON CONFLICT (user_id) DO UPDATE SET sex = excluded.sex
		`, patient, sex); err != nil {
			t.Errorf("patient_profiles.sex refused %q (case %d): %v", sex, index, err)
		}
	}

	// A name at the boundary, from both sides: the upper bound is refused in the
	// test above, and without this the CHECK could be `length >= 1` and nothing
	// would notice.
	for name, value := range map[string]string{
		"one character":       "я",
		"two hundred of them": strings.Repeat("я", 200),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := conn.Exec(ctx, `
				UPDATE app.profiles SET full_name = $2 WHERE user_id = $1
			`, patient, value); err != nil {
				t.Errorf("profiles.full_name refused a name of %d characters: %v", len([]rune(value)), err)
			}
		})
	}

	for index, careRole := range []string{"endo", "dietitian", "nurse"} {
		provider := fmt.Sprintf("8a1f3b7c-0000-4000-8000-00000000002%d", index)
		if _, err := conn.Exec(ctx, `
			INSERT INTO app.profiles (user_id, role, full_name) VALUES ($1, 'doctor', $2)
		`, provider, "Специалист "+careRole); err != nil {
			t.Fatalf("seeding a provider: %v", err)
		}

		if _, err := conn.Exec(ctx, `
			INSERT INTO app.care_team_assignments (patient_id, provider_id, care_role)
			VALUES ($1, $2, $3)
		`, patient, provider, careRole); err != nil {
			t.Errorf("care_team_assignments.care_role refused %q: %v", careRole, err)
		}
	}

	if _, err := conn.Exec(ctx, `INSERT INTO app.user_preferences (user_id) VALUES ($1)`, patient); err != nil {
		t.Fatalf("seeding the preferences: %v", err)
	}

	for _, lead := range []int{15, 30, 60} {
		if _, err := conn.Exec(ctx, `
			UPDATE app.user_preferences SET lead_time_min = $2 WHERE user_id = $1
		`, patient, lead); err != nil {
			t.Errorf("user_preferences.lead_time_min refused %d: %v", lead, err)
		}
	}

	for _, units := range []string{"kg", "lb"} {
		if _, err := conn.Exec(ctx, `
			UPDATE app.user_preferences SET units = $2 WHERE user_id = $1
		`, patient, units); err != nil {
			t.Errorf("user_preferences.units refused %q: %v", units, err)
		}
	}

	for _, format := range []int{12, 24} {
		if _, err := conn.Exec(ctx, `
			UPDATE app.user_preferences SET time_fmt = $2 WHERE user_id = $1
		`, patient, format); err != nil {
			t.Errorf("user_preferences.time_fmt refused %d: %v", format, err)
		}
	}
}

// One assignment per patient-specialist pair, and one primary specialist per
// patient. The second is a partial unique index rather than a constraint,
// because the uniqueness holds only over the rows that claim to be primary — so
// a second non-primary assignment has to keep working.
func TestTheCareTeamKeepsItsUniqueness(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := unforced(t, db)
	ctx := t.Context()

	patient := "8a1f3b7c-0000-4000-8000-000000000001"
	first := "8a1f3b7c-0000-4000-8000-000000000002"
	second := "8a1f3b7c-0000-4000-8000-000000000003"
	insertProfile(t, conn, patient, "patient", "Ирина Соколова")
	insertProfile(t, conn, first, "doctor", "Марина Крылова")
	insertProfile(t, conn, second, "doctor", "Ольга Ветрова")

	assign := func(provider, role string, primary bool) error {
		_, err := conn.Exec(ctx, `
			INSERT INTO app.care_team_assignments (patient_id, provider_id, care_role, is_primary)
			VALUES ($1, $2, $3, $4)
		`, patient, provider, role, primary)

		return err
	}

	if err := assign(first, "endo", true); err != nil {
		t.Fatalf("assigning the first specialist: %v", err)
	}

	if err := assign(first, "dietitian", false); err == nil {
		t.Error("the same specialist was assigned to the same patient twice")
	} else {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			t.Errorf("the repeated pair was refused with %v, want SQLSTATE 23505", err)
		}
	}

	if err := assign(second, "dietitian", true); err == nil {
		t.Error("a patient has two primary specialists")
	} else {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			t.Errorf("the second primary was refused with %v, want SQLSTATE 23505", err)
		}
	}

	// A second specialist who does not claim to be primary is the ordinary case,
	// and the index must not touch it.
	if err := assign(second, "dietitian", false); err != nil {
		t.Errorf("a second, non-primary specialist was refused: %v", err)
	}
}

// The subquery every doctor and patient policy runs looks through
// care_team_assignments in both directions, on the hot path of every query
// against every table. The indexes are part of the design rather than a tuning
// pass, so their absence is a defect rather than a slowdown.
func TestTheCareTeamCarriesTheIndexesThePoliciesNeed(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)

	rows, err := conn.Query(t.Context(), `
		SELECT indexname, indexdef
		FROM pg_indexes
		WHERE schemaname = $1 AND tablename = 'care_team_assignments'
		ORDER BY indexname
	`, testsupport.AppSchema)
	if err != nil {
		t.Fatalf("listing the indexes: %v", err)
	}
	defer rows.Close()

	// The definition, not only the name. The claim is about a column — the
	// policy subquery reads through this table in both directions — and an index
	// named by_patient built on `since` satisfies a test that reads names.
	definitions := map[string]string{}
	for rows.Next() {
		var name, definition string
		if err := rows.Scan(&name, &definition); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		definitions[name] = definition
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating: %v", err)
	}

	found := slices.Sorted(maps.Keys(definitions))
	want := []string{
		"care_team_assignments_by_patient",
		"care_team_assignments_by_provider",
		"care_team_assignments_one_primary",
		"care_team_assignments_pkey",
		"care_team_assignments_unique_pair",
	}
	if !slices.Equal(found, want) {
		t.Fatalf("care_team_assignments carries %v, want exactly %v", found, want)
	}

	for name, mustContain := range map[string]string{
		"care_team_assignments_by_patient":  "(patient_id)",
		"care_team_assignments_by_provider": "(provider_id)",
		"care_team_assignments_one_primary": "(patient_id) WHERE is_primary",
		"care_team_assignments_unique_pair": "(patient_id, provider_id)",
	} {
		if !strings.Contains(definitions[name], mustContain) {
			t.Errorf("%s is defined as %q, want it to cover %s", name, definitions[name], mustContain)
		}
	}
}

// Cascading from profiles is deliberate for the tables that describe a person,
// and deliberately absent from audit_log: an audit row outlives the rows it
// describes and the people it names, and a cascade would delete exactly the
// evidence of the deletion.
func TestAuditRowsOutliveTheProfilesTheyName(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := unforced(t, db)
	ctx := t.Context()

	patient := "8a1f3b7c-0000-4000-8000-000000000001"
	doctor := "8a1f3b7c-0000-4000-8000-000000000002"
	insertProfile(t, conn, patient, "patient", "Ирина Соколова")
	insertProfile(t, conn, doctor, "doctor", "Марина Крылова")

	// Every table that references profiles, not one of them. RESTRICT instead of
	// CASCADE on care_team_assignments is a behaviour change rather than a
	// slowdown — deleting a profile with any assignment would fail outright —
	// and it is invisible unless a row exists to be cascaded.
	for _, seed := range []struct {
		statement string
		args      []any
	}{
		{`INSERT INTO app.patient_profiles (user_id) VALUES ($1)`, []any{patient}},
		{`INSERT INTO app.provider_profiles (user_id) VALUES ($1)`, []any{doctor}},
		{`INSERT INTO app.user_preferences (user_id) VALUES ($1)`, []any{patient}},
		{
			`INSERT INTO app.care_team_assignments (patient_id, provider_id, care_role)
			 VALUES ($1, $2, 'endo')`,
			[]any{patient, doctor},
		},
		// invites references profiles through invited_by and not through user_id,
		// so the row below names the doctor and describes the patient being
		// deleted.
		{
			`INSERT INTO app.invites (user_id, email, invited_by) VALUES ($1, $2, $3)`,
			[]any{patient, "irina@example.test", doctor},
		},
	} {
		if _, err := conn.Exec(ctx, seed.statement, seed.args...); err != nil {
			t.Fatalf("seeding (%s): %v", seed.statement, err)
		}
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO app.audit_log (actor_job, action, entity, patient_id)
		VALUES ('seed', 'patient.create', 'profiles', $1)
	`, patient); err != nil {
		t.Fatalf("seeding the audit row: %v", err)
	}

	if _, err := conn.Exec(ctx, `DELETE FROM app.profiles WHERE user_id = $1`, patient); err != nil {
		t.Fatalf("deleting the profile: %v", err)
	}

	var cards, preferences, assignments, audits, invitations int
	if err := conn.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM app.patient_profiles WHERE user_id = $1),
		       (SELECT count(*) FROM app.user_preferences WHERE user_id = $1),
		       (SELECT count(*) FROM app.care_team_assignments WHERE patient_id = $1),
		       (SELECT count(*) FROM app.audit_log WHERE patient_id = $1),
		       (SELECT count(*) FROM app.invites WHERE user_id = $1)
	`, patient).Scan(&cards, &preferences, &assignments, &audits, &invitations); err != nil {
		t.Fatalf("counting what survived: %v", err)
	}

	for what, count := range map[string]int{
		"the patient card":     cards,
		"the preferences":      preferences,
		"the care assignments": assignments,
	} {
		if count != 0 {
			t.Errorf("%s survived the profile it describes", what)
		}
	}
	if audits != 1 {
		t.Error("the audit row went with the profile it names; the evidence of a deletion " +
			"cannot be deleted by that deletion")
	}
	// The invitation outlives the profile it brought into being, for the same
	// reason and by a different mechanism: there is no reference on user_id to
	// cascade through, because the row exists before the profile does.
	if invitations != 1 {
		t.Error("the invitation went with the profile it created")
	}
}

// The columns, declared here and reconciled against the catalogue.
//
// The step's acceptance criterion is that every column §03 names is present or
// its absence recorded as a decision, and nothing else in the suite reaches a
// column: the table set is pinned but its contents are not, so dropping
// joined_at, clinic_name, since, entity_id or meta leaves everything green — and
// so does flipping locale to 'en' or weekly_report to false, which is a change a
// patient sees in their notifications.
//
// Two absences are deliberate and are the reason this list is written out rather
// than derived: `initials` and the avatar colour, both of which §03 names and
// neither of which is stored, because they are functions of values that are.
//
// The shape is "name type nullable default", in ordinal order, exactly — the
// same registry shape the grants and policies get in the next step.
func identityColumns() map[string][]string {
	return map[string][]string{
		"profiles": {
			"user_id uuid NOT NULL",
			"role text NOT NULL",
			"full_name text NOT NULL",
			"timezone text NULL",
			"locale text NOT NULL DEFAULT 'ru'::text",
			"created_at timestamp with time zone NOT NULL DEFAULT now()",
		},
		"patient_profiles": {
			"user_id uuid NOT NULL",
			"dob date NULL",
			"sex text NULL",
			"height_cm numeric NULL",
			"target_weight_kg numeric NULL",
			"joined_at timestamp with time zone NULL",
		},
		"provider_profiles": {
			"user_id uuid NOT NULL",
			"title_ru text NULL",
			"clinic_name text NULL",
		},
		"care_team_assignments": {
			"id uuid NOT NULL DEFAULT gen_random_uuid()",
			"patient_id uuid NOT NULL",
			"provider_id uuid NOT NULL",
			"care_role text NOT NULL",
			"is_primary boolean NOT NULL DEFAULT false",
			"since date NULL",
		},
		"user_preferences": {
			"user_id uuid NOT NULL",
			"dose_reminders boolean NOT NULL DEFAULT true",
			"lead_time_min integer NOT NULL DEFAULT 30",
			"meal_reminders boolean NOT NULL DEFAULT false",
			"units text NOT NULL DEFAULT 'kg'::text",
			"time_fmt smallint NOT NULL DEFAULT 24",
			"weekly_report boolean NOT NULL DEFAULT true",
			"team_messages boolean NOT NULL DEFAULT true",
			"reorder_alerts boolean NOT NULL DEFAULT true",
		},
		// No status, no payload, no role: the first is derived, the second lives
		// in the rows the same transaction writes, and the third is on the profile
		// beside them. §03 names all three.
		"invites": {
			"user_id uuid NOT NULL",
			"email text NOT NULL",
			"invited_by uuid NOT NULL",
			"invited_at timestamp with time zone NOT NULL DEFAULT now()",
		},
		"audit_log": {
			"id bigint NOT NULL GENERATED ALWAYS AS IDENTITY",
			"at timestamp with time zone NOT NULL DEFAULT now()",
			"actor_id uuid NULL",
			"actor_job text NULL",
			"action text NOT NULL",
			"entity text NOT NULL",
			"entity_id uuid NULL",
			"patient_id uuid NULL",
			"meta jsonb NULL",
		},
	}
}

func TestTheColumnsAreTheOnesDeclared(t *testing.T) {
	db := cluster.NewDatabase(t)
	conn := testsupport.Connect(t, db.SuperuserURL)

	rows, err := conn.Query(t.Context(), `
		SELECT table_name, column_name, data_type, is_nullable,
		       coalesce(column_default, ''), is_identity
		FROM information_schema.columns
		WHERE table_schema = $1
		ORDER BY table_name, ordinal_position
	`, testsupport.AppSchema)
	if err != nil {
		t.Fatalf("reading the columns: %v", err)
	}
	defer rows.Close()

	found := map[string][]string{}
	for rows.Next() {
		var table, column, kind, nullable, def, identity string
		if err := rows.Scan(&table, &column, &kind, &nullable, &def, &identity); err != nil {
			t.Fatalf("scanning: %v", err)
		}

		described := column + " " + kind
		if nullable == "YES" {
			described += " NULL"
		} else {
			described += " NOT NULL"
		}
		switch {
		case identity == "YES":
			described += " GENERATED ALWAYS AS IDENTITY"
		case def != "":
			described += " DEFAULT " + def
		}

		found[table] = append(found[table], described)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating: %v", err)
	}

	declared := identityColumns()

	for table, want := range declared {
		if !slices.Equal(found[table], want) {
			t.Errorf("app.%s columns:\n got %v\nwant %v", table, found[table], want)
		}
	}
	for table := range found {
		if _, declaredHere := declared[table]; !declaredHere {
			t.Errorf("app.%s exists and is not declared here", table)
		}
	}
}

//go:build integration

package identity_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

var cluster *testsupport.Cluster

func TestMain(m *testing.M) {
	ctx := context.Background()

	c, err := testsupport.StartCluster(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting the test cluster: %v\n", err)
		os.Exit(1)
	}
	cluster = c

	code := m.Run()

	if err := cluster.Terminate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "terminating the test cluster: %v\n", err)
	}

	os.Exit(code)
}

const (
	patientID = "8a1f3b7c-0000-4000-8000-000000000001"
	doctorID  = "8a1f3b7c-0000-4000-8000-000000000002"
	adminID   = "8a1f3b7c-0000-4000-8000-000000000003"
)

// stand brings up a database with the chain applied, a service pool, and the one
// row every test here needs: a doctor to assign the patient to.
//
// The doctor is written through the service seam rather than by the owner,
// because under forced row level security the owner cannot write these tables
// either. That is not an inconvenience of the harness — it is the property the
// whole arrangement is built on, and this path is the only way rows exist at all.
func stand(t *testing.T) (*pgxpool.Pool, *testsupport.Database, database.Actor) {
	t.Helper()

	db := cluster.NewDatabase(t)

	pool, err := database.NewPool(t.Context(), db.ServiceAppURL)
	if err != nil {
		t.Fatalf("opening the service pool: %v", err)
	}
	t.Cleanup(pool.Close)

	actor, err := database.ActingAsJob("test.provisioning")
	if err != nil {
		t.Fatalf("building the actor: %v", err)
	}

	if err := database.WithService(
		t.Context(), pool, actor,
		func(ctx context.Context, tx pgx.Tx) error {
			for _, seed := range []struct {
				id, role, name string
			}{
				{doctorID, "doctor", "Марина Крылова"},
				{adminID, "admin", "Пётр Аверин"},
			} {
				if _, err := tx.Exec(ctx, `
					INSERT INTO app.profiles (user_id, role, full_name, timezone)
					VALUES ($1, $2, $3, 'Europe/Moscow')
				`, seed.id, seed.role, seed.name); err != nil {
					return err
				}
			}

			return nil
		},
	); err != nil {
		t.Fatalf("seeding the clinic: %v", err)
	}

	return pool, db, actor
}

// The values are deliberately all different from one another: two adjacent
// *float64 fields with the same number, or a name and a timezone that could be
// swapped without anybody noticing, is how a mis-mapped INSERT passes a test
// that reads its own fixture back.
func newPatient() identity.NewPatient {
	sex := "female"
	height := 168.0
	target := 62.5
	dob := "1990-03-14"

	return identity.NewPatient{
		UserID:         patientID,
		FullName:       "Ирина Соколова",
		Timezone:       "Asia/Yekaterinburg",
		DateOfBirth:    &dob,
		Sex:            &sex,
		HeightCM:       &height,
		TargetWeightKG: &target,
		Specialists: []identity.Assignment{
			{ProviderID: doctorID, CareRole: "dietitian", Primary: true},
		},
	}
}

// Four rows and an audit record, or none of them. This is the direction that has
// to work before any of the refusals below mean anything.
func TestCreatePatientWritesEverythingThatDescribesThem(t *testing.T) {
	pool, db, actor := stand(t)
	ctx := t.Context()

	if err := identity.CreatePatient(ctx, pool, actor, newPatient()); err != nil {
		t.Fatalf("CreatePatient: %v", err)
	}

	// Counted through the superuser, not through a seam. The service role holds
	// INSERT on audit_log and not SELECT — the grants matrix says so — and a
	// test that had to be granted something to see its own result would be
	// changing the arrangement it is checking.
	observer := testsupport.Connect(t, db.SuperuserURL)

	var profiles, cards, assignments, preferences, audits int
	var joined *string
	if err := observer.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM app.profiles WHERE user_id = $1),
		       (SELECT count(*) FROM app.patient_profiles WHERE user_id = $1),
		       (SELECT count(*) FROM app.care_team_assignments WHERE patient_id = $1),
		       (SELECT count(*) FROM app.user_preferences WHERE user_id = $1),
		       (SELECT count(*) FROM app.audit_log
		          WHERE patient_id = $1 AND action = 'patient.create'),
		       (SELECT joined_at::text FROM app.patient_profiles WHERE user_id = $1)
	`, patientID).Scan(&profiles, &cards, &assignments, &preferences, &audits, &joined); err != nil {
		t.Fatalf("counting what was written: %v", err)
	}

	for what, count := range map[string]int{
		"profile":     profiles,
		"card":        cards,
		"assignment":  assignments,
		"preferences": preferences,
		"audit row":   audits,
	} {
		if count != 1 {
			t.Errorf("%d %s(s) written, want 1", count, what)
		}
	}

	// The values, not only the counts. Counting rows leaves every column
	// unmeasured — and the one that matters most is role: the token issuance
	// hook reads it to choose the Postgres role a request runs as, so a patient
	// written with 'doctor' there is a patient issued a doctor's token, and five
	// row counts would all still say 1.
	patient := newPatient()

	var role, fullName, timezone, locale, sex, careRole string
	var height, target float64
	var isPrimary bool
	var action, entity, actorJob string
	var entityID string
	if err := observer.QueryRow(ctx, `
		SELECT p.role, p.full_name, p.timezone, p.locale,
		       c.sex, c.height_cm, c.target_weight_kg,
		       a.care_role, a.is_primary,
		       l.action, l.entity, l.entity_id::text, l.actor_job
		FROM app.profiles p
		JOIN app.patient_profiles c ON c.user_id = p.user_id
		JOIN app.care_team_assignments a ON a.patient_id = p.user_id
		JOIN app.audit_log l ON l.patient_id = p.user_id
		WHERE p.user_id = $1
	`, patientID).Scan(&role, &fullName, &timezone, &locale, &sex, &height, &target,
		&careRole, &isPrimary, &action, &entity, &entityID, &actorJob); err != nil {
		t.Fatalf("reading back what was written: %v", err)
	}

	for what, got := range map[string][2]string{
		"role":      {role, "patient"},
		"full_name": {fullName, patient.FullName},
		"timezone":  {timezone, patient.Timezone},
		"locale":    {locale, "ru"},
		"sex":       {sex, *patient.Sex},
		"care_role": {careRole, patient.Specialists[0].CareRole},
		"action":    {action, "patient.create"},
		"entity":    {entity, "profiles"},
		"entity_id": {entityID, patientID},
		"actor_job": {actorJob, "test.provisioning"},
	} {
		if got[0] != got[1] {
			t.Errorf("%s = %q, want %q", what, got[0], got[1])
		}
	}

	if height != *patient.HeightCM {
		t.Errorf("height_cm = %v, want %v", height, *patient.HeightCM)
	}
	if target != *patient.TargetWeightKG {
		t.Errorf("target_weight_kg = %v, want %v", target, *patient.TargetWeightKG)
	}
	if !isPrimary {
		t.Error("is_primary = false on the only specialist, who was named primary")
	}

	// joined_at is the moment the invitation is accepted, which has not
	// happened. profiles.created_at is the moment the clinic created the row.
	if joined != nil {
		t.Errorf("joined_at is %q on a patient who has not accepted an invitation", *joined)
	}
}

// A refusal partway through takes everything with it, the audit row included.
// There is nothing to have signed.
func TestCreatePatientLeavesNothingBehindWhenItRefuses(t *testing.T) {
	pool, db, actor := stand(t)
	ctx := t.Context()

	// The refusal has to arrive after writing has started, or there is nothing
	// to have left behind. Every check that can run before the transaction does
	// run before it — so the one that cannot is the database's own: the same
	// specialist named twice, refused by the unique pair after the profile, the
	// card and the first assignment are already in.
	patient := newPatient()
	patient.Specialists = append(patient.Specialists,
		identity.Assignment{ProviderID: doctorID, CareRole: "nurse"})

	if err := identity.CreatePatient(ctx, pool, actor, patient); !errors.Is(
		err, identity.ErrAlreadyExists,
	) {
		t.Fatalf("CreatePatient = %v, want ErrAlreadyExists", err)
	}

	observer := testsupport.Connect(t, db.SuperuserURL)

	var profiles, cards, assignments, preferences, audits int
	if err := observer.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM app.profiles WHERE user_id = $1),
		       (SELECT count(*) FROM app.patient_profiles WHERE user_id = $1),
		       (SELECT count(*) FROM app.care_team_assignments WHERE patient_id = $1),
		       (SELECT count(*) FROM app.user_preferences WHERE user_id = $1),
		       (SELECT count(*) FROM app.audit_log WHERE patient_id = $1)
	`, patientID).Scan(&profiles, &cards, &assignments, &preferences, &audits); err != nil {
		t.Fatalf("counting after the refusal: %v", err)
	}

	for what, count := range map[string]int{
		"profile":     profiles,
		"card":        cards,
		"assignment":  assignments,
		"preferences": preferences,
		"audit row":   audits,
	} {
		if count != 0 {
			t.Errorf("%d %s(s) survived a refused creation", count, what)
		}
	}
}

// mutate returns a patient with one thing changed. The table below reads as a
// list of rules rather than as a list of struct literals.
func mutate(change func(*identity.NewPatient)) identity.NewPatient {
	patient := newPatient()
	change(&patient)

	return patient
}

func TestCreatePatientRefusesWhatTheRulesForbid(t *testing.T) {
	pool, _, actor := stand(t)
	ctx := t.Context()

	cases := map[string]struct {
		patient identity.NewPatient
		want    error
	}{
		"a timezone the server does not know": {
			mutate(func(p *identity.NewPatient) { p.Timezone = "Middle-earth/Shire" }),
			identity.ErrUnknownTimezone,
		},
		"a specialist who is not a doctor": {
			mutate(func(p *identity.NewPatient) { p.Specialists[0].ProviderID = adminID }),
			identity.ErrNotAProvider,
		},
		"a specialist who does not exist": {
			mutate(func(p *identity.NewPatient) {
				p.Specialists[0].ProviderID = "8a1f3b7c-0000-4000-8000-0000000000ff"
			}),
			identity.ErrNotAProvider,
		},
		"nobody to look after them": {
			mutate(func(p *identity.NewPatient) { p.Specialists = nil }),
			identity.ErrNoSpecialist,
		},
		"two specialists leading at once": {
			mutate(func(p *identity.NewPatient) {
				p.Specialists = append(p.Specialists,
					identity.Assignment{ProviderID: adminID, CareRole: "nurse", Primary: true})
			}),
			identity.ErrTwoPrimarySpecialists,
		},
		"an identifier that is not a UUID": {
			mutate(func(p *identity.NewPatient) { p.UserID = "not-a-uuid" }),
			identity.ErrMalformedIdentifier,
		},
		"a specialist identifier that is not a UUID": {
			mutate(func(p *identity.NewPatient) { p.Specialists[0].ProviderID = "nope" }),
			identity.ErrMalformedIdentifier,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := identity.CreatePatient(ctx, pool, actor, tc.patient); !errors.Is(err, tc.want) {
				t.Errorf("CreatePatient = %v, want %v", err, tc.want)
			}
		})
	}
}

// The same patient twice, and the same pair twice. Both are the database
// speaking: a check-then-insert is two statements with a gap between them, and
// the gap is where two people adding the same patient both find nothing.
func TestCreatePatientRefusesWhatIsAlreadyThere(t *testing.T) {
	pool, _, actor := stand(t)
	ctx := t.Context()

	if err := identity.CreatePatient(ctx, pool, actor, newPatient()); err != nil {
		t.Fatalf("CreatePatient: %v", err)
	}

	if err := identity.CreatePatient(ctx, pool, actor, newPatient()); !errors.Is(
		err, identity.ErrAlreadyExists,
	) {
		t.Errorf("creating the same patient twice = %v, want ErrAlreadyExists", err)
	}

	// The same specialist named twice on one patient, which the unique pair
	// refuses rather than the profile's primary key.
	twice := newPatient()
	twice.UserID = "8a1f3b7c-0000-4000-8000-000000000009"
	twice.Specialists = append(twice.Specialists,
		identity.Assignment{ProviderID: doctorID, CareRole: "dietitian"})

	if err := identity.CreatePatient(ctx, pool, actor, twice); !errors.Is(
		err, identity.ErrAlreadyExists,
	) {
		t.Errorf("naming the same specialist twice = %v, want ErrAlreadyExists", err)
	}
}

// The audit policy reconciles the row's actor against the one the seam
// published. It is a "do not forget to name the actor" property rather than
// protection against forgery — code that can write the row can write the setting
// — and it is worth exactly that much.
func TestAnAuditRowNamingSomebodyElseIsRefused(t *testing.T) {
	pool, _, actor := stand(t)
	ctx := t.Context()

	err := database.WithService(ctx, pool, actor, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO app.audit_log (actor_job, action, entity)
			VALUES ('somebody.else', 'patient.create', 'profiles')
		`)

		return err
	})
	if err == nil {
		t.Fatal("an audit row named an actor the seam never published")
	}

	// 42501 is the policy refusing, not a constraint. A check violation here
	// would mean the row was wrong in some other way and the policy never ran.
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("the write failed with %v, want SQLSTATE 42501", err)
	}

	// The control: the same row naming the published actor goes in. Without it
	// the assertion above would hold against a policy that refuses everything.
	if err := database.WithService(
		ctx, pool, actor,
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.audit_log (actor_job, action, entity)
				VALUES ('test.provisioning', 'patient.create', 'profiles')
			`)

			return err
		},
	); err != nil {
		t.Fatalf("an audit row naming the published actor was refused: %v", err)
	}
}

// Append-only, as a property of the schema against both paths. Nobody holds
// UPDATE or DELETE on this table and no policy grants either, so the refusal
// arrives before row level security is consulted at all.
func TestTheAuditLogIsAppendOnlyForEverybody(t *testing.T) {
	pool, db, actor := stand(t)
	ctx := t.Context()

	if err := identity.CreatePatient(ctx, pool, actor, newPatient()); err != nil {
		t.Fatalf("CreatePatient: %v", err)
	}

	for _, role := range testsupport.ImpersonationRoles() {
		for _, statement := range []string{
			`UPDATE app.audit_log SET action = 'rewritten'`,
			`DELETE FROM app.audit_log`,
			`TRUNCATE app.audit_log`,
		} {
			t.Run(role+" "+strings.Fields(statement)[0], func(t *testing.T) {
				conn := testsupport.Connect(t, db.AppURL)
				if _, err := conn.Exec(ctx, `SET ROLE `+role); err != nil {
					// The request path can only assume the product roles; the
					// service role is reached from its own pool.
					conn = testsupport.Connect(t, db.ServiceAppURL)
					if _, err := conn.Exec(ctx, `SET ROLE `+role); err != nil {
						t.Fatalf("impersonating %s: %v", role, err)
					}
				}

				_, err := conn.Exec(ctx, statement)
				if err == nil {
					t.Fatalf("%s changed the audit log", role)
				}

				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
					t.Fatalf("%s was refused with %v, want SQLSTATE 42501", role, err)
				}
			})
		}
	}

	// The control: the rows really are there, so the refusals above are about
	// privileges rather than about an empty table.
	observer := testsupport.Connect(t, db.SuperuserURL)

	var audits int
	if err := observer.QueryRow(ctx, `SELECT count(*) FROM app.audit_log`).Scan(&audits); err != nil {
		t.Fatalf("counting the audit rows: %v", err)
	}
	if audits == 0 {
		t.Fatal("there are no audit rows, so nothing above was protected")
	}
}

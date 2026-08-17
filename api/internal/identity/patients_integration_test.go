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
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

var (
	cluster *testsupport.Cluster

	// cycle is the identity provider beside the chain, shared by the whole
	// binary: this context is the one that invites, claims and deletes accounts,
	// so the container it costs is paid for. Started from TestMain because that
	// is where it can also be torn down, after m.Run rather than after a test.
	cycle *testsupport.Cycle
)

// os.Exit runs no deferred function and a panicking test never returns through m.Run, so the teardown lives in a
// function of its own: without it a panic leaves the containers running, and TESTCONTAINERS_RYUK_DISABLED means
// nothing else reaps them.
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

	cy, err := testsupport.StartCycle(ctx, cluster)
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting the cycle harness: %v\n", err)

		return 1
	}
	cycle = cy

	defer func() {
		if err := cycle.Terminate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "terminating the cycle harness: %v\n", err)
		}
	}()

	return m.Run()
}

const (
	patientID = "8a1f3b7c-0000-4000-8000-000000000001"
	doctorID  = "8a1f3b7c-0000-4000-8000-000000000002"
	adminID   = "8a1f3b7c-0000-4000-8000-000000000003"
)

// provisioningJob is the harness itself: the rows the fixtures need exist
// because a test put them there, and no human asked for them.
const provisioningJob = "test.provisioning"

// asTheDoctor is the request a patient is created on behalf of.
//
// Since step-4 the audit actor is read off the principal instead of being passed
// in, so creating a patient is no longer something a test can do anonymously —
// which is the point: in production this is a doctor's action, and the audit row
// now has to say so.
func asTheDoctor(t *testing.T) context.Context {
	t.Helper()

	return auth.WithPrincipal(t.Context(), auth.Principal{Subject: doctorID, Role: "doctor"})
}

// stand brings up a database with the chain applied, a service pool, and the two
// rows every test here needs: a doctor to assign the patient to, and an admin to
// ask the policies about.
//
// The doctor is written through the service seam rather than by the owner,
// because under forced row level security the owner cannot write these tables
// either. That is not an inconvenience of the harness — it is the property the
// whole arrangement is built on, and this path is the only way rows exist at all.
//
// The admin is the exception, and it is one since 000006 rather than an oversight
// of the harness: the service policies on profiles refuse the value outright, so
// the row is written by the superuser instead. Everything the tests below assert
// about an admin caller is unchanged; only the way the row comes into being is,
// and that is the point of the step.
func stand(t *testing.T) (*pgxpool.Pool, *testsupport.Database) {
	t.Helper()

	db := cluster.NewDatabase(t)

	pool, err := database.NewPool(t.Context(), db.ServiceAppURL)
	if err != nil {
		t.Fatalf("opening the service pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := database.WithServiceJob(
		t.Context(), pool, provisioningJob,
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.profiles (user_id, role, full_name, timezone)
				VALUES ($1, 'doctor', 'Марина Крылова', 'Europe/Moscow')
			`, doctorID)

			return err
		},
	); err != nil {
		t.Fatalf("seeding the doctor: %v", err)
	}

	superuser := testsupport.Connect(t, db.SuperuserURL)
	if _, err := superuser.Exec(t.Context(), `
		INSERT INTO app.profiles (user_id, role, full_name, timezone)
		VALUES ($1, 'admin', 'Пётр Аверин', 'Europe/Moscow')
	`, adminID); err != nil {
		t.Fatalf("seeding the admin: %v", err)
	}

	return pool, db
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
	pool, db := stand(t)
	ctx := asTheDoctor(t)

	if err := identity.CreatePatient(ctx, pool, newPatient()); err != nil {
		t.Fatalf("CreatePatient: %v", err)
	}

	// Counted through the superuser: the service role holds INSERT on audit_log
	// and not SELECT, and a test that had to be granted something to see its own
	// result would be changing the arrangement it is checking.
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
	var action, entity, actorID, actorJob string
	var entityID string
	if err := observer.QueryRow(ctx, `
		SELECT p.role, p.full_name, p.timezone, p.locale,
		       c.sex, c.height_cm, c.target_weight_kg,
		       a.care_role, a.is_primary,
		       l.action, l.entity, l.entity_id::text,
		       coalesce(l.actor_id::text, ''), coalesce(l.actor_job, '')
		FROM app.profiles p
		JOIN app.patient_profiles c ON c.user_id = p.user_id
		JOIN app.care_team_assignments a ON a.patient_id = p.user_id
		JOIN app.audit_log l ON l.patient_id = p.user_id
		WHERE p.user_id = $1
	`, patientID).Scan(&role, &fullName, &timezone, &locale, &sex, &height, &target,
		&careRole, &isPrimary, &action, &entity, &entityID, &actorID, &actorJob); err != nil {
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
		// The audit row names the doctor the request was verified as, and names
		// no job: creating a patient is somebody's action, and since step-4 the
		// seam reads who from the principal rather than from an argument.
		"actor_id":  {actorID, doctorID},
		"actor_job": {actorJob, ""},
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

	if joined != nil {
		t.Errorf("joined_at is %q on a patient who has not accepted an invitation", *joined)
	}
}

// A refusal partway through takes everything with it, the audit row included.
// There is nothing to have signed.
func TestCreatePatientLeavesNothingBehindWhenItRefuses(t *testing.T) {
	pool, db := stand(t)
	ctx := asTheDoctor(t)

	// The refusal has to arrive after writing has started, or there is nothing
	// to have left behind. Every check that can run before the transaction does
	// run before it — so the one that cannot is the database's own: the same
	// specialist named twice, refused by the unique pair after the profile, the
	// card and the first assignment are already in.
	patient := newPatient()
	patient.Specialists = append(patient.Specialists,
		identity.Assignment{ProviderID: doctorID, CareRole: "nurse"})

	if err := identity.CreatePatient(ctx, pool, patient); !errors.Is(
		err, identity.ErrAssignmentCollided,
	) {
		t.Fatalf("CreatePatient = %v, want ErrAssignmentCollided", err)
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
	pool, _ := stand(t)
	ctx := asTheDoctor(t)

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
			if err := identity.CreatePatient(ctx, pool, tc.patient); !errors.Is(err, tc.want) {
				t.Errorf("CreatePatient = %v, want %v", err, tc.want)
			}
		})
	}
}

// The same patient twice, and the same pair twice. Both are the database
// speaking: a check-then-insert is two statements with a gap between them, and
// the gap is where two people adding the same patient both find nothing.
func TestCreatePatientRefusesWhatIsAlreadyThere(t *testing.T) {
	pool, _ := stand(t)
	ctx := asTheDoctor(t)

	if err := identity.CreatePatient(ctx, pool, newPatient()); err != nil {
		t.Fatalf("CreatePatient: %v", err)
	}

	if err := identity.CreatePatient(ctx, pool, newPatient()); !errors.Is(
		err, identity.ErrAlreadyExists,
	) {
		t.Errorf("creating the same patient twice = %v, want ErrAlreadyExists", err)
	}

	// The same specialist named twice, which the care team's unique pair refuses
	// rather than the profile's primary key — and they are different facts with
	// different answers. Collapsed onto one error they became one answer, and the
	// onboarding flow reported a repeated specialist as «this address already
	// belongs to a patient», after the invitation had gone out.
	twice := newPatient()
	twice.UserID = "8a1f3b7c-0000-4000-8000-000000000009"
	twice.Specialists = append(twice.Specialists,
		identity.Assignment{ProviderID: doctorID, CareRole: "dietitian"})

	if err := identity.CreatePatient(ctx, pool, twice); !errors.Is(
		err, identity.ErrAssignmentCollided,
	) {
		t.Errorf("naming the same specialist twice = %v, want ErrAssignmentCollided", err)
	}
}

// The audit policy reconciles the row's actor against the one the seam
// published. It is a "do not forget to name the actor" property rather than
// protection against forgery — code that can write the row can write the setting
// — and it is worth exactly that much.
func TestAnAuditRowNamingSomebodyElseIsRefused(t *testing.T) {
	pool, _ := stand(t)
	ctx := t.Context()

	err := database.WithServiceJob(ctx, pool, provisioningJob, func(ctx context.Context, tx pgx.Tx) error {
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
	if err := database.WithServiceJob(
		ctx, pool, provisioningJob,
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
	pool, db := stand(t)
	ctx := asTheDoctor(t)

	if err := identity.CreatePatient(ctx, pool, newPatient()); err != nil {
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

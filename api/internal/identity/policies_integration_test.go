//go:build integration

package identity_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// The regression suite the whole block exists for.
//
// Everything above this file proves the arrangement is as declared: the roles,
// the grants, the policies, their predicates. This proves what the arrangement
// does, on rows, and it is the one that would notice if all of that were right
// and the result still wrong. Every later migration is obliged to extend it —
// that is an acceptance rule of the data layer, not a wish.
//
// The rows come from the service path, because there is no other way to obtain
// them: forced row level security refuses the owner too, so a fixture inserted
// "just for the test" cannot exist. What that costs is that this suite depends
// on step-6 being right; what it buys is that it exercises the only way rows are
// ever created.

const (
	otherPatientID = "8a1f3b7c-0000-4000-8000-000000000011"
	otherDoctorID  = "8a1f3b7c-0000-4000-8000-000000000012"
)

// clinic is two patients and two doctors, assigned crosswise: each patient has
// one specialist, and each doctor has one patient. That shape is what makes
// "does not see an unassigned one" mean something — with a single patient, every
// row a doctor could read is one they are entitled to.
type clinic struct {
	service *pgxpool.Pool
	request *pgxpool.Pool
}

func newClinic(t *testing.T) clinic {
	t.Helper()

	service, db := stand(t)

	request, err := database.NewPool(t.Context(), db.AppURL)
	if err != nil {
		t.Fatalf("opening the request pool: %v", err)
	}
	t.Cleanup(request.Close)

	if err := database.WithServiceJob(
		t.Context(), service, provisioningJob,
		func(ctx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `
				INSERT INTO app.profiles (user_id, role, full_name, timezone)
				VALUES ($1, 'doctor', 'Ольга Ветрова', 'Europe/Moscow')
			`, otherDoctorID); err != nil {
				return err
			}

			// provider_profiles carries five policies and had no row anywhere in
			// this suite, so "a patient does not see an unassigned specialist's
			// card" was true of an empty table.
			for id, title := range map[string]string{
				doctorID:      "Эндокринолог",
				otherDoctorID: "Диетолог",
			} {
				if _, err := tx.Exec(ctx, `
					INSERT INTO app.provider_profiles (user_id, title_ru, clinic_name)
					VALUES ($1, $2, 'Клиника Cadence')
				`, id, title); err != nil {
					return err
				}
			}

			return nil
		},
	); err != nil {
		t.Fatalf("seeding the second doctor: %v", err)
	}

	first := newPatient()
	if err := identity.CreatePatient(asTheDoctor(t), service, first); err != nil {
		t.Fatalf("creating the first patient: %v", err)
	}

	second := newPatient()
	second.UserID = otherPatientID
	second.FullName = "Анна Лебедева"
	second.Specialists = []identity.Assignment{
		{ProviderID: otherDoctorID, CareRole: "endo", Primary: true},
	}
	if err := identity.CreatePatient(asTheDoctor(t), service, second); err != nil {
		t.Fatalf("creating the second patient: %v", err)
	}

	return clinic{service: service, request: request}
}

// as runs fn as one caller through the request seam — the way every request
// runs, and the only way these policies are ever consulted in production.
func (c clinic) as(t *testing.T, subject, role string, fn func(context.Context, pgx.Tx) error) error {
	t.Helper()

	return database.WithCaller(
		t.Context(), c.request, database.Caller{Subject: subject, Role: role}, fn,
	)
}

// visibleProfiles is the question every policy in this schema is really
// answering: whose rows come back.
func (c clinic) visibleProfiles(t *testing.T, subject, role string) []string {
	t.Helper()

	var seen []string
	if err := c.as(t, subject, role, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT user_id::text FROM app.profiles ORDER BY user_id`)
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
		t.Fatalf("reading profiles as %s: %v", role, err)
	}

	return seen
}

// visible reads one column of one table as one caller, so a "cannot see" claim
// is a set rather than a count.
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

// changed runs one statement as one caller and reports how many rows it touched.
//
// It exists because err == nil is not a witness for a write: an UPDATE whose
// rows a policy filters away returns success and touches nothing. Every positive
// write in this file went through that hole before review measured it — the
// control meant to rule out "a schema where the patient can change nothing" held
// in exactly that schema.
func (c clinic) changed(t *testing.T, subject, role, statement string, args ...any) int64 {
	t.Helper()

	var affected int64
	if err := c.as(t, subject, role, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, statement, args...)
		affected = tag.RowsAffected()

		return err
	}); err != nil {
		t.Fatalf("running %q as %s: %v", statement, role, err)
	}

	return affected
}

// A patient sees themselves and the specialists looking after them, and nobody
// else — not the other patient, not the other patient's doctor.
func TestAPatientSeesThemselvesAndTheirOwnSpecialists(t *testing.T) {
	c := newClinic(t)

	seen := c.visibleProfiles(t, patientID, "patient")

	for _, want := range []string{patientID, doctorID} {
		if !slices.Contains(seen, want) {
			t.Errorf("the patient cannot see %s", want)
		}
	}
	for _, forbidden := range []string{otherPatientID, otherDoctorID, adminID} {
		if slices.Contains(seen, forbidden) {
			t.Errorf("the patient can see %s", forbidden)
		}
	}
}

// A doctor sees themselves and the patients assigned to them. The unassigned
// patient is not refused — they are absent, which is the difference between a
// policy and a permission and the reason a doctor's dashboard needs no filter.
func TestADoctorSeesThemselvesAndTheirAssignedPatients(t *testing.T) {
	c := newClinic(t)

	seen := c.visibleProfiles(t, doctorID, "doctor")

	for _, want := range []string{doctorID, patientID} {
		if !slices.Contains(seen, want) {
			t.Errorf("the doctor cannot see %s", want)
		}
	}
	for _, forbidden := range []string{otherPatientID, otherDoctorID, adminID} {
		if slices.Contains(seen, forbidden) {
			t.Errorf("the doctor can see %s", forbidden)
		}
	}
}

// Somebody else's row is an empty result, not a refusal. A policy that answered
// 42501 would tell every caller which identifiers exist.
func TestSomebodyElsesRowComesBackEmptyRatherThanRefused(t *testing.T) {
	c := newClinic(t)

	var theirs, own int
	if err := c.as(t, patientID, "patient", func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT (SELECT count(*) FROM app.profiles WHERE user_id = $1),
			       (SELECT count(*) FROM app.profiles WHERE user_id = $2)
		`, otherPatientID, patientID).Scan(&theirs, &own)
	}); err != nil {
		t.Fatalf("reading somebody else's row: %v", err)
	}

	if theirs != 0 {
		t.Errorf("%d row(s) came back for somebody else's profile", theirs)
	}
	// The control: zero is also the answer with the policies removed entirely,
	// so the same query aimed at the caller's own row has to answer one.
	if own != 1 {
		t.Error("the caller cannot see their own row either, so the emptiness above says nothing")
	}
}

// Reassignment takes effect through the policies, with no query edited anywhere.
// Adding the row grants the visibility; deleting it takes the visibility away.
func TestVisibilityFollowsTheCareTeamInBothDirections(t *testing.T) {
	c := newClinic(t)
	ctx := t.Context()

	// The other doctor cannot see the first patient — the assignment does not
	// exist yet.
	if seen := c.visibleProfiles(t, otherDoctorID, "doctor"); slices.Contains(seen, patientID) {
		t.Fatal("a doctor sees a patient nobody assigned to them")
	}

	if err := database.WithServiceJob(
		ctx, c.service, provisioningJob,
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.care_team_assignments (patient_id, provider_id, care_role)
				VALUES ($1, $2, 'nurse')
			`, patientID, otherDoctorID)

			return err
		},
	); err != nil {
		t.Fatalf("adding the assignment: %v", err)
	}

	if seen := c.visibleProfiles(t, otherDoctorID, "doctor"); !slices.Contains(seen, patientID) {
		t.Error("adding an assignment did not grant visibility")
	}

	if err := database.WithServiceJob(
		ctx, c.service, provisioningJob,
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				DELETE FROM app.care_team_assignments
				WHERE patient_id = $1 AND provider_id = $2
			`, patientID, otherDoctorID)

			return err
		},
	); err != nil {
		t.Fatalf("removing the assignment: %v", err)
	}

	if seen := c.visibleProfiles(t, otherDoctorID, "doctor"); slices.Contains(seen, patientID) {
		t.Error("removing an assignment did not take the visibility away")
	}
}

// What a patient may change about themselves, and what they may not. The role is
// the escalation the seven-role split exists to prevent; the clinical fields are
// the clinic's to enter; the care team is not theirs to edit, because deleting an
// assignment would be a way to hide from the doctor.
func TestWhatAPatientMayNotChange(t *testing.T) {
	c := newClinic(t)

	refusals := map[string]string{
		"their own role": `UPDATE app.profiles SET role = 'admin' WHERE user_id = $1`,
		"their date of birth": `UPDATE app.patient_profiles SET dob = '1980-01-01'
			WHERE user_id = $1`,
		"their height":         `UPDATE app.patient_profiles SET height_cm = 200 WHERE user_id = $1`,
		"their sex":            `UPDATE app.patient_profiles SET sex = 'male' WHERE user_id = $1`,
		"somebody's care team": `DELETE FROM app.care_team_assignments WHERE patient_id = $1`,
		"a new assignment": `INSERT INTO app.care_team_assignments
			(patient_id, provider_id, care_role) VALUES ($1, $1, 'endo')`,
	}

	for what, statement := range refusals {
		t.Run(what, func(t *testing.T) {
			err := c.as(t, patientID, "patient", func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, statement, patientID)

				return err
			})
			if err == nil {
				t.Fatal("accepted")
			}

			// 42501 specifically: these are refused by a missing grant rather
			// than by a policy, which is the stronger of the two — the column
			// the patient cannot write does not exist for them at all.
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
				t.Fatalf("refused with %v, want SQLSTATE 42501", err)
			}
		})
	}

	// The control, and it counts rows rather than trusting a nil error: an UPDATE
	// whose rows a policy filters away succeeds and touches nothing, so the
	// original version of this check held in exactly the schema it claimed to
	// exclude — one where the patient can change nothing at all.
	if affected := c.changed(
		t, patientID, "patient",
		`UPDATE app.profiles SET full_name = 'Ирина Соколова-Ким' WHERE user_id = $1`,
		patientID,
	); affected != 1 {
		t.Errorf("correcting their own name touched %d row(s), want 1", affected)
	}

	// And not somebody else's. This is the only place the own-row UPDATE policy
	// on profiles is observable on its own: the patient can see their doctor's
	// row through the specialist policy and holds the column grant, so without
	// that policy this write lands on it.
	if affected := c.changed(
		t, patientID, "patient",
		`UPDATE app.profiles SET full_name = 'Подделка' WHERE user_id = $1`,
		doctorID,
	); affected != 0 {
		t.Errorf("the patient rewrote %d row(s) of their doctor's profile", affected)
	}
}

// A patient may set their own target weight and nothing else on that card. It is
// the one number on a clinical record that belongs to the person rather than to
// the clinic.
func TestAPatientOwnsTheirTargetWeightAndNothingElseOnTheCard(t *testing.T) {
	c := newClinic(t)

	if affected := c.changed(
		t, patientID, "patient",
		`UPDATE app.patient_profiles SET target_weight_kg = 61 WHERE user_id = $1`, patientID,
	); affected != 1 {
		t.Errorf("setting their own target touched %d row(s), want 1", affected)
	}

	// And it landed on the value, not merely on a row.
	var target float64
	if err := c.as(t, patientID, "patient", func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(
			ctx, `SELECT target_weight_kg FROM app.patient_profiles WHERE user_id = $1`, patientID,
		).Scan(&target)
	}); err != nil {
		t.Fatalf("reading back the target: %v", err)
	}
	if target != 61 {
		t.Errorf("target_weight_kg = %v after the write, want 61", target)
	}

	// And not on somebody else's card: the grant lets them write the column, the
	// policy decides whose row.
	var updated int64
	if err := c.as(t, patientID, "patient", func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(
			ctx, `UPDATE app.patient_profiles SET target_weight_kg = 40 WHERE user_id = $1`,
			otherPatientID,
		)
		updated = tag.RowsAffected()

		return err
	}); err != nil {
		t.Fatalf("writing somebody else's target: %v", err)
	}

	if updated != 0 {
		t.Errorf("%d row(s) of somebody else's card were changed", updated)
	}
}

// The admin reads and writes everything except the audit log, where they read
// and nothing more. Append-only is a property of the schema against both paths.
func TestTheAdminReadsEverythingAndCannotRewriteHistory(t *testing.T) {
	c := newClinic(t)

	seen := c.visibleProfiles(t, adminID, "admin")
	for _, want := range []string{patientID, otherPatientID, doctorID, otherDoctorID, adminID} {
		if !slices.Contains(seen, want) {
			t.Errorf("the admin cannot see %s", want)
		}
	}

	var audits int
	if err := c.as(t, adminID, "admin", func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log`).Scan(&audits)
	}); err != nil {
		t.Fatalf("the admin cannot read the audit log: %v", err)
	}
	if audits == 0 {
		t.Fatal("the audit log is empty, so nothing below is protected")
	}

	for what, statement := range map[string]string{
		"rewriting a record": `UPDATE app.audit_log SET action = 'rewritten'`,
		"deleting one":       `DELETE FROM app.audit_log`,
		"adding one":         `INSERT INTO app.audit_log (actor_job, action, entity) VALUES ('x', 'y', 'z')`,
	} {
		t.Run(what, func(t *testing.T) {
			err := c.as(t, adminID, "admin", func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, statement)

				return err
			})
			if err == nil {
				t.Fatal("accepted")
			}

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
				t.Fatalf("refused with %v, want SQLSTATE 42501", err)
			}
		})
	}
}

// A transaction with nothing published reads nothing. The refusal comes from the
// policies deciding on a NULL subject, not from privileges — which is why the
// count is zero rather than an error, and why it is worth asserting separately:
// a policy written with a negation would answer "everything" here.
func TestATransactionWithNoSubjectReadsNothing(t *testing.T) {
	c := newClinic(t)
	ctx := t.Context()

	conn, err := c.request.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquiring a connection: %v", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("beginning: %v", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	// The product role without the subject: exactly what a seam that forgot to
	// publish the claims would leave behind.
	if _, err := tx.Exec(ctx, `SELECT set_config('role', $1, true)`, testsupport.PatientRole); err != nil {
		t.Fatalf("assuming the product role: %v", err)
	}

	var profiles, cards, assignments int
	if err := tx.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM app.profiles),
		       (SELECT count(*) FROM app.patient_profiles),
		       (SELECT count(*) FROM app.care_team_assignments)
	`).Scan(&profiles, &cards, &assignments); err != nil {
		t.Fatalf("reading with no subject published: %v", err)
	}

	for what, count := range map[string]int{
		"profiles":    profiles,
		"cards":       cards,
		"assignments": assignments,
	} {
		if count != 0 {
			t.Errorf("%d %s came back to a caller with no subject", count, what)
		}
	}
}

// The residual property, at the level the policies work at.
//
// request.jwt.claims has USERSET context: a caller can overwrite it inside their
// own transaction and nothing can forbid that. What it buys them is a different
// subject within their own role — they read as somebody else of the same kind —
// and never a different role, because the role is the Postgres role the seam
// assumed and no claim decides it. That is the invariant the seven-role split
// was written for, and this is where it is measured against rows.
func TestOverwritingTheClaimsSubstitutesTheSubjectAndNotTheRole(t *testing.T) {
	c := newClinic(t)

	var current string
	var ownVisible, othersVisible int
	if err := c.as(t, patientID, "patient", func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			SELECT set_config('request.jwt.claims', $1, true)
		`, `{"sub":"`+otherPatientID+`","cadence_role":"admin"}`); err != nil {
			return err
		}

		return tx.QueryRow(ctx, `
			SELECT current_user,
			       (SELECT count(*) FROM app.profiles WHERE user_id = $1),
			       (SELECT count(*) FROM app.profiles WHERE user_id = $2)
		`, patientID, otherPatientID).Scan(&current, &ownVisible, &othersVisible)
	}); err != nil {
		t.Fatalf("overwriting the claims: %v", err)
	}

	// The substitution happened — this is the honest half, and without it the
	// assertion below would hold in a world where the setting could not be
	// written at all.
	if othersVisible != 1 {
		t.Error("overwriting the subject did not change which row is visible, so this test " +
			"proves nothing about what overwriting it does")
	}
	if ownVisible != 0 {
		t.Error("the caller still reads their own row after substituting another subject")
	}

	// And the role did not move. The claim said admin; the transaction is still
	// a patient, because the role is decided by the seam and by TO.
	if current != testsupport.PatientRole {
		t.Errorf("current_user = %q after claiming admin, want %q", current, testsupport.PatientRole)
	}
}

// A doctor reads a patient's card and their care team, and cannot read their
// preferences — the grants table leaves that row empty on purpose, because how
// somebody set up their reminders is not clinical information.
func TestADoctorReadsTheCardAndNotThePreferences(t *testing.T) {
	c := newClinic(t)

	var cards int
	if err := c.as(t, doctorID, "doctor", func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(
			ctx, `SELECT count(*) FROM app.patient_profiles WHERE user_id = $1`, patientID,
		).Scan(&cards)
	}); err != nil {
		t.Fatalf("the doctor cannot read an assigned patient's card: %v", err)
	}
	if cards != 1 {
		t.Errorf("%d card(s) visible to the assigned doctor, want 1", cards)
	}

	err := c.as(t, doctorID, "doctor", func(ctx context.Context, tx pgx.Tx) error {
		var rows int

		return tx.QueryRow(ctx, `SELECT count(*) FROM app.user_preferences`).Scan(&rows)
	})
	if err == nil {
		t.Fatal("a doctor read somebody's reminder settings")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("the read was refused with %v, want SQLSTATE 42501", err)
	}
}

// The tables the first version of this suite never touched, and the mutations
// that survived because of it: a patient reading every specialist's card, a
// doctor reading every patient's, a patient reading everybody's reminders, a
// doctor listing the whole care graph. Each is a set rather than a count.
func TestEveryTableAnswersWithTheCallersOwnRowsOnly(t *testing.T) {
	c := newClinic(t)

	for name, probe := range map[string]struct {
		subject, role, query string
		want                 []string
	}{
		"a patient reads the cards of their own specialists": {
			patientID, "patient",
			`SELECT user_id::text FROM app.provider_profiles ORDER BY user_id`,
			[]string{doctorID},
		},
		"a doctor reads their own card": {
			doctorID, "doctor",
			`SELECT user_id::text FROM app.provider_profiles ORDER BY user_id`,
			[]string{doctorID},
		},
		"a doctor reads the clinical cards of their own patients": {
			doctorID, "doctor",
			`SELECT user_id::text FROM app.patient_profiles ORDER BY user_id`,
			[]string{patientID},
		},
		"a patient reads their own clinical card": {
			patientID, "patient",
			`SELECT user_id::text FROM app.patient_profiles ORDER BY user_id`,
			[]string{patientID},
		},
		"a patient reads their own reminder settings": {
			patientID, "patient",
			`SELECT user_id::text FROM app.user_preferences ORDER BY user_id`,
			[]string{patientID},
		},
		"a patient lists their own care team": {
			patientID, "patient",
			`SELECT patient_id::text FROM app.care_team_assignments ORDER BY patient_id`,
			[]string{patientID},
		},
		"a doctor lists the assignments that name them": {
			doctorID, "doctor",
			`SELECT patient_id::text FROM app.care_team_assignments ORDER BY patient_id`,
			[]string{patientID},
		},
	} {
		t.Run(name, func(t *testing.T) {
			seen := c.visible(t, probe.subject, probe.role, probe.query)
			if !slices.Equal(seen, probe.want) {
				t.Errorf("saw %v, want exactly %v", seen, probe.want)
			}
		})
	}
}

// user_preferences is the only table where the patient holds a table-wide
// UPDATE, which makes its own-row policy the only thing scoping that write. Both
// halves are counted, because a nil error is not a witness.
func TestAPatientChangesTheirOwnRemindersAndNobodyElses(t *testing.T) {
	c := newClinic(t)

	if affected := c.changed(
		t, patientID, "patient",
		`UPDATE app.user_preferences SET lead_time_min = 60 WHERE user_id = $1`, patientID,
	); affected != 1 {
		t.Errorf("changing their own reminders touched %d row(s), want 1", affected)
	}

	if affected := c.changed(
		t, patientID, "patient",
		`UPDATE app.user_preferences SET lead_time_min = 15 WHERE user_id = $1`, otherPatientID,
	); affected != 0 {
		t.Errorf("changing somebody else's reminders touched %d row(s), want 0", affected)
	}
}

// The admin's write side. Without it the audit-log refusals above are consistent
// with an admin who can write nothing at all — and those refusals come from
// missing grants rather than from the policy, so nothing else in this file
// exercises a FOR ALL policy's WITH CHECK.
func TestTheAdminWritesWhatTheyAreAllowedTo(t *testing.T) {
	c := newClinic(t)

	if affected := c.changed(
		t, adminID, "admin",
		`UPDATE app.profiles SET full_name = 'Ирина Соколова-Ким' WHERE user_id = $1`, patientID,
	); affected != 1 {
		t.Errorf("the admin corrected %d profile(s), want 1", affected)
	}

	if affected := c.changed(
		t, adminID, "admin",
		`UPDATE app.patient_profiles SET dob = '1990-03-15' WHERE user_id = $1`, patientID,
	); affected != 1 {
		t.Errorf("the admin corrected %d card(s), want 1", affected)
	}

	if affected := c.changed(
		t, adminID, "admin",
		`DELETE FROM app.care_team_assignments WHERE patient_id = $1`, otherPatientID,
	); affected != 1 {
		t.Errorf("the admin removed %d assignment(s), want 1", affected)
	}
}

// The subject-less transaction, over every product role and every table it can
// reach — with a control proving the same counts are non-zero once a subject is
// published. Zero is also the answer in a world with no policies at all.
func TestNoSubjectMeansNoRowsForEveryProductRole(t *testing.T) {
	c := newClinic(t)
	ctx := t.Context()

	counts := func(tx pgx.Tx) (int, int, int, int) {
		t.Helper()

		var profiles, cards, providers, assignments int
		if err := tx.QueryRow(ctx, `
			SELECT (SELECT count(*) FROM app.profiles),
			       (SELECT count(*) FROM app.patient_profiles),
			       (SELECT count(*) FROM app.provider_profiles),
			       (SELECT count(*) FROM app.care_team_assignments)
		`).Scan(&profiles, &cards, &providers, &assignments); err != nil {
			t.Fatalf("counting: %v", err)
		}

		return profiles, cards, providers, assignments
	}

	for _, role := range []string{
		testsupport.PatientRole, testsupport.DoctorRole,
	} {
		t.Run(role, func(t *testing.T) {
			conn, err := c.request.Acquire(ctx)
			if err != nil {
				t.Fatalf("acquiring: %v", err)
			}
			defer conn.Release()

			tx, err := conn.Begin(ctx)
			if err != nil {
				t.Fatalf("beginning: %v", err)
			}
			defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

			if _, err := tx.Exec(ctx, `SELECT set_config('role', $1, true)`, role); err != nil {
				t.Fatalf("assuming %s: %v", role, err)
			}

			// The subject is absent, and said so directly rather than inferred
			// from a row count: a stale claim leaking across a pooled connection
			// is the bug closest to this test's subject, and it would still
			// produce zero rows here.
			var subject *string
			if err := tx.QueryRow(ctx, `SELECT app.jwt_subject()::text`).Scan(&subject); err != nil {
				t.Fatalf("reading the subject: %v", err)
			}
			if subject != nil {
				t.Fatalf("a subject is published (%q), so this measures nothing", *subject)
			}

			profiles, cards, providers, assignments := counts(tx)
			for what, count := range map[string]int{
				"profiles": profiles, "cards": cards,
				"provider cards": providers, "assignments": assignments,
			} {
				if count != 0 {
					t.Errorf("%d %s came back to a caller with no subject", count, what)
				}
			}
		})
	}

	// The control: with a subject published, the same counts are not zero.
	if err := c.as(t, patientID, "patient", func(ctx context.Context, tx pgx.Tx) error {
		profiles, cards, providers, assignments := counts(tx)
		if profiles == 0 || cards == 0 || providers == 0 || assignments == 0 {
			t.Errorf("a caller with a subject also sees nothing (%d/%d/%d/%d), so the zeros "+
				"above are not about the subject", profiles, cards, providers, assignments)
		}

		return nil
	}); err != nil {
		t.Fatalf("the control read: %v", err)
	}
}

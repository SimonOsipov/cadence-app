//go:build integration

package identity_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

const (
	firstDoctorID  = "8a1f3b7c-0000-4000-8000-0000000000c1"
	secondDoctorID = "8a1f3b7c-0000-4000-8000-0000000000c2"
)

// A doctor's signature on a clinical change has to mean that doctor, which is
// the whole motive of this step: a compromise of the API must not let anyone
// impersonate them.
//
// Two doctors, one patient each, and the rows read back by whose name is on
// them: with one doctor the assertion would hold for any actor of the right
// shape, and what has to hold is that the name on the row moves with the caller.
func TestAnAuditRowOnADoctorsBehalfNamesThatDoctor(t *testing.T) {
	db := cluster.NewDatabase(t)

	pool, err := database.NewPool(t.Context(), db.ServiceAppURL)
	if err != nil {
		t.Fatalf("opening the service pool: %v", err)
	}
	t.Cleanup(pool.Close)

	patients := map[string]string{
		firstDoctorID:  "8a1f3b7c-0000-4000-8000-0000000000d1",
		secondDoctorID: "8a1f3b7c-0000-4000-8000-0000000000d2",
	}

	if err := database.WithServiceJob(
		t.Context(), pool, provisioningJob,
		func(ctx context.Context, tx pgx.Tx) error {
			for _, doctor := range []string{firstDoctorID, secondDoctorID} {
				if _, err := tx.Exec(ctx, `
					INSERT INTO app.profiles (user_id, role, full_name, timezone)
					VALUES ($1, 'doctor', 'Врач', 'Europe/Moscow')
				`, doctor); err != nil {
					return err
				}
			}

			return nil
		},
	); err != nil {
		t.Fatalf("seeding the two doctors: %v", err)
	}

	for doctor, patient := range patients {
		request := auth.WithPrincipal(
			t.Context(), auth.Principal{Subject: doctor, Role: "doctor"},
		)

		newborn := newPatient()
		newborn.UserID = patient
		newborn.Specialists = []identity.Assignment{
			{ProviderID: doctor, CareRole: "endo", Primary: true},
		}

		if err := identity.CreatePatient(request, pool, newborn); err != nil {
			t.Fatalf("creating a patient on behalf of %s: %v", doctor, err)
		}
	}

	// Read by the superuser: the service role holds INSERT on audit_log and not
	// SELECT, and granting the test what it needs to see its own result would be
	// changing the arrangement it is checking.
	observer := testsupport.Connect(t, db.SuperuserURL)

	for doctor, patient := range patients {
		var actorID, actorJob string
		if err := observer.QueryRow(t.Context(), `
			SELECT coalesce(actor_id::text, ''), coalesce(actor_job, '')
			FROM app.audit_log
			WHERE patient_id = $1 AND action = 'patient.create'
		`, patient).Scan(&actorID, &actorJob); err != nil {
			t.Fatalf("reading the audit row for patient %s: %v", patient, err)
		}

		if actorID != doctor {
			t.Errorf("the row for patient %s is signed by %q, want the doctor who asked, %q",
				patient, actorID, doctor)
		}
		if actorJob != "" {
			t.Errorf("the row for patient %s also names the job %q; a doctor's action "+
				"recorded as a machine's is an unsigned change", patient, actorJob)
		}
	}
}

//go:build integration

package identity_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// The patients this file writes. doctorID, adminID and otherDoctorID come from the stand and the
// policy suite this package shares.
const (
	annaID  = "8a1f3b7c-0000-4000-8000-00000000000a"
	borisID = "8a1f3b7c-0000-4000-8000-00000000000b"
	veraID  = "8a1f3b7c-0000-4000-8000-00000000000c"
)

type seededPatient struct {
	id   string
	name string
	dob  *time.Time
	// Empty means nobody: a patient with no care team is what an admin sees and a doctor does not.
	assignedTo string
}

// rosterStand seeds a clinic of patients and returns the service over the request pool.
func rosterStand(t *testing.T, patients ...seededPatient) (*identity.Roster, *testsupport.Database) {
	t.Helper()

	servicePool, db := stand(t)

	if err := database.WithServiceJob(
		t.Context(), servicePool, provisioningJob,
		func(ctx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `
				INSERT INTO app.profiles (user_id, role, full_name) VALUES ($1, 'doctor', 'Игорь Седов')
			`, otherDoctorID); err != nil {
				return err
			}

			for _, patient := range patients {
				if _, err := tx.Exec(ctx, `
					INSERT INTO app.profiles (user_id, role, full_name) VALUES ($1, 'patient', $2)
				`, patient.id, patient.name); err != nil {
					return err
				}

				if _, err := tx.Exec(ctx, `
					INSERT INTO app.patient_profiles (user_id, dob) VALUES ($1, $2)
				`, patient.id, patient.dob); err != nil {
					return err
				}

				if patient.assignedTo == "" {
					continue
				}

				if _, err := tx.Exec(ctx, `
					INSERT INTO app.care_team_assignments (patient_id, provider_id, care_role, is_primary)
					VALUES ($1, $2, 'endo', true)
				`, patient.id, patient.assignedTo); err != nil {
					return err
				}
			}

			return nil
		},
	); err != nil {
		t.Fatalf("seeding the clinic: %v", err)
	}

	pool, err := database.NewPool(t.Context(), db.AppURL)
	if err != nil {
		t.Fatalf("opening the request pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return identity.NewRoster(pool), db
}

func namesSeenBy(t *testing.T, roster *identity.Roster, userID, role, cursor string, limit int) []string {
	t.Helper()

	page, err := roster.Patients(
		t.Context(), database.Caller{Subject: userID, Role: role}, cursor, limit,
	)
	if err != nil {
		t.Fatalf("reading the roster as %s: %v", role, err)
	}

	names := make([]string, 0, len(page.Patients))
	for _, patient := range page.Patients {
		names = append(names, patient.FullName)
	}

	return names
}

// The property the whole design rests on: one query, two doctors, two answers. Nothing in the Go
// names either doctor — the policy does the selecting, so a reassignment changes the answer with no
// edit here.
func TestTwoDoctorsReadDifferentRostersFromTheSameQuery(t *testing.T) {
	roster, _ := rosterStand(
		t,
		seededPatient{id: annaID, name: "Анна Петрова", assignedTo: doctorID},
		seededPatient{id: borisID, name: "Борис Ким", assignedTo: otherDoctorID},
	)

	mine := namesSeenBy(t, roster, doctorID, "doctor", "", 10)
	theirs := namesSeenBy(t, roster, otherDoctorID, "doctor", "", 10)

	if len(mine) != 1 || mine[0] != "Анна Петрова" {
		t.Errorf("the first doctor sees %v, want [Анна Петрова]", mine)
	}
	if len(theirs) != 1 || theirs[0] != "Борис Ким" {
		t.Errorf("the second doctor sees %v, want [Борис Ким]", theirs)
	}
}

// The administrator's row is USING (true), and a patient nobody is assigned to is the case that
// separates «everyone» from «the union of the doctors' rosters».
func TestAnAdministratorReadsEveryPatient(t *testing.T) {
	roster, _ := rosterStand(
		t,
		seededPatient{id: annaID, name: "Анна Петрова", assignedTo: doctorID},
		seededPatient{id: borisID, name: "Борис Ким", assignedTo: otherDoctorID},
		seededPatient{id: veraID, name: "Вера Ильина"},
	)

	seen := namesSeenBy(t, roster, adminID, "admin", "", 10)

	if len(seen) != 3 {
		t.Errorf("the administrator sees %v, want all three", seen)
	}
}

// Staff are not patients. Both doctors and the administrator are profiles this caller can read, so
// without role = 'patient' the doctor's own row and their colleague's arrive in the registry.
func TestTheRosterCarriesNoStaff(t *testing.T) {
	roster, _ := rosterStand(t, seededPatient{id: annaID, name: "Анна Петрова", assignedTo: doctorID})

	for _, name := range namesSeenBy(t, roster, adminID, "admin", "", 10) {
		if name != "Анна Петрова" {
			t.Errorf("the roster carries %q, which is not a patient", name)
		}
	}
}

// Reassignment takes effect through the policy alone — invariant 2 of the identity note, measured
// rather than asserted in prose.
func TestAReassignmentChangesWhatADoctorReads(t *testing.T) {
	roster, db := rosterStand(t, seededPatient{id: annaID, name: "Анна Петрова", assignedTo: otherDoctorID})

	if seen := namesSeenBy(t, roster, doctorID, "doctor", "", 10); len(seen) != 0 {
		t.Fatalf("the doctor sees %v before the assignment, want nothing", seen)
	}

	superuser := testsupport.Connect(t, db.SuperuserURL)
	if _, err := superuser.Exec(t.Context(), `
		UPDATE app.care_team_assignments SET provider_id = $1 WHERE patient_id = $2
	`, doctorID, annaID); err != nil {
		t.Fatalf("reassigning the patient: %v", err)
	}

	if seen := namesSeenBy(t, roster, doctorID, "doctor", "", 10); len(seen) != 1 {
		t.Errorf("the doctor sees %v after the assignment, want the patient", seen)
	}
}

// Age is the server's arithmetic, and absent when the clinic has not entered a date of birth.
func TestAgeIsWorkedOutByTheServer(t *testing.T) {
	born40YearsAgo := time.Now().UTC().AddDate(-40, 0, 1)

	roster, _ := rosterStand(
		t,
		seededPatient{id: annaID, name: "Анна Петрова", assignedTo: doctorID, dob: &born40YearsAgo},
		seededPatient{id: borisID, name: "Борис Ким", assignedTo: doctorID},
	)

	page, err := roster.Patients(t.Context(), database.Caller{Subject: doctorID, Role: "doctor"}, "", 10)
	if err != nil {
		t.Fatalf("reading the roster: %v", err)
	}

	byName := map[string]*int{}
	for _, patient := range page.Patients {
		byName[patient.FullName] = patient.Age
	}

	if got := byName["Анна Петрова"]; got == nil || *got != 39 {
		t.Errorf("Анна's age is %v, want 39 — a birthday one day away has not happened yet", got)
	}
	if got := byName["Борис Ким"]; got != nil {
		t.Errorf("Борис has age %v, want none: the clinic entered no date of birth", *got)
	}
}

// The paging property the step names, and the reason the cursor is keyset rather than an offset: a
// patient assigned between the two requests shifts every later row by one, and OFFSET 1 would then
// skip the row that moved into the gap.
func TestPagingSkipsNobodyWhenAnAssignmentArrivesBetweenPages(t *testing.T) {
	roster, db := rosterStand(
		t,
		seededPatient{id: borisID, name: "Борис Ким", assignedTo: doctorID},
		seededPatient{id: veraID, name: "Вера Ильина", assignedTo: doctorID},
		// Sorts before both and belongs to the other doctor until the reassignment below.
		seededPatient{id: annaID, name: "Анна Петрова", assignedTo: otherDoctorID},
	)

	first := namesSeenBy(t, roster, doctorID, "doctor", "", 1)
	if len(first) != 1 || first[0] != "Борис Ким" {
		t.Fatalf("the first page is %v, want [Борис Ким]", first)
	}

	page, err := roster.Patients(t.Context(), database.Caller{Subject: doctorID, Role: "doctor"}, "", 1)
	if err != nil {
		t.Fatalf("reading the first page: %v", err)
	}
	if page.Next == "" {
		t.Fatal("the first page of two carries no cursor for the second")
	}

	superuser := testsupport.Connect(t, db.SuperuserURL)
	if _, err := superuser.Exec(t.Context(), `
		UPDATE app.care_team_assignments SET provider_id = $1 WHERE patient_id = $2
	`, doctorID, annaID); err != nil {
		t.Fatalf("reassigning the patient: %v", err)
	}

	second := namesSeenBy(t, roster, doctorID, "doctor", page.Next, 1)

	// Вера and not Борис: the row that arrived sorts before the cursor, so it is not on this page —
	// which is the correct answer, and the one an offset would have got wrong by repeating Борис.
	if len(second) != 1 || second[0] != "Вера Ильина" {
		t.Errorf("the second page is %v, want [Вера Ильина]", second)
	}
}

// Two patients with the same name: the case a cursor keyed on the name alone cannot page through,
// because «after Иванов» is ambiguous between them.
func TestPagingSeparatesTwoPatientsOfTheSameName(t *testing.T) {
	roster, _ := rosterStand(
		t,
		seededPatient{id: annaID, name: "Иван Иванов", assignedTo: doctorID},
		seededPatient{id: borisID, name: "Иван Иванов", assignedTo: doctorID},
	)

	page, err := roster.Patients(t.Context(), database.Caller{Subject: doctorID, Role: "doctor"}, "", 1)
	if err != nil {
		t.Fatalf("reading the first page: %v", err)
	}
	if len(page.Patients) != 1 || page.Next == "" {
		t.Fatalf("the first page is %v with cursor %q, want one row and a cursor", page.Patients, page.Next)
	}

	second, err := roster.Patients(
		t.Context(), database.Caller{Subject: doctorID, Role: "doctor"}, page.Next, 1,
	)
	if err != nil {
		t.Fatalf("reading the second page: %v", err)
	}

	if len(second.Patients) != 1 {
		t.Fatalf("the second page is %v, want the other Иванов", second.Patients)
	}
	if second.Patients[0].UserID == page.Patients[0].UserID {
		t.Error("the second page repeats the first one's patient, so the name alone is the cursor")
	}
}

// The last page carries no cursor, or a client pages for ever.
func TestTheLastPageSaysSo(t *testing.T) {
	roster, _ := rosterStand(t, seededPatient{id: annaID, name: "Анна Петрова", assignedTo: doctorID})

	page, err := roster.Patients(t.Context(), database.Caller{Subject: doctorID, Role: "doctor"}, "", 10)
	if err != nil {
		t.Fatalf("reading the roster: %v", err)
	}

	if page.Next != "" {
		t.Errorf("the last page carries cursor %q", page.Next)
	}
}

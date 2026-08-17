//go:build integration

package identity_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// The patients this file writes. doctorID, adminID and otherDoctorID come from the stand and the
// policy suite this package shares.
// Against the alphabet on purpose: a roster ordered by id is then a different sequence.
const (
	annaID  = "8a1f3b7c-0000-4000-8000-00000000000c"
	galyaID = "8a1f3b7c-0000-4000-8000-00000000000d"
	borisID = "8a1f3b7c-0000-4000-8000-00000000000b"
	veraID  = "8a1f3b7c-0000-4000-8000-00000000000a"
)

type seededPatient struct {
	id   string
	name string
	dob  *time.Time
	// Empty means nobody: a patient with no care team is what an admin sees and a doctor does not.
	assignedTo string

	// A profile with no clinical card. The creation path writes both rows in one transaction, so this
	// is a state of the schema rather than of the product — and the join has to survive it.
	noCard bool
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

				if !patient.noCard {
					if _, err := tx.Exec(ctx, `
						INSERT INTO app.patient_profiles (user_id, dob) VALUES ($1, $2)
					`, patient.id, patient.dob); err != nil {
						return err
					}
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
		// Ё is the letter that separates the Russian alphabet from the byte order: under the database's
		// own collation it sorts before А, and a doctor looking for Ёлкина finds her above everyone.
		seededPatient{id: galyaID, name: "Ёлкина Мария", assignedTo: doctorID},
	)

	seen := namesSeenBy(t, roster, adminID, "admin", "", 10)

	// Set and order together: a size holds for the wrong three, and the contract says «ordered by name».
	want := []string{"Анна Петрова", "Борис Ким", "Вера Ильина", "Ёлкина Мария"}
	if !slices.Equal(seen, want) {
		t.Errorf("the administrator sees %v, want %v", seen, want)
	}
}

// Staff are not patients. Both doctors and the administrator are profiles this caller can read, so
// without role = 'patient' the doctor's own row and their colleague's arrive in the registry.
func TestTheRosterCarriesNoStaff(t *testing.T) {
	roster, _ := rosterStand(t, seededPatient{id: annaID, name: "Анна Петрова", assignedTo: doctorID})

	// Both callers, compared whole: as a loop this passed on the empty answer a broken predicate gives.
	want := []string{"Анна Петрова"}

	if seen := namesSeenBy(t, roster, doctorID, "doctor", "", 10); !slices.Equal(seen, want) {
		t.Errorf("the doctor's roster is %v, want %v", seen, want)
	}
	if seen := namesSeenBy(t, roster, adminID, "admin", "", 10); !slices.Equal(seen, want) {
		t.Errorf("the administrator's roster is %v, want %v", seen, want)
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
	// Three points around one boundary, and the expectations computed by Go rather than written down:
	// a literal 39 is only a discriminator on the days of the year when it happens to be one, and on
	// 31 December the day-before-birthday row agrees with a naive difference of years.
	tomorrow := time.Now().UTC().AddDate(-40, 0, 1)
	today := time.Now().UTC().AddDate(-40, 0, 0)
	yesterday := time.Now().UTC().AddDate(-40, 0, -1)

	roster, _ := rosterStand(
		t,
		seededPatient{id: annaID, name: "Анна Петрова", assignedTo: doctorID, dob: &tomorrow},
		seededPatient{id: borisID, name: "Борис Ким", assignedTo: doctorID},
		seededPatient{id: veraID, name: "Вера Ильина", assignedTo: doctorID, dob: &today},
		seededPatient{id: galyaID, name: "Галина Русу", assignedTo: doctorID, dob: &yesterday},
	)

	page, err := roster.Patients(t.Context(), database.Caller{Subject: doctorID, Role: "doctor"}, "", 10)
	if err != nil {
		t.Fatalf("reading the roster: %v", err)
	}

	byName := map[string]*int{}
	for _, patient := range page.Patients {
		byName[patient.FullName] = patient.Age
	}

	if len(page.Patients) != 4 {
		t.Fatalf("the roster is %v, want all four: an absent row makes every assertion below vacuous", page.Patients)
	}

	for _, expected := range []struct {
		name string
		dob  time.Time
	}{
		{"Анна Петрова", tomorrow},
		{"Вера Ильина", today},
		{"Галина Русу", yesterday},
	} {
		want := yearsSince(expected.dob)

		if got := byName[expected.name]; got == nil || *got != want {
			t.Errorf("%s's age is %v, want %d", expected.name, got, want)
		}
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

// Walking to the end with a page wider than one row. Every other paging test here uses a page of one,
// where «the cursor is the last row» and «the cursor is the first row» are the same statement — and
// the second of those repeats a page for ever.
func TestPagingWalksTheWholeRosterWithoutRepeatingARow(t *testing.T) {
	roster, _ := rosterStand(
		t,
		seededPatient{id: annaID, name: "Анна Петрова", assignedTo: doctorID},
		seededPatient{id: borisID, name: "Борис Ким", assignedTo: doctorID},
		seededPatient{id: veraID, name: "Вера Ильина", assignedTo: doctorID},
	)

	caller := database.Caller{Subject: doctorID, Role: "doctor"}

	seen := map[string]int{}
	cursor := ""

	// Bounded, because the failure this exists for is a walk that never ends: five requests is more
	// than the three rows can need at two a page.
	for range 5 {
		page, err := roster.Patients(t.Context(), caller, cursor, 2)
		if err != nil {
			t.Fatalf("reading a page: %v", err)
		}

		for _, patient := range page.Patients {
			seen[patient.FullName]++
		}

		if page.Next == "" {
			break
		}
		cursor = page.Next
	}

	if len(seen) != 3 {
		t.Errorf("the walk saw %v, want all three patients", seen)
	}
	for name, times := range seen {
		if times != 1 {
			t.Errorf("%s was on %d pages, want exactly one", name, times)
		}
	}
}

// A page filled exactly to the limit is the last one, and it must say so. Without this a roster whose
// size is a multiple of the page size hands out a cursor to nothing — «show more», and then nothing.
func TestAFullLastPageCarriesNoCursor(t *testing.T) {
	roster, _ := rosterStand(
		t,
		seededPatient{id: annaID, name: "Анна Петрова", assignedTo: doctorID},
		seededPatient{id: borisID, name: "Борис Ким", assignedTo: doctorID},
	)

	page, err := roster.Patients(t.Context(), database.Caller{Subject: doctorID, Role: "doctor"}, "", 2)
	if err != nil {
		t.Fatalf("reading the roster: %v", err)
	}

	if len(page.Patients) != 2 {
		t.Fatalf("the page carries %d rows, want both", len(page.Patients))
	}
	if page.Next != "" {
		t.Errorf("a page holding every remaining row carries cursor %q", page.Next)
	}
}

// A doctor with nobody assigned: the first day of a new hire, and the state the 403 to a patient
// exists to keep distinguishable. The empty page is a page and not a null.
func TestADoctorWithNoPatientsReadsAnEmptyPage(t *testing.T) {
	roster, _ := rosterStand(t, seededPatient{id: annaID, name: "Анна Петрова", assignedTo: otherDoctorID})

	page, err := roster.Patients(t.Context(), database.Caller{Subject: doctorID, Role: "doctor"}, "", 8)
	if err != nil {
		t.Fatalf("reading the roster: %v", err)
	}

	if len(page.Patients) != 0 {
		t.Fatalf("the roster is %v, want nothing", page.Patients)
	}
	if page.Patients == nil {
		t.Error("the empty roster is a nil slice, which reaches the wire as null rather than []")
	}
}

// The mounted route with a real token: the principal-to-Caller conversion and the composition root's
// pool are otherwise unmeasured.
func TestTheMountedRouteAnswersTheDoctorsOwnRoster(t *testing.T) {
	walked := walkTheCycle(t)

	request := httptest.NewRequest(http.MethodGet, "/v1/dashboard/overview", nil)
	request.Header.Set("Authorization", "Bearer "+walked.doctor.access)

	rec := httptest.NewRecorder()
	walked.clinic.mux.ServeHTTP(rec, request)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	var page identity.RosterPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decoding the page: %v", err)
	}

	// The patient walkTheCycle created for this doctor, and nobody else's: the other doctor's patient
	// exists in the same clinic, so this is the policy answering rather than the table.
	if len(page.Patients) != 1 {
		t.Fatalf("the doctor's roster is %v, want exactly their own patient", page.Patients)
	}
	if page.Patients[0].UserID != walked.patient.account {
		t.Errorf("the roster carries %q, want %q", page.Patients[0].UserID, walked.patient.account)
	}
}

// A patient's own token against the mounted route, so the refusal is measured where a device meets it
// rather than at the mapper.
func TestTheMountedRouteRefusesAPatientsToken(t *testing.T) {
	walked := walkTheCycle(t)

	request := httptest.NewRequest(http.MethodGet, "/v1/dashboard/overview", nil)
	request.Header.Set("Authorization", "Bearer "+walked.patient.access)

	rec := httptest.NewRecorder()
	walked.clinic.mux.ServeHTTP(rec, request)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body %s", rec.Code, rec.Body)
	}
}

// Why the join is a LEFT one: an inner join takes such a patient off their doctor's registry entirely.
func TestAPatientWithNoCardIsStillOnTheRoster(t *testing.T) {
	roster, _ := rosterStand(
		t,
		seededPatient{id: annaID, name: "Анна Петрова", assignedTo: doctorID, noCard: true},
	)

	page, err := roster.Patients(t.Context(), database.Caller{Subject: doctorID, Role: "doctor"}, "", 8)
	if err != nil {
		t.Fatalf("reading the roster: %v", err)
	}

	if len(page.Patients) != 1 {
		t.Fatalf("the roster is %v, want the patient whose card is missing", page.Patients)
	}
	if page.Patients[0].Age != nil {
		t.Errorf("the age is %v, want none: there is no card to read a date of birth from", *page.Patients[0].Age)
	}
}

// yearsSince is Go's own date arithmetic standing against Postgres': two implementations of the same
// rule, which is what makes the age assertions above discriminating on every day of the year rather
// than on the days a written-down number happens to be right.
func yearsSince(dob time.Time) int {
	now := time.Now().UTC()

	years := now.Year() - dob.Year()
	if now.YearDay() < dob.YearDay() {
		years--
	}

	return years
}

// The query parameters, which nothing drove through the route: every other paging test calls Patients
// directly, so replacing input.Cursor with "" at the call site — every «show more» silently reopening
// the first page — survives them all.
func TestTheMountedRoutePagesByItsOwnParameters(t *testing.T) {
	walked := walkTheCycle(t)
	walked.clinic.take(t, walked.doctor, "second.patient@clinic.example")

	first := rosterThrough(t, walked, walked.doctor.access, "?limit=1")
	if len(first.Patients) != 1 || first.Next == "" {
		t.Fatalf("the first page is %v with cursor %q, want one row and a cursor", first.Patients, first.Next)
	}

	second := rosterThrough(t, walked, walked.doctor.access, "?limit=1&cursor="+url.QueryEscape(first.Next))
	if len(second.Patients) != 1 {
		t.Fatalf("the second page is %v, want the other patient", second.Patients)
	}

	if second.Patients[0].UserID == first.Patients[0].UserID {
		t.Error("the second page repeats the first one's patient, so the cursor did not reach the query")
	}
}

// The empty roster on the wire rather than in the struct: a doctor hired and not yet given anybody is
// the only caller that produces it, and the encoding is what a browser then calls .map on.
func TestTheMountedRouteAnswersAnEmptyRosterAsAnArray(t *testing.T) {
	walked := walkTheCycle(t)
	newcomer := walked.clinic.hire(t, walked.admin, "newcomer@clinic.example", "Эндокринолог")

	request := httptest.NewRequest(http.MethodGet, "/v1/dashboard/overview", nil)
	request.Header.Set("Authorization", "Bearer "+newcomer.access)

	rec := httptest.NewRecorder()
	walked.clinic.mux.ServeHTTP(rec, request)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	if body := strings.TrimSpace(rec.Body.String()); !strings.Contains(body, `"patients":[]`) {
		t.Errorf("the empty roster reached the wire as %s", body)
	}
}

func rosterThrough(t *testing.T, walked cycleWalk, bearer, query string) identity.RosterPage {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, "/v1/dashboard/overview"+query, nil)
	request.Header.Set("Authorization", "Bearer "+bearer)

	rec := httptest.NewRecorder()
	walked.clinic.mux.ServeHTTP(rec, request)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	var page identity.RosterPage
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decoding the page: %v", err)
	}

	return page
}

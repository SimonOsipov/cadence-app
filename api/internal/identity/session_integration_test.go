//go:build integration

package identity_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// What all three profiles start out reporting, written by the helper below rather than inherited from stand's
// literals: a test asserting a row was left alone must not rest on a value another file happens to seed.
const seededZone = "Europe/Moscow"

// standWithAPatient adds the patient, puts the doctor on their care team, and builds the service over the request
// pool. The care team is not decoration: without it the patient can read no profile but their own, and «nobody
// else's row was touched» would be a claim about a row the caller cannot reach in the first place.
func standWithAPatient(t *testing.T) (*identity.Sessions, *testsupport.Database) {
	t.Helper()

	servicePool, db := stand(t)

	if err := database.WithServiceJob(
		t.Context(), servicePool, provisioningJob,
		func(ctx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `
				INSERT INTO app.profiles (user_id, role, full_name, timezone)
				VALUES ($1, 'patient', 'Марина Волкова', $2)
			`, patientID, seededZone); err != nil {
				return err
			}

			_, err := tx.Exec(ctx, `
				INSERT INTO app.care_team_assignments (patient_id, provider_id, care_role, is_primary)
				VALUES ($1, $2, 'endo', true)
			`, patientID, doctorID)

			return err
		},
	); err != nil {
		t.Fatalf("seeding the patient: %v", err)
	}

	superuser := testsupport.Connect(t, db.SuperuserURL)
	if _, err := superuser.Exec(t.Context(), `
		UPDATE app.profiles SET timezone = $1 WHERE user_id = ANY($2)
	`, seededZone, []string{doctorID, adminID}); err != nil {
		t.Fatalf("giving the staff a zone to start from: %v", err)
	}

	requestPool, err := database.NewPool(t.Context(), db.AppURL)
	if err != nil {
		t.Fatalf("opening the request pool: %v", err)
	}
	t.Cleanup(requestPool.Close)

	return identity.NewSessions(requestPool), db
}

// zoneOf reads as the superuser, so the assertion is about the row rather than about what the caller's own policies
// would let them see. An empty string is a NULL column.
func zoneOf(t *testing.T, db *testsupport.Database, userID string) string {
	t.Helper()

	superuser := testsupport.Connect(t, db.SuperuserURL)

	var zone *string
	if err := superuser.QueryRow(t.Context(), `
		SELECT timezone FROM app.profiles WHERE user_id = $1
	`, userID).Scan(&zone); err != nil {
		t.Fatalf("reading the timezone of %s: %v", userID, err)
	}

	if zone == nil {
		return ""
	}

	return *zone
}

func report(
	t *testing.T, sessions *identity.Sessions, principal auth.Principal, zone string,
) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	servedBy(&principal, sessions).ServeHTTP(rec, reporting(zone))

	return rec
}

func TestASessionRecordsThePatientsOwnTimezone(t *testing.T) {
	sessions, db := standWithAPatient(t)

	rec := report(t, sessions, auth.Principal{Subject: patientID, Role: "patient"}, "Asia/Tbilisi")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body %s", rec.Code, rec.Body)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("the response carries %s, want nothing", rec.Body)
	}
	if got := zoneOf(t, db, patientID); got != "Asia/Tbilisi" {
		t.Errorf("the patient's timezone is %q, want %q", got, "Asia/Tbilisi")
	}
}

// The first report a device ever makes. The column starts NULL on a created patient — that is what
// TestACreatedPatientHasNoTimezoneYet pins — and every fixture here starts it filled, so this is the one path that
// is not an overwrite.
func TestTheFirstReportFillsAnEmptyTimezone(t *testing.T) {
	sessions, db := standWithAPatient(t)

	superuser := testsupport.Connect(t, db.SuperuserURL)
	if _, err := superuser.Exec(t.Context(), `
		UPDATE app.profiles SET timezone = NULL WHERE user_id = $1
	`, patientID); err != nil {
		t.Fatalf("emptying the timezone: %v", err)
	}

	rec := report(t, sessions, auth.Principal{Subject: patientID, Role: "patient"}, "Asia/Tbilisi")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body %s", rec.Code, rec.Body)
	}
	if got := zoneOf(t, db, patientID); got != "Asia/Tbilisi" {
		t.Errorf("the patient's timezone is %q, want %q", got, "Asia/Tbilisi")
	}
}

// The spellings the probe must refuse. Whole-name equality is the claim, and each row is a mutation of it that one
// absurd value would not catch: a prefix survives LIKE $1 || '%', the wrong case survives ILIKE, and a fragment
// survives position($1 in name) > 0. Any of them would be stored verbatim and break the first AT TIME ZONE
// downstream.
func TestASessionRefusesAZoneTheServerDoesNotKnow(t *testing.T) {
	for _, zone := range []string{
		"Mars/Olympus",
		"Europe/Mos",
		"europe/moscow",
		"Europe/Moscow ",
		"MSK",
		"rope/Mosc",
	} {
		t.Run(zone, func(t *testing.T) {
			sessions, db := standWithAPatient(t)

			rec := report(t, sessions, auth.Principal{Subject: patientID, Role: "patient"}, zone)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body)
			}
			if got := zoneOf(t, db, patientID); got != seededZone {
				t.Errorf("the patient's timezone is %q, want it untouched at %q", got, seededZone)
			}
		})
	}
}

// Staff, measured against the database rather than against the branch in Go.
func TestASessionLeavesAStaffRowAlone(t *testing.T) {
	for _, staff := range []struct {
		role   string
		userID string
	}{
		{"doctor", doctorID},
		{"admin", adminID},
	} {
		t.Run(staff.role, func(t *testing.T) {
			sessions, db := standWithAPatient(t)

			rec := report(t, sessions, auth.Principal{Subject: staff.userID, Role: staff.role}, "Asia/Tbilisi")

			if rec.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want 204; body %s", rec.Code, rec.Body)
			}
			if got := zoneOf(t, db, staff.userID); got != seededZone {
				t.Errorf("the %s's timezone is %q, want it untouched at %q", staff.role, got, seededZone)
			}
		})
	}
}

// The method, not the handler. RecordTimezone is exported and takes whatever Caller it is given, and under
// cadence_admin the profiles policy is USING (true) over a table-wide grant — so an UPDATE without its predicate
// rewrites every profile in the clinic and reports no error at all. The handler's staff branch cannot be what that
// rests on: it is a different function in a different layer.
func TestRecordingUnderAnAdminCallerTouchesNobodyElse(t *testing.T) {
	sessions, db := standWithAPatient(t)

	caller := database.Caller{Subject: adminID, Role: "admin"}
	if err := sessions.RecordTimezone(t.Context(), caller, "Asia/Tbilisi"); err != nil {
		t.Fatalf("recording as the administrator: %v", err)
	}

	if got := zoneOf(t, db, patientID); got != seededZone {
		t.Errorf("the patient's timezone is %q, want it untouched at %q", got, seededZone)
	}
	if got := zoneOf(t, db, doctorID); got != seededZone {
		t.Errorf("the doctor's timezone is %q, want it untouched at %q", got, seededZone)
	}
	if got := zoneOf(t, db, adminID); got != "Asia/Tbilisi" {
		t.Errorf("the administrator's own timezone is %q, want %q", got, "Asia/Tbilisi")
	}
}

// Named rather than merely a 500: the sentinel separates «this account has no profile» from a database that failed,
// and a test asserting only the status passes for either.
func TestRecordingRefusesWhenThereIsNoProfileToRecordAgainst(t *testing.T) {
	sessions, _ := standWithAPatient(t)

	stranger := database.Caller{Subject: "8a1f3b7c-0000-4000-8000-00000000009f", Role: "patient"}

	err := sessions.RecordTimezone(t.Context(), stranger, "Asia/Tbilisi")
	if !errors.Is(err, identity.ErrNoProfileToRecordAgainst) {
		t.Fatalf("recording against a missing profile answered %v, want ErrNoProfileToRecordAgainst", err)
	}
}

func TestASessionRefusesWhenThereIsNoProfileToRecordAgainst(t *testing.T) {
	sessions, _ := standWithAPatient(t)

	stranger := auth.Principal{Subject: "8a1f3b7c-0000-4000-8000-00000000009f", Role: "patient"}

	rec := report(t, sessions, stranger, "Asia/Tbilisi")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body %s", rec.Code, rec.Body)
	}
}

// A row the caller can see and must not write. Without the care team seeded above the patient can read no profile
// but their own, and this would hold for the wrong reason.
func TestASessionWritesNobodyElsesRow(t *testing.T) {
	sessions, db := standWithAPatient(t)

	rec := report(t, sessions, auth.Principal{Subject: patientID, Role: "patient"}, "Asia/Tbilisi")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body %s", rec.Code, rec.Body)
	}
	if got := zoneOf(t, db, doctorID); got != seededZone {
		t.Errorf("the doctor's timezone is %q, want it untouched at %q", got, seededZone)
	}
}

// The wiring, which nothing else measures: every other test here builds the service itself, so the pool the
// composition root hands it goes unmeasured. Given the service pool instead of the request one, the seam cannot
// assume cadence_patient and this is where that shows.
func TestTheMountedRouteRecordsAPatientsTimezone(t *testing.T) {
	walked := walkTheCycle(t)

	said := walked.clinic.send(t, "/v1/me/session", walked.patient.access, `{"timezone":"Asia/Tbilisi"}`)
	if said.status != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body %s", said.status, said.body)
	}

	zones := walked.clinic.visibleTo(
		t, walked.clinic.whoTheTokenSays(t, walked.patient.access),
		// coalesce unqualified: it is grammar rather than a function, and pg_catalog.coalesce does not exist.
		`SELECT coalesce(timezone, '') FROM app.profiles WHERE user_id = app.jwt_subject()`,
	)

	if len(zones) != 1 || zones[0] != "Asia/Tbilisi" {
		t.Errorf("the patient's own row reads %v, want [Asia/Tbilisi]", zones)
	}
}

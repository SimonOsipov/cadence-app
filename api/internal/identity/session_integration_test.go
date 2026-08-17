//go:build integration

package identity_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// seededZone is what every profile in this file starts out reporting, so that a
// test asserting a zone was written cannot pass on the value it began with.
const seededZone = "Europe/Moscow"

// standWithAPatient adds the patient to the stand and builds the service over
// the request pool — the one operation here that writes under the caller's own
// identity rather than through the service path.
func standWithAPatient(t *testing.T) (*identity.Sessions, *testsupport.Database) {
	t.Helper()

	servicePool, db := stand(t)

	if err := database.WithServiceJob(
		t.Context(), servicePool, provisioningJob,
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.profiles (user_id, role, full_name, timezone)
				VALUES ($1, 'patient', 'Марина Волкова', $2)
			`, patientID, seededZone)

			return err
		},
	); err != nil {
		t.Fatalf("seeding the patient: %v", err)
	}

	requestPool, err := database.NewPool(t.Context(), db.AppURL)
	if err != nil {
		t.Fatalf("opening the request pool: %v", err)
	}
	t.Cleanup(requestPool.Close)

	return identity.NewSessions(requestPool), db
}

// zoneOf reads a profile the way nothing on the request path can: as the
// superuser, so the assertion is about the row rather than about what the
// caller's own policies would let them see.
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

func report(t *testing.T, sessions *identity.Sessions, principal auth.Principal, zone string) *httptest.ResponseRecorder {
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
	if got := zoneOf(t, db, patientID); got != "Asia/Tbilisi" {
		t.Errorf("the patient's timezone is %q, want %q", got, "Asia/Tbilisi")
	}
}

// The zone is checked against the server that stores it, and the refusal leaves
// the row as it was: a validation that answered 400 after writing would be worse
// than none, because the caller would be told nothing happened.
func TestASessionRefusesAZoneTheServerDoesNotKnow(t *testing.T) {
	sessions, db := standWithAPatient(t)

	rec := report(t, sessions, auth.Principal{Subject: patientID, Role: "patient"}, "Mars/Olympus")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body)
	}
	if got := zoneOf(t, db, patientID); got != seededZone {
		t.Errorf("the patient's timezone is %q, want it untouched at %q", got, seededZone)
	}
}

// The staff half, measured against the database rather than against the branch
// in Go: the doctor's row is read back, so a handler that stopped short-circuiting
// and let the write run would fail here rather than pass on a 42501 mistaken for
// success.
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

// A patient token whose profile is not there. The issuance hook does not put
// cadence_role: patient on a token without one, so this is a broken invariant
// rather than an ordinary miss — and the point of the assertion is that the
// write reports it instead of answering 204 for a row it never touched.
func TestASessionRefusesWhenThereIsNoProfileToRecordAgainst(t *testing.T) {
	sessions, _ := standWithAPatient(t)

	stranger := auth.Principal{Subject: "8a1f3b7c-0000-4000-8000-00000000009f", Role: "patient"}

	rec := report(t, sessions, stranger, "Asia/Tbilisi")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body %s", rec.Code, rec.Body)
	}
}

// The seam is what the write rests on: without the caller's claims published on
// the transaction the policy matches nothing, so this asserts the row moved for
// the caller who owns it and for nobody else.
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

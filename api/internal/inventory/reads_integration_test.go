//go:build integration

package inventory_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/inventory"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

// Half past midnight on 10 May in Yekaterinburg, where these patients live, and still the 9th
// in UTC — so an implementation reading the server's own day answers about a different one and
// this suite says so.
var theCabinetHour = time.Date(2026, time.May, 9, 19, 30, 0, 0, time.UTC)

// aCabinet mounts the reads over the request pool. No object store: these endpoints answer
// whether a label photograph exists and never its key, so signing has nothing to do here.
func aCabinet(t *testing.T, c clinic) (*chi.Mux, func(subject, role string)) {
	t.Helper()

	var subject, role string
	mux := chi.NewRouter()
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if subject == "" {
				next.ServeHTTP(w, r)

				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(
				r.Context(), auth.Principal{Subject: subject, Role: role},
			)))
		})
	})
	inventory.NewService(func() time.Time { return theCabinetHour }, inventory.Deps{
		RequestPool: c.request,
	}).Register(httpserver.NewAPI(mux))

	return mux, func(s, r string) { subject, role = s, r }
}

type cabinetBody struct {
	Vials []struct {
		ID                 string `json:"id"`
		CompoundID         string `json:"compound_id"`
		ConcentrationLabel string `json:"concentration_label"`
		TotalAmount        struct {
			Value float64 `json:"value"`
			Unit  string  `json:"unit"`
		} `json:"total_amount"`
		RemainingAmount struct {
			Value float64 `json:"value"`
			Unit  string  `json:"unit"`
		} `json:"remaining_amount"`
		DosesLeft   *int `json:"doses_left"`
		CurrentDose *struct {
			Value float64 `json:"value"`
			Unit  string  `json:"unit"`
		} `json:"current_dose"`
		Status        string  `json:"status"`
		OpenedAt      *string `json:"opened_at"`
		ExpiresOn     string  `json:"expires_on"`
		DisposedAt    *string `json:"disposed_at"`
		HasLabelPhoto bool    `json:"has_label_photo"`
	} `json:"vials"`
	Reorder []struct {
		CompoundID string `json:"compound_id"`
		WeeksLeft  int    `json:"weeks_left"`
	} `json:"reorder"`
}

func askCabinet(t *testing.T, mux *chi.Mux, path string) (int, []byte) {
	t.Helper()

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

	return recorder.Code, recorder.Body.Bytes()
}

func cabinetOf(t *testing.T, mux *chi.Mux) cabinetBody {
	t.Helper()

	status, body := askCabinet(t, mux, "/v1/me/vials")
	if status != http.StatusOK {
		t.Fatalf("the cabinet answered %d: %s", status, body)
	}
	var shelf cabinetBody
	if err := json.Unmarshal(body, &shelf); err != nil {
		t.Fatalf("reading the cabinet: %v", err)
	}

	return shelf
}

// What is left, what it buys, and the hint about buying more — one read, computed, none of it
// stored.
func TestTheCabinetAnswersWhatIsLeftAndWhatItBuys(t *testing.T) {
	c := newClinic(t)
	prescribe(t, c, patientA)
	openTheVial(t, c, patientA, c.vialA)
	drawADose(t, c, patientA, c.vialA, 0)

	mux, calling := aCabinet(t, c)
	calling(patientA, "patient")

	shelf := cabinetOf(t, mux)

	if len(shelf.Vials) != 1 {
		t.Fatalf("the cabinet holds %d vials, want the caller's one", len(shelf.Vials))
	}
	vial := shelf.Vials[0]
	if vial.ID != c.vialA {
		t.Errorf("the cabinet answered vial %s, want %s", vial.ID, c.vialA)
	}
	// A milligram in the box, a quarter of it drawn: three quarters left, and the count
	// is what that buys at today's dose rather than what the box held.
	if vial.RemainingAmount.Value != 0.75 || vial.RemainingAmount.Unit != "мг" {
		t.Errorf("%v %s left of a milligram after one 0,25 мг dose",
			vial.RemainingAmount.Value, vial.RemainingAmount.Unit)
	}
	if vial.TotalAmount.Value != 1 {
		t.Errorf("the box held %v мг", vial.TotalAmount.Value)
	}
	if vial.DosesLeft == nil || *vial.DosesLeft != 3 {
		t.Errorf("the vial buys %v injections, want 3", vial.DosesLeft)
	}
	if vial.CurrentDose == nil || vial.CurrentDose.Value != 0.25 {
		t.Errorf("today's dose reads %v, want 0,25 мг", vial.CurrentDose)
	}
	if vial.Status != "active" {
		t.Errorf("an opened vial with three quarters left reads %q", vial.Status)
	}
	if vial.HasLabelPhoto {
		t.Error("a vial with no photograph says it has one")
	}
	// Three doses at one a week is inside the threshold, and the hint is about the
	// compound rather than the course item that produced it.
	if len(shelf.Reorder) != 1 || shelf.Reorder[0].CompoundID != c.compound {
		t.Fatalf("the shelf answers hints %+v", shelf.Reorder)
	}
	if shelf.Reorder[0].WeeksLeft != 3 {
		t.Errorf("the hint says %d weeks, want 3", shelf.Reorder[0].WeeksLeft)
	}
}

// §03's rule, and the reason the two numbers are not one field: substance is a fact about the
// vial, and injections are a fact about a prescription.
func TestWithoutARunningCourseTheSubstanceAnswersAndTheCountDoesNot(t *testing.T) {
	c := newClinic(t)
	openTheVial(t, c, patientA, c.vialA)

	mux, calling := aCabinet(t, c)
	calling(patientA, "patient")

	shelf := cabinetOf(t, mux)

	if len(shelf.Vials) != 1 {
		t.Fatalf("the cabinet holds %d vials", len(shelf.Vials))
	}
	vial := shelf.Vials[0]
	if vial.DosesLeft != nil {
		t.Errorf("the card counts %d injections with nothing prescribed", *vial.DosesLeft)
	}
	if vial.CurrentDose != nil {
		t.Errorf("a dose of %v is in force with no course", vial.CurrentDose)
	}
	if vial.RemainingAmount.Value != 1 {
		t.Errorf("the substance left reads %v, want the whole milligram", vial.RemainingAmount.Value)
	}
	if vial.Status != "active" {
		t.Errorf("the status reads %q with no course to prescribe from", vial.Status)
	}
	if len(shelf.Reorder) != 0 {
		t.Errorf("a hint was given with no course: %+v", shelf.Reorder)
	}
}

// The day the counts are computed against is the patient's, not the server's — and at this
// instant the two disagree.
func TestTheCabinetIsAnsweredInThePatientsOwnDay(t *testing.T) {
	c := newClinic(t)
	openTheVial(t, c, patientA, c.vialA)
	// Fourteen days from the patient's day and fifteen from the server's, which is the
	// only place the two answer differently: «expiring» covers a vial already past its
	// date as well, so an expiry in the past cannot tell the days apart at all.
	expireOn(t, c, patientA, c.vialA, "2026-05-24")

	mux, calling := aCabinet(t, c)
	calling(patientA, "patient")

	shelf := cabinetOf(t, mux)

	if len(shelf.Vials) != 1 {
		t.Fatalf("the cabinet holds %d vials", len(shelf.Vials))
	}
	// On the patient's day the vial is inside the fortnight and reads expiring; a server
	// answering in its own day is a day short of it and reads the vial as plainly active.
	if shelf.Vials[0].Status != "expiring" {
		t.Errorf("a vial fourteen days from its date reads %q", shelf.Vials[0].Status)
	}
}

// «Not yours» and «not here» are one answer, because telling them apart is telling the caller
// about a cabinet they cannot see.
func TestAVialOfAnotherPatientAndOneThatIsNotThereAreOneAnswer(t *testing.T) {
	c := newClinic(t)
	mux, calling := aCabinet(t, c)
	calling(patientA, "patient")

	for _, asked := range []struct {
		name string
		vial string
	}{
		{"another patient's", c.vialB},
		{"one nobody has", "8a1f3b7c-0000-4000-8000-00000000dead"},
	} {
		t.Run(asked.name, func(t *testing.T) {
			status, body := askCabinet(t, mux, "/v1/me/vials/"+asked.vial)
			if status != http.StatusNotFound {
				t.Errorf("answered %d, want 404: %s", status, body)
			}
		})
	}
}

// A thrown-away vial is history: it leaves the shelf and keeps its card, which is where the
// patient reads what was drawn from it.
func TestTheCardAnswersADisposedVialWhichTheShelfOmits(t *testing.T) {
	c := newClinic(t)
	openTheVial(t, c, patientA, c.vialA)
	disposeOfTheVial(t, c, patientA, c.vialA)

	mux, calling := aCabinet(t, c)
	calling(patientA, "patient")

	if shelf := cabinetOf(t, mux); len(shelf.Vials) != 0 {
		t.Errorf("the shelf holds %d thrown-away vials", len(shelf.Vials))
	}

	status, body := askCabinet(t, mux, "/v1/me/vials/"+c.vialA)
	if status != http.StatusOK {
		t.Fatalf("the card answered %d: %s", status, body)
	}
	var card struct {
		ID         string  `json:"id"`
		Status     string  `json:"status"`
		DisposedAt *string `json:"disposed_at"`
	}
	if err := json.Unmarshal(body, &card); err != nil {
		t.Fatalf("reading the card: %v", err)
	}
	if card.ID != c.vialA || card.Status != "disposed" || card.DisposedAt == nil {
		t.Errorf("the card reads %+v", card)
	}
}

// The flag and never the key. A client holding an object-store path is a client that can ask
// for a path it was not given.
func TestTheLabelKeyNeverLeavesTheServer(t *testing.T) {
	c := newClinic(t)
	key := patientA + "/label.jpg"
	pointAtALabel(t, c, patientA, c.vialA, key)

	mux, calling := aCabinet(t, c)
	calling(patientA, "patient")

	status, body := askCabinet(t, mux, "/v1/me/vials/"+c.vialA)
	if status != http.StatusOK {
		t.Fatalf("the card answered %d: %s", status, body)
	}
	if strings.Contains(string(body), key) {
		t.Errorf("the key travelled in the body: %s", body)
	}
	var card struct {
		HasLabelPhoto bool `json:"has_label_photo"`
	}
	if err := json.Unmarshal(body, &card); err != nil {
		t.Fatalf("reading the card: %v", err)
	}
	if !card.HasLabelPhoto {
		t.Error("a vial with a photograph says it has none")
	}
}

// A patient-only surface: a doctor reads a cabinet through the policies the day the dashboard
// draws one, and until then this publishes nothing.
func TestOnlyAPatientReadsTheirOwnCabinet(t *testing.T) {
	c := newClinic(t)
	mux, calling := aCabinet(t, c)

	for _, who := range []struct {
		name string
		role string
		want int
	}{
		{"a doctor", "doctor", http.StatusForbidden},
		{"an admin", "admin", http.StatusForbidden},
	} {
		t.Run(who.name, func(t *testing.T) {
			calling(doctorA, who.role)
			if status, body := askCabinet(t, mux, "/v1/me/vials"); status != who.want {
				t.Errorf("answered %d, want %d: %s", status, who.want, body)
			}
		})
	}

	calling("", "")
	if status, body := askCabinet(t, mux, "/v1/me/vials"); status != http.StatusUnauthorized {
		t.Errorf("an unauthenticated read answered %d: %s", status, body)
	}
}

// prescribe gives the patient a running course of the clinic's compound: a weekly injection
// with one band of 0,25 мг, which is what makes a count of injections answerable at all.
func prescribe(t *testing.T, c clinic, patient string) {
	t.Helper()

	if err := database.WithServiceJob(
		t.Context(), c.service, seedJob,
		func(ctx context.Context, tx pgx.Tx) error {
			var course, item string
			if err := tx.QueryRow(ctx, `
				INSERT INTO app.protocols (patient_id, start_date, duration_weeks, status)
				VALUES ($1, DATE '2026-05-04', 12, 'active') RETURNING id::text
			`, patient).Scan(&course); err != nil {
				return err
			}
			if err := tx.QueryRow(ctx, `
				INSERT INTO app.protocol_items
				    (protocol_id, kind, compound_id, cadence, days_of_week, times, loggable)
				VALUES ($1, 'injection', $2, 'weekly', ARRAY[7]::smallint[],
				        ARRAY['08:00']::time[], true)
				RETURNING id::text
			`, course, c.compound).Scan(&item); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `
				INSERT INTO app.protocol_phases (protocol_item_id, from_week, to_week, dose_value, dose_unit)
				VALUES ($1, 1, 12, 0.25, 'мг')
			`, item)

			return err
		},
	); err != nil {
		t.Fatalf("prescribing for %s: %v", patient, err)
	}
}

func openTheVial(t *testing.T, c clinic, patient, vial string) {
	t.Helper()

	changeVial(t, c, patient, vial, `UPDATE app.vials SET opened_at = DATE '2026-05-01' WHERE id = $1`)
}

func disposeOfTheVial(t *testing.T, c clinic, patient, vial string) {
	t.Helper()

	changeVial(t, c, patient, vial, `UPDATE app.vials SET disposed_at = DATE '2026-05-08' WHERE id = $1`)
}

func pointAtALabel(t *testing.T, c clinic, patient, vial, key string) {
	t.Helper()

	if err := c.as(t, patient, "patient", func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE app.vials SET label_photo_path = $2 WHERE id = $1`, vial, key)
		if err == nil && tag.RowsAffected() != 1 {
			return fmt.Errorf("the statement matched %d vials, want 1", tag.RowsAffected())
		}

		return err
	}); err != nil {
		t.Fatalf("pointing vial %s at a label: %v", vial, err)
	}
}

// expireOn is written as the patient, and not by the service role: 000021 gives the service
// seam no UPDATE on app.vials at all, which step 3 measured as a 42501 rather than a silence.
func expireOn(t *testing.T, c clinic, patient, vial, on string) {
	t.Helper()

	if err := c.as(t, patient, "patient", func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE app.vials SET expires_on = $2::date WHERE id = $1`, vial, on)
		if err == nil && tag.RowsAffected() != 1 {
			return fmt.Errorf("the statement matched %d vials, want 1", tag.RowsAffected())
		}

		return err
	}); err != nil {
		t.Fatalf("expiring vial %s: %v", vial, err)
	}
}

func changeVial(t *testing.T, c clinic, patient, vial, statement string) {
	t.Helper()

	if err := c.as(t, patient, "patient", func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, statement, vial)
		if err == nil && tag.RowsAffected() != 1 {
			return fmt.Errorf("the statement matched %d vials, want 1", tag.RowsAffected())
		}

		return err
	}); err != nil {
		t.Fatalf("changing vial %s: %v", vial, err)
	}
}

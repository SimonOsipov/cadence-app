//go:build integration

package protocol_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/dosing"
	"github.com/SimonOsipov/cadence-app/api/internal/inventory"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// The three reads through the registered handler, with the real neighbours behind the
// interfaces — which is the half the stubs cannot measure: that dosing and inventory answer
// the questions this context declared, and that the wiring hands them over.
//
// 10 May 2026 is a Sunday, the day the seeded weekly injection falls on; 09:15 in
// Yekaterinburg, where the seeded patients live, is morning.
var theHour = time.Date(2026, time.May, 10, 4, 15, 0, 0, time.UTC)

func get(t *testing.T, service, requests *pgxpool.Pool, subject, role, path string) (int, string) {
	t.Helper()

	mux := chi.NewRouter()
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(
				r.Context(), auth.Principal{Subject: subject, Role: role})))
		})
	})
	history := dosing.NewHistory(func() time.Time { return theHour })
	protocol.NewService(func() time.Time { return theHour }, protocol.Deps{
		ServicePool: service,
		RequestPool: requests,
		Doses:       history,
		Rotation:    history,
		Cabinet:     inventory.NewSupply(),
	}).Register(httpserver.NewAPI(mux))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	return rec.Code, rec.Body.String()
}

func TestThePatientsDayIsAnsweredThroughTheTransport(t *testing.T) {
	service, requests := prescribingWithRequests(t)

	if _, err := protocol.Create(as(t, writeDoctorA, "doctor"), service, aCourse(writePatientA)); err != nil {
		t.Fatalf("prescribing: %v", err)
	}

	status, body := get(t, service, requests, writePatientA, "patient", "/v1/me/today")
	if status != http.StatusOK {
		t.Fatalf("answered %d: %s", status, body)
	}

	var today struct {
		Date          string   `json:"date"`
		PartOfDay     string   `json:"part_of_day"`
		CycleWeek     *int     `json:"cycle_week"`
		SuggestedSite string   `json:"suggested_site"`
		MealCount     *int     `json:"meal_count"`
		WeightKG      *float64 `json:"weight_kg"`
		NextDose      *struct {
			Kind   string `json:"kind"`
			Time   string `json:"time"`
			Status string `json:"status"`
		} `json:"next_dose"`
		WeekProtocol []struct {
			Title string `json:"title"`
		} `json:"week_protocol"`
	}
	if err := json.Unmarshal([]byte(body), &today); err != nil {
		t.Fatalf("reading the reply: %v", err)
	}

	if today.Date != "2026-05-10" || today.PartOfDay != "morning" {
		t.Errorf("the day reads %q %q", today.Date, today.PartOfDay)
	}
	if today.CycleWeek == nil || *today.CycleWeek != 1 {
		t.Errorf("the week is %v", today.CycleWeek)
	}
	// The zone comes from dosing through the interface this context declared, and the
	// patient has logged nothing — so it is the set's first, which is the rotation's own
	// rule for an empty history.
	if today.SuggestedSite != "l-abdomen" {
		t.Errorf("the suggested zone is %q", today.SuggestedSite)
	}
	if today.NextDose == nil || today.NextDose.Kind != "injection" || today.NextDose.Time != "08:00" {
		t.Errorf("the next dose reads %+v", today.NextDose)
	}
	if len(today.WeekProtocol) != 1 || today.WeekProtocol[0].Title != "Семаглутид" {
		t.Errorf("the strip reads %+v", today.WeekProtocol)
	}
	// Null and not zero — and null and not *absent*, which a pointer cannot tell apart:
	// adding omitempty to those fields would keep a nil check green while breaking the
	// criterion this asserts. The raw body is what says which of the three it is.
	var raw map[string]any
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		t.Fatalf("reading the raw reply: %v", err)
	}
	for _, absent := range []string{
		"meal_count", "meal_macros", "targets", "weight_kg", "weight_series", "target_weight_kg",
	} {
		value, present := raw[absent]
		if !present {
			t.Errorf("%s is missing from the reply rather than null", absent)
		}
		if value != nil {
			t.Errorf("%s answered %v, want null", absent, value)
		}
	}
}

// The cabinet's half of the answer, which nothing read: replacing SupplyFor's whole body with
// three nils kept the gate green, because the fixture seeded no vial and the reply's two
// numbers were never asserted. «Остаток флакона нигде не хранится: он считается как
// total_doses минус число событий» is an acceptance criterion of this feature, and this is
// the path it is answered on.
func TestTheOpenVialsRemainingDosesAreOnTheDay(t *testing.T) {
	service, requests := prescribingWithRequests(t)

	if _, err := protocol.Create(as(t, writeDoctorA, "doctor"), service, aCourse(writePatientA)); err != nil {
		t.Fatalf("prescribing: %v", err)
	}
	openAVial(t, service, requests, writePatientA, 6)

	left, reorder := supplyOn(t, service, requests, writePatientA)

	// Six doses in the vial and none drawn: the count is a subtraction, and there is no
	// column it could have come from.
	if left == nil || *left != 6 {
		t.Errorf("the open vial has %v doses left, want six", left)
	}
	// Weekly injection, six doses: six weeks of supply, past the four-week threshold, so
	// no hint. The pair is what says the number reached the rule rather than a constant.
	if reorder != nil {
		t.Errorf("a reorder hint at six weeks of supply: %+v", reorder)
	}

	drawDoses(t, service, requests, writePatientA, 3)

	left, reorder = supplyOn(t, service, requests, writePatientA)

	if left == nil || *left != 3 {
		t.Errorf("after three doses the vial has %v left, want three", left)
	}
	if reorder == nil || reorder.WeeksLeft != 3 {
		t.Errorf("three weeks of supply answered %+v, want a hint", reorder)
	}

	// One dose in the spare, deliberately: a large spare would silence the hint through
	// the weeks arithmetic instead, and the rule under test — «0 sealed spares» — would be
	// unmeasured. Three doses plus this one is four weeks, which is inside the threshold.
	openAVial(t, service, requests, writePatientA, 1)
	sealASpare(t, requests, writePatientA, 1)

	left, reorder = supplyOn(t, service, requests, writePatientA)

	// A sealed spare is not «doses left» — counting it is the prototype's own bug — and it
	// is what silences the hint: «0 sealed spares & ≤4 weeks supply» is one condition.
	if left == nil || *left != 3 {
		t.Errorf("a sealed spare moved the count to %v", left)
	}
	if reorder != nil {
		t.Errorf("a hint with a spare on the shelf: %+v", reorder)
	}

	sealASpare(t, requests, writePatientA, 6)

	left, _ = supplyOn(t, service, requests, writePatientA)

	// Nothing open: the count is absent rather than the sealed stock's, which is the whole
	// of «the open vial's remaining doses».
	if left != nil {
		t.Errorf("a cabinet with nothing open answered %v", *left)
	}
}

func supplyOn(t *testing.T, service, requests *pgxpool.Pool, patient string) (*int, *protocol.ReorderHint) {
	t.Helper()

	_, body := get(t, service, requests, patient, "patient", "/v1/me/today")

	var today struct {
		VialDosesLeft *int `json:"vial_doses_left"`
		Reorder       *struct {
			CompoundID string `json:"compound_id"`
			WeeksLeft  int    `json:"weeks_left"`
		} `json:"reorder"`
	}
	if err := json.Unmarshal([]byte(body), &today); err != nil {
		t.Fatalf("reading the reply: %v", err)
	}
	if today.Reorder == nil {
		return today.VialDosesLeft, nil
	}

	return today.VialDosesLeft, &protocol.ReorderHint{
		CompoundID: protocol.CompoundID(today.Reorder.CompoundID),
		WeeksLeft:  today.Reorder.WeeksLeft,
	}
}

// openAVial gives the patient an opened vial of the seeded compound. As the patient, because
// the service path holds no UPDATE on vials — opening one is their own act.
func openAVial(t *testing.T, service, requests *pgxpool.Pool, patient string, total int) {
	t.Helper()

	var compound string
	if err := database.WithServiceJob(
		t.Context(), service, writeJob,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`SELECT id::text FROM app.compounds WHERE name_ru = 'Семаглутид'`).Scan(&compound)
		},
	); err != nil {
		t.Fatalf("finding the compound: %v", err)
	}

	if err := database.WithCaller(
		t.Context(), requests, database.Caller{Subject: patient, Role: "patient"},
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.vials
				    (patient_id, compound_id, concentration_label, total_doses,
				     opened_at, expires_on)
				VALUES ($1, $2, '2,4 мг/0,75 мл', $3, DATE '2026-05-01', DATE '2026-12-31')
			`, patient, compound, total)

			return err
		},
	); err != nil {
		t.Fatalf("opening a vial: %v", err)
	}
}

func sealASpare(t *testing.T, requests *pgxpool.Pool, patient string, total int) {
	t.Helper()

	if err := database.WithCaller(
		t.Context(), requests, database.Caller{Subject: patient, Role: "patient"},
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx,
				`UPDATE app.vials SET opened_at = NULL WHERE patient_id = $1 AND total_doses = $2`,
				patient, total)

			return err
		},
	); err != nil {
		t.Fatalf("sealing the spare: %v", err)
	}
}

// drawDoses logs n injections against the seeded course, each drawn from the open vial and
// each on its own day — the slot's unique index is what makes the dates differ.
func drawDoses(t *testing.T, service, requests *pgxpool.Pool, patient string, n int) {
	t.Helper()

	if err := database.WithCaller(
		t.Context(), requests, database.Caller{Subject: patient, Role: "patient"},
		func(ctx context.Context, tx pgx.Tx) error {
			for day := range n {
				_, err := tx.Exec(ctx, `
					INSERT INTO app.dose_events
					    (patient_id, protocol_id, protocol_item_id, vial_id, compound_id,
					     scheduled_for_date, injected_at, dose_value, dose_unit,
					     client_request_id)
					SELECT $1, item.protocol_id, item.id, vial.id, item.compound_id,
					       DATE '2026-05-03' + $2::int, TIMESTAMPTZ '2026-05-03 08:00+03', 0.25, 'мг',
					       'reads-fixture-' || $2::text
					  FROM app.protocol_items AS item
					  JOIN app.protocols AS course ON course.id = item.protocol_id
					  JOIN app.vials AS vial
					    ON vial.patient_id = course.patient_id AND vial.opened_at IS NOT NULL
					 WHERE course.patient_id = $1
				`, patient, day)
				if err != nil {
					return err
				}
			}

			return nil
		},
	); err != nil {
		t.Fatalf("drawing doses: %v", err)
	}
}

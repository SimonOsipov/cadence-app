//go:build integration

package protocol_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/dosing"
	"github.com/SimonOsipov/cadence-app/api/internal/inventory"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
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
	protocol.NewService(protocol.Deps{
		ServicePool: service,
		RequestPool: requests,
		Doses:       history,
		Rotation:    history,
		Cabinet:     inventory.NewSupply(),
		Now:         func() time.Time { return theHour },
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
	// Null and not zero: the two contexts are not built, and a zero would be a sentence
	// about a prescription that does not exist.
	if today.MealCount != nil || today.WeightKG != nil {
		t.Errorf("a context that does not exist answered: %v %v", today.MealCount, today.WeightKG)
	}
}

func TestAMonthOfTheCalendarIsAnsweredThroughTheTransport(t *testing.T) {
	service, requests := prescribingWithRequests(t)

	if _, err := protocol.Create(as(t, writeDoctorA, "doctor"), service, aCourse(writePatientA)); err != nil {
		t.Fatalf("prescribing: %v", err)
	}

	status, body := get(t, service, requests, writePatientA, "patient", "/v1/me/schedule?month=2026-05")
	if status != http.StatusOK {
		t.Fatalf("answered %d: %s", status, body)
	}

	var month struct {
		Days []struct {
			Date         string `json:"date"`
			HasInjection bool   `json:"has_injection"`
			AnyPending   bool   `json:"any_pending"`
		} `json:"days"`
	}
	if err := json.Unmarshal([]byte(body), &month); err != nil {
		t.Fatalf("reading the reply: %v", err)
	}

	if len(month.Days) != 31 {
		t.Fatalf("May is %d days", len(month.Days))
	}
	if month.Days[0].Date != "2026-05-01" || month.Days[30].Date != "2026-05-31" {
		t.Errorf("the month runs %s..%s", month.Days[0].Date, month.Days[30].Date)
	}

	// The Sundays of May 2026 are the 3rd, 10th, 17th, 24th and 31st, and the course
	// starts on the 4th — so the 3rd carries no injection and the rest do.
	for _, day := range month.Days {
		sunday := day.Date == "2026-05-10" || day.Date == "2026-05-17" ||
			day.Date == "2026-05-24" || day.Date == "2026-05-31"
		if day.HasInjection != sunday {
			t.Errorf("%s: injection %v, want %v", day.Date, day.HasInjection, sunday)
		}
	}

	// A month the patient's course does not reach is every day and no dots, rather than
	// an empty list: the calendar draws a month whatever is in it.
	_, next := get(t, service, requests, writePatientA, "patient", "/v1/me/schedule?month=2027-01")
	var later struct {
		Days []struct {
			HasInjection bool `json:"has_injection"`
		} `json:"days"`
	}
	if err := json.Unmarshal([]byte(next), &later); err != nil {
		t.Fatalf("reading the later month: %v", err)
	}
	if len(later.Days) != 31 {
		t.Errorf("January is %d days", len(later.Days))
	}
	for _, day := range later.Days {
		if day.HasInjection {
			t.Error("a month past the course carries an injection")
		}
	}
}

func TestOneDayOfTheCalendarIsAnsweredThroughTheTransport(t *testing.T) {
	service, requests := prescribingWithRequests(t)

	if _, err := protocol.Create(as(t, writeDoctorA, "doctor"), service, aCourse(writePatientA)); err != nil {
		t.Fatalf("prescribing: %v", err)
	}

	status, body := get(t, service, requests, writePatientA, "patient",
		"/v1/me/schedule/day?date=2026-05-17")
	if status != http.StatusOK {
		t.Fatalf("answered %d: %s", status, body)
	}

	var day struct {
		Date        string `json:"date"`
		CycleWeek   *int   `json:"cycle_week"`
		Occurrences []struct {
			Kind   string `json:"kind"`
			Time   string `json:"time"`
			Status string `json:"status"`
			Dose   *struct {
				Value float64 `json:"value"`
				Unit  string  `json:"unit"`
			} `json:"dose"`
		} `json:"occurrences"`
	}
	if err := json.Unmarshal([]byte(body), &day); err != nil {
		t.Fatalf("reading the reply: %v", err)
	}

	if day.Date != "2026-05-17" || day.CycleWeek == nil || *day.CycleWeek != 2 {
		t.Errorf("the day reads %q week %v", day.Date, day.CycleWeek)
	}
	if len(day.Occurrences) != 1 {
		t.Fatalf("the day holds %d occurrences", len(day.Occurrences))
	}
	// A week ahead of the patient's today, so it is scheduled rather than pending.
	if day.Occurrences[0].Status != "scheduled" || day.Occurrences[0].Time != "08:00" {
		t.Errorf("the occurrence reads %+v", day.Occurrences[0])
	}
	if day.Occurrences[0].Dose == nil || day.Occurrences[0].Dose.Value != 0.25 {
		t.Errorf("the dose reads %+v", day.Occurrences[0].Dose)
	}
}

// A patient's own day, and nobody else's: every one of these reads runs under the caller's
// identity, so a doctor asking gets a refusal rather than a patient's screen.
func TestTheReadsAreAPatientsOwn(t *testing.T) {
	service, requests := prescribingWithRequests(t)

	for _, path := range []string{"/v1/me/today", "/v1/me/schedule", "/v1/me/schedule/day"} {
		for _, role := range []string{"doctor", "admin"} {
			if status, body := get(t, service, requests, writeDoctorA, role, path); status != http.StatusForbidden {
				t.Errorf("%s as %s answered %d: %s", path, role, status, body)
			}
		}
	}
}

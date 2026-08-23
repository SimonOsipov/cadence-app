//go:build integration

package dosing_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/dosing"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

// One request through the registered handler, the way the app would send it. Step 6 shipped
// two operations that could not answer anything but 422 — the format keyword and the parser
// disagreed — and nothing saw it because no test crossed this layer. This is that test.
func send(t *testing.T, c clinic, subject, payload string) (int, string) {
	t.Helper()

	mux := chi.NewRouter()
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(
				r.Context(), auth.Principal{Subject: subject, Role: "patient"})))
		})
	})
	dosing.NewServiceAt(c.request, func() time.Time { return theMoment }).Register(httpserver.NewAPI(mux))

	req := httptest.NewRequest(http.MethodPost, "/v1/me/dose-events", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return rec.Code, rec.Body.String()
}

func aPayload(c clinic, patient string, edit func(map[string]any)) string {
	body := map[string]any{
		"protocol_item_id":  c.item[patient],
		"dose_value":        0.25,
		"dose_unit":         "мг",
		"site_code":         "l-abdomen",
		"mood":              4,
		"side_effects":      []string{"nausea", "site"},
		"note":              "спокойно",
		"client_request_id": "wizard-through-http",
	}
	if edit != nil {
		edit(body)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}

	return string(payload)
}

func TestTheAppCanRecordADoseThroughTheTransport(t *testing.T) {
	c := newClinic(t)

	status, body := send(t, c, patientA, aPayload(c, patientA, nil))
	if status != http.StatusOK {
		t.Fatalf("answered %d: %s", status, body)
	}

	var answered struct {
		Outcome     string  `json:"outcome"`
		EventID     string  `json:"dose_event_id"`
		JournalDate string  `json:"journal_date"`
		DoseValue   float64 `json:"dose_value"`
		DoseUnit    string  `json:"dose_unit"`
	}
	if err := json.Unmarshal([]byte(body), &answered); err != nil {
		t.Fatalf("reading the reply: %v", err)
	}
	if answered.Outcome != "written" || answered.EventID == "" {
		t.Errorf("the reply is %+v", answered)
	}
	if answered.JournalDate != "2026-05-10" {
		t.Errorf("the day is %q", answered.JournalDate)
	}
	if answered.DoseValue != 0.25 || answered.DoseUnit != "мг" {
		t.Errorf("the dose reads %v %q", answered.DoseValue, answered.DoseUnit)
	}
}

// Every outcome reached through the transport rather than asserted of a function: an outcome
// no request can produce is a contract nobody honours, and all four are a 200.
func TestEachOutcomeIsReachedThroughTheTransport(t *testing.T) {
	c := newClinic(t)

	// Written first, so already_logged below is the state it leaves behind.
	if status, body := send(t, c, patientA, aPayload(c, patientA, nil)); status != http.StatusOK ||
		!strings.Contains(body, `"outcome":"written"`) {
		t.Fatalf("the first dose answered %d: %s", status, body)
	}

	for _, request := range []struct {
		name    string
		payload string
		want    string
	}{
		{
			"the same occurrence again, with another key",
			aPayload(c, patientA, func(b map[string]any) { b["client_request_id"] = "another-key-01" }),
			`"outcome":"already_logged"`,
		},
		{
			"the same key again",
			aPayload(c, patientA, nil),
			`"outcome":"written"`,
		},
		{
			"an item with no occurrence today",
			aPayload(c, patientA, func(b map[string]any) {
				b["protocol_item_id"] = "9b2f3b7c-0000-4000-8000-00000000dead"
				b["client_request_id"] = "another-key-02"
			}),
			`"outcome":"not_scheduled_today"`,
		},
	} {
		t.Run(request.name, func(t *testing.T) {
			status, body := send(t, c, patientA, request.payload)
			if status != http.StatusOK {
				t.Fatalf("answered %d: %s", status, body)
			}
			if !strings.Contains(body, request.want) {
				t.Errorf("the reply is %s, want %s", body, request.want)
			}
		})
	}
}

// The three closed sets the wire carries, each refused rather than cast. The schema's enum
// keyword is a courtesy to the generated client; a request that bypasses it reaches the
// parser either way, which is what these measure.
func TestAValueOffAClosedSetIsRefused(t *testing.T) {
	c := newClinic(t)

	for _, malformed := range []struct {
		name string
		edit func(map[string]any)
	}{
		{"a zone the body map cannot draw", func(b map[string]any) { b["site_code"] = "l-flank" }},
		{"a side effect nobody named", func(b map[string]any) { b["side_effects"] = []string{"dizziness"} }},
		{"a unit nobody prescribes", func(b map[string]any) { b["dose_unit"] = "ме" }},
		{"a client key carrying a path", func(b map[string]any) { b["client_request_id"] = "../secrets" }},
		{"a dose of nothing", func(b map[string]any) { b["dose_value"] = 0 }},
		{"a mood off the scale", func(b map[string]any) { b["mood"] = 6 }},
	} {
		t.Run(malformed.name, func(t *testing.T) {
			status, body := send(t, c, patientA, aPayload(c, patientA, malformed.edit))
			if status != http.StatusUnprocessableEntity {
				t.Errorf("answered %d, want 422: %s", status, body)
			}
		})
	}
}

// One patient's key is not another's, through the transport: the uniqueness is scoped to the
// patient, so two devices generating the same key must not collide.
func TestTheSameKeyFromTwoPatientsIsTwoDoses(t *testing.T) {
	c := newClinic(t)

	for _, patient := range []string{patientA, patientB} {
		if status, body := send(t, c, patient,
			aPayload(c, patient, nil)); status != http.StatusOK ||
			!strings.Contains(body, `"outcome":"written"`) {
			t.Fatalf("%s answered %d: %s", patient, status, body)
		}
	}

	for _, patient := range []string{patientA, patientB} {
		if doses := dosesOn(t, c, patient, "2026-05-10"); doses != 1 {
			t.Errorf("%s has %d doses on the day", patient, doses)
		}
	}
}

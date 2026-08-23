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

	return sendAs(t, c, subject, "patient", payload)
}

func sendAs(t *testing.T, c clinic, subject, role, payload string) (int, string) {
	t.Helper()

	mux := chi.NewRouter()
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(
				r.Context(), auth.Principal{Subject: subject, Role: role})))
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

// The approved decision of 2026-08-15: the same key with a different draft is a client error
// and not a repeat. Returning the first result silently would hide the fault in the one path
// that exists for an unreliable network — the retry queue sends what it saved, so a
// divergence means it saved the wrong thing.
func TestARepeatWithAChangedBodyIsAConflict(t *testing.T) {
	c := newClinic(t)

	if status, _ := send(t, c, patientA, aPayload(c, patientA, nil)); status != http.StatusOK {
		t.Fatalf("the first dose answered %d", status)
	}

	for _, changed := range []struct {
		name  string
		edit  func(map[string]any)
		field string
	}{
		{"another zone", func(b map[string]any) { b["site_code"] = "r-glute" }, "site_code"},
		{"another mood", func(b map[string]any) { b["mood"] = 2 }, "mood"},
		{"another note", func(b map[string]any) { b["note"] = "иначе" }, "note"},
		{
			"another side effect",
			func(b map[string]any) { b["side_effects"] = []string{"fatigue"} }, "side_effects",
		},
		{
			"another item",
			func(b map[string]any) { b["protocol_item_id"] = c.item[patientB] }, "protocol_item_id",
		},
	} {
		t.Run(changed.name, func(t *testing.T) {
			status, body := send(t, c, patientA, aPayload(c, patientA, changed.edit))
			if status != http.StatusConflict {
				t.Fatalf("answered %d, want 409: %s", status, body)
			}
			// The field, not merely the conflict: a client told «something differs» has
			// to diff the whole draft to find out what its queue saved wrongly.
			if !strings.Contains(body, changed.field) {
				t.Errorf("the refusal does not name %s: %s", changed.field, body)
			}
		})
	}

	// And the true repeat, in the same clinic, so the conflicts above are known not to be
	// the answer to every second request.
	if status, body := send(t, c, patientA, aPayload(c, patientA, nil)); status != http.StatusOK ||
		!strings.Contains(body, `"outcome":"written"`) {
		t.Errorf("the unchanged repeat answered %d: %s", status, body)
	}
	// Reordered side effects are the same draft: the decision says the draft's meaning
	// and not the request's bytes.
	reordered := aPayload(c, patientA, func(b map[string]any) {
		b["side_effects"] = []string{"site", "nausea"}
	})
	if status, body := send(t, c, patientA, reordered); status != http.StatusOK {
		t.Errorf("a reordered repeat answered %d: %s", status, body)
	}
}

// The two fields that come from the request body and name another row. What refuses them is
// the schema, on every path — and until this test the new path reaching those constraints was
// asserted nowhere, and answered a 500 rather than the field the caller filled in.
func TestAVialOrAPhotoBelongingToAnotherPatientIsRefused(t *testing.T) {
	c := newClinic(t)

	for _, borrowed := range []struct {
		name string
		edit func(map[string]any)
	}{
		{
			"another patient's vial",
			func(b map[string]any) { b["vial_id"] = c.vial[patientB] },
		},
		{
			"a photo key under another patient's prefix",
			func(b map[string]any) { b["photo_path"] = patientB + "/site.jpg" },
		},
		{
			// A key that begins with the right characters and points elsewhere, which
			// is why the constraint is shaped and not a prefix.
			"a photo key that climbs out of its own prefix",
			func(b map[string]any) { b["photo_path"] = patientA + "/../" + patientB + "/site.jpg" },
		},
	} {
		t.Run(borrowed.name, func(t *testing.T) {
			status, body := send(t, c, patientA, aPayload(c, patientA, borrowed.edit))
			if status != http.StatusUnprocessableEntity {
				t.Errorf("answered %d, want 422: %s", status, body)
			}
			// The whole transaction rolls back, so neither fact survives.
			if doses := dosesOn(t, c, patientA, "2026-05-10"); doses != 0 {
				t.Errorf("the refused request left %d doses", doses)
			}
			if days := daysOn(t, c, patientA, "2026-05-10"); days != 0 {
				t.Errorf("the refused request left %d diary entries", days)
			}
		})
	}

	// The patient's own vial goes through, so the refusals above are known to be about
	// whose vial it is rather than about naming one at all.
	if status, body := send(t, c, patientA, aPayload(c, patientA, func(b map[string]any) {
		b["vial_id"] = c.vial[patientA]
		b["photo_path"] = patientA + "/site.jpg"
	})); status != http.StatusOK {
		t.Errorf("their own vial answered %d: %s", status, body)
	}
}

// A patient-only surface. For a doctor the answer would be a bland «not scheduled today»,
// and for an admin every policy on the four tables in play is USING (true) — so the Go
// predicate would be the only boundary left.
func TestOnlyAPatientRecordsTheirOwnDoses(t *testing.T) {
	c := newClinic(t)

	for _, role := range []string{"doctor", "admin"} {
		t.Run(role, func(t *testing.T) {
			status, body := sendAs(t, c, doctorA, role, aPayload(c, patientA, nil))
			if status != http.StatusForbidden {
				t.Errorf("answered %d, want 403: %s", status, body)
			}
			if doses := dosesOn(t, c, patientA, "2026-05-10"); doses != 0 {
				t.Errorf("a %s wrote %d doses", role, doses)
			}
		})
	}
}

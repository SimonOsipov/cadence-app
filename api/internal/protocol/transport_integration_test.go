//go:build integration

package protocol_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// One request through the transport, the way the dashboard would send it — the layer the
// rest of this package's suite steps over by calling Course.draft directly. That gap is
// where the generated schema lives: the field formats, the required properties, the enums
// and the minItems are all only asked here.
func send(t *testing.T, pool *pgxpool.Pool, caller auth.Principal, method, path, payload string) (int, string) {
	t.Helper()

	mux := chi.NewRouter()
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), caller)))
		})
	})
	protocol.NewService(time.Now, protocol.Deps{ServicePool: pool}).Register(httpserver.NewAPI(mux))

	req := httptest.NewRequest(method, path, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return rec.Code, rec.Body.String()
}

const aWirePayload = `{
	"start_date": "2026-05-04",
	"weeks": 12,
	"status": "active",
	"items": [{
		"kind": "injection",
		"compound": {"name_ru": "Семаглутид", "default_unit": "мг", "route": "sc", "icon": "syringe"},
		"cadence": "weekly",
		"days_of_week": [7],
		"times": ["08:00"],
		"loggable": true,
		"phases": [
			{"from_week": 1, "to_week": 4, "dose_value": 0.25, "dose_unit": "мг"},
			{"from_week": 5, "to_week": 12, "dose_value": 0.5, "dose_unit": "мг"}
		]
	}]
}`

func TestTheDashboardCanPrescribeThroughTheTransport(t *testing.T) {
	pool, _ := prescribing(t)

	status, body := send(t, pool, auth.Principal{Subject: writeDoctorA, Role: "doctor"},
		http.MethodPost, "/v1/patients/"+writePatientA+"/protocols", aWirePayload)
	if status != http.StatusCreated {
		t.Fatalf("prescribing answered %d: %s", status, body)
	}
	if !strings.Contains(body, `"protocol_id"`) || !strings.Contains(body, `"item_ids"`) {
		t.Errorf("the reply carries neither identifier: %s", body)
	}
}

// Rewriting through the transport: the PUT was declared and never driven, so its 404 — the
// only place ErrNoSuchProtocol and ErrNoSuchItem become a wire status — was measured nowhere,
// and the round-one critical was exactly a route nothing had ever sent a request to.
func TestACourseIsRewrittenThroughTheTransport(t *testing.T) {
	pool, _ := prescribing(t)
	doctorA := auth.Principal{Subject: writeDoctorA, Role: "doctor"}

	status, body := send(t, pool, doctorA,
		http.MethodPost, "/v1/patients/"+writePatientA+"/protocols", aWirePayload)
	if status != http.StatusCreated {
		t.Fatalf("prescribing answered %d: %s", status, body)
	}

	var created struct {
		ProtocolID string   `json:"protocol_id"`
		ItemIDs    []string `json:"item_ids"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("reading the reply: %v", err)
	}

	kept := strings.Replace(aWirePayload, `"kind": "injection",`,
		`"id": "`+created.ItemIDs[0]+`", "kind": "injection",`, 1)
	path := "/v1/patients/" + writePatientA + "/protocols/"

	if status, body := send(t, pool, doctorA, http.MethodPut, path+created.ProtocolID, kept); status != http.StatusOK {
		t.Errorf("rewriting answered %d: %s", status, body)
	}

	for _, missing := range []struct {
		name    string
		course  string
		payload string
	}{
		{"a course nobody holds", "5d4f3b7c-0000-4000-8000-00000000dead", aWirePayload},
		{
			"an item this course does not hold", created.ProtocolID,
			strings.Replace(aWirePayload, `"kind": "injection",`,
				`"id": "5d4f3b7c-0000-4000-8000-00000000beef", "kind": "injection",`, 1),
		},
	} {
		t.Run(missing.name, func(t *testing.T) {
			if status, body := send(t, pool, doctorA, http.MethodPut,
				path+missing.course, missing.payload); status != http.StatusNotFound {
				t.Errorf("answered %d, want 404: %s", status, body)
			}
		})
	}
}

// The statuses the create route declares, each reached through the transport rather than
// asserted of a mapping function: a status that no request can produce is a contract nobody
// honours. 400 and 503 are declared and not driven here — the first is huma's own parse
// failure and the second needs the database gone.
func TestEachDeclaredStatusIsReachable(t *testing.T) {
	pool, _ := prescribing(t)

	doctorA := auth.Principal{Subject: writeDoctorA, Role: "doctor"}
	doctorB := auth.Principal{Subject: writeDoctorB, Role: "doctor"}
	path := "/v1/patients/" + writePatientA + "/protocols"

	if status, body := send(t, pool, doctorA, http.MethodPost, path, aWirePayload); status != http.StatusCreated {
		t.Fatalf("the first course answered %d: %s", status, body)
	}

	for _, request := range []struct {
		name    string
		caller  auth.Principal
		payload string
		want    int
	}{
		{"a course for somebody else's patient", doctorB, aWirePayload, http.StatusForbidden},
		{"a second course while one runs", doctorA, aWirePayload, http.StatusConflict},
		{
			"phases that overlap", doctorA,
			strings.Replace(aWirePayload, `"from_week": 5`, `"from_week": 4`, 1),
			http.StatusUnprocessableEntity,
		},
		{
			"a weekday outside the week", doctorA,
			strings.Replace(aWirePayload, `"days_of_week": [7]`, `"days_of_week": [8]`, 1),
			http.StatusUnprocessableEntity,
		},
	} {
		t.Run(request.name, func(t *testing.T) {
			status, body := send(t, pool, request.caller, http.MethodPost, path, request.payload)
			if status != request.want {
				t.Errorf("answered %d, want %d: %s", status, request.want, body)
			}
		})
	}
}

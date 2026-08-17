package identity_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

// reporting is the request a device makes on every launch.
func reporting(timezone string) *http.Request {
	body := strings.NewReader(`{"timezone":"` + timezone + `"}`)

	request := httptest.NewRequest(http.MethodPost, "/v1/me/session", body)
	request.Header.Set("Content-Type", "application/json")

	return request
}

// servedBy is the identity context on a bare router with sessions behind it and
// principal standing in for what the authentication middleware would have put on
// the context.
func servedBy(principal *auth.Principal, sessions *identity.Sessions) http.Handler {
	router := chi.NewRouter()

	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if principal != nil {
				r = r.WithContext(auth.WithPrincipal(r.Context(), *principal))
			}

			next.ServeHTTP(w, r)
		})
	})

	identity.NewService(nil, sessions).Register(httpserver.NewAPI(router))

	return router
}

func TestSessionRefusesWithoutAPrincipal(t *testing.T) {
	rec := httptest.NewRecorder()
	servedBy(nil, identity.NewSessions(nil)).ServeHTTP(rec, reporting("Asia/Tbilisi"))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != httpserver.ProblemContentType {
		t.Errorf("Content-Type = %q, want %q", got, httpserver.ProblemContentType)
	}
}

// The staff half of the contract, measured rather than described: the pool is
// nil, so a handler that reached the database to decide this would panic instead
// of passing. Only cadence_patient holds the column grant, and a doctor is
// answered without a write rather than refused, because one client serves both
// roles and should not have to branch.
func TestSessionWritesNothingForStaffAndSaysSo(t *testing.T) {
	for _, role := range []string{"doctor", "admin"} {
		t.Run(role, func(t *testing.T) {
			principal := auth.Principal{
				Subject: "8a1f3b7c-0000-4000-8000-000000000002",
				Role:    role,
			}

			rec := httptest.NewRecorder()
			servedBy(&principal, identity.NewSessions(nil)).ServeHTTP(rec, reporting("Asia/Tbilisi"))

			if rec.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want 204; body %s", rec.Code, rec.Body)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("the response carries %s, want nothing", rec.Body)
			}
		})
	}
}

// An account whose profile the hook found nothing to name is a real state — the
// invitation is out and the person is not provisioned — and it reaches here as a
// token with no role. It writes nothing, and it is answered rather than refused
// for the same reason a doctor is.
func TestSessionWritesNothingForAnAccountWithNoRole(t *testing.T) {
	principal := auth.Principal{Subject: "8a1f3b7c-0000-4000-8000-000000000004"}

	rec := httptest.NewRecorder()
	servedBy(&principal, identity.NewSessions(nil)).ServeHTTP(rec, reporting("Asia/Tbilisi"))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body %s", rec.Code, rec.Body)
	}
}

// The operation is in the contract, because the contract is what the Kotlin and
// the dashboard clients are generated from.
func TestSessionIsInTheContract(t *testing.T) {
	router := chi.NewRouter()
	api := httpserver.NewAPI(router)
	identity.NewService(nil, nil).Register(api)

	document, err := api.OpenAPI().MarshalJSON()
	if err != nil {
		t.Fatalf("marshalling the document: %v", err)
	}

	var spec struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(document, &spec); err != nil {
		t.Fatalf("decoding the document: %v", err)
	}

	if _, ok := spec.Paths["/v1/me/session"]; !ok {
		t.Fatalf("the document does not describe /v1/me/session; it has %v", spec.Paths)
	}
}

// An API assembled without the sessions service is the document generator's
// state, and an operation reached in it refuses rather than dereferences — the
// same arrangement the onboarding routes are in.
func TestSessionRefusesWhenTheServiceIsAbsent(t *testing.T) {
	principal := auth.Principal{
		Subject: "8a1f3b7c-0000-4000-8000-000000000001",
		Role:    "patient",
	}

	rec := httptest.NewRecorder()
	servedBy(&principal, nil).ServeHTTP(rec, reporting("Asia/Tbilisi"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body %s", rec.Code, rec.Body)
	}
}

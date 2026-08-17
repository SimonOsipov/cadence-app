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

func reporting(timezone string) *http.Request {
	request := httptest.NewRequest(
		http.MethodPost, "/v1/me/session", strings.NewReader(`{"timezone":"`+timezone+`"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	return request
}

// servedBy mounts the context with principal standing in for what the authentication middleware would have put on
// the context. A nil principal is the case the middleware is supposed to make impossible.
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

// The staff arm, measured rather than described: the service is nil, so a handler that reached the database to
// decide this would answer 500 instead of passing.
func TestSessionWritesNothingForStaffAndSaysSo(t *testing.T) {
	for _, role := range []string{"doctor", "admin"} {
		t.Run(role, func(t *testing.T) {
			principal := auth.Principal{Subject: "8a1f3b7c-0000-4000-8000-000000000002", Role: role}

			rec := httptest.NewRecorder()
			servedBy(&principal, nil).ServeHTTP(rec, reporting("Asia/Tbilisi"))

			if rec.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want 204; body %s", rec.Code, rec.Body)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("the response carries %s, want nothing", rec.Body)
			}
		})
	}
}

// An account the invitation reached and provisioning did not is refused rather than answered 204: nothing can be
// written for it, and 204 would claim a write that did not happen.
func TestSessionRefusesAnAccountWithNoRole(t *testing.T) {
	principal := auth.Principal{Subject: "8a1f3b7c-0000-4000-8000-000000000004"}

	rec := httptest.NewRecorder()
	servedBy(&principal, identity.NewSessions(nil)).ServeHTTP(rec, reporting("Asia/Tbilisi"))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body %s", rec.Code, rec.Body)
	}

	// The sentence itself is pinned by equality in TestWhichRefusalAReportedTimezoneHeard, which can see the
	// constant; here the assertion is that the handler routes this case through that mapper at all.
	var problem struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decoding the problem document: %v", err)
	}

	if problem.Type != httpserver.ProblemForbidden {
		t.Errorf("type = %q, want %q", problem.Type, httpserver.ProblemForbidden)
	}
}

// The contract is what the Kotlin and the dashboard clients are generated from.
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

func TestSessionRefusesWhenTheServiceIsAbsent(t *testing.T) {
	principal := auth.Principal{Subject: "8a1f3b7c-0000-4000-8000-000000000001", Role: "patient"}

	rec := httptest.NewRecorder()
	servedBy(&principal, nil).ServeHTTP(rec, reporting("Asia/Tbilisi"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body %s", rec.Code, rec.Body)
	}
}

package identity_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

// mounted returns the identity context served on a bare router, with principal
// standing in for what the authentication middleware would have put on the
// context. A nil principal is the case the middleware is supposed to make
// impossible — which is exactly why the handler is asked about it.
func mounted(principal *auth.Principal) http.Handler {
	router := chi.NewRouter()

	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if principal != nil {
				r = r.WithContext(auth.WithPrincipal(r.Context(), *principal))
			}

			next.ServeHTTP(w, r)
		})
	})

	identity.Register(httpserver.NewAPI(router))

	return router
}

func TestMeReturnsTheVerifiedCaller(t *testing.T) {
	expiry := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	principal := auth.Principal{
		Subject:   "8a1f3b7c-0000-4000-8000-000000000001",
		Role:      "authenticated",
		ExpiresAt: expiry,
	}

	rec := httptest.NewRecorder()
	mounted(&principal).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/me", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding the body: %v", err)
	}

	if body["sub"] != principal.Subject {
		t.Errorf("sub = %v, want %v", body["sub"], principal.Subject)
	}
	if body["role"] != principal.Role {
		t.Errorf("role = %v, want %v", body["role"], principal.Role)
	}
	if body["expires_at"] != expiry.Format(time.RFC3339) {
		t.Errorf("expires_at = %v, want %v", body["expires_at"], expiry.Format(time.RFC3339))
	}
}

// The narrowness requirement, enforced at the wire rather than promised in a
// comment. A Supabase token carries email, phone and app_metadata; if the
// handler ever starts marshalling claims instead of a principal, this fails.
func TestMeReturnsNothingBeyondTheNarrowPrincipal(t *testing.T) {
	principal := auth.Principal{
		Subject:   "8a1f3b7c-0000-4000-8000-000000000001",
		Role:      "authenticated",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	rec := httptest.NewRecorder()
	mounted(&principal).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/me", nil))

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding the body: %v", err)
	}

	var got []string
	for name := range body {
		got = append(got, name)
	}
	slices.Sort(got)

	if want := []string{"expires_at", "role", "sub"}; !slices.Equal(got, want) {
		t.Errorf("the response carries %v, want exactly %v", got, want)
	}
}

// The handler is only ever reached through the middleware, so this path should
// be unreachable. It is asserted anyway: "unreachable" is a property of the
// wiring, and the wiring is exactly what a later refactor changes. Answering
// 200 with an empty subject would be an anonymous caller presented as a
// verified one.
func TestMeRefusesWithoutAPrincipal(t *testing.T) {
	rec := httptest.NewRecorder()
	mounted(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/me", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != httpserver.ProblemContentType {
		t.Errorf("Content-Type = %q, want %q", got, httpserver.ProblemContentType)
	}

	// The same challenge the middleware sets. A 401 that is missing it here
	// would tell a caller which layer refused, on the one status whose entire
	// design is that the caller learns nothing.
	if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Errorf("WWW-Authenticate = %q, want Bearer", got)
	}
}

// The operation has to be in the contract, because the contract is what the
// Kotlin and the dashboard clients are generated from.
func TestMeIsInTheContract(t *testing.T) {
	router := chi.NewRouter()
	api := httpserver.NewAPI(router)
	identity.Register(api)

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

	if _, ok := spec.Paths["/v1/me"]; !ok {
		t.Fatalf("the document does not describe /v1/me; it has %v", spec.Paths)
	}
}

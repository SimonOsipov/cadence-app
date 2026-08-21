package identity_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

func askingForTheStaff(principal *auth.Principal, directory *identity.Directory) *httptest.ResponseRecorder {
	router := chi.NewRouter()

	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if principal != nil {
				r = r.WithContext(auth.WithPrincipal(r.Context(), *principal))
			}

			next.ServeHTTP(w, r)
		})
	})

	identity.NewService(identity.Deps{Directory: directory}).Register(httpserver.NewAPI(router))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/providers", nil))

	return rec
}

// The list exists so a doctor can put a colleague on a care team. A patient has no care team to
// build, and answering them the clinic's staff directory would be a disclosure nobody asked for.
func TestTheStaffListIsNotForPatients(t *testing.T) {
	principal := auth.Principal{
		Subject:   "8a1f3b7c-0000-4000-8000-000000000001",
		Role:      "patient",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	rec := askingForTheStaff(&principal, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body %s", rec.Code, rec.Body)
	}
}

// Invited and not provisioned: the same refusal every other route of this context gives it.
func TestTheStaffListRefusesAnAccountWithNoRole(t *testing.T) {
	principal := auth.Principal{
		Subject:   "8a1f3b7c-0000-4000-8000-000000000009",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	rec := askingForTheStaff(&principal, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body %s", rec.Code, rec.Body)
	}
}

func TestTheStaffListRefusesWithoutAPrincipal(t *testing.T) {
	rec := askingForTheStaff(nil, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body %s", rec.Code, rec.Body)
	}
}

// The document generator builds this context with no dependencies, and a request reaching an API
// assembled that way is a wiring mistake rather than a caller's.
func TestTheStaffListRefusesWhenTheAPIWasAssembledWithoutIt(t *testing.T) {
	principal := auth.Principal{
		Subject:   "8a1f3b7c-0000-4000-8000-000000000002",
		Role:      "doctor",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	rec := askingForTheStaff(&principal, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body %s", rec.Code, rec.Body)
	}
}

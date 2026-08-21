package identity_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

// stubProfiles answers what a row would say, without one.
type stubProfiles struct {
	name   string
	err    error
	asked  int
	caller database.Caller
}

func (p *stubProfiles) NameOf(_ context.Context, caller database.Caller) (string, error) {
	p.asked++
	p.caller = caller

	return p.name, p.err
}

func askingWhoIAm(principal *auth.Principal, profiles identity.ProfileReader) *httptest.ResponseRecorder {
	router := chi.NewRouter()

	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if principal != nil {
				r = r.WithContext(auth.WithPrincipal(r.Context(), *principal))
			}

			next.ServeHTTP(w, r)
		})
	})

	identity.NewService(identity.Deps{Profiles: profiles}).Register(httpserver.NewAPI(router))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/me", nil))

	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding the body %q: %v", rec.Body, err)
	}

	return body
}

func theSignedInDoctor() auth.Principal {
	return auth.Principal{
		Subject:   "8a1f3b7c-0000-4000-8000-000000000002",
		Role:      "doctor",
		ExpiresAt: time.Now().Add(time.Hour),
	}
}

// The dashboard greets the person by name, and this is the only place the API
// says one: every other route answers about patients.
func TestMeCarriesTheNameTheClinicWrote(t *testing.T) {
	principal := theSignedInDoctor()
	profiles := &stubProfiles{name: "Ксения Первеева"}

	rec := askingWhoIAm(&principal, profiles)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	if got := decodeBody(t, rec)["full_name"]; got != "Ксения Первеева" {
		t.Errorf("full_name = %v, want the name the clinic wrote", got)
	}

	// Their own row and under their own identity: read as anybody else and the
	// answer would be a different person's name with no error to show for it.
	if profiles.caller.Subject != principal.Subject || profiles.caller.Role != principal.Role {
		t.Errorf("the name was read as %+v, want the caller themselves", profiles.caller)
	}
}

// The account an invitation reached and provisioning did not. It has no profile
// row to name, and the rest of the answer is still the truth about the token.
func TestMeWithoutAProfileIsStillAnAnswer(t *testing.T) {
	principal := auth.Principal{
		Subject:   "8a1f3b7c-0000-4000-8000-000000000009",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	profiles := &stubProfiles{}

	rec := askingWhoIAm(&principal, profiles)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	body := decodeBody(t, rec)
	if _, named := body["full_name"]; named {
		t.Errorf("an account with no profile is named anyway: %v", body)
	}
	if body["sub"] != principal.Subject {
		t.Errorf("sub = %v, want the subject the token carries", body["sub"])
	}

	// No role, so no database role to assume: asking would be refused as an
	// unknown role, and the refusal would read as a broken endpoint.
	if profiles.asked != 0 {
		t.Errorf("a roleless caller's profile was looked up %d time(s)", profiles.asked)
	}
}

// The same 503 every other route gives, and for the same reason: a caller told
// «repeat this» can act on it, and one told 500 cannot.
func TestMeAnswersUnavailableWhenTheDatabaseWillNot(t *testing.T) {
	principal := theSignedInDoctor()

	rec := askingWhoIAm(&principal, &stubProfiles{err: identity.ErrDatabaseUnavailable})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body %s", rec.Code, rec.Body)
	}
}

func TestMeAnswersTheFixedInternalErrorWhenTheReadFails(t *testing.T) {
	principal := theSignedInDoctor()

	rec := askingWhoIAm(&principal, &stubProfiles{err: errors.New("the query names a column that is gone")})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body %s", rec.Code, rec.Body)
	}
	if body := rec.Body.String(); strings.Contains(body, "column that is gone") {
		t.Errorf("the response carries the underlying error: %s", body)
	}
}

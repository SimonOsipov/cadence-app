package identity_test

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

func reporting(timezone string) *http.Request {
	request := httptest.NewRequest(
		http.MethodPost, "/v1/me/session", strings.NewReader(`{"timezone":"`+timezone+`"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	return request
}

// servedBy mounts the context with principal standing in for the middleware. A nil principal is the case the
// middleware is supposed to make impossible.
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

	identity.NewService(nil, sessions, nil).Register(httpserver.NewAPI(router))

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

// The service is nil, so a handler that reached the database to decide this would answer 500 rather than pass.
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

// The sentence is pinned by equality in TestAnAccountWithNoRoleIsRefusedInTheMappersWords, which sees the constant.
func TestSessionRefusesAnAccountWithNoRole(t *testing.T) {
	principal := auth.Principal{Subject: "8a1f3b7c-0000-4000-8000-000000000004"}

	rec := httptest.NewRecorder()
	servedBy(&principal, identity.NewSessions(nil)).ServeHTTP(rec, reporting("Asia/Tbilisi"))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body %s", rec.Code, rec.Body)
	}

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
	identity.NewService(nil, nil, nil).Register(api)

	document, err := api.OpenAPI().MarshalJSON()
	if err != nil {
		t.Fatalf("marshalling the document: %v", err)
	}

	var spec struct {
		Paths map[string]struct {
			Post struct {
				Responses map[string]any `json:"responses"`
			} `json:"post"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(document, &spec); err != nil {
		t.Fatalf("decoding the document: %v", err)
	}

	operation, ok := spec.Paths["/v1/me/session"]
	if !ok {
		t.Fatalf("the document does not describe /v1/me/session; it has %v", spec.Paths)
	}

	// The statuses, not just the path: declaring any of them makes huma drop the default response, and the drift
	// gate cannot see a status removed from both the code and the document. 422 and 500 are huma's own additions
	// and are asserted for the same reason — TestSessionRefusesWhenTheServiceIsAbsent reaches the second.
	for _, status := range []string{"204", "400", "401", "403", "422", "500", "503"} {
		if _, declared := operation.Post.Responses[status]; !declared {
			t.Errorf("the operation does not declare %s; it declares %v", status, operation.Post.Responses)
		}
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

// The join between the write and database.IsUnavailable: both ends are measured elsewhere and the call between
// them was not.
func TestARecordingAgainstADatabaseThatIsDownIsUnavailable(t *testing.T) {
	var config net.ListenConfig

	listener, err := config.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	address := listener.Addr().String()

	// Closed rather than left open: a port the kernel chose cannot collide with something else on this machine.
	if err := listener.Close(); err != nil {
		t.Fatalf("closing the listener: %v", err)
	}

	pool, err := pgxpool.New(t.Context(), "postgres://nobody:nothing@"+address+"/none?sslmode=disable")
	if err != nil {
		t.Fatalf("building the pool: %v", err)
	}
	t.Cleanup(pool.Close)

	caller := database.Caller{Subject: "8a1f3b7c-0000-4000-8000-000000000001", Role: "patient"}

	err = identity.NewSessions(pool).RecordTimezone(t.Context(), caller, "Asia/Tbilisi")
	if !errors.Is(err, identity.ErrDatabaseUnavailable) {
		t.Fatalf("recording against a dead database answered %v, want ErrDatabaseUnavailable", err)
	}
}

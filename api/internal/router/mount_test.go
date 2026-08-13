package router_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth/token"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
	"github.com/SimonOsipov/cadence-app/api/internal/router"
)

// openPaths is the exemption list, written out rather than imported from the
// code it checks. A test that asks the middleware which paths it exempts agrees
// with the middleware by construction; this one disagrees the moment a path is
// opened, which is the entire point of it.
var openPaths = []string{
	"/healthz",
	"/openapi.json",
	"/openapi.yaml",
	"/openapi-3.0.json",
	"/openapi-3.0.yaml",
	"/docs",
}

// assembled builds the whole HTTP surface the way the composition root does, so
// that what the tests below walk is what the process serves. A second assembly
// written for the tests would prove things about the test.
func assembled(t *testing.T, probe func(context.Context) error) (*chi.Mux, *fixture) {
	t.Helper()

	key := testsupport.NewRS256Key(t, "primary")
	set := testsupport.StartJWKS(t, key)

	verifier, err := token.NewVerifier(t.Context(), token.VerifierConfig{
		Issuer:      set.Issuer,
		Audience:    "authenticated",
		JWKSURL:     set.Issuer + testsupport.JWKSPath,
		SessionKIDs: []string{key.KID},
	})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	server := httpserver.New(httpserver.Config{Port: "0"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	router.Mount(server.Router, router.Options{
		Verifier: verifier,
		Probe:    probe,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	return server.Router, &fixture{key: key, set: set}
}

type fixture struct {
	key *testsupport.SigningKey
	set *testsupport.JWKS
}

func (f *fixture) token(t *testing.T) string {
	t.Helper()

	now := time.Now()

	return f.key.Sign(t, jwt.MapClaims{
		"sub":  "8a1f3b7c-0000-4000-8000-000000000001",
		"role": "authenticated",
		"aud":  "authenticated",
		"iss":  f.set.Issuer,
		"nbf":  now.Add(-time.Minute).Unix(),
		"exp":  now.Add(time.Hour).Unix(),
	})
}

var pathParam = regexp.MustCompile(`\{[^}]*\}`)

// TestEveryMountedRouteRefusesAnUnauthenticatedRequest is the deny-by-default
// gate. It does not assert against a list of routes it knows about — it asks
// chi what is actually mounted and requires every one of them to refuse, so the
// route added next month is covered before anyone writes a test for it.
func TestEveryMountedRouteRefusesAnUnauthenticatedRequest(t *testing.T) {
	mux, _ := assembled(t, nil)

	var walked int

	err := chi.Walk(mux, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		// Chi reports the pattern; a request needs a concrete path.
		path := strings.TrimSuffix(pathParam.ReplaceAllString(route, "x"), "/*")
		if path == "" {
			path = "/"
		}

		if slices.Contains(openPaths, path) {
			return nil
		}

		walked++

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, path, nil))

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s answered %d without a token, want 401", method, path, rec.Code)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walking the router: %v", err)
	}

	// A walk that visited nothing would pass every assertion above.
	if walked == 0 {
		t.Fatal("no guarded route was walked — the assertion above proved nothing")
	}
}

// TestExemptPathsAnswerWithoutAToken is the other half: the paths on the list
// have to actually be open, or the deployment has a liveness probe that returns
// 401 and a monitor that reports an outage that is not happening.
func TestExemptPathsAnswerWithoutAToken(t *testing.T) {
	handler, _ := assembled(t, nil)

	for _, path := range openPaths {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		if rec.Code != http.StatusOK {
			t.Errorf("%s answered %d without a token, want 200", path, rec.Code)
		}
	}
}

// TestUnknownPathsAreRefusedRatherThanDescribed: an unauthenticated caller
// probing for routes learns nothing about which ones exist, because the guard
// runs before the router decides.
func TestUnknownPathsAreRefusedRatherThanDescribed(t *testing.T) {
	handler, _ := assembled(t, nil)

	for _, path := range []string{"/v1/patients", "/v1/admin", "/internal/metrics", "/"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s answered %d, want 401", path, rec.Code)
		}
	}
}

// TestAuthenticatedRequestReachesTheEndpoint runs a real token through the real
// middleware into the real handler. Everything above proves what is refused;
// without this one, a middleware that refused everything would look perfect.
func TestAuthenticatedRequestReachesTheEndpoint(t *testing.T) {
	handler, fix := assembled(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+fix.token(t))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding the body: %v", err)
	}

	if body["sub"] != "8a1f3b7c-0000-4000-8000-000000000001" {
		t.Errorf("sub = %v", body["sub"])
	}
}

// TestHealthzStillReportsAFailingProbe: the exemption opens the path, it does
// not change what the path says.
func TestHealthzStillReportsAFailingProbe(t *testing.T) {
	handler, _ := assembled(t, func(context.Context) error {
		return errors.New("postgres is not answering")
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "postgres") {
		t.Errorf("the probe's cause reached the caller: %s", rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != httpserver.ProblemContentType {
		t.Errorf("Content-Type = %q, want %q", got, httpserver.ProblemContentType)
	}
}

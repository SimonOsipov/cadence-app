package router_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// assembled mounts every route the composition root mounts, so that what the
// tests below walk is the surface the process serves rather than a list written
// for them.
//
// It is not the same assembly: main.go also passes the two pools and the
// provisioner, and this passes none of the three. Every test here refuses before
// a handler needs one — that is what they are about — and giving them a database
// would make the transport's own tests depend on Docker.
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

// huma's downgrade converts type *arrays* only, so the scalar "type": "null" — 3.1's only
// spelling for «this $ref or null» — reaches the 3.0.3 document untouched, and a tool that is
// not 3.1 compatible is exactly the one that chokes on it.
//
// It also pins the mechanism: the repair works because chi's last registration wins, and the
// first assertion below fails if that ever stops being true.
func TestTheThirtyDocumentIsValid(t *testing.T) {
	handler, _ := assembled(t, func(context.Context) error { return nil })

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi-3.0.json", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var document map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &document); err != nil {
		t.Fatalf("the 3.0 document is not JSON: %v", err)
	}

	if got := document["openapi"]; got != "3.0.3" {
		t.Errorf("openapi = %v, want 3.0.3", got)
	}

	// Anywhere, not just on the three properties known today: a fourth nullable
	// $ref added later is the case this is here to catch.
	if found := whereNullTypeIs(document, ""); len(found) > 0 {
		t.Errorf("the 3.0 document carries a type 3.0 has no value for, at:\n  %s",
			strings.Join(found, "\n  "))
	}

	schemas, ok := document["components"].(map[string]any)["schemas"].(map[string]any)
	if !ok {
		t.Fatal("the 3.0 document has no component schemas")
	}

	for _, want := range []struct{ schema, property, ref string }{
		{"TodayBody", "meal_macros", "#/components/schemas/MacrosBody"},
		{"TodayBody", "targets", "#/components/schemas/MacrosBody"},
		{"RowBody", "compound", "#/components/schemas/CompoundBody"},
	} {
		owner, ok := schemas[want.schema].(map[string]any)
		if !ok {
			t.Errorf("%s is missing from the 3.0 document", want.schema)
			continue
		}
		property, ok := owner["properties"].(map[string]any)[want.property].(map[string]any)
		if !ok {
			t.Errorf("%s.%s is missing from the 3.0 document", want.schema, want.property)
			continue
		}

		// The whole point of the repair: the reference survives, so a generated
		// client still names the type instead of inlining an anonymous copy.
		if property["nullable"] != true {
			t.Errorf("%s.%s is not nullable in 3.0: %v", want.schema, want.property, property)
		}
		members, ok := property["allOf"].([]any)
		if !ok || len(members) != 1 {
			t.Errorf("%s.%s is not allOf with one member: %v", want.schema, want.property, property)
			continue
		}
		member, _ := members[0].(map[string]any)
		if got := member["$ref"]; got != want.ref {
			t.Errorf("%s.%s references %v, want %s", want.schema, want.property, got, want.ref)
		}
	}
}

// whereNullTypeIs answers the JSON paths carrying `"type": "null"`, so a failure
// names the property rather than only asserting that one exists.
func whereNullTypeIs(node any, at string) []string {
	var found []string

	switch value := node.(type) {
	case map[string]any:
		if value["type"] == "null" {
			found = append(found, at)
		}
		for key, child := range value {
			found = append(found, whereNullTypeIs(child, at+"."+key)...)
		}
	case []any:
		for i, item := range value {
			found = append(found, whereNullTypeIs(item, fmt.Sprintf("%s[%d]", at, i))...)
		}
	}

	return found
}

// The YAML document is the same document, and the repair has to reach it too —
// huma builds it by converting the JSON, and so does this.
func TestTheThirtyYAMLDocumentIsRepairedToo(t *testing.T) {
	handler, _ := assembled(t, func(context.Context) error { return nil })

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi-3.0.yaml", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, "type: null") {
		t.Error("the 3.0 YAML document still carries a null type")
	}
	if !strings.Contains(rec.Body.String(), "nullable: true") {
		t.Error("the 3.0 YAML document carries no nullable property")
	}
}

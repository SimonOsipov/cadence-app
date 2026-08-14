package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// The two secrets the test process is configured with. Long enough to satisfy
// the loader, and obviously not credentials.
const (
	testSecret         = "provisioner-shared-secret-for-tests-0001"
	testPreviousSecret = "provisioner-shared-secret-for-tests-0002"
)

// theFiveOperations is the whole contract of this component, written out here
// rather than read from the code that mounts it. A test that asks the router
// what it mounted agrees with the router by construction; this one disagrees
// the moment a sixth route appears, which is the entire point — the surface has
// already grown three times, and an operation nobody declared is an operation
// nobody reviewed.
var theFiveOperations = []string{
	"POST /invite",
	"POST /users/lookup",
	"POST /users/lookup-batch",
	"POST /users/delete",
	"POST /users/password",
}

// testServer builds the component the way main does, against a fake identity
// provider. A second assembly written for the tests would prove things about
// the test, so mux() below is the same function the composition root calls.
func testServer(t *testing.T, env environment, fake *fakeGoTrue) *server {
	t.Helper()

	key := testsupport.NewES256Key(t, "admin-key")

	signer, err := newTokenSigner(key.PrivateJWK(t))
	if err != nil {
		t.Fatalf("building the token signer: %v", err)
	}

	base := "http://gotrue.invalid"
	if fake != nil {
		base = fake.URL
	}

	return &server{
		environment: env,
		secrets:     newSecrets(testSecret, testPreviousSecret),
		gotrue:      newGoTrueClient(base, signer),
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// call sends a request carrying the current secret in the header the guard
// reads, which is the shape every test that is not about the guard needs.
func call(t *testing.T, srv *server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(secretHeader, testSecret)

	rec := httptest.NewRecorder()
	srv.mux().ServeHTTP(rec, req)

	return rec
}

// TestTheMuxMountsExactlyTheFiveOperations walks the real mux. What is
// protected is the list and not its length: a route swapped for another would
// keep the count and change the contract.
func TestTheMuxMountsExactlyTheFiveOperations(t *testing.T) {
	var mounted []string

	err := chi.Walk(testServer(t, development, nil).mux(),
		func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
			mounted = append(mounted, method+" "+route)

			return nil
		})
	if err != nil {
		t.Fatalf("walking the router: %v", err)
	}

	slices.Sort(mounted)

	want := slices.Clone(theFiveOperations)
	slices.Sort(want)

	if !slices.Equal(mounted, want) {
		t.Fatalf("the mounted surface is\n  %v\nand the declared one is\n  %v", mounted, want)
	}
}

// TestTheBatchIsBoundedInTimeAndNotOnlyInCount asserts the relationship
// between the three constants that decide how long one roster call may run,
// rather than describing it in a comment beside one of them.
//
// The count bound alone permits maxBatchLookup × gotrueCallTimeout, and
// WriteTimeout ends the response without cancelling the handler — so with the
// budget raised past that product, the caller chooses how long a handler runs
// by choosing how many identifiers to send.
func TestTheBatchIsBoundedInTimeAndNotOnlyInCount(t *testing.T) {
	if unbounded := maxBatchLookup * gotrueCallTimeout; batchBudget >= unbounded {
		t.Fatalf("the batch budget is %s and the fan-out it bounds is %s, so it bounds nothing",
			batchBudget, unbounded)
	}
}

// TestNoDebugOrMetricsPathIsServed is the other half of the walk, and it is not
// redundant with it: a handler registered on the default mux, or a route mounted
// by a middleware rather than by a chi route, never appears in chi.Walk. These
// requests carry a valid secret on purpose — a 401 would say the guard refused,
// which is not the same claim as "there is nothing there".
func TestNoDebugOrMetricsPathIsServed(t *testing.T) {
	srv := testServer(t, development, nil)

	for _, path := range []string{
		"/debug/pprof/", "/debug/pprof/goroutine", "/debug/vars",
		"/metrics", "/healthz", "/", "/admin/users",
	} {
		rec := call(t, srv, http.MethodGet, path, "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s answered %d, want 404 — something is mounted there", path, rec.Code)
		}
	}
}

// TestSettingAPasswordDoesNotExistInProduction. The seeds need a password so a
// doctor has something to sign in with; production has no such need, and an
// operation that can set any account's password is the identity-capture chain
// the whole story is about. It is absent from the mux rather than refused
// inside the handler: a route that exists is a route one flag away from
// serving.
func TestSettingAPasswordDoesNotExistInProduction(t *testing.T) {
	srv := testServer(t, production, nil)

	var mounted []string

	err := chi.Walk(srv.mux(), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		mounted = append(mounted, method+" "+route)

		return nil
	})
	if err != nil {
		t.Fatalf("walking the router: %v", err)
	}

	if slices.Contains(mounted, "POST /users/password") {
		t.Error("the production mux mounts password setting")
	}

	// The four that remain, so a production build that quietly lost a second
	// operation is a failure here rather than a discovery in an incident.
	slices.Sort(mounted)

	want := []string{"POST /invite", "POST /users/delete", "POST /users/lookup", "POST /users/lookup-batch"}
	if !slices.Equal(mounted, want) {
		t.Fatalf("the production surface is\n  %v\nwant\n  %v", mounted, want)
	}

	rec := call(t, srv, http.MethodPost, "/users/password", `{"id":"x","password":"y"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("POST /users/password answered %d in production, want 404", rec.Code)
	}
}

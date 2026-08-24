package router_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth/token"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/storage"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
	"github.com/SimonOsipov/cadence-app/api/internal/router"
)

// recordingPhotos answers a link, or the failure it was given, and remembers
// which bucket it was asked about.
type recordingPhotos struct {
	askedAbout []string
	refuse     error
}

func (r *recordingPhotos) SignedGet(
	_ context.Context, bucket, key, _ string, _ time.Duration,
) (storage.Link, error) {
	r.askedAbout = append(r.askedAbout, bucket)
	if r.refuse != nil {
		return storage.Link{}, r.refuse
	}

	return storage.Link{URL: "https://example.invalid/" + bucket + "/" + key}, nil
}

func (r *recordingPhotos) SignedPut(
	_ context.Context, bucket, key string, _ time.Duration,
) (storage.Link, error) {
	r.askedAbout = append(r.askedAbout, bucket)
	if r.refuse != nil {
		return storage.Link{}, r.refuse
	}

	return storage.Link{URL: "https://example.invalid/" + bucket + "/" + key}, nil
}

// Which bucket each context is handed is decided in two places — the registry's
// two entries here, and the composition root that fills Options from the config —
// and until this test nothing measured either. Both contexts' own suites build
// their service themselves, with their own constant, so exchanging the registry's
// two values leaves every one of them green while doses go to the labels' bucket.
//
// The upload is what this asks, because it is the one photograph operation that
// reaches the object store without first reading a row: no database is needed to
// see which bucket was handed over. What it does not reach is cmd/api, which has
// no test of its own — the config→Options half of the mapping is still unmeasured.
func TestTheContextsAreHandedTheirOwnBuckets(t *testing.T) {
	const (
		vials      = "a-bucket-of-labels"
		injections = "a-bucket-of-injections"
	)

	photos := &recordingPhotos{}
	mux, f := assembledWithPhotos(t, photos, vials, injections)

	body := strings.NewReader(`{"content_type":"image/jpeg"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/me/dose-events/photo-uploads", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+aPatientsToken(t, f))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("asking for an upload answered %d: %s", rec.Code, rec.Body)
	}
	if len(photos.askedAbout) != 1 {
		t.Fatalf("the store was asked about %v", photos.askedAbout)
	}
	if photos.askedAbout[0] != injections {
		t.Errorf("a dose photograph was signed against %q, want %q", photos.askedAbout[0], injections)
	}

	var upload struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &upload); err != nil {
		t.Fatalf("reading the reply: %v", err)
	}
	if strings.Contains(upload.URL, vials) {
		t.Errorf("the link handed out addresses the labels' bucket: %s", upload.URL)
	}
}

// assembledWithPhotos mounts the whole surface the way the composition root
// does, with the object store replaced and the two buckets given names that
// cannot be confused for each other.
//
// The pool is real but never reached — this operation opens no transaction and pgxpool
// connects lazily. It must not be nil: that is how the document generator is told to declare
// the operations without wiring them.
func assembledWithPhotos(
	t *testing.T, photos router.Photos, vials, injections string,
) (*chi.Mux, *fixture) {
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
	pool, err := pgxpool.New(t.Context(), "postgres://nobody@127.0.0.1:1/nothing")
	if err != nil {
		t.Fatalf("building an unused pool: %v", err)
	}
	t.Cleanup(pool.Close)

	server := httpserver.New(httpserver.Config{Port: "0"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	router.Mount(server.Router, router.Options{
		Verifier:         verifier,
		Pool:             pool,
		Photos:           photos,
		VialsBucket:      vials,
		InjectionsBucket: injections,
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	return server.Router, &fixture{key: key, set: set}
}

// aPatientsToken is the fixture token with the claim the API actually reads for a
// role: cadence_role, put there by the issuance hook. fixture.token in
// mount_test.go carries no role, so a request made with it is refused as 403.
func aPatientsToken(t *testing.T, f *fixture) string {
	t.Helper()

	now := time.Now()

	return f.key.Sign(t, jwt.MapClaims{
		"sub":          "8a1f3b7c-0000-4000-8000-000000000001",
		"role":         "authenticated",
		"cadence_role": "patient",
		"aud":          "authenticated",
		"iss":          f.set.Issuer,
		"nbf":          now.Add(-time.Minute).Unix(),
		"exp":          now.Add(time.Hour).Unix(),
	})
}

// A signature that cannot be produced is a 500 and not a 503, and this is what
// says so. Signing is arithmetic and reaches nothing, so a failure here means the
// signer was built wrong — while a 503 tells the retry queue to come back, and it
// would come back for ever.
//
// Measured here rather than in the context's own suite because that one signs
// against a real MinIO, where the signature never fails.
func TestASignatureThatCannotBeProducedIsNotAnOutage(t *testing.T) {
	photos := &recordingPhotos{refuse: errors.New("the signer holds no credentials")}
	mux, f := assembledWithPhotos(t, photos, "a-bucket-of-labels", "a-bucket-of-injections")

	body := strings.NewReader(`{"content_type":"image/jpeg"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/me/dose-events/photo-uploads", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+aPatientsToken(t, f))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("a failed signature answered %d: %s", rec.Code, rec.Body)
	}
	// And the reason never reaches the caller: a 5xx keeps its status and loses
	// its detail, which is what Problem.normalise is for.
	if strings.Contains(rec.Body.String(), "credentials") {
		t.Errorf("the signer's failure reached the caller: %s", rec.Body)
	}
}

// The upload is the one path that mints a storage prefix out of the subject
// without opening a transaction, so it is the one place the subject's shape is
// examined at all. This drives it with a subject the verifier accepts and the
// minter does not.
//
// What it pins is which refusal comes back. A 422 naming content_type would blame
// the caller's body for the one gate that stands between two patients' prefixes,
// and the operator chasing it would be looking at image types.
func TestASubjectThatIsNotAnIdentifierIsNotTheClientsFault(t *testing.T) {
	photos := &recordingPhotos{}
	mux, f := assembledWithPhotos(t, photos, "a-bucket-of-labels", "a-bucket-of-injections")

	now := time.Now()
	token := f.key.Sign(t, jwt.MapClaims{
		"sub":          "not-an-identifier",
		"role":         "authenticated",
		"cadence_role": "patient",
		"aud":          "authenticated",
		"iss":          f.set.Issuer,
		"nbf":          now.Add(-time.Minute).Unix(),
		"exp":          now.Add(time.Hour).Unix(),
	})

	body := strings.NewReader(`{"content_type":"image/jpeg"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/me/dose-events/photo-uploads", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("a subject that is not an identifier answered %d: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "content_type") {
		t.Errorf("the refusal blames the caller's body: %s", rec.Body)
	}
	// And nothing was signed: the gate is before the store, not after it.
	if len(photos.askedAbout) != 0 {
		t.Errorf("the store was asked about %v", photos.askedAbout)
	}
}

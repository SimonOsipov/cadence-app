package router_test

import (
	"context"
	"encoding/json"
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

// recordingPhotos answers a link and remembers which bucket it was asked about.
type recordingPhotos struct{ askedAbout []string }

func (r *recordingPhotos) SignedGet(
	_ context.Context, bucket, key, _ string, ttl time.Duration,
) (storage.Link, error) {
	r.askedAbout = append(r.askedAbout, bucket)

	return storage.Link{URL: "https://example.invalid/" + bucket + "/" + key}, nil
}

func (r *recordingPhotos) SignedPut(
	_ context.Context, bucket, key string, ttl time.Duration,
) (storage.Link, error) {
	r.askedAbout = append(r.askedAbout, bucket)

	return storage.Link{URL: "https://example.invalid/" + bucket + "/" + key}, nil
}

// Which bucket each context is handed is decided in exactly one place — two
// adjacent lines of the registry — and nothing else in the tree measures it.
// Both contexts' own suites build their service themselves, with their own
// constant, so swapping those two lines leaves every one of them green while
// doses go to the labels' bucket and labels are looked for among the doses.
//
// The upload is what this asks, because it is the one photograph operation that
// reaches the object store without first reading a row: no database is needed to
// see which bucket the registry handed over. A swap shows up here, since after
// one this call would be answered about the vials bucket.
func TestTheContextsAreHandedTheirOwnBuckets(t *testing.T) {
	const (
		vials      = "a-bucket-of-labels"
		injections = "a-bucket-of-injections"
	)

	photos := &recordingPhotos{}
	mux := assembledWithPhotos(t, photos, vials, injections)

	body := strings.NewReader(`{"content_type":"image/jpeg"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/me/dose-events/photo-uploads", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+aPatientsToken(t))

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
// The pool is real but never reached: this one operation opens no transaction,
// and pgxpool connects lazily, so nothing here needs a database. What it does
// need is a pool that is not nil, because a nil one is how the document
// generator is told to declare the operations without wiring them.
func assembledWithPhotos(t *testing.T, photos router.Photos, vials, injections string) *chi.Mux {
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
	signingKey = key
	jwks = set

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

	return server.Router
}

var (
	signingKey *testsupport.SigningKey
	jwks       *testsupport.JWKS
)

// aPatientsToken is the fixture token with the claim the API actually reads for
// a role: cadence_role, put there by the issuance hook.
func aPatientsToken(t *testing.T) string {
	t.Helper()

	now := time.Now()

	return signingKey.Sign(t, jwt.MapClaims{
		"sub":          "8a1f3b7c-0000-4000-8000-000000000001",
		"role":         "authenticated",
		"cadence_role": "patient",
		"aud":          "authenticated",
		"iss":          jwks.Issuer,
		"nbf":          now.Add(-time.Minute).Unix(),
		"exp":          now.Add(time.Hour).Unix(),
	})
}

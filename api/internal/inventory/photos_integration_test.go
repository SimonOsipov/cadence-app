//go:build integration

package inventory_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/inventory"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/storage"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

const labelBucket = "cadence-vials"

var theSigningMoment = time.Date(2026, time.May, 10, 9, 0, 0, 0, time.UTC)

// withLabelPhotos assembles this context against a real object store.
func withLabelPhotos(t *testing.T, c clinic) (*chi.Mux, *storage.Signer, func(subject, role string)) {
	t.Helper()

	store := testsupport.StartObjectStore(t, labelBucket)
	signer, err := storage.New(storage.Config{
		Endpoint:        store.Endpoint,
		Region:          testsupport.MinIORegion,
		AccessKeyID:     testsupport.MinIOAccessKey,
		SecretAccessKey: testsupport.MinIOSecretKey,
		PathStyle:       true,
	}, func() time.Time { return theSigningMoment })
	if err != nil {
		t.Fatalf("building the signer: %v", err)
	}

	var subject, role string
	mux := chi.NewRouter()
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if subject == "" {
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(
				r.Context(), auth.Principal{Subject: subject, Role: role},
			)))
		})
	})
	inventory.NewService(func() time.Time { return theSigningMoment }, inventory.Deps{
		RequestPool: c.request,
		Photos:      signer,
		Bucket:      labelBucket,
	}).Register(httpserver.NewAPI(mux))

	return mux, signer, func(s, r string) { subject, role = s, r }
}

// attachLabel puts a picture in the store and points the vial's row at it, under
// the patient's own identity — the path the product uses.
func attachLabel(t *testing.T, c clinic, signer *storage.Signer, patient, vial string, picture []byte) string {
	t.Helper()

	key, err := storage.NewKey(patient, "image/jpeg")
	if err != nil {
		t.Fatalf("minting a key: %v", err)
	}

	upload, err := signer.SignedPut(t.Context(), labelBucket, key, time.Minute)
	if err != nil {
		t.Fatalf("signing the upload: %v", err)
	}
	if got := putObject(t, upload.URL, picture); got != http.StatusOK {
		t.Fatalf("the signed upload answered %d", got)
	}

	if err := database.WithCaller(t.Context(), c.request,
		database.Caller{Subject: patient, Role: "patient"},
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx,
				`UPDATE app.vials SET label_photo_path = $1 WHERE id = $2`, key, vial)
			return err
		}); err != nil {
		t.Fatalf("attaching the label: %v", err)
	}

	return key
}

func callLabel(t *testing.T, mux *chi.Mux, vial string) (int, string) {
	t.Helper()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(
		http.MethodGet, "/v1/me/vials/"+vial+"/label-photo", nil,
	))

	return rec.Code, rec.Body.String()
}

// The patient's own label, read through the link the endpoint hands out.
func TestAPatientReadsTheLabelOfTheirOwnVial(t *testing.T) {
	c := newClinic(t)
	mux, signer, as := withLabelPhotos(t, c)

	picture := []byte("a photograph of a vial's label")
	attachLabel(t, c, signer, patientA, c.vialA, picture)

	as(patientA, "patient")
	status, body := callLabel(t, mux, c.vialA)
	if status != http.StatusOK {
		t.Fatalf("answered %d: %s", status, body)
	}

	var link struct {
		URL       string `json:"url"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal([]byte(body), &link); err != nil {
		t.Fatalf("reading the reply: %v", err)
	}
	if want := theSigningMoment.Add(inventory.LinkLifetime).UTC().Format(time.RFC3339); link.ExpiresAt != want {
		t.Errorf("expires_at = %q, want %q", link.ExpiresAt, want)
	}

	code, got := getObject(t, link.URL)
	if code != http.StatusOK {
		t.Fatalf("the signed link answered %d: %s", code, got)
	}
	if !bytes.Equal(got, picture) {
		t.Errorf("the label read back as %q", got)
	}
}

// «Право проверяется по RLS строки прежде, чем ссылка выдаётся» — the acceptance
// criterion, answered here for the cabinet: B asks for A's label by A's own vial
// id and is told there is nothing to read.
func TestAnotherPatientGetsNoLinkToThisOnesLabel(t *testing.T) {
	c := newClinic(t)
	mux, signer, as := withLabelPhotos(t, c)

	key := attachLabel(t, c, signer, patientA, c.vialA, []byte("a photograph"))

	as(patientB, "patient")
	status, refusedToB := callLabel(t, mux, c.vialA)
	if status != http.StatusNotFound {
		t.Fatalf("B was answered %d for A's label: %s", status, refusedToB)
	}

	// And the same one A gets for a vial with no photograph, byte for byte on the
	// same path: the two told apart is an oracle for which vials exist.
	detachLabel(t, c, patientA, c.vialA)

	as(patientA, "patient")
	status, refusedToA := callLabel(t, mux, c.vialA)
	if status != http.StatusNotFound {
		t.Fatalf("A was answered %d for a vial with no label: %s", status, refusedToA)
	}
	if refusedToB != refusedToA {
		t.Errorf("the two refusals differ:\n  B: %s\n  A: %s", refusedToB, refusedToA)
	}
	if strings.Contains(refusedToB, key) {
		t.Errorf("the refusal carries A's key: %s", refusedToB)
	}
}

func detachLabel(t *testing.T, c clinic, patient, vial string) {
	t.Helper()

	if err := database.WithCaller(t.Context(), c.request,
		database.Caller{Subject: patient, Role: "patient"},
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx,
				`UPDATE app.vials SET label_photo_path = NULL WHERE id = $1`, vial)
			return err
		}); err != nil {
		t.Fatalf("detaching the label: %v", err)
	}
}

// A vial with no label photograph is the same 404 as an invisible one, so the two
// cannot be told apart from outside.
func TestAVialWithNoLabelHasNoLink(t *testing.T) {
	c := newClinic(t)
	mux, _, as := withLabelPhotos(t, c)

	as(patientA, "patient")
	if status, body := callLabel(t, mux, c.vialA); status != http.StatusNotFound {
		t.Errorf("a vial with no label answered %d: %s", status, body)
	}
}

// An id nobody owns is the same refusal as one somebody else owns: a different
// status would turn this endpoint into an oracle for which vials exist.
func TestAnUnknownVialIsTheSameRefusalAsSomebodyElses(t *testing.T) {
	c := newClinic(t)
	mux, _, as := withLabelPhotos(t, c)

	as(patientA, "patient")
	if status, _ := callLabel(t, mux, "7c1a2b3d-4e5f-4a6b-8c9d-0e1f2a3b4c5d"); status != http.StatusNotFound {
		t.Errorf("an unknown vial answered %d", status)
	}
}

// The same /v1/me refusals every neighbouring surface makes. The doctor is
// refused although the policies would hand them the row: admitting them now
// would publish a surface nothing has asked for.
func TestOnlyAPatientReachesTheLabel(t *testing.T) {
	c := newClinic(t)
	mux, signer, as := withLabelPhotos(t, c)

	attachLabel(t, c, signer, patientA, c.vialA, []byte("a photograph"))

	as(doctorA, "doctor")
	if status, body := callLabel(t, mux, c.vialA); status != http.StatusForbidden {
		t.Errorf("A's own doctor answered %d: %s", status, body)
	}

	as("", "")
	if status, body := callLabel(t, mux, c.vialA); status != http.StatusUnauthorized {
		t.Errorf("an unauthenticated call answered %d: %s", status, body)
	}
}

func putObject(t *testing.T, address string, body []byte) int {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodPut, address, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("building the upload: %v", err)
	}
	request.Header.Set("Content-Type", "image/jpeg")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("uploading: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, response.Body)

	return response.StatusCode
}

func getObject(t *testing.T, address string) (int, []byte) {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, address, nil)
	if err != nil {
		t.Fatalf("building the read: %v", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	read, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("reading the body: %v", err)
	}

	return response.StatusCode, read
}

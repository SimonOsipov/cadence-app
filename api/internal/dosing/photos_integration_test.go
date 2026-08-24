//go:build integration

package dosing_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/dosing"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/storage"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

const photoBucket = "cadence-injections"

// withPhotos assembles this context against a real object store: a stubbed signer
// would leave the one thing that decides access unmeasured.
func withPhotos(t *testing.T, c clinic) (*chi.Mux, func(subject, role string)) {
	t.Helper()

	store := testsupport.StartObjectStore(t, photoBucket)
	signer, err := storage.New(storage.Config{
		Endpoint:        store.Endpoint,
		Region:          testsupport.MinIORegion,
		AccessKeyID:     testsupport.MinIOAccessKey,
		SecretAccessKey: testsupport.MinIOSecretKey,
		PathStyle:       true,
	}, func() time.Time { return theMoment })
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
	dosing.NewService(func() time.Time { return theMoment }, dosing.Deps{
		RequestPool: c.request,
		Photos:      signer,
		PhotoBucket: photoBucket,
	}).Register(httpserver.NewAPI(mux))

	return mux, func(s, r string) { subject, role = s, r }
}

func call(t *testing.T, mux *chi.Mux, method, path, payload string) (int, string) {
	t.Helper()

	var body io.Reader
	if payload != "" {
		body = strings.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, body)
	if payload != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return rec.Code, rec.Body.String()
}

// The whole feature, end to end and in the order the app performs it: ask for
// somewhere to put the picture, put it there, record the dose naming the key,
// then ask for the picture back and read the bytes that were written.
//
// Assembled rather than unit-tested because every seam here is one that has
// already failed in this project: the key has to satisfy a CHECK, the row has to
// be visible to the reader, and the signature has to be one the store accepts.
func TestAPatientPutsAPhotographWithADoseAndReadsItBack(t *testing.T) {
	c := newClinic(t)
	mux, as := withPhotos(t, c)
	as(patientA, "patient")

	status, body := call(t, mux, http.MethodPost, "/v1/me/dose-events/photo-uploads",
		`{"content_type":"image/jpeg"}`)
	if status != http.StatusCreated {
		t.Fatalf("asking for an upload answered %d: %s", status, body)
	}

	var upload struct {
		URL       string `json:"url"`
		Key       string `json:"key"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal([]byte(body), &upload); err != nil {
		t.Fatalf("reading the upload reply: %v", err)
	}

	// Minted under the caller's own prefix, which is the whole reason the server
	// mints it: the CHECK on dose_events names patient_id, and a client-chosen
	// key would make that CHECK the only thing standing in the way.
	if !strings.HasPrefix(upload.Key, patientA+"/") {
		t.Errorf("the minted key %q is not under the patient's own prefix", upload.Key)
	}
	if want := theMoment.Add(dosing.LinkLifetime).UTC().Format(time.RFC3339); upload.ExpiresAt != want {
		t.Errorf("expires_at = %q, want %q", upload.ExpiresAt, want)
	}

	picture := []byte("the bytes of a photograph")
	if got := putObject(t, upload.URL, picture); got != http.StatusOK {
		t.Fatalf("the signed upload answered %d", got)
	}

	status, body = send(t, c, patientA, aPayload(c, patientA, func(payload map[string]any) {
		payload["photo_path"] = upload.Key
	}))
	if status != http.StatusOK {
		t.Fatalf("recording the dose answered %d: %s", status, body)
	}
	var recorded struct {
		Outcome string `json:"outcome"`
		EventID string `json:"dose_event_id"`
	}
	if err := json.Unmarshal([]byte(body), &recorded); err != nil {
		t.Fatalf("reading the dose reply: %v", err)
	}
	if recorded.Outcome != "written" {
		t.Fatalf("the dose was not written: %s", body)
	}

	status, body = call(t, mux, http.MethodGet, "/v1/me/dose-events/"+recorded.EventID+"/photo", "")
	if status != http.StatusOK {
		t.Fatalf("asking for the photograph answered %d: %s", status, body)
	}
	var link struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(body), &link); err != nil {
		t.Fatalf("reading the link reply: %v", err)
	}

	code, got := getObject(t, link.URL)
	if code != http.StatusOK {
		t.Fatalf("the signed link answered %d: %s", code, got)
	}
	if !bytes.Equal(got, picture) {
		t.Errorf("the photograph read back as %q", got)
	}
}

// «Право проверяется по RLS строки прежде, чем ссылка выдаётся» is an acceptance
// criterion of this feature, and this is the test that answers it: B asks for A's
// photograph by A's own event id and is told there is nothing to read.
//
// Two patients rather than one: with a single patient the policy is a
// pass-through and this would be green with no boundary at all.
func TestAnotherPatientGetsNoLinkToThisOnesPhotograph(t *testing.T) {
	c := newClinic(t)
	mux, as := withPhotos(t, c)

	as(patientA, "patient")
	eventID, _ := aRecordedPhotograph(t, c, mux, patientA)

	as(patientB, "patient")
	status, refusedToB := call(t, mux, http.MethodGet, "/v1/me/dose-events/"+eventID+"/photo", "")
	if status != http.StatusNotFound {
		t.Fatalf("B was answered %d for A's photograph: %s", status, refusedToB)
	}

	// And it has to be the same refusal A gets for a dose of their own that
	// carries no photograph. Those two are the pair the code can tell apart — a
	// hidden row and an absent one are one answer at the database, since a policy
	// that refuses a row returns no rows — so this is where an oracle could open:
	// «not yours» distinguished from «no picture» lets B walk event ids and learn
	// which doses somebody else has recorded.
	//
	// Compared apart from `instance`, which carries the path and so differs by
	// construction.
	status, body := send(t, c, patientB, aPayload(c, patientB, func(payload map[string]any) {
		payload["client_request_id"] = "a-dose-with-no-picture"
	}))
	if status != http.StatusOK {
		t.Fatalf("recording B's own dose answered %d: %s", status, body)
	}
	var bare struct {
		Outcome string `json:"outcome"`
		EventID string `json:"dose_event_id"`
	}
	if err := json.Unmarshal([]byte(body), &bare); err != nil {
		t.Fatalf("reading the dose reply: %v", err)
	}
	if bare.Outcome != "written" {
		t.Fatalf("B's own dose was not written: %s", body)
	}

	_, refusedForNoPicture := call(t, mux, http.MethodGet,
		"/v1/me/dose-events/"+bare.EventID+"/photo", "")

	if got, want := saidBy(t, refusedToB), saidBy(t, refusedForNoPicture); got != want {
		t.Errorf("A's event is refused to B as %q and a picture-less one as %q", got, want)
	}
	if strings.Contains(refusedToB, patientA) {
		t.Errorf("the refusal names A: %s", refusedToB)
	}
}

// saidBy is a problem document minus its instance, which is the request path and
// so differs between two calls by construction.
func saidBy(t *testing.T, body string) string {
	t.Helper()

	var problem map[string]any
	if err := json.Unmarshal([]byte(body), &problem); err != nil {
		t.Fatalf("the refusal is not a problem document: %v", err)
	}
	delete(problem, "instance")

	said, err := json.Marshal(problem)
	if err != nil {
		t.Fatalf("re-reading the refusal: %v", err)
	}

	return string(said)
}

// A dose recorded without a picture has no link, and the answer is the same 404
// the invisible row gets — so the two cannot be told apart from outside.
func TestADoseWithNoPhotographHasNoLink(t *testing.T) {
	c := newClinic(t)
	mux, as := withPhotos(t, c)
	as(patientA, "patient")

	status, body := send(t, c, patientA, aPayload(c, patientA, nil))
	if status != http.StatusOK {
		t.Fatalf("recording the dose answered %d: %s", status, body)
	}
	var recorded struct {
		EventID string `json:"dose_event_id"`
	}
	if err := json.Unmarshal([]byte(body), &recorded); err != nil {
		t.Fatalf("reading the dose reply: %v", err)
	}

	if status, body := call(t, mux, http.MethodGet,
		"/v1/me/dose-events/"+recorded.EventID+"/photo", ""); status != http.StatusNotFound {
		t.Errorf("a dose with no photograph answered %d: %s", status, body)
	}
}

// An event id nobody owns is the same 404 as one somebody else owns: a different
// status would turn this endpoint into an oracle for which ids exist.
func TestAnUnknownEventIsTheSameRefusalAsSomebodyElses(t *testing.T) {
	c := newClinic(t)
	mux, as := withPhotos(t, c)
	as(patientA, "patient")

	status, _ := call(t, mux, http.MethodGet,
		"/v1/me/dose-events/9f3c1c2e-0f6b-4f2a-8f7a-1d2e3f4a5b6c/photo", "")
	if status != http.StatusNotFound {
		t.Errorf("an unknown event answered %d", status)
	}
}

// The same /v1/me refusals every neighbouring surface makes, on both operations.
func TestOnlyAPatientReachesThePhotographs(t *testing.T) {
	c := newClinic(t)
	mux, as := withPhotos(t, c)

	const somePhoto = "/v1/me/dose-events/9f3c1c2e-0f6b-4f2a-8f7a-1d2e3f4a5b6c/photo"
	const uploads = "/v1/me/dose-events/photo-uploads"

	for _, call1 := range []struct{ method, path, payload string }{
		{http.MethodGet, somePhoto, ""},
		{http.MethodPost, uploads, `{"content_type":"image/jpeg"}`},
	} {
		as(doctorA, "doctor")
		if status, body := call(t, mux, call1.method, call1.path, call1.payload); status != http.StatusForbidden {
			t.Errorf("a doctor calling %s answered %d: %s", call1.path, status, body)
		}

		as("", "")
		if status, body := call(t, mux, call1.method, call1.path, call1.payload); status != http.StatusUnauthorized {
			t.Errorf("an unauthenticated call to %s answered %d: %s", call1.path, status, body)
		}
	}
}

// The closed set is enforced at the transport too, not only inside NewKey: the
// extension the key carries is all the read side has to decide what an object is
// served as, so a type outside the set has no extension it could be stored under.
func TestATypeOutsideTheSetGetsNoUploadLink(t *testing.T) {
	c := newClinic(t)
	mux, as := withPhotos(t, c)
	as(patientA, "patient")

	for _, contentType := range []string{"text/html", "application/pdf", "image/svg+xml"} {
		status, body := call(t, mux, http.MethodPost, "/v1/me/dose-events/photo-uploads",
			`{"content_type":"`+contentType+`"}`)
		if status != http.StatusUnprocessableEntity {
			t.Errorf("%s was answered %d: %s", contentType, status, body)
		}
	}
}

// Two uploads are two objects. Without it the second photograph would overwrite
// the first, and the earlier dose's row would point at the later dose's picture.
func TestTwoUploadsAreTwoKeys(t *testing.T) {
	c := newClinic(t)
	mux, as := withPhotos(t, c)
	as(patientA, "patient")

	seen := map[string]bool{}
	for range 3 {
		status, body := call(t, mux, http.MethodPost, "/v1/me/dose-events/photo-uploads",
			`{"content_type":"image/png"}`)
		if status != http.StatusCreated {
			t.Fatalf("asking for an upload answered %d: %s", status, body)
		}
		var upload struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal([]byte(body), &upload); err != nil {
			t.Fatalf("reading the upload reply: %v", err)
		}
		if seen[upload.Key] {
			t.Fatalf("a second upload was handed the key %q", upload.Key)
		}
		seen[upload.Key] = true

		if !regexp.MustCompile(`\.png$`).MatchString(upload.Key) {
			t.Errorf("a key minted for image/png is %q", upload.Key)
		}
	}
}

// aRecordedPhotograph does the upload, the write and the dose in one go, for the
// tests whose subject is what happens afterwards.
func aRecordedPhotograph(t *testing.T, c clinic, mux *chi.Mux, patient string) (eventID, key string) {
	t.Helper()

	status, body := call(t, mux, http.MethodPost, "/v1/me/dose-events/photo-uploads",
		`{"content_type":"image/jpeg"}`)
	if status != http.StatusCreated {
		t.Fatalf("asking for an upload answered %d: %s", status, body)
	}
	var upload struct {
		URL string `json:"url"`
		Key string `json:"key"`
	}
	if err := json.Unmarshal([]byte(body), &upload); err != nil {
		t.Fatalf("reading the upload reply: %v", err)
	}
	if got := putObject(t, upload.URL, []byte("a photograph")); got != http.StatusOK {
		t.Fatalf("the signed upload answered %d", got)
	}

	status, body = send(t, c, patient, aPayload(c, patient, func(payload map[string]any) {
		payload["photo_path"] = upload.Key
	}))
	if status != http.StatusOK {
		t.Fatalf("recording the dose answered %d: %s", status, body)
	}
	var recorded struct {
		Outcome string `json:"outcome"`
		EventID string `json:"dose_event_id"`
	}
	if err := json.Unmarshal([]byte(body), &recorded); err != nil {
		t.Fatalf("reading the dose reply: %v", err)
	}
	if recorded.Outcome != "written" {
		t.Fatalf("the dose was not written: %s", body)
	}

	return recorded.EventID, upload.Key
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

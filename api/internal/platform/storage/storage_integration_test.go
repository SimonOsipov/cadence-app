//go:build integration

package storage_test

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/storage"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

const bucket = "cadence-vials"

// theMoment is the clock the signer is built with, so a link's stated expiry can
// be compared with the moment it was signed rather than with time.Now.
var theMoment = time.Date(2026, time.May, 10, 9, 0, 0, 0, time.UTC)

func signerAgainst(t *testing.T, store *testsupport.ObjectStore) *storage.Signer {
	t.Helper()

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

	return signer
}

// The round trip, and the only thing that says the signature is right: a link is
// written by one method and read by another, against a store that verifies it.
// A unit test could only assert the URL's shape, which is the SDK's business and
// not evidence that anything would open.
func TestASignedLinkWritesAndReadsAnObject(t *testing.T) {
	store := testsupport.StartObjectStore(t, bucket)
	signer := signerAgainst(t, store)

	const key = "11111111-1111-1111-1111-111111111111/label.jpg"
	body := []byte("a photograph of a vial's label")

	upload, err := signer.SignedPut(t.Context(), bucket, key, time.Minute)
	if err != nil {
		t.Fatalf("signing the write: %v", err)
	}
	if got := put(t, upload.URL, "image/jpeg", body); got != http.StatusOK {
		t.Fatalf("the signed write answered %d", got)
	}

	read, err := signer.SignedGet(t.Context(), bucket, key, "image/jpeg", time.Minute)
	if err != nil {
		t.Fatalf("signing the read: %v", err)
	}
	status, got := get(t, read.URL)
	if status != http.StatusOK {
		t.Fatalf("the signed read answered %d: %s", status, got)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("the object read back as %q", got)
	}
}

// «Публичных URL нет» is an acceptance criterion of this feature, and this is
// where it is answered: the same address without the signature is refused by the
// store, so a link that leaks is the only way in — which is why it is short.
func TestTheSameAddressWithoutASignatureIsRefused(t *testing.T) {
	store := testsupport.StartObjectStore(t, bucket)
	signer := signerAgainst(t, store)

	const key = "22222222-2222-2222-2222-222222222222/label.jpg"

	upload, err := signer.SignedPut(t.Context(), bucket, key, time.Minute)
	if err != nil {
		t.Fatalf("signing the write: %v", err)
	}
	if got := put(t, upload.URL, "image/jpeg", []byte("private")); got != http.StatusOK {
		t.Fatalf("the signed write answered %d", got)
	}

	read, err := signer.SignedGet(t.Context(), bucket, key, "image/jpeg", time.Minute)
	if err != nil {
		t.Fatalf("signing the read: %v", err)
	}

	unsigned := read.URL[:strings.Index(read.URL, "?")]
	if status, body := get(t, unsigned); status == http.StatusOK {
		t.Errorf("the bucket answered an unsigned read with %d: %s", status, body)
	}
}

// A link that outlived its expiry is refused by the store and not merely by us,
// which is what makes «short-lived» a property of the link rather than of the
// endpoint's good manners.
func TestAnExpiredLinkIsRefused(t *testing.T) {
	store := testsupport.StartObjectStore(t, bucket)
	signer := signerAgainst(t, store)

	const key = "33333333-3333-3333-3333-333333333333/label.jpg"

	upload, err := signer.SignedPut(t.Context(), bucket, key, time.Minute)
	if err != nil {
		t.Fatalf("signing the write: %v", err)
	}
	if got := put(t, upload.URL, "image/jpeg", []byte("private")); got != http.StatusOK {
		t.Fatalf("the signed write answered %d", got)
	}

	// One second, then waited out. The alternative — signing against a clock in
	// the past — measures nothing here: the SDK stamps the request from the real
	// clock, so the injected one would change only ExpiresAt.
	read, err := signer.SignedGet(t.Context(), bucket, key, "image/jpeg", time.Second)
	if err != nil {
		t.Fatalf("signing the read: %v", err)
	}
	time.Sleep(2 * time.Second)

	if status, body := get(t, read.URL); status == http.StatusOK {
		t.Errorf("an expired link answered %d: %s", status, body)
	}
}

// What a client declares on the way in is not signed — measured: this SDK names
// only `host` in X-Amz-SignedHeaders, and the store takes the write. So the type
// is pinned on the way out instead, and this is the test that says the pin
// holds: an object stored as text/html is served as an image, as an attachment.
//
// Without it, a patient could put a script into a private bucket and then ask
// the API for a link that hands it back from a host the API vouches for.
func TestTheReadPinsTheTypeThatTheWriteCouldNot(t *testing.T) {
	store := testsupport.StartObjectStore(t, bucket)
	signer := signerAgainst(t, store)

	const key = "44444444-4444-4444-4444-444444444444/label.jpg"

	upload, err := signer.SignedPut(t.Context(), bucket, key, time.Minute)
	if err != nil {
		t.Fatalf("signing the write: %v", err)
	}
	// The control for the sentence above: if a future SDK started signing the
	// declared type, this would stop being 200 and the pin below would be
	// measuring something that can no longer happen.
	if got := put(t, upload.URL, "text/html", []byte("<script>alert(1)</script>")); got != http.StatusOK {
		t.Fatalf("the write declaring text/html answered %d, so the type is signed after all", got)
	}

	read, err := signer.SignedGet(t.Context(), bucket, key, "image/jpeg", time.Minute)
	if err != nil {
		t.Fatalf("signing the read: %v", err)
	}

	status, header := getWithHeader(t, read.URL)
	if status != http.StatusOK {
		t.Fatalf("the signed read answered %d", status)
	}
	if got := header.Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", got)
	}
	if got := header.Get("Content-Disposition"); got != "attachment" {
		t.Errorf("Content-Disposition = %q, want attachment", got)
	}
}

// A read whose content type was dropped from the query is refused: the type is
// inside the signature, so removing it is tampering rather than an option.
func TestAReadStrippedOfItsPinnedTypeIsRefused(t *testing.T) {
	store := testsupport.StartObjectStore(t, bucket)
	signer := signerAgainst(t, store)

	const key = "66666666-6666-6666-6666-666666666666/label.jpg"

	upload, err := signer.SignedPut(t.Context(), bucket, key, time.Minute)
	if err != nil {
		t.Fatalf("signing the write: %v", err)
	}
	if got := put(t, upload.URL, "image/jpeg", []byte("a photograph")); got != http.StatusOK {
		t.Fatalf("the signed write answered %d", got)
	}

	read, err := signer.SignedGet(t.Context(), bucket, key, "image/jpeg", time.Minute)
	if err != nil {
		t.Fatalf("signing the read: %v", err)
	}

	address, err := url.Parse(read.URL)
	if err != nil {
		t.Fatalf("parsing the link: %v", err)
	}
	query := address.Query()
	query.Del("response-content-type")
	address.RawQuery = query.Encode()

	if status, body := get(t, address.String()); status == http.StatusOK {
		t.Errorf("a link stripped of its pinned type answered %d: %s", status, body)
	}
}

// ExpiresAt is measured against the clock the signer was built with, not the
// wall clock, so a caller can state the link's lifetime to its own client.
func TestTheExpiryIsCountedFromTheSigningClock(t *testing.T) {
	store := testsupport.StartObjectStore(t, bucket)
	signer := signerAgainst(t, store)

	link, err := signer.SignedGet(t.Context(), bucket, "55555555-5555-5555-5555-555555555555/l.jpg", "image/jpeg", 90*time.Second)
	if err != nil {
		t.Fatalf("signing the read: %v", err)
	}

	if want := theMoment.Add(90 * time.Second); !link.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %s, want %s", link.ExpiresAt, want)
	}
}

func put(t *testing.T, url, contentType string, body []byte) int {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("building the write: %v", err)
	}
	request.Header.Set("Content-Type", contentType)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("writing: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, response.Body)

	return response.StatusCode
}

func get(t *testing.T, url string) (int, []byte) {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("building the read: %v", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("reading the body: %v", err)
	}

	return response.StatusCode, body
}

func getWithHeader(t *testing.T, address string) (int, http.Header) {
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
	_, _ = io.Copy(io.Discard, response.Body)

	return response.StatusCode, response.Header
}

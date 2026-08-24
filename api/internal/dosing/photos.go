package dosing

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/storage"
)

// LinkLifetime is how long a signed link lasts: short because whoever holds one
// reads the object, long enough for a phone on a slow connection to start.
const LinkLifetime = 5 * time.Minute

// Photos signs short-lived links to stored objects.
type Photos interface {
	SignedGet(ctx context.Context, bucket, key, contentType string, ttl time.Duration) (storage.Link, error)
	SignedPut(ctx context.Context, bucket, key string, ttl time.Duration) (storage.Link, error)
}

// ErrNoPhoto is one error for three cases — invisible, absent, no photograph —
// because which of them it was is a fact about somebody else's treatment.
var ErrNoPhoto = errors.New("no photograph is readable here")

// PhotoUploadInput asks for somewhere to put one photograph.
type PhotoUploadInput struct {
	Body struct {
		ContentType string `json:"content_type" enum:"image/jpeg,image/png,image/heic" doc:"What the client is about to upload. It decides the key's extension, and the read side serves the object as this and nothing else."`
	}
}

// PhotoUploadOutput is the link to write to and the key to send back afterwards.
type PhotoUploadOutput struct {
	Body struct {
		URL string `json:"url" doc:"A signed PUT. It constrains the key and not the bytes: neither the content type nor the size is bound by the signature."`
		Key string `json:"key" doc:"What to send as photo_path when recording the dose. The server minted it; a client-chosen key is never accepted."`

		ExpiresAt string `json:"expires_at" format:"date-time"`
	}
}

// PhotoInput names the recorded dose whose photograph is wanted.
type PhotoInput struct {
	EventID string `path:"eventId" format:"uuid"`
}

// PhotoOutput is a link that reads one object.
type PhotoOutput struct {
	Body struct {
		URL       string `json:"url"`
		ExpiresAt string `json:"expires_at" format:"date-time"`
	}
}

func (s *Service) registerPhotos(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "start-dose-photo-upload",
		Method:        http.MethodPost,
		Path:          "/v1/me/dose-events/photo-uploads",
		DefaultStatus: http.StatusCreated,
		Summary:       "Ask for somewhere to put a dose photograph",
		Description: "Answers a signed link to write one object to, and the key to send as " +
			"`photo_path` when the dose is recorded. The key is minted by the server and " +
			"never taken from the client, so it is always under the caller's own prefix. " +
			"The upload happens before the dose exists, which is why this names no dose.",
		Tags: []string{"dosing"},
		// No 503: this operation opens no transaction and signing reaches
		// nothing, so there is no dependency for it to be unavailable. A status
		// published and unreachable is a branch a client writes and never runs.
		Errors: []int{
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusUnprocessableEntity,
		},
	}, s.startPhotoUpload)

	huma.Register(api, huma.Operation{
		OperationID: "read-dose-photo",
		Method:      http.MethodGet,
		Path:        "/v1/me/dose-events/{eventId}/photo",
		Summary:     "A link to the photograph of a recorded dose",
		Description: "Answers a short-lived signed link. The right is decided here, before " +
			"the link exists: the object store has no row-level security, so the row is " +
			"read under the caller's own identity and the link is signed only if the " +
			"policies handed it over. An invisible row, a missing one and one carrying no " +
			"photograph are one 404 — which of the three it was is a fact about somebody " +
			"else's treatment.",
		Tags: []string{"dosing"},
		Errors: []int{
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusNotFound,
			http.StatusUnprocessableEntity,
			http.StatusServiceUnavailable,
		},
	}, s.readPhoto)
}

func (s *Service) startPhotoUpload(ctx context.Context, in *PhotoUploadInput) (*PhotoUploadOutput, error) {
	principal, err := s.photoCaller(ctx)
	if err != nil {
		return nil, err
	}

	key, err := storage.NewKey(principal.Subject, in.Body.ContentType)
	if err != nil {
		// Split, because the two are about different parties. A prefix that is
		// not one identifier is the *subject's* shape — nothing the caller put in
		// the body — and it is the whole of the tenant gate on this path, so
		// answering «your content type is wrong» would report the one refusal
		// that matters as somebody's typo, and drop it from the log besides.
		if errors.Is(err, storage.ErrPrefixNotAnIdentifier) {
			return nil, huma.Error500InternalServerError("this caller's subject is not an identifier", err)
		}

		return nil, huma.Error422UnprocessableEntity("content_type is not an image type this API stores", err)
	}

	link, err := s.photos.SignedPut(ctx, s.photoBucket, key, LinkLifetime)
	if err != nil {
		// A 500 and not a 503, although the object store is what the link is for:
		// signing is arithmetic and reaches nothing, so a failure here means the
		// signer was built wrong. A 503 tells the offline queue to retry, and it
		// would retry for ever something that will never start working.
		return nil, huma.Error500InternalServerError("the photograph's link cannot be signed", err)
	}

	out := &PhotoUploadOutput{}
	out.Body.URL = link.URL
	out.Body.Key = key
	out.Body.ExpiresAt = link.ExpiresAt.UTC().Format(time.RFC3339)

	return out, nil
}

func (s *Service) readPhoto(ctx context.Context, in *PhotoInput) (*PhotoOutput, error) {
	principal, err := s.photoCaller(ctx)
	if err != nil {
		return nil, err
	}

	var key string
	if err := database.WithCaller(ctx, s.requests, principal, func(ctx context.Context, tx pgx.Tx) error {
		found, err := photoKeyOf(ctx, tx, in.EventID)
		if err != nil {
			return err
		}
		key = found

		return nil
	}); err != nil {
		if errors.Is(err, ErrNoPhoto) {
			return nil, huma.Error404NotFound("no photograph is readable here")
		}
		if database.IsUnavailable(err) {
			return nil, huma.Error503ServiceUnavailable("the database is not answering", err)
		}

		// Its own mapping rather than answer(): that one is the dose write's, and
		// its default says «recording the dose» — which logProblem writes to the
		// log before the body is cleaned, so a failed read would be recorded under
		// the name of an operation that did not run.
		return nil, huma.Error500InternalServerError("reading the dose's photograph", err)
	}

	link, err := s.photos.SignedGet(ctx, s.photoBucket, key, storage.ContentTypeFor(key), LinkLifetime)
	if err != nil {
		return nil, huma.Error500InternalServerError("the photograph's link cannot be signed", err)
	}

	out := &PhotoOutput{}
	out.Body.URL = link.URL
	out.Body.ExpiresAt = link.ExpiresAt.UTC().Format(time.RFC3339)

	return out, nil
}

// photoKeyOf reads one row's key under whatever identity the transaction carries.
// No patient predicate on purpose: a predicate here would answer the same for the
// owner while hiding whether the policies do.
func photoKeyOf(ctx context.Context, tx pgx.Tx, eventID string) (string, error) {
	var key *string
	err := tx.QueryRow(ctx, `
		SELECT photo_path
		FROM app.dose_events
		WHERE id = $1
	`, eventID).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNoPhoto
	}
	if err != nil {
		return "", fmt.Errorf("reading the dose's photograph: %w", err)
	}
	if key == nil || *key == "" {
		return "", ErrNoPhoto
	}

	return *key, nil
}

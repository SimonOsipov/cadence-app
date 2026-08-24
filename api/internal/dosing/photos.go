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

// LinkLifetime is how long a signed link lasts.
//
// Short because the link is the whole of the authority: the store has no policies,
// so anyone holding one reads the object until it expires. Long enough for a phone
// on a slow connection to start the transfer — a link that dies before the picture
// arrives is a screen that never loads.
const LinkLifetime = 5 * time.Minute

// Photos signs short-lived links to stored objects.
//
// Declared here, by the consumer, and satisfied by platform/storage: this context
// decides who may see a photograph, and the signer must not be able to.
type Photos interface {
	SignedGet(ctx context.Context, bucket, key, contentType string, ttl time.Duration) (storage.Link, error)
	SignedPut(ctx context.Context, bucket, key string, ttl time.Duration) (storage.Link, error)
}

// ErrNoPhoto is what the reads answer for a row that is invisible, absent, or
// carries no photograph. One error for the three deliberately: which of them it
// was is a fact about somebody else's treatment.
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
		URL string `json:"url" doc:"A signed PUT. It constrains the key and not the bytes: a presigned SigV4 URL covers only the headers it names, and this SDK names host alone."`
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
			"`photo_path` when the dose is recorded. The key is minted here and never " +
			"taken from the client: both tables holding one constrain it by a CHECK naming " +
			"the patient, and a client-chosen key would make that CHECK the only thing " +
			"between a patient and another patient's prefix. The upload happens before the " +
			"dose exists, which is why this names no dose.",
		Tags: []string{"dosing"},
		Errors: []int{
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusUnprocessableEntity,
			http.StatusServiceUnavailable,
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
		return nil, huma.Error422UnprocessableEntity("content_type is not an image type this API stores")
	}

	link, err := s.photos.SignedPut(ctx, s.photoBucket, key, LinkLifetime)
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("the object store cannot be reached", err)
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

		return nil, answer(err)
	}

	link, err := s.photos.SignedGet(ctx, s.photoBucket, key, storage.ContentTypeFor(key), LinkLifetime)
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("the object store cannot be reached", err)
	}

	out := &PhotoOutput{}
	out.Body.URL = link.URL
	out.Body.ExpiresAt = link.ExpiresAt.UTC().Format(time.RFC3339)

	return out, nil
}

// photoKeyOf reads one row's key under whatever identity the transaction carries.
//
// No patient predicate, and that is the point: the policies are what decide, and a
// predicate here would answer the same for the owner while hiding whether they do.
// The tenant boundary is measured in this context's policy suite.
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

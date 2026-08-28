package inventory

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/storage"
)

// LinkLifetime is how long a signed link lasts; the same dosing gives a dose.
const LinkLifetime = 5 * time.Minute

// Photos signs short-lived links to stored objects.
//
// The read half only: a label is attached after the vial exists, nothing creates
// or updates a vial over HTTP until M4, and an upload link whose key no endpoint
// could store would be a hole with no user.
type Photos interface {
	SignedGet(ctx context.Context, bucket, key, contentType string, ttl time.Duration) (storage.Link, error)
}

// ErrNoPhoto is one error for three cases — invisible, absent, no photograph —
// because which of them it was is a fact about somebody else's cabinet.
var ErrNoPhoto = errors.New("no photograph is readable here")

type Service struct {
	now      func() time.Time
	requests *pgxpool.Pool
	photos   Photos
	bucket   string
}

// Deps is what this context needs from outside itself.
type Deps struct {
	RequestPool *pgxpool.Pool
	Photos      Photos
	Bucket      string
}

// NewService takes the clock positionally, for the reason protocol.NewService records: the
// cabinet's computed fields are answers about the patient's own day, and a package that reads
// time.Now cannot be asked about another one.
func NewService(now func() time.Time, deps Deps) *Service {
	return &Service{
		now: now, requests: deps.RequestPool, photos: deps.Photos, bucket: deps.Bucket,
	}
}

// LabelPhotoInput names the vial whose label photograph is wanted.
type LabelPhotoInput struct {
	VialID string `path:"vialId" format:"uuid"`
}

// LabelPhotoOutput is a link that reads one object.
type LabelPhotoOutput struct {
	Body struct {
		URL       string `json:"url"`
		ExpiresAt string `json:"expires_at" format:"date-time"`
	}
}

// Register mounts this context's operations on the API.
func (s *Service) Register(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "read-vial-label-photo",
		Method:      http.MethodGet,
		Path:        "/v1/me/vials/{vialId}/label-photo",
		Summary:     "A link to the photograph of a vial's label",
		Description: "Answers a short-lived signed link. The right is decided here, before " +
			"the link exists: the object store has no row-level security, so the vial is " +
			"read under the caller's own identity and the link is signed only if the " +
			"policies handed the row over. An invisible vial, a missing one and one with " +
			"no label photograph are one 404.",
		Tags: []string{"inventory"},
		Errors: []int{
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusNotFound,
			http.StatusUnprocessableEntity,
			http.StatusServiceUnavailable,
		},
	}, s.readLabelPhoto)

	s.registerReads(api)
}

func (s *Service) readLabelPhoto(ctx context.Context, in *LabelPhotoInput) (*LabelPhotoOutput, error) {
	if s.requests == nil || s.photos == nil || s.bucket == "" {
		return nil, huma.Error500InternalServerError(
			"this API was assembled without somewhere to keep photographs",
		)
	}

	caller, err := s.patientCalling(ctx)
	if err != nil {
		return nil, err
	}

	var key string
	if err := database.WithCaller(ctx, s.requests, caller, func(ctx context.Context, tx pgx.Tx) error {
		found, err := labelPhotoKeyOf(ctx, tx, in.VialID)
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

		return nil, huma.Error500InternalServerError("reading the vial's label", err)
	}

	link, err := s.photos.SignedGet(ctx, s.bucket, key, storage.ContentTypeFor(key), LinkLifetime)
	if err != nil {
		// A 500 and not a 503: signing reaches nothing, so a failure here is a
		// signer built wrong, and a 503 would have the client retry for ever.
		return nil, huma.Error500InternalServerError("the photograph's link cannot be signed", err)
	}

	out := &LabelPhotoOutput{}
	out.Body.URL = link.URL
	out.Body.ExpiresAt = link.ExpiresAt.UTC().Format(time.RFC3339)

	return out, nil
}

// labelPhotoKeyOf reads one vial's key under whatever identity the transaction
// carries; see dosing.photoKeyOf on why there is no patient predicate.
func labelPhotoKeyOf(ctx context.Context, tx pgx.Tx, vialID string) (string, error) {
	var key *string
	err := tx.QueryRow(ctx, `
		SELECT label_photo_path
		FROM app.vials
		WHERE id = $1
	`, vialID).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNoPhoto
	}
	if err != nil {
		return "", fmt.Errorf("reading the vial's label photograph: %w", err)
	}
	if key == nil || *key == "" {
		return "", ErrNoPhoto
	}

	return *key, nil
}

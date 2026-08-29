package inventory

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/storage"
)

// The refusals this side answers by name, so a CHECK does not reach the patient as a 23514
// about a form they filled in.
//
// Named rather than re-checked in Go: this write is one row, and every bound it can break maps
// to one field, so the amount's bounds stay where the schema keeps them. The string lengths are
// a different case and are duplicated in the tags above — huma refuses those at the door with
// the field named, which a CHECK cannot do. Draft.Check
// duplicates them on the course path for the opposite reason — a course is twelve items, and a
// constraint naming no row cannot say which one. Measured here: with a Go copy in place,
// deleting it changed no answer this suite could see.
var (
	ErrNoSuchCompound = errors.New("no such drug in the directory")
	ErrAmountTooFine  = errors.New("an amount is measured to the microgram, not finer")
	ErrAmountOffRange = errors.New("an amount lies between nothing and a hundred grams")
	ErrKeyNotTheirs   = errors.New("the label photo key is not one this API minted for you")

	// The one the lifecycle answers, and it is a conflict rather than a bad form: the
	// request is well made and the vial is in a state that refuses it.
	ErrAlreadyDisposed = errors.New("this vial was already thrown away")
)

type NewVialInput struct {
	Body NewVialBody
}

// NewVialBody is a vial as the patient's form fills it in. The server converts nothing and
// stores no count: the amount is written in the unit the box carries, and «sealed» is the
// absence of an opening date rather than a field anyone sets.
type NewVialBody struct {
	CompoundID         string         `json:"compound_id" format:"uuid"`
	ConcentrationLabel string         `json:"concentration_label" minLength:"1" maxLength:"50" doc:"What the box says, transcribed. Nothing computes with it."`
	TotalAmount        VialAmountBody `json:"total_amount"`
	ExpiresOn          string         `json:"expires_on" format:"date"`
	Lot                *string        `json:"lot,omitempty" minLength:"1" maxLength:"50"`
	LocationRU         *string        `json:"location_ru,omitempty" minLength:"1" maxLength:"100"`
	LabelPhotoPath     *string        `json:"label_photo_path,omitempty" doc:"A key this API minted at POST /v1/me/vials/label-photo-uploads. Refused where it points anywhere but the caller's own prefix — the object store has no row-level security, so the key is the only thing saying whose object it is."`
}

type NewVialOutput struct {
	Body VialBody
}

// LabelUploadInput asks for somewhere to put a photograph of a label.
type LabelUploadInput struct {
	Body struct {
		ContentType string `json:"content_type" enum:"image/jpeg,image/png,image/heic" doc:"What the client is about to upload. It decides the key's extension, and the read side serves the object as this and nothing else."`
	}
}

// LabelUploadOutput is where to write it and the key to keep.
//
// The key is answered and never chosen: the client sends it back with the vial, and a client
// that could choose one could choose a path under somebody else's prefix.
type LabelUploadOutput struct {
	Body struct {
		URL       string `json:"url"`
		Key       string `json:"key"`
		ExpiresAt string `json:"expires_at" format:"date-time"`
	}
}

type HeldBackInput struct {
	VialID string `path:"vialId" format:"uuid"`
	Body   struct {
		HeldBack bool `json:"held_back" doc:"True puts the vial aside, false takes it back. Sending the value it already carries is not an error and does not move the day it was set aside on."`
	}
}

type DisposeInput struct {
	VialID string `path:"vialId" format:"uuid"`
}

func (s *Service) registerLifecycle(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "hold-a-vial-back",
		Method:      http.MethodPut,
		Path:        "/v1/me/vials/{vialId}/held-back",
		Summary:     "Put a vial aside, or take it back",
		Description: "A vial the patient has set aside takes part in nothing the server " +
			"decides for them: it is not opened by a dose, not counted as supply, and " +
			"does not suppress the reorder hint as a spare. Idempotent — sending the " +
			"value it already carries answers the same card.",
		Tags: []string{"inventory"},
		Errors: []int{
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusNotFound,
			http.StatusConflict,
			http.StatusUnprocessableEntity,
			http.StatusServiceUnavailable,
		},
	}, s.holdBack)

	huma.Register(api, huma.Operation{
		OperationID: "dispose-of-a-vial",
		Method:      http.MethodPost,
		Path:        "/v1/me/vials/{vialId}/dispose",
		Summary:     "Throw a vial away",
		Description: "Marks the day it was thrown away and clears «set aside» in the same " +
			"write, because the two are opposite ends of one lifecycle and a vial that " +
			"could not be discarded while set aside would be a dead end. The row stays: " +
			"the doses drawn from it are history, and a delete would take them with it.",
		Tags: []string{"inventory"},
		Errors: []int{
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusNotFound,
			http.StatusConflict,
			http.StatusUnprocessableEntity,
			http.StatusServiceUnavailable,
		},
	}, s.dispose)
}

func (s *Service) holdBack(ctx context.Context, in *HeldBackInput) (*VialOutput, error) {
	return s.changeVial(ctx, in.VialID, func(ctx context.Context, tx pgx.Tx, vial, day string) error {
		// Written by value and not by toggle: PUT says what the vial should be, and the
		// day is only set where it was not set before, so repeating the request does not
		// move the day the patient put it aside on.
		//
		// Guarded on disposal like the disposal itself, and not only because 000021
		// forbids the two flags together: a thrown-away vial has no lifecycle left to
		// change, and without the guard the offline queue's delayed «put it aside» raised
		// that CHECK as a 500 — which a client retries for ever, where a 409 tells it to
		// read the card again.
		tag, err := tx.Exec(ctx, `
			UPDATE app.vials
			SET held_back_at = CASE WHEN $2 THEN coalesce(held_back_at, $3::date) END
			WHERE id = $1 AND disposed_at IS NULL
		`, vial, in.Body.HeldBack, day)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrAlreadyDisposed
		}

		return nil
	})
}

func (s *Service) dispose(ctx context.Context, in *DisposeInput) (*VialOutput, error) {
	return s.changeVial(ctx, in.VialID, func(ctx context.Context, tx pgx.Tx, vial, day string) error {
		// «Set aside» is cleared by the same statement: 000021 forbids the two together,
		// so leaving it would make a vial the patient shelved impossible to throw away —
		// the dead end that migration's own comment names.
		tag, err := tx.Exec(ctx, `
			UPDATE app.vials
			SET disposed_at = $2::date, held_back_at = NULL
			WHERE id = $1 AND disposed_at IS NULL
		`, vial, day)
		if err != nil {
			return err
		}
		// Zero rows is «no vial of mine» and «already thrown away» at once, and the two
		// are a 404 and a 409 — so the row is read rather than the count guessed at.
		if tag.RowsAffected() == 0 {
			return ErrAlreadyDisposed
		}

		return nil
	})
}

// changeVial runs one write against the caller's own vial and answers the card it became.
//
// The shelf is read first, so «not here», «not yours» and «not a readable identifier» are one
// 404 before anything is written. What zero rows means afterwards is then a single thing — the
// vial is in a state this write refuses — which is why the two answers can be told apart at
// all: a count alone cannot separate an absent row from a conflicting one.
func (s *Service) changeVial(
	ctx context.Context, asked string,
	write func(ctx context.Context, tx pgx.Tx, vial, day string) error,
) (*VialOutput, error) {
	caller, err := s.patientCalling(ctx)
	if err != nil {
		return nil, err
	}
	vial, ok := database.CanonicalUUID(asked)
	if !ok {
		return nil, huma.Error404NotFound("no vial is readable here")
	}

	out := &VialOutput{}
	if err := database.WithCaller(ctx, s.requests, caller, func(ctx context.Context, tx pgx.Tx) error {
		shelf, err := s.shelfOf(ctx, tx, civil.UserID(caller.Subject))
		if err != nil {
			return err
		}
		if !holds(shelf, vial) {
			return ErrNoVial
		}
		if err := write(ctx, tx, vial, shelf.today.String()); err != nil {
			return classifyWrite(err)
		}

		// Read back through the shelf that answers the card, so the client redraws what
		// the row became rather than what the request asked for.
		after, err := s.shelfOf(ctx, tx, civil.UserID(caller.Subject))
		if err != nil {
			return err
		}
		for _, held := range after.cabinet.vials {
			if string(held.ID) == vial {
				out.Body = after.render(held)

				return nil
			}
		}

		return fmt.Errorf("the vial %s was written and not read back", vial)
	}); err != nil {
		return nil, answerWrite(err)
	}

	return out, nil
}

func holds(shelf shelf, vial string) bool {
	for _, held := range shelf.cabinet.vials {
		if string(held.ID) == vial {
			return true
		}
	}

	return false
}

func (s *Service) registerWrites(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "add-vial",
		Method:        http.MethodPost,
		Path:          "/v1/me/vials",
		DefaultStatus: http.StatusCreated,
		Summary:       "Put a vial in the cabinet",
		Description: "Stores it sealed: an opening date is what «opened» means, and the " +
			"first dose drawn from a vial is what sets it. The amount is kept in the " +
			"unit the box carries and converted by nothing here. A label photograph is " +
			"attached by sending back the key this API minted for it.",
		Tags: []string{"inventory"},
		Errors: []int{
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusUnprocessableEntity,
			http.StatusServiceUnavailable,
		},
	}, s.addVial)

	huma.Register(api, huma.Operation{
		OperationID:   "start-label-photo-upload",
		Method:        http.MethodPost,
		Path:          "/v1/me/vials/label-photo-uploads",
		DefaultStatus: http.StatusCreated,
		Summary:       "A link to upload a photograph of a vial's label",
		Description: "Answers a short-lived link that writes exactly one object, and the " +
			"key to send back with the vial. The key is minted here and prefixed with " +
			"the caller's own identifier: a client choosing its own could write under " +
			"somebody else's prefix, and the store has no row-level security to stop it.",
		Tags: []string{"inventory"},
		// No 503, for the reason dosing's own upload records: this operation opens no
		// transaction and signing reaches nothing, so a status published here is a
		// branch a client writes and never runs.
		Errors: []int{
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusUnprocessableEntity,
		},
	}, s.startLabelUpload)
}

func (s *Service) addVial(ctx context.Context, in *NewVialInput) (*NewVialOutput, error) {
	caller, err := s.patientCalling(ctx)
	if err != nil {
		return nil, err
	}
	out := &NewVialOutput{}
	if err := database.WithCaller(ctx, s.requests, caller, func(ctx context.Context, tx pgx.Tx) error {
		id, err := insertVial(ctx, tx, caller.Subject, in.Body)
		if err != nil {
			return err
		}
		// Rendered by the shelf that answers the card, not assembled here: two
		// resolutions of one vial are two chances to disagree about what is in it,
		// and the client draws this answer as the card it just created.
		shelf, err := s.shelfOf(ctx, tx, civil.UserID(caller.Subject))
		if err != nil {
			return err
		}
		for _, vial := range shelf.cabinet.vials {
			if string(vial.ID) == id {
				out.Body = shelf.render(vial)

				return nil
			}
		}

		return fmt.Errorf("the vial %s was written and not read back", id)
	}); err != nil {
		return nil, answerWrite(err)
	}

	return out, nil
}

func insertVial(ctx context.Context, tx pgx.Tx, patient string, body NewVialBody) (string, error) {
	var id string
	// patient_id is the caller's and never the body's: a field for it would be a field to
	// forge, and the WITH CHECK behind this INSERT refuses one — which is the witness, not
	// the row count.
	err := tx.QueryRow(ctx, `
		INSERT INTO app.vials
		    (patient_id, compound_id, concentration_label, total_amount, amount_unit,
		     expires_on, lot, location_ru, label_photo_path)
		VALUES ($1, $2, $3, $4::numeric, $5, $6::date, $7, $8, $9)
		RETURNING id::text
	`, patient, body.CompoundID, body.ConcentrationLabel, body.TotalAmount.Value,
		body.TotalAmount.Unit, body.ExpiresOn, body.Lot, body.LocationRU,
		body.LabelPhotoPath).Scan(&id)
	if err != nil {
		return "", classifyWrite(err)
	}

	return id, nil
}

func (s *Service) startLabelUpload(ctx context.Context, in *LabelUploadInput) (*LabelUploadOutput, error) {
	caller, err := s.patientCalling(ctx)
	if err != nil {
		return nil, err
	}
	if s.photos == nil || s.bucket == "" {
		return nil, huma.Error500InternalServerError(
			"this API was assembled without somewhere to keep photographs",
		)
	}

	key, err := storage.NewKey(caller.Subject, in.Body.ContentType)
	if err != nil {
		// The two blame different parties, as dosing's own upload records: a subject
		// that is not an identifier is this API's problem, a content type is the
		// caller's.
		if errors.Is(err, storage.ErrPrefixNotAnIdentifier) {
			return nil, huma.Error500InternalServerError("this caller's subject is not an identifier", err)
		}

		return nil, huma.Error422UnprocessableEntity("content_type is not an image type this API stores")
	}

	link, err := s.photos.SignedPut(ctx, s.bucket, key, LinkLifetime)
	if err != nil {
		// A 500 and not a 503: signing reaches nothing, so a failure is a signer built
		// wrong, and a 503 would have the client retry it for ever.
		return nil, huma.Error500InternalServerError("the upload link cannot be signed", err)
	}

	out := &LabelUploadOutput{}
	out.Body.URL = link.URL
	out.Body.Key = key
	out.Body.ExpiresAt = link.ExpiresAt.UTC().Format(time.RFC3339)

	return out, nil
}

// classifyWrite names what the schema refused, so a check violation reaches the patient as the
// field they filled in rather than as an unclassified 500.
func classifyWrite(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}

	switch {
	case pgErr.Code == foreignKeyViolation && pgErr.ConstraintName == vialNamesADrug:
		return ErrNoSuchCompound
	case pgErr.Code == checkViolation && pgErr.ConstraintName == keyIsUnderItsOwnPrefix:
		return ErrKeyNotTheirs
	case pgErr.Code == checkViolation && pgErr.ConstraintName == amountIsNoFinerThanTheAtom:
		return ErrAmountTooFine
	case pgErr.Code == checkViolation &&
		(pgErr.ConstraintName == amountIsMoreThanNothing || pgErr.ConstraintName == amountIsUnderItsCeiling):
		return ErrAmountOffRange
	default:
		return err
	}
}

const (
	foreignKeyViolation = "23503"
	checkViolation      = "23514"

	vialNamesADrug             = "vials_compound_id_fkey"
	keyIsUnderItsOwnPrefix     = "vials_photo_key_is_under_its_own_prefix"
	amountIsNoFinerThanTheAtom = "vials_total_amount_scale_check"
	amountIsMoreThanNothing    = "vials_total_amount_check"
	amountIsUnderItsCeiling    = "vials_total_amount_magnitude_check"
)

func answerWrite(err error) error {
	switch {
	case errors.Is(err, ErrNoVial):
		return huma.Error404NotFound("no vial is readable here")
	case errors.Is(err, ErrAlreadyDisposed):
		// A conflict and not a bad form: the request is well made and the vial is in a
		// state that refuses it, which a client resolves by reading the card again.
		return huma.Error409Conflict(err.Error())
	case errors.Is(err, ErrNoSuchCompound), errors.Is(err, ErrKeyNotTheirs),
		errors.Is(err, ErrAmountTooFine), errors.Is(err, ErrAmountOffRange):
		// The schema's refusal read back as the field the caller filled in.
		return huma.Error422UnprocessableEntity(err.Error())
	case database.IsUnavailable(err):
		return huma.Error503ServiceUnavailable("the database is not answering", err)
	default:
		return huma.Error500InternalServerError("writing the vial", err)
	}
}

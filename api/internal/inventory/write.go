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
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// The refusals this side answers by name, so a CHECK does not reach the patient as a 23514
// about a form they filled in.
var (
	ErrNoSuchCompound = errors.New("no such drug in the directory")
	ErrAmountTooFine  = errors.New("an amount is measured to the microgram, not finer")
	ErrAmountOffRange = errors.New("an amount lies between nothing and a hundred grams")
	ErrKeyNotTheirs   = errors.New("the label photo key is not one this API minted for you")
)

// The vial's own ceiling from 000024, in each unit. A container is not an injection: a 10 мл
// vial of testosterone at 250 мг/мл is 2500 мг, which a dose ceiling would refuse.
const (
	maxVialMilligrams = 100_000
	maxVialMicrograms = 100_000_000
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
	Status int
	Body   VialBody
}

// LabelUploadInput asks for somewhere to put a photograph of a label.
type LabelUploadInput struct {
	Body struct {
		ContentType string `json:"content_type" enum:"image/jpeg,image/png,image/heic,image/webp"`
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
		OperationID: "start-label-photo-upload",
		Method:      http.MethodPost,
		Path:        "/v1/me/vials/label-photo-uploads",
		Summary:     "A link to upload a photograph of a vial's label",
		Description: "Answers a short-lived link that writes exactly one object, and the " +
			"key to send back with the vial. The key is minted here and prefixed with " +
			"the caller's own identifier: a client choosing its own could write under " +
			"somebody else's prefix, and the store has no row-level security to stop it.",
		Tags: []string{"inventory"},
		Errors: []int{
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusUnprocessableEntity,
			http.StatusServiceUnavailable,
		},
	}, s.startLabelUpload)
}

func (s *Service) addVial(ctx context.Context, in *NewVialInput) (*NewVialOutput, error) {
	caller, err := s.patientCalling(ctx)
	if err != nil {
		return nil, err
	}
	if err := in.Body.check(); err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}

	out := &NewVialOutput{Status: http.StatusCreated}
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

// check refuses what the schema would refuse, so the patient is told which field rather than
// which constraint. The bounds are the schema's own: 000021 for the scale, 000024 for the
// ceiling, and both are asked here in the unit the box carries.
func (b NewVialBody) check() error {
	if _, ok := protocol.ParseDoseUnit(b.TotalAmount.Unit); !ok {
		return fmt.Errorf("total_amount is in %q: %w", b.TotalAmount.Unit, protocol.ErrUnknownDoseUnit)
	}
	unit := protocol.DoseUnit(b.TotalAmount.Unit)
	if protocol.FinerThanItsAtom(b.TotalAmount.Value, unit) {
		return fmt.Errorf("total_amount is %v %s: %w", b.TotalAmount.Value, unit, ErrAmountTooFine)
	}
	ceiling := float64(maxVialMilligrams)
	if unit == protocol.MCG {
		ceiling = maxVialMicrograms
	}
	// Negated so a NaN, which loses every comparison, is refused here rather than reaching
	// a column where Postgres orders it above every number.
	if !(b.TotalAmount.Value > 0 && b.TotalAmount.Value <= ceiling) {
		return fmt.Errorf("total_amount is %v %s: %w", b.TotalAmount.Value, unit, ErrAmountOffRange)
	}

	return nil
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
	case errors.Is(err, ErrNoSuchCompound), errors.Is(err, ErrKeyNotTheirs),
		errors.Is(err, ErrAmountTooFine), errors.Is(err, ErrAmountOffRange):
		// The schema's refusal read back as the field the caller filled in.
		return huma.Error422UnprocessableEntity(err.Error())
	case database.IsUnavailable(err):
		return huma.Error503ServiceUnavailable("the database is not answering", err)
	default:
		return huma.Error500InternalServerError("adding the vial", err)
	}
}

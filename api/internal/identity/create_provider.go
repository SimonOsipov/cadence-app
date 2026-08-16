package identity

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// CreateProviderInput is the request body of POST /v1/providers.
type CreateProviderInput struct {
	Body NewProviderBody
}

// NewProviderBody is everything the clinic states when it takes on a doctor.
//
// There is no role. This route creates one, the first administrator comes into
// being through cmd/bootstrap-admin under the migration role, and a body that
// named a role would be a body choosing what a person may do.
type NewProviderBody struct {
	Email    string `json:"email" format:"email" maxLength:"320" doc:"Where the invitation is sent. Stored folded to lower case, which is the spelling the identity provider holds."`
	FullName string `json:"full_name" minLength:"1" maxLength:"200" doc:"The doctor's name as the clinic writes it."`

	TitleRU *string `json:"title_ru,omitempty" maxLength:"200" doc:"What the clinic calls them, in Russian. Shown to a patient beside the name on their care team."`
}

// CreateProviderOutput answers with the identifier the provider assigned, which
// is also the doctor's profile id everywhere else in the API.
type CreateProviderOutput struct {
	Status int
	Body   CreatedProvider
}

// CreatedProvider is what a successful creation says.
type CreatedProvider struct {
	UserID string `json:"user_id" doc:"The doctor's id. The same value the invitation link signs them in as."`
}

// providerRole is the one role this route brings into being. It is passed rather
// than written into the statement so that the closed set stays the database's:
// see NewProvider.Role.
const providerRole = "doctor"

// The Russian an administrator reads, where the patient route's sentence would
// be wrong for them.
const (
	detailOnlyAnAdminCreatesStaff = "Заводить врачей может только администратор."
	detailStaffAlreadyThere       = "Человек с этим адресом уже заведён в клинике."
	detailNoAdminThroughTheAPI    = "Администратора нельзя завести через API: " +
		"первый создаётся разовой командой при развёртывании."

	// The patient route's sentence for this ends «обратитесь к администратору»,
	// and the only caller of this one is the administrator.
	detailStaffAccountIsNotOurs = "На этот адрес уже есть аккаунт, который клиника не приглашала. " +
		"Проверьте адрес."
)

// createProvider is the admin's half of onboarding: one request takes a doctor
// on and sends their invitation, or neither happens.
func (s *Service) createProvider(
	ctx context.Context, input *CreateProviderInput,
) (*CreateProviderOutput, error) {
	if s.onboarding == nil {
		// Same assembly, same answer as the patient route.
		return nil, huma.Error500InternalServerError("this API was assembled without an onboarding service")
	}

	userID, err := s.onboarding.InviteProvider(ctx, input.Body.Email, NewProvider{
		Role:     providerRole,
		FullName: input.Body.FullName,
		TitleRU:  input.Body.TitleRU,
	})
	if err != nil {
		return nil, refusalForProvider(err)
	}

	return &CreateProviderOutput{
		Status: http.StatusCreated,
		Body:   CreatedProvider{UserID: userID},
	}, nil
}

// refusalForProvider answers what a staff creation is refused for, and defers
// everything it shares with a patient's creation to refusalFor.
//
// An arm of its own is one whose sentence would be wrong on the other route —
// who may create, what the database says about the role, which person already
// exists, and whose account the address turns out to be. The rest are deferred.
func refusalForProvider(err error) error {
	switch {
	case errors.Is(err, ErrCallerMayNotCreateProviders):
		return huma.Error403Forbidden(detailOnlyAnAdminCreatesStaff)

	// Named rather than left to the 500 it would otherwise be. It is unreachable
	// through this route — the role above is a constant — so what the sentence
	// is for is the day it stops being unreachable: a refusal that says which
	// rule spoke is a fixable state, and an internal error is a bug hunt.
	case errors.Is(err, ErrServicePathMakesNoAdmins):
		return huma.Error403Forbidden(detailNoAdminThroughTheAPI)

	case errors.Is(err, ErrAlreadyOnboarded), errors.Is(err, ErrAlreadyExists):
		return huma.Error409Conflict(detailStaffAlreadyThere)

	case errors.Is(err, ErrAccountIsNotOurs):
		return huma.Error409Conflict(detailStaffAccountIsNotOurs)

	// Deferred one refusal at a time rather than wholesale, because refusalFor's
	// own default names the act for the log — and a failure on this route logged
	// as "creating a patient" sends the next engineer to the wrong handler.
	case errors.Is(err, ErrProvisionerUnavailable),
		errors.Is(err, database.ErrLockUnavailable),
		errors.Is(err, ErrNoAddress), errors.Is(err, ErrMalformedIdentifier):
		return refusalFor(err)

	default:
		return huma.Error500InternalServerError("creating a provider", err)
	}
}

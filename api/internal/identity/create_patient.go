package identity

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// CreatePatientInput is the request body of POST /v1/patients.
type CreatePatientInput struct {
	Body NewPatientBody
}

// NewPatientBody is everything the clinic states when it creates a patient.
//
// There is no role and no user id: the server sets the first and the identity
// provider assigns the second. There is no timezone either — the profile carries
// none until the person's own device reports one, and that capture is named in
// the spec as owned by the block that adds first-sign-in handling.
//
// The clinical fields are pointers because "not measured yet" and "measured as
// zero" are different facts about a person.
type NewPatientBody struct {
	Email    string `json:"email" format:"email" maxLength:"320" doc:"Where the invitation is sent. Stored folded to lower case, which is the spelling the identity provider holds."`
	FullName string `json:"full_name" minLength:"1" maxLength:"200" doc:"The patient's name as the clinic writes it."`

	DateOfBirth    *string  `json:"date_of_birth,omitempty" format:"date"`
	Sex            *string  `json:"sex,omitempty" enum:"male,female"`
	HeightCM       *float64 `json:"height_cm,omitempty" minimum:"0" doc:"Height in centimetres."`
	TargetWeightKG *float64 `json:"target_weight_kg,omitempty" minimum:"0" doc:"Target weight in kilograms."`

	Specialists []SpecialistBody `json:"specialists" minItems:"1" doc:"The care team. A doctor creating a patient has to be on it; an admin names whoever the clinic decided."`
}

// SpecialistBody is one specialist on the new patient's care team.
type SpecialistBody struct {
	ProviderID string `json:"provider_id" format:"uuid"`
	CareRole   string `json:"care_role" enum:"endo,dietitian,nurse"`
	Primary    bool   `json:"primary,omitempty" doc:"The one specialist who leads. At most one per patient."`
}

// CreatePatientOutput answers with the identifier the provider assigned, which
// is also the patient's profile id everywhere else in the API.
type CreatePatientOutput struct {
	Status int
	Body   CreatedPatient
}

// CreatedPatient is what a successful creation says.
type CreatedPatient struct {
	UserID string `json:"user_id" doc:"The patient's id. The same value the invitation link signs the person in as."`
}

// The Russian the doctor reads. Together rather than beside the refusals they
// describe, because what has to hold between them is that each says something
// different to the person creating the patient.
const (
	detailAlreadyOnboarded = "Пациент с этим адресом уже заведён."
	detailAccountIsNotOurs = "На этот адрес уже есть аккаунт, который клиника не приглашала. " +
		"Проверьте адрес или обратитесь к администратору."
	detailNotOnTheCareTeam = "Врач, который заводит пациента, должен быть в его команде наблюдения."
	detailMayNotCreate     = "У вас нет прав заводить пациентов."
	detailNotAProvider     = "В команде наблюдения указан человек, который не является врачом клиники."
	detailUnprocessable    = "Проверьте поля формы: данные пациента заполнены неверно."
	detailBusy             = "Сейчас заняты — повторите через минуту."
)

// createPatient is the doctor's half of onboarding: one request creates the
// person and sends the invitation, or neither happens.
func (s *Service) createPatient(ctx context.Context, input *CreatePatientInput) (*CreatePatientOutput, error) {
	if s.onboarding == nil {
		// The document generator builds this context with no dependencies, so
		// that openapi.json describes the operation without a database behind
		// it. Nothing serves from that assembly, and this says so out loud
		// rather than dereferencing nothing.
		return nil, huma.Error500InternalServerError("this API was assembled without an onboarding service")
	}

	// A conversion, which the compiler checks: the wire type stays declared
	// separately so that renaming a domain field cannot rename a JSON member for
	// the two generated clients, and the day the two shapes diverge this line
	// stops building rather than mapping the wrong field.
	specialists := make([]Assignment, 0, len(input.Body.Specialists))
	for _, named := range input.Body.Specialists {
		specialists = append(specialists, Assignment(named))
	}

	userID, err := s.onboarding.InvitePatient(ctx, input.Body.Email, NewPatient{
		FullName:       input.Body.FullName,
		DateOfBirth:    input.Body.DateOfBirth,
		Sex:            input.Body.Sex,
		HeightCM:       input.Body.HeightCM,
		TargetWeightKG: input.Body.TargetWeightKG,
		Specialists:    specialists,
	})
	if err != nil {
		return nil, refusalFor(err)
	}

	return &CreatePatientOutput{Status: http.StatusCreated, Body: CreatedPatient{UserID: userID}}, nil
}

// refusalFor maps this context's refusals onto the answers the transport gives.
//
// Everything not named here keeps its 500, which is the right default: a
// refusal this function does not recognise is a failure rather than a rule, and
// the detail of a 5xx never reaches the caller anyway.
func refusalFor(err error) error {
	switch {
	// Two ways to arrive at one answer, and the second is the one no lookup can
	// prevent: the loser of a double click is refused by the claim rule, and a
	// creation that raced past it is refused by the profiles primary key.
	case errors.Is(err, ErrAlreadyOnboarded), errors.Is(err, ErrAlreadyExists):
		return huma.Error409Conflict(detailAlreadyOnboarded)

	case errors.Is(err, ErrAccountIsNotOurs):
		return huma.Error409Conflict(detailAccountIsNotOurs)

	case errors.Is(err, ErrCallerNotOnTheCareTeam):
		return huma.Error403Forbidden(detailNotOnTheCareTeam)

	case errors.Is(err, ErrCallerMayNotCreatePatients):
		return huma.Error403Forbidden(detailMayNotCreate)

	// The provisioner is the most likely failure in production, and without this
	// arm it is a 500 with an invitation that may or may not have gone out.
	case errors.Is(err, ErrProvisionerUnavailable):
		return huma.Error503ServiceUnavailable("the provisioner did not answer", err)

	// Both of the lock's own failures, and both are ordinary rather than
	// exceptional: the loser of a double click can burn its deadline waiting on
	// the winner, and the request pool has no explicit size, so concurrent
	// creations — each holding a connection for the length of the request — make
	// the next one wait. Left to the default these answer /problems/internal in
	// English, which tells a doctor nothing and suggests a bug rather than «try
	// again».
	case errors.Is(err, database.ErrLockUnavailable):
		return huma.Error503ServiceUnavailable(detailBusy, err)

	// Its own sentence because it is the one refusal here a doctor can act on
	// without rereading the form: the identifier came from a picker, and what is
	// wrong with it is not a typo.
	case errors.Is(err, ErrNotAProvider):
		return huma.Error422UnprocessableEntity(detailNotAProvider)

	// What else a body can say that the schema cannot refuse: two leading
	// specialists, an unknown timezone, an address that folds to nothing.
	case errors.Is(err, ErrTwoPrimarySpecialists),
		errors.Is(err, ErrNoSpecialist), errors.Is(err, ErrUnknownTimezone),
		errors.Is(err, ErrMalformedIdentifier), errors.Is(err, ErrNoAddress),
		// A repeated specialist and a collided assignment are both the form's
		// problem. Answered as a conflict they would be reported as a fact about
		// the address, and the doctor — whose invitation has already gone out —
		// would have no reason to correct the form and retry.
		errors.Is(err, ErrSpecialistNamedTwice), errors.Is(err, ErrAssignmentCollided):
		return huma.Error422UnprocessableEntity(detailUnprocessable)

	default:
		return huma.Error500InternalServerError("creating a patient", err)
	}
}

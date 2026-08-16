package identity

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// Service is this context's operations together with what they need to answer.
//
// The registry in internal/router holds one entry per context, so the
// dependencies arrive here rather than at each operation: a registrar taking a
// pool per route is a route that can be given the wrong one.
type Service struct {
	onboarding *Onboarding
}

// NewService builds the context around the flow its operations serve.
//
// A nil onboarding is legitimate and has exactly one caller: the generator that
// renders openapi.json needs the operations declared and nothing behind them.
// An operation reached in that state refuses rather than dereferences.
func NewService(onboarding *Onboarding) *Service {
	return &Service{onboarding: onboarding}
}

// Register mounts this context's operations on the API.
//
// One entry per operation, in the context that owns it — the registry in
// internal/router knows the list of contexts, not the list of routes.
func (s *Service) Register(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-me",
		Method:      http.MethodGet,
		Path:        "/v1/me",
		Summary:     "The authenticated caller",
		Description: "Returns the identity carried by the presented token. " +
			"It reads no database: a role changed there takes effect when the token expires.",
		Tags: []string{"identity"},
	}, me)

	huma.Register(api, huma.Operation{
		OperationID:   "create-patient",
		Method:        http.MethodPost,
		Path:          "/v1/patients",
		DefaultStatus: http.StatusCreated,
		Summary:       "Create a patient and invite them",
		Description: "Creates the patient and sends the invitation as one action: the address is " +
			"invited at the identity provider, and the profile, the card, the care team, the " +
			"preferences and the record of the invitation are written against the identifier it " +
			"assigned. The server sets the role; there is no public registration. " +
			"Answers 409 when the address already belongs to a patient or to an account this " +
			"clinic did not invite, and 503 when the provisioner is unreachable.",
		Tags: []string{"identity"},
	}, s.createPatient)
}

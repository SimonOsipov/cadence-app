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
	sessions   *Sessions
}

// NewService builds the context around the flows its operations serve.
//
// A nil dependency is legitimate and has exactly one caller: the generator that
// renders openapi.json needs the operations declared and nothing behind them.
// An operation reached in that state refuses rather than dereferences.
func NewService(onboarding *Onboarding, sessions *Sessions) *Service {
	return &Service{onboarding: onboarding, sessions: sessions}
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
		OperationID:   "record-session",
		Method:        http.MethodPost,
		Path:          "/v1/me/session",
		DefaultStatus: http.StatusNoContent,
		Summary:       "Report the device's timezone",
		Description: "Records the caller's timezone against their profile. A patient's is stored — the schedule, " +
			"the reminders and the server-side sweep all read it — and the zone is validated there; a " +
			"doctor's and an administrator's are not stored, and the call is answered without a write " +
			"rather than refused, so one client can serve every role. " +
			"Answers 400 when a patient's zone is not one the server knows, 403 when the account has no " +
			"role yet, and 503 when the database did not answer.",
		Tags: []string{"identity"},
		// Declared rather than left to the default response: both client surfaces are generated from this
		// document, and a status that exists only in the prose above is a branch neither of them can write.
		Errors: []int{http.StatusBadRequest, http.StatusForbidden, http.StatusServiceUnavailable},
	}, s.recordSession)

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
			"Answers 403 when the caller may not create patients or is not on the care team they " +
			"wrote, 409 when the address already belongs to a patient or to an account this " +
			"clinic did not invite, 422 when the body says something the schema cannot refuse — " +
			"two leading specialists, one named twice, somebody who is not a provider — and 503 " +
			"when the provisioner is unreachable.",
		Tags: []string{"identity"},
	}, s.createPatient)

	huma.Register(api, huma.Operation{
		OperationID:   "create-provider",
		Method:        http.MethodPost,
		Path:          "/v1/providers",
		DefaultStatus: http.StatusCreated,
		Summary:       "Take a doctor on and invite them",
		Description: "Creates the doctor and sends the invitation as one action, the way a patient " +
			"is created. Administrators only, and doctors only: the clinic's first administrator " +
			"comes into being through a one-off command under the migration role, and the database " +
			"refuses any other role from this path. " +
			"Answers 403 to a caller who is not an administrator, 409 when the address already " +
			"belongs to somebody this clinic knows or to an account it did not invite, 422 when " +
			"the address folds to nothing, and 503 when the provisioner is unreachable.",
		Tags: []string{"identity"},
	}, s.createProvider)
}

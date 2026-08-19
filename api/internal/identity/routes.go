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
	roster     *Roster
	profiles   ProfileReader
	directory  *Directory
}

// Deps is what this context's operations answer from. A nil field is the document generator's
// assembly, and an operation reached in it refuses rather than dereferences.
type Deps struct {
	Onboarding *Onboarding
	Sessions   *Sessions
	Roster     *Roster
	Profiles   ProfileReader
	Directory  *Directory
}

func NewService(deps Deps) *Service {
	return &Service{
		onboarding: deps.Onboarding,
		sessions:   deps.Sessions,
		roster:     deps.Roster,
		profiles:   deps.Profiles,
		directory:  deps.Directory,
	}
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
		Description: "Returns the identity the presented token carries, and the caller's own name. " +
			"The role is the token's: changed in the database, it takes effect when the token expires. " +
			"The name is read from the caller's own profile row, because nothing else in the API " +
			"answers a person their own name — it is absent for an account the clinic holds no " +
			"profile for. Answers 503 when the database could not serve the request.",
		Tags: []string{"identity"},
		Errors: []int{
			http.StatusUnauthorized,
			http.StatusServiceUnavailable,
		},
	}, s.me)

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
			"role yet, and 503 when the database could not serve the request.",
		Tags: []string{"identity"},
		// Declaring any status makes huma drop the default response, so 401 has to be named here too.
		Errors: []int{
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusServiceUnavailable,
		},
	}, s.recordSession)

	huma.Register(api, huma.Operation{
		OperationID: "dashboard-overview",
		Method:      http.MethodGet,
		Path:        "/v1/dashboard/overview",
		Summary:     "The patients this member of staff may see",
		Description: "The doctor's registry, on the route §11 names and the one M6 extends rather than replaces. " +
			"A doctor is answered their assigned patients and an administrator everyone; the selection is the " +
			"database's, so reassigning a patient changes the answer with no query edit. Paging is by cursor " +
			"rather than by offset, because the set changes underneath a doctor as assignments do. " +
			"Answers 400 for a cursor this server did not issue, 403 to a patient, and 503 when the database " +
			"could not serve the request.",
		Tags: []string{"identity"},
		Errors: []int{
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusServiceUnavailable,
		},
	}, s.overview)

	huma.Register(api, huma.Operation{
		OperationID: "list-providers",
		Method:      http.MethodGet,
		Path:        "/v1/providers",
		Summary:     "The clinic's staff",
		Description: "Everyone a care team may name: the clinic's doctors and administrators, by name and " +
			"by the title a patient reads beside it. Staff only — a patient is answered their own care " +
			"team by the screen that draws it, not this list. " +
			"The read is authorised here rather than by a policy: no policy lets a doctor read a " +
			"colleague's profile, which is why the new-patient form had nobody to offer. " +
			"Answers 403 to a patient and 503 when the database could not serve the request.",
		Tags: []string{"identity"},
		Errors: []int{
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusServiceUnavailable,
		},
	}, s.staff)

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

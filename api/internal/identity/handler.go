package identity

import (
	"context"
	"errors"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// MeOutput is the response of GET /v1/me.
type MeOutput struct {
	Body Me
}

// Me is the caller as the API is willing to describe them.
//
// Three fields, and the reason is the token they came from: a Supabase access
// token also carries email, phone and app_metadata, and returning claims
// wholesale would put contact details into a contract two client surfaces are
// generated from — permanently, and without anyone deciding to.
//
// ExpiresAt is named for what it is. The claim behind it is `exp`, a count of
// seconds; a JSON field called `exp` holding an RFC 3339 timestamp would read
// as that claim and decode as something else.
type Me struct {
	Subject   string    `json:"sub" doc:"The caller's user id. Every access policy is keyed on it."`
	Role      string    `json:"role" doc:"The product role the token asserts, from the cadence_role claim: patient, doctor or admin. Empty when the account has no profile yet, in which case no data endpoint will answer for it."`
	ExpiresAt time.Time `json:"expires_at" doc:"When the presented token stops being accepted."`

	// The one field that is not the token's. It is read from the caller's own
	// profile row, because nothing else in the API answers a person's own name
	// and the dashboard greets them by it.
	FullName string `json:"full_name,omitempty" doc:"The caller's name as the clinic wrote it. Absent for an account the clinic holds no profile for."`
}

// me answers the token, plus the one thing the token does not carry.
//
// The 401 below is not reachable through the assembled router — the middleware
// refuses before this runs. It exists because "not reachable" is a property of
// the wiring, and a handler that answered 200 with an empty subject would be
// presenting an anonymous caller as a verified one.
func (s *Service) me(ctx context.Context, _ *struct{}) (*MeOutput, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("no verified principal on the request context")
	}

	body := Me{
		Subject:   principal.Subject,
		Role:      principal.Role,
		ExpiresAt: principal.ExpiresAt,
	}

	// A token carrying no product role is an account provisioning has not reached, so there is no
	// profile to name — and no database role to read one as: the seam refuses an unknown role, and
	// that refusal would reach the caller as a broken endpoint.
	if principal.Role == "" {
		return &MeOutput{Body: body}, nil
	}

	if s.profiles == nil {
		return nil, huma.Error500InternalServerError("this API was assembled without a profiles service")
	}

	name, err := s.profiles.NameOf(ctx, database.Caller{Subject: principal.Subject, Role: principal.Role})
	if err != nil {
		if errors.Is(err, ErrDatabaseUnavailable) {
			return nil, huma.Error503ServiceUnavailable("the database could not serve the request", err)
		}

		return nil, huma.Error500InternalServerError("reading the caller's profile", err)
	}

	body.FullName = name

	return &MeOutput{Body: body}, nil
}

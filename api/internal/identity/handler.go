package identity

import (
	"context"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
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
	Role      string    `json:"role" doc:"The role asserted by the token."`
	ExpiresAt time.Time `json:"expires_at" doc:"When the presented token stops being accepted."`
}

// me answers from the verified token alone.
//
// The 401 below is not reachable through the assembled router — the middleware
// refuses before this runs. It exists because "not reachable" is a property of
// the wiring, and a handler that answered 200 with an empty subject would be
// presenting an anonymous caller as a verified one.
func me(ctx context.Context, _ *struct{}) (*MeOutput, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("no verified principal on the request context")
	}

	return &MeOutput{Body: Me{
		Subject:   principal.Subject,
		Role:      principal.Role,
		ExpiresAt: principal.ExpiresAt,
	}}, nil
}

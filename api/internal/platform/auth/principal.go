// Package auth carries the verified identity of a caller and nothing else.
//
// It has no third-party dependencies, and that is the point rather than an
// accident: every bounded context needs auth.PrincipalFrom, so anything this
// package imports is imported by all eleven of them. Token verification lives
// in auth/token, which only the transport and the composition root import —
// which is what makes replacing the identity provider one implementation
// instead of a sweeping edit.
//
// It knows nothing about the product: no database, no profiles, no roles beyond
// the one the token asserts. The consequence, recorded rather than discovered
// later, is that a role changed in the database takes effect only when the
// caller's token expires.
package auth

import (
	"context"
	"time"
)

// Principal is the verified identity of one caller.
//
// It is deliberately three fields. A GoTrue access token also carries email,
// phone, and app_metadata; handing those through would widen the contract by
// accident and put contact details on every code path that touches a request.
// What the rest of the API needs is who is calling, in what role, and until
// when — so that is what exists.
type Principal struct {
	// Subject is the user id in the identity provider: the value every RLS
	// policy is keyed on.
	Subject string

	// Role is the product role the token asserts, from the cadence_role claim,
	// not looked up in the database. It is empty when the token carries no such
	// claim — an account with no profile yet — and the impersonation seam
	// refuses that rather than choosing a role for it.
	Role string

	// ExpiresAt is when the token stops being valid. It is exposed so a client
	// can refresh before a request fails rather than after.
	ExpiresAt time.Time
}

// principalKey is unexported and of a type declared here, so nothing outside
// this package can write a principal into a context by guessing the key.
type principalKey struct{}

// WithPrincipal returns a context carrying the verified caller.
//
// Only the middleware calls it. A handler that could construct its own
// principal could construct one it was never given.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

// PrincipalFrom returns the verified caller, if the request went through the
// authentication middleware.
//
// The boolean is not decoration: a handler mounted outside the guarded prefix
// gets false, and the difference between "anonymous" and "authenticated as the
// zero value" is the difference between a refusal and a breach.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)

	return principal, ok
}

package identity

import (
	"context"
	"time"
)

// Account is what this context is told about a person's account at the identity
// provider.
//
// Three fields, and the narrowing happened at the component that holds the admin
// key: the provider's user document also carries the confirmation token — the
// value in the invitation link, and therefore the credential itself — the
// encrypted password, and whatever a clinic wrote into the user metadata. None
// of it is on this type, so none of it can reach this context by being
// forgotten.
//
// ConfirmedAt and LastSignInAt are here because the onboarding rules rest on
// both: an invitation that was opened and an account that has signed in are
// different states of the same person.
type Account struct {
	ID           string
	ConfirmedAt  *time.Time
	LastSignInAt *time.Time
}

// Deletion is an account this context asks to have removed, with the proof that
// removing it is safe.
//
// The two conditions are stated by this context because the component that
// deletes cannot see the app schema and so cannot check them itself. That is the
// weak point of the arrangement and it is written down rather than papered over:
// what the proof buys is that deleting the wrong account has to be a lie rather
// than an omission. A struct rather than two bool arguments, because two bools
// in a row at a call site are two bools nobody can read.
type Deletion struct {
	ID            string
	InviteExists  bool
	ProfileExists bool
}

// Provisioner is the account lifecycle at the identity provider, as this context
// needs it.
//
// Declared here, by the consumer, and implemented elsewhere. That is the whole
// arrangement of the trust boundary in one sentence: the admin key belongs to a
// component of its own, this context knows only what it wants done, and nothing
// under internal/identity compiles against the thing that speaks to it —
// TestTheContextDoesNotCompileAgainstTheProvisionerClient is the witness.
//
// Four operations rather than the component's five: setting a password exists
// outside production only, for seeding, and no context asks for it.
//
// Lookup answers with no account and no error when there is none at that
// address. It is the answer that makes an address free to invite, and it is not
// a failure.
type Provisioner interface {
	Invite(ctx context.Context, email string) (Account, error)
	Lookup(ctx context.Context, email string) (*Account, error)
	LookupBatch(ctx context.Context, ids []string) ([]Account, error)
	Delete(ctx context.Context, deletion Deletion) error
}

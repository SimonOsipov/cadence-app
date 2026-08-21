package identity

import "time"

// InviteState is where a person is between the invitation and an account they
// use. Computed on read from what the identity provider states, never stored:
// a column would be a second answer to a question the provider already answers.
type InviteState string

const (
	// InviteAccepted: the person has been inside. Either instant is enough —
	// confirming the link and signing in are two ways of arriving, and a patient
	// who opened the link and never launched the app has still arrived.
	InviteAccepted InviteState = "accepted"

	// InvitePending: invited, not yet arrived, and the link still works.
	InvitePending InviteState = "pending"

	// InviteExpired: the link has run out. The clinic's move is to invite again.
	InviteExpired InviteState = "expired"

	// InviteUnknown: nobody could say. The provider did not answer, or holds no
	// account under this identifier, or states no invitation for it — three
	// different silences, and telling them apart on the screen would claim a
	// certainty none of them carries.
	InviteUnknown InviteState = "unknown"
)

// inviteStateOf reads the state from the provider's three instants.
//
// now is a parameter because the expired state is the only one that moves on
// its own, and a boundary nobody can place is a boundary nobody can test.
func inviteStateOf(account *Account, now time.Time) InviteState {
	switch {
	case account == nil:
		return InviteUnknown

	case account.ConfirmedAt != nil || account.LastSignInAt != nil:
		return InviteAccepted

	case account.InvitedAt == nil:
		// Not pending: pending is a claim that the link still works, and this is
		// an account with no invitation to measure that against.
		return InviteUnknown

	case now.Sub(*account.InvitedAt) > InviteLinkLifetime:
		return InviteExpired

	default:
		return InvitePending
	}
}

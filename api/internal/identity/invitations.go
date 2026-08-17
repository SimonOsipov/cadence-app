package identity

import (
	"strings"
	"time"
)

// InviteLinkLifetime is how long an invitation link works, and the value
// GOTRUE_MAILER_OTP_EXP is set to — the two are tied together by a test, because
// the pending state is derived from this one and the link is refused by the
// other. Chosen to survive a weekend: an invitation sent on a Friday evening is
// opened on Monday morning.
const InviteLinkLifetime = 72 * time.Hour

// NormalizeAddress folds an address to the spelling the identity provider
// stores it under.
//
// Every use of an address on this side is of the folded one: the advisory lock
// that serialises two requests for one mailbox, the row that records whom we
// invited, and the lookup that decides between inviting and claiming. The
// provider's own filter is case-sensitive, so an unfolded lookup answers "no
// such account" for an account that exists — and the invitation that follows
// rotates the token of a link somebody is holding.
//
// The provisioner folds again before it asks and cannot share this function —
// it is a package main this context may not import. Neither fold is taken on
// trust: this one is compared with the spelling the provider stores by
// TestTheProviderStoresTheAddressThisContextFolds, and the component's by its
// own TestMixedCaseFindsAnExistingAccount.
func NormalizeAddress(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

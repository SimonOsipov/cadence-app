package identity

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrTooManyInvites means one doctor has asked for more invitations inside the
// window than the clinic allows.
var ErrTooManyInvites = errors.New("too many invitations inside the window")

// InviteLinkLifetime is how long an invitation link works, and the value
// GOTRUE_MAILER_OTP_EXP is set to — the two are tied together by a test, because
// the pending state is derived from this one and the link is refused by the
// other. Chosen to survive a weekend: an invitation sent on a Friday evening is
// opened on Monday morning.
const InviteLinkLifetime = 72 * time.Hour

// What one doctor may send. Twenty is a clinic onboarding a cohort in one
// sitting and still an order of magnitude below a loop; the window slides
// rather than resetting on the hour, so a doctor who spends the allowance at
// 10:59 is not handed a second one at 11:00.
const (
	InvitesPerWindow = 20
	InviteWindow     = time.Hour
)

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

// InviteLimit bounds how many invitations one doctor may send inside a window.
//
// It exists because the provider's per-address gap does not cover the admin
// /invite — measured, not assumed — so the only limit reaching that route is a
// quota per hour for the whole instance, which cannot tell one doctor's loop
// from the rest of the clinic working. What it can do is refuse everybody at
// once, which is why the deployment's quota is required to be at least
// InvitesPerWindow: a doctor spending the allowance this grants them must not
// be stopped by the provider instead.
//
// The count is this process's. With a second replica the clinic's real bound is
// twice this one, which is the weakness of keeping it here rather than in the
// database; it is worth the absence of a write on the path that invites.
type InviteLimit struct {
	mu        sync.Mutex
	perWindow int
	window    time.Duration

	// Timestamps rather than a counter and a reset time: a counter is a fixed
	// window, and a fixed window lets twice the allowance through across the
	// moment it resets.
	sent map[string][]time.Time
}

func NewInviteLimit(perWindow int, window time.Duration) *InviteLimit {
	return &InviteLimit{
		perWindow: perWindow,
		window:    window,
		sent:      make(map[string][]time.Time),
	}
}

// Take records an invitation about to be sent, or refuses it.
//
// A refusal records nothing: an attempt that sent no email must not push the
// moment the doctor may send one again further out, or a burst against the
// limit would keep them refused for as long as they kept trying.
//
// The clock is a parameter because the only other way to test a window is to
// wait one out.
func (l *InviteLimit) Take(doctorID string, now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Rebuilt rather than filtered in place, which bounds an entry at a window's
	// worth of timestamps. The map itself is not bounded: it keeps one entry per
	// doctor who has invited since this process started, which is the clinic's
	// staff and not its patients.
	within := make([]time.Time, 0, len(l.sent[doctorID])+1)
	for _, at := range l.sent[doctorID] {
		if now.Sub(at) < l.window {
			within = append(within, at)
		}
	}

	if len(within) >= l.perWindow {
		l.sent[doctorID] = within

		return fmt.Errorf("%d in the last %s: %w", len(within), l.window, ErrTooManyInvites)
	}

	l.sent[doctorID] = append(within, now)

	return nil
}

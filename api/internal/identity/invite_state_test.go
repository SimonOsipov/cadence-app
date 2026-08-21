package identity

import (
	"testing"
	"time"
)

func at(t *testing.T, stamp string) *time.Time {
	t.Helper()

	moment, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		t.Fatalf("parsing %q: %v", stamp, err)
	}

	return &moment
}

// The four states and what each is read from. The provider is the source for
// all of them: app.invites would answer the same question and a doctor holds no
// SELECT on it.
func TestTheInviteStateOfAnAccount(t *testing.T) {
	now := *at(t, "2026-08-19T12:00:00Z")

	// Inside the link's lifetime, and outside it. Written against now rather
	// than as literals so that changing InviteLinkLifetime moves both.
	recent := now.Add(-InviteLinkLifetime / 2)
	stale := now.Add(-InviteLinkLifetime - time.Hour)

	tests := map[string]struct {
		account *Account
		want    InviteState
	}{
		"opened the link and never launched the app": {
			account: &Account{InvitedAt: &stale, ConfirmedAt: &recent},
			want:    InviteAccepted,
		},
		"signed in without a confirmation on the row": {
			account: &Account{InvitedAt: &stale, LastSignInAt: &recent},
			want:    InviteAccepted,
		},
		"invited within the link's lifetime": {
			account: &Account{InvitedAt: &recent},
			want:    InvitePending,
		},
		"invited longer ago than the link lasts": {
			account: &Account{InvitedAt: &stale},
			want:    InviteExpired,
		},
		"the provider has no account under that identifier": {
			account: nil,
			want:    InviteUnknown,
		},
		"an account the provider states no invitation for": {
			account: &Account{},
			want:    InviteUnknown,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := inviteStateOf(test.account, now); got != test.want {
				t.Errorf("state = %q, want %q", got, test.want)
			}
		})
	}
}

// The boundary itself, both sides of it: a link is good for exactly as long as
// the provider will accept it, and the two are the same constant.
func TestTheInviteExpiresWhenTheLinkDoes(t *testing.T) {
	now := *at(t, "2026-08-19T12:00:00Z")

	justInside := now.Add(-InviteLinkLifetime).Add(time.Second)
	justOutside := now.Add(-InviteLinkLifetime).Add(-time.Second)

	if got := inviteStateOf(&Account{InvitedAt: &justInside}, now); got != InvitePending {
		t.Errorf("a second before the link runs out the state is %q, want %q", got, InvitePending)
	}
	if got := inviteStateOf(&Account{InvitedAt: &justOutside}, now); got != InviteExpired {
		t.Errorf("a second after the link runs out the state is %q, want %q", got, InviteExpired)
	}
}

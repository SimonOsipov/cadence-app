package identity_test

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// The fold is what makes one address one person. Two spellings of the same
// mailbox that fold differently are two advisory locks, two invite records and,
// at the provider, one account the second request cannot find.
func TestTheAddressIsFoldedToTheSpellingTheProviderStores(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "as a human types it", raw: "Anna.Petrova@Clinic.Example", want: "anna.petrova@clinic.example"},
		{name: "pasted with the whitespace around it", raw: "  anna@clinic.example\n", want: "anna@clinic.example"},
		{name: "already folded", raw: "anna@clinic.example", want: "anna@clinic.example"},
		{name: "the domain alone in capitals", raw: "anna@CLINIC.EXAMPLE", want: "anna@clinic.example"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := identity.NormalizeAddress(tc.raw); got != tc.want {
				t.Errorf("folded %q to %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// The limit fires, and the assertion is on the refusal rather than on a count:
// a limiter that refused everybody would satisfy "the twenty-first is refused"
// and nothing else here.
func TestTheInviteLimitFires(t *testing.T) {
	const doctor = "8a1f3b7c-0000-4000-8000-000000000002"

	limit := identity.NewInviteLimit(identity.InvitesPerWindow, identity.InviteWindow)
	start := time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)

	for i := range identity.InvitesPerWindow {
		if err := limit.Take(doctor, start.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatalf("invitation %d of the %d the window allows was refused: %v",
				i+1, identity.InvitesPerWindow, err)
		}
	}

	if err := limit.Take(doctor, start.Add(time.Minute)); !errors.Is(err, identity.ErrTooManyInvites) {
		t.Errorf("the invitation past the limit answered %v, want %v",
			err, identity.ErrTooManyInvites)
	}

	// The bound is per doctor: one doctor onboarding a cohort does not stop the
	// rest of the clinic working.
	if err := limit.Take("8a1f3b7c-0000-4000-8000-000000000003", start.Add(time.Minute)); err != nil {
		t.Errorf("another doctor's first invitation was refused: %v", err)
	}

	// And it is a window rather than a lifetime allowance.
	if err := limit.Take(doctor, start.Add(identity.InviteWindow+time.Second)); err != nil {
		t.Errorf("the first invitation after the window was refused: %v", err)
	}
}

// A refused attempt does not consume the allowance a moment later.
//
// Without this, a limiter that recorded every call — the shortest thing that
// passes the test above — would keep a doctor refused for a whole window after
// one burst, however long they waited and however few they then sent.
func TestARefusedInvitationDoesNotExtendTheRefusal(t *testing.T) {
	const doctor = "8a1f3b7c-0000-4000-8000-000000000002"

	limit := identity.NewInviteLimit(1, time.Minute)
	start := time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)

	if err := limit.Take(doctor, start); err != nil {
		t.Fatalf("the first invitation was refused: %v", err)
	}

	for i := range 5 {
		if err := limit.Take(doctor, start.Add(time.Duration(i+1)*time.Second)); !errors.Is(
			err, identity.ErrTooManyInvites,
		) {
			t.Fatalf("attempt %d inside the window answered %v, want %v",
				i+2, err, identity.ErrTooManyInvites)
		}
	}

	if err := limit.Take(doctor, start.Add(time.Minute+time.Second)); err != nil {
		t.Errorf("the first invitation after the window was refused: %v — the refusals "+
			"inside it were recorded as if they had been sent", err)
	}
}

// The lifetime this context derives the pending state from is the lifetime the
// deployment gives the link. Nothing stores an invitation's status, so a reader
// computes it from invited_at plus this constant; a constant that has drifted
// from the provider's configuration would show an invitation as pending for
// three days after the link stopped working.
func TestTheDeploymentGivesLinksTheLifetimeThisContextDerivesFrom(t *testing.T) {
	set := deploymentSetting(t, testsupport.OTPExpiryVariable)

	if want := strconv.Itoa(int(identity.InviteLinkLifetime.Seconds())); set != want {
		t.Errorf("the deployment gives links %s seconds and this context derives the pending "+
			"state from %s — an invitation reads as pending after its link stopped working",
			set, want)
	}
}

// The gap between two emails to one address is set, and set to something.
//
// A gap of zero names the right variable and leaves /recover — the one mail
// route a stranger can trigger — with no gap at all, so the value is read
// rather than merely found. Which variable the provider actually reads is
// measured against a container by
// TestTheAdminInviteIsNotCoveredByThePerAddressGap; this is the cheap half.
func TestTheDeploymentSetsAGapBetweenTwoEmailsToOneAddress(t *testing.T) {
	set := deploymentSetting(t, testsupport.MailerMaxFrequencyVariable)

	gap, err := time.ParseDuration(set)
	if err != nil {
		t.Fatalf("%s is set to %q, which is not a duration: %v",
			testsupport.MailerMaxFrequencyVariable, set, err)
	}

	if gap <= 0 {
		t.Errorf("%s is set to %s, so there is no gap between two emails to one address",
			testsupport.MailerMaxFrequencyVariable, gap)
	}
}

// The provider's hourly quota is the only limit that reaches /invite, and it
// counts the whole instance. Below what one doctor is allowed here, the
// provider refuses invitations this context let through — a limit nobody
// configured refusing on behalf of one this file did.
func TestTheDeploymentAllowsAtLeastOneDoctorsWorthOfInvitations(t *testing.T) {
	set := deploymentSetting(t, testsupport.EmailsPerHourVariable)

	quota, err := strconv.Atoi(set)
	if err != nil {
		t.Fatalf("%s is set to %q, which is not a number: %v",
			testsupport.EmailsPerHourVariable, set, err)
	}

	if quota < identity.InvitesPerWindow {
		t.Errorf("the provider allows %d emails an hour for the whole clinic while one doctor "+
			"may send %d invitations in %s", quota, identity.InvitesPerWindow, identity.InviteWindow)
	}
}

// deploymentSetting is what docker-compose.yml gives an environment variable.
//
// A whole line up to the name, because a commented-out one still contains the
// text; quotes off, because YAML reads "1m" and 1m the same way and a test that
// does not would fail on a change that means nothing.
func deploymentSetting(t *testing.T, variable string) string {
	t.Helper()

	compose, err := os.ReadFile(filepath.Join(moduleRoot(t), "docker-compose.yml"))
	if err != nil {
		t.Fatalf("reading the deployment: %v", err)
	}

	for line := range strings.SplitSeq(string(compose), "\n") {
		rest, found := strings.CutPrefix(strings.TrimSpace(line), variable+":")
		if !found {
			continue
		}

		return strings.Trim(strings.TrimSpace(rest), `"'`)
	}

	t.Fatalf("no line of docker-compose.yml sets %s, so the provider runs at its own "+
		"default and nothing here knows what that is", variable)

	return ""
}

func moduleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("reading the working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found above the working directory")
		}
		dir = parent
	}
}

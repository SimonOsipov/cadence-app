package identity_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// The fold is what makes one address one person: two spellings that fold differently are two
// advisory locks, two invite records and, at the provider, one account the second request misses.
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

// Nothing stores an invitation's status, so a reader computes it from invited_at plus
// InviteLinkLifetime; a constant that has drifted from the provider's configuration would show an
// invitation as pending for days after the link stopped working.
func TestTheDeploymentGivesLinksTheLifetimeThisContextDerivesFrom(t *testing.T) {
	set := deploymentSetting(t, testsupport.OTPExpiryVariable)

	if want := strconv.Itoa(int(identity.InviteLinkLifetime.Seconds())); set != want {
		t.Errorf("the deployment gives links %s seconds and this context derives the pending "+
			"state from %s — an invitation reads as pending after its link stopped working",
			set, want)
	}
}

// The value is read rather than merely found: a gap of zero names the right variable and leaves
// /recover — the one mail route a stranger can trigger — with no gap at all. That the provider
// reads this variable at all is measured by TestTheAdminInviteIsNotCoveredByThePerAddressGap.
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

// The provider's hourly quota is the only limit that reaches /invite, and since this side enforces
// none of its own it is the only limit on invitations at all. Nothing here can say what the number
// should be — a clinic's cohort is a business decision — so what is asserted is that somebody chose
// one: a quota of zero refuses every invitation the dashboard sends.
func TestTheDeploymentBoundsInvitationsSomewhere(t *testing.T) {
	set := deploymentSetting(t, testsupport.EmailsPerHourVariable)

	quota, err := strconv.Atoi(set)
	if err != nil {
		t.Fatalf("%s is set to %q, which is not a number: %v",
			testsupport.EmailsPerHourVariable, set, err)
	}

	if quota <= 0 {
		t.Errorf("%s is set to %d, so no invitation is sent at all", testsupport.EmailsPerHourVariable, quota)
	}
}

// deploymentSetting is what docker-compose.yml gives an environment variable. A whole line up to
// the name, because a commented-out one still contains the text; quotes off, because YAML reads
// "1m" and 1m the same way and a test that did not would fail on a change that means nothing.
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

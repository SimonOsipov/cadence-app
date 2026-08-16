//go:build integration

// Where a link lands, measured against the pinned image. The allow-list is the
// one part of the mail configuration reachable without an SMTP server: the
// address is decided when the link is followed, not when it is rendered.
package identity_test

import (
	"strings"
	"testing"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// Written out rather than built from the harness's pattern: an address derived
// from the list under test would follow it however it changed.
const allowedRedirect = "http://cadence.test/welcome"

const refusedRedirect = "http://not-invited.example/welcome"

// An uncovered redirect_to is not refused: the provider answers it exactly as it
// answers an allowed one and silently substitutes SITE_URL, so a link still
// signs the person in — somewhere nobody can use. Hence the pair; neither row
// alone tells a list that governs from one that is ignored. The mutation both
// must fail against is a harness list that stops covering allowedRedirect, which
// collapses the two onto the same answer.
func TestTheAllowListDecidesWhereALinkLands(t *testing.T) {
	cycle.Reset(t)

	if set := cycle.ConfiguredWith(t, testsupport.RedirectAllowListVariable); set == "" {
		t.Fatalf("the harness configured no %s, so both cases below land on SITE_URL and "+
			"agree for the wrong reason", testsupport.RedirectAllowListVariable)
	}

	// Read off the container rather than taken from the issuer constant: the two
	// are one value in the harness and deliberately different in the deployment,
	// and it is the site URL a refused redirect falls back to.
	siteURL := cycle.ConfiguredWith(t, "GOTRUE_SITE_URL")

	tests := []struct {
		name    string
		address string
		asked   string
		want    string
	}{
		{
			name:    "an address the list covers",
			address: "allowed@clinic.example",
			asked:   allowedRedirect,
			want:    allowedRedirect,
		},
		{
			name:    "an address the list does not cover",
			address: "refused@clinic.example",
			asked:   refusedRedirect,
			want:    siteURL,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			invite(t, tc.address)

			accepted, landed := follow(t, inviteToken(t, tc.address), "invite", tc.asked)
			if !accepted {
				t.Fatalf("the link was refused, so there is no landing to read: %s", landed)
			}

			if !strings.HasPrefix(landed, tc.want) {
				t.Errorf("asking to land on %s landed on %s, want a %s address",
					tc.asked, landed, tc.want)
			}
		})
	}
}

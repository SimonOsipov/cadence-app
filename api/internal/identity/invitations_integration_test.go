//go:build integration

// What the identity provider does with an invitation, measured against the pinned image rather than
// taken from its documentation.
//
// Four behaviours the onboarding block rests on, each of them a decision somewhere else in this
// context: the address is folded because the provider stores it folded; the pending state expires
// with the link because the link expires at all; /recover is a second way in because it does not
// disturb a pending invitation; and the invitation limit is ours because the only limit the
// provider applies to /invite counts the whole clinic and cannot tell one doctor from another.
//
// Not measured here, and recorded rather than asserted: v2.194.0 has no key that disables changing
// the address through PUT /user. Nothing was found to switch off, so there is no behaviour to pin —
// a recorded address diverging from the provider's is accepted, and reconciliation belongs to
// whoever reads the state.
package identity_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// The provider holds the address folded, which is what makes the fold on this side an identity
// rather than a tidying step: the advisory lock, the invite record and the lookup all name the
// same string the provider does.
func TestTheProviderStoresTheAddressThisContextFolds(t *testing.T) {
	cycle.Reset(t)

	const typed = "Anna.Petrova@Clinic.Example"

	invite(t, typed)

	var stored string
	scanProvider(t, `SELECT email FROM auth.users`, nil, &stored)

	if want := identity.NormalizeAddress(typed); stored != want {
		t.Errorf("the provider stored %q for an invitation to %q; this context would lock on "+
			"and record %q", stored, typed, want)
	}

	// The other half: the spelling a human typed matches nothing the provider holds.
	var underTheTypedSpelling int
	scanProvider(t, `SELECT count(*) FROM auth.users WHERE email = $1`,
		[]any{typed}, &underTheTypedSpelling)

	if underTheTypedSpelling != 0 {
		t.Errorf("the provider also answers to %q; the fold would be optional and this "+
			"measurement is stale", typed)
	}
}

// The case in the middle is what stops this reading "backdating breaks a link": at half the
// lifetime the same edit leaves the link working. With the harness override gone the container
// would run at the provider's default of a day and the expired case would come out accepted.
func TestALinkOlderThanTheConfiguredLifetimeIsRefused(t *testing.T) {
	cycle.Reset(t)

	tests := []struct {
		name    string
		address string
		aged    time.Duration
		want    bool
	}{
		{name: "a link just sent", address: "fresh@clinic.example", want: true},
		{
			name: "a link halfway through its life", address: "halfway@clinic.example",
			aged: cycle.OTPExpiry / 2, want: true,
		},
		{
			name: "a link past the lifetime", address: "stale@clinic.example",
			aged: cycle.OTPExpiry + time.Minute,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			invite(t, tc.address)

			if tc.aged > 0 {
				age(t, tc.address, "confirmation_sent_at", tc.aged)
			}

			accepted, location := follow(t, inviteToken(t, tc.address), "invite", "")
			if accepted != tc.want {
				t.Errorf("the provider answered %q for a link %s old, want it %s",
					location, tc.aged, outcome(tc.want))
			}
		})
	}
}

// /recover costs the first way in nothing: on its own it neither confirms the account nor disturbs
// the invitation. Following the recovery link does confirm — and extinguishes the invitation, which
// is why a pending invite cannot be read off the provider and is recorded on our side.
func TestRecoveryLeavesThePendingInvitationAloneUntilItsLinkIsFollowed(t *testing.T) {
	cycle.Reset(t)

	const address = "recovered@clinic.example"

	invite(t, address)
	invited := inviteToken(t, address)

	askForRecovery(t, address)

	if confirmed(t, address) {
		t.Error("asking for recovery confirmed the account by itself; the account would be " +
			"claimable by anybody who knows the address")
	}

	if still := inviteToken(t, address); still != invited {
		t.Errorf("asking for recovery changed the invitation token from %q to %q — the link "+
			"in the patient's mailbox is dead", invited, still)
	}

	if accepted, location := follow(t, recoveryToken(t, address), "recovery", ""); !accepted {
		t.Fatalf("the recovery link was refused: %s", location)
	}

	if !confirmed(t, address) {
		t.Error("following the recovery link left the account unconfirmed")
	}

	// The residual property, asserted rather than read off a column: the invitation is gone, so the
	// provider cannot tell a doctor it is still pending after their patient recovered their way in.
	if accepted, _ := follow(t, invited, "invite", ""); accepted {
		t.Error("the invitation link still works after the recovery link was followed; " +
			"a pending invitation could then be read off the provider")
	}
}

// The measurement the invitation limit exists for: the provider's gap between two emails to one
// address governs /recover, which anybody can ask for, and not the admin /invite, which the clinic
// uses. Both halves, because either alone is an assumption — without the second the limit on our
// side would be belt and braces, without the first the gap would look like something nobody has
// seen fire. The gap and not every limit: the hourly quota does reach /invite and counts the whole
// instance, which is what TestTheDeploymentAllowsAtLeastOneDoctorsWorthOfInvitations compares.
func TestTheAdminInviteIsNotCoveredByThePerAddressGap(t *testing.T) {
	cycle.Reset(t)

	const address = "limited@clinic.example"

	invite(t, address)

	// Immediately, and to the same mailbox: a gap covering this route would refuse the second one.
	invite(t, address)

	if status, said := ask(t, "/recover", "", map[string]string{"email": address}); status != http.StatusOK {
		t.Fatalf("the first recovery request answered %d: %s", status, said)
	}

	status, said := ask(t, "/recover", "", map[string]string{"email": address})
	if status != http.StatusTooManyRequests {
		t.Errorf("a second recovery request inside %s answered %d, want 429: %s",
			cycle.MailerMaxFrequency, status, said)
	}

	// Which of the provider's refusals it is: the gap and the hourly quota share
	// over_email_send_rate_limit and differ only in the message — see EmailsPerHourVariable for
	// that measurement — and the quota firing here would mean this measured the wrong limit.
	if !strings.Contains(said, "you can only request this after") {
		t.Errorf("the refusal is %s, which is not the per-address gap this measures", said)
	}

	// And the gap is the configured one rather than a permanent refusal: with the override gone the
	// container would run at the provider's default of a minute and this would still be refused.
	time.Sleep(cycle.MailerMaxFrequency + time.Second)

	if status, said := ask(t, "/recover", "", map[string]string{"email": address}); status != http.StatusOK {
		t.Errorf("a recovery request after %s answered %d: %s",
			cycle.MailerMaxFrequency, status, said)
	}
}

func invite(t *testing.T, address string) {
	t.Helper()

	status, said := ask(t, "/invite", cycle.AdminToken(t), map[string]string{"email": address})
	if status != http.StatusOK {
		t.Fatalf("inviting %s answered %d: %s", address, status, said)
	}
}

func askForRecovery(t *testing.T, address string) {
	t.Helper()

	status, said := ask(t, "/recover", "", map[string]string{"email": address})
	if status != http.StatusOK {
		t.Fatalf("asking for recovery of %s answered %d: %s", address, status, said)
	}
}

// ask returns the answer rather than failing on a refusal: two tests here are about which refusal
// the provider gives, and it has more than one reason to answer 429.
func ask(t *testing.T, path, token string, payload map[string]string) (int, string) {
	t.Helper()

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encoding the request: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		cycle.GoTrue.URL+path, bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("calling %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	said, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the answer from %s: %v", path, err)
	}

	return resp.StatusCode, string(said)
}

// follow walks a link the way a mail client does and says whether the provider let it in.
//
// The answer is a redirect either way — the session arrives in the fragment and so does the refusal
// — so the status says nothing and the location is what is read. It is not followed: the address it
// points at is the answer. An empty redirectTo asks for none.
func follow(t *testing.T, token, kind, redirectTo string) (accepted bool, location string) {
	t.Helper()

	asked := cycle.GoTrue.URL + "/verify?type=" + kind + "&token=" + token
	if redirectTo != "" {
		asked += "&redirect_to=" + url.QueryEscape(redirectTo)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, asked, nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}

	client := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("following the %s link: %v", kind, err)
	}
	defer func() { _ = resp.Body.Close() }()

	said, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the answer: %v", err)
	}

	location = resp.Header.Get("Location")
	if location == "" {
		location = string(said)
	}

	return strings.Contains(location, "access_token="), location
}

func confirmed(t *testing.T, address string) bool {
	t.Helper()

	var yes bool
	scanProvider(t, `SELECT confirmed_at IS NOT NULL FROM auth.users WHERE email = $1`,
		[]any{address}, &yes)

	return yes
}

func inviteToken(t *testing.T, address string) string {
	t.Helper()

	return providerToken(t, address, `SELECT confirmation_token FROM auth.users WHERE email = $1`)
}

func recoveryToken(t *testing.T, address string) string {
	t.Helper()

	return providerToken(t, address, `SELECT recovery_token FROM auth.users WHERE email = $1`)
}

// The token is read out of the provider's own table, where a mail server would have got it. The
// alternative — picking it out of the container's log — rests on a format nobody promised.
func providerToken(t *testing.T, address, statement string) string {
	t.Helper()

	var token string
	scanProvider(t, statement, []any{address}, &token)

	if token == "" {
		t.Fatalf("the provider holds no token for %s, so what follows would measure nothing", address)
	}

	return token
}

// age moves one of the provider's timestamps back, which is how a lifetime is measured without
// waiting one out.
func age(t *testing.T, address, column string, by time.Duration) {
	t.Helper()

	conn := testsupport.Connect(t, cycle.DB.SuperuserURL)

	// The column is a constant at every call site — nothing a test received reaches the statement.
	statement := `UPDATE auth.users SET ` + column +
		` = ` + column + ` - make_interval(secs => $1) WHERE email = $2`

	tag, err := conn.Exec(t.Context(), statement, by.Seconds(), address)
	if err != nil {
		t.Fatalf("ageing %s: %v", column, err)
	}

	if tag.RowsAffected() != 1 {
		t.Fatalf("ageing %s touched %d rows, want 1", column, tag.RowsAffected())
	}
}

func scanProvider(t *testing.T, statement string, args []any, into ...any) {
	t.Helper()

	conn := testsupport.Connect(t, cycle.DB.SuperuserURL)

	if err := conn.QueryRow(t.Context(), statement, args...).Scan(into...); err != nil {
		t.Fatalf("asking the provider's own table: %v", err)
	}
}

func outcome(want bool) string {
	if want {
		return "accepted"
	}

	return "refused"
}

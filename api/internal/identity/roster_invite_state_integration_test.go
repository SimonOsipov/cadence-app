//go:build integration

package identity_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
)

// Three patients of one doctor, each left in a different state at the provider.
const (
	theAcceptedPatient = "sofia.orlova@clinic.example"
	thePendingPatient  = "darya.mints@clinic.example"
	theExpiredPatient  = "kirill.nazarov@clinic.example"
)

// statesOnTheRoster reads the route the dashboard reads and returns what it drew for each patient.
func statesOnTheRoster(t *testing.T, clinic *cycleClinic, bearer string) map[string]identity.InviteState {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/overview?limit=50", nil)
	req.Header.Set("Authorization", "Bearer "+bearer)

	rec := httptest.NewRecorder()
	clinic.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("the roster answered %d: %s", rec.Code, rec.Body)
	}

	var page struct {
		Patients []struct {
			UserID string               `json:"user_id"`
			Invite identity.InviteState `json:"invite_state"`
		} `json:"patients"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decoding the roster: %v", err)
	}

	states := make(map[string]identity.InviteState, len(page.Patients))
	for _, patient := range page.Patients {
		states[patient.UserID] = patient.Invite
	}

	return states
}

// backdateTheInvitation moves an account's invited_at into the past, at the provider's own table:
// the link's lifetime is three days and a test cannot wait them out.
func backdateTheInvitation(t *testing.T, account string) {
	t.Helper()

	conn, err := pgx.Connect(t.Context(), cycle.DB.SuperuserURL)
	if err != nil {
		t.Fatalf("connecting to the provider's own database: %v", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()

	tag, err := conn.Exec(t.Context(), `
		UPDATE auth.users SET invited_at = pg_catalog.now() - $2::interval WHERE id::text = $1
	`, account, (identity.InviteLinkLifetime + time.Hour).String())
	if err != nil {
		t.Fatalf("backdating the invitation of %s: %v", account, err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("backdating the invitation of %s touched %d rows", account, tag.RowsAffected())
	}
}

// The three states the dashboard draws, against the provider that decides them — and the one the
// source was moved for: a patient who opened their link and never launched the app is accepted,
// not expired, however long ago the invitation went out.
func TestTheRosterDrawsTheStateTheProviderReports(t *testing.T) {
	clinic := onlineClinic(t)

	admin := theFirstAdministrator(t, "Пётр Аверин")
	doctor := clinic.hire(t, admin, theDoctorsAddress, "Эндокринолог")

	// take() ends in a sign-in, which is what confirms the account.
	accepted := clinic.take(t, doctor, theAcceptedPatient)
	backdateTheInvitation(t, accepted.account)

	pending := clinic.invited(t, doctor, thePendingPatient)

	expired := clinic.invited(t, doctor, theExpiredPatient)
	backdateTheInvitation(t, expired)

	states := statesOnTheRoster(t, clinic, doctor.access)

	want := map[string]identity.InviteState{
		accepted.account: identity.InviteAccepted,
		pending:          identity.InvitePending,
		expired:          identity.InviteExpired,
	}
	for account, state := range want {
		if got := states[account]; got != state {
			t.Errorf("%s is drawn %q, want %q (whole roster: %v)", account, got, state, states)
		}
	}
}

// A provisioner that is not answering costs the states and not the roster: an empty screen is
// indistinguishable from a clinic with no patients, and one of those is a breakage.
func TestAProvisionerThatIsDownStillLeavesTheRosterDrawn(t *testing.T) {
	clinic := onlineClinic(t)

	admin := theFirstAdministrator(t, "Пётр Аверин")
	doctor := clinic.hire(t, admin, theDoctorsAddress, "Эндокринолог")
	patient := clinic.take(t, doctor, theAcceptedPatient)

	clinic.provider.unreachable = true

	states := statesOnTheRoster(t, clinic, doctor.access)

	if len(states) != 1 {
		t.Fatalf("the roster came back with %d patients, want the one that exists: %v", len(states), states)
	}
	if got := states[patient.account]; got != identity.InviteUnknown {
		t.Errorf("with the provisioner down the state is %q, want %q", got, identity.InviteUnknown)
	}
}

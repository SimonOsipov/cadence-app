//go:build integration

package identity_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

// The staff side of the same flow: an admin names a doctor, the address is
// invited, and the person is written as a doctor and as nobody's patient.
func TestAnAdminCreatesADoctorAndInvitesThem(t *testing.T) {
	clinic := onboardingStand(t)

	const address = "olga@clinic.example"

	answered := clinic.create(t, asAdmin, providersPath, providerBody(address, "Эндокринолог"))
	if answered.status != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", answered.status, answered.body)
	}

	// The identifier is the provider's, and the answer says which account the
	// person will sign in as. A locally generated one would leave the token
	// issuance hook resolving no profile at that moment.
	invited := accountID(t, address)

	var said struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal([]byte(answered.body), &said); err != nil {
		t.Fatalf("decoding the answer: %v", err)
	}

	if said.UserID != invited {
		t.Errorf("answered with %q, want the account the provider assigned, %q", said.UserID, invited)
	}

	var (
		role     string
		fullName string
		title    string
		timezone *string
	)

	clinic.scan(t, `
		SELECT p.role, p.full_name, p.timezone, coalesce(v.title_ru, '')
		FROM app.profiles p
		JOIN app.provider_profiles v ON v.user_id = p.user_id
		WHERE p.user_id = $1
	`, []any{invited}, &role, &fullName, &timezone, &title)

	if role != "doctor" {
		t.Errorf("role = %q, want doctor: this route creates one role and only one", role)
	}

	if fullName != "Ольга Тимофеева" {
		t.Errorf("full_name = %q, want the name the clinic typed", fullName)
	}

	if title != "Эндокринолог" {
		t.Errorf("title_ru = %q, want the title the clinic typed", title)
	}

	// The same absence a new patient's profile carries: nothing has reported a
	// zone until the person's own device does.
	if timezone != nil {
		t.Errorf("timezone = %q, want none until the doctor signs in", *timezone)
	}

	if sent := clinic.provider.invitations(); sent != 1 {
		t.Errorf("%d invitations went out, want 1", sent)
	}

	if !clinic.holdsInviteFor(t, invited) {
		t.Error("no record of the invitation: a retry would read the account as somebody else's")
	}
}

// Both rows a staff creation signs, and the column that separates them from a
// patient's.
//
// patient_id is what a patient's audit trail is read by, and audit_log carries no
// foreign key at all — so a doctor's identifier in that column does not fail, it
// reads as a row about a patient.
func TestTheRowsThatSignAStaffCreationNameTheAdminAndNoPatient(t *testing.T) {
	clinic := onboardingStand(t)

	const address = "sergey@clinic.example"

	answered := clinic.create(t, asAdmin, providersPath, providerBody(address, ""))
	if answered.status != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", answered.status, answered.body)
	}

	invited := accountID(t, address)

	for _, signed := range []struct {
		action string
		entity string
	}{
		{action: "provider.create", entity: "profiles"},
		{action: "invite.send", entity: "invites"},
	} {
		t.Run(signed.action, func(t *testing.T) {
			var rows int
			clinic.scan(t, `
				SELECT count(*) FROM app.audit_log
				WHERE action = $1 AND entity = $2 AND entity_id = $3
				  AND actor_id = $4 AND actor_job IS NULL AND patient_id IS NULL
			`, []any{signed.action, signed.entity, invited, theAdmin}, &rows)

			if rows != 1 {
				t.Errorf("%d rows signed by the admin against no patient, want 1", rows)
			}
		})
	}
}

// A doctor creating doctors is how a clinic acquires staff nobody hired. The
// refusal costs the address nothing: it is spoken before the provider is asked.
func TestADoctorMayNotCreateAProvider(t *testing.T) {
	clinic := onboardingStand(t)

	const address = "not.hired@clinic.example"

	answered := clinic.create(t, asDoctor, providersPath, providerBody(address, ""))
	if answered.status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", answered.status, answered.body)
	}

	if problem := problemFrom(t, answered); problem.Type != httpserver.ProblemForbidden {
		t.Errorf("type = %q, want %q", problem.Type, httpserver.ProblemForbidden)
	}

	if sent := clinic.provider.invitations(); sent != 0 {
		t.Errorf("%d invitations went out for a refused request", sent)
	}

	var invites int
	clinic.scan(t, `SELECT count(*) FROM app.invites WHERE email = $1`, []any{address}, &invites)

	if invites != 0 {
		t.Errorf("%d invite rows for a refused request: the address is now spent", invites)
	}
}

// The rule this route rests on, measured where it lives: the service path is
// refused an administrator by migration 000006's policy, not by anything in Go.
//
// Which refusal it was matters. Postgres spells a missing grant and a row a
// policy would not admit with the same 42501, and only the second is a statement
// about the role — the first would mean the arrangement is broken, and telling
// an admin their colleague is an administrator would be a lie they cannot act on.
func TestTheServicePathRefusesToWriteAnAdmin(t *testing.T) {
	clinic := onboardingStand(t)

	ctx := auth.WithPrincipal(t.Context(), asAdmin)

	err := identity.CreateProvider(ctx, clinic.writes, identity.NewProvider{
		UserID:   unhirableID,
		Role:     "admin",
		FullName: "Пётр Аверин",
	})

	if !errors.Is(err, identity.ErrServicePathMakesNoAdmins) {
		t.Fatalf("refused with %v, want %v", err, identity.ErrServicePathMakesNoAdmins)
	}

	if by := refusedBy(t, err); by != policyRefusal {
		t.Errorf("refused by %s, want %s: the role rule is the policy's and nothing else's", by, policyRefusal)
	}

	var written int
	clinic.scan(t, `SELECT count(*) FROM app.profiles WHERE user_id = $1`, []any{unhirableID}, &written)

	if written != 0 {
		t.Errorf("%d profiles written by a refused transaction", written)
	}
}

// The same path with the role this route actually passes, so that the test above
// is measuring the role and not the arrangement around it: everything else about
// the two calls is identical.
func TestTheServicePathWritesADoctor(t *testing.T) {
	clinic := onboardingStand(t)

	ctx := auth.WithPrincipal(t.Context(), asAdmin)

	if err := identity.CreateProvider(ctx, clinic.writes, identity.NewProvider{
		UserID:   unhirableID,
		Role:     "doctor",
		FullName: "Пётр Аверин",
	}); err != nil {
		t.Fatalf("writing a doctor through the service path: %v", err)
	}

	var written int
	clinic.scan(t, `SELECT count(*) FROM app.profiles WHERE user_id = $1`, []any{unhirableID}, &written)

	if written != 1 {
		t.Errorf("%d profiles written, want 1", written)
	}
}

// unhirableID is the person the two service-path tests write, or fail to. It is
// nobody the stand seeds, so the count it asserts is about this transaction.
const unhirableID = "8a1f3b7c-0000-4000-8000-000000000031"

// A staff creation interrupted after the invitation is curable by a retry, and
// the retry is measured after the invitee has opened the link.
//
// This is the ordering the route rests on and the reason the invitation is
// committed in a transaction of its own: with the person's transaction first,
// a failure inside it leaves an account nobody has a record of, the next request
// reads it as somebody else's, and the address answers 409 for good.
//
// The interruption is the role rule itself, driven through the flow rather than
// through the handler — the handler passes a constant. It is the one failure of
// the second transaction this suite can arrange without breaking the pool the
// first one commits on.
func TestAStaffCreationInterruptedAfterTheInvitationIsCuredByARetry(t *testing.T) {
	clinic := onboardingStand(t)

	const address = "interrupted.doctor@clinic.example"

	ctx := auth.WithPrincipal(t.Context(), asAdmin)

	_, err := identity.NewOnboarding(clinic.requests, clinic.writes, clinic.provider).
		InviteProvider(ctx, address, identity.NewProvider{Role: "admin", FullName: "Пётр Аверин"})

	if !errors.Is(err, identity.ErrServicePathMakesNoAdmins) {
		t.Fatalf("the interrupted creation failed with %v, want %v",
			err, identity.ErrServicePathMakesNoAdmins)
	}

	// What the interruption left: an account, our record of it, and no person.
	invited := accountID(t, address)

	if !clinic.holdsInviteFor(t, invited) {
		t.Fatal("the invitation was not recorded, so the retry below has nothing to recognise")
	}

	var written int
	clinic.scan(t, `SELECT count(*) FROM app.profiles WHERE user_id = $1`, []any{invited}, &written)

	if written != 0 {
		t.Fatalf("%d profiles survived the refused transaction, want 0", written)
	}

	// The invitee opens the link before the retry, which is what makes the cure
	// go through the claim that deletes: a password can be set from that session.
	if accepted, location := follow(t, inviteToken(t, address), "invite", ""); !accepted {
		t.Fatalf("the invitation link was refused: %s", location)
	}

	if cured := clinic.create(t, asAdmin, providersPath, providerBody(address, "")); cured.status !=
		http.StatusCreated {
		t.Fatalf("the retry answered %d, want 201: %s", cured.status, cured.body)
	}

	if deleted := clinic.provider.deletions(); deleted != 1 {
		t.Errorf("the claim deleted %d accounts, want 1: an account somebody has been inside "+
			"can have a password set on it", deleted)
	}

	// The deletion is the second of the two rows the shared spine signs, and the
	// only place it is signed for staff. Measured here because nothing else
	// reaches it: without this the patient/staff distinction is pinned at one of
	// its two call sites.
	var signed int
	clinic.scan(t, `
		SELECT count(*) FROM app.audit_log
		WHERE action = 'account.delete' AND entity_id = $1 AND patient_id IS NULL
	`, []any{invited}, &signed)

	if signed != 1 {
		t.Errorf("%d account.delete rows name the removed account and no patient, want 1", signed)
	}

	claimed := accountID(t, address)
	if claimed == invited {
		t.Error("the account claimed is the one that was opened, so nothing was deleted")
	}

	clinic.scan(t, `SELECT count(*) FROM app.profiles WHERE user_id = $1`, []any{claimed}, &written)

	if written != 1 {
		t.Errorf("%d profiles exist after the cure, want 1", written)
	}
}

// providerBody is the request the dashboard sends: an address, a name, and what
// the clinic calls them. An empty title is a title the clinic did not state.
func providerBody(address, title string) string {
	if title == "" {
		return fmt.Sprintf(`{"email":%q,"full_name":"Ольга Тимофеева"}`, address)
	}

	return fmt.Sprintf(`{"email":%q,"full_name":"Ольга Тимофеева","title_ru":%q}`, address, title)
}

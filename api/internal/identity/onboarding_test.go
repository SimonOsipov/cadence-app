package identity_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

// Asserted here rather than only through the endpoint because the states are cheap to write down and expensive to
// arrange: an account signed in but not confirmed takes a provider and a mailbox to produce, and one line here.
func TestTheClaimRuleReadsTheAccountAndWhatThisClinicHolds(t *testing.T) {
	opened := time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		account identity.Account
		held    identity.Held
		want    identity.Claim
	}{
		{
			name: "invited, untouched, ours",
			held: identity.Held{Invite: true},
			want: identity.ClaimByInviting,
		},
		{
			name:    "confirmed",
			account: identity.Account{ConfirmedAt: &opened},
			held:    identity.Held{Invite: true},
			want:    identity.ClaimByDeletingFirst,
		},
		{
			// Either is enough: a person who signed in can have set a password from that session.
			name:    "signed in",
			account: identity.Account{LastSignInAt: &opened},
			held:    identity.Held{Invite: true},
			want:    identity.ClaimByDeletingFirst,
		},
		{
			name: "already a patient",
			held: identity.Held{Invite: true, Profile: true},
			want: identity.RefuseAlreadyOnboarded,
		},
		{
			// The profile settles it before the account's state is read: the loser of a double click must
			// not invite, and the winner's patient may well have opened the link by then.
			name:    "already a patient, and they have opened the link",
			account: identity.Account{ConfirmedAt: &opened},
			held:    identity.Held{Invite: true, Profile: true},
			want:    identity.RefuseAlreadyOnboarded,
		},
		{
			name: "no record of ours",
			want: identity.RefuseNotOurs,
		},
		{
			// A person the clinic has a profile for and no invitation of — the first administrator is
			// exactly this — is not somebody else's account to refuse, but a person who already exists.
			name: "a profile with no invitation",
			held: identity.Held{Profile: true},
			want: identity.RefuseAlreadyOnboarded,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := identity.ClaimFor(tc.account, tc.held); got != tc.want {
				t.Errorf("ClaimFor = %d, want %d", got, tc.want)
			}
		})
	}
}

// The refusal costs the patient nothing: no address is invited, so the mailbox stays free for the request that
// gets it right.
func TestADoctorWhoIsNotOnTheCareTeamInvitesNobody(t *testing.T) {
	provider := &countingProvisioner{}

	onboarding := identity.NewOnboarding(nil, nil, provider)

	_, err := onboarding.InvitePatient(
		asPrincipal(t, auth.Principal{Subject: theDoctor, Role: "doctor"}),
		"anna@clinic.example",
		identity.NewPatient{
			FullName:    "Анна Петрова",
			Specialists: []identity.Assignment{{ProviderID: theColleague, CareRole: "endo"}},
		},
	)

	if !errors.Is(err, identity.ErrCallerNotOnTheCareTeam) {
		t.Fatalf("creating a patient off the doctor's own team: %v, want %v",
			err, identity.ErrCallerNotOnTheCareTeam)
	}

	if provider.calls != 0 {
		t.Errorf("the provisioner was asked %d things by a refused request", provider.calls)
	}
}

// Everything a request can be refused for before the provider is asked, asserted together with the fact that it was
// not asked. The order is the property: the address is the one resource a failed attempt can burn.
func TestTheRefusalsThatHappenBeforeAnythingIsInvited(t *testing.T) {
	tests := []struct {
		name      string
		principal *auth.Principal
		email     string
		want      error
	}{
		{
			name:      "a patient's token",
			principal: &auth.Principal{Subject: thePatient, Role: "patient"},
			email:     "anna@clinic.example",
			want:      identity.ErrCallerMayNotCreatePatients,
		},
		{
			// Never a 5xx: a token with no product role is a person with no profile, which is an ordinary
			// state between an invitation and its acceptance.
			name:      "a token carrying no product role",
			principal: &auth.Principal{Subject: thePatient},
			email:     "anna@clinic.example",
			want:      identity.ErrCallerMayNotCreatePatients,
		},
		{
			name:  "no verified caller at all",
			email: "anna@clinic.example",
			want:  identity.ErrCallerMayNotCreatePatients,
		},
		{
			name:      "an address that folds to nothing",
			principal: &auth.Principal{Subject: theDoctor, Role: "doctor"},
			email:     "   ",
			want:      identity.ErrNoAddress,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := &countingProvisioner{}

			ctx := t.Context()
			if tc.principal != nil {
				ctx = asPrincipal(t, *tc.principal)
			}

			_, err := identity.NewOnboarding(nil, nil, provider).InvitePatient(ctx, tc.email,
				identity.NewPatient{
					FullName:    "Анна Петрова",
					Specialists: []identity.Assignment{{ProviderID: theDoctor, CareRole: "endo"}},
				})

			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}

			if provider.calls != 0 {
				t.Errorf("the provisioner was asked %d things by a refused request", provider.calls)
			}
		})
	}
}

// Asserted over the wire rather than on the error, because what a doctor reads is the problem document and not the
// Go value behind it.
func TestARefusedCreationAnswersInRussianWithItsOwnProblemType(t *testing.T) {
	tests := []struct {
		name       string
		principal  auth.Principal
		wantStatus int
		wantType   string
		wantDetail string
	}{
		{
			name:       "a doctor off the team",
			principal:  auth.Principal{Subject: theDoctor, Role: "doctor"},
			wantStatus: http.StatusForbidden,
			wantType:   httpserver.ProblemForbidden,
			wantDetail: "команде наблюдения",
		},
		{
			name:       "a patient",
			principal:  auth.Principal{Subject: thePatient, Role: "patient"},
			wantStatus: http.StatusForbidden,
			wantType:   httpserver.ProblemForbidden,
			wantDetail: "нет прав",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"email":"anna@clinic.example","full_name":"Анна Петрова",` +
				`"specialists":[{"provider_id":"` + theColleague + `","care_role":"endo"}]}`

			answer := post(t, identity.NewOnboarding(nil, nil, &countingProvisioner{}),
				tc.principal, patientsPath, body)

			if answer.status != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body %s", answer.status, tc.wantStatus, answer.body)
			}

			var problem httpserver.Problem
			if err := json.Unmarshal([]byte(answer.body), &problem); err != nil {
				t.Fatalf("decoding the problem document: %v", err)
			}

			if problem.Type != tc.wantType {
				t.Errorf("type = %q, want %q", problem.Type, tc.wantType)
			}

			if !strings.Contains(problem.Detail, tc.wantDetail) {
				t.Errorf("detail = %q, want it to mention %q", problem.Detail, tc.wantDetail)
			}
		})
	}
}

// A `role` in the body is the one field that must not be quietly ignored: the server sets the role, and a request
// that tried to choose one has to be told it was not read.
//
// Measured against huma v2.39 on 2026-08-16: an unexpected property is a 422 rather than the 400 the spec's
// criterion names — the schema huma generates carries additionalProperties: false, and its validator answers 422 for
// every violation of it. A malformed body is the 400; neither is the 403 the criterion was written to rule out.
func TestABodyThatChoosesARoleIsRefusedRatherThanIgnored(t *testing.T) {
	body := `{"email":"anna@clinic.example","full_name":"Анна Петрова","role":"admin",` +
		`"specialists":[{"provider_id":"` + theDoctor + `","care_role":"endo"}]}`

	answer := post(t, identity.NewOnboarding(nil, nil, &countingProvisioner{}),
		auth.Principal{Subject: theDoctor, Role: "doctor"}, patientsPath, body)

	if answer.status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %s", answer.status, answer.body)
	}

	var problem httpserver.Problem
	if err := json.Unmarshal([]byte(answer.body), &problem); err != nil {
		t.Fatalf("decoding the problem document: %v", err)
	}

	if problem.Type != httpserver.ProblemValidation {
		t.Errorf("type = %q, want %q", problem.Type, httpserver.ProblemValidation)
	}

	if len(problem.Errors) == 0 || !strings.Contains(problem.Errors[0].Location, "role") {
		t.Errorf("the refusal does not name the field it refused: %+v", problem.Errors)
	}
}

// Distinct people, so that "the doctor assigned somebody else" is a different value rather than a reading of one.
const (
	theDoctor    = "8a1f3b7c-0000-4000-8000-000000000002"
	theColleague = "8a1f3b7c-0000-4000-8000-000000000004"
	thePatient   = "8a1f3b7c-0000-4000-8000-000000000001"
	theAdmin     = "8a1f3b7c-0000-4000-8000-000000000003"
)

// Named rather than spelled at every call site, so a body meant for one route cannot quietly be posted to the other.
const (
	patientsPath  = "/v1/patients"
	providersPath = "/v1/providers"
)

func asPrincipal(t *testing.T, principal auth.Principal) context.Context {
	t.Helper()

	return auth.WithPrincipal(t.Context(), principal)
}

// countingProvisioner answers nothing and counts being asked. Every test using it is about a request that must not
// reach the provider, so being called at all is the failure.
type countingProvisioner struct {
	calls int
}

func (p *countingProvisioner) Invite(context.Context, string) (identity.Account, error) {
	p.calls++

	return identity.Account{}, errors.New("this provisioner invites nobody")
}

func (p *countingProvisioner) Lookup(context.Context, string) (*identity.Account, error) {
	p.calls++

	return nil, errors.New("this provisioner looks nobody up")
}

func (p *countingProvisioner) LookupBatch(context.Context, []string) ([]identity.Account, error) {
	p.calls++

	return nil, errors.New("this provisioner looks nobody up")
}

func (p *countingProvisioner) Delete(context.Context, identity.Deletion) error {
	p.calls++

	return errors.New("this provisioner deletes nobody")
}

type answer struct {
	status int
	body   string
}

// post drives the operation through the transport that serves it: huma's validation, the problem documents and the
// status codes are the thing under test, and a direct call to the handler would skip all three.
func post(
	t *testing.T, onboarding *identity.Onboarding, principal auth.Principal, path, body string,
) answer {
	t.Helper()

	mux := chi.NewRouter()
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), principal)))
		})
	})

	identity.NewService(onboarding).Register(httpserver.NewAPI(mux))

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return answer{status: rec.Code, body: rec.Body.String()}
}

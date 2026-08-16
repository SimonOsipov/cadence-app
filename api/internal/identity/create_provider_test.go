package identity_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

// Everything a staff creation is refused for before the provider is asked
// anything, asserted together with the fact that it was not asked. The address
// is the resource a failed attempt burns, and only an admin may spend one here.
func TestTheStaffRefusalsThatHappenBeforeAnythingIsInvited(t *testing.T) {
	tests := []struct {
		name      string
		principal *auth.Principal
		email     string
		want      error
	}{
		{
			// The whole authorization rule of this route: a doctor creating
			// doctors is how a clinic grows a second clinic nobody decided on.
			name:      "a doctor's token",
			principal: &auth.Principal{Subject: theDoctor, Role: "doctor"},
			email:     "olga@clinic.example",
			want:      identity.ErrCallerMayNotCreateProviders,
		},
		{
			name:      "a patient's token",
			principal: &auth.Principal{Subject: thePatient, Role: "patient"},
			email:     "olga@clinic.example",
			want:      identity.ErrCallerMayNotCreateProviders,
		},
		{
			// Never a 5xx: a token with no product role is a person with no
			// profile, which is the ordinary state between an invitation and
			// its acceptance.
			name:      "a token carrying no product role",
			principal: &auth.Principal{Subject: thePatient},
			email:     "olga@clinic.example",
			want:      identity.ErrCallerMayNotCreateProviders,
		},
		{
			name:  "no verified caller at all",
			email: "olga@clinic.example",
			want:  identity.ErrCallerMayNotCreateProviders,
		},
		{
			name:      "an address that folds to nothing",
			principal: &auth.Principal{Subject: theAdmin, Role: "admin"},
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

			_, err := identity.NewOnboarding(nil, nil, provider).InviteProvider(ctx, tc.email,
				identity.NewProvider{FullName: "Ольга Тимофеева"})

			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}

			if provider.calls != 0 {
				t.Errorf("the provisioner was asked %d things by a refused request", provider.calls)
			}
		})
	}
}

// What an admin's colleague reads when they try this from the dashboard: their
// own language, and the type a client branches on rather than a 500.
func TestARefusedStaffCreationAnswersInRussianWithItsOwnProblemType(t *testing.T) {
	answered := post(t, identity.NewOnboarding(nil, nil, &countingProvisioner{}),
		auth.Principal{Subject: theDoctor, Role: "doctor"}, providersPath, staffBody())

	if answered.status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body %s", answered.status, answered.body)
	}

	var problem httpserver.Problem
	if err := json.Unmarshal([]byte(answered.body), &problem); err != nil {
		t.Fatalf("decoding the problem document: %v", err)
	}

	if problem.Type != httpserver.ProblemForbidden {
		t.Errorf("type = %q, want %q", problem.Type, httpserver.ProblemForbidden)
	}

	if !strings.Contains(problem.Detail, "администратор") {
		t.Errorf("detail = %q, which does not say who may create a doctor", problem.Detail)
	}
}

// The body says who the person is and not what they may do. A `role` in it is
// the field that must not be quietly ignored: this route creates doctors, the
// first administrator comes into being through a one-off command, and a request
// that tried to choose has to be told it was not read.
func TestABodyThatChoosesAStaffRoleIsRefusedRatherThanIgnored(t *testing.T) {
	body := `{"email":"olga@clinic.example","full_name":"Ольга Тимофеева","role":"admin"}`

	answered := post(t, identity.NewOnboarding(nil, nil, &countingProvisioner{}),
		auth.Principal{Subject: theAdmin, Role: "admin"}, providersPath, body)

	if answered.status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %s", answered.status, answered.body)
	}

	var problem httpserver.Problem
	if err := json.Unmarshal([]byte(answered.body), &problem); err != nil {
		t.Fatalf("decoding the problem document: %v", err)
	}

	if problem.Type != httpserver.ProblemValidation {
		t.Errorf("type = %q, want %q", problem.Type, httpserver.ProblemValidation)
	}

	if len(problem.Errors) == 0 || !strings.Contains(problem.Errors[0].Location, "role") {
		t.Errorf("the refusal does not name the field it refused: %+v", problem.Errors)
	}
}

func staffBody() string {
	return `{"email":"olga@clinic.example","full_name":"Ольга Тимофеева"}`
}

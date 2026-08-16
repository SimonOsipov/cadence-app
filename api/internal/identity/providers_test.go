package identity

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// Which 42501 the database spoke, decided on the message because Postgres gives all of them one SQLSTATE.
//
// The wrong reading in either direction costs something real: a missing grant read as the role rule tells an admin
// that the person they typed is an administrator, which is false and unfixable from the form; the role rule read as
// a failure is the 500 this step exists to remove.
//
// The messages are the ones Postgres 17 writes, taken from a live refusal by
// TestTheServicePathRefusesToWriteAnAdmin rather than from the documentation.
func TestWhichRefusalTheServicePathHeard(t *testing.T) {
	tests := []struct {
		name    string
		refusal *pgconn.PgError
		want    error
	}{
		{
			name: "a policy that would not admit the row",
			refusal: &pgconn.PgError{
				Code:    insufficientPrivilege,
				Message: `new row violates row-level security policy for table "profiles"`,
			},
			want: ErrServicePathMakesNoAdmins,
		},
		{
			// The service path holds INSERT on profiles today, so this is a grant revoked or a column added
			// without one — a failure of the arrangement, and nothing an admin can act on.
			name: "a grant the service path does not hold",
			refusal: &pgconn.PgError{
				Code:    insufficientPrivilege,
				Message: "permission denied for table profiles",
			},
		},
		{
			// The audit row's actor is checked by a policy too, and that refusal is also 42501 on this
			// transaction's path. It says nothing about which role was written.
			name: "a policy on another table of the same transaction",
			refusal: &pgconn.PgError{
				Code:    insufficientPrivilege,
				Message: `new row violates row-level security policy for table "audit_log"`,
			},
		},
		{
			// The third cause, and the only one the table alone does not separate: this message names
			// profiles too. profiles already carries a column-list grant, so narrowing the service path's
			// INSERT to one, or adding a column and forgetting it, is an ordinary change.
			name: "a grant on one column the service path does not hold",
			refusal: &pgconn.PgError{
				Code:    insufficientPrivilege,
				Message: `permission denied for column "role" of relation "profiles"`,
			},
		},
		{
			name: "the profile already exists",
			refusal: &pgconn.PgError{
				Code:           uniqueViolation,
				ConstraintName: "profiles_pkey",
			},
			want: ErrAlreadyExists,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyStaff(tc.refusal)

			if !errors.Is(got, tc.refusal) {
				t.Fatalf("classified as %v, which no longer carries the database's own error", got)
			}

			// Asserted as an absence, and it has to be: the database's error stays in the chain whichever
			// way this goes, so "the refusal is still reachable" is true of a misreading too.
			if tc.want == nil {
				for _, named := range []error{ErrServicePathMakesNoAdmins, ErrAlreadyExists} {
					if errors.Is(got, named) {
						t.Fatalf("read as %v, want a refusal this package does not name", named)
					}
				}

				return
			}

			if !errors.Is(got, tc.want) {
				t.Fatalf("classified as %v, want %v", got, tc.want)
			}
		})
	}
}

// Every refusal this route can answer, and the answer each gets.
//
// The two the patient route does not have are the point of the table, but the four it defers are in it for a reason
// of their own: they are deferred one at a time, and a `case` deleted from that list falls to the new default — the
// unreachable provisioner, the most likely production failure, answering 500 where the operation promises 503.
func TestWhatAStaffCreationIsRefusedWith(t *testing.T) {
	tests := []struct {
		name       string
		refusal    error
		wantStatus int
		wantDetail string

		// The patient route's answer to the same refusal. An expectation derived from the constant under test
		// moves with it — editing the staff sentence back to the patient's would keep wantDetail green — so
		// the reason the constant exists is asserted as a difference from the sentence it replaced.
		notTheSentence string

		// The same property where there is no sentence to differ from: a substring the answer is worthless
		// without.
		mustMention string

		// What the inequality above cannot catch: the likely drift is a near copy of the patient's sentence,
		// unequal to the constant while saying the same wrong thing to the same reader.
		mustNotMention string
	}{
		{
			name:       "a caller who is not an administrator",
			refusal:    ErrCallerMayNotCreateProviders,
			wantStatus: http.StatusForbidden,
			wantDetail: detailOnlyAnAdminCreatesStaff,
		},
		{
			// Its own sentence rather than the patient's: what already exists is a person, and calling them
			// a patient is the one thing this answer must not do.
			name:           "an address this clinic already knows",
			refusal:        ErrAlreadyOnboarded,
			wantStatus:     http.StatusConflict,
			wantDetail:     detailStaffAlreadyThere,
			notTheSentence: detailAlreadyOnboarded,
			mustNotMention: "ациент",
		},
		{
			// The patient route's sentence sends the reader to the administrator, the only caller here.
			name:           "an account this clinic did not invite",
			refusal:        ErrAccountIsNotOurs,
			wantStatus:     http.StatusConflict,
			wantDetail:     detailStaffAccountIsNotOurs,
			notTheSentence: detailAccountIsNotOurs,
			mustNotMention: "обратитесь",
		},
		{
			name:       "the provisioner did not answer",
			refusal:    ErrProvisionerUnavailable,
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "the lock could not be taken",
			refusal:    database.ErrLockUnavailable,
			wantStatus: http.StatusServiceUnavailable,
			wantDetail: detailBusy,
		},
		{
			name:       "an address that folds to nothing",
			refusal:    ErrNoAddress,
			wantStatus: http.StatusUnprocessableEntity,
			wantDetail: detailUnprocessable,
		},
		{
			name:       "the provider named an account this side cannot parse",
			refusal:    ErrMalformedIdentifier,
			wantStatus: http.StatusUnprocessableEntity,
			wantDetail: detailUnprocessable,
		},
		{
			// Nothing this route can produce — the role it passes is a constant — and answered anyway; see
			// the arm in refusalForProvider.
			name:       "the database refusing the role",
			refusal:    ErrServicePathMakesNoAdmins,
			wantStatus: http.StatusForbidden,
			wantDetail: detailNoAdminThroughTheAPI,

			// No patient-route sentence to differ from, so the property is stated directly: what makes this
			// clear rather than a 500 is that it names what does create an administrator.
			mustMention: "команд",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			answered := answerFor(t, refusalForProvider(tc.refusal))

			if answered.Status != tc.wantStatus {
				t.Errorf("status = %d, want %d: %s", answered.Status, tc.wantStatus, answered.Detail)
			}

			// The provisioner's arm is the one with no constant to name: it is an inline literal in
			// refusalFor, and what a caller reads there is httpserver's own sentence anyway.
			if tc.wantDetail != "" && answered.Detail != tc.wantDetail {
				t.Errorf("detail = %q, want %q", answered.Detail, tc.wantDetail)
			}

			if tc.notTheSentence != "" && answered.Detail == tc.notTheSentence {
				t.Errorf("detail = %q, which is the patient route's sentence", answered.Detail)
			}

			if tc.mustMention != "" && !strings.Contains(answered.Detail, tc.mustMention) {
				t.Errorf("detail = %q, which does not mention %q", answered.Detail, tc.mustMention)
			}

			if tc.mustNotMention != "" && strings.Contains(answered.Detail, tc.mustNotMention) {
				t.Errorf("detail = %q, which says %q to an administrator", answered.Detail, tc.mustNotMention)
			}
		})
	}
}

func answerFor(t *testing.T, refused error) *huma.ErrorModel {
	t.Helper()

	answered := &huma.ErrorModel{}
	if !errors.As(refused, &answered) {
		t.Fatalf("refused with %T, which the transport has no answer for", refused)
	}

	return answered
}

package identity

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

// Which 23505 the database spoke, and what each one is a fact about.
//
// Two of the three are facts about the form, and answering them as «this address is already taken» is what makes them
// unfixable: the invitation has gone out by the time this runs, and a doctor told the patient exists has no reason to
// correct the care team and retry. The checks above the lock refuse both before anything is sent — these arms are the
// answer when one of those checks is removed or a concurrent creation gets past it.
func TestWhichRefusalTheCreationHeard(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		want       error
	}{
		{name: "the patient already exists", constraint: "profiles_pkey", want: ErrAlreadyExists},
		{
			name:       "one specialist named twice",
			constraint: careTeamPairKey,
			want:       ErrAssignmentCollided,
		},
		{
			name:       "two specialists leading",
			constraint: careTeamPrimaryKey,
			want:       ErrTwoPrimarySpecialists,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			refusal := &pgconn.PgError{Code: uniqueViolation, ConstraintName: tc.constraint}

			got := classify(refusal)

			if !errors.Is(got, tc.want) {
				t.Errorf("classified as %v, want %v", got, tc.want)
			}

			// The database's own error stays reachable: without it a caller has the sentence and no error to
			// inspect, and the two classifiers answered differently on exactly this.
			if !errors.Is(got, refusal) {
				t.Errorf("classified as %v, which no longer carries the database's own error", got)
			}
		})
	}
}

// Anything that is not a UNIQUE violation keeps its own error: an unrecognised refusal is a failure rather than a rule,
// and a 500 is the honest answer to it.
func TestARefusalTheCreationDoesNotRecogniseIsLeftAlone(t *testing.T) {
	refusal := &pgconn.PgError{Code: "23503", ConstraintName: "invites_invited_by_fkey"}

	got := classify(refusal)

	for _, named := range []error{ErrAlreadyExists, ErrAssignmentCollided, ErrTwoPrimarySpecialists} {
		if errors.Is(got, named) {
			t.Fatalf("read as %v, want a refusal this package does not name", named)
		}
	}
}

// Every refusal this route can answer, and the document each one becomes.
//
// The staff route has had this table since it was written; the patient route had two tests reaching a 422 and
// asserting the status alone, which is why the one refusal here with a sentence of its own could be replaced by the
// generic one without anything failing.
func TestWhatAPatientCreationIsRefusedWith(t *testing.T) {
	tests := []struct {
		name       string
		refusal    error
		wantStatus int
		wantType   string
		wantDetail string

		// The likely drift is towards detailUnprocessable, which is unequal to the constant while saying nothing
		// the doctor can act on. Stated as a difference because an expectation derived from the constant under
		// test would move with it.
		notTheSentence string
	}{
		{
			name:       "the address already belongs to a patient",
			refusal:    ErrAlreadyOnboarded,
			wantStatus: http.StatusConflict,
			wantType:   httpserver.ProblemConflict,
			wantDetail: detailAlreadyOnboarded,
		},
		{
			// The creation that raced past the claim rule and was refused by the profiles primary key: the same
			// answer, because what the doctor has to do about it is the same.
			name:       "the profile was written by somebody who got there first",
			refusal:    ErrAlreadyExists,
			wantStatus: http.StatusConflict,
			wantType:   httpserver.ProblemConflict,
			wantDetail: detailAlreadyOnboarded,
		},
		{
			name:       "an account this clinic did not invite",
			refusal:    ErrAccountIsNotOurs,
			wantStatus: http.StatusConflict,
			wantType:   httpserver.ProblemConflict,
			wantDetail: detailAccountIsNotOurs,
		},
		{
			name:       "a doctor off the care team they are writing",
			refusal:    ErrCallerNotOnTheCareTeam,
			wantStatus: http.StatusForbidden,
			wantType:   httpserver.ProblemForbidden,
			wantDetail: detailNotOnTheCareTeam,
		},
		{
			name:       "a caller whose role creates nobody",
			refusal:    ErrCallerMayNotCreatePatients,
			wantStatus: http.StatusForbidden,
			wantType:   httpserver.ProblemForbidden,
			wantDetail: detailMayNotCreate,
		},
		{
			name:       "the provisioner did not answer",
			refusal:    ErrProvisionerUnavailable,
			wantStatus: http.StatusServiceUnavailable,
			wantType:   httpserver.ProblemUnavailable,
			wantDetail: detailUnavailableOnTheWire,
		},
		{
			name:       "the lock could not be taken",
			refusal:    database.ErrLockUnavailable,
			wantStatus: http.StatusServiceUnavailable,
			wantType:   httpserver.ProblemUnavailable,
			wantDetail: detailUnavailableOnTheWire,
		},
		{
			// The one 422 the spec gave a sentence of its own: the identifier came out of a picker, so
			// «check the form» sends the doctor to reread something that is spelled correctly.
			name:           "a specialist who is not a provider of this clinic",
			refusal:        ErrNotAProvider,
			wantStatus:     http.StatusUnprocessableEntity,
			wantType:       httpserver.ProblemValidation,
			wantDetail:     detailNotAProvider,
			notTheSentence: detailUnprocessable,
		},
		{
			name:       "two leading specialists",
			refusal:    ErrTwoPrimarySpecialists,
			wantStatus: http.StatusUnprocessableEntity,
			wantType:   httpserver.ProblemValidation,
			wantDetail: detailUnprocessable,
		},
		{
			name:       "a care team with nobody on it",
			refusal:    ErrNoSpecialist,
			wantStatus: http.StatusUnprocessableEntity,
			wantType:   httpserver.ProblemValidation,
			wantDetail: detailUnprocessable,
		},
		{
			name:       "a timezone this server does not know",
			refusal:    ErrUnknownTimezone,
			wantStatus: http.StatusUnprocessableEntity,
			wantType:   httpserver.ProblemValidation,
			wantDetail: detailUnprocessable,
		},
		{
			name:       "an identifier this side cannot parse",
			refusal:    ErrMalformedIdentifier,
			wantStatus: http.StatusUnprocessableEntity,
			wantType:   httpserver.ProblemValidation,
			wantDetail: detailUnprocessable,
		},
		{
			name:       "an address that folds to nothing",
			refusal:    ErrNoAddress,
			wantStatus: http.StatusUnprocessableEntity,
			wantType:   httpserver.ProblemValidation,
			wantDetail: detailUnprocessable,
		},
		{
			name:       "the same specialist named twice",
			refusal:    ErrSpecialistNamedTwice,
			wantStatus: http.StatusUnprocessableEntity,
			wantType:   httpserver.ProblemValidation,
			wantDetail: detailUnprocessable,
		},
		{
			name:       "an assignment somebody else already wrote",
			refusal:    ErrAssignmentCollided,
			wantStatus: http.StatusUnprocessableEntity,
			wantType:   httpserver.ProblemValidation,
			wantDetail: detailUnprocessable,
		},
		{
			// The default, and the reason the arms above are worth having: anything this package does not name
			// is a failure rather than a rule, and says nothing to the caller.
			name:       "a refusal this package does not name",
			refusal:    errors.New("the database went away mid-statement"),
			wantStatus: http.StatusInternalServerError,
			wantType:   httpserver.ProblemInternal,
			wantDetail: detailInternalOnTheWire,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			answered := answerFor(t, refusalFor, tc.refusal)

			if answered.Status != tc.wantStatus {
				t.Errorf("status = %d, want %d: %s", answered.Status, tc.wantStatus, answered.Detail)
			}

			if answered.Type != tc.wantType {
				t.Errorf("type = %q, want %q", answered.Type, tc.wantType)
			}

			if answered.Detail != tc.wantDetail {
				t.Errorf("detail = %q, want %q", answered.Detail, tc.wantDetail)
			}

			if tc.notTheSentence != "" && answered.Detail == tc.notTheSentence {
				t.Errorf("detail = %q, which is the answer given to a form that needs rereading",
					answered.Detail)
			}
		})
	}
}

package identity

import (
	"errors"
	"net/http"
	"testing"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

// What the device is told, type included: a client branches on the type URI, so a status arriving under the wrong
// type is a break the status assertion cannot see.
func TestWhichRefusalAReportedTimezoneHeard(t *testing.T) {
	tests := []struct {
		name       string
		refusal    error
		wantStatus int
		wantType   string
		wantDetail string
	}{
		{
			name:       "a zone this server does not know",
			refusal:    ErrUnknownTimezone,
			wantStatus: http.StatusBadRequest,
			wantType:   httpserver.ProblemValidation,
			wantDetail: detailNotATimezone,
		},
		{
			// Unreachable through the schema, which pins minLength 1 — and reachable through RecordTimezone,
			// which is exported and whose zone probe accepts the empty string by design.
			name:       "a report with no zone at all",
			refusal:    ErrNoTimezone,
			wantStatus: http.StatusBadRequest,
			wantType:   httpserver.ProblemValidation,
			wantDetail: detailNotATimezone,
		},
		{
			name:       "an account the invitation reached and provisioning did not",
			refusal:    ErrNoRole,
			wantStatus: http.StatusForbidden,
			wantType:   httpserver.ProblemForbidden,
			wantDetail: detailNoRole,
		},
		{
			name:       "the database did not answer",
			refusal:    ErrDatabaseUnavailable,
			wantStatus: http.StatusServiceUnavailable,
			wantType:   httpserver.ProblemUnavailable,
			wantDetail: detailUnavailableOnTheWire,
		},
		{
			name:       "a refusal this package does not name",
			refusal:    errors.New("the database went away mid-statement"),
			wantStatus: http.StatusInternalServerError,
			wantType:   httpserver.ProblemInternal,
			wantDetail: detailInternalOnTheWire,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			answered := answerFor(t, refusalForSession, tc.refusal)

			if answered.Status != tc.wantStatus {
				t.Errorf("status = %d, want %d: %s", answered.Status, tc.wantStatus, answered.Detail)
			}

			if answered.Type != tc.wantType {
				t.Errorf("type = %q, want %q", answered.Type, tc.wantType)
			}

			if answered.Detail != tc.wantDetail {
				t.Errorf("detail = %q, want %q", answered.Detail, tc.wantDetail)
			}
		})
	}
}

// A nil pool builds no service, so the handler's assembly check has something to refuse on rather than a value that
// dereferences. Asserted here because it is the whole of what the constructor does.
func TestNoPoolBuildsNoService(t *testing.T) {
	if sessions := NewSessions(nil); sessions != nil {
		t.Errorf("NewSessions(nil) = %v, want nil", sessions)
	}
}

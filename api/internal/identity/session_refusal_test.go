package identity

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

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
			// Unreachable through the schema, which pins minLength 1, and answered anyway: RecordTimezone is
			// exported and the empty string is the one value requireKnownTimezone lets through.
			name:       "a report with no zone at all",
			refusal:    ErrNoTimezone,
			wantStatus: http.StatusBadRequest,
			wantType:   httpserver.ProblemValidation,
			wantDetail: detailNotATimezone,
		},
		{
			name:       "the database did not answer",
			refusal:    ErrDatabaseUnavailable,
			wantStatus: http.StatusServiceUnavailable,
			wantType:   httpserver.ProblemUnavailable,
			wantDetail: detailUnavailableOnTheWire,
		},
		{
			name:       "a patient token whose profile is not there",
			refusal:    ErrNoProfileToRecordAgainst,
			wantStatus: http.StatusInternalServerError,
			wantType:   httpserver.ProblemInternal,
			wantDetail: detailInternalOnTheWire,
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

// Which failures mean «not now». The refusal below them is the one this route must not answer 503 to: a permission
// that went missing is a broken deployment, and telling the device to retry would hide it forever.
func TestWhatCountsAsTheDatabaseNotAnswering(t *testing.T) {
	tests := []struct {
		name            string
		err             error
		wantUnavailable bool
	}{
		{"the request ran out of time", context.DeadlineExceeded, true},
		{"the connection failed", &pgconn.PgError{Code: "08006"}, true},
		{"the server is shutting down", &pgconn.PgError{Code: "57P01"}, true},
		{"the backend was terminated", &pgconn.PgError{Code: "57P02"}, true},
		{"the database stopped accepting connections", &pgconn.PgError{Code: "57P03"}, true},
		{"a grant is missing", &pgconn.PgError{Code: insufficientPrivilege}, false},
		{"a row already exists", &pgconn.PgError{Code: uniqueViolation}, false},
		{"the caller gave up", context.Canceled, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			classified := classifyAvailability(tc.err)

			if got := errors.Is(classified, ErrDatabaseUnavailable); got != tc.wantUnavailable {
				t.Errorf("unavailable = %v, want %v", got, tc.wantUnavailable)
			}

			// The cause survives the classification, or the log loses the only description of what happened.
			if !errors.Is(classified, tc.err) {
				t.Errorf("the classified error no longer carries %v", tc.err)
			}
		})
	}
}

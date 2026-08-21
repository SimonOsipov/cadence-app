package identity

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

// Type included: a client branches on the URI, so a status under the wrong type is a break a status assertion misses.
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
			// Unreachable through the schema, reachable through the exported method.
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

// The sentences, by literal. Everywhere else they are compared against their own constants, so an empty string or
// the wrong one of the two would pass the suite while the device showed a patient the wrong reason.
func TestTheRussianTheDeviceReads(t *testing.T) {
	if detailNotATimezone != "Часовой пояс не распознан." {
		t.Errorf("detailNotATimezone = %q", detailNotATimezone)
	}

	if detailNoRole != "Аккаунт ещё не заведён в клинике." {
		t.Errorf("detailNoRole = %q", detailNoRole)
	}
}

// The whole of what the constructor does.
func TestNoPoolBuildsNoService(t *testing.T) {
	if sessions := NewSessions(nil); sessions != nil {
		t.Errorf("NewSessions(nil) = %v, want nil", sessions)
	}
}

// The join: any 403 satisfies the type, so the sentence is what says this went through refusalForSession.
func TestAnAccountWithNoRoleIsRefusedInTheMappersWords(t *testing.T) {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), auth.Principal{
				Subject: "8a1f3b7c-0000-4000-8000-000000000004",
			})))
		})
	})
	NewService(Deps{}).Register(httpserver.NewAPI(router))

	request := httptest.NewRequest(
		http.MethodPost, "/v1/me/session", strings.NewReader(`{"timezone":"Asia/Tbilisi"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, request)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body %s", rec.Code, rec.Body)
	}

	var problem struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decoding the problem document: %v", err)
	}

	if problem.Detail != detailNoRole {
		t.Errorf("detail = %q, want %q", problem.Detail, detailNoRole)
	}
}

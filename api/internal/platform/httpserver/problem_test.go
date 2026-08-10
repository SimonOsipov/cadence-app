package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chimw "github.com/go-chi/chi/v5/middleware"
)

func decodeProblem(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	if ct := w.Header().Get("Content-Type"); ct != ProblemContentType {
		t.Errorf("Content-Type = %q, want %q", ct, ProblemContentType)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body %q: %v", w.Body.String(), err)
	}

	return body
}

func TestWriteProblemCarriesTheRFCFields(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/patients/17", nil)

	WriteProblem(w, r, Problem{Status: http.StatusNotFound, Type: ProblemNotFound})

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	body := decodeProblem(t, w)
	if body["type"] != ProblemNotFound {
		t.Errorf("type = %v, want %q", body["type"], ProblemNotFound)
	}
	if body["title"] != "Not Found" {
		t.Errorf("title = %v, want the status text", body["title"])
	}
	if body["status"] != float64(http.StatusNotFound) {
		t.Errorf("status field = %v, want %d", body["status"], http.StatusNotFound)
	}
	if body["instance"] != "/v1/patients/17" {
		t.Errorf("instance = %v, want the request path", body["instance"])
	}
}

// The request id is the only thread between a response a caller can quote and a
// log line an engineer can find. Without it in the body, a 500 is unreportable.
func TestWriteProblemCarriesTheRequestID(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/thing", nil)

	var seen string
	chimw.RequestID(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		seen = chimw.GetReqID(req.Context())
		WriteProblem(w, req, Problem{Status: http.StatusInternalServerError, Type: ProblemInternal})
	})).ServeHTTP(httptest.NewRecorder(), r)

	if seen == "" {
		t.Fatal("the middleware produced no request id")
	}
	if got := decodeProblem(t, w)["request_id"]; got != seen {
		t.Errorf("request_id = %v, want %q", got, seen)
	}
}

// A 5xx says the same thing whatever went wrong. The caller learns nothing
// about the inside of the system, and the engineer learns everything from the
// log line the request id points at.
func TestServerErrorDetailIsFixed(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/thing", nil)

	WriteProblem(w, r, Problem{
		Status: http.StatusInternalServerError,
		Type:   ProblemInternal,
		Detail: `pq: relation "app.dose_events" does not exist`,
		Errors: []ProblemDetail{{Message: "0.25", Location: "body.dose.value"}},
	})

	body := decodeProblem(t, w)
	if detail, _ := body["detail"].(string); detail != internalDetail {
		t.Errorf("detail = %q, want the fixed string", detail)
	}
	if strings.Contains(w.Body.String(), "dose_events") {
		t.Errorf("the response carries the underlying error: %s", w.Body.String())
	}
	if _, ok := body["errors"]; ok {
		t.Errorf("a 5xx carries per-field errors: %s", w.Body.String())
	}
}

// Below 500 the detail is the point: it tells the caller what to change.
func TestClientErrorKeepsItsDetail(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/thing", nil)

	WriteProblem(w, r, Problem{
		Status: http.StatusUnprocessableEntity,
		Type:   ProblemValidation,
		Detail: "validation failed",
		Errors: []ProblemDetail{{Message: "expected a number", Location: "body.dose.value"}},
	})

	body := decodeProblem(t, w)
	if body["detail"] != "validation failed" {
		t.Errorf("detail = %v, want it preserved", body["detail"])
	}

	errs, ok := body["errors"].([]any)
	if !ok || len(errs) != 1 {
		t.Fatalf("errors = %v, want one entry", body["errors"])
	}
	first, _ := errs[0].(map[string]any)
	if first["location"] != "body.dose.value" {
		t.Errorf("location = %v, want it preserved", first["location"])
	}
}

// A rejected token says 401 and nothing else. Distinguishing "expired" from
// "wrong issuer" from "no such key" tells an attacker which half of the guess
// was right; the distinction lives in the log, where it is useful and private.
func TestUnauthorizedDoesNotDistinguishTheReason(t *testing.T) {
	bodies := make([]string, 0, 2)
	for _, detail := range []string{"token expired", "unknown key id"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/me", nil)

		WriteProblem(w, r, Problem{
			Status: http.StatusUnauthorized,
			Type:   ProblemUnauthorized,
			Detail: detail,
		})

		bodies = append(bodies, w.Body.String())
	}

	if bodies[0] != bodies[1] {
		t.Errorf("two rejection reasons produced two bodies:\n%s\n%s", bodies[0], bodies[1])
	}
	if strings.Contains(bodies[0], "expired") || strings.Contains(bodies[0], "key id") {
		t.Errorf("the body describes why the token was rejected: %s", bodies[0])
	}
}

// The challenge belongs to the status, not to the caller that happened to
// remember it. RFC 7235 requires it on every 401, and there are two writers of
// this type — this one and huma's error path — so a rule enforced at the call
// site is a rule one of them will eventually skip.
func TestUnauthorizedCarriesTheChallenge(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/me", nil)

	WriteProblem(w, r, Problem{Status: http.StatusUnauthorized, Type: ProblemUnauthorized})

	if got := w.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Errorf("WWW-Authenticate = %q, want Bearer", got)
	}
}

// ...and only to that status. A challenge on a 404 would invite a client to
// retry with credentials against a path that does not exist.
func TestOtherStatusesCarryNoChallenge(t *testing.T) {
	for _, status := range []int{
		http.StatusNotFound,
		http.StatusMethodNotAllowed,
		http.StatusUnprocessableEntity,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
	} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/v1/me", nil)

		WriteProblem(w, r, Problem{Status: status})

		if got := w.Header().Get("WWW-Authenticate"); got != "" {
			t.Errorf("status %d carries WWW-Authenticate = %q, want none", status, got)
		}
	}
}

// The fifth path. A payload that cannot be marshalled used to answer in a
// hand-written literal that bypassed the error writer entirely — right shape by
// coincidence, and the coincidence would not have survived this change.
func TestUnmarshallablePayloadStillAnswersProblemJSON(t *testing.T) {
	w := httptest.NewRecorder()

	JSON(w, http.StatusOK, map[string]any{"bad": func() {}})

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	body := decodeProblem(t, w)
	if body["status"] != float64(http.StatusInternalServerError) {
		t.Errorf("status field = %v", body["status"])
	}
	if detail, _ := body["detail"].(string); detail != internalDetail {
		t.Errorf("detail = %q, want the fixed string", detail)
	}
}

func TestJSONWithoutPayloadWritesEmptyBody(t *testing.T) {
	w := httptest.NewRecorder()

	JSON(w, http.StatusNoContent, nil)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if got := w.Body.Len(); got != 0 {
		t.Errorf("body length = %d, want 0 (body: %q)", got, w.Body.String())
	}
}

func TestJSONSuccessKeepsPlainContentType(t *testing.T) {
	w := httptest.NewRecorder()

	JSON(w, http.StatusOK, map[string]string{"status": "ok"})

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

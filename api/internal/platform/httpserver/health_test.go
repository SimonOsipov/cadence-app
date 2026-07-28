package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestHealthWithoutProbeReportsOK(t *testing.T) {
	w := httptest.NewRecorder()

	Health(nil, discardLogger())(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want ok", body["status"])
	}
}

func TestHealthWithPassingProbeReportsOK(t *testing.T) {
	called := false
	probe := func(context.Context) error {
		called = true
		return nil
	}

	w := httptest.NewRecorder()
	Health(probe, discardLogger())(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if !called {
		t.Error("probe was not called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHealthWithFailingProbeReportsUnavailable(t *testing.T) {
	probe := func(context.Context) error {
		return errors.New("dial tcp 10.0.0.1:5432: connection refused")
	}

	w := httptest.NewRecorder()
	Health(probe, discardLogger())(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Code != "UNHEALTHY" {
		t.Errorf("Code = %q, want UNHEALTHY", body.Code)
	}
	// The endpoint is unauthenticated: it must not describe the failure to the
	// caller. The detail belongs in the log.
	if strings.Contains(w.Body.String(), "10.0.0.1") {
		t.Errorf("response leaks probe internals: %s", w.Body.String())
	}
}

func TestHealthPassesRequestContextToProbe(t *testing.T) {
	type ctxKey struct{}

	var seen any
	probe := func(ctx context.Context) error {
		seen = ctx.Value(ctxKey{})
		return nil
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxKey{}, "marker"))

	Health(probe, discardLogger())(httptest.NewRecorder(), req)

	if seen != "marker" {
		t.Errorf("probe context value = %v, want marker — probe did not receive the request context", seen)
	}
}

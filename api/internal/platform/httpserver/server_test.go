package httpserver

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testServer(t *testing.T) *Server {
	t.Helper()

	return New(Config{
		Port:           "0",
		ReadTimeout:    time.Second,
		WriteTimeout:   time.Second,
		IdleTimeout:    time.Second,
		AllowedOrigins: []string{"https://dash.example.com"},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestUnknownRouteAnswersErrorResponse(t *testing.T) {
	srv := testServer(t)
	w := httptest.NewRecorder()

	srv.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/nope", nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body %q: %v", w.Body.String(), err)
	}
	if body.Code != "NOT_FOUND" {
		t.Errorf("Code = %q, want NOT_FOUND", body.Code)
	}
}

func TestWrongMethodAnswersErrorResponse(t *testing.T) {
	srv := testServer(t)
	srv.Router.Get("/thing", func(w http.ResponseWriter, _ *http.Request) { JSON(w, http.StatusOK, nil) })

	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/thing", nil))

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}

	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body %q: %v", w.Body.String(), err)
	}
	if body.Code != "METHOD_NOT_ALLOWED" {
		t.Errorf("Code = %q, want METHOD_NOT_ALLOWED", body.Code)
	}
}

func TestPanicBecomesErrorResponse(t *testing.T) {
	srv := testServer(t)
	srv.Router.Get("/boom", func(http.ResponseWriter, *http.Request) { panic("handler exploded") })

	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body %q: %v", w.Body.String(), err)
	}
	if body.Code != "INTERNAL" {
		t.Errorf("Code = %q, want INTERNAL", body.Code)
	}
	// The panic value must not reach the caller.
	if body.Error == "handler exploded" {
		t.Errorf("response leaks the panic value: %s", w.Body.String())
	}
}

func TestCORSAllowsConfiguredOriginOnly(t *testing.T) {
	srv := testServer(t)
	srv.Router.Get("/thing", func(w http.ResponseWriter, _ *http.Request) { JSON(w, http.StatusOK, nil) })

	tests := []struct {
		name       string
		origin     string
		wantOrigin string
	}{
		{name: "configured origin", origin: "https://dash.example.com", wantOrigin: "https://dash.example.com"},
		{name: "foreign origin", origin: "https://evil.example.com", wantOrigin: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, "/thing", nil)
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Access-Control-Request-Method", http.MethodGet)

			w := httptest.NewRecorder()
			srv.Router.ServeHTTP(w, req)

			if got := w.Header().Get("Access-Control-Allow-Origin"); got != tt.wantOrigin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tt.wantOrigin)
			}
		})
	}
}

func TestCORSDoesNotAllowCredentials(t *testing.T) {
	srv := testServer(t)
	srv.Router.Get("/thing", func(w http.ResponseWriter, _ *http.Request) { JSON(w, http.StatusOK, nil) })

	req := httptest.NewRequest(http.MethodOptions, "/thing", nil)
	req.Header.Set("Origin", "https://dash.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)

	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	// Callers authenticate with a Bearer token, never with cookies. Allowing
	// credentials would widen CORS for nothing.
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want it unset", got)
	}
}

package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONWritesBodyAndContentType(t *testing.T) {
	w := httptest.NewRecorder()

	JSON(w, http.StatusCreated, map[string]string{"key": "value"})

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["key"] != "value" {
		t.Errorf("body[key] = %q, want value", body["key"])
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

func TestErrorWritesErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()

	Error(w, http.StatusBadRequest, "bad request", "VALIDATION_ERROR")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Error != "bad request" {
		t.Errorf("Error = %q, want %q", body.Error, "bad request")
	}
	if body.Code != "VALIDATION_ERROR" {
		t.Errorf("Code = %q, want %q", body.Code, "VALIDATION_ERROR")
	}
}

func TestErrorOmitsEmptyCode(t *testing.T) {
	w := httptest.NewRecorder()

	Error(w, http.StatusInternalServerError, "internal error", "")

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if _, ok := body["code"]; ok {
		t.Errorf("body carries an empty code field: %v", body)
	}
}

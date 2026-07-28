package httpserver

import (
	"encoding/json"
	"net/http"
)

// internalErrorBody is the pre-rendered body used when a payload cannot be
// marshalled. It is a literal so that writing it can never fail for the same
// reason the original payload did.
const internalErrorBody = `{"error":"internal error","code":"INTERNAL"}`

// ErrorResponse is the single error shape of the API. Every non-2xx response
// carries it, so clients parse one form regardless of which context failed.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// JSON writes data as a JSON body with the given status code. A nil payload
// writes the status line only.
//
// The payload is marshalled before anything is written, so a payload that
// cannot be encoded degrades to a 500 instead of a 200 with a truncated body.
func JSON(w http.ResponseWriter, status int, data any) {
	var body []byte
	if data != nil {
		marshalled, err := json.Marshal(data)
		if err != nil {
			status, body = http.StatusInternalServerError, []byte(internalErrorBody)
		} else {
			body = marshalled
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if len(body) > 0 {
		// The status line is already on the wire; a failed write means the
		// client is gone and there is nothing left to report to it.
		_, _ = w.Write(body)
	}
}

// Error writes an ErrorResponse with the given status. An empty code is
// omitted from the body.
func Error(w http.ResponseWriter, status int, message, code string) {
	JSON(w, status, ErrorResponse{Error: message, Code: code})
}

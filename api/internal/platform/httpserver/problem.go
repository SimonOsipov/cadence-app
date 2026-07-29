package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// ProblemContentType is the media type of every non-2xx response, per RFC 9457.
// A client that sees it knows the body is a problem document and not the
// resource it asked for, without having to look at the status code first.
const ProblemContentType = "application/problem+json"

// Problem types. RFC 9457 puts the machine-readable discriminator in `type`,
// which is why there is no separate code field: a client branches on the URI,
// a human follows it. They are relative on purpose — the API is reachable under
// more than one host, and a document that hard-codes one is wrong on the others.
const (
	ProblemNotFound         = "/problems/not-found"
	ProblemMethodNotAllowed = "/problems/method-not-allowed"
	ProblemUnauthorized     = "/problems/unauthorized"
	ProblemValidation       = "/problems/validation"
	ProblemInternal         = "/problems/internal"
	ProblemUnhealthy        = "/problems/unhealthy"
)

// internalDetail is the only detail a 5xx ever carries. What actually happened
// goes to the log under the same request id, where it is useful to an engineer
// and invisible to everyone else.
const internalDetail = "The request could not be completed. Quote the request id when reporting this."

// unauthorizedDetail is the only detail a 401 ever carries, for the same reason
// in a different direction: what the caller learns must not depend on which
// check failed.
const unauthorizedDetail = "The request was not authenticated."

// problemFallbackBody is the pre-rendered document used when a payload cannot
// be marshalled. A literal, so that writing it cannot fail for the same reason
// the original payload did.
const problemFallbackBody = `{"type":"/problems/internal","title":"Internal Server Error","status":500,` +
	`"detail":"` + internalDetail + `"}`

// Problem is the single error shape of the API: RFC 9457 problem details.
//
// It is also what huma returns for validation failures and for anything a
// handler raises, so the generated clients decode one type on every path
// regardless of which layer refused.
type Problem struct {
	Type   string `json:"type" doc:"A URI reference identifying the problem type."`
	Title  string `json:"title" doc:"A short summary of the problem type, stable across occurrences."`
	Status int    `json:"status" doc:"The HTTP status code."`

	// Detail explains this occurrence. Above 499 it is a fixed string: see
	// internalDetail.
	Detail string `json:"detail,omitempty" doc:"An explanation specific to this occurrence."`

	// Instance is the path of the request that produced the problem. The client
	// sent it, so echoing it discloses nothing it does not already know.
	Instance string `json:"instance,omitempty" doc:"The request path this problem occurred on."`

	// RequestID is the thread between a response a caller can quote and the log
	// line an engineer can find.
	RequestID string `json:"request_id,omitempty" doc:"Correlates this response with the server log."`

	Errors []ProblemDetail `json:"errors,omitempty" doc:"Per-field detail, present on validation failures."`
}

// ProblemDetail locates one field-level failure.
//
// Deliberately narrower than huma's own ErrorDetail, which also echoes the
// submitted value. For a dose or a measurement that value is medical data, and
// echoing it would put it in the response, in the access log, and in whatever
// the client reports to its own error tracker. The message and the location say
// what has to change without repeating what was sent.
type ProblemDetail struct {
	Message  string `json:"message" doc:"What is wrong with the value at this location."`
	Location string `json:"location,omitempty" doc:"Where the problem is, e.g. body.dose.value."`
}

// MarshalJSON scrubs before it serialises.
//
// This is the last gate, and it is here rather than at the call sites because
// there is no path to the wire that goes around it. A Problem built by a
// handler, by huma's validator or by the router is normalised at the moment it
// becomes bytes — so forgetting to scrub is not a mistake anyone can make.
//
// The receiver is a value: normalise runs on a copy, and whatever the caller
// still holds keeps the detail that has to reach the log.
func (p Problem) MarshalJSON() ([]byte, error) {
	p.normalise()

	// A distinct type so that marshalling it does not call this method again.
	type wire Problem

	body, err := json.Marshal(wire(p))
	if err != nil {
		return nil, fmt.Errorf("marshalling problem document: %w", err)
	}

	return body, nil
}

// Error satisfies the error interface so that a Problem can be returned from a
// handler.
func (p *Problem) Error() string {
	if p.Detail != "" {
		return p.Detail
	}

	return p.Title
}

// GetStatus satisfies huma.StatusError.
func (p *Problem) GetStatus() int {
	return p.Status
}

// GetHeaders satisfies huma.HeadersError.
//
// It exists so that a 401 raised inside a handler is byte-for-byte the same
// response as a 401 raised by the authentication middleware, headers included.
// RFC 7235 requires the challenge on every 401, and a challenge present on one
// path and absent on another is itself a distinction — on a status whose whole
// design is that the caller learns nothing from it.
//
// Bare "Bearer": no realm, and above all no error parameter. `error=
// "invalid_token"` would put in a header exactly the reason normalise removes
// from the body.
func (p *Problem) GetHeaders() http.Header {
	if p.Status != http.StatusUnauthorized {
		return nil
	}

	return http.Header{"Www-Authenticate": []string{"Bearer"}}
}

// ContentType satisfies huma.ContentTypeFilter, which is how huma learns to
// send problem+json rather than plain JSON for this body.
func (p *Problem) ContentType(ct string) string {
	if ct == "application/json" {
		return ProblemContentType
	}

	return ct
}

// WriteProblem sends p as a problem document, filling in what comes from the
// request and from the status: the title, the instance, the request id, and —
// for a server error — the fixed detail that replaces whatever the caller
// passed.
func WriteProblem(w http.ResponseWriter, r *http.Request, p Problem) {
	requestID := chimw.GetReqID(r.Context())

	logScrubbed(r.Context(), p, requestID)
	p.normalise()
	p.Instance = r.URL.Path
	p.RequestID = requestID

	body, err := json.Marshal(p)
	if err != nil {
		// Problem has no field that can fail to marshal, so this is unreachable
		// today. It is handled rather than ignored because the day someone adds
		// one, the alternative is a 200 with half a body.
		writeRaw(w, http.StatusInternalServerError, ProblemContentType, []byte(problemFallbackBody))

		return
	}

	writeRaw(w, p.Status, ProblemContentType, body)
}

// logScrubbed writes down what normalise is about to destroy.
//
// Scrubbing and logging are one action on purpose. Every place that says the
// caller learns nothing also says the reason lives in the log, and the only way
// that stays true is for the code that removes the reason to be the code that
// records it. A caller passes the real cause into Problem.Detail and gets the
// guarantee for free; a caller that passes nothing has nothing to lose here and
// has usually logged already, which is why this is silent in that case.
func logScrubbed(ctx context.Context, p Problem, requestID string) {
	if p.Detail == "" && len(p.Errors) == 0 {
		return
	}

	level := slog.LevelError
	switch {
	case p.Status >= http.StatusInternalServerError:
	case p.Status == http.StatusUnauthorized:
		level = slog.LevelWarn
	default:
		// Below 500 and not a rejected token: the detail reaches the caller
		// unchanged, so there is nothing the log would be preserving.
		return
	}

	attrs := []any{
		"status", p.Status,
		"type", p.Type,
		"detail", p.Detail,
		"request_id", requestID,
	}
	for i, detail := range p.Errors {
		attrs = append(attrs, fmt.Sprintf("error_%d", i), detail.Location+": "+detail.Message)
	}

	loggerFrom(ctx).Log(ctx, level, "response detail withheld from the caller", attrs...)
}

// normalise applies the rules that must not depend on the caller remembering
// them at the call site. A title always matches the status; a server error
// never says more than internalDetail; and a rejected token says only that it
// was rejected.
//
// Enforced here rather than at each call site because there will be more call
// sites than there are today, and the one that forgets is the one that leaks.
func (p *Problem) normalise() {
	if p.Type == "" {
		p.Type = "about:blank"
	}
	if p.Title == "" {
		p.Title = http.StatusText(p.Status)
	}

	switch {
	case p.Status >= http.StatusInternalServerError:
		p.Detail = internalDetail
		p.Errors = nil

	case p.Status == http.StatusUnauthorized:
		// Expired, wrong issuer, unknown key, no header at all — one body for
		// all of them. Telling them apart tells whoever is guessing which half
		// of the guess was right. The distinction goes to the log, where it is
		// useful and private.
		p.Detail = unauthorizedDetail
		p.Errors = nil
	}
}

// JSON writes data as a JSON body with the given status code. A nil payload
// writes the status line only.
//
// The payload is marshalled before anything is written, so a payload that
// cannot be encoded degrades to a problem document instead of a 200 with a
// truncated body.
func JSON(w http.ResponseWriter, status int, data any) {
	if data == nil {
		writeRaw(w, status, "application/json", nil)

		return
	}

	body, err := json.Marshal(data)
	if err != nil {
		writeRaw(w, http.StatusInternalServerError, ProblemContentType, []byte(problemFallbackBody))

		return
	}

	writeRaw(w, status, "application/json", body)
}

func writeRaw(w http.ResponseWriter, status int, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)

	if len(body) > 0 {
		// The status line is already on the wire; a failed write means the client
		// is gone and there is nothing left to report to it.
		_, _ = w.Write(body)
	}
}

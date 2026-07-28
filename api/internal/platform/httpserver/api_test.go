package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func testAPI(t *testing.T) (*chi.Mux, huma.API) {
	t.Helper()

	router := chi.NewRouter()

	return router, NewAPI(router)
}

// The security scheme has to be in the document, not only in the middleware: a
// generated client reads the document to decide whether to send Authorization
// at all. A guarded API whose spec says nothing produces clients that never
// authenticate.
func TestAPIDeclaresBearerSecurity(t *testing.T) {
	_, api := testAPI(t)

	scheme, ok := api.OpenAPI().Components.SecuritySchemes["bearer"]
	if !ok {
		t.Fatalf("no bearer security scheme; have %v", api.OpenAPI().Components.SecuritySchemes)
	}
	if scheme.Type != "http" || scheme.Scheme != "bearer" {
		t.Errorf("scheme = %+v, want http/bearer", scheme)
	}
}

func TestRegisteredOperationsRequireBearer(t *testing.T) {
	_, api := testAPI(t)
	registerProbeOperations(api)

	op := api.OpenAPI().Paths["/v1/probe"].Get
	if op == nil {
		t.Fatal("the probe operation is not in the document under its full path")
	}

	// nil means "inherit the document default"; an empty non-nil slice means
	// "this operation is open". The two are indistinguishable by length, and it
	// is the second one this test exists to catch.
	if op.Security != nil && len(op.Security) == 0 {
		t.Fatal("the operation opts out of security entirely")
	}
	if op.Security == nil && len(api.OpenAPI().Security) == 0 {
		t.Fatal("the operation inherits a document default that requires nothing")
	}
}

// The contract has to name the error shape, or a generated client decodes
// whatever it guesses. huma will fall back to its own ErrorModel — the one that
// carries the submitted value — for any API not built through NewAPI, and the
// drift gate cannot notice because the committed document has no operations yet.
func TestTheContractDescribesTheProblemShape(t *testing.T) {
	_, api := testAPI(t)
	registerProbeOperations(api)

	responses := api.OpenAPI().Paths["/v1/probe"].Get.Responses
	fallback, ok := responses["default"]
	if !ok {
		t.Fatalf("the operation declares no default response; have %v", responses)
	}

	content, ok := fallback.Content[ProblemContentType]
	if !ok {
		t.Fatalf("the default response is not %s; have %v", ProblemContentType, fallback.Content)
	}
	if got, want := content.Schema.Ref, "#/components/schemas/Problem"; got != want {
		t.Errorf("default response schema = %q, want %q", got, want)
	}
	if _, ok := api.OpenAPI().Components.Schemas.Map()["Problem"]; !ok {
		t.Error("the document has no Problem schema")
	}
}

// huma's default configuration stamps a $schema member into every response body,
// built from the request Host, and serves the schemas under a third
// unauthenticated path. Both are off: an error must have one wire shape whether
// it came from huma or from the router, and the next step's middleware should
// not have to learn about a path nobody asked for.
func TestNoSchemaMemberAndNoSchemasPath(t *testing.T) {
	router, api := testAPI(t)
	registerProbeOperations(api)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/probe", nil))

	if strings.Contains(w.Body.String(), "$schema") {
		t.Errorf("the error body carries a $schema member: %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/schemas/Problem.json", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("GET /schemas/Problem.json = %d, want 404 — the path should not exist", w.Code)
	}
}

// The reason a 5xx or a 401 withholds has to end up somewhere, or the request id
// the caller is told to quote leads to an access-log line saying status=500.
func TestWithheldDetailReachesTheLog(t *testing.T) {
	var logged bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug}))

	router := chi.NewRouter()
	router.Use(chimw.RequestID)
	router.Use(withLogger(logger))
	registerProbeOperations(NewAPI(router))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/probe", nil))

	if !strings.Contains(logged.String(), "dose_events") {
		t.Errorf("the withheld cause is nowhere in the log: %s", logged.String())
	}
	if strings.Contains(w.Body.String(), "dose_events") {
		t.Errorf("the cause reached the caller as well: %s", w.Body.String())
	}

	// Useless without the thread back to the response.
	if !strings.Contains(logged.String(), "request_id") {
		t.Errorf("the log line has no request id: %s", logged.String())
	}
}

// huma's own error detail echoes the submitted value back to the caller. For a
// dose or a measurement that value is medical data, and echoing it puts it in
// the response body, the access log, and whatever the client reports to its own
// error tracker.
func TestValidationFailureNeverEchoesTheSubmittedValue(t *testing.T) {
	router, api := testAPI(t)
	registerProbeOperations(api)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/probe",
		strings.NewReader(`{"dose":"a patient identifier that must not come back"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body: %s)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, ProblemContentType) {
		t.Errorf("Content-Type = %q, want %q", ct, ProblemContentType)
	}
	if strings.Contains(w.Body.String(), "must not come back") {
		t.Errorf("the response echoes the submitted value: %s", w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body %q: %v", w.Body.String(), err)
	}

	errs, ok := body["errors"].([]any)
	if !ok || len(errs) == 0 {
		t.Fatalf("errors = %v, want the field locations preserved", body["errors"])
	}
	first, _ := errs[0].(map[string]any)
	if _, leaked := first["value"]; leaked {
		t.Errorf("an error detail carries a value field: %s", w.Body.String())
	}
	if first["location"] == nil {
		t.Errorf("the error detail has no location, so it says nothing useful: %s", w.Body.String())
	}
}

// A handler that fails says the same thing every other 5xx says.
func TestHandlerFailureIsAProblemWithoutDetail(t *testing.T) {
	router, api := testAPI(t)
	registerProbeOperations(api)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/probe", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body: %s)", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "dose_events") {
		t.Errorf("the response carries the underlying error: %s", w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body %q: %v", w.Body.String(), err)
	}
	if detail, _ := body["detail"].(string); detail != internalDetail {
		t.Errorf("detail = %q, want the fixed string", detail)
	}
}

// The document and its viewer are what someone generating a client reaches
// for. A typo in either path would leave both 404 and nothing else would
// notice — the API would serve every operation correctly and be undiscoverable.
func TestTheDocumentAndDocsAreServed(t *testing.T) {
	router, api := testAPI(t)
	registerProbeOperations(api)

	for _, path := range []string{OpenAPIPath + ".json", OpenAPIPath + ".yaml", DocsPath} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))

		if w.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, w.Code)
		}
	}
}

type probeInput struct {
	Body struct {
		Dose float64 `json:"dose"`
	}
}

type probeOutput struct {
	Body struct {
		OK bool `json:"ok"`
	}
}

func registerProbeOperations(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "probe-validation",
		Method:      http.MethodPost,
		Path:        "/v1/probe",
		Summary:     "Probe used by the transport tests",
	}, func(context.Context, *probeInput) (*probeOutput, error) {
		return &probeOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "probe-failure",
		Method:      http.MethodGet,
		Path:        "/v1/probe",
		Summary:     "Probe that always fails",
	}, func(context.Context, *struct{}) (*probeOutput, error) {
		return nil, huma.Error500InternalServerError(`pq: relation "app.dose_events" does not exist`)
	})
}

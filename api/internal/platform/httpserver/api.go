package httpserver

import (
	"errors"
	"net/http"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// APIVersion is the version reported in the OpenAPI document. It describes the
// contract, not the deployed revision — the revision is a build argument and
// would make the committed document churn on every commit.
const APIVersion = "0.1.0"

// Paths that belong to the transport rather than to a bounded context. They sit
// outside /v1 and outside authentication: /healthz is polled by an external
// monitor, and the document and its viewer have to be readable by whoever is
// generating a client.
const (
	HealthPath  = "/healthz"
	OpenAPIPath = "/openapi"
	DocsPath    = "/docs"
)

// UnauthenticatedPaths lists every path that answers without a bearer token.
//
// It is the exemption list the authentication middleware is given, and it is
// exhaustive: anything not on it is refused. The four document paths are what
// huma mounts for a single OpenAPIPath — the current version and the 3.0
// downgrade, each as JSON and as YAML — and all four have to be readable by
// whoever is generating a client, or the contract is only reachable by someone
// who already has a token.
//
// The list is a function rather than a package variable so that a caller cannot
// append to it and open a path from the other side of the module.
func UnauthenticatedPaths() []string {
	return []string{
		HealthPath,
		OpenAPIPath + ".json",
		OpenAPIPath + ".yaml",
		OpenAPIPath + "-3.0.json",
		OpenAPIPath + "-3.0.yaml",
		DocsPath,
	}
}

// NewAPI builds the huma API on top of an existing chi router.
//
// Operations register their full paths — /v1/... — on the root mux rather than
// on a subrouter. humachi has no prefix-aware variant in v2.39: mounting it
// under a /v1 subrouter serves the operations correctly but writes them into
// the document as /me rather than /v1/me, and moves the document itself under
// whatever middleware guards /v1. Registering full paths costs a repeated
// prefix in one place and keeps the document truthful and reachable.
func NewAPI(router *chi.Mux) huma.API {
	useProblemErrors()

	config := huma.DefaultConfig("Cadence API", APIVersion)
	config.OpenAPIPath = OpenAPIPath
	config.DocsPath = DocsPath

	// huma's default config serves the response schemas under /schemas and adds
	// a transformer that stamps a $schema member, built from the request Host,
	// into every response body — including error bodies. Two costs we decline:
	// a third unauthenticated transport path that the next step's middleware
	// would have to know about, and a wire shape for errors that differs
	// depending on whether the error came from huma or from the router.
	config.SchemasPath = ""
	config.CreateHooks = nil

	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearer": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description:  "An access token from the project's identity provider. Verified against its JWK Set on every request.",
		},
	}

	// Declared once for the whole document rather than per operation. A scheme
	// an operation has to opt into is a scheme an operation can forget, and the
	// generated clients would then simply not send the header.
	config.Security = []map[string][]string{{"bearer": {}}}

	// The one hook that runs with the request context in hand for *every* error
	// huma writes. The construction hooks below do not: a handler returning
	// huma.Error500InternalServerError builds its error with no context at all,
	// which is most of the errors this has to record.
	config.Transformers = append(config.Transformers, logProblem)

	return humachi.New(router, config)
}

var problemErrorsOnce sync.Once

// useProblemErrors makes huma build our Problem instead of its own ErrorModel.
//
// The hook is a package-level variable in huma, so this is a global assignment
// — done once, from the only constructor that builds an API, so that the shape
// of an error cannot depend on which package happened to initialise first.
//
// Two things change relative to huma's default. The submitted value is dropped
// from every error detail: for a dose or a measurement it is medical data, and
// huma echoes it back to help with debugging. And a 5xx loses its message,
// which by the time it reaches here is usually a wrapped driver error carrying
// SQL and table names.
func useProblemErrors() {
	problemErrorsOnce.Do(func() {
		huma.NewErrorWithContext = func(ctx huma.Context, status int, msg string, errs ...error) huma.StatusError {
			problem := &Problem{
				Status: status,
				Type:   problemTypeFor(status),
				Detail: msg,
			}

			for _, err := range errs {
				if err == nil {
					continue
				}

				detail := ProblemDetail{Message: err.Error()}

				// errors.As rather than a bare assertion: (*huma.ErrorDetail).Error
				// renders the submitted value into its message, so a detail behind
				// a wrapper would fall to the branch above and echo exactly what
				// this conversion exists to drop.
				var detailer huma.ErrorDetailer
				if errors.As(err, &detailer) {
					// Message and location, never Value.
					source := detailer.ErrorDetail()
					detail = ProblemDetail{Message: source.Message, Location: source.Location}
				}

				problem.Errors = append(problem.Errors, detail)
			}

			if ctx != nil {
				problem.Instance = ctx.URL().Path
				problem.RequestID = chimw.GetReqID(ctx.Context())
			}

			// Deliberately not normalised here. The detail is what the log needs,
			// and Problem.MarshalJSON removes it on the way to the wire — so the
			// value stays complete for exactly as long as it is useful.
			return problem
		}

		// Kept in step: anything still calling the context-free hook gets the
		// same document, minus the fields that need a request.
		huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
			return huma.NewErrorWithContext(nil, status, msg, errs...)
		}
	})
}

// logProblem records what a problem document is about to stop saying, then
// hands it on unchanged. Registered as a huma transformer because that is the
// only hook that sees every error together with its request.
func logProblem(ctx huma.Context, _ string, v any) (any, error) {
	problem, ok := v.(*Problem)
	if !ok {
		return v, nil
	}

	if problem.Instance == "" {
		problem.Instance = ctx.URL().Path
	}
	if problem.RequestID == "" {
		problem.RequestID = chimw.GetReqID(ctx.Context())
	}

	logScrubbed(ctx.Context(), *problem, problem.RequestID)

	return problem, nil
}

func problemTypeFor(status int) string {
	switch status {
	case http.StatusNotFound:
		return ProblemNotFound
	case http.StatusMethodNotAllowed:
		return ProblemMethodNotAllowed
	case http.StatusUnauthorized:
		return ProblemUnauthorized
	case http.StatusForbidden:
		return ProblemForbidden
	case http.StatusConflict:
		return ProblemConflict
	case http.StatusUnprocessableEntity, http.StatusBadRequest:
		return ProblemValidation
	case http.StatusServiceUnavailable:
		// Before the 5xx arm below, which would otherwise make this the same
		// document as a 500. It is not: the caller did nothing wrong and the
		// answer to it is to try again.
		return ProblemUnavailable
	}

	if status >= http.StatusInternalServerError {
		return ProblemInternal
	}

	return "about:blank"
}

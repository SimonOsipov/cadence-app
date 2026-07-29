package router

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

// Options is what the composition root supplies to build the HTTP surface.
type Options struct {
	// Verifier checks the bearer token on every guarded request.
	Verifier *auth.Verifier

	// Probe reports whether the API's dependencies are reachable, for
	// /healthz. A nil probe answers for the process alone.
	Probe func(context.Context) error

	// Logger is used by the health endpoint to record a failing probe.
	Logger *slog.Logger
}

// Mount assembles the whole HTTP surface on mux: the authentication guard, the
// liveness endpoint, and every bounded context's operations.
//
// It exists so that there is one assembly rather than two. The deny-by-default
// test walks what this function built; if the composition root wired the same
// pieces in its own order, the test would be proving things about an
// arrangement nothing serves.
//
// Order matters and is enforced by chi: middleware has to be registered before
// any route, so the guard goes on first. That is also the safe direction — a
// route mounted before the guard would be served without it.
func Mount(mux *chi.Mux, opts Options) {
	if opts.Logger == nil {
		// Matching auth.NewVerifier. Without it, the first failing health probe
		// panics inside the handler — turning the one moment the log matters
		// most into a 500 with a stack trace where the cause should be.
		opts.Logger = slog.Default()
	}

	mux.Use(auth.Middleware(opts.Verifier, httpserver.UnauthenticatedPaths()))

	// Unauthenticated by design: the external uptime monitor polls it.
	mux.Get(httpserver.HealthPath, httpserver.Health(opts.Probe, opts.Logger))

	// Operations register their full /v1/... paths on the root mux, so the
	// document describes the paths it actually serves and stays reachable
	// without a token.
	Register(httpserver.NewAPI(mux))
}

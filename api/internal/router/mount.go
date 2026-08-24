package router

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth/token"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/storage"
)

// Photos is the object store as this package needs it: wide enough to satisfy
// every context's own narrower interface.
type Photos interface {
	SignedGet(ctx context.Context, bucket, key, contentType string, ttl time.Duration) (storage.Link, error)
	SignedPut(ctx context.Context, bucket, key string, ttl time.Duration) (storage.Link, error)
}

// Options is what the composition root supplies to build the HTTP surface.
type Options struct {
	// Verifier checks the bearer token on every guarded request.
	Verifier *token.Verifier

	// Pool is the request path. The operations that read under the caller's own
	// identity run on it, and so does the advisory lock that serialises two
	// creations of one address — which is why it is not the service pool: a
	// lock holder waiting for a second connection from the pool it locked on
	// waits for one only another holder can release.
	Pool *pgxpool.Pool

	// ServicePool is the service path: the writes no policy lets a request make.
	ServicePool *pgxpool.Pool

	// Provisioner is the account lifecycle at the identity provider. The API
	// never speaks to it directly — the component that holds the admin key does.
	Provisioner identity.Provisioner

	// Probe reports whether the API's dependencies are reachable, for
	// /healthz. A nil probe answers for the process alone.
	Probe func(context.Context) error

	// Logger is used by the health endpoint to record a failing probe.
	Logger *slog.Logger

	// Photos signs short-lived links to the object store. Its own interface and
	// not a context's, so that satisfying one context does not make this package
	// depend on another's.
	Photos Photos

	// One private bucket per kind of picture: minted keys carry the patient's id
	// and nothing else, so one bucket would let two kinds collide.
	VialsBucket      string
	InjectionsBucket string
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
		// Matching token.NewVerifier. Without it, the first failing health probe
		// panics inside the handler — turning the one moment the log matters
		// most into a 500 with a stack trace where the cause should be.
		opts.Logger = slog.Default()
	}

	mux.Use(token.Middleware(opts.Verifier, httpserver.UnauthenticatedPaths()))

	// Unauthenticated by design: the external uptime monitor polls it.
	mux.Get(httpserver.HealthPath, httpserver.Health(opts.Probe, opts.Logger))

	// Operations register their full /v1/... paths on the root mux, so the
	// document describes the paths it actually serves and stays reachable
	// without a token.
	Register(httpserver.NewAPI(mux), opts)
}

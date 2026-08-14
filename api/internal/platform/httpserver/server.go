// Package httpserver holds the HTTP transport shared by every bounded
// context: the chi router with its middleware stack, the JSON response
// helpers, and the liveness endpoint. It carries no business rules.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// shutdownTimeout bounds how long a graceful shutdown waits for in-flight
// requests before the process exits anyway.
const shutdownTimeout = 10 * time.Second

// RequestDeadline bounds one request's context, and it is the outermost of the
// system's time limits: everything a handler waits on has to fire before it, or
// the reason a request failed is "the deadline", which names nothing. It is
// also the only budget a connection acquisition has — see deadline.go.
//
// Exported because the ordering against the two database limits and the outward
// call is asserted rather than described:
// TestAnOutwardCallCannotOutlastTheTransactionItIsMadeIn compares all three.
// Below the server's WriteTimeout, whose default is 30s — that one is
// configurable, so it is not part of the assertion.
const RequestDeadline = 20 * time.Second

// Config holds the listener settings of a Server.
type Config struct {
	Port           string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	AllowedOrigins []string
}

// Server is the API's HTTP listener. Routes are registered on Router by the
// composition root before Run is called.
type Server struct {
	Router *chi.Mux

	server *http.Server
	logger *slog.Logger
}

// New builds a Server with the middleware stack every route shares: request
// ID, real client IP, structured access log, panic recovery, CORS.
func New(cfg Config, logger *slog.Logger) *Server {
	router := chi.NewRouter()

	router.Use(chimw.RequestID)
	// Before anything that can fail: the error writer reads the logger from the
	// request context in order to record what it withholds from the caller.
	router.Use(withLogger(logger))
	// No RealIP middleware: it rewrites RemoteAddr from headers any client can
	// forge (GHSA-3fxj-6jh8-hvhx). The day the client IP has to be trusted —
	// rate limiting, audit — it gets derived from an explicit allowlist of the
	// proxies in front of us, not from whatever X-Forwarded-For says.
	router.Use(requestLogger(logger))
	router.Use(recoverer(logger))
	router.Use(deadline(RequestDeadline))
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins: cfg.AllowedOrigins,
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
		// Callers authenticate with a Bearer token, never with cookies, so
		// credentialed cross-origin requests are never needed.
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Routing failures answer in the same shape as everything else, so a
	// generated client can decode one error type on every path.
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		WriteProblem(w, r, Problem{Status: http.StatusNotFound, Type: ProblemNotFound})
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		WriteProblem(w, r, Problem{Status: http.StatusMethodNotAllowed, Type: ProblemMethodNotAllowed})
	})

	return &Server{
		Router: router,
		server: &http.Server{
			Addr:         ":" + cfg.Port,
			Handler:      router,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
		logger: logger,
	}
}

// Run serves until ctx is cancelled or the listener fails, then drains
// in-flight requests within shutdownTimeout. Cancelling ctx is the only way to
// stop a Server; the caller owns the signal handling that cancels it.
func (s *Server) Run(ctx context.Context) error {
	// Owned by Run: started here, ends when the listener closes, and its only
	// error path is errCh, which Run below is guaranteed to be selecting on.
	errCh := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	s.logger.Info("server listening", "addr", s.server.Addr)

	select {
	case err := <-errCh:
		return fmt.Errorf("listening on %s: %w", s.server.Addr, err)

	case <-ctx.Done():
		s.logger.Info("server stopping")

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()

		if err := s.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutting down: %w", err)
		}

		s.logger.Info("server stopped")

		return nil
	}
}

// recoverer turns a panicking handler into a 500 in the API's error shape and
// logs the stack with the request ID. chi's own Recoverer is not used: it
// answers with an empty body, which breaks the one-error-shape contract.
func recoverer(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}
				if err, ok := recovered.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					// A deliberate abort, not a bug. Let net/http handle it.
					panic(recovered)
				}

				logger.Error(
					"panic recovered",
					"error", fmt.Sprint(recovered),
					"method", r.Method,
					"path", r.URL.Path,
					"request_id", chimw.GetReqID(r.Context()),
					"stack", string(debug.Stack()),
				)

				// A handler that already started responding cannot be given a
				// second status line.
				if ww, ok := w.(chimw.WrapResponseWriter); ok && ww.Status() != 0 {
					return
				}

				WriteProblem(w, r, Problem{Status: http.StatusInternalServerError, Type: ProblemInternal})
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// requestLogger emits one structured line per request, correlated by the
// request ID that chimw.RequestID put on the context.
func requestLogger(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			logger.Info(
				"request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration", time.Since(start).String(),
				"request_id", chimw.GetReqID(r.Context()),
			)
		})
	}
}

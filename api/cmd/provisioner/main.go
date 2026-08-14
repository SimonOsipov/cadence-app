// Command provisioner is the sole holder of the GoTrue admin key.
//
// Whoever holds that key can set any account's password and then obtain a
// session token by entirely legitimate means — measured, as a chain of three
// calls. What it buys is not data but the capture of a named doctor's identity,
// which is the nullification of audit attribution. So the key lives in this
// process and in no other, behind an exhaustive five operations, and the API
// talks to those five instead of to GoTrue's admin contract.
//
// It holds no database connection of any kind: TestTheComponentCannotReachTheDatabase
// asserts that transitively, because the 2026-08-14 decision to keep this
// component inside the api module gave up the compiler as the thing that
// proved it.
//
// # Deployment
//
// Neither a Dockerfile nor an App Platform manifest exists in this repository
// yet, so this block is the placement this component requires of whoever writes
// one. The platform is Timeweb App Platform (ADR-008):
//
//   - only the first service in the manifest is proxied outward, so
//     provisioner is never the first one. That it is unreachable from the
//     public name is a routing property, so it is verified by a probe against
//     the deployed harness — scripts/probe/provisioner-is-not-proxied.sh. The
//     probe has not been run: standing the deployment up is SKL-06, which only
//     a human can do. Until it has, this is the one claim about this component
//     that is stated and not measured.
//   - ports 80 and 443 are taken, hence PROVISIONER_PORT and its default.
//   - `volumes` are forbidden, which costs nothing here: this process keeps
//     no state at all.
//
// Its configuration is exactly the variables load() refuses to start without.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second

	// shutdownGrace is longer than one call to the identity provider, so a
	// redeploy does not leave an account created and an answer nobody received.
	shutdownGrace = 15 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(logger); err != nil {
		logger.Error("provisioner stopped", "error", err)
		os.Exit(1)
	}
}

// run lives apart from main so that os.Exit never skips a deferred cleanup.
func run(logger *slog.Logger) error {
	cfg, err := load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &server{
		environment: cfg.Environment,
		secrets:     cfg.Secrets,
		gotrue:      newGoTrueClient(cfg.GoTrueURL, cfg.Signer),
		logger:      logger,
	}

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.mux(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	served := make(chan error, 1)

	go func() {
		logger.Info("provisioner listening",
			"port", cfg.Port, "environment", cfg.Environment, "identity_provider", cfg.GoTrueURL)

		served <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-served:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serving: %w", err)
		}

		return nil

	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownGrace)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutting down: %w", err)
		}

		return nil
	}
}

// Command api is the Cadence API: one process, one deployment, one migration
// chain, with the eleven bounded contexts mounted on a single chi router.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/config"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
	"github.com/SimonOsipov/cadence-app/api/internal/router"
)

const (
	// databaseConnectTimeout bounds the startup connection attempt, so an
	// unreachable Postgres fails the deploy instead of hanging it.
	databaseConnectTimeout = 10 * time.Second

	// healthProbeTimeout bounds the /healthz probe. A Postgres that accepts the
	// connection but never answers has to produce a 503; without this, every
	// poll from the uptime monitor would park a goroutine instead.
	healthProbeTimeout = 2 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(logger); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

// run is the composition root: it is the only place that knows every
// component, and it holds no business rules of its own. It lives apart from
// main so that os.Exit never skips a deferred cleanup.
func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	connectCtx, cancelConnect := context.WithTimeout(ctx, databaseConnectTimeout)
	defer cancelConnect()

	pool, err := database.NewPool(connectCtx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	// Started before the listener so the key set is already being fetched when
	// the first request arrives. A provider that is unreachable right now does
	// not stop the process: every request is refused while that lasts, which is
	// correct, and a startup failure would turn a provider blip into a deploy
	// that needs a human.
	verifier, err := auth.NewVerifier(ctx, auth.VerifierConfig{
		Issuer:   cfg.Auth.Issuer,
		Audience: cfg.Auth.Audience,
		JWKSURL:  cfg.Auth.JWKSURL,
		Logger:   logger,
	})
	if err != nil {
		return fmt.Errorf("building the token verifier: %w", err)
	}

	srv := httpserver.New(httpserver.Config{
		Port:           cfg.Server.Port,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		IdleTimeout:    cfg.Server.IdleTimeout,
		AllowedOrigins: cfg.CORS.AllowedOrigins,
	}, logger)

	// One assembly, shared with the test that walks it: everything mounted is
	// guarded unless it is on the exemption list, and the list lives with the
	// transport paths it names.
	router.Mount(srv.Router, router.Options{
		Verifier: verifier,
		Probe: func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, healthProbeTimeout)
			defer cancel()

			return database.HealthCheck(ctx, pool)
		},
		Logger: logger,
	})

	if err := srv.Run(ctx); err != nil {
		return fmt.Errorf("running server: %w", err)
	}

	return nil
}

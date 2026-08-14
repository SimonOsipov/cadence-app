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

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth/token"
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

	// A second pool, not a second statement, and its own constructor rather than
	// NewPool with another URL. The boundary between the request path and the
	// service path runs along session_user: the roles they connect as can assume
	// different things, so a bug on one cannot reach the other's grants even
	// inside the same process. What NewServicePool adds on top of that is the two
	// time limits and a connection count somebody chose.
	servicePool, err := database.NewServicePool(connectCtx, cfg.Database.ServiceURL)
	if err != nil {
		return fmt.Errorf("connecting to database on the service path: %w", err)
	}
	defer servicePool.Close()

	// Asserted at startup rather than assumed from the connection strings. Two
	// variables in a store are two strings somebody can swap, and the symptom of
	// swapping them is not visible in any request.
	if err := database.VerifyPools(connectCtx, pool, servicePool); err != nil {
		return fmt.Errorf("verifying the connection pools: %w", err)
	}

	// Started before the listener so the key set is already being fetched when
	// the first request arrives. A provider that is unreachable right now does
	// not stop the process: every request is refused while that lasts, which is
	// correct, and a startup failure would turn a provider blip into a deploy
	// that needs a human.
	verifier, err := token.NewVerifier(ctx, token.VerifierConfig{
		Issuer:      cfg.Auth.Issuer,
		Audience:    cfg.Auth.Audience,
		JWKSURL:     cfg.Auth.JWKSURL,
		SessionKIDs: cfg.Auth.SessionKIDs,
		Logger:      logger,
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

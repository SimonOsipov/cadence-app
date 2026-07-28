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

	"github.com/SimonOsipov/cadence-app/api/internal/platform/config"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
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

	srv := httpserver.New(httpserver.Config{
		Port:           cfg.Server.Port,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		IdleTimeout:    cfg.Server.IdleTimeout,
		AllowedOrigins: cfg.CORS.AllowedOrigins,
	}, logger)

	// Unauthenticated by design: the external uptime monitor polls it.
	srv.Router.Get("/healthz", httpserver.Health(func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, healthProbeTimeout)
		defer cancel()

		return database.HealthCheck(ctx, pool)
	}, logger))

	// The bounded contexts mount their routers under /v1 from here.

	if err := srv.Run(ctx); err != nil {
		return fmt.Errorf("running server: %w", err)
	}

	return nil
}

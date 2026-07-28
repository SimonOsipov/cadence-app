package config

import (
	"slices"
	"testing"
	"time"
)

// clearEnv unsets every variable Load reads, so a developer shell that has
// api/.env loaded cannot change what the defaults test observes.
func clearEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"DATABASE_URL", "DATABASE_SERVICE_URL", "MIGRATIONS_PATH",
		"SERVER_PORT", "SERVER_READ_TIMEOUT", "SERVER_WRITE_TIMEOUT", "SERVER_IDLE_TIMEOUT",
		"CORS_ALLOWED_ORIGINS",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://cadence@localhost:5432/cadence")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Server.Port != "8080" {
		t.Errorf("Server.Port = %q, want 8080", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 10*time.Second {
		t.Errorf("Server.ReadTimeout = %v, want 10s", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 30*time.Second {
		t.Errorf("Server.WriteTimeout = %v, want 30s", cfg.Server.WriteTimeout)
	}
	if cfg.Server.IdleTimeout != 60*time.Second {
		t.Errorf("Server.IdleTimeout = %v, want 60s", cfg.Server.IdleTimeout)
	}
	if cfg.Database.MigrationsPath != "migrations" {
		t.Errorf("Database.MigrationsPath = %q, want migrations", cfg.Database.MigrationsPath)
	}
	if cfg.Database.ServiceURL != "" {
		t.Errorf("Database.ServiceURL = %q, want empty by default", cfg.Database.ServiceURL)
	}
	if got, want := cfg.CORS.AllowedOrigins, []string{"http://localhost:5173"}; !slices.Equal(got, want) {
		t.Errorf("CORS.AllowedOrigins = %v, want %v", got, want)
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://low@db/cadence")
	t.Setenv("DATABASE_SERVICE_URL", "postgres://service@db/cadence")
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("SERVER_READ_TIMEOUT", "5s")
	t.Setenv("SERVER_WRITE_TIMEOUT", "15s")
	t.Setenv("SERVER_IDLE_TIMEOUT", "45s")
	t.Setenv("MIGRATIONS_PATH", "db/migrations")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://dash.example.com, https://admin.example.com ,")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Database.URL != "postgres://low@db/cadence" {
		t.Errorf("Database.URL = %q", cfg.Database.URL)
	}
	if cfg.Database.ServiceURL != "postgres://service@db/cadence" {
		t.Errorf("Database.ServiceURL = %q", cfg.Database.ServiceURL)
	}
	if cfg.Server.Port != "9090" {
		t.Errorf("Server.Port = %q, want 9090", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 5*time.Second {
		t.Errorf("Server.ReadTimeout = %v, want 5s", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 15*time.Second {
		t.Errorf("Server.WriteTimeout = %v, want 15s", cfg.Server.WriteTimeout)
	}
	if cfg.Server.IdleTimeout != 45*time.Second {
		t.Errorf("Server.IdleTimeout = %v, want 45s", cfg.Server.IdleTimeout)
	}
	if cfg.Database.MigrationsPath != "db/migrations" {
		t.Errorf("Database.MigrationsPath = %q", cfg.Database.MigrationsPath)
	}

	want := []string{"https://dash.example.com", "https://admin.example.com"}
	if got := cfg.CORS.AllowedOrigins; !slices.Equal(got, want) {
		t.Errorf("CORS.AllowedOrigins = %v, want %v", got, want)
	}
}

func TestLoadRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "missing database URL",
			env:  map[string]string{},
		},
		{
			name: "unparsable read timeout",
			env: map[string]string{
				"DATABASE_URL":        "postgres://cadence@localhost/cadence",
				"SERVER_READ_TIMEOUT": "ten seconds",
			},
		},
		{
			name: "unparsable write timeout",
			env: map[string]string{
				"DATABASE_URL":         "postgres://cadence@localhost/cadence",
				"SERVER_WRITE_TIMEOUT": "-",
			},
		},
		{
			name: "unparsable idle timeout",
			env: map[string]string{
				"DATABASE_URL":        "postgres://cadence@localhost/cadence",
				"SERVER_IDLE_TIMEOUT": "forever",
			},
		},
		{
			// Falling back to the localhost default here would silently grant a
			// dev origin access to a production deployment.
			name: "allowed origins set but empty",
			env: map[string]string{
				"DATABASE_URL":         "postgres://cadence@localhost/cadence",
				"CORS_ALLOWED_ORIGINS": " , ,",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			if _, err := Load(); err == nil {
				t.Fatal("Load: want error, got nil")
			}
		})
	}
}

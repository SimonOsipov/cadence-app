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
		"DATABASE_URL", "DATABASE_SERVICE_URL", "DATABASE_MIGRATION_URL", "MIGRATIONS_PATH",
		"SERVER_PORT", "SERVER_READ_TIMEOUT", "SERVER_WRITE_TIMEOUT", "SERVER_IDLE_TIMEOUT",
		"CORS_ALLOWED_ORIGINS",
		"SUPABASE_JWT_ISSUER", "SUPABASE_JWT_AUDIENCE",
	} {
		t.Setenv(key, "")
	}
}

// setRequired fills in everything Load refuses to start without, so that a test
// about one variable is not really a test about the first one Load happens to
// check.
func setRequired(t *testing.T) {
	t.Helper()

	t.Setenv("DATABASE_URL", "postgres://cadence@localhost:5432/cadence")
	t.Setenv("SUPABASE_JWT_ISSUER", "https://ref.supabase.co/auth/v1")
	t.Setenv("SUPABASE_JWT_AUDIENCE", "authenticated")
}

func TestLoadAppliesDefaults(t *testing.T) {
	clearEnv(t)
	setRequired(t)

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
	if cfg.Database.ServiceURL != "" {
		t.Errorf("Database.ServiceURL = %q, want empty by default", cfg.Database.ServiceURL)
	}
	if got, want := cfg.CORS.AllowedOrigins, []string{"http://localhost:5173"}; !slices.Equal(got, want) {
		t.Errorf("CORS.AllowedOrigins = %v, want %v", got, want)
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	clearEnv(t)
	setRequired(t)
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

	want := []string{"https://dash.example.com", "https://admin.example.com"}
	if got := cfg.CORS.AllowedOrigins; !slices.Equal(got, want) {
		t.Errorf("CORS.AllowedOrigins = %v, want %v", got, want)
	}
}

func TestLoadMigrationReadsEnvironment(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_MIGRATION_URL", "postgres://owner@db/cadence")
	t.Setenv("MIGRATIONS_PATH", "db/migrations")

	cfg, err := LoadMigration()
	if err != nil {
		t.Fatalf("LoadMigration: %v", err)
	}

	if cfg.URL != "postgres://owner@db/cadence" {
		t.Errorf("URL = %q", cfg.URL)
	}
	if cfg.Path != "db/migrations" {
		t.Errorf("Path = %q", cfg.Path)
	}
}

func TestLoadMigrationAppliesDefaultPath(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_MIGRATION_URL", "postgres://owner@db/cadence")

	cfg, err := LoadMigration()
	if err != nil {
		t.Fatalf("LoadMigration: %v", err)
	}

	if cfg.Path != "migrations" {
		t.Errorf("Path = %q, want migrations", cfg.Path)
	}
}

// The migration URL is a role of its own — the owner of the schema, not the
// request path. Falling back to DATABASE_URL would apply the chain under the
// low-privilege role, which cannot create anything, and the failure would look
// like a permissions bug rather than a missing variable.
func TestLoadMigrationRequiresItsOwnURL(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://app@db/cadence")

	if _, err := LoadMigration(); err == nil {
		t.Fatal("LoadMigration: want error when DATABASE_MIGRATION_URL is unset, got nil")
	}
}

// TestLoadDerivesJWKSURL pins the rule that the JWKS address is not a variable
// of its own. A separate knob is a knob that can point somewhere else than the
// issuer being enforced, which is an authentication bypass that looks like a
// deployment setting.
func TestLoadDerivesJWKSURL(t *testing.T) {
	tests := []struct {
		name   string
		issuer string
		want   string
	}{
		{
			name:   "supabase issuer",
			issuer: "https://ref.supabase.co/auth/v1",
			want:   "https://ref.supabase.co/auth/v1/.well-known/jwks.json",
		},
		{
			// Supabase prints the issuer with the slash in some places and
			// without it in others; a copy-paste must not produce a doubled one.
			name:   "trailing slash is not doubled",
			issuer: "https://ref.supabase.co/auth/v1/",
			want:   "https://ref.supabase.co/auth/v1/.well-known/jwks.json",
		},
		{
			name:   "issuer at the root",
			issuer: "https://auth.example.com",
			want:   "https://auth.example.com/.well-known/jwks.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			setRequired(t)
			t.Setenv("SUPABASE_JWT_ISSUER", tt.issuer)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}

			if cfg.Auth.JWKSURL != tt.want {
				t.Errorf("Auth.JWKSURL = %q, want %q", cfg.Auth.JWKSURL, tt.want)
			}
		})
	}
}

// TestLoadKeepsIssuerVerbatim guards the other half of the derivation: the
// string compared against the token's `iss` claim is the one that was
// configured, not the normalised one used to build the JWKS address.
func TestLoadKeepsIssuerVerbatim(t *testing.T) {
	clearEnv(t)
	setRequired(t)
	t.Setenv("SUPABASE_JWT_ISSUER", "https://ref.supabase.co/auth/v1")
	t.Setenv("SUPABASE_JWT_AUDIENCE", "authenticated")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Auth.Issuer != "https://ref.supabase.co/auth/v1" {
		t.Errorf("Auth.Issuer = %q", cfg.Auth.Issuer)
	}
	if cfg.Auth.Audience != "authenticated" {
		t.Errorf("Auth.Audience = %q", cfg.Auth.Audience)
	}
}

func TestLoadRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "missing database URL",
			env:  map[string]string{"DATABASE_URL": ""},
		},
		{
			// Unset means "no authentication configured", and the one thing it
			// must never mean is "no authentication". Failing at startup is the
			// difference between a deployment that does not come up and a
			// deployment that comes up open.
			name: "missing JWT issuer",
			env:  map[string]string{"SUPABASE_JWT_ISSUER": ""},
		},
		{
			name: "missing JWT audience",
			env:  map[string]string{"SUPABASE_JWT_AUDIENCE": ""},
		},
		{
			// A relative or malformed issuer cannot produce a JWKS address, and
			// the failure has to happen here rather than as a fetch error on the
			// first request of the day.
			name: "issuer is not an absolute URL",
			env:  map[string]string{"SUPABASE_JWT_ISSUER": "ref.supabase.co/auth/v1"},
		},
		{
			name: "issuer is not http",
			env:  map[string]string{"SUPABASE_JWT_ISSUER": "postgres://ref.supabase.co/auth/v1"},
		},
		{
			name: "unparsable read timeout",
			env:  map[string]string{"SERVER_READ_TIMEOUT": "ten seconds"},
		},
		{
			name: "unparsable write timeout",
			env:  map[string]string{"SERVER_WRITE_TIMEOUT": "-"},
		},
		{
			name: "unparsable idle timeout",
			env:  map[string]string{"SERVER_IDLE_TIMEOUT": "forever"},
		},
		{
			// Falling back to the localhost default here would silently grant a
			// dev origin access to a production deployment.
			name: "allowed origins set but empty",
			env:  map[string]string{"CORS_ALLOWED_ORIGINS": " , ,"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			setRequired(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			if _, err := Load(); err == nil {
				t.Fatal("Load: want error, got nil")
			}
		})
	}
}

// Package config loads the API configuration from the process environment.
//
// Everything the API needs to run comes from environment variables: secrets
// never live in the repository, and dev and prod differ only by what Railway
// injects. Required variables are missing at startup, not at first use.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config is the fully resolved configuration of one API process.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	CORS     CORSConfig
	Auth     AuthConfig
}

// ServerConfig holds the HTTP listener settings.
type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// DatabaseConfig holds the two Postgres connection strings the API uses.
//
// URL is the request path: a low-privilege role that cannot bypass RLS, under
// which every request runs in a transaction carrying the caller's verified
// claims. ServiceURL is the service role and is reserved for system jobs —
// the reminder sweep, invitations, push fan-out — each of which writes to the
// audit log. It stays empty until the first such job exists.
type DatabaseConfig struct {
	URL        string
	ServiceURL string
}

// CORSConfig lists the origins allowed to call the API from a browser.
type CORSConfig struct {
	AllowedOrigins []string
}

// AuthConfig is everything the token check needs.
//
// Issuer and Audience are both required: unset has to mean "this process does
// not start", never "this process accepts anything". JWKSURL is derived rather
// than read, so that there is no variable capable of pointing the key source at
// one identity provider while the issuer check names another.
type AuthConfig struct {
	Issuer   string
	Audience string
	JWKSURL  string
}

// jwksSuffix is where an OAuth 2.0 authorisation server publishes its keys
// (RFC 8414). Supabase follows it: an issuer of https://<ref>.supabase.co/auth/v1
// serves its JWK Set at that path underneath.
const jwksSuffix = "/.well-known/jwks.json"

// Load reads the configuration from the environment and validates it.
// It returns an error if a required variable is unset or a duration is
// unparsable, so that a misconfigured process fails at startup.
func Load() (*Config, error) {
	readTimeout, err := durationEnv("SERVER_READ_TIMEOUT", 10*time.Second)
	if err != nil {
		return nil, err
	}

	writeTimeout, err := durationEnv("SERVER_WRITE_TIMEOUT", 30*time.Second)
	if err != nil {
		return nil, err
	}

	idleTimeout, err := durationEnv("SERVER_IDLE_TIMEOUT", 60*time.Second)
	if err != nil {
		return nil, err
	}

	databaseURL := getEnv("DATABASE_URL", "")
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	allowedOrigins, err := originsEnv("CORS_ALLOWED_ORIGINS", []string{"http://localhost:5173"})
	if err != nil {
		return nil, err
	}

	auth, err := loadAuth()
	if err != nil {
		return nil, err
	}

	return &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			IdleTimeout:  idleTimeout,
		},
		Database: DatabaseConfig{
			URL:        databaseURL,
			ServiceURL: getEnv("DATABASE_SERVICE_URL", ""),
		},
		CORS: CORSConfig{
			AllowedOrigins: allowedOrigins,
		},
		Auth: *auth,
	}, nil
}

// loadAuth reads the two Supabase variables and derives the JWKS address from
// the issuer.
//
// The issuer is kept exactly as configured — it is compared byte for byte
// against the `iss` claim, so normalising it here would quietly widen what the
// check accepts. Only the copy used to build the JWKS address is trimmed.
func loadAuth() (*AuthConfig, error) {
	issuer := getEnv("SUPABASE_JWT_ISSUER", "")
	if issuer == "" {
		return nil, errors.New("SUPABASE_JWT_ISSUER is required")
	}

	audience := getEnv("SUPABASE_JWT_AUDIENCE", "")
	if audience == "" {
		return nil, errors.New("SUPABASE_JWT_AUDIENCE is required")
	}

	parsed, err := url.Parse(issuer)
	if err != nil {
		return nil, fmt.Errorf("parsing SUPABASE_JWT_ISSUER: %w", err)
	}
	if parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, fmt.Errorf("SUPABASE_JWT_ISSUER must be an absolute http(s) URL, got %q", issuer)
	}

	return &AuthConfig{
		Issuer:   issuer,
		Audience: audience,
		JWKSURL:  strings.TrimSuffix(issuer, "/") + jwksSuffix,
	}, nil
}

// MigrationConfig is what the migration command needs, and nothing else.
//
// It is deliberately not part of Config: the API process must not be able to
// reach the owner role, and the migration command must not require the request
// path's variables to run.
type MigrationConfig struct {
	URL  string
	Path string
}

// LoadMigration reads the configuration of the migration command.
//
// DATABASE_MIGRATION_URL has no fallback to DATABASE_URL on purpose. The chain
// is applied by the role that owns the schema; the request path's role owns
// nothing and could not create a table if it tried.
func LoadMigration() (*MigrationConfig, error) {
	url := getEnv("DATABASE_MIGRATION_URL", "")
	if url == "" {
		return nil, errors.New("DATABASE_MIGRATION_URL is required")
	}

	return &MigrationConfig{
		URL:  url,
		Path: getEnv("MIGRATIONS_PATH", "migrations"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parsing %s: %w", key, err)
	}

	return d, nil
}

// originsEnv reads a comma-separated origin list. An unset variable takes the
// fallback; a variable that is set but lists nothing usable is an error rather
// than a silent fallback, so a mistyped production value cannot leave a
// development origin allowed.
func originsEnv(key string, fallback []string) ([]string, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}

	var origins []string
	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			origins = append(origins, s)
		}
	}
	if len(origins) == 0 {
		return nil, fmt.Errorf("%s is set but lists no origins", key)
	}

	return origins, nil
}

// Package config loads the API configuration from the process environment.
//
// Everything the API needs to run comes from environment variables: secrets
// never live in the repository, and dev and prod differ only by what Railway
// injects. Required variables are missing at startup, not at first use.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Config is the fully resolved configuration of one API process.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	CORS     CORSConfig
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
	URL            string
	ServiceURL     string
	MigrationsPath string
}

// CORSConfig lists the origins allowed to call the API from a browser.
type CORSConfig struct {
	AllowedOrigins []string
}

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

	return &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			IdleTimeout:  idleTimeout,
		},
		Database: DatabaseConfig{
			URL:            databaseURL,
			ServiceURL:     getEnv("DATABASE_SERVICE_URL", ""),
			MigrationsPath: getEnv("MIGRATIONS_PATH", "migrations"),
		},
		CORS: CORSConfig{
			AllowedOrigins: allowedOrigins,
		},
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

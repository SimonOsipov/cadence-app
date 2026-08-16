// Package config loads the API configuration from the process environment.
//
// Everything the API needs to run comes from environment variables: secrets
// never live in the repository, and dev and prod differ only by what the
// platform injects. Required variables are missing at startup, not at first use.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"
)

// Config is the fully resolved configuration of one API process.
type Config struct {
	Server      ServerConfig
	Database    DatabaseConfig
	CORS        CORSConfig
	Auth        AuthConfig
	Provisioner ProvisionerConfig
}

// ProvisionerConfig is how the API reaches the component that holds the admin
// key: an internal address and the shared secret that bounds who else can.
//
// The secret is not a defence against a compromised API — this process is its
// legitimate holder — and it is not the admin key. Creating, deleting and
// re-passwording accounts stays behind that component, which is the whole
// arrangement of the trust boundary.
type ProvisionerConfig struct {
	BaseURL string
	Secret  string
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
// claims. ServiceURL is the service path — a second role on a second pool, for
// the writes no policy lets a request make.
//
// Both are required. An empty ServiceURL was tempting while no system job
// existed, and it is the wrong shape: pgx falls through to libpq's defaults on
// an empty string, which resolves to the local user — in a test container, the
// superuser. That gives green tests and a different privilege domain in
// production, which is the failure mode this whole block is built to prevent.
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

	// SessionKIDs is the closed list of key ids GoTrue may sign a session
	// token with. It is mandatory rather than optional: an unset or empty
	// list would have to mean "trust whatever the key set publishes", which
	// is the door pinning exists to close. An empty list therefore fails
	// startup rather than falling back to "accept anything".
	//
	// AUTH_JWT_ADMIN_KID — the key id GoTrue's admin routes sign with, held
	// only by provisioner — is read and checked against this list inside
	// loadAuth, but deliberately does not appear as a field here. It exists
	// purely to let Load refuse a SessionKIDs value that names it: were the
	// admin key id ever a permitted session key id, a compromised provisioner
	// would turn its admin key into an accepted session token in one step,
	// which is the one barrier this pinning is meant to provide. Nothing past
	// startup validation needs the admin kid, and carrying it on this struct
	// would be an invitation to use it for something else — the API's own
	// stated direction is to hold as little of the admin side as possible.
	// The rotation order — how SessionKIDs and GoTrue's key material are
	// meant to change over the lifetime of a key — is documented on
	// token.VerifierConfig.SessionKIDs.
	SessionKIDs []string
}

// jwksSuffix is where an OAuth 2.0 authorisation server publishes its keys
// (RFC 8414). GoTrue follows it: an issuer of https://<host>/auth/v1 serves its
// JWK Set at that path underneath. Deriving rather than configuring is what
// makes the key source and the issuer check impossible to point at two
// different providers.
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

	serviceURL := getEnv("DATABASE_SERVICE_URL", "")
	if serviceURL == "" {
		return nil, errors.New("DATABASE_SERVICE_URL is required")
	}

	if serviceURL == databaseURL {
		return nil, errors.New(
			"DATABASE_SERVICE_URL and DATABASE_URL are the same connection string, " +
				"so the request path and the service path are one privilege domain",
		)
	}

	allowedOrigins, err := originsEnv("CORS_ALLOWED_ORIGINS", []string{"http://localhost:5173"})
	if err != nil {
		return nil, err
	}

	auth, err := loadAuth()
	if err != nil {
		return nil, err
	}

	provisioner, err := loadProvisioner()
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
			ServiceURL: serviceURL,
		},
		CORS: CORSConfig{
			AllowedOrigins: allowedOrigins,
		},
		Auth:        *auth,
		Provisioner: *provisioner,
	}, nil
}

// loadProvisioner reads where the provisioner component listens and the secret
// that reaches it.
//
// Both are required, and the secret is required in every environment: the
// component refuses a request without it, so an API started without one invites
// nobody — and would discover that at the first patient a doctor creates rather
// than at startup.
//
// What is deliberately absent is the admin key. It belongs to that component
// alone, and a gate rule fails the build when its variable is so much as named
// inside the API.
func loadProvisioner() (*ProvisionerConfig, error) {
	baseURL := strings.TrimSpace(getEnv("PROVISIONER_URL", ""))
	if baseURL == "" {
		return nil, errors.New("PROVISIONER_URL is required")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("PROVISIONER_URL is not a URL: %w", err)
	}

	if parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("PROVISIONER_URL must be an absolute http(s) address, got %q", baseURL)
	}

	secret := getEnv("PROVISIONER_SHARED_SECRET", "")
	if secret == "" {
		return nil, errors.New("PROVISIONER_SHARED_SECRET is required")
	}

	return &ProvisionerConfig{BaseURL: baseURL, Secret: secret}, nil
}

// loadAuth reads the authentication variables and derives the JWKS address
// from the issuer.
//
// The issuer is kept exactly as configured — it is compared byte for byte
// against the `iss` claim, so normalising it here would quietly widen what the
// check accepts. Only the copy used to build the JWKS address is trimmed.
func loadAuth() (*AuthConfig, error) {
	issuer := getEnv("AUTH_JWT_ISSUER", "")
	if issuer == "" {
		return nil, errors.New("AUTH_JWT_ISSUER is required")
	}

	audience := getEnv("AUTH_JWT_AUDIENCE", "")
	if audience == "" {
		return nil, errors.New("AUTH_JWT_AUDIENCE is required")
	}

	parsed, err := url.Parse(issuer)
	if err != nil {
		return nil, fmt.Errorf("parsing AUTH_JWT_ISSUER: %w", err)
	}
	if parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, fmt.Errorf("AUTH_JWT_ISSUER must be an absolute http(s) URL, got %q", issuer)
	}

	sessionKIDs, err := sessionKIDsEnv()
	if err != nil {
		return nil, err
	}

	if err := requireSessionKIDsExcludeTheAdminOne(sessionKIDs); err != nil {
		return nil, err
	}

	return &AuthConfig{
		Issuer:      issuer,
		Audience:    audience,
		JWKSURL:     strings.TrimSuffix(issuer, "/") + jwksSuffix,
		SessionKIDs: sessionKIDs,
	}, nil
}

// requireSessionKIDsExcludeTheAdminOne reads AUTH_JWT_ADMIN_KID and refuses a
// sessionKIDs list that names it.
//
// The admin key id is not carried past this function — see the comment on
// AuthConfig.SessionKIDs for why. It is trimmed the same way sessionKIDsEnv
// trims every entry of its own list: comparing an untrimmed admin kid against
// a trimmed session list would silently accept the exact configuration
// mistake this guard exists to catch — an admin kid that differs from a
// listed session kid only by surrounding whitespace — and a whitespace-only
// value would satisfy "required" while disarming the guard for every list.
func requireSessionKIDsExcludeTheAdminOne(sessionKIDs []string) error {
	adminKID := strings.TrimSpace(os.Getenv("AUTH_JWT_ADMIN_KID"))
	if adminKID == "" {
		return errors.New("AUTH_JWT_ADMIN_KID is required")
	}

	if slices.Contains(sessionKIDs, adminKID) {
		return fmt.Errorf(
			"AUTH_JWT_SESSION_KIDS names the admin key id %q: a compromised provisioner "+
				"could then turn its admin key into an accepted session token", adminKID,
		)
	}

	return nil
}

// sessionKIDsEnv reads the closed list of permitted session key ids.
//
// Unlike originsEnv, there is no fallback: a session-key allowlist that
// defaults to "accept anything published" is the exact door pinning exists to
// close, so an unset or empty value fails startup rather than falling back to
// anything.
func sessionKIDsEnv() ([]string, error) {
	raw := os.Getenv("AUTH_JWT_SESSION_KIDS")

	var kids []string
	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			kids = append(kids, s)
		}
	}
	if len(kids) == 0 {
		return nil, errors.New("AUTH_JWT_SESSION_KIDS is required and must name at least one permitted key id")
	}

	return kids, nil
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

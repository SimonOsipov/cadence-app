package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
)

// environment decides one thing: whether password setting exists at all.
type environment string

const (
	production  environment = "production"
	staging     environment = "staging"
	development environment = "development"
)

// Closed, so an unrecognised value fails startup rather than falling through to
// "not production" — the fall-through would mount password setting on a typo in
// the one deployment where it must not exist.
var environments = []environment{production, staging, development}

// The secret is the only thing in front of five operations on the clinic's
// accounts, so a value somebody could have typed is not one.
const minSecretLength = 32

const defaultPort = "8081"

// Config carries no admin key: it is read once, turned into a signer, and never
// held as text, because a struct carrying it is a struct one log line away from
// publishing the most dangerous value in the system.
type Config struct {
	Port        string
	Environment environment
	GoTrueURL   string
	Signer      *tokenSigner
	Secrets     secrets
}

// load requires every variable but one and names it in the error: this process
// holds the GoTrue admin key, and a misconfiguration that lets it start is
// worse than one that does not.
func load() (*Config, error) {
	env := environment(strings.TrimSpace(os.Getenv("PROVISIONER_ENVIRONMENT")))
	if !slices.Contains(environments, env) {
		return nil, fmt.Errorf("PROVISIONER_ENVIRONMENT must be one of %v, got %q", environments, env)
	}

	gotrueURL := strings.TrimSpace(os.Getenv("PROVISIONER_GOTRUE_URL"))
	if err := requireAbsoluteURL("PROVISIONER_GOTRUE_URL", gotrueURL); err != nil {
		return nil, err
	}

	signer, err := newTokenSigner(os.Getenv("PROVISIONER_GOTRUE_ADMIN_JWK"))
	if err != nil {
		return nil, fmt.Errorf("PROVISIONER_GOTRUE_ADMIN_JWK: %w", err)
	}

	current := os.Getenv("PROVISIONER_SHARED_SECRET")
	if err := requireSecret("PROVISIONER_SHARED_SECRET", current); err != nil {
		return nil, err
	}

	// The optional one: a deployment that has never rotated has one secret, and
	// requiring two would be satisfied by writing the same string twice —
	// which is why the two are also required to differ.
	previous := os.Getenv("PROVISIONER_SHARED_SECRET_PREVIOUS")
	if previous != "" {
		if err := requireSecret("PROVISIONER_SHARED_SECRET_PREVIOUS", previous); err != nil {
			return nil, err
		}

		if previous == current {
			return nil, errors.New(
				"PROVISIONER_SHARED_SECRET_PREVIOUS is the same value as PROVISIONER_SHARED_SECRET, " +
					"so a rotation that looks done has not started",
			)
		}
	}

	port := os.Getenv("PROVISIONER_PORT")
	if port == "" {
		port = defaultPort
	}

	return &Config{
		Port:        port,
		Environment: env,
		GoTrueURL:   gotrueURL,
		Signer:      signer,
		Secrets:     newSecrets(current, previous),
	}, nil
}

func requireAbsoluteURL(name, raw string) error {
	if raw == "" {
		return fmt.Errorf("%s is required", name)
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", name, err)
	}

	if parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("%s must be an absolute http(s) URL, got %q", name, raw)
	}

	return nil
}

func requireSecret(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}

	if len(value) < minSecretLength {
		return fmt.Errorf("%s is %d bytes; the shared secret must be at least %d",
			name, len(value), minSecretLength)
	}

	return nil
}

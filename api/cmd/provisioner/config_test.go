package main

import (
	"strings"
	"testing"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// setRequired puts a complete configuration in the environment, so that each
// case below breaks exactly one variable and a failure names it.
func setRequired(t *testing.T) {
	t.Helper()

	for name, value := range map[string]string{
		"PROVISIONER_ENVIRONMENT":            "development",
		"PROVISIONER_GOTRUE_URL":             "http://auth:9999",
		"PROVISIONER_GOTRUE_ADMIN_JWK":       testsupport.NewES256Key(t, "admin-key").PrivateJWK(t),
		"PROVISIONER_SHARED_SECRET":          testSecret,
		"PROVISIONER_SHARED_SECRET_PREVIOUS": testPreviousSecret,
	} {
		t.Setenv(name, value)
	}
}

func TestLoadAcceptsACompleteConfiguration(t *testing.T) {
	setRequired(t)

	cfg, err := load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.Environment != development {
		t.Errorf("environment = %q", cfg.Environment)
	}
	if cfg.GoTrueURL != "http://auth:9999" {
		t.Errorf("GoTrue URL = %q", cfg.GoTrueURL)
	}
}

// TestLoadRejectsBadInput. Each of these is a way the process could come up
// holding the admin key and being wrong about who may use it, which is worse
// than not coming up at all.
func TestLoadRejectsBadInput(t *testing.T) {
	tests := []struct {
		name  string
		set   map[string]string
		names string
	}{
		{
			name:  "no environment",
			set:   map[string]string{"PROVISIONER_ENVIRONMENT": ""},
			names: "PROVISIONER_ENVIRONMENT",
		},
		{
			// Fail closed: read as "not production", this typo would mount
			// password setting.
			name:  "an environment nobody declared",
			set:   map[string]string{"PROVISIONER_ENVIRONMENT": "prod"},
			names: "PROVISIONER_ENVIRONMENT",
		},
		{
			name:  "no identity provider",
			set:   map[string]string{"PROVISIONER_GOTRUE_URL": ""},
			names: "PROVISIONER_GOTRUE_URL",
		},
		{
			name:  "an identity provider that is not an address",
			set:   map[string]string{"PROVISIONER_GOTRUE_URL": "auth:9999"},
			names: "PROVISIONER_GOTRUE_URL",
		},
		{
			name:  "no admin key",
			set:   map[string]string{"PROVISIONER_GOTRUE_ADMIN_JWK": ""},
			names: "PROVISIONER_GOTRUE_ADMIN_JWK",
		},
		{
			name:  "no shared secret",
			set:   map[string]string{"PROVISIONER_SHARED_SECRET": ""},
			names: "PROVISIONER_SHARED_SECRET",
		},
		{
			// The guard is the only thing in front of five operations on the
			// clinic's accounts.
			name:  "a shared secret short enough to guess",
			set:   map[string]string{"PROVISIONER_SHARED_SECRET": "hunter2"},
			names: "PROVISIONER_SHARED_SECRET",
		},
		{
			// Equal values look like a rotation that happened and did not.
			name:  "two current secrets that are the same value",
			set:   map[string]string{"PROVISIONER_SHARED_SECRET_PREVIOUS": testSecret},
			names: "PROVISIONER_SHARED_SECRET_PREVIOUS",
		},
		{
			name:  "a previous secret short enough to guess",
			set:   map[string]string{"PROVISIONER_SHARED_SECRET_PREVIOUS": "hunter2"},
			names: "PROVISIONER_SHARED_SECRET_PREVIOUS",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setRequired(t)

			for name, value := range tc.set {
				t.Setenv(name, value)
			}

			_, err := load()
			if err == nil {
				t.Fatal("load accepted it")
			}

			if !strings.Contains(err.Error(), tc.names) {
				t.Errorf("the error does not name %s: %v", tc.names, err)
			}
		})
	}
}

// TestOneCurrentSecretIsEnough: a deployment that has never rotated has one
// secret, and requiring two would be answered by setting the same string twice.
func TestOneCurrentSecretIsEnough(t *testing.T) {
	setRequired(t)
	t.Setenv("PROVISIONER_SHARED_SECRET_PREVIOUS", "")

	cfg, err := load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.Secrets.permits(testPreviousSecret) {
		t.Error("a secret that was not configured is accepted")
	}
	if !cfg.Secrets.permits(testSecret) {
		t.Error("the configured secret is not accepted")
	}
}

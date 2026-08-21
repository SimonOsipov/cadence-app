package config

import (
	"strings"
	"testing"
)

func seedEnvironment(t *testing.T, environment string) {
	t.Helper()

	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://app@db/cadence")
	t.Setenv("DATABASE_SERVICE_URL", "postgres://service@db/cadence")
	t.Setenv("PROVISIONER_URL", "http://provisioner:8081")
	t.Setenv("PROVISIONER_SHARED_SECRET", strings.Repeat("s", 32))
	t.Setenv("SEED_PASSWORD", "a-seeded-password-nobody-uses")
	t.Setenv("SEED_ENVIRONMENT", environment)
}

func TestLoadSeedReadsWhatTheCommandNeeds(t *testing.T) {
	seedEnvironment(t, "development")

	cfg, err := LoadSeed()
	if err != nil {
		t.Fatalf("LoadSeed: %v", err)
	}

	if cfg.Database.URL == "" || cfg.Database.ServiceURL == "" {
		t.Errorf("the seed got no database to write to: %+v", cfg.Database)
	}
	if cfg.Provisioner.BaseURL == "" || cfg.Provisioner.Secret == "" {
		t.Errorf("the seed got no provisioner to create accounts through: %+v", cfg.Provisioner)
	}
	if cfg.Password != "a-seeded-password-nobody-uses" {
		t.Errorf("Password = %q", cfg.Password)
	}
	if cfg.Environment != Development {
		t.Errorf("Environment = %q, want %q", cfg.Environment, Development)
	}
}

// Unset must mean "this command does not run", never "not production": a
// fall-through would seed a clinic's real accounts on a typo.
func TestLoadSeedRefusesAnEnvironmentItDoesNotKnow(t *testing.T) {
	for name, value := range map[string]string{
		"unset":         "",
		"a typo":        "developement",
		"the API's own": "prod",
	} {
		t.Run(name, func(t *testing.T) {
			seedEnvironment(t, value)

			if _, err := LoadSeed(); err == nil {
				t.Errorf("SEED_ENVIRONMENT=%q was accepted", value)
			}
		})
	}
}

// The password is what the seeded people sign in with. A default here would be
// one credential shared by every environment that forgot to set it, committed
// in this repository.
func TestLoadSeedRequiresAPassword(t *testing.T) {
	seedEnvironment(t, "development")
	t.Setenv("SEED_PASSWORD", "")

	if _, err := LoadSeed(); err == nil {
		t.Error("the seed was configured with no password, so nobody it creates could sign in")
	}
}

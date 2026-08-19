package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Environment is which deployment a command is pointed at. The same closed set
// the provisioner reads from PROVISIONER_ENVIRONMENT, and closed for the same
// reason: an unrecognised value must fail rather than fall through to "not
// production".
type Environment string

const (
	Production  Environment = "production"
	Staging     Environment = "staging"
	Development Environment = "development"
)

var environments = []Environment{Production, Staging, Development}

// SeedConfig is what the seed command needs: two roles to write as, the
// component that creates accounts, the password the seeded people sign in with,
// and which deployment this is.
type SeedConfig struct {
	Database    DatabaseConfig
	Provisioner ProvisionerConfig
	Password    string
	Environment Environment
}

// LoadSeed reads the seed command's configuration.
//
// It refuses nothing about production itself — the command does, and says so in
// its own words. What this refuses is a value nobody can act on: an environment
// outside the set, or no password, which would seed a clinic whose people can
// none of them sign in.
func LoadSeed() (*SeedConfig, error) {
	database, err := loadDatabase()
	if err != nil {
		return nil, err
	}

	provisioner, err := loadProvisioner()
	if err != nil {
		return nil, err
	}

	password := strings.TrimSpace(getEnv("SEED_PASSWORD", ""))
	if password == "" {
		return nil, errors.New("SEED_PASSWORD is required: the people this creates sign in with it")
	}

	environment := Environment(strings.TrimSpace(getEnv("SEED_ENVIRONMENT", "")))
	if !slices.Contains(environments, environment) {
		return nil, fmt.Errorf("SEED_ENVIRONMENT must be one of %v, got %q", environments, environment)
	}

	return &SeedConfig{
		Database:    *database,
		Provisioner: *provisioner,
		Password:    password,
		Environment: environment,
	}, nil
}

package main

import (
	"os/exec"
	"strings"
	"testing"
)

// TestTheComponentCannotReachTheDatabase.
//
// The 2026-08-14 decision to keep provisioner inside the api module rather than
// in one of its own named exactly what it gives up: the compiler stops being
// the thing that proves this component cannot reach the database. Two checks
// replace it — step-6's grep rule on the admin key's variable, and this one.
//
// It is transitive on purpose. `depguard` and a reading of the import block
// both see only the direct edge, and the shortest way in is not one: a single
// import of internal/platform/database or internal/identity brings pgx, the
// pools, and the seam that sets a caller's claims.
func TestTheComponentCannotReachTheDatabase(t *testing.T) {
	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("listing dependencies: %v", err)
	}

	deps := strings.Split(strings.TrimSpace(string(out)), "\n")

	forbidden := []string{
		"github.com/jackc/pgx",
		"github.com/golang-migrate/migrate",
		"github.com/SimonOsipov/cadence-app/api/internal/platform/database",
		"github.com/SimonOsipov/cadence-app/api/internal/identity",
	}

	for _, dep := range deps {
		for _, prefix := range forbidden {
			if strings.HasPrefix(dep, prefix) {
				t.Errorf("provisioner compiles against %s — the holder of the admin key can reach the database", dep)
			}
		}
	}

	// The control: a dependency that must be there. Without it a `go list`
	// that returned nothing useful would pass every assertion above.
	if !strings.Contains(string(out), "github.com/golang-jwt/jwt/v5\n") {
		t.Error("provisioner does not depend on the JWT library at all, so this check read nothing")
	}
}

package identity_test

import (
	"os/exec"
	"strings"
	"testing"
)

// A bounded context needs the caller's identity, and until the token verifier
// was split out of the package that carries it, needing the identity meant
// compiling against a JWKS client and a JWT library. Eleven contexts were about
// to inherit that.
//
// It is not cosmetic. ADR-006 counts the replaceability of the identity provider
// as part of the bounded cost of a migration — "replacing it with our own token
// issuance is one implementation, not a sweeping edit" — and that is only true
// while the contexts import auth and nothing else.
//
// Asserted as a test rather than checked once by hand, because the failure is
// silent: a single import somewhere puts the dependency back and nothing else
// would notice. depguard cannot express it, because the dependency is
// transitive.
func TestTheContextDoesNotCompileAgainstTokenVerification(t *testing.T) {
	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("listing dependencies: %v", err)
	}

	forbidden := []string{
		"github.com/MicahParks/keyfunc",
		"github.com/MicahParks/jwkset",
		"github.com/golang-jwt/jwt",
	}

	for _, dep := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		for _, prefix := range forbidden {
			if strings.HasPrefix(dep, prefix) {
				t.Errorf("identity compiles against %s; the principal and the token verifier "+
					"have grown back together", dep)
			}
		}
	}

	// The control: a dependency that must be there. Without it a `go list` that
	// silently returned nothing would pass.
	if !strings.Contains(string(out), "internal/platform/auth\n") {
		t.Error("identity does not depend on the auth package at all, so this check read nothing")
	}
}

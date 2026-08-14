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

// The Provisioner interface is declared here and implemented elsewhere, and this
// is the direction of that arrangement asserted rather than intended.
//
// The client that speaks to the provisioner imports this package; nothing here
// imports it back. An import the other way would be this context compiling
// against HTTP, a shared secret and somebody else's wire format — and would take
// the interface's whole reason with it, since a consumer-declared interface that
// is only ever satisfied by a package it imports is a package boundary drawn on
// paper.
//
// What this adds today is a backstop rather than the guard, and the difference
// is worth stating: while the client carries `var _ identity.Provisioner`, an
// import from here is an import cycle and the compiler refuses it — measured,
// not assumed. It becomes the only check the moment that assertion goes, which
// is one deleted line: a second implementation, or a client that stops declaring
// which interface it satisfies. Then the cycle is gone and nothing but this
// notices the import.
func TestTheContextDoesNotCompileAgainstTheProvisionerClient(t *testing.T) {
	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("listing dependencies: %v", err)
	}

	for _, dep := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.HasSuffix(dep, "/internal/platform/provisioning") {
			t.Errorf("identity compiles against %s: the interface is declared here and the "+
				"implementation is supposed to be somebody else's problem", dep)
		}
	}

	// The control, again: a package this one certainly reaches. `go list` failing
	// quietly would otherwise make the loop above a walk over nothing.
	if !strings.Contains(string(out), "internal/platform/database\n") {
		t.Error("identity does not depend on the database package at all, so this check read nothing")
	}
}

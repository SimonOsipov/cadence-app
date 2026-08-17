package router

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The registry has to hold every bounded context, and the way to know is to
// look at the directories rather than at a number.
//
// The failure this catches is the twelfth context: a package added with its
// routes, its handlers and its passing unit tests, which no request ever
// reaches because nobody mounted it. Counting to eleven in an assertion would
// not catch it — the count would simply be updated along with the package.
func TestEveryBoundedContextIsRegistered(t *testing.T) {
	entries, err := os.ReadDir("..")
	if err != nil {
		t.Fatalf("reading internal/: %v", err)
	}

	// platform holds the transport, the config and the database; router is this
	// package. Neither is a bounded context and neither serves routes of its own.
	notContexts := []string{"platform", "router"}

	var found []string
	for _, entry := range entries {
		if !entry.IsDir() || slices.Contains(notContexts, entry.Name()) {
			continue
		}
		found = append(found, entry.Name())
	}

	var registered []string
	for _, c := range contexts(Options{}) {
		registered = append(registered, c.name)
	}

	slices.Sort(found)
	slices.Sort(registered)

	if !slices.Equal(found, registered) {
		t.Errorf("bounded contexts on disk and in the registry disagree:\n  on disk:     %v\n  registered:  %v",
			found, registered)
	}
}

// The name is a free-form string next to a function value, which is where a
// copy-paste slips: {"audit", content.Register} satisfies the registry test
// above while audit stays unmounted. The function knows which package it came
// from, so ask it rather than trusting the label.
func TestEachRegistrarBelongsToTheContextItIsNamedAfter(t *testing.T) {
	for _, c := range contexts(Options{}) {
		if c.register == nil {
			t.Errorf("bounded context %q has no registrar", c.name)

			continue
		}

		full := runtime.FuncForPC(reflect.ValueOf(c.register).Pointer()).Name()
		pkg := full
		if i := strings.LastIndex(full, "/"); i >= 0 {
			pkg = full[i+1:]
		}
		pkg, _, _ = strings.Cut(pkg, ".")

		if pkg != c.name {
			t.Errorf("context %q is registered by %s, which lives in package %q", c.name, full, pkg)
		}
	}
}

// The committed document is the contract two client surfaces are generated
// from. Regenerating it here means a Go type renamed in a refactor cannot
// rename a JSON field for the mobile app and the dashboard without the change
// appearing in the diff and being approved on purpose.
func TestCommittedDocumentIsUpToDate(t *testing.T) {
	generated, err := Document()
	if err != nil {
		t.Fatalf("building the document: %v", err)
	}

	path := filepath.Join("..", "..", "openapi.json")
	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	if string(generated) != string(committed) {
		t.Errorf("openapi.json is stale — the contract changed without being committed.\n" +
			"Run `make openapi` and review the diff: it is a change to what the mobile app\n" +
			"and the dashboard are generated from, not an incidental build artefact.")
	}
}

// A document that ends without a newline is a document that shows up as a
// one-line diff in every editor that adds one.
func TestDocumentEndsWithANewline(t *testing.T) {
	generated, err := Document()
	if err != nil {
		t.Fatalf("building the document: %v", err)
	}

	if len(generated) == 0 || generated[len(generated)-1] != '\n' {
		t.Error("the generated document does not end with a newline")
	}
}

// /healthz is served by chi directly and is not a huma operation, so it must
// not appear in the contract. A generated client that discovers it would start
// polling it with an Authorization header it does not need, against an endpoint
// that is deliberately open.
func TestHealthzIsNotInTheContract(t *testing.T) {
	generated, err := Document()
	if err != nil {
		t.Fatalf("building the document: %v", err)
	}

	if strings.Contains(string(generated), "/healthz") {
		t.Error("the contract describes /healthz, which is not part of the versioned API")
	}
}

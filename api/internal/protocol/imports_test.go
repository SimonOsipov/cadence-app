package protocol_test

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The direction is one-way by design: dosing knows about protocol, never the reverse. That
// is why OccurrencesFor takes a LoggedSlot instead of a dose event, and a cycle here would be
// a redesign wearing an import statement.
func TestProtocolDoesNotImportItsCallers(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("reading the package: %v", err)
	}

	callers := []string{
		"github.com/SimonOsipov/cadence-app/api/internal/dosing",
		"github.com/SimonOsipov/cadence-app/api/internal/inventory",
		"github.com/SimonOsipov/cadence-app/api/internal/journal",
	}
	// XTestImports too: an external test file is part of this package's dependency graph,
	// and leaving it out would exempt every future one from the check.
	all := append(append(append([]string{}, pkg.Imports...), pkg.TestImports...), pkg.XTestImports...)
	for _, imported := range all {
		for _, caller := range callers {
			if imported == caller {
				t.Errorf("protocol imports %s", imported)
			}
		}
	}
}

// The transport files, named as the exception rather than the generator being named as the
// rule: a list of generator files is a list a new generator file steps around, and this check
// has to keep meaning something after routes.go and the handlers of step 6 arrive.
var transport = map[string]bool{
	"routes.go":  true,
	"doc.go":     true,
	"handler.go": true,
}

// Purity is what makes the suite able to run this against a calendar: today arrives as a
// parameter, and no query, no request and no clock reading reaches the generator.
func TestTheGeneratorItselfTouchesNoDatabaseAndNoClock(t *testing.T) {
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing the package: %v", err)
	}

	checked := 0
	for _, name := range names {
		if transport[name] || strings.HasSuffix(name, "_test.go") {
			continue
		}
		checked++
		assertPure(t, name)
	}
	// Without this the guard passes an empty package: a rename of every generator file, or
	// a glob that stops matching, would read exactly like purity.
	if checked < 6 {
		t.Fatalf("expected the six generator files, walked %d", checked)
	}
}

func assertPure(t *testing.T, name string) {
	t.Helper()

	source, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), name, source, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}

	// time is not on this list and cannot be: Date is built out of it. The clock is caught
	// below, at the call rather than at the import.
	impure := []string{
		"context",
		"database/sql",
		"net/http",
		"github.com/jackc/pgx",
		"github.com/danielgtaylor/huma",
		"github.com/SimonOsipov/cadence-app/api/internal/platform/database",
	}
	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, `"`)
		for _, deny := range impure {
			if path == deny || strings.HasPrefix(path, deny+"/") {
				t.Errorf("%s imports %s", name, path)
			}
		}
	}

	clock := map[string]bool{"Now": true, "Since": true, "Until": true}
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok || pkg.Name != "time" || !clock[selector.Sel.Name] {
			return true
		}
		t.Errorf("%s reads the clock: time.%s", name, selector.Sel.Name)
		return true
	})
}

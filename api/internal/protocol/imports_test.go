package protocol_test

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
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

// The generator, named as the subject of the claim rather than as a filter. A list of
// *transport* files would be the wrong shape: this project puts one aggregate per file with
// the check, the transaction and the SQL mixed in — internal/identity has twelve such files
// and only one of them is called handler.go — so step 6's protocols.go would fail a check it
// conforms to, and the fix would be to keep widening the exemption.
//
// A file added here and not listed is caught by the floor below, not by this list.
var generator = []string{
	"calendar.go",
	"types.go",
	"occurrence.go",
	"titration.go",
	"bands.go",
	"row.go",
}

// No query and no request reaches the ported functions.
func TestTheGeneratorImportsNoDatabaseAndNoTransport(t *testing.T) {
	impure := []string{
		"context",
		"database/sql",
		"net/http",
		"github.com/jackc/pgx",
		"github.com/danielgtaylor/huma",
		"github.com/SimonOsipov/cadence-app/api/internal/platform/database",
	}

	for _, name := range generator {
		file := parse(t, name)
		for _, spec := range file.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			for _, deny := range impure {
				if path == deny || strings.HasPrefix(path, deny+"/") {
					t.Errorf("%s imports %s", name, path)
				}
			}
		}
	}
}

// Every non-test file in the package, transport included — a helper reading the clock in
// routes.go and called from occurrence.go is the same defect one indirection away, and
// exempting a file from the import check must not exempt it from this one.
//
// today arrives as a parameter, which is what lets the suite run the generator against a
// calendar instead of against whatever day it is.
func TestNothingInThePackageReadsTheClock(t *testing.T) {
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing the package: %v", err)
	}

	checked := 0
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		checked++
		assertNoClock(t, name)
	}
	// Without this the guard passes an empty package: a rename of every file, or a glob that
	// stops matching, would read exactly like purity.
	if checked < len(generator) {
		t.Fatalf("expected at least the %d generator files, walked %d", len(generator), checked)
	}
}

func parse(t *testing.T, name string) *ast.File {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	return file
}

func assertNoClock(t *testing.T, name string) {
	t.Helper()
	file := parse(t, name)

	// The local name of the time package, not the string "time": `import clock "time"` would
	// otherwise walk straight through.
	local := ""
	for _, spec := range file.Imports {
		if strings.Trim(spec.Path.Value, `"`) != "time" {
			continue
		}
		local = "time"
		if spec.Name != nil {
			local = spec.Name.Name
		}
	}
	if local == "" || local == "_" {
		return
	}

	reads := map[string]bool{
		"Now": true, "Since": true, "Until": true,
		"After": true, "Tick": true, "NewTimer": true, "NewTicker": true, "AfterFunc": true,
	}
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok || pkg.Name != local || !reads[selector.Sel.Name] {
			return true
		}
		t.Errorf("%s reads the clock: %s.%s", name, local, selector.Sel.Name)
		return true
	})
}

package inventory_test

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// inventory reads protocol and must not be read by it: the reorder hint needs a
// prescription's weekly rate, and nothing in the prescription needs a vial. The
// other half of this rule lives in protocol's own guard, which refuses the reverse.
//
// dosing is the one it must not learn about, for the same reason protocol must not:
// RemainingDoses takes the vials doses came out of, not the dose events, so the
// import stays one-way and a vial can be counted without a transaction.
func TestInventoryReadsTheProtocolAndNotItsCallers(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("reading the package: %v", err)
	}

	const prefix = "github.com/SimonOsipov/cadence-app/api/internal/"
	callers := []string{"dosing", "journal", "measurements", "nutrition"}

	all := append(append(append([]string{}, pkg.Imports...), pkg.TestImports...), pkg.XTestImports...)
	for _, imported := range all {
		for _, caller := range callers {
			if imported == prefix+caller {
				t.Errorf("inventory imports %s", caller)
			}
		}
	}

	// The positive half, so the guard cannot pass on a package that stopped reading
	// the protocol at all — which is how the reorder hint would lose its rate.
	if !slices.Contains(pkg.Imports, prefix+"protocol") {
		t.Error("inventory no longer reads protocol; the weekly rate has to come from somewhere")
	}
}

// The math is pure: no query, no request, no clock. today arrives as a parameter,
// which is what lets the suite run an expiry against a calendar.
//
// Named as the subject of the claim rather than filtered, and reconciled against the
// directory below, for the reason protocol's guard records: a list is a thing new
// files are absent from, and absence there means exemption.
var (
	math = []string{"math.go"}
	// Everything deliberately not the pure half. vials.go reads the cabinet for the
	// context that draws a dose from it, and this project puts the check, the transaction
	// and the SQL in one file rather than behind a repository.
	transport = []string{"routes.go", "doc.go", "vials.go", "supply.go"}
)

func TestEveryFileInThePackageIsClassified(t *testing.T) {
	onDisk, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing the package: %v", err)
	}

	classified := map[string]bool{}
	for _, name := range append(append([]string{}, math...), transport...) {
		classified[name] = true
	}

	seen := 0
	for _, name := range onDisk {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		seen++
		if !classified[name] {
			t.Errorf("%s is in neither math nor transport: classify it", name)
		}
		delete(classified, name)
	}
	for name := range classified {
		t.Errorf("%s is listed but not on disk", name)
	}
	if seen != len(math)+len(transport) {
		t.Errorf("walked %d files, the two lists name %d", seen, len(math)+len(transport))
	}
}

func TestTheMathTouchesNoDatabaseAndNoClock(t *testing.T) {
	impure := []string{
		"context",
		"database/sql",
		"net/http",
		"github.com/jackc/pgx",
		"github.com/danielgtaylor/huma",
		"github.com/SimonOsipov/cadence-app/api/internal/platform/database",
	}

	for _, name := range math {
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		for _, spec := range file.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			for _, deny := range impure {
				if path == deny || strings.HasPrefix(path, deny+"/") {
					t.Errorf("%s imports %s", name, path)
				}
			}
		}
	}

	// Every non-test file, transport included: a clock read in routes.go and called
	// from math.go is the same defect one indirection away.
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing the package: %v", err)
	}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		assertNoClock(t, name)
	}
}

func assertNoClock(t *testing.T, name string) {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}

	// The local name of the time package, not the string: `import clock "time"` would
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
	if local == "." {
		t.Fatalf("%s dot-imports time; the clock check cannot see through it", name)
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

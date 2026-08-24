package journal_test

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// The diary is written by the dose path, not the other way round: one patient action
// writes a dose event and this row, so dosing reads journal. That is why the closed
// set of tags lives here — in the KMP it is a typealias onto SideEffect,
// declared in Journal.kt, which Kotlin can afford because it has no import direction to protect.
func TestJournalDoesNotImportItsCallers(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("reading the package: %v", err)
	}

	const prefix = "github.com/SimonOsipov/cadence-app/api/internal/"
	callers := []string{"dosing", "measurements", "nutrition", "messaging"}

	// protocol among them, and that is the point of the civil package: the diary
	// imported the prescription for nothing but Date and UserID, which is a shared
	// kernel by accident. Nothing about a day's mood depends on a course.
	callers = append(callers, "protocol")

	all := append(append(append([]string{}, pkg.Imports...), pkg.TestImports...), pkg.XTestImports...)
	for _, imported := range all {
		for _, caller := range callers {
			if imported == prefix+caller {
				t.Errorf("journal imports %s", caller)
			}
		}
	}
}

// The merge is pure: no query, no request, no clock. The day it is about arrives on
// the draft, which is what lets the suite run it against a calendar.
var (
	// parse.go joins the merge rather than the transport: it is the vocabulary the merge
	// is written in, and nothing in it knows about HTTP — the seed is its other caller.
	merge     = []string{"merge.go", "parse.go"}
	transport = []string{"routes.go", "doc.go"}
)

func TestEveryFileInThePackageIsClassified(t *testing.T) {
	onDisk, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing the package: %v", err)
	}

	classified := map[string]bool{}
	for _, name := range append(append([]string{}, merge...), transport...) {
		classified[name] = true
	}

	seen := 0
	for _, name := range onDisk {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		seen++
		if !classified[name] {
			t.Errorf("%s is in neither merge nor transport: classify it", name)
		}
		delete(classified, name)
	}
	for name := range classified {
		t.Errorf("%s is listed but not on disk", name)
	}
	if seen != len(merge)+len(transport) {
		t.Errorf("walked %d files, the two lists name %d", seen, len(merge)+len(transport))
	}
}

func TestTheMergeTouchesNoDatabaseAndNoClock(t *testing.T) {
	impure := []string{
		"context",
		"database/sql",
		"net/http",
		"github.com/jackc/pgx",
		"github.com/danielgtaylor/huma",
		"github.com/SimonOsipov/cadence-app/api/internal/platform/database",
	}

	for _, name := range merge {
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
	// from merge.go is the same defect one indirection away.
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

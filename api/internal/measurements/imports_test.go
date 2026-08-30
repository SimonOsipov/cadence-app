package measurements_test

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const module = "github.com/SimonOsipov/cadence-app/api/internal/"

// protocol is the one context this one reaches across for — the last course, whose geometry
// the cycle window is, and the titrating position the overlay follows. Everything else in
// internal/ is denied, and the denial is reconciled against the directory rather than listed:
// a context added next month is absent from a list, and absence there reads as exemption.
//
// platform holds the shared kernel and router is the composition root that mounts this
// package; an import of either is not a boundary crossing.
func TestMeasurementsImportsNoContextButProtocol(t *testing.T) {
	// The integration tag, or the guard reads none of the files likeliest to reach across a
	// boundary: build.ImportDir's default context drops every //go:build integration file,
	// and measurements keeps its policy suite behind exactly that tag.
	tagged := build.Default
	tagged.BuildTags = []string{"integration"}

	pkg, err := tagged.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("reading the package: %v", err)
	}

	contexts, err := os.ReadDir("..")
	if err != nil {
		t.Fatalf("reading internal/: %v", err)
	}

	var denied []string
	for _, entry := range contexts {
		if !entry.IsDir() {
			continue
		}
		switch entry.Name() {
		case "measurements", "protocol", "platform", "router":
			continue
		}
		denied = append(denied, entry.Name())
	}

	// The set and not its size: a ReadDir that came back empty, or a skip widened by one
	// case too many, reads exactly like a package that imports nothing. A twelfth context
	// lands here as a failure asking to be classified, which is the point.
	if want := []string{
		"audit", "content", "dosing", "identity", "inventory",
		"journal", "messaging", "notifications", "nutrition",
	}; !slices.Equal(denied, want) {
		t.Fatalf("the denied contexts are %v, not %v", denied, want)
	}

	// XTestImports too: an external test file is part of this package's dependency graph,
	// and leaving it out would exempt every future one from the check.
	all := append(append(append([]string{}, pkg.Imports...), pkg.TestImports...), pkg.XTestImports...)
	for _, imported := range all {
		for _, name := range denied {
			if imported == module+name {
				t.Errorf("measurements imports %s", name)
			}
		}
	}
}

// The two halves of the package, reconciled against the directory below rather than merely
// listed, for the reason protocol's own guard gives: a list is a thing new files are absent
// from, and absence there means exemption.
var (
	// The half that is a function of its arguments: the closed sets, the parser, the table
	// of units, directions and thresholds, and the windows; from steps 4 and 5 the series and
	// the overlay's choice of position. No clock, no query, no request.
	arithmetic = []string{"types.go", "parse.go", "meta.go", "window.go", "series.go", "overlay.go"}
	// Everything that is deliberately not it. Named for what it is rather than «transport»,
	// which the reads and the write of step 6 are not either: they take a transaction and
	// know nothing of HTTP.
	notTheArithmetic = []string{"routes.go", "doc.go", "read.go", "write.go"}
)

func TestEveryFileInThePackageIsClassified(t *testing.T) {
	onDisk, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing the package: %v", err)
	}

	classified := map[string]bool{}
	for _, name := range append(append([]string{}, arithmetic...), notTheArithmetic...) {
		classified[name] = true
	}

	seen := 0
	for _, name := range onDisk {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		seen++
		if !classified[name] {
			t.Errorf("%s is in neither list: classify it", name)
		}
		delete(classified, name)
	}
	for name := range classified {
		t.Errorf("%s is listed but not on disk", name)
	}
	if seen != len(arithmetic)+len(notTheArithmetic) {
		t.Errorf("walked %d files, the two lists name %d", seen, len(arithmetic)+len(notTheArithmetic))
	}
}

// No query and no request reaches it: the same functions are read by the seed, by the
// transport and by each other, and a connection inside them would make the unit of a weight a
// thing that can fail.
func TestTheArithmeticImportsNoDatabaseAndNoTransport(t *testing.T) {
	impure := []string{
		"context",
		"database/sql",
		"net/http",
		"github.com/jackc/pgx",
		"github.com/danielgtaylor/huma",
		module + "platform/database",
	}

	for _, name := range arithmetic {
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

// Every non-test file, transport included — a helper reading the clock in routes.go and called
// from the window arithmetic is the same defect one indirection away. «Today» arrives as a
// parameter, which is what lets the suite run the windows against a calendar.
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
	if checked < len(arithmetic)+len(notTheArithmetic) {
		t.Fatalf("expected %d files, walked %d", len(arithmetic)+len(notTheArithmetic), checked)
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
	if local == "." {
		// Nothing below can see it: Now() under a dot-import is a bare identifier, not a
		// selector. Refuse the form rather than pass a file the check cannot read.
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

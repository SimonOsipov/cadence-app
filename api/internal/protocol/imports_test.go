package protocol_test

import (
	"go/build"
	"go/parser"
	"go/token"
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
	for _, imported := range append(pkg.Imports, pkg.TestImports...) {
		for _, caller := range callers {
			if imported == caller {
				t.Errorf("protocol imports %s", imported)
			}
		}
	}
}

// Asserted per file rather than per package on purpose: routes.go and the handlers that
// arrive with the first endpoint will import huma, and a package-wide check would either
// fail on them or have to be deleted. What has to stay pure is the generator itself — today
// arrives as a parameter, which is what lets the suite run it against a calendar instead of
// a clock, and no query reaches it.
func TestTheGeneratorItselfTouchesNoDatabaseAndNoClock(t *testing.T) {
	generator := []string{
		"calendar.go",
		"types.go",
		"occurrence.go",
		"titration.go",
		"bands.go",
		"row.go",
	}
	impure := []string{
		"context",
		"database/sql",
		"github.com/jackc/pgx",
		"github.com/danielgtaylor/huma",
		"github.com/SimonOsipov/cadence-app/api/internal/platform/database",
	}

	for _, name := range generator {
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
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
}

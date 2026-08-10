package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// analyse writes one file into a throwaway package directory and runs the check
// over it. Working from source text rather than from fixtures on disk keeps each
// case readable next to its expectation, which is the whole point of a rule this
// small: if the case is hard to read, the rule is wrong.
func analyse(t *testing.T, source string) []Finding {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "probe.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("writing the probe source: %v", err)
	}

	report, err := Check(dir, nil)
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if report.Files != 1 {
		t.Fatalf("read %d files, want 1 — the case was not looked at", report.Files)
	}

	return report.Findings
}

const probeHeader = `package probe

import (
	"context"
	"fmt"
)

const roleName = "cadence_patient"

type conn struct{}

func (conn) Exec(ctx context.Context, sql string, args ...any) error     { return nil }
func (conn) Query(ctx context.Context, sql string, args ...any) error    { return nil }
func (conn) QueryRow(ctx context.Context, sql string, args ...any) error { return nil }
func (conn) SendBatch(ctx context.Context, b any) error                  { return nil }
func (conn) Queue(sql string, args ...any)                               {}
func (conn) Prepare(ctx context.Context, name, sql string) error         { return nil }

var _ = fmt.Sprint
`

func TestAcceptsExpressionsBuiltOnlyFromConstants(t *testing.T) {
	cases := map[string]string{
		"a string literal": `
			func f(ctx context.Context, c conn) { _ = c.Exec(ctx, "SELECT 1") }`,
		"a package constant": `
			const stmt = "SELECT 1"
			func f(ctx context.Context, c conn) { _ = c.Exec(ctx, stmt) }`,
		"a literal joined to a package constant": `
			func f(ctx context.Context, c conn) { _ = c.Exec(ctx, "SET LOCAL ROLE " + roleName) }`,
		"a parenthesised constant expression": `
			func f(ctx context.Context, c conn) { _ = c.Exec(ctx, ("SELECT " + "1")) }`,
		"a queued literal": `
			func f(c conn) { c.Queue("SELECT 1") }`,
		"a prepared literal": `
			func f(ctx context.Context, c conn, name string) { _ = c.Prepare(ctx, name, "SELECT 1") }`,
		"a constant declared inside the function": `
			func f(ctx context.Context, c conn) {
				const stmt = "SELECT 1"
				_ = c.Exec(ctx, stmt)
			}`,
		// The context is found by position, not by name, so a request-scoped
		// context called anything at all leaves the statement in view.
		"a context that is not called ctx": `
			func f(reqCtx context.Context, c conn) { _ = c.Exec(reqCtx, "SELECT 1") }`,
		// SendBatch carries a *pgx.Batch and no statement text; the text enters
		// through Queue. Checking its argument would fail every batch there is.
		"a batch handed to SendBatch": `
			func f(ctx context.Context, c conn, b any) { _ = c.SendBatch(ctx, b) }`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if findings := analyse(t, probeHeader+body); len(findings) != 0 {
				t.Errorf("%v, want none", findings)
			}
		})
	}
}

func TestRejectsExpressionsAVariableCanReach(t *testing.T) {
	cases := map[string]struct {
		body     string
		call     string
		wantExpr string
	}{
		"a parameter concatenated in": {
			body: `
				func f(ctx context.Context, c conn, role string) {
					_ = c.Exec(ctx, "SET LOCAL ROLE " + role)
				}`,
			call:     "Exec",
			wantExpr: "role",
		},
		"a formatted string": {
			body: `
				func f(ctx context.Context, c conn, table string) {
					_ = c.Query(ctx, fmt.Sprintf("SELECT * FROM %s", table))
				}`,
			call:     "Query",
			wantExpr: "fmt.Sprintf",
		},
		"a local variable, even one holding a literal": {
			body: `
				func f(ctx context.Context, c conn) {
					stmt := "SELECT 1"
					_ = c.QueryRow(ctx, stmt)
				}`,
			call:     "QueryRow",
			wantExpr: "stmt",
		},
		"a queued variable": {
			body: `
				func f(c conn, stmt string) { c.Queue(stmt) }`,
			call:     "Queue",
			wantExpr: "stmt",
		},
		// Prepare sends real SQL to the server, to be executed later by name. A
		// statement composed there and run through Exec(ctx, name) would pass
		// every other check in this file.
		"a prepared statement composed from a parameter": {
			body: `
				func f(ctx context.Context, c conn, role string) {
					_ = c.Prepare(ctx, "p1", "SET ROLE " + role)
				}`,
			call:     "Prepare",
			wantExpr: "role",
		},
		// The context under another name must not become the argument under
		// examination: the statement, not the context, is what has to be read.
		"a concatenation behind a context that is not called ctx": {
			body: `
				func f(reqCtx context.Context, c conn, role string) {
					_ = c.Exec(reqCtx, "SET ROLE " + role)
				}`,
			call:     "Exec",
			wantExpr: "role",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			findings := analyse(t, probeHeader+tc.body)
			if len(findings) != 1 {
				t.Fatalf("got %d findings %v, want exactly 1", len(findings), findings)
			}
			if findings[0].Call != tc.call {
				t.Errorf("finding names call %q, want %q", findings[0].Call, tc.call)
			}
			if !strings.Contains(findings[0].Expr, tc.wantExpr) {
				t.Errorf("finding reports %q, want it to name %q — otherwise it may have read "+
					"the wrong argument and still looked right", findings[0].Expr, tc.wantExpr)
			}
			if findings[0].Line == 0 {
				t.Error("finding carries no line, so it cannot be acted on")
			}
		})
	}
}

// A local name that happens to match a package constant is a local name. Without
// this the rule lifts itself by accident: a package-level `const query` and a
// `query :=` anywhere in the same file, and the statement stops being read.
func TestALocalNameShadowsAPackageConstant(t *testing.T) {
	findings := analyse(t, probeHeader+`
		const stmt = "SELECT 1"

		func f(ctx context.Context, c conn, role string) {
			stmt := fmt.Sprintf("SET ROLE %s", role)
			_ = c.Exec(ctx, stmt)
		}`)
	if len(findings) != 1 {
		t.Fatalf("got %v, want the shadowed name reported", findings)
	}
	if !strings.Contains(findings[0].Expr, "stmt") {
		t.Errorf("finding reports %q, want it to name the expression", findings[0].Expr)
	}
}

// A local variable holding a literal is rejected on purpose, and this states
// why: the rule is about the shape a value can take, not about the value it
// happens to hold today. The variable is where the next edit puts a parameter.
func TestRejectionIsAboutShapeRatherThanTodaysValue(t *testing.T) {
	findings := analyse(t, probeHeader+`
		func f(ctx context.Context, c conn) {
			stmt := "SELECT 1"
			_ = c.Exec(ctx, stmt)
		}`)
	if len(findings) != 1 {
		t.Fatalf("got %v, want the local variable reported", findings)
	}
	if !strings.Contains(findings[0].Expr, "stmt") {
		t.Errorf("finding reports %q, want it to name the expression", findings[0].Expr)
	}
}

func TestSkipsTestFilesAndNamedDirectories(t *testing.T) {
	dir := t.TempDir()

	offending := probeHeader + `
		func f(ctx context.Context, c conn, role string) { _ = c.Exec(ctx, "SET ROLE " + role) }`
	clean := probeHeader + `
		func f(ctx context.Context, c conn) { _ = c.Exec(ctx, "SELECT 1") }`

	if err := os.WriteFile(filepath.Join(dir, "probe_test.go"), []byte(offending), 0o600); err != nil {
		t.Fatalf("writing the test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "probe.go"), []byte(clean), 0o600); err != nil {
		t.Fatalf("writing the package source: %v", err)
	}

	harness := filepath.Join(dir, "testsupport")
	if err := os.Mkdir(harness, 0o750); err != nil {
		t.Fatalf("creating the harness directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(harness, "probe.go"), []byte(offending), 0o600); err != nil {
		t.Fatalf("writing the harness source: %v", err)
	}

	report, err := Check(dir, []string{"testsupport"})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if len(report.Findings) != 0 {
		t.Errorf("%v, want none: a _test.go file and a skipped directory are both out of scope",
			report.Findings)
	}
	if report.Files != 1 {
		t.Errorf("read %d files, want 1 — the count is what tells an operator "+
			"a clean result was earned rather than skipped into", report.Files)
	}
	if len(report.Skipped) != 1 || !strings.HasSuffix(report.Skipped[0], "testsupport") {
		t.Errorf("skipped %v, want the harness directory named", report.Skipped)
	}
}

// The root is never skipped by name. Otherwise `-skip internal ./internal` walks
// nothing, finds nothing, and is indistinguishable from a clean tree.
func TestTheRootIsNeverSkipped(t *testing.T) {
	dir := t.TempDir()

	root := filepath.Join(dir, "internal")
	if err := os.Mkdir(root, 0o750); err != nil {
		t.Fatalf("creating the root: %v", err)
	}
	source := probeHeader + `
		func f(ctx context.Context, c conn, role string) { _ = c.Exec(ctx, "SET ROLE " + role) }`
	if err := os.WriteFile(filepath.Join(root, "probe.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("writing the source: %v", err)
	}

	report, err := Check(root, []string{"internal"})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if len(report.Findings) != 1 {
		t.Errorf("got %v, want the root still walked", report.Findings)
	}
}

// The skip list is not a wildcard. A directory whose name merely contains a
// skipped name stays in scope, because "everything with support in the name" is
// not a boundary anybody decided on.
func TestSkipMatchesAPathSegmentRatherThanASubstring(t *testing.T) {
	dir := t.TempDir()

	nested := filepath.Join(dir, "testsupportish")
	if err := os.Mkdir(nested, 0o750); err != nil {
		t.Fatalf("creating the directory: %v", err)
	}
	source := probeHeader + `
		func f(ctx context.Context, c conn, role string) { _ = c.Exec(ctx, "SET ROLE " + role) }`
	if err := os.WriteFile(filepath.Join(nested, "probe.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("writing the source: %v", err)
	}

	report, err := Check(dir, []string{"testsupport"})
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if len(report.Findings) != 1 {
		t.Errorf("got %v, want the nested directory still checked", report.Findings)
	}
}

// A file that does not parse must not pass for a file with nothing wrong in it.
func TestAFileThatDoesNotParseIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package probe\nfunc ("), 0o600); err != nil {
		t.Fatalf("writing the broken source: %v", err)
	}

	if _, err := Check(dir, nil); err == nil {
		t.Fatal("a file that does not parse was reported as checked")
	}
}

// A function literal assigned at package level binds parameters too, and a
// checker that computes bindings only for func declarations hands them back the
// package constant's name. The rule lifts itself, silently.
func TestAFunctionLiteralOutsideAFunctionDeclarationStillShadows(t *testing.T) {
	findings := analyse(t, probeHeader+`
		const stmt = "SELECT 1"

		var run = func(ctx context.Context, c conn, stmt string) { _ = c.Exec(ctx, stmt) }`)
	if len(findings) != 1 {
		t.Fatalf("got %v, want the shadowed parameter reported", findings)
	}
	if !strings.Contains(findings[0].Expr, "stmt") {
		t.Errorf("finding reports %q, want it to name the expression", findings[0].Expr)
	}
}

// The name matched and the arity did not. database/sql's Exec takes no context,
// so the statement sits where a pgx call keeps its context — and skipping the
// call leaves a whole driver unread while the gate stays green.
func TestACallWhoseArityDoesNotMatchIsReportedRatherThanSkipped(t *testing.T) {
	cases := map[string]string{
		"a driver that takes no context": `
			type db struct{}
			func (db) Exec(sql string, args ...any) error { return nil }
			func f(d db, table string) { _ = d.Exec(fmt.Sprintf("DELETE FROM %s", table)) }`,
		"a prepare that takes no name": `
			type db struct{}
			func (db) Prepare(sql string) error { return nil }
			func f(d db, table string) { _ = d.Prepare(fmt.Sprintf("SELECT * FROM %s", table)) }`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if findings := analyse(t, probeHeader+body); len(findings) != 1 {
				t.Errorf("got %v, want the call reported as unread", findings)
			}
		})
	}
}

// A call with no arguments at all carries no statement, so it is not a call this
// rule has anything to say about. Without this the checker would report every
// builder method that happens to be called Query.
func TestACallWithNoArgumentsIsNotAFinding(t *testing.T) {
	findings := analyse(t, probeHeader+`
		type builder struct{}
		func (builder) Query() string { return "" }
		func f(b builder) { _ = b.Query() }`)
	if len(findings) != 0 {
		t.Errorf("got %v, want none", findings)
	}
}

// database/sql spells them with a Context suffix, and a name absent from the map
// is a statement nobody looks at.
func TestTheContextSpellingsAreChecked(t *testing.T) {
	findings := analyse(t, probeHeader+`
		type db struct{}
		func (db) ExecContext(ctx context.Context, sql string, args ...any) error { return nil }
		func f(ctx context.Context, d db, table string) {
			_ = d.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", table))
		}`)
	if len(findings) != 1 {
		t.Fatalf("got %v, want the formatted statement reported", findings)
	}
	if findings[0].Call != "ExecContext" {
		t.Errorf("finding names %q, want ExecContext", findings[0].Call)
	}
}

package database_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// exportedSurface is every function this package hands to the rest of the API,
// with the types it takes and returns.
//
// It is here because of one property that has no other witness: on the service
// seam, nothing outside this package can choose the audit actor. That is not
// enforced by a check — it is enforced by there being nowhere to write one.
// WithService takes a pool and a closure, derives the actor from the verified
// principal on the context, and the actor type is unexported, so no caller can
// build one.
//
// A check would be the wrong shape for it. Forging the actor survived two drafts
// of the proposal precisely because it was an argument's value: every call site
// looked right, and one naming a different subject would have looked right too.
// Removing the argument moves the refusal from run time to compile time, and this
// list is what makes that removal something a diff has to state out loud rather
// than something a later signature quietly takes back.
//
// WithServiceJob's string is the one string on this surface that reaches the
// audit log, and it reaches app.actor_job. A uuid passed there names a job called
// after a uuid; it does not attribute the action to that person.
//
// What this does not close, and the list says so: WithCaller still takes the
// caller's subject as a field of Caller, so on the *request* seam a subject is
// still an argument. That path is guarded by RLS rather than by attribution and
// it is outside this step; the list is where a change to it would show up.
//
// This file carries no build tag on purpose. It needs no database, it belongs in
// the fast gate, and golangci-lint never sees integration-tagged code.
func exportedSurface() []string {
	return []string{
		"HealthCheck(context.Context, *pgxpool.Pool) error",
		"IsUUIDShaped(string) bool",
		"MigrateDown(string, string, int) error",
		"MigrateForce(string, string, int) error",
		"NewPool(context.Context, string) (*pgxpool.Pool, error)",
		"RunMigrations(string, string) error",
		"VerifyPools(context.Context, *pgxpool.Pool, *pgxpool.Pool) error",
		"WithCaller(context.Context, *pgxpool.Pool, Caller, func(context.Context, pgx.Tx) error) error",
		"WithService(context.Context, *pgxpool.Pool, func(context.Context, pgx.Tx) error) error",
		"WithServiceJob(context.Context, *pgxpool.Pool, string, func(context.Context, pgx.Tx) error) error",
	}
}

func TestTheExportedSurfaceIsTheOneDeclared(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package directory: %v", err)
	}

	set := token.NewFileSet()

	var sources []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		sources = append(sources, name)
	}
	if len(sources) == 0 {
		t.Fatal("no source files found, so the surface below was compared against nothing")
	}

	var found []string
	for _, name := range sources {
		file, err := parser.ParseFile(set, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			// Recv non-nil is a method, and a method needs a value of its type
			// to be called on — which, for the actor, nothing outside this
			// package can obtain.
			if !ok || fn.Recv != nil || !fn.Name.IsExported() {
				continue
			}

			found = append(found, signature(fn))
		}
	}
	slices.Sort(found)

	if want := exportedSurface(); !slices.Equal(found, want) {
		t.Errorf("the exported functions are\n  %s\nwant\n  %s",
			strings.Join(found, "\n  "), strings.Join(want, "\n  "))
	}
}

// signature renders a declaration the way the declaration list is written: the
// name, the parameter types, and the results. Parameter names are left out —
// they are documentation, and a rename is not a change to the surface.
func signature(fn *ast.FuncDecl) string {
	render := func(fields *ast.FieldList) []string {
		var out []string
		if fields == nil {
			return out
		}

		for _, field := range fields.List {
			text := types.ExprString(field.Type)
			// One entry per name: `func f(a, b string)` is two parameters, and a
			// field with no names at all is one unnamed parameter or result.
			for range max(len(field.Names), 1) {
				out = append(out, text)
			}
		}

		return out
	}

	results := render(fn.Type.Results)
	rendered := strings.Join(results, ", ")
	if len(results) > 1 {
		rendered = "(" + rendered + ")"
	}

	signature := fmt.Sprintf("%s(%s)", fn.Name.Name, strings.Join(render(fn.Type.Params), ", "))
	if rendered != "" {
		signature += " " + rendered
	}

	return signature
}

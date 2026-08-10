package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

// statementArgument is where the statement sits in each call that carries one.
//
// The position is fixed per method rather than guessed from the shape of the
// arguments, because guessing goes wrong in both directions: a context named
// anything but ctx would push the check onto the context itself, reporting safe
// code and quietly leaving the statement unread.
//
// The list is pgx's, because pgx is the only driver here. SendBatch is
// deliberately absent: it takes a *pgx.Batch and no statement text at all — the
// text enters through Queue, which is on the list. Prepare is on it because
// pgx.Tx, which is what the impersonation seam hands to its closure, has a
// Prepare that sends real SQL to the server to be executed later by name.
// The *Context spellings are database/sql's, which nothing here imports today
// and nothing forbids either. A name absent from this map is a statement nobody
// looks at, so the cost of listing one that never appears is nothing and the
// cost of omitting one is the whole rule.
var statementArgument = map[string]int{
	"Exec":            1,
	"ExecContext":     1,
	"ExecParams":      1,
	"Query":           1,
	"QueryContext":    1,
	"QueryRow":        1,
	"QueryRowContext": 1,
	"Prepare":         2,
	"PrepareContext":  1,
	"Queue":           0,
}

// Finding is one statement assembled from something other than constants.
type Finding struct {
	// File is the path the check was given, not an absolute one: it is printed
	// for a person to open.
	File string

	// Line is where the offending argument sits, not where the call starts. A
	// multi-line call would otherwise point at a line with nothing wrong on it.
	Line int

	// Call is the method the statement was heading for.
	Call string

	// Expr is the offending argument, printed back. Naming it is what makes the
	// output actionable: "a non-constant expression" says nothing about which.
	Expr string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: %s(%s)", f.File, f.Line, f.Call, f.Expr)
}

// Report is what one run saw. The counts are part of the answer, not decoration:
// a checker that walked nothing returns no findings, and without them that is
// indistinguishable from a clean tree — which is the failure an operator would
// never notice, because the gate stays green.
type Report struct {
	Findings []Finding
	Files    int
	Skipped  []string
}

// Check walks root and reports every SQL call whose statement is not built
// entirely from constants.
//
// The rule is syntactic on purpose. A variable holding a literal today is
// reported exactly like a variable holding a parameter, because the rule is
// about the shape a statement can take rather than about the value it happens to
// carry: the variable is where the next edit puts input. What it buys is a
// property no review has to re-establish — a value from a token cannot reach a
// statement, whatever the statement is.
//
// It exists because gosec cannot do this. G201 and G202 match on SQL keywords —
// their regexp is (SELECT|DELETE|INSERT|UPDATE|INTO|FROM|WHERE) — so they see a
// formatted SELECT and miss SET, GRANT and TRUNCATE entirely.
//
// Directories named in skip are left alone, matched by path segment rather than
// by substring, and so are _test.go files. Both compose identifiers from their
// own constants and counters and never see a token. The root itself is never
// skipped: a skip list that swallowed the root would report a clean tree.
func Check(root string, skip []string) (Report, error) {
	skipped := make(map[string]bool, len(skip))
	for _, name := range skip {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			skipped[trimmed] = true
		}
	}

	report := Report{}

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walking %s: %w", path, err)
		}

		if entry.IsDir() {
			if path != root && skipped[entry.Name()] {
				report.Skipped = append(report.Skipped, path)

				return fs.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		report.Files++

		fileFindings, err := checkFile(path)
		if err != nil {
			return err
		}
		report.Findings = append(report.Findings, fileFindings...)

		return nil
	})
	if err != nil {
		return Report{}, err
	}

	return report, nil
}

func checkFile(path string) ([]Finding, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	fileConstants := constantNames(file.Decls)

	var findings []Finding

	inspect := func(node ast.Node, constants, shadowed map[string]bool) {
		ast.Inspect(node, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			index, carriesStatement := statementArgument[selector.Sel.Name]
			if !carriesStatement || len(call.Args) == 0 {
				return true
			}

			// The name matched and the arity did not, so the statement is not
			// where this checker expects it — pgx's Exec takes a context first,
			// database/sql's does not. Reported rather than skipped: "the
			// statement is somewhere else" and "there is nothing wrong here"
			// have to look different, or a whole driver goes unread in silence.
			if index >= len(call.Args) {
				findings = append(findings, Finding{
					File: path,
					Line: fset.Position(call.Pos()).Line,
					Call: selector.Sel.Name,
					Expr: render(fset, call),
				})

				return true
			}

			statement := call.Args[index]
			if isConstant(statement, constants, shadowed) {
				return true
			}

			findings = append(findings, Finding{
				File: path,
				Line: fset.Position(statement.Pos()).Line,
				Call: selector.Sel.Name,
				Expr: render(fset, statement),
			})

			return true
		})
	}

	// A constant of the package is only a constant where nothing local has taken
	// its name. Shadowing is how the rule would otherwise be lifted by accident:
	// a package-level `const query` and a `query :=` in the same file, and the
	// statement stops being checked.
	//
	// Every declaration, not only functions. A function literal assigned to a
	// package-level var has parameters too, and they shadow just as well.
	for _, decl := range file.Decls {
		constants, shadowed := localBindings(decl)
		for name := range fileConstants {
			constants[name] = true
		}

		inspect(decl, constants, shadowed)
	}

	return findings, nil
}

// localBindings collects the names a declaration binds. Constants it declares
// are still constants; everything else — parameters, results, the receiver,
// short declarations, var declarations, range and type-switch variables —
// shadows a package constant of the same name.
//
// The two sets are returned apart rather than merged, because a name can be both
// a package constant and a local variable, and in that case the local wins.
func localBindings(node ast.Node) (constants, shadowed map[string]bool) {
	constants = map[string]bool{}
	shadowed = map[string]bool{}

	bind := func(expressions []ast.Expr) {
		for _, expression := range expressions {
			if ident, ok := expression.(*ast.Ident); ok && ident.Name != "_" {
				shadowed[ident.Name] = true
			}
		}
	}

	bindFields := func(fields *ast.FieldList) {
		if fields == nil {
			return
		}

		for _, field := range fields.List {
			for _, name := range field.Names {
				if name.Name != "_" {
					shadowed[name.Name] = true
				}
			}
		}
	}

	if function, ok := node.(*ast.FuncDecl); ok {
		bindFields(function.Recv)
		if function.Type != nil {
			bindFields(function.Type.Params)
			bindFields(function.Type.Results)
		}
	}

	ast.Inspect(node, func(node ast.Node) bool {
		switch statement := node.(type) {
		case *ast.AssignStmt:
			if statement.Tok == token.DEFINE {
				bind(statement.Lhs)
			}

		case *ast.RangeStmt:
			if statement.Tok == token.DEFINE {
				bind([]ast.Expr{statement.Key, statement.Value})
			}

		case *ast.TypeSwitchStmt:
			if assign, ok := statement.Assign.(*ast.AssignStmt); ok {
				bind(assign.Lhs)
			}

		case *ast.FuncLit:
			if statement.Type != nil {
				bindFields(statement.Type.Params)
				bindFields(statement.Type.Results)
			}

		case *ast.DeclStmt:
			declaration, ok := statement.Decl.(*ast.GenDecl)
			if !ok {
				return true
			}

			for name := range constantNames([]ast.Decl{declaration}) {
				constants[name] = true
			}

			if declaration.Tok != token.VAR {
				return true
			}

			for _, spec := range declaration.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}

				for _, name := range value.Names {
					if name.Name != "_" {
						shadowed[name.Name] = true
					}
				}
			}
		}

		return true
	})

	return constants, shadowed
}

// constantNames collects the names declared const among the given declarations.
// A constant declared in another file of the same package is not seen, which
// makes the rule stricter than it could be rather than looser — the failure mode
// of a missed constant is a finding somebody has to move, not a statement nobody
// checked.
func constantNames(decls []ast.Decl) map[string]bool {
	constants := map[string]bool{}

	for _, decl := range decls {
		general, ok := decl.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST {
			continue
		}

		for _, spec := range general.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			for _, name := range value.Names {
				constants[name.Name] = true
			}
		}
	}

	return constants
}

func isConstant(expr ast.Expr, constants, shadowed map[string]bool) bool {
	switch node := expr.(type) {
	case *ast.BasicLit:
		return node.Kind == token.STRING

	case *ast.Ident:
		return constants[node.Name] && !shadowed[node.Name]

	case *ast.ParenExpr:
		return isConstant(node.X, constants, shadowed)

	case *ast.BinaryExpr:
		return node.Op == token.ADD &&
			isConstant(node.X, constants, shadowed) &&
			isConstant(node.Y, constants, shadowed)

	default:
		return false
	}
}

func render(fset *token.FileSet, expr ast.Expr) string {
	var out strings.Builder
	if err := printer.Fprint(&out, fset, expr); err != nil {
		// The expression was parsed from this very file a moment ago, so a
		// printer failure is not a condition to handle — but reporting a
		// finding with an empty name would be worse than reporting the reason.
		return fmt.Sprintf("<unprintable: %v>", err)
	}

	return out.String()
}

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestTheGuardRefusesEverythingButACurrentSecret — two values are current from
// day one, so that replacing one never needs a moment when neither works.
//
// The wrong-secret case is not the same test as the no-secret case: a guard
// that only checked for presence would pass the second and fail nothing.
func TestTheGuardRefusesEverythingButACurrentSecret(t *testing.T) {
	fake := startFakeGoTrue(t)
	srv := testServer(t, development, fake)

	tests := []struct {
		name   string
		set    func(*http.Request)
		status int
	}{
		{
			name:   "no secret at all",
			set:    func(*http.Request) {},
			status: http.StatusUnauthorized,
		},
		{
			name:   "a wrong secret",
			set:    func(r *http.Request) { r.Header.Set(secretHeader, "provisioner-shared-secret-for-tests-9999") },
			status: http.StatusUnauthorized,
		},
		{
			name:   "an empty secret",
			set:    func(r *http.Request) { r.Header.Set(secretHeader, "") },
			status: http.StatusUnauthorized,
		},
		{
			name: "a prefix of a current secret",
			set: func(r *http.Request) {
				r.Header.Set(secretHeader, testSecret[:len(testSecret)-1])
			},
			status: http.StatusUnauthorized,
		},
		{
			name:   "the current secret",
			set:    func(r *http.Request) { r.Header.Set(secretHeader, testSecret) },
			status: http.StatusOK,
		},
		{
			name:   "the previous secret, still current",
			set:    func(r *http.Request) { r.Header.Set(secretHeader, testPreviousSecret) },
			status: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/users/lookup",
				strings.NewReader(`{"email":"nobody@example.invalid"}`))
			tc.set(req)

			rec := httptest.NewRecorder()
			srv.mux().ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Fatalf("answered %d, want %d; body %s", rec.Code, tc.status, rec.Body)
			}
		})
	}
}

// TestAPathThatIsNotMountedIsAlsoRefusedWithoutASecret.
//
// The alternative shape — the guard wrapped around each route, or around a
// group — keeps every other test in this package green while answering 404 to a
// secretless probe. That difference tells whoever is probing whether POST
// /users/password is mounted, which is to say whether this deployment is
// production.
func TestAPathThatIsNotMountedIsAlsoRefusedWithoutASecret(t *testing.T) {
	srv := testServer(t, development, nil)

	for _, path := range []string{"/debug/pprof/", "/metrics", "/users/password", "/"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}"))

		rec := httptest.NewRecorder()
		srv.mux().ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("POST %s answered %d without a secret, want 401 — the router decided before the guard did",
				path, rec.Code)
		}
	}
}

// TestTheSecretIsNotAcceptedFromTheQueryString. A secret in a URL is a secret
// in every access log, proxy trace and browser history on the way.
func TestTheSecretIsNotAcceptedFromTheQueryString(t *testing.T) {
	srv := testServer(t, development, startFakeGoTrue(t))

	req := httptest.NewRequest(http.MethodPost,
		"/users/lookup?secret="+testSecret, strings.NewReader(`{"email":"nobody@example.com"}`))

	rec := httptest.NewRecorder()
	srv.mux().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("a secret in the query string answered %d, want 401", rec.Code)
	}
}

// TestARefusalSaysNothingAboutTheSecret: the body of a refusal is the same
// whether the secret was absent, wrong, or one character short. Telling them
// apart tells whoever is guessing which half of the guess was right.
func TestARefusalSaysNothingAboutTheSecret(t *testing.T) {
	srv := testServer(t, development, startFakeGoTrue(t))

	bodies := map[string]struct{}{}

	for _, offered := range []string{"", "wrong", testSecret[:8]} {
		req := httptest.NewRequest(http.MethodPost, "/users/lookup", strings.NewReader("{}"))
		if offered != "" {
			req.Header.Set(secretHeader, offered)
		}

		rec := httptest.NewRecorder()
		srv.mux().ServeHTTP(rec, req)

		if strings.Contains(rec.Body.String(), offered) && offered != "" {
			t.Errorf("the refusal echoed the offered secret: %s", rec.Body)
		}

		bodies[rec.Body.String()] = struct{}{}
	}

	if len(bodies) != 1 {
		t.Errorf("refusals differ by reason, so they distinguish them: %v", bodies)
	}
}

// TestTheSecretIsComparedInConstantTime pins the shape of the comparison rather
// than its timing: timing is not measurable in a unit test on a machine that
// also runs a garbage collector, so a test claiming to measure it would pass
// for the wrong reason. What is checkable is that the function works through
// crypto/subtle and carries no equality operator of its own, `==` on a secret
// returning as soon as two bytes differ. It is enforced against the source
// because an implementation that went back to `==` would pass every
// behavioural test above.
func TestTheSecretIsComparedInConstantTime(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "secret.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing secret.go: %v", err)
	}

	body := functionBody(t, file, "permits")

	var (
		constantTimeCalls int
		equality          []string
	)

	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if selector, ok := node.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := selector.X.(*ast.Ident); ok &&
					pkg.Name == "subtle" && strings.HasPrefix(selector.Sel.Name, "ConstantTime") {
					constantTimeCalls++
				}
			}
		case *ast.BinaryExpr:
			if node.Op == token.EQL || node.Op == token.NEQ {
				equality = append(equality, node.Op.String())
			}
		}

		return true
	})

	if constantTimeCalls == 0 {
		t.Error("permits does not call crypto/subtle at all")
	}

	if len(equality) != 0 {
		t.Errorf("permits compares with %v; an equality operator on a secret leaks where it first differs", equality)
	}
}

// TestBothCurrentValuesAreConsultedEvenWhenTheFirstMatches: permits must not
// short-circuit on the first, or which of the two is offered becomes observable
// in the time it takes to answer.
func TestBothCurrentValuesAreConsultedEvenWhenTheFirstMatches(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "secret.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing secret.go: %v", err)
	}

	var returns int

	ast.Inspect(functionBody(t, file, "permits"), func(n ast.Node) bool {
		if _, ok := n.(*ast.ReturnStmt); ok {
			returns++
		}

		return true
	})

	if returns != 1 {
		t.Errorf("permits has %d return statements; with more than one, an early exit on the first "+
			"matching value makes which value was offered observable", returns)
	}
}

func functionBody(t *testing.T, file *ast.File, name string) *ast.BlockStmt {
	t.Helper()

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name {
			return fn.Body
		}
	}

	t.Fatalf("secret.go declares no function %q, so this check read nothing", name)

	return nil
}

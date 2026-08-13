package token_test

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth/token"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// guarded builds the middleware around a handler that records whether it ran
// and what principal it saw. Reaching the handler at all is the failure this
// suite is looking for, so it is observable rather than inferred from a status.
func guarded(t *testing.T, exempt []string) (http.Handler, *testsupport.SigningKey, *testsupport.JWKS, *reached) {
	t.Helper()

	key := testsupport.NewRS256Key(t, "primary")
	set := testsupport.StartJWKS(t, key)
	verifier := newVerifier(t, set, key.KID)

	seen := &reached{}
	handler := token.Middleware(verifier, exempt)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.called = true
		seen.principal, seen.hasPrincipal = auth.PrincipalFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	return handler, key, set, seen
}

// formatHeaders renders a response's headers in a stable order, so two
// responses can be compared as strings.
func formatHeaders(header http.Header) string {
	names := make([]string, 0, len(header))
	for name := range header {
		names = append(names, name)
	}
	sort.Strings(names)

	var out strings.Builder
	for _, name := range names {
		out.WriteString(name + ": " + strings.Join(header.Values(name), ",") + "\n")
	}

	return out.String()
}

type reached struct {
	called       bool
	principal    auth.Principal
	hasPrincipal bool
}

func do(t *testing.T, handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

func TestMiddlewarePassesAVerifiedCallerThrough(t *testing.T) {
	handler, key, set, seen := guarded(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+key.Sign(t, validClaims(set.Issuer)))

	if rec := do(t, handler, req); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	if !seen.called {
		t.Fatal("the handler did not run")
	}
	if !seen.hasPrincipal {
		t.Fatal("the handler saw no principal")
	}
	if seen.principal.Subject != "8a1f3b7c-0000-4000-8000-000000000001" {
		t.Errorf("Subject = %q", seen.principal.Subject)
	}
}

// TestMiddlewareAcceptsTheSchemeCaseInsensitively follows RFC 7235: the auth
// scheme is a case-insensitive token. A client sending "bearer" is correct, and
// refusing it would be a bug that looks like an authentication failure.
func TestMiddlewareAcceptsTheSchemeCaseInsensitively(t *testing.T) {
	for _, scheme := range []string{"Bearer", "bearer", "BEARER", "BeArEr"} {
		t.Run(scheme, func(t *testing.T) {
			handler, key, set, seen := guarded(t, nil)

			req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
			req.Header.Set("Authorization", scheme+" "+key.Sign(t, validClaims(set.Issuer)))

			if rec := do(t, handler, req); rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if !seen.called {
				t.Error("the handler did not run")
			}
		})
	}
}

// TestMiddlewareRefuses is the rejection surface of the transport, as opposed to
// the token rules covered in verifier_test.go. Every case must stop before the
// handler.
func TestMiddlewareRefuses(t *testing.T) {
	tests := []struct {
		name    string
		request func(t *testing.T, key *testsupport.SigningKey, set *testsupport.JWKS) *http.Request
	}{
		{
			name: "no Authorization header",
			request: func(*testing.T, *testsupport.SigningKey, *testsupport.JWKS) *http.Request {
				return httptest.NewRequest(http.MethodGet, "/v1/me", nil)
			},
		},
		{
			name: "empty Authorization header",
			request: func(*testing.T, *testsupport.SigningKey, *testsupport.JWKS) *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
				req.Header.Set("Authorization", "")

				return req
			},
		},
		{
			name: "another scheme",
			request: func(*testing.T, *testsupport.SigningKey, *testsupport.JWKS) *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
				req.Header.Set("Authorization", "Basic Zm9vOmJhcg==")

				return req
			},
		},
		{
			// The case that makes the scheme comparison load-bearing. "another
			// scheme" above carries base64 that is not a JWT, so it is refused
			// by the verifier and would be refused with no scheme check at all
			// — measured: deleting the EqualFold left every package green.
			// Case-insensitivity is covered separately, and it survives the
			// same deletion for the same reason: all four spellings pass.
			//
			// A credential minted for this API but presented under Basic is not
			// a bearer credential, and treating it as one would accept whatever
			// else a client chose to call it.
			name: "a valid token under another scheme",
			request: func(t *testing.T, key *testsupport.SigningKey, set *testsupport.JWKS) *http.Request {
				t.Helper()

				req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
				req.Header.Set("Authorization", "Basic "+key.Sign(t, validClaims(set.Issuer)))

				return req
			},
		},
		{
			name: "scheme with no token",
			request: func(*testing.T, *testsupport.SigningKey, *testsupport.JWKS) *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
				req.Header.Set("Authorization", "Bearer ")

				return req
			},
		},
		{
			// A token in the URL ends up in access logs, in Referer headers and
			// in browser history. The same rule the architecture review sets for
			// the chat socket: the credential travels in a header or not at all.
			name: "token in the query string",
			request: func(t *testing.T, key *testsupport.SigningKey, set *testsupport.JWKS) *http.Request {
				token := key.Sign(t, validClaims(set.Issuer))

				return httptest.NewRequest(http.MethodGet, "/v1/me?access_token="+token, nil)
			},
		},
		{
			name: "expired token",
			request: func(t *testing.T, key *testsupport.SigningKey, set *testsupport.JWKS) *http.Request {
				claims := validClaims(set.Issuer)
				claims["exp"] = time.Now().Add(-time.Hour).Unix()

				req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
				req.Header.Set("Authorization", "Bearer "+key.Sign(t, claims))

				return req
			},
		},
		{
			name: "unknown key id",
			request: func(t *testing.T, key *testsupport.SigningKey, set *testsupport.JWKS) *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
				req.Header.Set("Authorization", "Bearer "+key.SignWithKID(t, "nope", validClaims(set.Issuer)))

				return req
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, key, set, seen := guarded(t, nil)

			rec := do(t, handler, tt.request(t, key, set))

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if seen.called {
				t.Error("the handler ran on an unauthenticated request")
			}
			if got := rec.Header().Get("Content-Type"); got != httpserver.ProblemContentType {
				t.Errorf("Content-Type = %q, want %q", got, httpserver.ProblemContentType)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("WWW-Authenticate = %q, want Bearer", got)
			}
		})
	}
}

// TestMiddlewareRefusalsAreIndistinguishable is the requirement the whole 401
// design exists for. Whoever is guessing must not learn which half of the guess
// was right — not from the body, and not from a header.
func TestMiddlewareRefusalsAreIndistinguishable(t *testing.T) {
	handler, key, set, _ := guarded(t, nil)

	expired := validClaims(set.Issuer)
	expired["exp"] = time.Now().Add(-time.Hour).Unix()

	foreignIssuer := validClaims("https://someone-else.supabase.co/auth/v1")

	noExpiry := validClaims(set.Issuer)
	delete(noExpiry, "exp")

	headers := []string{
		"",
		"Bearer ",
		"Basic Zm9vOmJhcg==",
		"Bearer not.a.token",
		"Bearer " + key.Sign(t, expired),
		"Bearer " + key.Sign(t, foreignIssuer),
		"Bearer " + key.Sign(t, noExpiry),
		"Bearer " + key.SignWithKID(t, "unknown", validClaims(set.Issuer)),
		"Bearer " + key.SignWithoutKID(t, validClaims(set.Issuer)),
		"Bearer " + testsupport.SignHS256(t, key.KID, key.PublicKeyPEM(t), validClaims(set.Issuer)),
	}

	var first, firstHeaders string

	for i, header := range headers {
		req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}

		rec := do(t, handler, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("header %d: status = %d, want 401", i, rec.Code)
		}

		body := rec.Body.String()
		// The response headers are part of what a caller can see, and the doc
		// comment above promises they carry no distinction either. Comparing
		// only the body would leave a Retry-After or a WWW-Authenticate
		// parameter free to say which check failed.
		responseHeaders := formatHeaders(rec.Header())

		if i == 0 {
			first, firstHeaders = body, responseHeaders

			continue
		}

		if body != first {
			t.Errorf("header %d produced a different body:\n got %s\nwant %s", i, body, first)
		}
		if responseHeaders != firstHeaders {
			t.Errorf("header %d produced different response headers:\n got %s\nwant %s",
				i, responseHeaders, firstHeaders)
		}
	}

	// A body that never mentions the reason is only half the guarantee; it also
	// must not be empty, or the "one error shape" contract has a hole at 401.
	if first == "" {
		t.Fatal("the 401 body is empty")
	}
}

// TestMiddlewareLetsExemptPathsThrough covers the other direction: the handful
// of transport paths that have to answer without a token.
func TestMiddlewareLetsExemptPathsThrough(t *testing.T) {
	exempt := []string{"/healthz", "/openapi.json", "/docs"}
	handler, _, _, seen := guarded(t, exempt)

	for _, path := range exempt {
		seen.called = false

		if rec := do(t, handler, httptest.NewRequest(http.MethodGet, path, nil)); rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", path, rec.Code)
		}
		if !seen.called {
			t.Errorf("%s: the handler did not run", path)
		}
		if seen.hasPrincipal {
			t.Errorf("%s: an exempt path must not carry a principal", path)
		}
	}
}

// TestMiddlewareExemptionIsExact stops the exemption list from being read as a
// prefix list. "/healthz" must not open "/healthzzz" or "/healthz/../v1/me".
func TestMiddlewareExemptionIsExact(t *testing.T) {
	handler, _, _, seen := guarded(t, []string{"/healthz"})

	for _, path := range []string{"/healthzzz", "/healthz/detail", "/healthz2"} {
		seen.called = false

		if rec := do(t, handler, httptest.NewRequest(http.MethodGet, path, nil)); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", path, rec.Code)
		}
		if seen.called {
			t.Errorf("%s: the handler ran unauthenticated", path)
		}
	}
}

// TestMiddlewareGuardsEveryUnlistedPath is deny-by-default stated as a
// property: a path nobody thought about is guarded, rather than a path nobody
// thought about being open.
func TestMiddlewareGuardsEveryUnlistedPath(t *testing.T) {
	handler, _, _, seen := guarded(t, []string{"/healthz"})

	for _, path := range []string{"/", "/v1", "/v1/me", "/v2/anything", "/internal/debug", "/openapi.json"} {
		seen.called = false

		if rec := do(t, handler, httptest.NewRequest(http.MethodGet, path, nil)); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", path, rec.Code)
		}
		if seen.called {
			t.Errorf("%s: the handler ran unauthenticated", path)
		}
	}
}

// TestMiddlewareRefusesWhenKeysAreUnavailable proves the outage path answers
// like every other refusal. It is a separate test because it is the one case
// where the honest answer is "cannot say" — and "cannot say" must not become
// "come in".
func TestMiddlewareRefusesWhenKeysAreUnavailable(t *testing.T) {
	key := testsupport.NewRS256Key(t, "primary")
	set := testsupport.StartJWKS(t, key)
	set.Break()

	verifier := newVerifier(t, set, key.KID)

	called := false
	handler := token.Middleware(verifier, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+key.Sign(t, validClaims(set.Issuer)))

	if rec := do(t, handler, req); rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if called {
		t.Error("the handler ran while the key set was unreachable")
	}
}

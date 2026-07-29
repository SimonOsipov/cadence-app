package auth_test

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

const testAudience = "authenticated"

// newVerifier wires a verifier to a local key set, the way the composition root
// wires one to Supabase: the JWKS address is derived from the issuer, never
// passed alongside it.
func newVerifier(t *testing.T, set *testsupport.JWKS) *auth.Verifier {
	t.Helper()

	verifier, err := auth.NewVerifier(t.Context(), auth.VerifierConfig{
		Issuer:   set.Issuer,
		Audience: testAudience,
		JWKSURL:  set.Issuer + testsupport.JWKSPath,
	})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	return verifier
}

// validClaims is a token that must be accepted. Every rejection test below is
// this set with exactly one thing changed, so a test that fails names the rule
// that rejected it rather than an unrelated omission.
func validClaims(issuer string) jwt.MapClaims {
	now := time.Now()

	return jwt.MapClaims{
		"sub":  "8a1f3b7c-0000-4000-8000-000000000001",
		"role": "authenticated",
		"aud":  testAudience,
		"iss":  issuer,
		"iat":  now.Unix(),
		"nbf":  now.Add(-time.Minute).Unix(),
		"exp":  now.Add(time.Hour).Unix(),
	}
}

func TestVerifyAcceptsBothPermittedAlgorithms(t *testing.T) {
	tests := []struct {
		name string
		key  func(*testing.T, string) *testsupport.SigningKey
	}{
		// Supabase issues one or the other depending on the project's age, and
		// which one cadence-dev uses is confirmed in SKL-01. Both are proven here
		// so that answer cannot turn into a code change.
		{"RS256", testsupport.NewRS256Key},
		{"ES256", testsupport.NewES256Key},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := tt.key(t, "primary")
			set := testsupport.StartJWKS(t, key)
			verifier := newVerifier(t, set)

			claims := validClaims(set.Issuer)
			principal, err := verifier.Verify(t.Context(), key.Sign(t, claims))
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}

			if principal.Subject != claims["sub"] {
				t.Errorf("Subject = %q, want %q", principal.Subject, claims["sub"])
			}
			if principal.Role != "authenticated" {
				t.Errorf("Role = %q, want authenticated", principal.Role)
			}
			exp, ok := claims["exp"].(int64)
			if !ok {
				t.Fatalf("the fixture's exp claim is %T, not int64", claims["exp"])
			}

			if want := time.Unix(exp, 0); !principal.ExpiresAt.Equal(want) {
				t.Errorf("ExpiresAt = %v, want %v", principal.ExpiresAt, want)
			}
		})
	}
}

// TestVerifyRejects is the whole list of refusals from the specification. Each
// case starts from a token that is known to be accepted and breaks one rule.
func TestVerifyRejects(t *testing.T) {
	tests := []struct {
		name string
		// token builds the token under test. issuerKey is published in the key
		// set; otherKey is not.
		token func(t *testing.T, issuerKey, otherKey *testsupport.SigningKey, issuer string) string
	}{
		{
			name: "expired",
			token: func(t *testing.T, key, _ *testsupport.SigningKey, issuer string) string {
				claims := validClaims(issuer)
				claims["exp"] = time.Now().Add(-time.Hour).Unix()

				return key.Sign(t, claims)
			},
		},
		{
			// jwt/v5 treats a missing exp as "never expires" unless expiration is
			// required. A token that cannot expire is a credential, not a session.
			name: "no expiry claim",
			token: func(t *testing.T, key, _ *testsupport.SigningKey, issuer string) string {
				claims := validClaims(issuer)
				delete(claims, "exp")

				return key.Sign(t, claims)
			},
		},
		{
			name: "not yet valid",
			token: func(t *testing.T, key, _ *testsupport.SigningKey, issuer string) string {
				claims := validClaims(issuer)
				claims["nbf"] = time.Now().Add(time.Hour).Unix()

				return key.Sign(t, claims)
			},
		},
		{
			name: "foreign issuer",
			token: func(t *testing.T, key, _ *testsupport.SigningKey, _ string) string {
				return key.Sign(t, validClaims("https://someone-elses-project.supabase.co/auth/v1"))
			},
		},
		{
			// Supabase sets aud to "authenticated" in every project, so this check
			// isolates nothing between projects — the issuer and the key identity
			// do that. It is enforced because the contract declares it, and it is
			// said plainly here so the green test is not read as isolation.
			name: "foreign audience",
			token: func(t *testing.T, key, _ *testsupport.SigningKey, issuer string) string {
				claims := validClaims(issuer)
				claims["aud"] = "someone-else"

				return key.Sign(t, claims)
			},
		},
		{
			name: "unknown key id",
			token: func(t *testing.T, key, _ *testsupport.SigningKey, issuer string) string {
				return key.SignWithKID(t, "not-in-the-set", validClaims(issuer))
			},
		},
		{
			// Without a kid the key set has to be searched, and a token is then
			// accepted if *any* published key verifies it. That is the door a
			// rotated-out key walks back through.
			name: "no key id",
			token: func(t *testing.T, key, _ *testsupport.SigningKey, issuer string) string {
				return key.SignWithoutKID(t, validClaims(issuer))
			},
		},
		{
			name: "signed by a key that is not published",
			token: func(t *testing.T, key, other *testsupport.SigningKey, issuer string) string {
				// The published kid, someone else's key: the substitution the
				// signature check exists to catch.
				return other.SignWithKID(t, key.KID, validClaims(issuer))
			},
		},
		{
			// The algorithm-confusion attack: HS256 over the published public key
			// as the shared secret. Verifying it would mean the server treated a
			// public value as a private one.
			name: "HS256 over the public key",
			token: func(t *testing.T, key, _ *testsupport.SigningKey, issuer string) string {
				return testsupport.SignHS256(t, key.KID, key.PublicKeyPEM(t), validClaims(issuer))
			},
		},
		{
			name: "malformed",
			token: func(*testing.T, *testsupport.SigningKey, *testsupport.SigningKey, string) string {
				return "not.a.token"
			},
		},
		{
			name: "empty",
			token: func(*testing.T, *testsupport.SigningKey, *testsupport.SigningKey, string) string {
				return ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := testsupport.NewRS256Key(t, "primary")
			other := testsupport.NewRS256Key(t, "unpublished")
			set := testsupport.StartJWKS(t, key)
			verifier := newVerifier(t, set)

			if _, err := verifier.Verify(t.Context(), tt.token(t, key, other, set.Issuer)); err == nil {
				t.Fatal("Verify: want error, got nil")
			}
		})
	}
}

// TestVerifyRejectsAForeignAlgorithmSignedByTheRightKey is the test that
// actually exercises the allowlist, and it exists because the obvious one does
// not.
//
// Deleting jwt.WithValidMethods leaves the HS256 and "alg": "none" cases in
// TestVerifyRejects green, whatever the key set declares. Those two are stopped
// earlier and by something else: the key resolver hands back a typed
// *rsa.PublicKey, which golang-jwt refuses to use as an HMAC secret or as the
// sentinel that "none" demands. Good, but not this rule.
//
// What the allowlist alone stands between is here. One RSA public key verifies
// RS256, RS384, RS512 and the whole PSS family, so a token signed by the
// genuine key under an algorithm the provider never issues is cryptographically
// valid — and is accepted, unless the server names the algorithms it expects.
func TestVerifyRejectsAForeignAlgorithmSignedByTheRightKey(t *testing.T) {
	key := testsupport.NewRS256Key(t, "primary")

	// Published without an `alg` member, which RFC 7517 permits. With one, the
	// JWKS library does the comparison itself and this test would again be
	// passing for a reason other than the one it names.
	set := testsupport.StartJWKSWithoutAlg(t, key)
	verifier := newVerifier(t, set)

	if _, err := verifier.Verify(t.Context(), key.Sign(t, validClaims(set.Issuer))); err != nil {
		t.Fatalf("Verify on the permitted algorithm: %v", err)
	}

	for _, method := range []jwt.SigningMethod{
		jwt.SigningMethodRS512,
		jwt.SigningMethodPS256,
	} {
		token := key.SignAs(t, method, validClaims(set.Issuer))
		if _, err := verifier.Verify(t.Context(), token); err == nil {
			t.Errorf("Verify accepted a %s token signed by the published key", method.Alg())
		}
	}
}

// TestVerifyRejectsUnsignedAndSymmetricTokens covers the two classic forgeries
// end to end. They are refused by the type of the resolved key rather than by
// the allowlist — see the test above — and they are asserted anyway, because
// what matters is that they are refused, not which line refuses them.
func TestVerifyRejectsUnsignedAndSymmetricTokens(t *testing.T) {
	key := testsupport.NewRS256Key(t, "primary")
	set := testsupport.StartJWKSWithoutAlg(t, key)
	verifier := newVerifier(t, set)

	forged := testsupport.SignHS256(t, key.KID, key.PublicKeyPEM(t), validClaims(set.Issuer))
	if _, err := verifier.Verify(t.Context(), forged); err == nil {
		t.Error("Verify accepted HS256 signed with the published public key")
	}

	if _, err := verifier.Verify(t.Context(), testsupport.SignNone(t, key.KID, validClaims(set.Issuer))); err == nil {
		t.Error(`Verify accepted a token with "alg": "none"`)
	}
}

// TestVerifyRejectsTamperedSignature is separate because the tampering is done
// to a token the same verifier has already accepted — which is what makes the
// failure attributable to the signature and to nothing else.
func TestVerifyRejectsTamperedSignature(t *testing.T) {
	key := testsupport.NewRS256Key(t, "primary")
	set := testsupport.StartJWKS(t, key)
	verifier := newVerifier(t, set)

	token := key.Sign(t, validClaims(set.Issuer))
	if _, err := verifier.Verify(t.Context(), token); err != nil {
		t.Fatalf("Verify on the untampered token: %v", err)
	}

	tampered := token[:len(token)-2] + flipLast(token)
	if _, err := verifier.Verify(t.Context(), tampered); err == nil {
		t.Fatal("Verify: want error for a tampered signature, got nil")
	}
}

func flipLast(token string) string {
	last := token[len(token)-1]
	if last == 'A' {
		return "AB"
	}

	return "AA"
}

// TestVerifyRefusesWhenKeySetIsUnreachable is the difference between an outage
// and an open door. With no key cached there is nothing to verify against, and
// the only two honest answers are "no" and "cannot say" — never "yes".
func TestVerifyRefusesWhenKeySetIsUnreachable(t *testing.T) {
	key := testsupport.NewRS256Key(t, "primary")
	set := testsupport.StartJWKS(t, key)
	set.Break()

	verifier := newVerifier(t, set)

	if _, err := verifier.Verify(t.Context(), key.Sign(t, validClaims(set.Issuer))); err == nil {
		t.Fatal("Verify: want error while the key set is unreachable, got nil")
	}
}

// TestVerifyRateLimitsUnknownKeyIDRefresh guards the availability side. Every
// unknown kid triggering a fetch turns a stream of junk tokens into a request
// flood against the provider, whose rate limiter then takes down authentication
// for everyone.
func TestVerifyRateLimitsUnknownKeyIDRefresh(t *testing.T) {
	key := testsupport.NewRS256Key(t, "primary")
	set := testsupport.StartJWKS(t, key)
	verifier := newVerifier(t, set)

	// One fetch to fill the cache, so the count below is refreshes and not the
	// initial load.
	if _, err := verifier.Verify(t.Context(), key.Sign(t, validClaims(set.Issuer))); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	before := set.Requests()

	for range 20 {
		junk := key.SignWithKID(t, "unknown-"+time.Now().Format(time.RFC3339Nano), validClaims(set.Issuer))
		if _, err := verifier.Verify(t.Context(), junk); err == nil {
			t.Fatal("Verify: want error for an unknown kid, got nil")
		}
	}

	if refreshes := set.Requests() - before; refreshes > 1 {
		t.Errorf("the key set was fetched %d times for 20 unknown key ids, want at most 1", refreshes)
	}
}

// TestRateLimitedUnknownKeyIDIsStillTheCallersProblem: hitting the refresh
// budget must not turn a bad token into a signing-key outage. The key set is
// cached and healthy throughout — if these were reported as ErrKeysUnavailable,
// a token spray would fill the log with an outage that is not happening, and a
// real one would be indistinguishable from attacker traffic.
func TestRateLimitedUnknownKeyIDIsStillTheCallersProblem(t *testing.T) {
	key := testsupport.NewRS256Key(t, "primary")
	set := testsupport.StartJWKS(t, key)
	verifier := newVerifier(t, set)

	if _, err := verifier.Verify(t.Context(), key.Sign(t, validClaims(set.Issuer))); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	for i := range 5 {
		junk := key.SignWithKID(t, fmt.Sprintf("unknown-%d", i), validClaims(set.Issuer))

		_, err := verifier.Verify(t.Context(), junk)
		if !errors.Is(err, auth.ErrTokenRejected) {
			t.Errorf("unknown kid %d: err = %v, want it to wrap ErrTokenRejected", i, err)
		}
		if errors.Is(err, auth.ErrKeysUnavailable) {
			t.Errorf("unknown kid %d was reported as a key-set outage: %v", i, err)
		}
	}
}

// TestVerifyPicksUpRotatedKeysOverANetwork is TestVerifyPicksUpRotatedKeys with
// the provider where it actually lives — behind a round trip.
//
// It exists because the localhost version passes with an on-demand fetch budget
// of 100 ms, and against Supabase a TLS handshake alone exceeds that. With too
// small a budget, the rotation is never picked up on demand and every token
// signed by the new key is refused until the next scheduled refresh — an hour
// of blanket 401s, invisible to any test that runs at localhost latency.
func TestVerifyPicksUpRotatedKeysOverANetwork(t *testing.T) {
	old := testsupport.NewRS256Key(t, "old")
	set := testsupport.StartJWKS(t, old)
	set.Delay(300 * time.Millisecond)

	// Deliberately the production defaults: the budget under test is one of them.
	verifier := newVerifier(t, set)

	if _, err := verifier.Verify(t.Context(), old.Sign(t, validClaims(set.Issuer))); err != nil {
		t.Fatalf("Verify with the original key: %v", err)
	}

	rotated := testsupport.NewRS256Key(t, "rotated")
	set.Publish(t, old, rotated)

	if _, err := verifier.Verify(t.Context(), rotated.Sign(t, validClaims(set.Issuer))); err != nil {
		t.Fatalf("Verify after rotation at 300ms provider latency: %v", err)
	}
}

// TestVerifyPicksUpRotatedKeys is the other half of the rate limit: a bounded
// refresh must still be a refresh. A key published after the verifier started
// has to become usable without a restart.
func TestVerifyPicksUpRotatedKeys(t *testing.T) {
	old := testsupport.NewRS256Key(t, "old")
	set := testsupport.StartJWKS(t, old)

	verifier, err := auth.NewVerifier(t.Context(), auth.VerifierConfig{
		Issuer:             set.Issuer,
		Audience:           testAudience,
		JWKSURL:            set.Issuer + testsupport.JWKSPath,
		UnknownKIDInterval: time.Nanosecond,
	})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	if _, err := verifier.Verify(t.Context(), old.Sign(t, validClaims(set.Issuer))); err != nil {
		t.Fatalf("Verify with the original key: %v", err)
	}

	rotated := testsupport.NewRS256Key(t, "rotated")
	set.Publish(t, old, rotated)

	if _, err := verifier.Verify(t.Context(), rotated.Sign(t, validClaims(set.Issuer))); err != nil {
		t.Fatalf("Verify after rotation: %v", err)
	}
}

// TestVerifyRejectsTokenWithoutSubject keeps a principal from existing without
// an identity: everything downstream — impersonation, RLS, audit — is keyed on
// it, and an empty subject would silently become a valid one.
func TestVerifyRejectsTokenWithoutSubject(t *testing.T) {
	key := testsupport.NewRS256Key(t, "primary")
	set := testsupport.StartJWKS(t, key)
	verifier := newVerifier(t, set)

	claims := validClaims(set.Issuer)
	delete(claims, "sub")

	if _, err := verifier.Verify(t.Context(), key.Sign(t, claims)); err == nil {
		t.Fatal("Verify: want error for a token with no subject, got nil")
	}
}

// TestVerifyDoesNotCarryClaimsBeyondThePrincipal is the narrowness requirement
// as a test rather than as a promise. A Supabase token carries email, phone and
// app_metadata; the type the rest of the API sees has three fields, so there is
// no route by which the rest can arrive.
func TestVerifyDoesNotCarryClaimsBeyondThePrincipal(t *testing.T) {
	key := testsupport.NewRS256Key(t, "primary")
	set := testsupport.StartJWKS(t, key)
	verifier := newVerifier(t, set)

	claims := validClaims(set.Issuer)
	claims["email"] = "patient@example.com"
	claims["phone"] = "+70000000000"
	claims["app_metadata"] = map[string]any{"provider": "email"}

	principal, err := verifier.Verify(t.Context(), key.Sign(t, claims))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	var fields []string
	for i, typ := 0, reflect.TypeOf(principal); i < typ.NumField(); i++ {
		fields = append(fields, typ.Field(i).Name)
	}

	if want := []string{"Subject", "Role", "ExpiresAt"}; !slices.Equal(fields, want) {
		t.Errorf("Principal fields = %v, want %v — a field added here reaches every caller", fields, want)
	}
}

// TestVerifyErrorsAreNotSentinel documents what callers may rely on: the
// middleware distinguishes "rejected" from "could not tell", because the second
// is an outage on our side and belongs in a different log line.
func TestVerifyErrorsAreNotSentinel(t *testing.T) {
	key := testsupport.NewRS256Key(t, "primary")
	set := testsupport.StartJWKS(t, key)
	verifier := newVerifier(t, set)

	claims := validClaims(set.Issuer)
	claims["exp"] = time.Now().Add(-time.Hour).Unix()

	_, err := verifier.Verify(t.Context(), key.Sign(t, claims))
	if !errors.Is(err, auth.ErrTokenRejected) {
		t.Errorf("Verify on an expired token = %v, want it to wrap ErrTokenRejected", err)
	}

	set.Break()
	broken := newVerifier(t, set)

	_, err = broken.Verify(t.Context(), key.Sign(t, validClaims(set.Issuer)))
	if !errors.Is(err, auth.ErrKeysUnavailable) {
		t.Errorf("Verify with an unreachable key set = %v, want it to wrap ErrKeysUnavailable", err)
	}
}

package token_test

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth/token"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

const testAudience = "authenticated"

// newVerifier wires a verifier to a local key set, the way the composition root
// wires one to Supabase: the JWKS address is derived from the issuer, never
// passed alongside it.
//
// permittedKIDs is the caller's session-key allowlist and is required: a test
// that forgets to name the kid it signs with fails at NewVerifier with "at
// least one permitted session key id is required", not at some assertion
// three lines later that only reports "Verify: unexpected error".
func newVerifier(t *testing.T, set *testsupport.JWKS, permittedKIDs ...string) *token.Verifier {
	t.Helper()

	verifier, err := token.NewVerifier(t.Context(), token.VerifierConfig{
		Issuer:      set.Issuer,
		Audience:    testAudience,
		JWKSURL:     set.Issuer + testsupport.JWKSPath,
		SessionKIDs: permittedKIDs,
	})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	return verifier
}

// TestNewVerifierRejectsAnEmptySessionKIDsList is this package's own copy of
// the "an empty list fails startup" requirement, independent of
// config.Load's. This package is what a flood of unknown kids would actually
// reach, so it must refuse to start on an empty allowlist even if the
// composition root's own check were ever skipped or bypassed.
func TestNewVerifierRejectsAnEmptySessionKIDsList(t *testing.T) {
	set := testsupport.StartJWKS(t, testsupport.NewRS256Key(t, "primary"))

	_, err := token.NewVerifier(t.Context(), token.VerifierConfig{
		Issuer:   set.Issuer,
		Audience: testAudience,
		JWKSURL:  set.Issuer + testsupport.JWKSPath,
	})
	if err == nil {
		t.Fatal("NewVerifier: want an error for an empty SessionKIDs list, got nil")
	}
}

// validClaims is a token that must be accepted. Every rejection test below is
// this set with exactly one thing changed, so a test that fails names the rule
// that rejected it rather than an unrelated omission.
func validClaims(issuer string) jwt.MapClaims {
	now := time.Now()

	return jwt.MapClaims{
		"sub":          "8a1f3b7c-0000-4000-8000-000000000001",
		"role":         "authenticated",
		"cadence_role": "patient",
		"aud":          testAudience,
		"iss":          issuer,
		"iat":          now.Unix(),
		"nbf":          now.Add(-time.Minute).Unix(),
		"exp":          now.Add(time.Hour).Unix(),
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
			verifier := newVerifier(t, set, key.KID)

			claims := validClaims(set.Issuer)
			principal, err := verifier.Verify(t.Context(), key.Sign(t, claims))
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}

			if principal.Subject != claims["sub"] {
				t.Errorf("Subject = %q, want %q", principal.Subject, claims["sub"])
			}
			// The product role, not the stock one. GoTrue puts "authenticated"
			// in role for every user, so a verifier reading that would hand the
			// same role to the patient, the doctor and the admin alike.
			if principal.Role != "patient" {
				t.Errorf("Role = %q, want patient", principal.Role)
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
			// Before pinning, this kid was refused because it was absent
			// from the published JWKS. Now it is refused one step earlier, by
			// the permitted-kid check in keyfunc, before the JWKS is ever
			// consulted for it — this case alone cannot tell the two apart
			// any more. See
			// TestVerifyRejectsAnImpermissibleKeyIDBeforeConsultingTheKeySet
			// for the test that isolates the new check specifically, by
			// proving key resolution is never reached at all.
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
			// "unpublished" is deliberately absent from the permitted list
			// too: the "signed by a key that is not published" case signs
			// under key.KID (permitted) with other's private key, and must be
			// caught by signature verification rather than by the kid check.
			verifier := newVerifier(t, set, key.KID)

			if _, err := verifier.Verify(t.Context(), tt.token(t, key, other, set.Issuer)); err == nil {
				t.Fatal("Verify: want error, got nil")
			}
		})
	}
}

// TestVerifyRejectsAnImpermissibleKeyIDBeforeConsultingTheKeySet is the
// ordering proof the AC asks for directly: the permitted-kid check "stands
// before key resolution, so a flood of unknown kids never reaches the JWKS
// refresh budget."
//
// Every kid below is both unpublished and unpermitted, and distinct from one
// another, so the library's own per-kid state cannot short-circuit a second
// look at the same one. If the permitted check ran after key resolution
// instead of before it, each one would still cost at least an attempt at the
// on-demand refresh — one real HTTP request the first time, per
// TestVerifyRateLimitsUnknownKeyIDRefresh's measurement of that path. Running
// the check first means resolve is never reached at all, so the fixture sees
// zero requests, not "at most one": that is the difference this test makes
// observable, plus the refusal naming the kid and the reason in its message.
func TestVerifyRejectsAnImpermissibleKeyIDBeforeConsultingTheKeySet(t *testing.T) {
	key := testsupport.NewRS256Key(t, "primary")
	set := testsupport.StartJWKS(t, key)
	verifier := newVerifier(t, set, key.KID)

	// One fetch to fill the cache and get it out of the count below.
	if _, err := verifier.Verify(t.Context(), key.Sign(t, validClaims(set.Issuer))); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	before := set.Requests()

	for i := range 5 {
		kid := fmt.Sprintf("published-but-impermissible-%d", i)
		signed := key.SignWithKID(t, kid, validClaims(set.Issuer))

		_, err := verifier.Verify(t.Context(), signed)
		if err == nil {
			t.Fatalf("Verify with kid %q: want error, got nil", kid)
		}
		if !errors.Is(err, token.ErrTokenRejected) {
			t.Errorf("kid %q: err = %v, want it to wrap ErrTokenRejected", kid, err)
		}
		if !strings.Contains(err.Error(), kid) {
			t.Errorf("kid %q: error %q does not name the kid", kid, err)
		}
		if !strings.Contains(err.Error(), "permitted") {
			t.Errorf("kid %q: error %q does not name the reason", kid, err)
		}
	}

	if requests := set.Requests() - before; requests > 0 {
		t.Errorf("the key set was fetched %d times for 5 impermissible kids, want 0 — "+
			"the permitted-kid check must run before key resolution", requests)
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
	verifier := newVerifier(t, set, key.KID)

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
	verifier := newVerifier(t, set, key.KID)

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
	verifier := newVerifier(t, set, key.KID)

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

	verifier := newVerifier(t, set, key.KID)

	if _, err := verifier.Verify(t.Context(), key.Sign(t, validClaims(set.Issuer))); err == nil {
		t.Fatal("Verify: want error while the key set is unreachable, got nil")
	}
}

// TestVerifyRateLimitsUnknownKeyIDRefresh guards the availability side. Every
// unknown kid triggering a fetch turns a stream of junk tokens into a request
// flood against the provider, whose rate limiter then takes down authentication
// for everyone.
//
// The junk kids are on the permitted list, not off it. Pinning refuses an
// impermissible kid before the key set is ever consulted (see
// TestVerifyRejectsAnImpermissibleKeyIDBeforeConsultingTheKeySet), which would
// make this test pass at zero requests for the wrong reason if its kids were
// merely unpermitted. Permitting them in advance is how a kid still reaches
// the JWKS library's own unknown-kid handling, which is the rate limiter this
// test is actually about.
func TestVerifyRateLimitsUnknownKeyIDRefresh(t *testing.T) {
	key := testsupport.NewRS256Key(t, "primary")
	set := testsupport.StartJWKS(t, key)

	permitted := []string{key.KID}
	for i := range 20 {
		permitted = append(permitted, fmt.Sprintf("unknown-%d", i))
	}
	verifier := newVerifier(t, set, permitted...)

	// One fetch to fill the cache, so the count below is refreshes and not the
	// initial load.
	if _, err := verifier.Verify(t.Context(), key.Sign(t, validClaims(set.Issuer))); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	before := set.Requests()

	for i := range 20 {
		junk := key.SignWithKID(t, fmt.Sprintf("unknown-%d", i), validClaims(set.Issuer))
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

	// As in TestVerifyRateLimitsUnknownKeyIDRefresh: these kids must be
	// permitted so they still reach classifyKeyFailure through the JWKS
	// library's own unknown-kid path, rather than being turned away earlier
	// by the pinning check — which would also produce ErrTokenRejected, but
	// without exercising the outage-vs-refusal distinction this test names.
	verifier := newVerifier(t, set, key.KID, "unknown-0", "unknown-1", "unknown-2", "unknown-3", "unknown-4")

	if _, err := verifier.Verify(t.Context(), key.Sign(t, validClaims(set.Issuer))); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	for i := range 5 {
		junk := key.SignWithKID(t, fmt.Sprintf("unknown-%d", i), validClaims(set.Issuer))

		_, err := verifier.Verify(t.Context(), junk)
		if !errors.Is(err, token.ErrTokenRejected) {
			t.Errorf("unknown kid %d: err = %v, want it to wrap ErrTokenRejected", i, err)
		}
		if errors.Is(err, token.ErrKeysUnavailable) {
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
//
// Before pinning, "rotated" needed no advance permission: any kid the key set
// published was trusted. Pinning deliberately removes that — see the
// Architecture decision in the spec — so this test now names both kids on the
// permitted list from the start, matching the documented rotation order
// (VerifierConfig.SessionKIDs): the new kid joins the list before GoTrue is
// ever told to sign with it. What is left to prove is narrower and still real:
// a key GoTrue starts signing with while the verifier is already running must
// still be picked up live, over a real round trip, or every rotation would
// require restarting the API in lockstep with GoTrue.
func TestVerifyPicksUpRotatedKeysOverANetwork(t *testing.T) {
	old := testsupport.NewRS256Key(t, "old")
	set := testsupport.StartJWKS(t, old)

	// A slow but entirely ordinary round trip: a mobile network, a cold TLS
	// handshake, a provider having a bad minute. The number is chosen to be
	// larger than any budget that would only ever have worked on localhost, so
	// that shrinking the budget fails this test rather than squeaking past it.
	set.Delay(800 * time.Millisecond)

	// Deliberately the production defaults: the budget under test is one of them.
	verifier := newVerifier(t, set, "old", "rotated")

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
// has to become usable without a restart — provided its kid was already
// permitted, per the rotation order on VerifierConfig.SessionKIDs. See
// TestVerifyPicksUpRotatedKeysOverANetwork for why both kids are permitted in
// advance rather than only the original one.
func TestVerifyPicksUpRotatedKeys(t *testing.T) {
	old := testsupport.NewRS256Key(t, "old")
	set := testsupport.StartJWKS(t, old)

	verifier, err := token.NewVerifier(t.Context(), token.VerifierConfig{
		Issuer:             set.Issuer,
		Audience:           testAudience,
		JWKSURL:            set.Issuer + testsupport.JWKSPath,
		SessionKIDs:        []string{"old", "rotated"},
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

// TestVerifyRefusesAKeyRotatedInWithoutBeingPermittedFirst is the negative
// half TestVerifyPicksUpRotatedKeys alone cannot prove: that pinning actually
// changed the behaviour, rather than the two tests above having merely grown
// an unused parameter. GoTrue rotates to a kid nobody added to SessionKIDs —
// the reverse of the documented order, where the signing marker moves before
// the API is told about the new kid — and every token the new key signs must
// still be refused, live traffic and all, exactly as the AC requires: "the
// reverse order refuses all authenticated traffic".
func TestVerifyRefusesAKeyRotatedInWithoutBeingPermittedFirst(t *testing.T) {
	old := testsupport.NewRS256Key(t, "old")
	set := testsupport.StartJWKS(t, old)

	// Only "old" is permitted — "rotated" is deliberately absent, standing in
	// for an operator who handed GoTrue the new signing marker before adding
	// the kid to SessionKIDs.
	verifier := newVerifier(t, set, "old")

	if _, err := verifier.Verify(t.Context(), old.Sign(t, validClaims(set.Issuer))); err != nil {
		t.Fatalf("Verify with the original key: %v", err)
	}

	rotated := testsupport.NewRS256Key(t, "rotated")
	set.Publish(t, old, rotated)

	_, err := verifier.Verify(t.Context(), rotated.Sign(t, validClaims(set.Issuer)))
	if err == nil {
		t.Fatal("Verify: want an error for a kid rotated in without being permitted first, got nil")
	}
	if !errors.Is(err, token.ErrTokenRejected) {
		t.Errorf("err = %v, want it to wrap ErrTokenRejected", err)
	}
	if !strings.Contains(err.Error(), "rotated") {
		t.Errorf("error %q does not name the kid", err)
	}
}

// TestVerifyRejectsTokenWithoutSubject keeps a principal from existing without
// an identity: everything downstream — impersonation, RLS, audit — is keyed on
// it, and an empty subject would silently become a valid one.
func TestVerifyRejectsTokenWithoutSubject(t *testing.T) {
	key := testsupport.NewRS256Key(t, "primary")
	set := testsupport.StartJWKS(t, key)
	verifier := newVerifier(t, set, key.KID)

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
	verifier := newVerifier(t, set, key.KID)

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
	verifier := newVerifier(t, set, key.KID)

	claims := validClaims(set.Issuer)
	claims["exp"] = time.Now().Add(-time.Hour).Unix()

	_, err := verifier.Verify(t.Context(), key.Sign(t, claims))
	if !errors.Is(err, token.ErrTokenRejected) {
		t.Errorf("Verify on an expired token = %v, want it to wrap ErrTokenRejected", err)
	}

	set.Break()
	broken := newVerifier(t, set, key.KID)

	_, err = broken.Verify(t.Context(), key.Sign(t, validClaims(set.Issuer)))
	if !errors.Is(err, token.ErrKeysUnavailable) {
		t.Errorf("Verify with an unreachable key set = %v, want it to wrap ErrKeysUnavailable", err)
	}
}

// A token with only the stock role claim carries no product role, and that is a
// real state rather than a malformed token: the issuance hook removes
// cadence_role for a user with no profile — an invited account that has not been
// provisioned. The verifier hands back an empty role and lets the impersonation
// seam refuse it, with a reason of its own; refusing here would make every
// unprovisioned account indistinguishable from a bad token.
func TestVerifyLeavesTheRoleEmptyWhenTheTokenCarriesOnlyTheStockClaim(t *testing.T) {
	key := testsupport.NewES256Key(t, "kid-1")
	set := testsupport.StartJWKS(t, key)
	verifier := newVerifier(t, set, key.KID)

	claims := validClaims(set.Issuer)
	delete(claims, "cadence_role")

	principal, err := verifier.Verify(t.Context(), key.Sign(t, claims))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if principal.Role != "" {
		t.Errorf("Role = %q, want empty — the stock role claim is not a product role",
			principal.Role)
	}
	if principal.Subject != claims["sub"] {
		t.Errorf("Subject = %q, want %q", principal.Subject, claims["sub"])
	}
}

// `aud` arrives as a list in the first token GoTrue issues after every start,
// and as a string in all the rest. RFC 7519 permits both, and this API is handed
// both — so the shape cannot be assumed from a sample of one sign-in.
//
// The cause is a global in golang-jwt that supabase/gotrue:v2.194.0 sets inside
// its own signing routine: `jwt.MarshalSingleStringAsArray` starts true and is
// set to false the first time a token is signed. The issuance hook's event is
// marshalled before that has happened, so the very first token carries the list
// form all the way through. Measured against supabase/gotrue:v2.194.0 on
// 2026-08-10: restart, sign in five times, and the first `aud` is
// `["authenticated"]` while the next four are `"authenticated"` — twice over,
// across two restarts.
//
// One token per deployment sounds like nothing until it is the first sign-in
// after every release, which is the one somebody is watching.
//
// Both halves are here on purpose. The accepting one guards that shape; the
// refusing one is what makes it an audience check rather than a test that passes
// because the claim is no longer read.
func TestVerifyAcceptsTheAudienceInEitherShape(t *testing.T) {
	key := testsupport.NewES256Key(t, "kid-1")
	set := testsupport.StartJWKS(t, key)
	verifier := newVerifier(t, set, key.KID)

	// The one-element list first, because that is the shape production emits;
	// the longer one because RFC 7519 permits it and a naive check that compared
	// the whole list against one string would pass the first and fail the second.
	for _, audience := range [][]string{{testAudience}, {"someone-else", testAudience}} {
		accepted := validClaims(set.Issuer)
		accepted["aud"] = audience

		if _, err := verifier.Verify(t.Context(), key.Sign(t, accepted)); err != nil {
			t.Errorf("Verify with aud = %v: %v", audience, err)
		}
	}

	refused := validClaims(set.Issuer)
	refused["aud"] = []string{"someone-else", "and-another"}

	if _, err := verifier.Verify(t.Context(), key.Sign(t, refused)); !errors.Is(err, token.ErrTokenRejected) {
		t.Errorf("Verify with a list naming other audiences = %v, want ErrTokenRejected", err)
	}
}

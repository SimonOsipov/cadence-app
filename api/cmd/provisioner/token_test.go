package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth/token"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// TestTheAdminTokenExpiresWithinAMinute. The token is minted per call and
// travels one hop; a lifetime long enough to be worth stealing buys nothing.
// Sixty seconds is the ceiling the spec fixes, and the assertion is on the
// claim rather than on the constant, so a lifetime taken from somewhere else
// still has to satisfy it.
func TestTheAdminTokenExpiresWithinAMinute(t *testing.T) {
	signer := testSigner(t, testsupport.NewES256Key(t, "admin-key"))

	issued := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)

	claims := parseWithoutVerifying(t, mint(t, signer, issued))

	expiry, err := claims.GetExpirationTime()
	if err != nil || expiry == nil {
		t.Fatalf("the token carries no expiry: %v", err)
	}

	lifetime := expiry.Sub(issued)
	if lifetime <= 0 || lifetime > time.Minute {
		t.Fatalf("the token lives %s, want a positive lifetime no longer than a minute", lifetime)
	}
}

// TestTheAdminTokenNamesTheServiceRole is what GoTrue actually checks — the
// only claim on the admin routes that it looks at. Without it the four
// refusals below would be satisfied by a token nothing accepts.
func TestTheAdminTokenNamesTheServiceRole(t *testing.T) {
	signer := testSigner(t, testsupport.NewES256Key(t, "admin-key"))

	claims := parseWithoutVerifying(t, mint(t, signer, time.Now()))

	if claims["role"] != "service_role" {
		t.Fatalf("the token names role %v, which GoTrue's admin routes do not admit", claims["role"])
	}
}

// TestTheAPIVerifierRefusesTheAdminTokenForEachReasonSeparately.
//
// The chain this closes was measured: whoever holds the admin key can set any
// account's password and then obtain a session token by entirely legitimate
// means. Pinning the session `kid` is one barrier; GoTrue checks neither `aud`
// nor `iss` on its admin routes, which makes a foreign audience and a foreign
// issuer two more barriers that cost nothing.
//
// Each is asserted on its own, with the other two neutralised, because three
// reasons asserted together are one reason with two spares: a regression that
// lost the `kid` pinning would be invisible behind an audience the verifier
// also disliked.
func TestTheAPIVerifierRefusesTheAdminTokenForEachReasonSeparately(t *testing.T) {
	admin := testsupport.NewES256Key(t, "admin-key")
	set := testsupport.StartJWKS(t, admin)

	minted := mint(t, testSigner(t, admin), time.Now())

	tests := []struct {
		name string
		// The verifier is configured to agree with the token about everything
		// but the one thing under test.
		config token.VerifierConfig
		want   error
		says   string
	}{
		{
			name: "the key id is not a permitted session key id",
			config: token.VerifierConfig{
				Issuer:      adminTokenIssuer,
				Audience:    adminTokenAudience,
				SessionKIDs: []string{"a-session-key"},
			},
			want: token.ErrTokenRejected,
			says: "not on the permitted session key id list",
		},
		{
			name: "the audience is foreign",
			config: token.VerifierConfig{
				Issuer:      adminTokenIssuer,
				Audience:    "authenticated",
				SessionKIDs: []string{admin.KID},
			},
			want: jwt.ErrTokenInvalidAudience,
		},
		{
			name: "the issuer is foreign",
			config: token.VerifierConfig{
				Issuer:      "http://localhost:9999",
				Audience:    adminTokenAudience,
				SessionKIDs: []string{admin.KID},
			},
			want: jwt.ErrTokenInvalidIssuer,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.config
			cfg.JWKSURL = set.Issuer + testsupport.JWKSPath

			verifier, err := token.NewVerifier(t.Context(), cfg)
			if err != nil {
				t.Fatalf("building the verifier: %v", err)
			}

			principal, err := verifier.Verify(t.Context(), minted)
			if err == nil {
				t.Fatalf("the API accepted the admin token as %s", principal.Subject)
			}

			if !errors.Is(err, tc.want) {
				t.Fatalf("refused with %v, want %v", err, tc.want)
			}

			if tc.says != "" && !strings.Contains(err.Error(), tc.says) {
				t.Errorf("the refusal does not say %q: %v", tc.says, err)
			}
		})
	}
}

// TestTheThreeBarriersAreTheOnlyThingRefusingTheAdminToken is the control the
// three subtests above need. Each of them asserts a refusal, and a token that
// was malformed — or signed with a key the fixture does not publish — would
// produce three refusals for a reason that has nothing to do with the barriers
// they name.
//
// So: a verifier that agrees about the key id, the audience and the issuer gets
// through all three and past the signature. What stops it there is the fourth
// barrier, which is free in the same way and was not designed for: the token
// names no subject, because it stands for no person, and the verifier refuses a
// principal it cannot name. Asserting that specific refusal is what says the
// other three were passed rather than merely reordered.
func TestTheThreeBarriersAreTheOnlyThingRefusingTheAdminToken(t *testing.T) {
	admin := testsupport.NewES256Key(t, "admin-key")
	set := testsupport.StartJWKS(t, admin)

	verifier, err := token.NewVerifier(t.Context(), token.VerifierConfig{
		Issuer:      adminTokenIssuer,
		Audience:    adminTokenAudience,
		JWKSURL:     set.Issuer + testsupport.JWKSPath,
		SessionKIDs: []string{admin.KID},
	})
	if err != nil {
		t.Fatalf("building the verifier: %v", err)
	}

	_, err = verifier.Verify(t.Context(), mint(t, testSigner(t, admin), time.Now()))
	if err == nil {
		t.Fatal("a token naming no subject was accepted as a principal")
	}

	if !strings.Contains(err.Error(), "names no subject") {
		t.Fatalf("refused with %v; the three barriers were meant to be the only ones under test, "+
			"so anything else here means the subtests above are passing for a reason of their own", err)
	}
}

// TestTheAdminTokenNamesNoSubject states the fourth barrier as an intention
// rather than leaving it as an accident of the control above. A subject is the
// name of a person, and this token is minted by a process on its own behalf; a
// `sub` here would be a person's identifier attached to a machine's authority,
// which is the exact confusion the audit actor work in step-4 removed from the
// other direction.
func TestTheAdminTokenNamesNoSubject(t *testing.T) {
	claims := parseWithoutVerifying(t, mint(t, testSigner(t, testsupport.NewES256Key(t, "admin-key")), time.Now()))

	if subject, found := claims["sub"]; found {
		t.Fatalf("the admin token names subject %v", subject)
	}
}

// TestTheSignerRefusesAKeyItCannotSignWith. The admin key arrives as one line
// of configuration; every way it can be wrong has to stop the process rather
// than produce a token nothing accepts and an incident nobody can read.
func TestTheSignerRefusesAKeyItCannotSignWith(t *testing.T) {
	public := `{"kty":"EC","crv":"P-256","alg":"ES256","use":"sig","kid":"admin-key",` +
		`"x":"2e9iv7Gxg80pCO61rNEtPEs-mIr1mErwg8vW7QcugY8",` +
		`"y":"9K6Q4xt--k5fF48WfBjVeFnxsn3NAJY-so03DMZnEH0"}`

	tests := []struct {
		name string
		jwk  string
	}{
		{name: "nothing at all", jwk: ""},
		{name: "not JSON", jwk: "-----BEGIN EC PRIVATE KEY-----"},
		{name: "a public key with no private half", jwk: public},
		{
			name: "a key with no id",
			jwk: strings.Replace(testsupport.NewES256Key(t, "admin-key").PrivateJWK(t),
				`"kid":"admin-key",`, "", 1),
		},
		{name: "an array of keys, the way GoTrue is configured", jwk: "[" + public + "]"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := newTokenSigner(tc.jwk); err == nil {
				t.Fatal("the signer accepted it")
			}
		})
	}
}

func testSigner(t *testing.T, key *testsupport.SigningKey) *tokenSigner {
	t.Helper()

	signer, err := newTokenSigner(key.PrivateJWK(t))
	if err != nil {
		t.Fatalf("building the token signer: %v", err)
	}

	return signer
}

func mint(t *testing.T, signer *tokenSigner, now time.Time) string {
	t.Helper()

	minted, err := signer.mint(now)
	if err != nil {
		t.Fatalf("minting the admin token: %v", err)
	}

	return minted
}

func tokenKID(t *testing.T, minted string) string {
	t.Helper()

	parsed, _, err := jwt.NewParser().ParseUnverified(minted, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("reading the minted token: %v", err)
	}

	kid, _ := parsed.Header["kid"].(string)

	return kid
}

func parseWithoutVerifying(t *testing.T, minted string) jwt.MapClaims {
	t.Helper()

	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(minted, claims); err != nil {
		t.Fatalf("reading the minted token: %v", err)
	}

	return claims
}

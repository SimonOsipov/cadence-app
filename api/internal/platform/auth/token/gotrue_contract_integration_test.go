//go:build integration

// The contract with the identity provider: five behaviours of supabase/gotrue
// that are measured rather than documented, and that the trust boundary rests
// on. They live beside the verifier because the verifier is what they hold up —
// pinning a `kid` is worth nothing if the admin token is accepted against any
// configured key, and knowing that it is, is what moved the admin key out of
// the API process altogether.
//
// The image is pinned by digest. That is the whole mechanism: an upgrade that
// changes any of these behaviours turns this file red instead of reaching a
// deployment, and each behaviour is a test of its own so the red one names
// which behaviour moved.
package token_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

var gotrueCluster *testsupport.Cluster

func TestMain(m *testing.M) {
	ctx := context.Background()

	cluster, err := testsupport.StartCluster(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting the test cluster: %v\n", err)
		os.Exit(1)
	}
	gotrueCluster = cluster

	code := m.Run()

	if err := gotrueCluster.Terminate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "terminating the test cluster: %v\n", err)
	}

	os.Exit(code)
}

// served is what an admin route that actually ran answers with: the user list,
// empty on a database nothing has created an account in. Asserting it rather
// than the status alone is what tells "GoTrue admitted the token" apart from
// "something answered 200".
const served = `"users"`

// adminClaims are what a token has to carry to be admitted to GoTrue's admin
// routes: the role, and nothing else that is checked. Every test below starts
// from these and changes exactly one thing, so a 200 says which claim was
// ignored.
func adminClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"role": "service_role",
		"aud":  testsupport.GoTrueAudience,
		"iss":  testsupport.GoTrueIssuer,
		"exp":  time.Now().Add(time.Minute).Unix(),
	}
}

// GET /admin/users is the cheapest route behind the admin check that reads the
// database, so a 200 also says the process is genuinely serving rather than
// answering out of its router. The body is returned because a refusal that
// names its reason is the difference between a control and a coincidence.
func adminCall(t *testing.T, gotrue *testsupport.GoTrue, token string) (int, string) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, gotrue.URL+"/admin/users", nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("calling the admin route: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the answer: %v", err)
	}

	return resp.StatusCode, string(body)
}

// The first of the five, and the one the whole model rests on: GoTrue verifies
// an admin token against every key in GOTRUE_JWT_KEYS, not against the one it
// signs sessions with. Pinning the session `kid` in our verifier therefore does
// nothing to stop the holder of the second key from reaching the admin routes —
// which is why the key lives in a process of its own.
//
// The third case is the control. Without it a route that answered 200 to
// anything would pass the first two.
func TestGoTrueAcceptsAnAdminTokenSignedByAnyConfiguredKey(t *testing.T) {
	session := testsupport.NewES256Key(t, "session-key")
	admin := testsupport.NewES256Key(t, "admin-key")
	stranger := testsupport.NewES256Key(t, "stranger-key")

	gotrue := testsupport.StartGoTrue(t, gotrueCluster, testsupport.GoTrueJWKS(
		t,
		testsupport.JWKEntry{Key: session, Signing: true},
		testsupport.JWKEntry{Key: admin},
	))

	tests := []struct {
		name   string
		key    *testsupport.SigningKey
		status int
		says   string
	}{
		{name: "the key GoTrue signs sessions with", key: session, status: http.StatusOK, says: served},
		{name: "the configured key carrying no signing marker", key: admin, status: http.StatusOK, says: served},
		{
			name:   "a key GoTrue is not configured with",
			key:    stranger,
			status: http.StatusForbidden,
			// The reason is asserted, not just the refusal: a 403 for a claim
			// GoTrue disliked would be a control that proves nothing about keys.
			says: `"error_code":"bad_jwt"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, body := adminCall(t, gotrue, tc.key.Sign(t, adminClaims()))
			if status != tc.status || !strings.Contains(body, tc.says) {
				t.Fatalf("admin route answered %d %s, want %d containing %s", status, body, tc.status, tc.says)
			}
		})
	}
}

// The second: `aud` is not checked on the admin routes, though GoTrue is
// configured with one and puts it in every session token it issues. That makes
// a foreign `aud` a free second reason for our own verifier to refuse an admin
// token — free because GoTrue does not mind.
func TestGoTrueDoesNotCheckTheAudienceOfAnAdminToken(t *testing.T) {
	session := testsupport.NewES256Key(t, "session-key")

	gotrue := testsupport.StartGoTrue(t, gotrueCluster, testsupport.GoTrueJWKS(
		t,
		testsupport.JWKEntry{Key: session, Signing: true},
	))

	claims := adminClaims()
	claims["aud"] = "cadence-provisioner"

	status, body := adminCall(t, gotrue, session.Sign(t, claims))
	if status != http.StatusOK || !strings.Contains(body, served) {
		t.Fatalf("admin route answered %d %s for a foreign audience, want %d containing %s",
			status, body, http.StatusOK, served)
	}
}

// The third, and the same bargain as the audience: `iss` is not checked either,
// so the provisioner's token can name an issuer our verifier has never heard of
// and still be admitted here.
func TestGoTrueDoesNotCheckTheIssuerOfAnAdminToken(t *testing.T) {
	session := testsupport.NewES256Key(t, "session-key")

	gotrue := testsupport.StartGoTrue(t, gotrueCluster, testsupport.GoTrueJWKS(
		t,
		testsupport.JWKEntry{Key: session, Signing: true},
	))

	claims := adminClaims()
	claims["iss"] = "https://provisioner.invalid"

	status, body := adminCall(t, gotrue, session.Sign(t, claims))
	if status != http.StatusOK || !strings.Contains(body, served) {
		t.Fatalf("admin route answered %d %s for a foreign issuer, want %d containing %s",
			status, body, http.StatusOK, served)
	}
}

// The fourth and fifth are what make "exactly one key signs sessions" a
// property of the arrangement rather than an operator's promise: both
// degenerate configurations are refused by GoTrue itself, at startup, loudly.
// Neither can reach a deployment as a silently mis-signing process.
func TestGoTrueRefusesToStartWithNoSigningKey(t *testing.T) {
	first := testsupport.NewES256Key(t, "first-key")
	second := testsupport.NewES256Key(t, "second-key")

	logs := testsupport.StartGoTrueExpectingRefusal(t, gotrueCluster, testsupport.GoTrueJWKS(
		t,
		testsupport.JWKEntry{Key: first},
		testsupport.JWKEntry{Key: second},
	))

	if !strings.Contains(logs, "no signing key found") {
		t.Fatalf("GoTrue exited without saying %q; its log was:\n%s", "no signing key found", logs)
	}
}

func TestGoTrueRefusesToStartWithTwoSigningKeys(t *testing.T) {
	first := testsupport.NewES256Key(t, "first-key")
	second := testsupport.NewES256Key(t, "second-key")

	logs := testsupport.StartGoTrueExpectingRefusal(t, gotrueCluster, testsupport.GoTrueJWKS(
		t,
		testsupport.JWKEntry{Key: first, Signing: true},
		testsupport.JWKEntry{Key: second, Signing: true},
	))

	if !strings.Contains(logs, "multiple signing keys detected") {
		t.Fatalf("GoTrue exited without saying %q; its log was:\n%s", "multiple signing keys detected", logs)
	}
}

// The five tests above are worth exactly as much as the claim that the image
// they run against is the image everything else runs against. The digest is
// written twice — here and in the development stack — and nothing but this
// makes the two move together.
func TestTheContractRunsAgainstTheImageTheStackRuns(t *testing.T) {
	compose, err := os.ReadFile(filepath.Join(testsupport.ModuleRoot(t), "docker-compose.yml"))
	if err != nil {
		t.Fatalf("reading the development stack: %v", err)
	}

	// A whole line rather than a substring anywhere in the file, and measured
	// rather than assumed: the stack keeps the recipe for resolving a digest in
	// prose beside the line, and commenting the old `image:` out is the ordinary
	// way to upgrade one. `# image: <digest>` contains `image: <digest>`, so
	// both weaker forms of this check stay green on exactly the upgrade it
	// exists to catch.
	runs := slices.ContainsFunc(strings.Split(string(compose), "\n"), func(line string) bool {
		return strings.TrimSpace(line) == "image: "+testsupport.GoTrueImage
	})

	if !runs {
		t.Fatalf("no line of docker-compose.yml is %q — either the contract tests are pinning an image the stack "+
			"no longer runs, or the line was reformatted and this check needs to learn the new shape",
			"image: "+testsupport.GoTrueImage)
	}
}

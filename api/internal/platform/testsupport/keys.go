package testsupport

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
)

// JWKSPath is where the fixture publishes its key set, relative to the issuer.
// It matches what an OAuth 2.0 authorisation server serves (RFC 8414) and what
// the configuration derives, so a test exercises the derivation rather than
// being handed the address.
const JWKSPath = "/.well-known/jwks.json"

// issuerPath mirrors the shape of a real Supabase issuer —
// https://<ref>.supabase.co/auth/v1 — rather than sitting at the root. An
// issuer with a path is the case where a naive join produces the wrong JWKS
// address, so it is the one the fixture uses by default.
const issuerPath = "/auth/v1"

// SigningKey is one key pair: what the JWKS fixture publishes and what the
// token helpers sign with.
type SigningKey struct {
	// KID is the key id, sent in the token header and matched against the set.
	KID string

	// Alg is the JWS algorithm name, e.g. RS256.
	Alg string

	method  jwt.SigningMethod
	private crypto.Signer
}

// NewRS256Key generates an RSA key pair for signing test tokens.
//
// 2048 bits rather than 4096: this is generated per test, and the cost of the
// larger modulus buys nothing a test can observe.
func NewRS256Key(t *testing.T, kid string) *SigningKey {
	t.Helper()

	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	return &SigningKey{KID: kid, Alg: "RS256", method: jwt.SigningMethodRS256, private: private}
}

// NewES256Key generates an ECDSA P-256 key pair for signing test tokens.
//
// Both algorithms exist in the fixtures because the allowlist declares both,
// and a list has to be proven to accept everything on it rather than whichever
// entry happened to be written first. Our GoTrue signs ES256; RS256 is the one
// that would otherwise go untested until the day somebody reconfigures it.
func NewES256Key(t *testing.T, kid string) *SigningKey {
	t.Helper()

	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating ECDSA key: %v", err)
	}

	return &SigningKey{KID: kid, Alg: "ES256", method: jwt.SigningMethodES256, private: private}
}

// Sign mints a token carrying claims, with this key's kid in the header.
func (k *SigningKey) Sign(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()

	return k.sign(t, k.method, claims, map[string]any{"kid": k.KID})
}

// SignWithoutKID mints a token with no kid header.
//
// A key set with one key would verify it anyway, which is exactly the reason
// the fixture can produce it: a token that names no key must be refused because
// the rule says so, not because the fixture happened to hold two keys.
func (k *SigningKey) SignWithoutKID(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()

	return k.sign(t, k.method, claims, nil)
}

// SignWithKID mints a token claiming a kid this key does not have.
func (k *SigningKey) SignWithKID(t *testing.T, kid string, claims jwt.MapClaims) string {
	t.Helper()

	return k.sign(t, k.method, claims, map[string]any{"kid": kid})
}

// PrivateJWK renders the key as one JWK carrying its private half.
//
// It is the shape a single key is configured with — one variable, one key, the
// key id travelling with the material it names so the two cannot drift apart —
// as opposed to the bare array GOTRUE_JWT_KEYS takes, which GoTrueJWKS builds
// out of the same rendering.
func (k *SigningKey) PrivateJWK(t *testing.T) string {
	t.Helper()

	raw, err := json.Marshal(privateJWKMarshal(t, k, false))
	if err != nil {
		t.Fatalf("marshalling the private JWK for %q: %v", k.KID, err)
	}

	return string(raw)
}

// privateJWKMarshal renders one key, with the signing marker if signing is set.
// The marker is what GoTrue reads to pick the key it signs sessions with; every
// other consumer of a private JWK here wants the key without it.
func privateJWKMarshal(t *testing.T, key *SigningKey, signing bool) jwkset.JWKMarshal {
	t.Helper()

	metadata := jwkset.JWKMetadataOptions{
		ALG: jwkset.ALG(key.Alg),
		KID: key.KID,
		USE: jwkset.UseSig,
	}
	if signing {
		metadata.KEYOPS = []jwkset.KEYOPS{jwkset.KeyOpsSign}
	}

	jwk, err := jwkset.NewJWKFromKey(key.private, jwkset.JWKOptions{
		Marshal:  jwkset.JWKMarshalOptions{Private: true},
		Metadata: metadata,
	})
	if err != nil {
		t.Fatalf("building the JWK for %q: %v", key.KID, err)
	}

	return jwk.Marshal()
}

// PublicKeyPEM returns the PEM-encoded public key.
//
// It exists for one test: signing a token with HS256 using the published public
// key as the shared secret. That is the algorithm-confusion attack the closed
// algorithm list is there to stop, and it can only be written if the attacker's
// view of the key — the public one, which is public — is reachable.
func (k *SigningKey) PublicKeyPEM(t *testing.T) []byte {
	t.Helper()

	der, err := x509.MarshalPKIXPublicKey(k.private.Public())
	if err != nil {
		t.Fatalf("marshalling public key: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
}

// SignHS256 mints a token signed with HMAC-SHA256 over secret, carrying this
// key's kid. See PublicKeyPEM for why.
func SignHS256(t *testing.T, kid string, secret []byte, claims jwt.MapClaims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = kid

	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}

	return signed
}

// SignAs mints a token signed with this key under a different algorithm.
//
// The key is legitimate and published; only the algorithm changes. That is the
// case the algorithm allowlist actually owns: an RSA public key verifies RS256,
// RS384, RS512, PS256, PS384 and PS512 alike, so a token the provider would
// never issue still checks out unless the server insists on the algorithms it
// expects.
func (k *SigningKey) SignAs(t *testing.T, method jwt.SigningMethod, claims jwt.MapClaims) string {
	t.Helper()

	return k.sign(t, method, claims, map[string]any{"kid": k.KID})
}

// SignNone mints an unsigned token: `"alg": "none"`, with an empty signature.
//
// The oldest JWT attack there is. It has to be constructed by hand because
// golang-jwt refuses to produce one without being handed a sentinel value, and
// a rule that is never tested against the attack it names is a comment.
func SignNone(t *testing.T, kid string, claims jwt.MapClaims) string {
	t.Helper()

	encode := func(v any) string {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshalling token part: %v", err)
		}

		return base64.RawURLEncoding.EncodeToString(raw)
	}

	header := encode(map[string]any{"alg": "none", "typ": "JWT", "kid": kid})

	return header + "." + encode(map[string]any(claims)) + "."
}

func (k *SigningKey) sign(
	t *testing.T, method jwt.SigningMethod, claims jwt.MapClaims, header map[string]any,
) string {
	t.Helper()

	token := jwt.NewWithClaims(method, claims)
	for name, value := range header {
		token.Header[name] = value
	}
	// jwt.NewWithClaims always sets a kid-less header; delete rather than skip so
	// that SignWithoutKID cannot be defeated by a future default.
	if _, ok := header["kid"]; !ok {
		delete(token.Header, "kid")
	}

	signed, err := token.SignedString(k.private)
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}

	return signed
}

// JWKS is a local JWK Set endpoint standing in for the identity provider.
type JWKS struct {
	// Issuer is what tokens claim in `iss` and what the API is configured with.
	// The key set is served underneath it, at JWKSPath.
	Issuer string

	server     *httptest.Server
	requests   atomic.Int64
	declareAlg bool

	mu     sync.Mutex
	body   []byte
	broken bool
	delay  time.Duration
}

// StartJWKS serves the public halves of keys and stops when the test ends.
func StartJWKS(t *testing.T, keys ...*SigningKey) *JWKS {
	t.Helper()

	return startJWKS(t, true, keys)
}

// StartJWKSWithoutAlg serves the same keys with no `alg` member on each JWK.
//
// The member is optional in RFC 7517 and a provider is free to omit it. That
// matters because a key set that declares an algorithm gives the JWKS library a
// check of its own, which can mask a missing allowlist on our side — a test
// written against a set that declares one would pass with the allowlist
// deleted. This fixture removes the crutch.
func StartJWKSWithoutAlg(t *testing.T, keys ...*SigningKey) *JWKS {
	t.Helper()

	return startJWKS(t, false, keys)
}

func startJWKS(t *testing.T, declareAlg bool, keys []*SigningKey) *JWKS {
	t.Helper()

	set := &JWKS{declareAlg: declareAlg, body: jwksJSON(t, declareAlg, keys)}

	mux := http.NewServeMux()
	mux.HandleFunc(issuerPath+JWKSPath, set.serve)

	set.server = httptest.NewServer(mux)
	t.Cleanup(set.server.Close)

	set.Issuer = set.server.URL + issuerPath

	return set
}

// Requests counts how many times the key set has been fetched. It is how a test
// observes that a refresh was rate-limited rather than trusting that it was.
func (j *JWKS) Requests() int64 {
	return j.requests.Load()
}

// Publish replaces the served key set, as a provider does when it rotates.
func (j *JWKS) Publish(t *testing.T, keys ...*SigningKey) {
	t.Helper()

	body := jwksJSON(t, j.declareAlg, keys)

	j.mu.Lock()
	defer j.mu.Unlock()
	j.body = body
}

// Break makes the endpoint answer 500, standing in for a provider that is down.
func (j *JWKS) Break() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.broken = true
}

// Delay makes the endpoint take d to answer.
//
// A fixture on localhost answers in well under a millisecond, which hides every
// timeout that is too short: a fetch budget of 100 ms passes here and fails
// against a provider across the internet. Tests about *whether a fetch happens*
// need this; tests about what a key set contains do not.
func (j *JWKS) Delay(d time.Duration) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.delay = d
}

func (j *JWKS) serve(w http.ResponseWriter, _ *http.Request) {
	j.requests.Add(1)

	j.mu.Lock()
	broken, body, delay := j.broken, j.body, j.delay
	j.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	if broken {
		http.Error(w, "jwks unavailable", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func jwksJSON(t *testing.T, declareAlg bool, keys []*SigningKey) []byte {
	t.Helper()

	ctx := t.Context()
	storage := jwkset.NewMemoryStorage()

	for _, key := range keys {
		metadata := jwkset.JWKMetadataOptions{KID: key.KID, USE: jwkset.UseSig}
		if declareAlg {
			metadata.ALG = jwkset.ALG(key.Alg)
		}

		jwk, err := jwkset.NewJWKFromKey(key.private.Public(), jwkset.JWKOptions{Metadata: metadata})
		if err != nil {
			t.Fatalf("building JWK for %q: %v", key.KID, err)
		}

		if err := storage.KeyWrite(ctx, jwk); err != nil {
			t.Fatalf("writing JWK for %q: %v", key.KID, err)
		}
	}

	raw, err := storage.JSONPublic(ctx)
	if err != nil {
		t.Fatalf("marshalling the key set: %v", err)
	}

	body, err := json.Marshal(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("re-marshalling the key set: %v", err)
	}

	return body
}

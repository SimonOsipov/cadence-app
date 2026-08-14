package main

import (
	"crypto"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// adminTokenLifetime is well inside the sixty seconds the spec fixes as the
	// ceiling. The token is minted per call and travels one hop, so a lifetime
	// long enough to be worth capturing buys nothing.
	adminTokenLifetime = 30 * time.Second

	// adminTokenAudience and adminTokenIssuer are foreign to the API on
	// purpose, and they are free: GoTrue checks neither on its admin routes —
	// measured, and pinned by the contract tests beside the verifier. So each
	// is a reason the API's verifier refuses this token that costs nothing to
	// carry, and they stand behind the `kid` pinning rather than beside it. A
	// regression that lost the pinning still runs into two more refusals.
	//
	// The issuer names a reserved TLD (RFC 2606) so that nothing can ever
	// resolve it and mistake it for a key source.
	adminTokenAudience = "cadence-provisioner"
	adminTokenIssuer   = "https://provisioner.cadence.invalid"

	// adminTokenRole is the one claim GoTrue's admin routes do check.
	adminTokenRole = "service_role"
)

// tokenSigner mints the admin tokens this component presents to GoTrue. It
// holds the only copy of the private admin key in the system.
type tokenSigner struct {
	kid    string
	method jwt.SigningMethod
	key    crypto.Signer
}

// newTokenSigner reads the admin key from one JWK.
//
// One key in one variable, with its `kid` travelling in the same document as
// the material it names: the two cannot be rotated apart, which a key here and
// an id in a second variable would allow. Every way the value can be wrong is
// an error rather than a signer that produces tokens nothing accepts.
func newTokenSigner(rawJWK string) (*tokenSigner, error) {
	if strings.TrimSpace(rawJWK) == "" {
		return nil, errors.New("the admin key is required")
	}

	jwk, err := jwkset.NewJWKFromRawJSON(
		[]byte(rawJWK), jwkset.JWKMarshalOptions{Private: true}, jwkset.JWKValidateOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("reading the admin key: %w", err)
	}

	marshal := jwk.Marshal()

	if marshal.KID == "" {
		return nil, errors.New("the admin key carries no kid, so nothing can name the key that signed a token")
	}

	// A public key parses cleanly and cannot sign. Without this, the failure
	// arrives as a refusal from GoTrue on the first invitation instead.
	key, ok := jwk.Key().(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("the admin key %q carries no private half", marshal.KID)
	}

	method := jwt.GetSigningMethod(string(marshal.ALG))
	if method == nil {
		return nil, fmt.Errorf("the admin key %q declares no usable alg, got %q", marshal.KID, marshal.ALG)
	}

	return &tokenSigner{kid: marshal.KID, method: method, key: key}, nil
}

// mint issues one admin token, valid from now.
//
// It carries no `sub`. GoTrue does not require one on its admin routes —
// measured — and this token is minted by a process on its own behalf: a subject
// would be a person's identifier attached to a machine's authority. It is also
// a fourth reason the API's verifier refuses it, on the same terms as the
// audience and the issuer.
func (s *tokenSigner) mint(now time.Time) (string, error) {
	token := jwt.NewWithClaims(s.method, jwt.MapClaims{
		"role": adminTokenRole,
		"aud":  adminTokenAudience,
		"iss":  adminTokenIssuer,
		"iat":  now.Unix(),
		"exp":  now.Add(adminTokenLifetime).Unix(),
	})
	token.Header["kid"] = s.kid

	signed, err := token.SignedString(s.key)
	if err != nil {
		return "", fmt.Errorf("signing the admin token: %w", err)
	}

	return signed, nil
}

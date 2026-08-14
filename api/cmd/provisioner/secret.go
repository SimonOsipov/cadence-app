package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
)

// secretHeader is the one place the shared secret is read from — a header
// rather than a query parameter, with no fallback to one: a secret in a URL is
// a secret in every access log, proxy trace and error report on the way, and a
// fallback would put it there the first time a caller found the header
// inconvenient.
const secretHeader = "X-Cadence-Provisioner-Secret"

// secrets holds two current values from day one. That is a shape of
// configuration rather than a rotation choreography: with one, replacing it
// needs a moment in which the old value is refused and the new one is not yet
// deployed. What order they are replaced in, and how long both are held, is an
// open question in the spec and belongs to a runbook that does not exist yet.
//
// Against a compromised API the secret is useless by construction — the API is
// its legitimate holder. It bounds who else can reach this component, which is
// a different and smaller claim, and it is written down rather than papered
// over.
type secrets struct {
	current  [sha256.Size]byte
	previous [sha256.Size]byte
}

func newSecrets(current, previous string) secrets {
	pair := secrets{current: sha256.Sum256([]byte(current))}

	if previous != "" {
		pair.previous = sha256.Sum256([]byte(previous))

		return pair
	}

	// The digest of the first one's digest: nobody can offer a value that
	// hashes to it without already holding the current secret's digest, so
	// permits does the same two comparisons either way — whether a deployment
	// holds one secret or two is not observable from how long a refusal takes.
	pair.previous = sha256.Sum256(pair.current[:])

	return pair
}

// permits runs both comparisons and has one exit: an early return on the first
// match would make which of the two was offered observable, and `==` on the
// values themselves returns as soon as two bytes differ. The offered value is
// hashed first, so the comparison is over two fixed-length digests and its
// length discloses nothing either. TestTheSecretIsComparedInConstantTime holds
// this shape.
func (s secrets) permits(offered string) bool {
	digest := sha256.Sum256([]byte(offered))

	return subtle.ConstantTimeCompare(digest[:], s.current[:])|
		subtle.ConstantTimeCompare(digest[:], s.previous[:]) > 0
}

// guard runs before the router decides, so an unauthenticated caller probing
// for operations learns nothing about which ones exist.
func (s *server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.secrets.permits(r.Header.Get(secretHeader)) {
			s.refuse(w, r, http.StatusUnauthorized, "the request carried no current shared secret")

			return
		}

		next.ServeHTTP(w, r)
	})
}

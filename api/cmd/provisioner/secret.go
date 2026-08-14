package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
)

// secretHeader is the one place the shared secret is read from.
//
// A header rather than a query parameter, and no fallback to one: a secret in a
// URL is a secret in every access log, every proxy trace and every error report
// on the way, and a fallback would put it there the first time a caller found
// the header inconvenient.
const secretHeader = "X-Cadence-Provisioner-Secret"

// secrets is the pair of values a request may present.
//
// Two are current from day one. That is a shape of configuration rather than a
// rotation choreography: with one, replacing it needs a moment in which the old
// value is refused and the new one is not yet deployed. What order they are
// replaced in, and how long both are held, is an open question in the spec and
// belongs to a runbook that does not exist yet — so it is deliberately not
// decided here.
//
// Against a compromised API the secret is useless by construction: the API is
// the legitimate holder of it. It bounds who else can reach this component,
// which is a different and smaller claim, and it is written down rather than
// papered over.
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

	// With no second value configured, the slot holds the digest of the first
	// one's digest. Nobody can offer a value that hashes to it without already
	// holding the current secret's digest, so permits below does the same two
	// comparisons either way — whether this deployment holds one secret or two
	// is not observable from how long a refusal takes.
	pair.previous = sha256.Sum256(pair.current[:])

	return pair
}

// permits reports whether offered is one of the two current values.
//
// Both comparisons always run and there is one exit: an early return on the
// first match would make which of the two was offered observable, and `==` on
// the values themselves returns as soon as two bytes differ, which is the leak
// this whole function is shaped around. The offered value is hashed first, so
// the comparison is over two fixed-length digests and its length discloses
// nothing either. TestTheSecretIsComparedInConstantTime holds this shape.
func (s secrets) permits(offered string) bool {
	digest := sha256.Sum256([]byte(offered))

	return subtle.ConstantTimeCompare(digest[:], s.current[:])|
		subtle.ConstantTimeCompare(digest[:], s.previous[:]) > 0
}

// guard is the only middleware. It runs before the router decides, so an
// unauthenticated caller probing for operations learns nothing about which ones
// exist.
func (s *server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.secrets.permits(r.Header.Get(secretHeader)) {
			s.refuse(w, r, http.StatusUnauthorized, "the request carried no current shared secret")

			return
		}

		next.ServeHTTP(w, r)
	})
}

package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
)

// bearerScheme is the only credential this API accepts. RFC 7235 makes the
// scheme name case-insensitive, so it is matched that way.
const bearerScheme = "bearer"

// Middleware refuses any request that does not carry a verified bearer token.
//
// Deny by default: the guard is on every path, and exempt lists the exact ones
// that answer without a token — the liveness probe, the OpenAPI document and
// its viewer. A prefix list would be the wrong shape here. "/v1" as a rule
// leaves everything outside /v1 open, and every path added later is open until
// somebody remembers to guard it; this way a path nobody thought about is
// refused, which is the failure that can be noticed.
//
// The comparison is exact for the same reason: "/healthz" as a prefix would
// also open "/healthzzz".
func Middleware(verifier *Verifier, exempt []string) func(http.Handler) http.Handler {
	open := make(map[string]struct{}, len(exempt))
	for _, path := range exempt {
		open[path] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := open[r.URL.Path]; ok {
				next.ServeHTTP(w, r)

				return
			}

			token, err := bearerToken(r)
			if err != nil {
				refuse(w, r, err)

				return
			}

			principal, err := verifier.Verify(r.Context(), token)
			if err != nil {
				refuse(w, r, err)

				return
			}

			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), principal)))
		})
	}
}

// bearerToken reads the credential out of the Authorization header.
//
// The query string is not consulted, and that is a rule rather than an
// omission: a token in a URL is written to the access log of every proxy it
// passes, sent onward in Referer, and kept in browser history. It is the same
// rule the architecture review sets for the chat socket.
func bearerToken(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", errors.New("no Authorization header")
	}

	scheme, credential, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, bearerScheme) {
		return "", errors.New("the Authorization header is not a bearer credential")
	}

	credential = strings.TrimSpace(credential)
	if credential == "" {
		return "", errors.New("the bearer credential is empty")
	}

	return credential, nil
}

// refuse writes the one 401 this API has.
//
// The reason travels in Problem.Detail on purpose: httpserver logs it and then
// removes it on the way to the wire, so passing the real cause is how it
// reaches the log. Sanitising here would delete the only copy.
func refuse(w http.ResponseWriter, r *http.Request, cause error) {
	// RFC 7235 requires a challenge on a 401. Bare "Bearer", with no realm and
	// no error parameter: an error code here would tell the caller apart from
	// the body, which is the distinction this whole path exists to remove.
	w.Header().Set("WWW-Authenticate", "Bearer")

	httpserver.WriteProblem(w, r, httpserver.Problem{
		Status: http.StatusUnauthorized,
		Type:   httpserver.ProblemUnauthorized,
		Detail: cause.Error(),
	})
}

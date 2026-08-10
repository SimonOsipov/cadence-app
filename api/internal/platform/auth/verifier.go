package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

// The two reasons a token does not produce a principal. They are separated
// because they are different incidents: the first is a caller's problem and the
// second is ours, and an operator reading the log needs to know which one is
// happening. The caller is told neither — see httpserver.Problem.
var (
	// ErrTokenRejected means the token was read and found wanting.
	ErrTokenRejected = errors.New("the bearer token was rejected")

	// ErrKeysUnavailable means no signing key could be consulted, so the token
	// was neither accepted nor refuted. It is still a refusal: an unverifiable
	// token is not a verified one.
	ErrKeysUnavailable = errors.New("the signing keys could not be read")
)

// permittedAlgorithms is a closed list, on purpose.
//
// The identity provider is our own GoTrue, configured to sign ES256 through
// GOTRUE_JWT_KEYS. RS256 stays on the list because the provider can be
// reconfigured, and because a list of one is a list nobody notices is a list.
// What must never join them is HS256: a shared secret both verifies *and*
// mints tokens, which is a different threat model, and it is also how the
// algorithm-confusion attack lands when a public key is fed in as the secret.
var permittedAlgorithms = []string{jwt.SigningMethodRS256.Alg(), jwt.SigningMethodES256.Alg()}

const (
	// defaultLeeway absorbs clock skew between the provider and this process.
	// Large enough for a machine whose NTP is lagging, small enough that an
	// expired token is not usefully expired.
	defaultLeeway = 30 * time.Second

	// defaultRefreshInterval is how often the key set is re-fetched regardless
	// of what any token asks for. Provider rotation is picked up here; the
	// unknown-kid path below only shortens the window.
	defaultRefreshInterval = time.Hour

	// defaultUnknownKIDInterval bounds how often a token naming a key we do not
	// have may cause a fetch. Without it, a stream of random kids becomes a
	// request flood against the provider, whose own rate limiter then refuses
	// us and takes authentication down for everyone.
	defaultUnknownKIDInterval = 5 * time.Minute

	// defaultKeyWaitAllowance is the most a request will park waiting for the
	// refresh budget to come free.
	//
	// It is what makes the deadline below two guarantees instead of one. The
	// limiter sleeps whenever the wait fits inside the deadline, so without a
	// separate allowance a request arriving late in the interval would be
	// granted the refresh with almost none of the deadline left for it — and
	// would fail the fetch and burn the budget, which is the defect the
	// deadline exists to prevent, recurring at the end of every window.
	//
	// At most one request is ever parked: the limiter reserves the token before
	// sleeping, so anything arriving beside it sees a wait of minutes, larger
	// than the deadline, and is refused immediately.
	defaultKeyWaitAllowance = 250 * time.Millisecond

	// defaultKeyFetchBudget is what an on-demand fetch is guaranteed, after the
	// wait above has taken its worst case.
	//
	// It has to cover a real round trip to the provider, TLS handshake
	// included. At 100 ms — enough for a localhost fixture and for nothing else
	// — the on-demand refresh never completes against a provider across the
	// network, and a key rotation produces blanket 401s until the next
	// scheduled refresh an hour later.
	defaultKeyFetchBudget = 3 * time.Second

	// defaultKeyWaitMax is the single deadline jwkset accepts. It covers the
	// wait for the refresh budget *and*, once granted, the fetch — jwkset
	// derives one context from it and passes that context into the refresh, so
	// a value chosen only to bound the wait silently becomes the HTTP timeout.
	// Summing the two above is how each gets its own guarantee anyway.
	defaultKeyWaitMax = defaultKeyWaitAllowance + defaultKeyFetchBudget

	// defaultHTTPTimeout bounds a scheduled fetch of the key set — the one at
	// startup and the ones on the refresh interval. The on-demand path is
	// bounded by defaultKeyWaitMax instead.
	defaultHTTPTimeout = 5 * time.Second
)

// VerifierConfig is what a Verifier needs. The zero value of every duration
// takes the default above; Issuer, Audience and JWKSURL have no defaults.
type VerifierConfig struct {
	// Issuer is compared byte for byte against the token's `iss` claim.
	Issuer string

	// Audience is compared against `aud`.
	//
	// GoTrue sets it to "authenticated" for every user, so this check
	// isolates nothing between deployments — the issuer and the identity of
	// the keys do that. It is enforced because the contract declares it, and said
	// plainly here so it is not mistaken for tenant isolation.
	Audience string

	// JWKSURL is where the key set is fetched. config derives it from the
	// issuer; it is a field rather than a second environment variable so that
	// no deployment can point the keys at one provider and the issuer check at
	// another.
	JWKSURL string

	Leeway             time.Duration
	RefreshInterval    time.Duration
	UnknownKIDInterval time.Duration
	KeyWaitMax         time.Duration
	HTTPTimeout        time.Duration

	// Logger records key-set refresh failures. Defaults to slog.Default.
	Logger *slog.Logger
}

// Verifier turns a bearer token into a Principal, or into a reason it is not
// one. It is safe for concurrent use and holds the cached key set.
type Verifier struct {
	keys   keyfunc.Keyfunc
	parser *jwt.Parser
}

// claims is the subset of a Supabase token this package reads. Everything not
// named here is discarded at the parse boundary rather than carried around and
// filtered later.
type claims struct {
	jwt.RegisteredClaims

	Role string `json:"role"`
}

// NewVerifier starts a verifier and the background refresh of its key set. The
// refresh goroutine ends with ctx, so the caller's shutdown ends it.
//
// A key set that cannot be fetched at startup is not an error here. The process
// must come up during a provider outage — every request will be refused while
// it lasts, which is the correct behaviour, and refusing to start would instead
// turn a provider blip into a deployment that needs a human.
func NewVerifier(ctx context.Context, cfg VerifierConfig) (*Verifier, error) {
	if cfg.Issuer == "" || cfg.Audience == "" || cfg.JWKSURL == "" {
		return nil, errors.New("auth: issuer, audience and JWKS URL are all required")
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	keys, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{cfg.JWKSURL}, keyfunc.Override{
		HTTPTimeout:      orDefault(cfg.HTTPTimeout, defaultHTTPTimeout),
		RefreshInterval:  orDefault(cfg.RefreshInterval, defaultRefreshInterval),
		RateLimitWaitMax: orDefault(cfg.KeyWaitMax, defaultKeyWaitMax),
		RefreshUnknownKID: rate.NewLimiter(
			rate.Every(orDefault(cfg.UnknownKIDInterval, defaultUnknownKIDInterval)), 1,
		),
		RefreshErrorHandlerFunc: func(url string) func(context.Context, error) {
			return func(ctx context.Context, err error) {
				logger.ErrorContext(ctx, "could not refresh the JWK Set", "url", url, "error", err)
			}
		},
	})
	if err != nil {
		return nil, fmt.Errorf("building the JWK Set client for %q: %w", cfg.JWKSURL, err)
	}

	return &Verifier{
		keys: keys,
		parser: jwt.NewParser(
			jwt.WithValidMethods(permittedAlgorithms),
			jwt.WithIssuer(cfg.Issuer),
			jwt.WithAudience(cfg.Audience),
			// jwt/v5 treats a missing exp as "never expires". A token that
			// cannot expire is a credential, and this API does not issue those.
			jwt.WithExpirationRequired(),
			jwt.WithLeeway(orDefault(cfg.Leeway, defaultLeeway)),
		),
	}, nil
}

// Verify checks a token and returns the caller it names.
//
// Every failure wraps ErrTokenRejected or ErrKeysUnavailable, and carries the
// specific reason in its message — for the log. The caller of the API is told
// none of it.
func (v *Verifier) Verify(ctx context.Context, token string) (Principal, error) {
	var parsed claims

	if _, err := v.parser.ParseWithClaims(token, &parsed, v.keyfunc(ctx)); err != nil {
		if errors.Is(err, ErrKeysUnavailable) {
			return Principal{}, fmt.Errorf("verifying the token: %w", err)
		}

		return Principal{}, fmt.Errorf("%w: %w", ErrTokenRejected, err)
	}

	if parsed.Subject == "" {
		return Principal{}, fmt.Errorf("%w: the token names no subject", ErrTokenRejected)
	}

	// Guaranteed non-nil: WithExpirationRequired has already refused a token
	// without one.
	return Principal{
		Subject:   parsed.Subject,
		Role:      parsed.Role,
		ExpiresAt: parsed.ExpiresAt.Time,
	}, nil
}

// keyfunc resolves the signing key for one token.
//
// It insists on a kid before it looks anything up. Without one, the key set has
// to be searched and the token is accepted if *any* published key verifies it —
// which is the door through which a key that was rotated out walks back in.
func (v *Verifier) keyfunc(ctx context.Context) jwt.Keyfunc {
	resolve := v.keys.KeyfuncCtx(ctx)

	return func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, fmt.Errorf("%w: the token header names no key", ErrTokenRejected)
		}

		key, err := resolve(token)
		if err != nil {
			return nil, v.classifyKeyFailure(ctx, kid, err)
		}

		return key, nil
	}
}

// classifyKeyFailure decides whether a key lookup failed because of the token
// or because of us.
//
// The state of the cache decides, not the shape of the error, and the order
// matters. The library reports several different situations through the same
// lookup: a kid genuinely absent from a healthy set, a fetch that failed and
// left the cache empty, and a refresh the rate limiter declined. Only the
// middle one is our outage. Reading the error kind first gets the third case
// wrong — a spray of junk kids exhausts the budget within seconds, and every
// token after that would be logged as a signing-key failure, burying a real
// outage under attacker traffic.
//
// So: if any key is cached, this process could consult a key and the token did
// not match one. With none cached, it could not consult anything — it is not
// refuting the token, only declining to accept it, and the log should say so.
func (v *Verifier) classifyKeyFailure(ctx context.Context, kid string, err error) error {
	cached, readErr := v.keys.Storage().KeyReadAll(ctx)
	if readErr != nil {
		return fmt.Errorf("%w: reading the cached key set: %w", ErrKeysUnavailable, readErr)
	}

	if len(cached) > 0 {
		return fmt.Errorf("%w: no usable key for kid %q: %w", ErrTokenRejected, kid, err)
	}

	return fmt.Errorf("%w: no key is cached and none could be fetched for kid %q: %w",
		ErrKeysUnavailable, kid, err)
}

func orDefault(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}

	return fallback
}

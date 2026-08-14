// Package provisioning is how the API asks the provisioner to do something
// about an account at the identity provider.
//
// It is the implementation of identity.Provisioner and it lives out here rather
// than inside that context on purpose: the context declares what it needs and
// compiles against nothing that speaks HTTP, so replacing the identity provider
// stays one implementation rather than a sweeping edit. The direction of the
// import is the whole arrangement — this package knows identity, identity does
// not know this package, and identity's own boundary test is what keeps it that
// way.
//
// What this package must never hold is the admin key. It holds a shared secret,
// which bounds who else can reach the component; the key that GoTrue's admin
// routes accept lives in the provisioner and nowhere else, and a gate rule in
// scripts/gate/go.sh fails the build when its variable is named anywhere under
// api/cmd/api or api/internal.
package provisioning

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
)

// Compiled here rather than asserted in a test: the interface belongs to the
// consumer, and a method that stops matching it is a build failure at the place
// that got it wrong.
var _ identity.Provisioner = (*Client)(nil)

// callTimeout bounds one call end to end — connection, TLS handshake and
// response. It is http.Client.Timeout rather than a dialer's, which would cover
// the first of those three and leave a component that accepts the connection and
// then says nothing bounded by whatever gives up next.
//
// Ordered against the database's idle-in-transaction limit by
// TestAnOutwardCallCannotOutlastTheTransactionItIsMadeIn — a call made between
// two statements of a service transaction is idle time for the session, and a
// bound above that limit means Postgres terminates the session while the call is
// in flight, after the invitation has already gone out.
const callTimeout = 10 * time.Second

// secretHeader is where the shared secret travels, and the only place.
//
// It is the component's own constant, which lives in package main under
// cmd/provisioner and cannot be imported. TestTheClientSpeaksTheSurfaceTheComponentServes
// reads that file and fails when the two stop agreeing.
const secretHeader = "X-Cadence-Provisioner-Secret"

// The five paths of the component's surface, four of which this client uses.
const (
	invitePath      = "/invite"
	lookupPath      = "/users/lookup"
	lookupBatchPath = "/users/lookup-batch"
	deletePath      = "/users/delete"
)

// The keys the component answers under. The struct tags below say them a second
// time because a tag cannot be a constant; TestTheClientSpeaksTheSurfaceTheComponentServes
// ties all three spellings — these, the tags, and the component's own — together.
const (
	accountKey  = "account"
	accountsKey = "accounts"
)

// maxAnswer bounds what is read back. The answers are small documents; a
// component answering with something enormous is a failure, not a large roster.
const maxAnswer = 1 << 20

// maxRefusalInAnError is how much of a refusal's body travels in the error text.
// The whole of it would put a megabyte of somebody else's document on one log
// line, and what a person needs from it is the first sentence.
const maxRefusalInAnError = 512

// ErrRefused is what a call the provisioner did not accept comes back as. The
// status and what it said travel inside it, so a handler can tell a refusal from
// a provisioner that is not answering at all.
var ErrRefused = errors.New("the provisioner refused the call")

// Config is what this client needs to reach the component.
type Config struct {
	// BaseURL is where the provisioner listens. An internal address: no route
	// resolves to this component from outside, which is a property of the
	// deployment and is probed rather than assumed.
	BaseURL string

	// Secret is one of the component's two current shared secrets. It is not the
	// admin key and it is not a defence against a compromised API — this process
	// is its legitimate holder. What it bounds is who else can reach the port.
	Secret string
}

// Client is the provisioner as this process sees it.
type Client struct {
	base    string
	secret  string
	timeout time.Duration
	http    *http.Client
}

// New builds the client, and refuses a configuration that could not work.
//
// Refused here rather than at the first call: the first call is a patient being
// invited, and "the address is unreachable" is not something to discover with a
// half-written record in front of it.
func New(cfg Config) (*Client, error) {
	base := strings.TrimSuffix(strings.TrimSpace(cfg.BaseURL), "/")

	parsed, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("parsing the provisioner address: %w", err)
	}
	if parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, fmt.Errorf("the provisioner address must be absolute http(s), got %q", cfg.BaseURL)
	}

	if cfg.Secret == "" {
		return nil, errors.New("the provisioner shared secret is required")
	}

	return &Client{
		base:    base,
		secret:  cfg.Secret,
		timeout: callTimeout,
		http:    &http.Client{Timeout: callTimeout},
	}, nil
}

// Invite creates an account for email and has the invitation sent.
func (c *Client) Invite(ctx context.Context, email string) (identity.Account, error) {
	var answer accountAnswer

	if err := c.call(ctx, invitePath, map[string]string{"email": email}, &answer); err != nil {
		return identity.Account{}, fmt.Errorf("inviting: %w", err)
	}

	// An invitation that answers with no account is not a success with a zero
	// value: the address has been taken at the provider and this process would go
	// on to write a profile against an empty identifier.
	// The address is deliberately not in the message. It is in a request body
	// rather than a URL for exactly this reason, and an error is the one thing on
	// this path that this process writes to its own log.
	if answer.Account == nil {
		return identity.Account{}, fmt.Errorf("inviting: %w: the answer names no account", ErrRefused)
	}

	return answer.Account.asIdentity(), nil
}

// Lookup answers with the one account at email, or with nothing.
//
// Nothing is an answer rather than a failure: it is what makes an address free
// to invite.
func (c *Client) Lookup(ctx context.Context, email string) (*identity.Account, error) {
	var answer accountAnswer

	if err := c.call(ctx, lookupPath, map[string]string{"email": email}, &answer); err != nil {
		return nil, fmt.Errorf("looking up an address: %w", err)
	}

	if answer.Account == nil {
		return nil, nil
	}

	found := answer.Account.asIdentity()

	return &found, nil
}

// LookupBatch answers with the accounts among ids that exist. An identifier
// nobody has is absent from the answer rather than an error for the others: a
// roster holding one stale row still has to render.
func (c *Client) LookupBatch(ctx context.Context, ids []string) ([]identity.Account, error) {
	// A roster of nobody is answered here rather than at the component, which
	// refuses an empty list — correctly, since a call that asks about nothing is a
	// caller's mistake. It is not one here: a clinic with no patients yet is an
	// ordinary state, and the screen still has to render.
	if len(ids) == 0 {
		return nil, nil
	}

	var answer accountsAnswer

	if err := c.call(ctx, lookupBatchPath, map[string][]string{"ids": ids}, &answer); err != nil {
		return nil, fmt.Errorf("looking up a roster: %w", err)
	}

	accounts := make([]identity.Account, 0, len(answer.Accounts))
	for _, found := range answer.Accounts {
		accounts = append(accounts, found.asIdentity())
	}

	return accounts, nil
}

// Delete removes an account, stating the proof that removing it is safe.
//
// Both halves travel explicitly, including the false one: the component refuses
// a request that leaves either of them out, and a field omitted by a marshaller
// would be a proof nobody made.
func (c *Client) Delete(ctx context.Context, deletion identity.Deletion) error {
	body := map[string]any{
		"id":             deletion.ID,
		"invite_exists":  deletion.InviteExists,
		"profile_exists": deletion.ProfileExists,
	}

	if err := c.call(ctx, deletePath, body, nil); err != nil {
		return fmt.Errorf("deleting an account: %w", err)
	}

	return nil
}

// The envelopes the component answers in. Named types rather than a struct
// declared inside each method, so that the keys they decode by can be compared
// against the component's own — see the drift test.
type (
	accountAnswer struct {
		Account *account `json:"account"`
	}

	accountsAnswer struct {
		Accounts []account `json:"accounts"`
	}
)

// account is the wire shape of what the component answers with. It is here
// rather than on identity.Account so that the context's type owes nothing to
// somebody else's field names.
type account struct {
	ID           string     `json:"id"`
	ConfirmedAt  *time.Time `json:"confirmed_at"`
	LastSignInAt *time.Time `json:"last_sign_in_at"`
}

func (a account) asIdentity() identity.Account {
	return identity.Account{ID: a.ID, ConfirmedAt: a.ConfirmedAt, LastSignInAt: a.LastSignInAt}
}

// call makes one call to the component.
//
// Every operation is a POST carrying a JSON body, including the two that read
// and the one that deletes: an address or an account identifier in a path or a
// query string is an address in every access log on the way.
func (c *Client) call(ctx context.Context, path string, body, into any) error {
	// A deadline of this client's own, so that a caller who brings a context
	// without one still makes a bounded call.
	//
	// Not for the sake of the error, and not because the transport would leave the
	// context unbounded: measured on Go 1.26.4, net/http derives a deadline from
	// Client.Timeout itself and its failure already unwraps to
	// context.DeadlineExceeded. What this adds is that the bound starts here —
	// covering the encoding below and whatever a later edit puts in front of the
	// request — rather than at Do.
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encoding the request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("building the request: %w", err)
	}

	// The header, and nowhere else. Never a query parameter and never logged: the
	// secret is what stands between this port and whoever else can reach it.
	req.Header.Set(secretHeader, c.secret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling the provisioner: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// One byte past the limit, so that a document too large to read says so.
	// Truncating at the limit and decoding what came back would report
	// "unexpected end of JSON input", which sends whoever reads it looking for a
	// malformed answer rather than an enormous one.
	said, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswer+1))
	if err != nil {
		return fmt.Errorf("reading the answer: %w", err)
	}
	if len(said) > maxAnswer {
		return fmt.Errorf("%w: POST %s answered with more than %d bytes", ErrRefused, path, maxAnswer)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%w: POST %s answered %d: %s", ErrRefused, path, resp.StatusCode,
			said[:min(len(said), maxRefusalInAnError)])
	}

	if into == nil {
		return nil
	}

	if err := json.Unmarshal(said, into); err != nil {
		return fmt.Errorf("decoding the answer: %w", err)
	}

	return nil
}

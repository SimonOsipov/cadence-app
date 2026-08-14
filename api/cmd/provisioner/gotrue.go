package main

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
)

// gotrueCallTimeout bounds one admin call end to end — connection, TLS and
// response. Nothing here retries: every operation is a write somebody else acts
// on, and a repeated invitation is a second email to a patient.
const gotrueCallTimeout = 10 * time.Second

// lookupPageSize: the component answers with one account or none, so a larger
// page would only make a substring nobody may send more expensive to answer.
const lookupPageSize = 50

// The provider's own message travels inside it for the log and never reaches
// the caller: it is GoTrue's vocabulary, and it describes accounts the caller
// may have no business knowing about.
var errProviderRefused = errors.New("the identity provider refused the call")

// refusal keeps the status, so that the one caller for which a 404 is an answer
// rather than a failure can tell them apart.
type refusal struct {
	status int
	method string
	path   string
	said   string
}

func (r *refusal) Error() string {
	return fmt.Sprintf("%s: %s %s answered %d: %s", errProviderRefused, r.method, r.path, r.status, r.said)
}

func (r *refusal) Unwrap() error { return errProviderRefused }

// noSuchAccount may only be asked by the operations that put an identifier in
// the URL. For the listing and for /invite a 404 is the base address being
// wrong, and reading it as "no such account" would turn a mistyped
// PROVISIONER_GOTRUE_URL into a roster that answers "this clinic has nobody" —
// a misconfiguration that looks exactly like an empty database.
func noSuchAccount(err error) bool {
	var refused *refusal

	return errors.As(err, &refused) && refused.status == http.StatusNotFound
}

// account is the whole answer this component gives about a person, and the
// narrowing is the type rather than a filtering step: GoTrue's user document
// also carries `confirmation_token` — the value in the invitation link, so the
// credential itself — the encrypted password, and whatever a clinic wrote into
// the user metadata. None of it can reach a caller by being forgotten, because
// nothing decodes it. `confirmed_at` and `last_sign_in_at` are here because the
// claim rule in the onboarding block rests on both.
type account struct {
	ID           string     `json:"id"`
	ConfirmedAt  *time.Time `json:"confirmed_at"`
	LastSignInAt *time.Time `json:"last_sign_in_at"`
}

// gotrueUser is an account plus the address, which the exact match needs and
// the answer does not carry.
type gotrueUser struct {
	account

	Email string `json:"email"`
}

type gotrueClient struct {
	base   string
	signer *tokenSigner
	http   *http.Client
}

func newGoTrueClient(base string, signer *tokenSigner) *gotrueClient {
	return &gotrueClient{
		base:   strings.TrimSuffix(base, "/"),
		signer: signer,
		http:   &http.Client{Timeout: gotrueCallTimeout},
	}
}

// invite expects address lowercased: GoTrue stores it that way, and an account
// created under one spelling could never be found under another.
func (c *gotrueClient) invite(ctx context.Context, address string) (account, error) {
	var user gotrueUser

	err := c.call(ctx, http.MethodPost, "/invite", nil, map[string]string{"email": address}, &user)
	if err != nil {
		return account{}, fmt.Errorf("inviting: %w", err)
	}

	return user.account, nil
}

// findByAddress does the exact match here, inside the component, because
// GoTrue's `?filter=` is a substring match and an empty one returns everybody.
// Exposing the filter outward would put the clinic's directory behind the one
// component whose purpose is that it holds it.
func (c *gotrueClient) findByAddress(ctx context.Context, address string) (*account, error) {
	var page struct {
		Users []gotrueUser `json:"users"`
	}

	query := url.Values{
		"filter":   {address},
		"page":     {"1"},
		"per_page": {fmt.Sprint(lookupPageSize)},
	}

	if err := c.call(ctx, http.MethodGet, "/admin/users", query, nil, &page); err != nil {
		return nil, fmt.Errorf("looking up an address: %w", err)
	}

	for _, user := range page.Users {
		if strings.EqualFold(user.Email, address) {
			found := user.account

			return &found, nil
		}
	}

	// A full page means the exact account may be on a page nobody read.
	// Answering "no account at that address" then sends the caller to /invite,
	// which refuses an address GoTrue still holds — the dead end deletion
	// exists to clean up. A refusal is recoverable; a false negative is not.
	if len(page.Users) >= lookupPageSize {
		return nil, fmt.Errorf("%w: %d accounts match %q, so the exact one may be on a page this lookup did not read",
			errProviderRefused, len(page.Users), address)
	}

	// An answer, not a failure: it is what makes an address free to invite.
	return nil, nil
}

func (c *gotrueClient) findByID(ctx context.Context, id string) (*account, error) {
	var user gotrueUser

	err := c.call(ctx, http.MethodGet, "/admin/users/"+id, nil, nil, &user)

	switch {
	case noSuchAccount(err):
		// An answer, not a failure: a roster holding one stale row still has to
		// render.
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("looking up an identifier: %w", err)
	}

	return &user.account, nil
}

func (c *gotrueClient) deleteAccount(ctx context.Context, id string) error {
	if err := c.call(ctx, http.MethodDelete, "/admin/users/"+id, nil, nil, nil); err != nil {
		return fmt.Errorf("deleting an account: %w", err)
	}

	return nil
}

func (c *gotrueClient) setPassword(ctx context.Context, id, password string) error {
	err := c.call(ctx, http.MethodPut, "/admin/users/"+id, nil, map[string]string{"password": password}, nil)
	if err != nil {
		return fmt.Errorf("setting a password: %w", err)
	}

	return nil
}

// call mints a token per call rather than caching one: it lives thirty seconds,
// so a cache would be a copy of the most dangerous credential in the system
// kept around for the sake of an allocation.
func (c *gotrueClient) call(
	ctx context.Context, method, path string, query url.Values, body, into any,
) error {
	var payload io.Reader

	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding the request: %w", err)
		}

		payload = bytes.NewReader(encoded)
	}

	address := c.base + path
	if len(query) > 0 {
		address += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, address, payload)
	if err != nil {
		return fmt.Errorf("building the request: %w", err)
	}

	token, err := c.signer.mint(time.Now())
	if err != nil {
		return err
	}

	// The Authorization header and nowhere else — never a query parameter,
	// never logged: this token is admitted by every key GoTrue is configured
	// with, which is what took it out of the API process.
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling the identity provider: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	said, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return fmt.Errorf("reading the answer: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &refusal{status: resp.StatusCode, method: method, path: path, said: string(said)}
	}

	if into == nil {
		return nil
	}

	if err := json.Unmarshal(said, into); err != nil {
		return fmt.Errorf("decoding the answer: %w", err)
	}

	return nil
}

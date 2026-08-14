//go:build integration

// The component against the real identity provider, on the digest-pinned image.
//
// Everything else in this package runs against fakeGoTrue, which encodes two
// measured behaviours — `?filter=` is a substring match and it is
// case-sensitive, `/invite` stores the address folded — and a fake is only
// worth what the claim that it matches is worth. This file is that claim: the
// same round trip, against the process the deployment runs.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

var cluster *testsupport.Cluster

func TestMain(m *testing.M) {
	ctx := context.Background()

	started, err := testsupport.StartCluster(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting the test cluster: %v\n", err)
		os.Exit(1)
	}
	cluster = started

	code := m.Run()

	if err := cluster.Terminate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "terminating the test cluster: %v\n", err)
	}

	os.Exit(code)
}

// liveProvisioner is the component wired to a real GoTrue, configured the way
// the deployment is: two keys with distinct ids, exactly one of which signs
// sessions, and the other held only here.
func liveProvisioner(t *testing.T) (*server, *testsupport.GoTrue, *tokenSigner) {
	t.Helper()

	session := testsupport.NewES256Key(t, "session-key")
	admin := testsupport.NewES256Key(t, "admin-key")

	gotrue := testsupport.StartGoTrue(t, cluster, testsupport.GoTrueJWKS(
		t,
		testsupport.JWKEntry{Key: session, Signing: true},
		testsupport.JWKEntry{Key: admin},
	))

	signer, err := newTokenSigner(admin.PrivateJWK(t))
	if err != nil {
		t.Fatalf("building the token signer: %v", err)
	}

	return &server{
		environment: development,
		secrets:     newSecrets(testSecret, testPreviousSecret),
		gotrue:      newGoTrueClient(gotrue.URL, signer),
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, gotrue, signer
}

// TestTheWholeRoundTripAgainstTheRealIdentityProvider walks the operations in
// the order the onboarding block will use them, against one account.
//
// One test rather than five: each step is the previous one's precondition —
// there is nothing to look up before an invitation and nothing to delete before
// a lookup — and five tests would be five GoTrue containers arranging the same
// state four times over.
func TestTheWholeRoundTripAgainstTheRealIdentityProvider(t *testing.T) {
	srv, _, _ := liveProvisioner(t)

	const typed = "Anna.Petrova@Clinic.Example"

	var invited account

	t.Run("the minted admin token is admitted, and an invitation creates an account", func(t *testing.T) {
		rec := call(t, srv, http.MethodPost, "/invite", `{"email":"`+typed+`"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("answered %d, want 200; body %s", rec.Code, rec.Body)
		}

		invited = decodeAccount(t, rec.Body.Bytes())
		if invited.ID == "" {
			t.Fatalf("the invitation answered with no identifier: %s", rec.Body)
		}
	})

	t.Run("the address a human typed finds the account it created", func(t *testing.T) {
		rec := call(t, srv, http.MethodPost, "/users/lookup", `{"email":"`+typed+`"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("answered %d, want 200; body %s", rec.Code, rec.Body)
		}

		if found := decodeAccount(t, rec.Body.Bytes()); found.ID != invited.ID {
			t.Fatalf("looked up %q, found %q", invited.ID, found.ID)
		}
	})

	t.Run("a lookup by identifier answers the same account", func(t *testing.T) {
		rec := call(t, srv, http.MethodPost, "/users/lookup-batch",
			`{"ids":["`+invited.ID+`","00000000-0000-4000-8000-000000000000"]}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("answered %d, want 200; body %s", rec.Code, rec.Body)
		}

		var answer struct {
			Accounts []account `json:"accounts"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
			t.Fatalf("decoding the answer: %v", err)
		}

		if len(answer.Accounts) != 1 || answer.Accounts[0].ID != invited.ID {
			t.Fatalf("answered %v, want the one account that exists", answer.Accounts)
		}
	})

	t.Run("setting a password outside production is accepted by the provider", func(t *testing.T) {
		rec := call(t, srv, http.MethodPost, "/users/password",
			`{"id":"`+invited.ID+`","password":"a-seeded-password-nobody-uses"}`)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("answered %d, want 204; body %s", rec.Code, rec.Body)
		}
	})

	t.Run("deletion without the proof leaves the account alone", func(t *testing.T) {
		rec := call(t, srv, http.MethodPost, "/users/delete",
			`{"id":"`+invited.ID+`","invite_exists":true,"profile_exists":true}`)
		if rec.Code != http.StatusConflict {
			t.Fatalf("answered %d, want 409; body %s", rec.Code, rec.Body)
		}

		// The account, not the status: a refusal that answered 409 and deleted
		// anyway would pass an assertion on the answer alone.
		lookup := call(t, srv, http.MethodPost, "/users/lookup", `{"email":"`+typed+`"}`)
		if found := decodeAccount(t, lookup.Body.Bytes()); found.ID != invited.ID {
			t.Fatalf("the account is gone after a refused deletion")
		}
	})

	t.Run("deletion with the proof frees the address", func(t *testing.T) {
		rec := call(t, srv, http.MethodPost, "/users/delete",
			`{"id":"`+invited.ID+`","invite_exists":true,"profile_exists":false}`)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("answered %d, want 204; body %s", rec.Code, rec.Body)
		}

		lookup := call(t, srv, http.MethodPost, "/users/lookup", `{"email":"`+typed+`"}`)
		if lookup.Code != http.StatusOK {
			t.Fatalf("the lookup after a deletion answered %d", lookup.Code)
		}

		var answer struct {
			Account *account `json:"account"`
		}
		if err := json.Unmarshal(lookup.Body.Bytes(), &answer); err != nil {
			t.Fatalf("decoding the answer: %v", err)
		}

		if answer.Account != nil {
			t.Fatalf("the address still resolves to %s after the account was deleted", answer.Account.ID)
		}

		// And the address is genuinely reusable, which is the entire reason
		// this operation exists: /invite answers 422 for an address it still
		// holds.
		again := call(t, srv, http.MethodPost, "/invite", `{"email":"`+typed+`"}`)
		if again.Code != http.StatusOK {
			t.Fatalf("re-inviting the freed address answered %d; body %s", again.Code, again.Body)
		}
	})
}

// TestTheProviderFilterIsCaseSensitive is the measurement the lowercasing
// exists for, taken against the image rather than assumed from the fake.
//
// Without it, "mixed case finds an existing account" is a test that would stay
// green with the lowercasing removed the day GoTrue started folding the filter
// itself — and the component would keep a step nobody could justify, or lose a
// step nobody could see was load-bearing.
func TestTheProviderFilterIsCaseSensitive(t *testing.T) {
	srv, gotrue, signer := liveProvisioner(t)

	const typed = "Boris.Ivanov@Clinic.Example"

	if rec := call(t, srv, http.MethodPost, "/invite", `{"email":"`+typed+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("the invitation answered %d; body %s", rec.Code, rec.Body)
	}

	tests := []struct {
		name   string
		filter string
		want   int
	}{
		{name: "the address as it was typed", filter: typed, want: 0},
		{name: "the address folded", filter: strings.ToLower(typed), want: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(listUsers(t, gotrue, signer, tc.filter)); got != tc.want {
				t.Fatalf("filtering on %q found %d accounts, want %d", tc.filter, got, tc.want)
			}
		})
	}
}

// listUsers calls the admin listing directly, with a token this component
// minted. It is also what proves the minted token is admitted: everything above
// reaches GoTrue through the same signer, so a token GoTrue refused would fail
// every one of them for one reason.
func listUsers(t *testing.T, gotrue *testsupport.GoTrue, signer *tokenSigner, filter string) []any {
	t.Helper()

	minted, err := signer.mint(time.Now())
	if err != nil {
		t.Fatalf("minting the admin token: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		gotrue.URL+"/admin/users?filter="+strings.ReplaceAll(filter, "@", "%40"), nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+minted)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("calling the admin listing: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	said, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the answer: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the admin listing answered %d with a token this component minted: %s", resp.StatusCode, said)
	}

	var page struct {
		Users []any `json:"users"`
	}
	if err := json.Unmarshal(said, &page); err != nil {
		t.Fatalf("decoding the listing: %v", err)
	}

	return page.Users
}

func decodeAccount(t *testing.T, body []byte) account {
	t.Helper()

	var answer struct {
		Account *account `json:"account"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		t.Fatalf("decoding the answer: %v", err)
	}

	if answer.Account == nil {
		t.Fatalf("the answer carries no account: %s", body)
	}

	return *answer.Account
}

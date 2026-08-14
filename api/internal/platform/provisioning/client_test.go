package provisioning

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
)

// The secret this client is built with in every test here. Long enough to be
// the real shape and distinctive enough that a substring search for it means
// something.
const testSecret = "a-shared-secret-nobody-else-holds-0123456789"

// seen is what the fake provisioner saw, so a test can ask where a value
// travelled rather than whether the call worked.
type seen struct {
	method string
	url    *url.URL
	header http.Header
	body   string
}

// fakeProvisioner answers with reply and records the request.
func fakeProvisioner(t *testing.T, status int, reply string) (*Client, *seen) {
	t.Helper()

	record := &seen{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading the request body: %v", err)
		}

		record.method, record.url, record.header, record.body = r.Method, r.URL, r.Header.Clone(), string(body)

		w.WriteHeader(status)
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(server.Close)

	client, err := New(Config{BaseURL: server.URL, Secret: testSecret})
	if err != nil {
		t.Fatalf("building the client: %v", err)
	}

	return client, record
}

// Where the secret is allowed to be, and the four places it is not.
//
// A secret in a URL is a secret in every access log, every proxy trace and every
// error report between here and the component — and an error is the one of those
// this process writes itself, which is why the refusal below is checked too.
func TestTheSharedSecretTravelsOnlyInTheHeader(t *testing.T) {
	// Every operation, because the header is set per call: one of them written
	// without it would be one operation reachable by whoever reaches the port,
	// and a test that walked only /invite would not say so.
	for name, operation := range map[string]struct {
		reply string
		call  func(*Client) error
	}{
		"invite": {`{"account":{"id":"3f2a"}}`, func(c *Client) error {
			_, err := c.Invite(context.Background(), "anna@clinic.example")

			return err
		}},
		"lookup": {`{"account":null}`, func(c *Client) error {
			_, err := c.Lookup(context.Background(), "anna@clinic.example")

			return err
		}},
		"lookup-batch": {`{"accounts":[]}`, func(c *Client) error {
			_, err := c.LookupBatch(context.Background(), []string{"3f2a"})

			return err
		}},
		"delete": {``, func(c *Client) error {
			return c.Delete(context.Background(), identity.Deletion{ID: "3f2a", InviteExists: true})
		}},
	} {
		t.Run(name, func(t *testing.T) {
			client, record := fakeProvisioner(t, http.StatusOK, operation.reply)

			if err := operation.call(client); err != nil {
				t.Fatalf("calling the provisioner: %v", err)
			}

			if got := record.header.Get(secretHeader); got != testSecret {
				t.Errorf("the %s header carried %q, want the shared secret", secretHeader, got)
			}
			if strings.Contains(record.url.String(), testSecret) {
				t.Errorf("the secret is in the request target %q", record.url)
			}
			if strings.Contains(record.url.RawQuery, testSecret) {
				t.Errorf("the secret is in the query string %q", record.url.RawQuery)
			}
			if strings.Contains(record.body, testSecret) {
				t.Errorf("the secret is in the request body %q", record.body)
			}
		})
	}

	t.Run("and not into what a refusal says", func(t *testing.T) {
		client, _ := fakeProvisioner(t, http.StatusBadGateway, `{"error":"Bad Gateway"}`)

		err := client.Delete(context.Background(), identity.Deletion{ID: "3f2a", InviteExists: true})
		if err == nil {
			t.Fatal("a 502 was reported as success")
		}
		if !errors.Is(err, ErrRefused) {
			t.Errorf("a 502 came back as %v, want a refusal", err)
		}
		if strings.Contains(err.Error(), testSecret) {
			t.Errorf("the secret is in the error this process logs: %v", err)
		}
	})
}

// The bound covers the connection and the TLS handshake, not only the wait for
// an answer.
//
// Measured against a listener that accepts the connection and then does nothing
// at all, so the call never gets past the handshake: a client bounded by a
// dialer's timeout — the shape it is easy to reach for — would sit there until
// something else gave up.
func TestOneCallIsBoundedWhereTheHandshakeNeverFinishes(t *testing.T) {
	var listen net.ListenConfig

	listener, err := listen.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	accepted := make(chan net.Conn, 1)

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		// Held, not closed: a closed connection would end the call by itself and
		// the timeout would never be what returned it.
		accepted <- conn
	}()

	t.Cleanup(func() {
		select {
		case conn := <-accepted:
			_ = conn.Close()
		default:
		}
	})

	client, err := New(Config{BaseURL: "https://" + listener.Addr().String(), Secret: testSecret})
	if err != nil {
		t.Fatalf("building the client: %v", err)
	}
	// The transport's bound alone, with this client's own left where New put it:
	// what returns the call is http.Client.Timeout and nothing else, which is the
	// mirror of the test below and what gives that field a mutation of its own.
	client.http.Timeout = 100 * time.Millisecond

	started := time.Now()

	_, err = client.Invite(context.Background(), "anna@clinic.example")
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("a call into a handshake that never finished came back as success")
	}
	// Of the same order as the bound under test. Five seconds would also be green
	// on a client bounded at three, which is already a call that outlives the
	// transaction it may be made inside.
	if elapsed > time.Second {
		t.Errorf("the call took %s to give up on a %s bound; what ended it was not that bound",
			elapsed, client.http.Timeout)
	}
}

// A caller that brings no deadline still makes a bounded call, and what bounds
// it is this client's own number.
//
// The elapsed time is the assertion, and it is there because the first version
// of this test passed without a context deadline at all: measured on Go 1.26.4,
// 2026-08-14, an http.Client.Timeout failure already unwraps to
// context.DeadlineExceeded, so errors.Is alone was green ten seconds later on
// the transport's bound. The two spellings of the bound are told apart here by
// leaving the transport's where New put it and moving only the client's.
func TestACallWithNoDeadlineOfItsOwnStillTimesOutAsOne(t *testing.T) {
	hang := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-hang:
		case <-r.Context().Done():
		}
	}))

	// Ordered, and not with t.Cleanup: Server.Close waits for the handler, so the
	// handler has to be released first. Deferred last means released first.
	defer server.Close()
	defer close(hang)

	client, err := New(Config{BaseURL: server.URL, Secret: testSecret})
	if err != nil {
		t.Fatalf("building the client: %v", err)
	}
	client.timeout = 100 * time.Millisecond

	// The transport's own bound is left where New put it — a hundred times the
	// one under test — so a call that comes back quickly came back on the context
	// deadline and on nothing else.
	started := time.Now()

	_, err = client.Invite(context.Background(), "anna@clinic.example")
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("a call to a handler that never answered came back as success")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("the call failed with %v, which no caller can tell from a refusal", err)
	}
	// Of the same order as the bound under test, like its mirror above: comparing
	// against callTimeout would also pass a deadline derived wrongly as five
	// seconds, which is a call that outlives the transaction it may be made in.
	if elapsed > time.Second {
		t.Errorf("the call took %s on a %s deadline, so what ended it was the transport's %s "+
			"and this client's own bound is not applied to the request",
			elapsed, client.timeout, callTimeout)
	}
}

// The two bounds a call runs under are two spellings of one number.
//
// A shape assertion, because the behavioural tests above each disable one of the
// two to measure the other and so cannot see either being left unset. What this
// forbids is a client built with a transport that waits forever, or with a
// deadline somebody tuned on its own.
func TestBothBoundsOfACallComeFromTheOneConstant(t *testing.T) {
	client, err := New(Config{BaseURL: "http://provisioner:8081", Secret: testSecret})
	if err != nil {
		t.Fatalf("building the client: %v", err)
	}

	if client.timeout != callTimeout {
		t.Errorf("the per-call deadline is %s, want %s", client.timeout, callTimeout)
	}
	if client.http.Timeout != callTimeout {
		t.Errorf("the transport waits %s, want %s", client.http.Timeout, callTimeout)
	}
}

func TestNewRefusesAConfigurationThatCouldNotWork(t *testing.T) {
	for name, cfg := range map[string]Config{
		"no address":         {Secret: testSecret},
		"a relative address": {BaseURL: "/provisioner", Secret: testSecret},
		"no secret":          {BaseURL: "http://provisioner:8081"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := New(cfg); err == nil {
				t.Error("the client was built anyway, and the first call is where it would be found")
			}
		})
	}
}

// The answer is what the component gives and nothing more.
func TestTheAnswerCarriesTheThreeFieldsAndNoOthers(t *testing.T) {
	confirmed := "2026-08-14T10:00:00Z"
	client, _ := fakeProvisioner(t, http.StatusOK,
		`{"account":{"id":"3f2a","confirmed_at":"`+confirmed+`",`+
			`"last_sign_in_at":null,"email":"anna@clinic.example",`+
			`"confirmation_token":"the-credential-itself"}}`)

	found, err := client.Lookup(context.Background(), "anna@clinic.example")
	if err != nil {
		t.Fatalf("looking up an address: %v", err)
	}
	if found == nil {
		t.Fatal("an account that exists came back as absent")
	}
	if found.ID != "3f2a" {
		t.Errorf("the account is %q, want 3f2a", found.ID)
	}
	if found.ConfirmedAt == nil || found.ConfirmedAt.Format(time.RFC3339) != confirmed {
		t.Errorf("confirmed_at came back as %v, want %s", found.ConfirmedAt, confirmed)
	}
	if found.LastSignInAt != nil {
		t.Errorf("last_sign_in_at came back as %v, want nothing", found.LastSignInAt)
	}

	// The credential is not on the type, so it cannot arrive by being forgotten.
	// Asserted by encoding what this package hands over: a field added to
	// identity.Account later would show up here.
	rendered, err := json.Marshal(found)
	if err != nil {
		t.Fatalf("encoding the account: %v", err)
	}
	if strings.Contains(string(rendered), "the-credential-itself") {
		t.Errorf("the confirmation token reached the context: %s", rendered)
	}
}

// A roster of nobody is answered without asking, because the component refuses
// an empty list — and a clinic that has not admitted anybody yet is an ordinary
// state, not a caller's mistake.
func TestARosterOfNobodyIsNotACallAtAll(t *testing.T) {
	client, record := fakeProvisioner(t, http.StatusBadRequest,
		`{"error":"ids must name between one and one hundred accounts"}`)

	for name, ids := range map[string][]string{"an empty list": {}, "no list at all": nil} {
		t.Run(name, func(t *testing.T) {
			accounts, err := client.LookupBatch(context.Background(), ids)
			if err != nil {
				t.Fatalf("looking up nobody: %v", err)
			}
			if len(accounts) != 0 {
				t.Errorf("looking up nobody found %d accounts", len(accounts))
			}
			if record.method != "" {
				t.Errorf("the component was called anyway, with %s", record.body)
			}
		})
	}
}

// No account at that address is an answer rather than a failure: it is what
// makes an address free to invite.
func TestAnAddressNobodyHasIsAnAnswer(t *testing.T) {
	client, _ := fakeProvisioner(t, http.StatusOK, `{"account":null}`)

	found, err := client.Lookup(context.Background(), "nobody@clinic.example")
	if err != nil {
		t.Fatalf("looking up an address: %v", err)
	}
	if found != nil {
		t.Errorf("an address nobody has came back as account %v", found)
	}
}

// Both halves of the proof are sent as the component requires them: stated, not
// inferred from a field somebody left out.
func TestDeletionStatesBothHalvesOfItsProof(t *testing.T) {
	client, record := fakeProvisioner(t, http.StatusNoContent, "")

	if err := client.Delete(context.Background(), identity.Deletion{
		ID: "3f2a", InviteExists: true, ProfileExists: false,
	}); err != nil {
		t.Fatalf("deleting an account: %v", err)
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte(record.body), &sent); err != nil {
		t.Fatalf("reading what was sent: %v", err)
	}

	for _, field := range []string{"id", "invite_exists", "profile_exists"} {
		if _, ok := sent[field]; !ok {
			t.Errorf("the request states %v, and the component refuses one without %q", sent, field)
		}
	}
	if sent["invite_exists"] != true || sent["profile_exists"] != false {
		t.Errorf("the proof travelled as %v", sent)
	}
}

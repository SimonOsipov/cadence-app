//go:build integration

// What the invitation actually puts in front of a patient, and what the app can do with it.
// Both halves are measured against the pinned image because both are undocumented: the template
// field that carries the token, and the route that spends it without a browser.
package identity_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// Where this template is expected to send patients: a page on the dashboard that reads the token
// out of the fragment — which no browser sends to a server — and hands it to the app. That the page
// and the app agree on the scheme is not measured here and cannot be, web/ and kmp/ being other
// stacks; `scripts/gate/kmp.sh` holds that whole chain together, reading it from ACCEPT_LINK.
const acceptLink = "/accept#token_hash="

// The token as the template renders it: GoTrue's own hashed token, which is hex.
var deepLinkToken = regexp.MustCompile(regexp.QuoteMeta(acceptLink) + `([0-9a-f]+)`)

// End to end, because either half alone passes while a patient is stuck: a mail carrying the right
// scheme and an empty token still arrives, still parses and still reads as Russian; and a token
// that spends cleanly proves nothing if no message ever carried it. So the token this asserts on
// is the one taken out of the delivered message, and it is spent.
func TestTheInvitationCarriesATokenTheAppCanSpend(t *testing.T) {
	mail := catchMail(t)
	provider, admin := providerSending(t, mail)

	if status, said := askOf(t, provider, "/invite", admin,
		map[string]string{"email": "carries@clinic.example"}); status != http.StatusOK {
		t.Fatalf("inviting answered %d: %s", status, said)
	}

	delivered := decodeQuotedPrintable(mail.await(t))

	carried := deepLinkToken.FindStringSubmatch(delivered)
	if carried == nil {
		t.Fatalf("the mail carries no %s<token>, so the invitation opens no app:\n%s",
			acceptLink, delivered)
	}

	spend := map[string]string{"type": "invite", "token_hash": carried[1]}

	status, said := askOf(t, provider, "/verify", "", spend)
	if status != http.StatusOK {
		t.Fatalf("the token the patient was sent answered %d: %s", status, said)
	}

	var session struct {
		Access  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
	}

	if err := json.Unmarshal([]byte(said), &session); err != nil {
		t.Fatalf("reading the session: %v", err)
	}

	// Both, because the app stores both: an answer carrying only an access token would leave a
	// patient signed in until it expires and then signed out for good.
	if session.Access == "" || session.Refresh == "" {
		t.Errorf("the answer carries no session, so the app has nothing to store: %s", said)
	}

	// The ordinary case — the mail opened on a second device — and the answer the acceptance
	// screen explains rather than retries. Pinned by status and by code, and both are load
	// bearing: `acceptInvitation` reads the status to decide whether another try is worth
	// offering, and carries the code onward for step 3 to write its Russian from.
	status, said = askOf(t, provider, "/verify", "", spend)
	if status != http.StatusForbidden {
		t.Errorf("spending the same token twice answered %d, not 403: %s", status, said)
	}

	if !strings.Contains(said, "otp_expired") {
		t.Errorf("the refusal does not say otp_expired, which is what the screen explains: %s", said)
	}
}

// providerSending starts a GoTrue that renders this repository's own invite template and hands the
// message to sink, and mints the admin token its routes want.
func providerSending(t *testing.T, sink *mailSink) (*testsupport.GoTrue, string) {
	t.Helper()

	key := testsupport.NewES256Key(t, "invite-link-key")

	provider := testsupport.StartGoTrueWith(t, cluster,
		testsupport.GoTrueJWKS(t, testsupport.JWKEntry{Key: key, Signing: true}),
		map[string]string{
			// Autoconfirm sends nothing; without SMTP nothing renders, and a template field is
			// then unobservable — which is how a misspelling would reach a patient.
			"GOTRUE_MAILER_AUTOCONFIRM":            "false",
			inviteTemplateVariable:                 serveTemplate(t, readTemplate(t, "invite.html")),
			"GOTRUE_SMTP_HOST":                     "host.docker.internal",
			"GOTRUE_SMTP_PORT":                     sink.port,
			"GOTRUE_SMTP_ADMIN_EMAIL":              "clinic@cadence.test",
			"GOTRUE_SMTP_SENDER_NAME":              "Cadence",
			testsupport.MailerMaxFrequencyVariable: "1ns",
			testsupport.EmailsPerHourVariable:      "1000",
			// Read from the deployment rather than spelled: a harness running the provider's own
			// floor would measure a bound the product does not have.
			testsupport.PasswordMinLengthVariable: deploymentSetting(t, testsupport.PasswordMinLengthVariable),
		})

	admin := key.Sign(t, jwt.MapClaims{
		"role": "service_role",
		"aud":  testsupport.GoTrueAudience,
		"iss":  testsupport.GoTrueIssuer,
		"exp":  time.Now().Add(time.Minute).Unix(),
	})

	return provider, admin
}

// serveTemplate answers body at an address the container can reach; v2.194.0 fetches templates
// over HTTP and cannot read one off disk.
func serveTemplate(t *testing.T, body string) string {
	t.Helper()

	listener, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening for the template: %v", err)
	}

	server := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		}),
	}

	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("reading the template port: %v", err)
	}

	return fmt.Sprintf("http://host.docker.internal:%s/invite.html", port)
}

type mailSink struct {
	port     string
	messages chan string
}

func (m *mailSink) await(t *testing.T) string {
	t.Helper()

	select {
	case message := <-m.messages:
		return message
	case <-time.After(mailTimeout):
		t.Fatal("no message arrived, so nothing was rendered to read")

		return ""
	}
}

const mailTimeout = 30 * time.Second

// catchMail speaks the few verbs GoTrue needs to hand over one message. Credentials are not
// configured on purpose: measured against v2.194.0, a server offering no STARTTLS is answered
// with QUIT the moment SMTP_USER is set, and /invite then fails with 500.
func catchMail(t *testing.T) *mailSink {
	t.Helper()

	// 0.0.0.0 and not loopback: the container reaches this through host.docker.internal.
	listener, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("listening for mail: %v", err)
	}

	t.Cleanup(func() { _ = listener.Close() })

	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("reading the mail port: %v", err)
	}

	sink := &mailSink{port: port, messages: make(chan string, 4)}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			go speakSMTP(conn, sink)
		}
	}()

	return sink
}

func speakSMTP(conn net.Conn, sink *mailSink) {
	defer func() { _ = conn.Close() }()

	reader := bufio.NewReader(conn)
	write := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }

	write("220 cadence.test ESMTP")

	var body strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		switch verb := strings.ToUpper(strings.TrimSpace(line)); {
		case strings.HasPrefix(verb, "EHLO"), strings.HasPrefix(verb, "HELO"):
			write("250-cadence.test")
			write("250 8BITMIME")
		case strings.HasPrefix(verb, "DATA"):
			write("354 go ahead")
			readMessage(reader, &body)
			write("250 2.0.0 ok")
			sink.messages <- body.String()
		case strings.HasPrefix(verb, "QUIT"):
			write("221 2.0.0 bye")

			return
		default:
			write("250 2.0.0 ok")
		}
	}
}

func readMessage(reader *bufio.Reader, into *strings.Builder) {
	for {
		line, err := reader.ReadString('\n')
		if err != nil || strings.TrimSpace(line) == "." {
			return
		}

		into.WriteString(line)
	}
}

// decodeQuotedPrintable undoes what the transfer encoding did to the link: soft line breaks split
// it, and every `=` in it — the one before the token included — arrives as `=3D`.
func decodeQuotedPrintable(message string) string {
	return strings.ReplaceAll(strings.ReplaceAll(message, "=\r\n", ""), "=3D", "=")
}

// The other way an invitation ends, and the one the app's mapping now rests on: a link that was
// never spent, refused because the clinic banned the patient. Told «already used», they would ask
// for another and be refused the same way; told «try again», they would ask for ever. Neither
// sentence is writable without knowing this answer, and it was an unpinned measurement until here.
func TestABannedPatientIsRefusedForADifferentReasonThanASpentLink(t *testing.T) {
	mail := catchMail(t)
	provider, admin := providerSending(t, mail)

	status, said := askOf(t, provider, "/admin/generate_link", admin,
		map[string]string{"type": "invite", "email": "banned@clinic.example"})
	if status != http.StatusOK {
		t.Fatalf("generate_link answered %d: %s", status, said)
	}

	var generated struct {
		ID     string `json:"id"`
		Hashed string `json:"hashed_token"`
	}

	if err := json.Unmarshal([]byte(said), &generated); err != nil {
		t.Fatalf("reading the generated link: %v", err)
	}

	banUntilTheClinicSaysOtherwise(t, provider, admin, generated.ID)

	status, said = askOf(t, provider, "/verify", "",
		map[string]string{"type": "invite", "token_hash": generated.Hashed})

	if status != http.StatusForbidden {
		t.Errorf("an unspent link held by a banned patient answered %d, not 403: %s", status, said)
	}

	// The code and not merely the status: `otp_expired` is what a spent link answers, and the two
	// need different sentences. A refusal that named neither would leave the screen with nothing.
	if !strings.Contains(said, "user_banned") {
		t.Errorf("the refusal does not say user_banned, so it is indistinguishable from a "+
			"spent link: %s", said)
	}
}

func banUntilTheClinicSaysOtherwise(t *testing.T, provider *testsupport.GoTrue, admin, id string) {
	t.Helper()

	encoded, err := json.Marshal(map[string]string{"ban_duration": "876000h"})
	if err != nil {
		t.Fatalf("encoding the ban: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut,
		provider.URL+"/admin/users/"+id, bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("building the ban: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+admin)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("banning: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("banning answered %d: %s", resp.StatusCode, body)
	}
}

// Where the password floor actually is, against the provider that enforces it. The screen states
// the rule before the server has to refuse, so the number it states has to be this one — and the
// refusal it explains has to be this shape.
func TestThePasswordBoundIsWhereTheDeploymentPutIt(t *testing.T) {
	floor, err := strconv.Atoi(deploymentSetting(t, testsupport.PasswordMinLengthVariable))
	if err != nil {
		t.Fatalf("the deployment names no password floor: %v", err)
	}

	mail := catchMail(t)
	provider, admin := providerSending(t, mail)

	token := aSessionFromAnInvitation(t, provider, admin, "floor@clinic.example")

	// The pair is the measurement: one row alone passes on a provider that refuses everything, or
	// on one that refuses nothing.
	short := strings.Repeat("a", floor-1)

	status, said := setPassword(t, provider.URL, token, short)
	if status != http.StatusUnprocessableEntity {
		t.Errorf("a password of %d characters answered %d, not 422: %s", len(short), status, said)
	}

	// The code and the reason, because the screen writes its Russian from them rather than from
	// the provider's English sentence.
	for _, names := range []string{"weak_password", "length"} {
		if !strings.Contains(said, names) {
			t.Errorf("the refusal does not name %q, so the screen cannot explain it: %s", names, said)
		}
	}

	long := strings.Repeat("a", floor)

	if status, said = setPassword(t, provider.URL, token, long); status != http.StatusOK {
		t.Errorf("a password of exactly %d characters answered %d: %s", floor, status, said)
	}
}

// aSessionFromAnInvitation walks the flow far enough to hold an access token.
func aSessionFromAnInvitation(t *testing.T, provider *testsupport.GoTrue, admin, address string) string {
	t.Helper()

	status, said := askOf(t, provider, "/admin/generate_link", admin,
		map[string]string{"type": "invite", "email": address})
	if status != http.StatusOK {
		t.Fatalf("generate_link answered %d: %s", status, said)
	}

	var generated struct {
		Hashed string `json:"hashed_token"`
	}

	if err := json.Unmarshal([]byte(said), &generated); err != nil {
		t.Fatalf("reading the generated link: %v", err)
	}

	status, said = askOf(t, provider, "/verify", "",
		map[string]string{"type": "invite", "token_hash": generated.Hashed})
	if status != http.StatusOK {
		t.Fatalf("spending the token answered %d: %s", status, said)
	}

	var session struct {
		Access string `json:"access_token"`
	}

	if err := json.Unmarshal([]byte(said), &session); err != nil {
		t.Fatalf("reading the session: %v", err)
	}

	return session.Access
}

func setPassword(t *testing.T, base, token, password string) (int, string) {
	t.Helper()

	encoded, err := json.Marshal(map[string]string{"password": password})
	if err != nil {
		t.Fatalf("encoding the password: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut,
		base+"/user", bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("calling PUT /user: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the answer: %v", err)
	}

	return resp.StatusCode, string(body)
}

// What the clinic's mail looks like before it is sent, measured against the files and the
// deployment rather than against a description of them.
//
// Nothing here observes an email: there is no SMTP provider to send one — SKL-01, not yet chosen —
// so «the Russian email arrives» is not asserted anywhere and is not done.
//
// What this file used to claim, and what cost it a shipped defect: that the provider renders a
// template only when it has an SMTP server. It does not.
// `templatemailer.FromConfig` substitutes a noop client when SMTP.Host is empty,
// and `Mailer.mail` fetches and executes the template BEFORE handing anything to
// that client. So the fetch is observable without a provider, and the fetch is
// exactly where three review rounds of `file://` URLs were failing silently.
package identity_test

import (
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"text/template"
	"time"
	"unicode"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// Read off the config struct of the binary docker-compose.yml pins rather than out of its
// documentation: envconfig derives these names from field names, and one the provider does not read
// is silent — the failure GOTRUE_MAILER_MAX_FREQUENCY produced at step 2.
const (
	inviteSubjectVariable    = "GOTRUE_MAILER_SUBJECTS_INVITE"
	magicLinkSubjectVariable = "GOTRUE_MAILER_SUBJECTS_MAGIC_LINK"
	recoverySubjectVariable  = "GOTRUE_MAILER_SUBJECTS_RECOVERY"

	inviteTemplateVariable    = "GOTRUE_MAILER_TEMPLATES_INVITE"
	magicLinkTemplateVariable = "GOTRUE_MAILER_TEMPLATES_MAGIC_LINK"
	recoveryTemplateVariable  = "GOTRUE_MAILER_TEMPLATES_RECOVERY"

	senderNameVariable = "GOTRUE_SMTP_SENDER_NAME"
)

// The container path and the repository directory the mount carries there. Both are pinned because
// a test naming only the URLs stays green with the mount deleted, and the provider then fetches
// nothing. templateServerRoot is a path on the serving container, never on the auth one.
const (
	templateServerRoot = "/usr/share/nginx/html"
	templateOrigin     = "http://mail-templates"
	templateSource     = "mail-templates"
)

// The two link placeholders, both measured against the pinned image by rendering one: a template
// served to a GoTrue with an SMTP sink behind it fills TokenHash with the same value the
// ConfirmationURL carries as its token (2026-09-01). A misspelling silently renders empty, which
// is a mail with a dead link rather than a refusal anybody sees.
const (
	confirmationURL = "{{ .ConfirmationURL }}"
	tokenHash       = "{{ .TokenHash }}"
	siteURL         = "{{ .SiteURL }}"
)

// The three mails this block causes the provider to send. Confirmation and email-change are absent
// on purpose: signup is disabled, and changing an address is out of scope, left at the English default.
var mails = []struct {
	name     string
	subject  string
	template string
	file     string

	// says is a phrase belonging to this mail and to no other. Without it the three files are
	// interchangeable to the suite — all Russian, all carrying the link, all parsing — so swapping
	// recovery.html and magic-link.html passes while a patient who asked to recover is told how to
	// sign in.
	says string

	// carries is the link this mail delivers, and the invitation's is not the others'. It leads to
	// a page on the dashboard rather than through GoTrue's redirect: the token rides in the
	// fragment, which no browser sends to a server, and the page hands it to the app — see the
	// proposal «Приём приглашения: PKCE недостижим».
	carries string
}{
	{
		name:     "the invitation",
		subject:  inviteSubjectVariable,
		template: inviteTemplateVariable,
		file:     "invite.html",
		says:     "завела для вас личный кабинет",
		carries:  siteURL + "/accept#token_hash=" + tokenHash,
	},
	{
		name:     "the sign-in link",
		subject:  magicLinkSubjectVariable,
		template: magicLinkTemplateVariable,
		file:     "magic-link.html",
		says:     "Ссылка одноразовая",
		carries:  confirmationURL,
	},
	{
		name:     "the recovery link",
		subject:  recoverySubjectVariable,
		template: recoveryTemplateVariable,
		file:     "recovery.html",
		says:     "восстановление доступа",
		// The same shape as the invitation's since the interstitial went in: a page on the
		// dashboard, the token in the fragment, and the app reached from there.
		carries: siteURL + "/recover#token_hash=" + tokenHash,
	},
}

// The first half alone would pass on three identical files; the second is what makes a swap fail.
func TestEachTemplateSaysWhatItsOwnMailIsFor(t *testing.T) {
	for _, mail := range mails {
		t.Run(mail.name, func(t *testing.T) {
			if body := readTemplate(t, mail.file); !strings.Contains(body, mail.says) {
				t.Errorf("%s does not say %q, so it is not recognisably about %s",
					mail.file, mail.says, mail.name)
			}

			for _, other := range mails {
				if other.file == mail.file {
					continue
				}
				if body := readTemplate(t, other.file); strings.Contains(body, mail.says) {
					t.Errorf("%s also says %q: the two are interchangeable, and a swap "+
						"would send the wrong mail under the right subject",
						other.file, mail.says)
				}
			}
		})
	}
}

func TestEveryMailTheBlockSendsHasARussianSubject(t *testing.T) {
	for _, mail := range mails {
		t.Run(mail.name, func(t *testing.T) {
			subject := deploymentSetting(t, mail.subject)

			requireRussian(t, subject, "the subject of "+mail.name)
		})
	}
}

func TestTheSenderIsNamedInRussian(t *testing.T) {
	requireRussian(t, deploymentSetting(t, senderNameVariable), "the sender name")
}

// The templates the deployment names are the files this repository carries, reaching the provider
// the only way it accepts them — over HTTP, from the service that serves them. This test used to
// require the opposite, a `file://` URL and a mount onto the auth container, and asserting that is
// what kept the defect alive through three review rounds: the suite was pinning the mistake.
func TestEveryMailIsRenderedFromATemplateInThisRepository(t *testing.T) {
	if mount := "./" + templateSource + ":" + templateServerRoot; !composeMounts(t, mount) {
		t.Errorf("no line of docker-compose.yml mounts %s, so the service that serves the "+
			"templates has nothing to serve and every fetch falls back to English", mount)
	}

	for _, mail := range mails {
		t.Run(mail.name, func(t *testing.T) {
			want := templateOrigin + "/" + mail.file
			if set := deploymentSetting(t, mail.template); set != want {
				t.Errorf("%s is %q, want %q", mail.template, set, want)
			}

			if _, err := os.Stat(templatePath(t, mail.file)); err != nil {
				t.Errorf("the deployment renders %s from a file this repository does not "+
					"have: %v", mail.name, err)
			}
		})
	}
}

func TestEveryTemplateIsRussianAndCarriesTheLink(t *testing.T) {
	for _, mail := range mails {
		t.Run(mail.name, func(t *testing.T) {
			body := readTemplate(t, mail.file)

			requireRussian(t, body, mail.file)

			if !strings.Contains(body, mail.carries) {
				t.Errorf("%s carries no %s, so the mail arrives without the link it exists "+
					"to deliver", mail.file, mail.carries)
			}

			// A syntax check of the {{ }} dialect, not a claim about which engine renders these. With
			// no SMTP nothing else parses these files, so an unbalanced action would wait for SKL-01.
			if _, err := template.New(mail.file).Parse(body); err != nil {
				t.Errorf("%s does not parse: %v", mail.file, err)
			}
		})
	}
}

func TestTheInvitationStatesTheLifetimeTheProviderEnforces(t *testing.T) {
	// Truncating to hours would let 90 minutes match copy that says "1 час".
	if identity.InviteLinkLifetime%time.Hour != 0 {
		t.Fatalf("identity.InviteLinkLifetime is %s, which no whole number of hours states; "+
			"invite.html has to say something this test cannot check",
			identity.InviteLinkLifetime)
	}

	// With the unit, so the count is not matched against whatever else in the file has those digits.
	stated := strconv.Itoa(int(identity.InviteLinkLifetime.Hours())) + " час"

	if body := readTemplate(t, "invite.html"); !strings.Contains(body, stated) {
		t.Errorf("invite.html does not say %q; identity.InviteLinkLifetime is %s, and the "+
			"copy has drifted from the window the link actually has",
			stated, identity.InviteLinkLifetime)
	}
}

// Shape, not merely presence: `**` on its own is non-empty and would satisfy every other assertion
// here while opening the list to any host a caller names — and the session rides in the fragment of
// the address the link lands on. That the list governs at all is measured by
// TestTheAllowListDecidesWhereALinkLands.
func TestTheDeploymentSetsARedirectAllowList(t *testing.T) {
	allowed := deploymentSetting(t, testsupport.RedirectAllowListVariable)
	if allowed == "" {
		t.Fatalf("%s is empty, so this environment inherited the provider's default rather "+
			"than choosing where a link may land", testsupport.RedirectAllowListVariable)
	}

	for _, entry := range strings.Split(allowed, ",") {
		entry = strings.TrimSpace(entry)

		parsed, err := url.Parse(entry)
		if err != nil || parsed.Host == "" {
			t.Errorf("%q names no host, so it does not restrict where a link lands", entry)
			continue
		}
		if strings.Contains(parsed.Host, "*") {
			t.Errorf("%q wildcards the host: any origin a caller names would be allowed, "+
				"and the session travels in the landing address", entry)
		}
	}
}

// The provider fetches a template; it never reads one off disk. `loadEntryBody`
// prefixes anything not starting with "http" with SITE_URL and then issues a GET,
// so `file:///etc/gotrue/invite.html` becomes
// `http://localhost:8080file:///etc/gotrue/invite.html` and fails to parse.
//
// The failure is silent and costs more than the body: `load` falls back to
// `loadEntryDefault`, which returns the provider's English body AND its English
// subject — so a broken URL here also throws away the SUBJECTS_ lines.
//
// That is the defect that shipped: all three variables named `file://` paths, and three review
// rounds passed over them because every assertion in this file compared the deployment's text
// against constants declared beside it.
//
// This measures the scheme, the class the defect belonged to. It does NOT measure reachability —
// `http://typo/invite.html` passes here and fails in production. That needs a container fetching
// from a server in the test process, and it is not written; the gap is named rather than implied.
func TestEveryMailTemplateIsFetchableByScheme(t *testing.T) {
	for _, variable := range []string{
		"GOTRUE_MAILER_TEMPLATES_INVITE",
		"GOTRUE_MAILER_TEMPLATES_MAGIC_LINK",
		"GOTRUE_MAILER_TEMPLATES_RECOVERY",
	} {
		t.Run(variable, func(t *testing.T) {
			configured := deploymentSetting(t, variable)
			if configured == "" {
				t.Fatalf("%s is unset: the patient would get the provider's English "+
					"body, though the Russian subject would survive", variable)
			}

			parsed, err := url.Parse(configured)
			if err != nil {
				t.Fatalf("%s = %q does not parse as a URL: %v", variable, configured, err)
			}

			if parsed.Scheme != "http" && parsed.Scheme != "https" {
				t.Errorf("%s = %q has scheme %q; the provider issues an HTTP GET and "+
					"cannot read this, so the template and its subject both fall back "+
					"to English", variable, configured, parsed.Scheme)
			}

			if parsed.Host == "" {
				t.Errorf("%s = %q names no host, so SITE_URL is prefixed onto it",
					variable, configured)
			}
		})
	}
}

func templatePath(t *testing.T, file string) string {
	t.Helper()

	return filepath.Join(moduleRoot(t), templateSource, file)
}

func readTemplate(t *testing.T, file string) string {
	t.Helper()

	body, err := os.ReadFile(templatePath(t, file))
	if err != nil {
		t.Fatalf("reading the template: %v", err)
	}

	return string(body)
}

// composeMounts reads the list entry rather than the file's text, for the reason deploymentSetting
// gives: a commented-out mount still contains the pair of paths.
func composeMounts(t *testing.T, mount string) bool {
	t.Helper()

	compose, err := os.ReadFile(filepath.Join(moduleRoot(t), "docker-compose.yml"))
	if err != nil {
		t.Fatalf("reading the deployment: %v", err)
	}

	for line := range strings.SplitSeq(string(compose), "\n") {
		entry, found := strings.CutPrefix(strings.TrimSpace(line), "- ")
		if found && strings.HasPrefix(entry, mount) {
			return true
		}
	}

	return false
}

// The whole of «it is in Russian» a machine can carry: what it refuses is the English default.
func requireRussian(t *testing.T, text, what string) {
	t.Helper()

	for _, r := range text {
		if unicode.Is(unicode.Cyrillic, r) {
			return
		}
	}

	t.Errorf("%s has no Cyrillic in it: %q. RU is the product language, and the provider's "+
		"own default is English", what, text)
}

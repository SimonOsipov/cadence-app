//go:build integration

// The cycle, end to end: an empty clinic, the one-off command that gives it its first
// administrator, a doctor taken on, a patient created, both signing in by the link they were sent,
// and the rows each of them can see afterwards.
//
// What separates this from the route suites beside it is that nothing here hands a handler a
// principal. Every request carries the token the provider minted for the person making it, over the
// whole surface cmd/api mounts, and the role deciding what happens is the one the issuance hook
// wrote from a profile some earlier request created.
package identity_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth/token"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
	"github.com/SimonOsipov/cadence-app/api/internal/router"
)

// The mailboxes the cycle uses, each its own for the reason testsupport.Cycle records. Two doctors
// and two patients, assigned crosswise: with one of each, every row a doctor could read is one they
// are entitled to, and «does not see the other one» is a claim about an empty table.
const (
	theAdminsAddress = "petr.averin@clinic.example"

	theDoctorsAddress  = "olga.timofeeva@clinic.example"
	thePatientsAddress = "irina.sokolova@clinic.example"

	theOtherDoctorsAddress  = "oleg.kim@clinic.example"
	theOtherPatientsAddress = "anna.lebedeva@clinic.example"
)

// person is somebody the cycle produced: the account the provider assigned and the session their
// link landed in.
type person struct {
	account string
	access  string
	refresh string
}

// cycleClinic is the whole API in front of the cycle's database and provider, assembled by the
// function the composition root uses.
type cycleClinic struct {
	*onboarding

	mux      *chi.Mux
	verifier *token.Verifier
}

// onlineClinic mounts that surface over an empty clinic.
func onlineClinic(t *testing.T) *cycleClinic {
	t.Helper()

	clinic := freshClinic(t)

	// Pointed at the provider's own key set, as a deployment derives it from the issuer. A fixture
	// serving the public half of cycle.SigningKey would verify the same tokens and leave the moving
	// part — the provider publishing the key it signs with — unmeasured.
	verifier, err := token.NewVerifier(t.Context(), token.VerifierConfig{
		Issuer:      testsupport.GoTrueIssuer,
		Audience:    testsupport.GoTrueAudience,
		JWKSURL:     cycle.GoTrue.URL + testsupport.JWKSPath,
		SessionKIDs: []string{cycle.SigningKey.KID},
	})
	if err != nil {
		t.Fatalf("building the verifier: %v", err)
	}

	mux := chi.NewRouter()
	router.Mount(mux, router.Options{
		Verifier:    verifier,
		Pool:        clinic.requests,
		ServicePool: clinic.writes,
		Provisioner: clinic.provider,
	})

	return &cycleClinic{onboarding: clinic, mux: mux, verifier: verifier}
}

// send drives one request the way a client does: the whole surface, a bearer token, and nothing put
// behind the guard by the test.
func (c *cycleClinic) send(t *testing.T, path, bearer, payload string) answer {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)

	rec := httptest.NewRecorder()
	c.mux.ServeHTTP(rec, req)

	return answer{status: rec.Code, body: rec.Body.String()}
}

// whoTheTokenSays is the API's own verifier reading a session token — the one place the role
// inside one is a value this process can assert on, since on the request path the claim is read,
// turned into a principal and spent, and a handler seeing the wrong role answers wrongly in silence.
func (c *cycleClinic) whoTheTokenSays(t *testing.T, access string) auth.Principal {
	t.Helper()

	principal, err := c.verifier.Verify(t.Context(), access)
	if err != nil {
		t.Fatalf("the API refused a token the provider minted: %v", err)
	}

	return principal
}

// visibleTo reads one relation through the request seam as the caller a token names — the same road
// every screen of the product reads it by. The statement is a constant at every call site: nothing
// a request carried reaches it.
func (c *cycleClinic) visibleTo(t *testing.T, caller auth.Principal, statement string) []string {
	t.Helper()

	var seen []string

	if err := database.WithCaller(
		t.Context(), c.requests,
		database.Caller{Subject: caller.Subject, Role: caller.Role},
		func(ctx context.Context, tx pgx.Tx) error {
			rows, err := tx.Query(ctx, statement)
			if err != nil {
				return err
			}

			seen, err = pgx.CollectRows(rows, pgx.RowTo[string])

			return err
		},
	); err != nil {
		t.Fatalf("reading what %s can see: %v", caller.Subject, err)
	}

	slices.Sort(seen)

	return seen
}

// refusalReading is the same read where being refused is the answer.
func (c *cycleClinic) refusalReading(
	t *testing.T, caller auth.Principal, statement string,
) error {
	t.Helper()

	return database.WithCaller(
		t.Context(), c.requests,
		database.Caller{Subject: caller.Subject, Role: caller.Role},
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, statement)

			return err
		},
	)
}

// cycleWalk is the clinic one walk produced: an administrator and two doctors, each with a patient.
type cycleWalk struct {
	clinic *cycleClinic

	admin person

	doctor  person
	patient person

	otherDoctor  person
	otherPatient person
}

// walkTheCycle runs the sequence a clinic actually goes through, from a database with nobody in it
// to patients holding sessions. It asserts only that each step went through: the walk is the
// arrangement, and an arrangement that quietly half-failed would otherwise be measured as a property.
func walkTheCycle(t *testing.T) cycleWalk {
	t.Helper()

	clinic := onlineClinic(t)

	admin := theFirstAdministrator(t, "Пётр Аверин")

	doctor := clinic.hire(t, admin, theDoctorsAddress, "Эндокринолог")
	patient := clinic.take(t, doctor, thePatientsAddress)

	otherDoctor := clinic.hire(t, admin, theOtherDoctorsAddress, "Диетолог")

	return cycleWalk{
		clinic:       clinic,
		admin:        admin,
		doctor:       doctor,
		patient:      patient,
		otherDoctor:  otherDoctor,
		otherPatient: clinic.take(t, otherDoctor, theOtherPatientsAddress),
	}
}

// hire is the administrator taking a doctor on, through to the doctor holding a session.
func (c *cycleClinic) hire(t *testing.T, admin person, address, title string) person {
	t.Helper()

	hired := c.send(t, providersPath, admin.access, providerBody(address, title))
	if hired.status != http.StatusCreated {
		t.Fatalf("the administrator could not take on %s: %d %s", address, hired.status, hired.body)
	}

	return signIn(t, address)
}

// invited is take() stopped one step short: the patient exists, the invitation has gone out, and
// nobody has opened it. It is the state most of a clinic's roster is in on any given day.
func (c *cycleClinic) invited(t *testing.T, doctor person, address string) string {
	t.Helper()

	created := c.send(t, patientsPath, doctor.access, body(address, doctor.account))
	if created.status != http.StatusCreated {
		t.Fatalf("the doctor could not create %s: %d %s", address, created.status, created.body)
	}

	return accountID(t, address)
}

// take is a doctor creating a patient and putting themselves on their care team, through to the
// patient holding a session.
func (c *cycleClinic) take(t *testing.T, doctor person, address string) person {
	t.Helper()

	created := c.send(t, patientsPath, doctor.access, body(address, doctor.account))
	if created.status != http.StatusCreated {
		t.Fatalf("the doctor could not create %s: %d %s", address, created.status, created.body)
	}

	return signIn(t, address)
}

// theFirstAdministrator is the sequence a deployment starts with: the account is created at the
// provider, and the one-off command writes the profile and the audit row under the migration role.
// The command is run rather than imitated — it is a package main, and a copy of its two statements
// here would be a second implementation, which this suite would then be measuring.
func theFirstAdministrator(t *testing.T, name string) person {
	t.Helper()

	// Compiled before the invitation rather than run after it: the link is good for HarnessOTPExpiry,
	// and on a cold build cache a `go run` here would spend that minute linking a binary and then
	// fail as a link the provider refused.
	command := bootstrapAdminBinary(t)

	invite(t, theAdminsAddress)
	account := accountID(t, theAdminsAddress)

	run := exec.CommandContext(t.Context(), command, account, name)
	run.Env = append(os.Environ(), "DATABASE_MIGRATION_URL="+cycle.DB.MigrationURL)

	if said, err := run.CombinedOutput(); err != nil {
		t.Fatalf("bootstrap-admin refused to make %s the administrator: %v\n%s", account, err, said)
	}

	return signIn(t, theAdminsAddress)
}

func bootstrapAdminBinary(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "bootstrap-admin")

	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "./cmd/bootstrap-admin")
	build.Dir = testsupport.ModuleRoot(t)

	if said, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building bootstrap-admin: %v\n%s", err, said)
	}

	return binary
}

// signIn opens the link in somebody's mailbox and comes back with the session it landed in — the
// access token every later request carries, and the refresh token that outlives it.
func signIn(t *testing.T, address string) person {
	t.Helper()

	accepted, location := follow(t, inviteToken(t, address), "invite", "")
	if !accepted {
		t.Fatalf("the invitation link to %s was refused: %s", address, location)
	}

	// In the fragment, where a browser leaves it for the client rather than sending it to the target.
	fragment := location
	if _, after, found := strings.Cut(location, "#"); found {
		fragment = after
	}

	landed, err := url.ParseQuery(fragment)
	if err != nil {
		t.Fatalf("reading the session %s landed in: %v", address, err)
	}

	who := person{
		account: accountID(t, address),
		access:  landed.Get("access_token"),
		refresh: landed.Get("refresh_token"),
	}

	if who.access == "" || who.refresh == "" {
		t.Fatalf("the link for %s landed on %s, which carries no session", address, location)
	}

	return who
}

// All three roles, not the patient alone: the hook copies app.profiles.role verbatim, so a route
// writing the wrong role produces a token that is correct about a person the clinic did not mean to
// create. The administrator's is the one no route wrote — it comes from the one-off command.
func TestEveryTokenTheCycleIssuesCarriesThatPersonsOwnRole(t *testing.T) {
	walked := walkTheCycle(t)

	for _, who := range []struct {
		name string
		is   person
		role string
	}{
		{name: "the administrator", is: walked.admin, role: "admin"},
		{name: "the doctor", is: walked.doctor, role: "doctor"},
		{name: "the patient", is: walked.patient, role: "patient"},
		{name: "the second doctor", is: walked.otherDoctor, role: "doctor"},
		{name: "the second patient", is: walked.otherPatient, role: "patient"},
	} {
		t.Run(who.name, func(t *testing.T) {
			principal := walked.clinic.whoTheTokenSays(t, who.is.access)

			if principal.Subject != who.is.account {
				t.Errorf("the token names %s, want the account the provider assigned, %s",
					principal.Subject, who.is.account)
			}

			if principal.Role != who.role {
				t.Errorf("the token carries the role %q, want %q", principal.Role, who.role)
			}
		})
	}
}

// The care team is read directly rather than left to the tables whose visibility derives from it:
// RLS on a referenced table only removes rows from an EXISTS subquery, and each of those predicates
// carries its own `= app.jwt_subject()`, so a care-team policy widened to `USING (true)` leaves
// every derived set below unchanged and a doctor could read the clinic's whole care graph unnoticed.
//
// `user_preferences` is the fifth and is measured by TestADoctorIsNotEvenGrantedTheReminderSettings,
// because a doctor is stopped there by a missing grant rather than by a policy.
const (
	everyProfile        = `SELECT user_id::text FROM app.profiles`
	everyPatientProfile = `SELECT user_id::text FROM app.patient_profiles`
	everyProviderCard   = `SELECT user_id::text FROM app.provider_profiles`
	everyAssignment     = `SELECT patient_id::text FROM app.care_team_assignments`
	everyPreference     = `SELECT user_id::text FROM app.user_preferences`
)

// The policy suite's result reached by another road: there the caller was a value a test
// constructed, here it is the subject and role of a token the provider minted for somebody the
// routes created, and these five people are all the clinic has.
//
// The whole set rather than a membership check, because the failure this catches adds rows instead
// of removing them: a doctor policy written as «role = 'doctor'» rather than through the care team
// passes every «can see» assertion and hands one doctor every patient in the clinic.
func TestVisibilityAtTheEndOfTheCycleIsWhatThePolicySuiteProved(t *testing.T) {
	walked := walkTheCycle(t)

	for _, who := range []struct {
		name string
		is   person

		profiles      []string
		patientCards  []string
		providerCards []string
		assignments   []string
	}{
		{
			name:          "the patient",
			is:            walked.patient,
			profiles:      []string{walked.patient.account, walked.doctor.account},
			patientCards:  []string{walked.patient.account},
			providerCards: []string{walked.doctor.account},
			assignments:   []string{walked.patient.account},
		},
		{
			name:          "the doctor",
			is:            walked.doctor,
			profiles:      []string{walked.doctor.account, walked.patient.account},
			patientCards:  []string{walked.patient.account},
			providerCards: []string{walked.doctor.account},
			assignments:   []string{walked.patient.account},
		},
		{
			name: "the administrator",
			is:   walked.admin,
			profiles: []string{
				walked.admin.account,
				walked.doctor.account, walked.patient.account,
				walked.otherDoctor.account, walked.otherPatient.account,
			},
			patientCards:  []string{walked.patient.account, walked.otherPatient.account},
			providerCards: []string{walked.doctor.account, walked.otherDoctor.account},
			assignments:   []string{walked.patient.account, walked.otherPatient.account},
		},
	} {
		t.Run(who.name, func(t *testing.T) {
			caller := walked.clinic.whoTheTokenSays(t, who.is.access)

			for _, relation := range []struct {
				name      string
				statement string
				want      []string
			}{
				{name: "profiles", statement: everyProfile, want: who.profiles},
				{name: "patient cards", statement: everyPatientProfile, want: who.patientCards},
				{name: "provider cards", statement: everyProviderCard, want: who.providerCards},
				{name: "care team", statement: everyAssignment, want: who.assignments},
			} {
				t.Run(relation.name, func(t *testing.T) {
					want := slices.Clone(relation.want)
					slices.Sort(want)

					if seen := walked.clinic.visibleTo(
						t, caller, relation.statement,
					); !slices.Equal(seen, want) {
						t.Errorf("sees %v, want exactly %v", seen, want)
					}
				})
			}
		})
	}
}

// The refusals, driven by the tokens of the people the cycle created rather than by principals a
// test wrote down: the route suites prove each rule against a caller they constructed, and these
// prove the rule is reached by the role the provider actually put in somebody's token.
func TestWhatTheCycleRefuses(t *testing.T) {
	walked := walkTheCycle(t)
	clinic := walked.clinic

	t.Run("a doctor may not take on staff", func(t *testing.T) {
		refused := clinic.send(t, providersPath, walked.doctor.access,
			providerBody("hired.by.a.doctor@clinic.example", ""))

		requireRefusal(t, refused, http.StatusForbidden, "только администратор")
	})

	t.Run("a patient may not create patients", func(t *testing.T) {
		refused := clinic.send(t, patientsPath, walked.patient.access,
			body("created.by.a.patient@clinic.example", walked.doctor.account))

		requireRefusal(t, refused, http.StatusForbidden, "нет прав заводить пациентов")
	})

	// The administrator is the one caller the staff route admits, so this is where a body naming a
	// role would be read if anything read it. Nothing does, and the refusal must cost nothing: the
	// address has to still be free afterwards.
	t.Run("not even an administrator creates an administrator", func(t *testing.T) {
		const address = "second.admin@clinic.example"

		refused := clinic.send(t, providersPath, walked.admin.access,
			`{"email":"`+address+`","full_name":"Мария Седова","role":"admin"}`)

		if refused.status != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422: %s", refused.status, refused.body)
		}

		// Which 422 it is: the schema answers the same status for every violation, so a body refused
		// for its name or its address would satisfy the check above and say nothing about the role.
		problem := problemFrom(t, refused)
		if !slices.ContainsFunc(problem.Errors, func(refusal httpserver.ProblemDetail) bool {
			return strings.Contains(refusal.Location, "role")
		}) {
			t.Errorf("the refusal does not name the field it refused: %+v", problem.Errors)
		}

		var accounts int
		scanProvider(t, `SELECT count(*) FROM auth.users WHERE email = $1`, []any{address}, &accounts)

		if accounts != 0 {
			t.Errorf("%d accounts exist at %s after a refused body: the address is spent",
				accounts, address)
		}
	})

	// An account at the address with no record of it here is somebody else's, and claiming deletes —
	// so the refusal is what stands between a stranger's account and a new patient.
	t.Run("an account this clinic did not invite", func(t *testing.T) {
		// TestAnAccountThisClinicDidNotInviteIsRefused's address is free only because nothing in this
		// package runs in parallel.
		const address = "stranger.account@clinic.example"

		invite(t, address)

		refused := clinic.send(t, patientsPath, walked.doctor.access,
			body(address, walked.doctor.account))

		requireRefusal(t, refused, http.StatusConflict, "клиника не приглашала")
	})
}

// The invariant the whole block rests on, and the one nothing else in this package measures. Both
// routes a stranger can reach without a token, and the account count after each: a 422 that created
// an unconfirmed user anyway would leave an address this clinic can never invite — the same
// permanent 409 the claim rule exists to avoid.
func TestThereIsNoPublicRegistration(t *testing.T) {
	cycle.Reset(t)

	// An address per route: with one between them, a /signup that created an account would fail the
	// /otp case too, for a reason that is not /otp's.
	for path, stranger := range map[string]string{
		"/signup": "walked.in@clinic.example",
		"/otp":    "asked.for.a.code@clinic.example",
	} {
		t.Run(path, func(t *testing.T) {
			status, said := ask(t, path, "", map[string]string{
				"email": stranger, "password": "a-password-nobody-uses",
			})

			if status != http.StatusUnprocessableEntity {
				t.Errorf("%s answered %d, want 422: %s", path, status, said)
			}

			// Which refusal it was: a 422 for a malformed body would pass the check above.
			if !strings.Contains(said, "signup_disabled") {
				t.Errorf("%s was refused with %s, which is not signup being disabled", path, said)
			}

			var accounts int
			scanProvider(t, `SELECT count(*) FROM auth.users WHERE email = $1`,
				[]any{stranger}, &accounts)

			if accounts != 0 {
				t.Errorf("%d accounts exist at %s after a refused registration", accounts, stranger)
			}
		})
	}
}

// The one relation of the five a doctor does not reach at all: 000004 grants SELECT on it to the
// patient, the service path and the administrator, and to nobody else.
//
// The witness is that the read fails rather than comes back empty, which is why it is not a set
// beside the others: a policy filters a SELECT instead of raising, so zero rows are
// indistinguishable from a clinic whose patients have no settings, and green on the day somebody
// grants the read «to show a reminder time». refusedBy pins the privilege error specifically.
func TestADoctorIsNotEvenGrantedTheReminderSettings(t *testing.T) {
	walked := walkTheCycle(t)

	if theirs := walked.clinic.visibleTo(
		t, walked.clinic.whoTheTokenSays(t, walked.patient.access), everyPreference,
	); !slices.Equal(theirs, []string{walked.patient.account}) {
		t.Errorf("the patient reads %v of their own settings, want exactly their row", theirs)
	}

	refused := walked.clinic.refusalReading(
		t, walked.clinic.whoTheTokenSays(t, walked.doctor.access), everyPreference,
	)
	if refused == nil {
		t.Fatal("a doctor reached app.user_preferences: whatever they can read there now, " +
			"the grant that kept them out is gone")
	}

	if by := refusedBy(t, refused); by != grantRefusal {
		t.Errorf("refused by %s, want %s", by, grantRefusal)
	}
}

// The refusals above are the harness's own configuration until something pairs them with the stack
// this repository ships. Without the pair, deleting the line from docker-compose.yml opens public
// registration for every local run while the test above goes on passing — and its subject is
// invariant 3 of the identity note rather than a convenience of this file.
func TestTheStackThisRepositoryShipsDisablesSignupToo(t *testing.T) {
	running := cycle.ConfiguredWith(t, testsupport.DisableSignupVariable)
	if running != testsupport.SignupDisabled {
		t.Fatalf("the container runs with %s=%q, want %q — the refusals measured above are "+
			"about a provider nobody configured this way",
			testsupport.DisableSignupVariable, running, testsupport.SignupDisabled)
	}

	if given := deploymentSetting(t, testsupport.DisableSignupVariable); given != running {
		t.Fatalf("docker-compose.yml sets %s to %q while the harness proves the refusal at %q",
			testsupport.DisableSignupVariable, given, running)
	}
}

// Why claiming is not simply «invite again»: a permanent password can be set from the link's
// session, so an account handed to a new patient would hand them whoever held it before.
//
// The mutation this must fail against is a claim that clears the account's confirmation instead of
// deleting it — plausible, less destructive, and enough to make the retry answer 201 all the same.
// The control comes first for the reverse reason: both refusals below are also what a token that
// never worked answers.
func TestAClaimWithDeletionKillsTheSessionsOfTheAccountItTakesOver(t *testing.T) {
	walked := walkTheCycle(t)
	clinic := walked.clinic

	const address = "opened.then.claimed@clinic.example"

	// The route suite's interruption: a specialist who is not a doctor, refused inside the
	// transaction and therefore after the mail has gone out.
	const notADoctor = "8a1f3b7c-0000-4000-8000-0000000000ff"

	interrupted := clinic.send(t, patientsPath, walked.doctor.access,
		body(address, walked.doctor.account, notADoctor))
	if interrupted.status != http.StatusUnprocessableEntity {
		t.Fatalf("the interrupted creation answered %d, want 422: %s",
			interrupted.status, interrupted.body)
	}

	// From here somebody holds a session on an account the clinic is about to hand to someone else.
	opened := signIn(t, address)

	if status, said := whoIsSignedIn(t, opened.access); status != http.StatusOK {
		t.Fatalf("the session was already dead before the claim: %d %s", status, said)
	}

	// Exchanged once rather than merely held: the grant rotates the token, so the half the claim has
	// to take away is the one this control was answered with.
	status, live := exchangeRefreshToken(t, opened.refresh)
	if status != http.StatusOK || live == "" {
		t.Fatalf("the refresh grant answered %d before the claim, want a session: %s", status, live)
	}

	cured := clinic.send(t, patientsPath, walked.doctor.access,
		body(address, walked.doctor.account))
	if cured.status != http.StatusCreated {
		t.Fatalf("the retry answered %d, want 201: %s", cured.status, cured.body)
	}

	// Reported rather than fatal, so the two assertions below are measured on the same run: a claim
	// that took the account over without deleting it has to be shown to leave the session alive,
	// not merely the identifier.
	if claimed := accountID(t, address); claimed == opened.account {
		t.Error("the account after the claim is the one that was opened, so nothing was deleted")
	}

	if status, said := whoIsSignedIn(t, opened.access); status != http.StatusForbidden ||
		!strings.Contains(said, "user_not_found") {
		t.Errorf("the provider still answers for the previous session: %d %s", status, said)
	}

	if status, said := exchangeRefreshToken(t, live); status != http.StatusBadRequest ||
		!strings.Contains(said, "refresh_token_not_found") {
		t.Errorf("the refresh grant answered %d %s for the previous session, want it gone "+
			"with the account", status, said)
	}
}

// whoIsSignedIn asks the provider who a session token belongs to: a deleted account answers this
// differently from one that has not been deleted, which is the whole of the measurement.
func whoIsSignedIn(t *testing.T, access string) (int, string) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, cycle.GoTrue.URL+"/user", nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+access)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("asking the provider about a session: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	said, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the answer: %v", err)
	}

	return resp.StatusCode, string(said)
}

// exchangeRefreshToken spends a refresh token and returns the one that replaces it. The access
// token expires on its own within the hour; the refresh token outlives it and is what a deleted
// account takes with it, so it is the half worth measuring.
func exchangeRefreshToken(t *testing.T, refresh string) (int, string) {
	t.Helper()

	status, said := ask(t, "/token?grant_type=refresh_token", "",
		map[string]string{"refresh_token": refresh})

	if status != http.StatusOK {
		return status, said
	}

	var session struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal([]byte(said), &session); err != nil {
		t.Fatalf("decoding the session: %v", err)
	}

	return status, session.RefreshToken
}

func requireRefusal(t *testing.T, got answer, status int, says string) {
	t.Helper()

	if got.status != status {
		t.Fatalf("status = %d, want %d: %s", got.status, status, got.body)
	}

	if detail := problemFrom(t, got).Detail; !strings.Contains(detail, says) {
		t.Errorf("detail = %q, want it to say %q", detail, says)
	}
}

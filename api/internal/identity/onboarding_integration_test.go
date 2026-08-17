//go:build integration

// POST /v1/patients against the identity provider it invites through. Everything here runs on the cycle's database,
// the one GoTrue is connected to: the accounts these tests create and claim are the provider's own rows, and the
// profiles are written beside them by the endpoint under test.
package identity_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/identity"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/httpserver"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/testsupport"
)

// A double click is two requests for one address, and the answer to the second is the whole reason the lock exists:
// the loser must not invite, because a second invitation rotates the token and kills the link the winner just sent.
func TestADoubleClickCreatesOnePatientAndSendsOneInvitation(t *testing.T) {
	clinic := onboardingStand(t)

	const address = "double.click@clinic.example"

	var (
		wg      sync.WaitGroup
		answers = make([]answer, 2)
	)

	for i := range answers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			answers[i] = clinic.create(t, asDoctor, patientsPath, body(address, theDoctor))
		}()
	}

	wg.Wait()

	created, refused := 0, 0

	for _, got := range answers {
		switch got.status {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			refused++
		default:
			t.Fatalf("a request answered %d: %s", got.status, got.body)
		}
	}

	if created != 1 || refused != 1 {
		t.Fatalf("two simultaneous requests answered %d created and %d refused, want one of each",
			created, refused)
	}

	// The property the lock buys, which a serialised-but-inviting implementation would fail: the loser asked for
	// nothing.
	if sent := clinic.provider.invitations(); sent != 1 {
		t.Errorf("%d invitations went out for one address, want 1", sent)
	}

	if patients := clinic.countPatients(t, address); patients != 1 {
		t.Errorf("%d patients exist for one address, want 1", patients)
	}
}

// The window between the invitation and the commit is irreducible, and this is what makes it survivable: the record
// of the invitation is committed on its own, so a repeat of the request recognises its own account instead of
// finding somebody else's.
//
// The interruption is real rather than injected — the first attempt names a specialist who is not a doctor, refused
// inside the transaction after the mail has gone out. The retry happens AFTER the invitee has opened the link, the
// state the previous draft of this design was green in and broken in: without the deletion the new patient would
// inherit whatever password the previous session set.
func TestACreationInterruptedAfterTheInvitationIsCuredByARetry(t *testing.T) {
	clinic := onboardingStand(t)

	const address = "interrupted@clinic.example"

	stranger := "8a1f3b7c-0000-4000-8000-0000000000ff"

	if refused := clinic.create(t, asDoctor, patientsPath, body(address, theDoctor, stranger)); refused.status !=
		http.StatusUnprocessableEntity {
		t.Fatalf("the interrupted creation answered %d, want 422: %s", refused.status, refused.body)
	}

	// What the interruption left: an account, our record of it, and no patient.
	invited := accountID(t, address)

	if !clinic.holdsInviteFor(t, invited) {
		t.Fatal("the invitation was not recorded, so the retry below has nothing to recognise")
	}

	if patients := clinic.countPatients(t, address); patients != 0 {
		t.Fatalf("%d patients exist after a refused creation, want 0", patients)
	}

	// Which transaction each audit row belongs to, measured where the two come apart: one transaction carrying
	// both would have neither row here.
	if signed := clinic.auditRows(t, "invite.send", invited); signed != 1 {
		t.Errorf("%d invite.send rows survived the refused creation, want 1: a mail was asked "+
			"for and nothing in the log says so", signed)
	}

	if signed := clinic.auditRows(t, "patient.create", invited); signed != 0 {
		t.Errorf("%d patient.create rows survived the refused creation, want 0", signed)
	}

	// The invitee opens the link before the doctor tries again.
	if accepted, location := follow(t, inviteToken(t, address), "invite", ""); !accepted {
		t.Fatalf("the invitation link was refused: %s", location)
	}

	if cured := clinic.create(t, asDoctor, patientsPath, body(address, theDoctor)); cured.status != http.StatusCreated {
		t.Fatalf("the retry answered %d, want 201: %s", cured.status, cured.body)
	}

	if deleted := clinic.provider.deletions(); deleted != 1 {
		t.Errorf("the claim deleted %d accounts, want 1: an account somebody has been inside "+
			"can have a password set on it, and inviting over it hands that password to the "+
			"new patient", deleted)
	}

	// The deletion is signed: it destroys an account and every session on it, and a log showing a second
	// invitation with nothing removed to make room for it would be describing a different event.
	if signed := clinic.auditRows(t, "account.delete", invited); signed != 1 {
		t.Errorf("%d account.delete rows name the account that was removed, want 1", signed)
	}

	// And filed under the patient — the half of that column the staff route's own test measures from the other side.
	var underThePatient int
	clinic.scan(t, `
		SELECT count(*) FROM app.audit_log
		WHERE action = 'account.delete' AND entity_id = $1 AND patient_id = $1
	`, []any{invited}, &underThePatient)

	if underThePatient != 1 {
		t.Errorf("%d account.delete rows are filed under the patient, want 1", underThePatient)
	}

	// The account is a new one, which is what the deletion means: the person the clinic now has is not the
	// account that was opened.
	claimed := accountID(t, address)
	if claimed == invited {
		t.Error("the account claimed is the one that was opened, so nothing was deleted")
	}

	if patients := clinic.countPatients(t, address); patients != 1 {
		t.Errorf("%d patients exist after the cure, want 1", patients)
	}

	// And the person can get in: the link in the second mail works, which is the end of the cycle.
	if accepted, location := follow(t, inviteToken(t, address), "invite", ""); !accepted {
		t.Errorf("the link of the second invitation was refused: %s", location)
	}
}

// The invitation goes out before its answer comes back, so a call that fails on the way home leaves an account
// nobody here has recorded — and the next request would read it as somebody else's and refuse the address for good.
// The request is still refused; what must survive it is the record, so the doctor's retry recognises its own account.
func TestAnInvitationWhoseAnswerWasLostIsStillRecorded(t *testing.T) {
	clinic := onboardingStand(t)

	const address = "lost.answer@clinic.example"

	clinic.provider.loseTheAnswer = true

	refused := clinic.create(t, asDoctor, patientsPath, body(address, theDoctor))
	if refused.status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: %s", refused.status, refused.body)
	}

	// The account is there: the provider created it before the answer was lost.
	invited := accountID(t, address)

	if !clinic.holdsInviteFor(t, invited) {
		t.Fatal("the account exists at the provider and this clinic has no record of it — " +
			"every later request for this address answers 409 and nothing can undo it")
	}

	if patients := clinic.countPatients(t, address); patients != 0 {
		t.Fatalf("%d patients exist after a refused creation, want 0", patients)
	}

	clinic.provider.loseTheAnswer = false

	cured := clinic.create(t, asDoctor, patientsPath, body(address, theDoctor))
	if cured.status != http.StatusCreated {
		t.Fatalf("the retry answered %d, want 201: %s", cured.status, cured.body)
	}

	// Claimed rather than deleted and recreated: nobody had been inside, so the second invitation went to the
	// account already there.
	if claimed := accountID(t, address); claimed != invited {
		t.Errorf("the retry landed on account %s and the lost invitation was to %s", claimed, invited)
	}

	if deleted := clinic.provider.deletions(); deleted != 0 {
		t.Errorf("%d accounts were deleted to claim one nobody had opened, want 0", deleted)
	}
}

// The same cure for the request that is no longer there to receive it, which is the case it was written for: a call
// that fails because the provisioner was slow has usually spent the request's whole budget first.
//
// Named for the mutation it exists to fail against — context.WithoutCancel(ctx) replaced by ctx in invite's recovery.
// Under it the lookup and the write that record the account are refused before they leave the process, the account
// stays one this clinic has no record of, and every later request for that address answers 409 with nothing able to
// undo it. The test above runs on a live context, so both versions of that line pass it.
//
// Driven through InvitePatient rather than the transport: the context is the subject, and httptest gives a request one
// that outlives its caller.
func TestTheInvitationOfARequestThatGaveUpIsStillRecorded(t *testing.T) {
	clinic := onboardingStand(t)

	const address = "gave.up@clinic.example"

	ctx, cancel := context.WithCancel(auth.WithPrincipal(t.Context(), asDoctor))
	defer cancel()

	clinic.provider.loseTheAnswer = true
	clinic.provider.beforeTheAnswerIsLost = cancel

	_, err := identity.NewOnboarding(clinic.requests, clinic.writes, clinic.provider).
		InvitePatient(ctx, address, identity.NewPatient{
			FullName:    "Ирина Соколова",
			Specialists: []identity.Assignment{{ProviderID: theDoctor, CareRole: "endo", Primary: true}},
		})
	if !errors.Is(err, identity.ErrProvisionerUnavailable) {
		t.Fatalf("the abandoned request came back with %v, want %v", err, identity.ErrProvisionerUnavailable)
	}

	if !clinic.holdsInviteFor(t, accountID(t, address)) {
		t.Fatal("the caller was gone when the answer was lost, so the account the provider created was " +
			"never recorded as this clinic's — the address is refused from here on and nothing undoes it")
	}
}

// The same window on the other side of a successful invitation, on both routes: the mail has left, and the row that
// says whose account it is has not been written yet.
//
// Until this, only the failure of the provisioner's own call was cured. An invitation that succeeded and a write that
// then failed — the service pool exhausted, a five-second statement timeout, a connection dropped, a caller who gave
// up — left the same account nobody here recognises, and this time with nothing that tried again: the request
// answered 500 and the address was refused from then on.
//
// Driven through the flow rather than the transport, because what has to end mid-request is the context.
func TestAnInvitationIsRecordedEvenWhenTheRequestEndsRightAfterIt(t *testing.T) {
	tests := []struct {
		name   string
		caller auth.Principal
		invite func(*identity.Onboarding, context.Context, string) (string, error)
	}{
		{
			name:   "a patient",
			caller: asDoctor,
			invite: func(o *identity.Onboarding, ctx context.Context, address string) (string, error) {
				return o.InvitePatient(ctx, address, identity.NewPatient{
					FullName: "Ирина Соколова",
					Specialists: []identity.Assignment{
						{ProviderID: theDoctor, CareRole: "endo", Primary: true},
					},
				})
			},
		},
		{
			name:   "a member of staff",
			caller: asAdmin,
			invite: func(o *identity.Onboarding, ctx context.Context, address string) (string, error) {
				return o.InviteProvider(ctx, address, identity.NewProvider{
					Role: "doctor", FullName: "Олег Ким",
				})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clinic := onboardingStand(t)

			address := strings.ReplaceAll(tc.name, " ", "-") + "@clinic.example"

			ctx, cancel := context.WithCancel(auth.WithPrincipal(t.Context(), tc.caller))
			defer cancel()

			clinic.provider.afterTheInvitation = cancel

			onboarding := identity.NewOnboarding(clinic.requests, clinic.writes, clinic.provider)

			if _, err := tc.invite(onboarding, ctx, address); err == nil {
				t.Fatal("a request cancelled mid-creation answered as if the person had been created")
			}

			if !clinic.holdsInviteFor(t, accountID(t, address)) {
				t.Fatal("the invitation went out and the row recording it did not, so the account is " +
					"one this clinic does not recognise — every later request for the address is a " +
					"409 and no application path undoes it")
			}
		})
	}
}

// The other half of that cure, and the reason it is conditional: an account somebody has already been inside is not
// recorded as ours on the strength of a failed call. From here the two are the same picture — an account at the
// address, and no answer saying whose it is — and claiming deletes: a stranger's account, and every session on it,
// destroyed because one call to the provisioner timed out. The address stays refused instead, which the spec
// records as the cost of that safety.
func TestAnAccountSomebodyHasOpenedIsNotClaimedByAFailedInvitation(t *testing.T) {
	clinic := onboardingStand(t)

	const address = "opened.first@clinic.example"

	clinic.provider.loseTheAnswer = true
	clinic.provider.openedFirst = true

	if refused := clinic.create(t, asDoctor, patientsPath, body(address, theDoctor)); refused.status !=
		http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: %s", refused.status, refused.body)
	}

	if clinic.holdsInviteFor(t, accountID(t, address)) {
		t.Fatal("an account somebody had opened was recorded as this clinic's; the next request " +
			"would delete it and everything signed in on it")
	}

	// And the refusal that follows says so, rather than answering something a retry could work around.
	clinic.provider.loseTheAnswer = false
	clinic.provider.openedFirst = false

	again := clinic.create(t, asDoctor, patientsPath, body(address, theDoctor))
	if again.status != http.StatusConflict {
		t.Fatalf("the retry answered %d, want 409: %s", again.status, again.body)
	}
}

// Refused without touching the account: it is somebody else's, and claiming it would hand it to a patient.
func TestAnAccountThisClinicDidNotInviteIsRefused(t *testing.T) {
	clinic := onboardingStand(t)

	const address = "somebody.else@clinic.example"

	invite(t, address)

	refused := clinic.create(t, asDoctor, patientsPath, body(address, theDoctor))
	if refused.status != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", refused.status, refused.body)
	}

	if detail := problemFrom(t, refused).Detail; !strings.Contains(detail, "клиника не приглашала") {
		t.Errorf("detail = %q, which does not tell the doctor whose account it is", detail)
	}

	if sent := clinic.provider.invitations(); sent != 0 {
		t.Errorf("the refusal sent %d invitations to an account it does not own", sent)
	}
}

// The provider's filter is case-sensitive and the provider stores the address folded, so an unfolded lookup would
// answer "no such account" for an account that exists — and the invitation that followed would kill a live link.
func TestMixedCaseDoesNotCreateASecondPatient(t *testing.T) {
	clinic := onboardingStand(t)

	if created := clinic.create(t, asDoctor, patientsPath, body("Anna.Petrova@Clinic.Example", theDoctor)); created.status !=
		http.StatusCreated {
		t.Fatalf("the first creation answered %d: %s", created.status, created.body)
	}

	again := clinic.create(t, asDoctor, patientsPath, body("ANNA.PETROVA@clinic.example", theDoctor))
	if again.status != http.StatusConflict {
		t.Fatalf("the same address in another case answered %d, want 409: %s", again.status, again.body)
	}

	if detail := problemFrom(t, again).Detail; !strings.Contains(detail, "уже заведён") {
		t.Errorf("detail = %q, which is not the answer for an address already taken", detail)
	}

	if sent := clinic.provider.invitations(); sent != 1 {
		t.Errorf("%d invitations went out for one mailbox, want 1", sent)
	}

	if patients := clinic.countPatients(t, "anna.petrova@clinic.example"); patients != 1 {
		t.Errorf("%d patients exist for one mailbox, want 1", patients)
	}
}

// Who may be named on the care team, and by whom: the doctor's own refusal is in the fast tests, these write rows.
func TestTheCareTeamTheCreationWrites(t *testing.T) {
	tests := []struct {
		name      string
		caller    auth.Principal
		address   string
		providers []string
		want      int
	}{
		{
			name: "a doctor names themselves", caller: asDoctor,
			address: "one.specialist@clinic.example", providers: []string{theDoctor}, want: 1,
		},
		{
			name: "a doctor names themselves and a colleague", caller: asDoctor,
			address: "two.specialists@clinic.example", providers: []string{theDoctor, theColleague}, want: 2,
		},
		{
			// An admin is not a provider and can look after nobody, so requiring them on the team would
			// make the operation unusable for them.
			name: "an admin names a doctor and not themselves", caller: asAdmin,
			address: "assigned.by.admin@clinic.example", providers: []string{theColleague}, want: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clinic := onboardingStand(t)

			created := clinic.create(t, tc.caller, patientsPath, body(tc.address, tc.providers...))
			if created.status != http.StatusCreated {
				t.Fatalf("status = %d, want 201: %s", created.status, created.body)
			}

			var assignments int
			clinic.scan(t, `
				SELECT count(*) FROM app.care_team_assignments
				WHERE patient_id = (SELECT user_id FROM app.invites WHERE email = $1)
			`, []any{tc.address}, &assignments)

			if assignments != tc.want {
				t.Errorf("%d specialists were assigned, want %d", assignments, tc.want)
			}
		})
	}
}

// Signed by the doctor rather than by a job: the audit log's whole purpose here is that a person created this
// patient and it says which one. Which transaction each row belongs to is the interrupted creation's subject.
func TestBothRowsOfACreationAreSignedByTheDoctor(t *testing.T) {
	clinic := onboardingStand(t)

	const address = "audited@clinic.example"

	if created := clinic.create(t, asDoctor, patientsPath, body(address, theDoctor)); created.status != http.StatusCreated {
		t.Fatalf("status = %d: %s", created.status, created.body)
	}

	// The entity travels with the action: unasserted, both rows file under `invites` and a deletion reads as an
	// invite record having been removed.
	for action, entity := range map[string]string{
		"patient.create": "profiles",
		"invite.send":    "invites",
	} {
		var rows int
		clinic.scan(t, `
			SELECT count(*) FROM app.audit_log
			WHERE action = $1
			  AND entity = $4
			  AND actor_id = $2
			  AND actor_job IS NULL
			  AND patient_id = (SELECT user_id FROM app.invites WHERE email = $3)
		`, []any{action, theDoctor, address, entity}, &rows)

		if rows != 1 {
			t.Errorf("%d rows of %s name the doctor, want 1", rows, action)
		}
	}
}

// The service path accepts a profile without one: nobody has signed in, and the device reports its zone when
// somebody does.
func TestACreatedPatientHasNoTimezoneYet(t *testing.T) {
	clinic := onboardingStand(t)

	const address = "no.timezone@clinic.example"

	if created := clinic.create(t, asDoctor, patientsPath, body(address, theDoctor)); created.status != http.StatusCreated {
		t.Fatalf("status = %d: %s", created.status, created.body)
	}

	var missing bool
	clinic.scan(t, `
		SELECT timezone IS NULL FROM app.profiles
		WHERE user_id = (SELECT user_id FROM app.invites WHERE email = $1)
	`, []any{address}, &missing)

	if !missing {
		t.Error("the profile carries a timezone nobody reported; an empty string here is a " +
			"value no first sign-in would replace")
	}
}

// The most likely failure in production, and the one the doctor has to be able to act on: in Russian, telling them
// to repeat the request, under a type of its own rather than the internal-error one.
func TestAnUnreachableProvisionerIsAnsweredAsUnavailable(t *testing.T) {
	clinic := onboardingStand(t)
	clinic.provider.unreachable = true

	refused := clinic.create(t, asDoctor, patientsPath, body("unreachable@clinic.example", theDoctor))
	if refused.status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: %s", refused.status, refused.body)
	}

	problem := problemFrom(t, refused)

	if problem.Type != httpserver.ProblemUnavailable {
		t.Errorf("type = %q, want %q", problem.Type, httpserver.ProblemUnavailable)
	}

	// No more than the transport can know: a 503 claiming the mail had not gone out would be guessing, since the
	// invitation is sent before its answer comes back.
	if !strings.Contains(problem.Detail, "Повторите запрос") {
		t.Errorf("detail = %q, which does not tell the doctor to try again", problem.Detail)
	}
}

// The patient's identifier is never among them: it is the provider's to assign, and a test that chose one would be
// measuring a flow this endpoint does not have.
var (
	asDoctor = auth.Principal{Subject: theDoctor, Role: "doctor"}
	asAdmin  = auth.Principal{Subject: theAdmin, Role: "admin"}
)

// One test's world: the endpoint as the router mounts it, the provider behind it, and a connection to read rows.
type onboarding struct {
	provider *harnessProvisioner
	requests *pgxpool.Pool
	writes   *pgxpool.Pool
}

// onboardingStand brings up the endpoint against the cycle's database: the accounts and the profiles have to be in
// one place for the token issuance hook to resolve anything.
func onboardingStand(t *testing.T) *onboarding {
	t.Helper()

	clinic := freshClinic(t)
	seedStaff(t, clinic.writes)

	return clinic
}

// freshClinic is the same arrangement with nobody in it. The cycle suite starts from here: it creates its own staff
// through the routes, and a seeded doctor would be a person no request created.
func freshClinic(t *testing.T) *onboarding {
	t.Helper()

	cycle.Reset(t)

	requests, err := database.NewPool(t.Context(), cycle.DB.AppURL)
	if err != nil {
		t.Fatalf("opening the request pool: %v", err)
	}
	t.Cleanup(requests.Close)

	writes, err := database.NewServicePool(t.Context(), cycle.DB.ServiceAppURL)
	if err != nil {
		t.Fatalf("opening the service pool: %v", err)
	}
	t.Cleanup(writes.Close)

	return &onboarding{
		provider: &harnessProvisioner{token: cycle.AdminToken(t)},
		requests: requests,
		writes:   writes,
	}
}

// seedStaff writes the two doctors and the admin the tests name. The doctors go through the service seam, the only
// path that writes these tables at all; the admin is written by the superuser because migration 000006 makes the
// service path refuse role='admin' outright — the same exception the policy tests make.
func seedStaff(t *testing.T, writes *pgxpool.Pool) {
	t.Helper()

	if err := database.WithServiceJob(
		t.Context(), writes, provisioningJob,
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO app.profiles (user_id, role, full_name, timezone)
				VALUES ($1, 'doctor', 'Марина Крылова', 'Europe/Moscow'),
				       ($2, 'doctor', 'Олег Ким', 'Europe/Moscow')
			`, theDoctor, theColleague)

			return err
		},
	); err != nil {
		t.Fatalf("seeding the doctors: %v", err)
	}

	superuser := testsupport.Connect(t, cycle.DB.SuperuserURL)
	if _, err := superuser.Exec(t.Context(), `
		INSERT INTO app.profiles (user_id, role, full_name, timezone)
		VALUES ($1, 'admin', 'Пётр Аверин', 'Europe/Moscow')
	`, theAdmin); err != nil {
		t.Fatalf("seeding the admin: %v", err)
	}
}

// create drives one request through the transport, the way a dashboard would.
func (c *onboarding) create(t *testing.T, caller auth.Principal, path, payload string) answer {
	t.Helper()

	mux := chi.NewRouter()
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), caller)))
		})
	})

	identity.NewService(identity.Deps{Onboarding: identity.NewOnboarding(c.requests, c.writes, c.provider)}).
		Register(httpserver.NewAPI(mux))

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return answer{status: rec.Code, body: rec.Body.String()}
}

// countPatients counts through the record of the invitation, the only place in the app schema an address is written.
func (c *onboarding) countPatients(t *testing.T, address string) int {
	t.Helper()

	var patients int
	c.scan(t, `
		SELECT count(*) FROM app.profiles
		WHERE user_id IN (SELECT user_id FROM app.invites WHERE email = $1)
	`, []any{address}, &patients)

	return patients
}

// auditRows counts rows of one action against one entity id, with the entity column part of the question: without
// it the two actions this flow signs could file under the same entity.
func (c *onboarding) auditRows(t *testing.T, action, entityID string) int {
	t.Helper()

	// Spelled out here rather than asked of the package, so that a pair changed on one side is a failure and not a
	// silent agreement. It also has to be right: asking for patient.create under `invites` counts zero rows
	// whatever the flow wrote, which is what «no patient.create row survived» used to measure.
	entity, known := map[string]string{
		"invite.send":     "invites",
		"account.delete":  "auth.account",
		"patient.create":  "profiles",
		"provider.create": "profiles",
	}[action]
	if !known {
		t.Fatalf("no entity is written down here for %q, so this counts rows of nothing", action)
	}

	var rows int
	c.scan(t, `
		SELECT count(*) FROM app.audit_log
		WHERE action = $1 AND entity_id = $2 AND entity = $3
	`, []any{action, entityID, entity}, &rows)

	return rows
}

func (c *onboarding) holdsInviteFor(t *testing.T, userID string) bool {
	t.Helper()

	var held bool
	c.scan(t, `SELECT EXISTS (SELECT FROM app.invites WHERE user_id = $1)`, []any{userID}, &held)

	return held
}

// scan reads through the superuser, like the rest of this suite: the service role holds INSERT on audit_log and no
// SELECT, and granting a test what it needs to see its own result would change the arrangement it is checking.
func (c *onboarding) scan(t *testing.T, statement string, args []any, into ...any) {
	t.Helper()

	conn := testsupport.Connect(t, cycle.DB.SuperuserURL)

	if err := conn.QueryRow(t.Context(), statement, args...).Scan(into...); err != nil {
		t.Fatalf("reading what was written: %v", err)
	}
}

// body is the request a dashboard sends: an address, a name and a care team.
func body(address string, providers ...string) string {
	specialists := make([]string, 0, len(providers))
	for i, provider := range providers {
		specialists = append(specialists, fmt.Sprintf(
			`{"provider_id":%q,"care_role":"endo","primary":%t}`, provider, i == 0,
		))
	}

	return fmt.Sprintf(`{"email":%q,"full_name":"Ирина Соколова","specialists":[%s]}`,
		address, strings.Join(specialists, ","))
}

func problemFrom(t *testing.T, got answer) httpserver.Problem {
	t.Helper()

	var problem httpserver.Problem
	if err := json.Unmarshal([]byte(got.body), &problem); err != nil {
		t.Fatalf("decoding the problem document: %v", err)
	}

	return problem
}

// accountID is what the provider says the account at an address is, read off its own table.
func accountID(t *testing.T, address string) string {
	t.Helper()

	var id string
	scanProvider(t, `SELECT id::text FROM auth.users WHERE email = $1`, []any{address}, &id)

	return id
}

// harnessProvisioner is identity.Provisioner against the container the cycle runs, standing in for the component
// that holds the admin key. A stand-in for one reason worth naming: cmd/provisioner is a package main and cannot be
// imported, so these tests exercise this context's half of the boundary — the order of the calls and what it does
// with the answers. That the component itself speaks this protocol is its own integration suite's subject.
// The mutex covers the counters and nothing else. Every field below it is set before the request that reads it and
// not touched again while one is in flight — the double-click test is the only one with two requests at once and it
// changes none of them — so the contract is the ordering rather than the lock. A test that flips one mid-request is
// what -race would then report.
type harnessProvisioner struct {
	token string

	mu      sync.Mutex
	invited int
	deleted int

	// unreachable makes every call fail, which is the state the 503 exists for.
	unreachable bool

	// Invites and then reports a failure, which is what a timeout looks like from this side: the account exists
	// and the caller was told nothing.
	loseTheAnswer bool

	// Confirms the account before the answer is lost, arranging the one state the cure must refuse: an account
	// somebody has been inside, indistinguishable from a stranger's.
	openedFirst bool

	// Runs after the account exists and before the failure is reported. One caller, and it ends the request's
	// context there: the caller who gave up while the invitation was in flight.
	beforeTheAnswerIsLost func()

	// Runs after an invitation this side was told about. The window it opens is the one the creation cannot
	// close: the mail has left and nothing here has recorded it yet.
	afterTheInvitation func()
}

// The client wraps a transport failure and the caller may not read it, so any error at all is the whole signal.
var errUnreachable = errors.New("the provisioner is not answering")

func (p *harnessProvisioner) Invite(ctx context.Context, email string) (identity.Account, error) {
	if p.unreachable {
		return identity.Account{}, errUnreachable
	}

	p.mu.Lock()
	p.invited++
	p.mu.Unlock()

	status, said := p.call(ctx, http.MethodPost, "/invite", map[string]string{"email": email})
	if status != http.StatusOK {
		return identity.Account{}, fmt.Errorf("the provider answered %d: %s", status, said)
	}

	// Read back rather than decoded from the answer: the invitation's own answer then cannot disagree with the
	// row a later lookup will find.
	account, err := p.read(ctx, email)
	if err != nil {
		return identity.Account{}, err
	}

	if account == nil {
		return identity.Account{}, fmt.Errorf("the provider invited %s and holds no account for it", email)
	}

	if p.loseTheAnswer {
		if p.openedFirst {
			if err := p.confirm(ctx, email); err != nil {
				return identity.Account{}, err
			}
		}

		if p.beforeTheAnswerIsLost != nil {
			p.beforeTheAnswerIsLost()
		}

		return identity.Account{}, errUnreachable
	}

	if p.afterTheInvitation != nil {
		p.afterTheInvitation()
	}

	return *account, nil
}

// confirm marks the account as one somebody has opened, by the same surgery on the provider's own table the rest of
// this suite uses to age a link.
func (p *harnessProvisioner) confirm(ctx context.Context, email string) error {
	conn, err := pgx.Connect(ctx, cycle.DB.SuperuserURL)
	if err != nil {
		return fmt.Errorf("connecting to the provider's own database: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	// email_confirmed_at rather than confirmed_at: measured against this image on 2026-08-16, confirmed_at is
	// GENERATED ALWAYS AS LEAST(email_confirmed_at, phone_confirmed_at), so an UPDATE of it is refused.
	tag, err := conn.Exec(ctx,
		`UPDATE auth.users SET email_confirmed_at = pg_catalog.now() WHERE email = $1`, email)
	if err != nil {
		return fmt.Errorf("confirming %s: %w", email, err)
	}

	if tag.RowsAffected() != 1 {
		return fmt.Errorf("confirming %s touched %d rows", email, tag.RowsAffected())
	}

	return nil
}

func (p *harnessProvisioner) Lookup(ctx context.Context, email string) (*identity.Account, error) {
	if p.unreachable {
		return nil, errUnreachable
	}

	return p.read(ctx, email)
}

// read is the lookup the component answers from its admin listing, taken off the table that listing reads. Nothing
// is inferred from the address: the match is exact, on the folded spelling, which is the property this side's fold
// exists to satisfy.
func (p *harnessProvisioner) read(ctx context.Context, email string) (*identity.Account, error) {
	conn, err := pgx.Connect(ctx, cycle.DB.SuperuserURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to the provider's own database: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	var found identity.Account

	err = conn.QueryRow(ctx, `
		SELECT id::text, confirmed_at, last_sign_in_at FROM auth.users WHERE email = $1
	`, email).Scan(&found.ID, &found.ConfirmedAt, &found.LastSignInAt)

	// No account is an answer rather than a failure: it is what makes an address free to invite.
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("looking up %s: %w", email, err)
	}

	return &found, nil
}

func (p *harnessProvisioner) Delete(ctx context.Context, deletion identity.Deletion) error {
	if p.unreachable {
		return errUnreachable
	}

	// The component refuses a deletion whose proof does not say the account is claimable, and so does this: a
	// caller that got the proof wrong must fail here rather than take the account with it.
	if !deletion.InviteExists || deletion.ProfileExists {
		return fmt.Errorf("only an account with an invite record and no profile may be deleted, "+
			"asked with invite=%t profile=%t", deletion.InviteExists, deletion.ProfileExists)
	}

	p.mu.Lock()
	p.deleted++
	p.mu.Unlock()

	status, said := p.call(ctx, http.MethodDelete, "/admin/users/"+deletion.ID, nil)
	if status != http.StatusOK && status != http.StatusNoContent {
		return fmt.Errorf("the provider answered %d to a deletion: %s", status, said)
	}

	return nil
}

func (p *harnessProvisioner) invitations() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.invited
}

func (p *harnessProvisioner) deletions() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.deleted
}

func (p *harnessProvisioner) call(
	ctx context.Context, method, path string, payload map[string]string,
) (int, string) {
	var reader io.Reader

	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return 0, err.Error()
		}

		reader = strings.NewReader(string(encoded))
	}

	req, err := http.NewRequestWithContext(ctx, method, cycle.GoTrue.URL+path, reader)
	if err != nil {
		return 0, err.Error()
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()

	said, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err.Error()
	}

	return resp.StatusCode, string(said)
}

// A specialist named twice is the form's mistake, and it has to be refused before anything leaves the process.
//
// Left to the database it arrives as a UNIQUE violation on the care team, which was answered 409 «этот адрес уже
// заведён» — measured on the stand. By then the invitation had gone out and the invites row was committed, so the
// invitee held a link to an account with no profile while the doctor, told the patient already existed, had no
// reason to correct the form and retry.
func TestNamingOneSpecialistTwiceIsTheFormsMistakeAndNothingIsSent(t *testing.T) {
	clinic := onboardingStand(t)

	const address = "named-twice@clinic.example"

	answered := clinic.create(t, asDoctor, patientsPath, body(address, theDoctor, theDoctor))

	if answered.status != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422: a repeated specialist is the form's problem, "+
			"not a fact about the address; body was %s", answered.status, answered.body)
	}

	// Nothing left the process, which is what makes the 422 honest: a retry with the form corrected is a first
	// attempt, not a second.
	var invited int
	clinic.scan(t, `SELECT count(*) FROM app.invites WHERE email = $1`,
		[]any{address}, &invited)

	if invited != 0 {
		t.Errorf("%d invites rows for a refused body: the address is now spent", invited)
	}

	if sent := clinic.provider.invitations(); sent != 0 {
		t.Errorf("%d invitations went out for a body that was refused", sent)
	}
}

// Two leading specialists, and the same property: refused before the address is spent.
//
// The witness for InvitePatient's own call to checkBody, which had none. `primary` is a bare bool with no
// cross-field constraint, so huma's schema passes a body naming two — this is the one refusal in checkBody the
// transport cannot catch first. Delete that call and the suite stayed green: the invitation goes out, the invites
// row commits, and the 422 arrives from the check inside CreatePatient with the address already spent.
func TestNamingTwoLeadingSpecialistsIsTheFormsMistakeAndNothingIsSent(t *testing.T) {
	clinic := onboardingStand(t)

	const address = "two-leaders@clinic.example"

	answered := clinic.create(t, asDoctor, patientsPath, fmt.Sprintf(
		`{"email":%q,"full_name":"Ирина Соколова","specialists":[`+
			`{"provider_id":%q,"care_role":"endo","primary":true},`+
			`{"provider_id":%q,"care_role":"nurse","primary":true}]}`,
		address, theDoctor, theColleague,
	))

	if answered.status != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422: which specialist leads is the form's problem; body was %s",
			answered.status, answered.body)
	}

	var invited int
	clinic.scan(t, `SELECT count(*) FROM app.invites WHERE email = $1`, []any{address}, &invited)

	if invited != 0 {
		t.Errorf("%d invites rows for a refused body: the address is now spent", invited)
	}

	if sent := clinic.provider.invitations(); sent != 0 {
		t.Errorf("%d invitations went out for a body that was refused", sent)
	}
}

// A staff creation interrupted between its two commits leaves an address with an invite record and no profile, and
// nothing in that state says the invitation was for a doctor. ClaimFor reads the same three facts it reads for a
// patient — invite yes, profile no, account unopened — and answers ClaimByInviting, so an ordinary doctor's
// POST /v1/patients finishes an admin's staff onboarding as a patient of its own.
//
// The state is produced by removing the profile rather than by killing the process: the two commits are ordered
// deliberately (see rememberOrRetry) and the interruptions that land between them — a crash, a deploy, a lost
// connection — have no seam a request can drive. What the address holds afterwards is identical either way, and what
// is under test is what ClaimFor makes of it.
//
// Expected to fail until app.invites records the invited role. Written first, so that the fix is measured rather
// than asserted: without it this test cannot tell «the window is closed» from «the test never reached the window».
func TestAStaffCreationInterruptedBeforeItsProfileIsNotClaimedAsAPatient(t *testing.T) {
	clinic := onboardingStand(t)

	const address = "interrupted.staff@clinic.example"

	if hired := clinic.create(t, asAdmin, providersPath, providerBody(address, "эндокринолог")); hired.status !=
		http.StatusCreated {
		t.Fatalf("the staff creation answered %d, want 201: %s", hired.status, hired.body)
	}

	invited := accountID(t, address)

	// The interruption, after the invite record and before the person.
	superuser := testsupport.Connect(t, cycle.DB.SuperuserURL)
	if _, err := superuser.Exec(t.Context(),
		`DELETE FROM app.profiles WHERE user_id = $1`, invited); err != nil {
		t.Fatalf("removing the profile to reproduce the window: %v", err)
	}

	if !clinic.holdsInviteFor(t, invited) {
		t.Fatal("the invite record went with the profile, so this is not the window under test")
	}

	// A doctor who knows the pending address tries to create a patient on it.
	stolen := clinic.create(t, asDoctor, patientsPath, body(address, theDoctor))

	// The status, not merely «not 201»: the refusal has a sentence of its own, and a fall through to the default
	// 500 would satisfy an inequality while telling the doctor nothing.
	if stolen.status != http.StatusConflict {
		t.Errorf("the claim of a half-created member of staff answered %d, want 409: %s", stolen.status, stolen.body)
	}

	if patients := clinic.countPatients(t, address); patients != 0 {
		t.Errorf("%d patients exist on a staff address, want 0", patients)
	}

	// The admin's retry must still work, or the fix has traded this defect for the permanent refusal the
	// invite-first ordering exists to prevent.
	if cured := clinic.create(t, asAdmin, providersPath, providerBody(address, "эндокринолог")); cured.status !=
		http.StatusCreated {
		t.Errorf("the admin's retry answered %d, want 201: %s — the address is now unusable, which is worse "+
			"than the defect this test is about", cured.status, cured.body)
	}
}

package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

// One per state of the address at the provider: a doctor must know whether to retry, change address, or call somebody.
var (
	// Reached without asking the provider: a second invitation rotates the token and kills the link the winner sent.
	ErrAlreadyOnboarded = errors.New("this address already belongs to a patient")

	// Not a half-finished onboarding of ours: claiming it would hand somebody else's account to a new patient.
	ErrAccountIsNotOurs = errors.New("the address has an account this clinic did not invite")

	// A refusal too: the component's fixed 502 body makes "already taken" and "down" indistinguishable — see settle.
	ErrProvisionerUnavailable = errors.New("the identity provider could not be reached")

	ErrNoAddress = errors.New("an invitation needs an address")

	// They could not then see the patient they created: no screen shows a doctor a patient they are not assigned to.
	ErrCallerNotOnTheCareTeam = errors.New("a doctor must be on the care team of a patient they create")

	// The token's role is a patient's, or none at all.
	ErrCallerMayNotCreatePatients = errors.New("this caller may not create patients")

	// A doctor who could would be growing the clinic's staff, and every doctor's access runs through the care team.
	ErrCallerMayNotCreateProviders = errors.New("only an administrator may create providers")
)

// forPatient is carried for one column: patient_id is what a patient's audit trail is read by, so a doctor's
// identifier in it would file the clinic's own hiring inside the trail of whichever patient holds that id.
type creation struct {
	address    string
	invitedBy  string
	forPatient bool
}

// The audit row's patient_id: nothing when the person is staff.
func (c creation) patient(userID string) *string {
	if !c.forPatient {
		return nil
	}

	return &userID
}

// Held is what this clinic's own tables say about an account the provider knows: an invite record is what makes it
// ours to claim, and a profile is what makes the creation already done.
type Held struct {
	Invite  bool
	Profile bool
}

// Claim is what may be done with an address the provider already has an account for.
type Claim int

const (
	// The account is ours, nobody has opened the link, and a fresh invitation replaces it.
	ClaimByInviting Claim = iota

	// The account has been opened, and a session from that link can set a permanent password: inviting over it
	// would hand the new patient the previous person's credential.
	ClaimByDeletingFirst

	// RefuseAlreadyOnboarded — a profile exists, so the creation is done.
	RefuseAlreadyOnboarded

	// RefuseNotOurs — no invite record, so this is somebody else's account.
	RefuseNotOurs
)

// The order of the arms is the rule: a profile settles it before anything else is asked, which stops the loser of a
// double click from sending a second invitation; then the invite record, the only evidence that an account is ours;
// and only then the account's own state, where confirmed and signed-in both mean somebody has been inside.
func ClaimFor(account Account, held Held) Claim {
	switch {
	case held.Profile:
		return RefuseAlreadyOnboarded
	case !held.Invite:
		return RefuseNotOurs
	case hasBeenOpened(account):
		return ClaimByDeletingFirst
	default:
		return ClaimByInviting
	}
}

// Onboarding creates patients and staff by invitation.
//
// Two pools because the lock and the writes must not compete for one budget: a request holds its lock connection for
// as long as the provider takes to answer, and a second connection from that pool waits on another lock holder.
type Onboarding struct {
	// The request path's pool, used for the advisory lock alone: no statement of this context reads or writes here.
	requests *pgxpool.Pool

	// The service path's pool: no policy lets a request create a person, so the authorization for it is in Go, above.
	writes *pgxpool.Pool

	provisioner Provisioner
}

func NewOnboarding(requests, writes *pgxpool.Pool, provisioner Provisioner) *Onboarding {
	return &Onboarding{requests: requests, writes: writes, provisioner: provisioner}
}

// InvitePatient invites an address, writes the patient, and answers with the identifier the provider assigned.
//
// patient.UserID is not read: an identifier generated here would leave the token issuance hook resolving no profile
// at the moment the person signs in.
//
// The lock comes first, on the folded address, and everything the provider is asked happens under it and outside a
// transaction — an external call between two statements of a transaction made a retry destructive.
func (o *Onboarding) InvitePatient(ctx context.Context, email string, patient NewPatient) (string, error) {
	address := NormalizeAddress(email)
	if address == "" {
		return "", ErrNoAddress
	}

	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		// Refused here so that a request the authentication middleware does not cover invites nobody.
		return "", ErrCallerMayNotCreatePatients
	}

	if err := requireCallerMayCreate(principal, patient.Specialists); err != nil {
		return "", err
	}

	// Before the lock and before anything leaves the process. CreatePatient checks again later, but by then the
	// invitation has gone out and a malformed body has spent the address: the invitee holds a link to an account
	// that never gets a profile. Only the body's half — the identifier is not knowable yet.
	if err := checkBody(patient); err != nil {
		return "", err
	}

	if err := refuseRepeatedSpecialist(patient); err != nil {
		return "", err
	}

	of := creation{address: address, invitedBy: principal.Subject, forPatient: true}

	var userID string

	err := database.WithAdvisoryLock(ctx, o.requests, database.OnboardingLock, address,
		func(ctx context.Context) error {
			account, err := o.settle(ctx, of)
			if err != nil {
				return err
			}

			patient.UserID = account.ID
			userID = account.ID

			return o.record(ctx, of, patient)
		})
	if err != nil {
		return "", err
	}

	return userID, nil
}

// InviteProvider invites an address, writes the member of staff, and answers with the identifier the provider
// assigned. The same spine as InvitePatient, with three differences: only an administrator may ask, the person
// written is a doctor, and the rows this flow signs name no patient.
func (o *Onboarding) InviteProvider(
	ctx context.Context, email string, provider NewProvider,
) (string, error) {
	address := NormalizeAddress(email)
	if address == "" {
		return "", ErrNoAddress
	}

	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return "", ErrCallerMayNotCreateProviders
	}

	if principal.Role != "admin" {
		return "", fmt.Errorf("the token's role is %q: %w", principal.Role, ErrCallerMayNotCreateProviders)
	}

	of := creation{address: address, invitedBy: principal.Subject}

	var userID string

	err := database.WithAdvisoryLock(ctx, o.requests, database.OnboardingLock, address,
		func(ctx context.Context) error {
			account, err := o.settle(ctx, of)
			if err != nil {
				return err
			}

			provider.UserID = account.ID
			userID = account.ID

			// Committed before the person, for the reason record states: a creation interrupted after the
			// mail has gone out has to be curable by a retry.
			if err := o.rememberOrRetry(ctx, account.ID, of); err != nil {
				return err
			}

			return CreateProvider(ctx, o.writes, provider)
		})
	if err != nil {
		return "", err
	}

	return userID, nil
}

// settle decides between inviting, claiming and refusing, and comes back with the account the person will be.
//
// Every failure of the provisioner arrives as ErrProvisionerUnavailable, including a refusal: measured on
// 2026-08-16, cmd/provisioner answers 502 with a fixed body for everything the identity provider refuses
// (routes.go's failed and refuse), so this side cannot tell "the address is already taken" from "the provider is
// down". The cost is one wrong status on a race this lock closes, and the retry answers 409 through the lookup.
func (o *Onboarding) settle(ctx context.Context, of creation) (Account, error) {
	found, err := o.provisioner.Lookup(ctx, of.address)
	if err != nil {
		return Account{}, fmt.Errorf("looking up the address: %w", ErrProvisionerUnavailable)
	}

	if found == nil {
		return o.invite(ctx, of)
	}

	held, err := o.held(ctx, found.ID)
	if err != nil {
		return Account{}, err
	}

	claim := ClaimFor(*found, held)

	switch claim {
	case RefuseAlreadyOnboarded:
		return Account{}, ErrAlreadyOnboarded

	case RefuseNotOurs:
		return Account{}, ErrAccountIsNotOurs

	case ClaimByDeletingFirst:
		// The proof travels as it was measured rather than as two literals: the component cannot see the app
		// schema, and refuses a deletion whose proof does not say an invite record exists and a profile does not.
		if err := o.provisioner.Delete(ctx, Deletion{
			ID: found.ID, InviteExists: held.Invite, ProfileExists: held.Profile,
		}); err != nil {
			return Account{}, fmt.Errorf("deleting the account being claimed: %w", ErrProvisionerUnavailable)
		}

		// In a transaction of its own, before the invitation that follows, and on a clock of its own: this is
		// the most destructive thing the clinic does to another system, the account is already gone by the
		// time this runs, and a caller who hangs up in that instant must not take the record of it with them.
		auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recoveryBudget)
		defer cancel()

		if err := o.audit(auditCtx, accountDeleted, found.ID, of); err != nil {
			return Account{}, err
		}

		return o.invite(ctx, of)

	case ClaimByInviting:
		return o.invite(ctx, of)

	default:
		// Named rather than folded into the arm above: inviting was the default, so a fifth claim added to the
		// rule and not to this switch would have sent an invitation to whatever state it describes.
		return Account{}, fmt.Errorf("the claim rule answered %d, which this flow does not know", claim)
	}
}

// invite asks for the invitation, and cures the failure that would otherwise burn the address.
//
// The account is created and the mail sent before the answer reaches this side, so a call that fails on the way home
// — a timeout against the bound in provisioning.callTimeout, a lost connection — leaves an account this clinic has
// no record of, which the next request reads as somebody else's and refuses permanently.
//
// So the address is looked up again while the lock still holds, and an account nobody has been inside is recorded as
// ours: untouched is exactly what a lost answer leaves behind, and a stranger's account is never claimed on a
// failure.
//
// The request is still refused. Whether the mail left is not knowable from here, so a 201 would promise a link that
// may not be coming; the retry finds the record and invites again, and that invitation is one this side saw succeed.
func (o *Onboarding) invite(ctx context.Context, of creation) (Account, error) {
	account, err := o.provisioner.Invite(ctx, of.address)
	if err == nil {
		return account, nil
	}

	// Detached from the request, and that is the whole point of this arm: an Invite that failed because the
	// provisioner was slow has already eaten the request's own 20s, so an inherited deadline leaves the recovery
	// nothing — DeadlineExceeded, no invites row, and the permanent 409 this arm exists to prevent. Same move
	// lock.go makes for its release.
	recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recoveryBudget)
	defer cancel()

	orphan, lookupErr := o.provisioner.Lookup(recoveryCtx, of.address)
	if lookupErr != nil || orphan == nil || hasBeenOpened(*orphan) {
		return Account{}, fmt.Errorf("inviting the address: %w", ErrProvisionerUnavailable)
	}

	if err := o.remember(recoveryCtx, orphan.ID, of); err != nil {
		// The address is burned if this write is lost and there is nothing left to try, so it travels inside
		// the refusal, where the log keeps it.
		return Account{}, fmt.Errorf("recording an invitation whose answer was lost: %w: %w",
			err, ErrProvisionerUnavailable)
	}

	return Account{}, fmt.Errorf("inviting the address: %w", ErrProvisionerUnavailable)
}

// One provisioner call plus one short transaction, on a clock of their own: long enough for both against a provisioner
// that has just been slow.
//
// It is spent past the request's own deadline, which is the point — and the cost, since the handler goes on holding
// the lock and its connection for that long after the caller has been answered. Fifteen seconds sits above the service
// path's 5s statement timeout, so a write that is going to succeed has room and one that is not gives the connection
// back.
//
// The claim path is the one that can spend two of them: the deletion's own record, then invite's recovery. Both only
// when the database or the provisioner is already hanging, and thirty seconds is the ceiling.
const recoveryBudget = 15 * time.Second

// «Somebody has been inside this account» — one predicate because it guards two things: whether an orphan may be
// adopted, and whether an account may be deleted to make way for a new patient. Written twice it was measured once:
// dropping the LastSignInAt half left both suites green, on the copy that guards the destructive direction.
func hasBeenOpened(account Account) bool {
	return account.ConfirmedAt != nil || account.LastSignInAt != nil
}

// held reads what this clinic already has for an account, on the service path because that is where the answer is
// readable at all: a doctor holds INSERT on invites and no SELECT, so the request path cannot ask this at all.
func (o *Onboarding) held(ctx context.Context, userID string) (Held, error) {
	if !database.IsUUIDShaped(userID) {
		return Held{}, fmt.Errorf("the provider named account %q: %w", userID, ErrMalformedIdentifier)
	}

	var held Held

	err := database.WithService(ctx, o.writes, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT FROM app.invites   WHERE user_id = $1),
			       EXISTS (SELECT FROM app.profiles  WHERE user_id = $1)
		`, userID).Scan(&held.Invite, &held.Profile)
	})
	if err != nil {
		return Held{}, fmt.Errorf("reading what this clinic holds for %s: %w", userID, err)
	}

	return held, nil
}

// record writes the two transactions of a creation, in the order that makes an interrupted one curable.
//
// The invitation is recorded first and on its own, which departs from the spec's diagram: inside the one
// transaction, a creation that failed after the mail went out would leave an account the next request cannot
// recognise as its own, and would answer 409 forever. The window does not close — the commit always comes after the
// side effect — it shrinks to the gap between the invitation and this first commit.
func (o *Onboarding) record(ctx context.Context, of creation, patient NewPatient) error {
	if err := o.rememberOrRetry(ctx, patient.UserID, of); err != nil {
		return err
	}

	return CreatePatient(ctx, o.writes, patient)
}

// rememberOrRetry writes the record of an invitation that has already gone out, and tries once more on a clock of its
// own when the request's has run out.
//
// Without that row the address is an account nobody on this side recognises: held reports no invite, ClaimFor answers
// RefuseNotOurs, and every later request for the address is refused permanently — no role holds DELETE on invites, and
// nothing deletes the account either. The invitation cannot be unsent, so the write that records it does not depend on
// a caller who may already be gone. Same move invite makes for the answer it lost, and for the same reason.
//
// The retry can duplicate the audit row, in the one case where the first attempt committed and only its answer was
// lost. One extra «asked for» costs less than an address the clinic can never use again.
func (o *Onboarding) rememberOrRetry(ctx context.Context, userID string, of creation) error {
	err := o.remember(ctx, userID, of)
	if err == nil {
		return nil
	}

	retryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recoveryBudget)
	defer cancel()

	if again := o.remember(retryCtx, userID, of); again != nil {
		// Both, because they are rarely the same refusal: the first is usually the request giving up, and the
		// second is what the database actually says.
		return fmt.Errorf("%w (retried: %w)", err, again)
	}

	return nil
}

// remember commits the record of the invitation and the row that signs it.
func (o *Onboarding) remember(ctx context.Context, userID string, of creation) error {
	err := database.WithService(ctx, o.writes, func(ctx context.Context, tx pgx.Tx) error {
		// DO NOTHING rather than an update: no role holds UPDATE on this table, and the row says an account was
		// invited by this clinic, which a second invitation to it does not change.
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.invites (user_id, email, invited_by)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_id) DO NOTHING
		`, userID, of.address, of.invitedBy); err != nil {
			return fmt.Errorf("writing the invite record: %w", err)
		}

		// One row per invitation asked for. Asked for rather than sent: delivery is the provider's and is not
		// reported back, and a request whose answer was lost records the ask it certainly made.
		return writeAudit(ctx, tx, invitationAsked, userID, of.patient(userID))
	})
	if err != nil {
		return fmt.Errorf("recording the invitation to %s: %w", userID, classify(err))
	}

	return nil
}

// audit records one act of this flow that stands outside the creation's own transactions.
func (o *Onboarding) audit(ctx context.Context, action, entityID string, of creation) error {
	err := database.WithService(ctx, o.writes, func(ctx context.Context, tx pgx.Tx) error {
		return writeAudit(ctx, tx, action, entityID, of.patient(entityID))
	})
	if err != nil {
		return fmt.Errorf("recording %s for %s: %w", action, entityID, err)
	}

	return nil
}

// Both columns are free text with no CHECK and no registry, so the vocabulary is settled by whoever writes it first:
// invite.send is the spec's, account.delete is this step's and names the one act performed against the identity
// provider rather than against our own schema. The entities differ for the same reason — a deletion filed under
// `invites` would read as an invite record having been removed, which deviation 10 says never happens.
const (
	invitationAsked = "invite.send"
	accountDeleted  = "account.delete"
	patientCreated  = "patient.create"
	providerCreated = "provider.create"

	invitesEntity  = "invites"
	accountEntity  = "auth.account"
	profilesEntity = "profiles"
)

// The action and the entity are one fact, and this is where it is held: an action absent from here writes no row at
// all, so a fifth one cannot arrive as a pair spelled out at its own call site — which is how the two columns drifted
// while a comment claimed they could not.
var auditEntities = map[string]string{
	invitationAsked: invitesEntity,
	accountDeleted:  accountEntity,
	patientCreated:  profilesEntity,
	providerCreated: profilesEntity,
}

// writeAudit signs one act inside the caller's transaction. Every audit row this context writes goes through here.
//
// patientID is nil for anything that concerns no patient: a doctor's identifier in that column reads as an action
// inside the trail of whichever patient holds that id.
func writeAudit(ctx context.Context, tx pgx.Tx, action, entityID string, patientID *string) error {
	entity, known := auditEntities[action]
	if !known {
		return fmt.Errorf("no entity is registered for audit action %q", action)
	}

	// The setting names travel as bound parameters — current_setting takes its name as an argument — which keeps
	// the statement the constant the authorship gate requires.
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.audit_log (actor_id, actor_job, action, entity, entity_id, patient_id)
		VALUES (
			nullif(current_setting($3, true), '')::uuid,
			nullif(current_setting($4, true), ''),
			$2, $5, $1, $6
		)
	`, entityID, action, database.ActorIDSetting, database.ActorJobSetting,
		entity, patientID); err != nil {
		return fmt.Errorf("writing the audit record: %w", err)
	}

	return nil
}

// requireCallerMayCreate is the authorization the database does not hold: the service path's policies carry no row
// predicate. A doctor must be on the care team of the patient they create — otherwise they create somebody they
// cannot then see — and may name colleagues beside themselves. An admin names whoever the clinic decided, themselves
// included or not: they are not a provider and no policy would let them look after anybody.
func requireCallerMayCreate(caller auth.Principal, specialists []Assignment) error {
	switch caller.Role {
	case "admin":
		return nil

	case "doctor":
		for _, assignment := range specialists {
			if assignment.ProviderID == caller.Subject {
				return nil
			}
		}

		return ErrCallerNotOnTheCareTeam

	default:
		return fmt.Errorf("the token's role is %q: %w", caller.Role, ErrCallerMayNotCreatePatients)
	}
}

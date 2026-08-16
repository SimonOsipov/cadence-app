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

// The refusals of a creation by invitation. Each names a different state of the
// address at the identity provider, because the transport answers them
// differently and a doctor reading the answer has to know whether to retry, to
// pick another address, or to call somebody.
var (
	// ErrAlreadyOnboarded means the address already belongs to a person this
	// clinic has created. It is what the loser of a double click sees, and it is
	// reached without asking the provider for anything: a second invitation
	// would rotate the token and kill the link the winner just sent.
	ErrAlreadyOnboarded = errors.New("this address already belongs to a patient")

	// ErrAccountIsNotOurs means the provider has an account at this address and
	// this clinic has no record of inviting it. It is not a half-finished
	// onboarding of ours to take over — claiming it would hand somebody else's
	// account to a new patient.
	ErrAccountIsNotOurs = errors.New("the address has an account this clinic did not invite")

	// ErrProvisionerUnavailable means the component that speaks to the identity
	// provider did not answer. It covers a refusal too: the component replaces
	// the provider's own words with a fixed 502 body, so "the address is already
	// taken" and "the provider is down" arrive here as the same failure — see
	// the note on settle.
	ErrProvisionerUnavailable = errors.New("the identity provider could not be reached")

	// ErrNoAddress means there is nothing to invite.
	ErrNoAddress = errors.New("an invitation needs an address")

	// ErrCallerNotOnTheCareTeam means a doctor created a patient without putting
	// themselves on the team. They would not be able to see the patient they
	// just created, and no screen shows a doctor a patient they are not assigned
	// to.
	ErrCallerNotOnTheCareTeam = errors.New("a doctor must be on the care team of a patient they create")

	// ErrCallerMayNotCreatePatients means the token's role is one that does not
	// create people — a patient's, or none at all.
	ErrCallerMayNotCreatePatients = errors.New("this caller may not create patients")

	// ErrCallerMayNotCreateProviders means somebody other than an administrator
	// tried to take a doctor on. A doctor who could would be a doctor who can
	// grow the clinic's staff, and the care team is what every doctor's access
	// runs through.
	ErrCallerMayNotCreateProviders = errors.New("only an administrator may create providers")
)

// creation is the one request the spine is serving: the folded address, the
// caller who asked, and whether the person being created is a patient.
//
// The last is carried for one column. The audit rows this flow writes outside
// the creating transaction — the invitation, and the deletion of an account
// being claimed — are the same act for either person, and patient_id is the
// column a patient's trail is read by: a doctor's identifier in it files the
// clinic's own hiring inside the trail of whichever patient holds that id.
type creation struct {
	address    string
	invitedBy  string
	forPatient bool
}

// patient is the audit row's patient_id: the person when the person is one, and
// nothing when they are staff.
func (c creation) patient(userID string) *string {
	if !c.forPatient {
		return nil
	}

	return &userID
}

// Held is what this clinic's own tables say about an account the provider knows.
//
// Both halves are read in one transaction and both are needed: an invite record
// is what makes an account ours to claim, and a profile is what makes the
// creation already done.
type Held struct {
	Invite  bool
	Profile bool
}

// Claim is what may be done with an address the provider already has an account
// for.
type Claim int

const (
	// ClaimByInviting finishes a creation that was interrupted: the account is
	// ours, nobody has opened the link, and a fresh invitation replaces it.
	ClaimByInviting Claim = iota

	// ClaimByDeletingFirst is the same with one difference that cannot be
	// skipped: the account has been opened, and a session from that link can set
	// a permanent password. Inviting over it would hand the new patient the
	// previous person's credential.
	ClaimByDeletingFirst

	// RefuseAlreadyOnboarded — a profile exists, so the creation is done.
	RefuseAlreadyOnboarded

	// RefuseNotOurs — no invite record, so this is somebody else's account.
	RefuseNotOurs
)

// ClaimFor is the claim rule of this block.
//
// The order of the arms is the rule: a profile settles it before anything else
// is asked, which is what stops the loser of a double click from sending a
// second invitation; then the invite record, which is the only evidence that an
// account is ours; and only then the account's own state, where confirmed and
// signed-in are two ways of saying the same thing — somebody has been inside.
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

// Onboarding creates patients by invitation.
//
// It holds two pools because the lock and the writes must not compete for one
// budget: every request in flight holds a lock connection for as long as the
// provider takes to answer, and a request that then wants a second connection
// from the same pool waits for one only another lock holder can release.
type Onboarding struct {
	// requests is the request path's pool, and it is used for nothing but the
	// advisory lock — no statement of this context reads or writes through it.
	requests *pgxpool.Pool

	// writes is the service path's pool: no policy lets a request create a
	// person, so the authorization for it is in Go, above.
	writes *pgxpool.Pool

	provisioner Provisioner
}

func NewOnboarding(requests, writes *pgxpool.Pool, provisioner Provisioner) *Onboarding {
	return &Onboarding{requests: requests, writes: writes, provisioner: provisioner}
}

// InvitePatient invites an address and writes the patient it belongs to,
// answering with the identifier the provider assigned.
//
// patient.UserID is not read: the identifier is the provider's, and one
// generated here would leave the token issuance hook resolving no profile at
// the moment the person signs in.
//
// The order is the whole design. The lock is taken first, on the folded
// address, and everything the provider is asked happens under it and outside a
// transaction — an external call between two statements of a transaction made a
// retry destructive, because a second invitation rotates the token of a link
// somebody is already holding.
func (o *Onboarding) InvitePatient(ctx context.Context, email string, patient NewPatient) (string, error) {
	address := NormalizeAddress(email)
	if address == "" {
		return "", ErrNoAddress
	}

	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		// The service seam refuses this too, and later. Refused here so that a
		// request the authentication middleware does not cover invites nobody.
		return "", ErrCallerMayNotCreatePatients
	}

	if err := requireCallerMayCreate(principal, patient.Specialists); err != nil {
		return "", err
	}

	// Before the lock and before anything leaves the process. CreatePatient runs
	// the full check later — it is that function's own guard — but by then the
	// invitation has gone out, so a malformed body spends the address: the
	// invitee holds a link to an account that will never get a profile, and the
	// doctor is answered without being told what to correct. The identifier is
	// not knowable yet, which is why this is the body's half alone.
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

// InviteProvider invites an address and writes the member of staff it belongs
// to, answering with the identifier the provider assigned.
//
// The same spine as InvitePatient — the lock on the folded address, the lookup,
// the invitation or the claim, then the record of the invitation and the person
// — with three differences: only an administrator may ask, the person written is
// a doctor rather than a patient, and the rows this flow signs name no patient.
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

			// The invitation is committed before the person, for the reason
			// record states: a creation interrupted after the mail has gone out
			// has to be curable by a retry.
			if err := o.remember(ctx, account.ID, of); err != nil {
				return err
			}

			return CreateProvider(ctx, o.writes, provider)
		})
	if err != nil {
		return "", err
	}

	return userID, nil
}

// settle decides between inviting, claiming and refusing, and comes back with
// the account the patient will be.
//
// Every failure of the provisioner arrives as ErrProvisionerUnavailable,
// including a refusal: measured on 2026-08-16, cmd/provisioner answers 502 with
// a fixed body for everything the identity provider refuses (routes.go's failed
// and refuse), so this side cannot tell "the address is already taken" from "the
// provider is down". What that costs is one wrong status on a race this lock
// closes — and the retry that follows it answers 409 through the lookup below,
// which is the answer the spec asks for by another road.
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

	switch ClaimFor(*found, held) {
	case RefuseAlreadyOnboarded:
		return Account{}, ErrAlreadyOnboarded

	case RefuseNotOurs:
		return Account{}, ErrAccountIsNotOurs

	case ClaimByDeletingFirst:
		// The proof travels as it was measured rather than as two literals: the
		// component cannot see the app schema and refuses a deletion whose proof
		// does not say an invite record exists and a profile does not.
		if err := o.provisioner.Delete(ctx, Deletion{
			ID: found.ID, InviteExists: held.Invite, ProfileExists: held.Profile,
		}); err != nil {
			return Account{}, fmt.Errorf("deleting the account being claimed: %w", ErrProvisionerUnavailable)
		}

		// Recorded before the invitation that follows it, and in a transaction of
		// its own: this is the most destructive thing the clinic does to another
		// system — an account and every session on it are gone — and the record of
		// it must not depend on the rest of the creation going through.
		if err := o.audit(ctx, accountDeleted, found.ID, of); err != nil {
			return Account{}, err
		}

		return o.invite(ctx, of)

	default:
		return o.invite(ctx, of)
	}
}

// invite asks for the invitation, and cures the failure that would otherwise
// burn the address.
//
// The account is created and the mail is sent before the answer reaches this
// side, so a call that fails on the way home — a timeout against the bound in
// provisioning.callTimeout, a lost connection — leaves an account this clinic
// has no record of. The next request would read it as somebody else's and
// refuse the address permanently, which is the one state this whole block
// exists to prevent.
//
// So the address is looked up again while the lock still holds, and an account
// nobody has been inside is recorded as ours. Untouched is the condition that
// keeps this from taking over a stranger's account: what a lost answer leaves
// behind is exactly an account with no confirmation and no sign-in, and an
// account somebody has used is not claimed on the strength of a failure.
//
// The request is still refused. Whether the mail actually left is not knowable
// from here, so answering 201 would tell the doctor a link is on its way that
// may not be; the retry finds the record, invites again, and that invitation is
// one this side saw succeed.
func (o *Onboarding) invite(ctx context.Context, of creation) (Account, error) {
	account, err := o.provisioner.Invite(ctx, of.address)
	if err == nil {
		return account, nil
	}

	// Detached from the request, and that is the whole point of this arm. The
	// case it exists for is an Invite that failed because the provisioner was
	// slow — and by then the request's own deadline is what the slowness ate.
	// Inheriting it, the recovery below gets whatever is left of 20s, which in
	// the dominant case is nothing: the lookup returns DeadlineExceeded, no
	// invites row is written, and the address is in the permanent 409 this arm
	// was written to prevent. Same move lock.go makes for its release.
	recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recoveryBudget)
	defer cancel()

	orphan, lookupErr := o.provisioner.Lookup(recoveryCtx, of.address)
	if lookupErr != nil || orphan == nil || hasBeenOpened(*orphan) {
		return Account{}, fmt.Errorf("inviting the address: %w", ErrProvisionerUnavailable)
	}

	if err := o.remember(recoveryCtx, orphan.ID, of); err != nil {
		// The address is burned if this write is lost, and there is nothing left
		// to try — so it travels inside the refusal, where the log keeps it.
		return Account{}, fmt.Errorf("recording an invitation whose answer was lost: %w: %w",
			err, ErrProvisionerUnavailable)
	}

	return Account{}, fmt.Errorf("inviting the address: %w", ErrProvisionerUnavailable)
}

// recoveryBudget bounds the two steps that cure a lost invitation. One
// provisioner call plus one short transaction, on a clock of their own: long
// enough for both against a provisioner that has just been slow, short enough
// that a request cannot hang on them after its own deadline has passed.
const recoveryBudget = 15 * time.Second

// hasBeenOpened is «somebody has been inside this account», and it is one
// predicate because it guards two different things: whether an orphan may be
// adopted, and whether an account may be deleted to make way for a new patient.
// Written twice it was measured once — dropping the LastSignInAt half left both
// suites green, on the copy that guards the destructive direction.
func hasBeenOpened(account Account) bool {
	return account.ConfirmedAt != nil || account.LastSignInAt != nil
}

// held reads what this clinic already has for an account.
//
// On the service path because that is where the answer is readable at all: a
// doctor holds INSERT on invites and no SELECT, so the request path cannot ask
// this question about its own invitation.
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

// record writes the two transactions of a creation, in the order that makes an
// interrupted one curable.
//
// The invitation is recorded first and on its own, and that is a departure from
// the spec's diagram worth stating: with the record inside the one transaction,
// a creation that failed after the mail went out would leave an account the next
// request cannot recognise as its own, and the retry the record exists for would
// answer 409 forever. The window does not close — the commit always comes after
// the side effect — it shrinks to the gap between the invitation and this first
// commit.
func (o *Onboarding) record(ctx context.Context, of creation, patient NewPatient) error {
	if err := o.remember(ctx, patient.UserID, of); err != nil {
		return err
	}

	return CreatePatient(ctx, o.writes, patient)
}

// remember commits the record of the invitation and the row that signs it.
func (o *Onboarding) remember(ctx context.Context, userID string, of creation) error {
	err := database.WithService(ctx, o.writes, func(ctx context.Context, tx pgx.Tx) error {
		// DO NOTHING rather than an update: no role holds UPDATE on this table,
		// and the row says an account was invited by this clinic — which a second
		// invitation to the same account does not change.
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.invites (user_id, email, invited_by)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_id) DO NOTHING
		`, userID, of.address, of.invitedBy); err != nil {
			return fmt.Errorf("writing the invite record: %w", err)
		}

		// One row per invitation asked for, so a second invitation to one account
		// is a second row. It says asked for rather than sent: delivery is the
		// provider's and is not reported back, and a request whose answer was lost
		// records the ask it certainly made.
		return writeAudit(ctx, tx, invitationAsked, userID, of.patient(userID))
	})
	if err != nil {
		return fmt.Errorf("recording the invitation to %s: %w", userID, classify(err))
	}

	return nil
}

// audit records one act of this flow that stands outside the creation's own
// transactions.
func (o *Onboarding) audit(ctx context.Context, action, entityID string, of creation) error {
	err := database.WithService(ctx, o.writes, func(ctx context.Context, tx pgx.Tx) error {
		return writeAudit(ctx, tx, action, entityID, of.patient(entityID))
	})
	if err != nil {
		return fmt.Errorf("recording %s for %s: %w", action, entityID, err)
	}

	return nil
}

// The actions this flow signs, and the entity each one acted on. Both columns
// are free text with no CHECK and no registry, so the vocabulary is settled by
// whoever writes it first: invite.send is the spec's, account.delete is this
// step's and names the one act performed against the identity provider rather
// than against our own schema.
//
// The entities differ for the same reason. An invitation writes our own row, so
// `invites` is what it acted on. A deletion destroys an account at the provider
// and leaves every row of ours alone — filing it under `invites` would read as
// an invite record having been removed, which deviation 10 says never happens.
const (
	invitationAsked = "invite.send"
	accountDeleted  = "account.delete"

	invitesEntity = "invites"
	accountEntity = "auth.account"
)

// entityOf is the one place the pairing lives, so the two columns cannot drift.
func entityOf(action string) string {
	if action == accountDeleted {
		return accountEntity
	}

	return invitesEntity
}

func writeAudit(ctx context.Context, tx pgx.Tx, action, entityID string, patientID *string) error {
	// The setting names travel as bound parameters — current_setting takes its
	// name as an argument — which keeps the statement the constant the authorship
	// gate requires.
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.audit_log (actor_id, actor_job, action, entity, entity_id, patient_id)
		VALUES (
			nullif(current_setting($3, true), '')::uuid,
			nullif(current_setting($4, true), ''),
			$2, $5, $1, $6
		)
	`, entityID, action, database.ActorIDSetting, database.ActorJobSetting,
		entityOf(action), patientID); err != nil {
		return fmt.Errorf("writing the audit record: %w", err)
	}

	return nil
}

// requireCallerMayCreate is the authorization the database does not hold: the
// service path's policies carry no row predicate, so who may create a patient
// and on whose behalf is decided here.
//
// A doctor must be on the care team of the patient they create — otherwise they
// create somebody they cannot then see — and may name colleagues beside
// themselves. An admin names whoever the clinic decided, themselves included or
// not: they are not a provider and no policy would let them look after anybody.
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

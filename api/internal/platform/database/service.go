package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// serviceRole is the impersonation target of the service path. It is a
// NOLOGIN role, reachable only from the service pool's connection role, and the
// request path cannot assume it — the two paths are separated by session_user
// rather than by a statement, so a bug on one cannot reach the other's grants.
const serviceRole = "cadence_service"

// The roles the two pools connect as. Pinned here because VerifyPools compares
// against them: a connection string pointing somewhere else is a deployment
// mistake with no symptom until the first request.
const (
	appRole        = "cadence_app"
	serviceAppRole = "cadence_service_app"
)

// The settings the audit actor travels under. Exactly one of them is published
// per transaction, which is what the audit_log constraint requires: a row with
// neither is unattributed, and a row with both names two actors for one action.
//
// Exported because the audit row is written by the bounded context that owns the
// action, not by this package, and it has to name them. A copy of these strings
// over there is a contract with no compile-time link: renaming one here would
// leave that package compiling and failing at runtime.
const (
	ActorIDSetting  = "app.actor_id"
	ActorJobSetting = "app.actor_job"
)

// ErrNoActor is returned for a service transaction with nobody to attribute it
// to. Every service write is audited, and an audit row nobody signed is a row
// that answers "who changed this" with silence.
var ErrNoActor = errors.New("the service path has no actor")

// Actor is who a service transaction is acting as.
//
// The distinction between a person and a job is made at compile time and kept
// there. Its fields are unexported and there are exactly two constructors, so a
// caller has to decide which it is rather than filling in whichever field is at
// hand — and the audit log's "exactly one of actor_id and actor_job" stops being
// a rule anybody can forget.
type Actor struct {
	subject string
	job     string
}

// ActingAsUser attributes the transaction to a person: the subject of the
// principal that asked for it.
//
// The subject is checked for UUID shape here rather than at the insert, for the
// same reason the caller's is: audit_log.actor_id is a uuid column, and a cast
// that fails inside a policy or a constraint is a 500 where a refusal belongs.
func ActingAsUser(subject string) (Actor, error) {
	if subject == "" {
		return Actor{}, fmt.Errorf("acting as a user: %w", ErrNoActor)
	}

	if !IsUUIDShaped(subject) {
		return Actor{}, fmt.Errorf("acting as %q: %w", subject, ErrInvalidSubject)
	}

	return Actor{subject: subject}, nil
}

// ActingAsJob attributes the transaction to a named system job — the reminder
// sweep, the invitation mailer — which has no principal behind it.
//
// The name is trimmed and a blank one is refused. It is the key the audit log
// groups by, and " reminder-sweep" beside "reminder-sweep" is two jobs in every
// report anybody runs.
func ActingAsJob(name string) (Actor, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return Actor{}, fmt.Errorf("acting as a job: %w", ErrNoActor)
	}

	return Actor{job: trimmed}, nil
}

// setting returns the connection setting this actor is published under and the
// value it carries — and the setting that has to be cleared, because "exactly
// one" is a property of the transaction rather than of this value. The zero
// Actor returns an empty value, which is what the guard in WithService reads.
func (a Actor) setting() (published, value, cleared string) {
	if a.subject != "" {
		return ActorIDSetting, a.subject, ActorJobSetting
	}

	return ActorJobSetting, a.job, ActorIDSetting
}

// WithService runs fn in a transaction on the service pool, as cadence_service.
//
// It is the path for what no policy lets a request do: creating a patient,
// writing another person's clinical fields, recording an invitation. Its
// authorization lives in Go rather than in a policy — the service-path policies
// carry no row predicate — and that is written down here rather than implied,
// because it is the one place in this codebase where the database is not the
// last word.
//
// Three things are true inside fn and false outside it. The effective role is
// cadence_service. The caller's claims are cleared — to the empty string, since
// set_config cannot unset, which is why everything reading them goes through
// nullif — so a service transaction cannot read an identity it was never given.
// The audit actor is published, and the audit_log policy reconciles the row
// against it — which is a "do not forget to name the actor" property rather
// than protection against forgery: code that can write the row can write the
// setting.
//
// Nesting either seam inside the other is refused rather than blocked. The cost
// of that refusal — no single transaction that both checks a caller's rights
// under RLS and writes through the service path — is real and is deferred, not
// solved.
func WithService(
	ctx context.Context, pool *pgxpool.Pool, actor Actor, fn func(context.Context, pgx.Tx) error,
) error {
	published, value, cleared := actor.setting()
	if value == "" {
		return fmt.Errorf("opening the service seam: %w", ErrNoActor)
	}

	ctx, err := enterSeam(ctx, serviceSeam)
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning the service transaction: %w", err)
	}

	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
		defer cancel()

		_ = tx.Rollback(rollbackCtx)
	}()

	// Cleared explicitly rather than relied upon to be absent — the claims and
	// whichever actor setting this transaction is not publishing. The settings are transaction-scoped and a fresh transaction inherits
	// nothing — but connections are pooled, and a statement inside somebody's
	// closure that issues SET without LOCAL leaves its value on the connection
	// for the next transaction to inherit. A property that holds only because
	// nobody wrote that statement is a property nobody is checking.
	//
	// The other actor setting matters as much as the claims: audit_log requires
	// exactly one of actor_id and actor_job, and a leftover from an earlier
	// transaction makes it two — breaking the constraint at the one moment the
	// row is being written, in the seam whose Actor type exists to make that
	// impossible.
	//
	// Note that clearing sets the value to the empty string, not to NULL:
	// set_config has no way to unset. Everything reading these settings must go
	// through nullif — app.jwt_subject() does exactly that, and it is why it
	// does.
	for _, setting := range []string{claimsSetting, cleared} {
		if _, err := tx.Exec(ctx, "SELECT set_config($1, '', true)", setting); err != nil {
			return fmt.Errorf("clearing %s: %w", setting, err)
		}
	}

	if _, err := tx.Exec(ctx, "SELECT set_config($1, $2, true)", published, value); err != nil {
		return fmt.Errorf("publishing the audit actor: %w", err)
	}

	// The same spelling as the request path, and for the same reason: the
	// statement form takes an identifier, so the role would have to be
	// concatenated into the SQL.
	if _, err := tx.Exec(ctx, "SELECT set_config('role', $1, true)", serviceRole); err != nil {
		return fmt.Errorf("assuming %s: %w", serviceRole, err)
	}

	if err := fn(ctx, tx); err != nil {
		return fmt.Errorf("running as %s: %w", serviceRole, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	return nil
}

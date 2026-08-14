package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/auth"
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

// ErrNoPrincipal is returned when a service transaction is opened on a context
// that carries no verified caller.
//
// It is a separate error from ErrNoActor because it names a different mistake: a
// service write reached for on a path the authentication middleware does not
// cover. Falling back to a job would answer it, and it would answer it by
// recording a person's action as a machine's.
var ErrNoPrincipal = errors.New("the service path has no verified caller")

// actor is who a service transaction is acting as.
//
// The distinction between a person and a job is made at compile time and kept
// there. The type is unexported, and so are its two constructors, so nothing
// outside this package can produce one — which is what makes "a handler cannot
// name somebody else's uuid" a property of the surface rather than a rule at
// every call site. TestTheExportedSurfaceIsTheOneDeclared is its witness.
type actor struct {
	subject string
	job     string
}

// actingAsUser attributes the transaction to a person: the subject of the
// principal the request was verified as.
//
// The subject is checked for UUID shape here rather than at the insert, for the
// same reason the caller's is: audit_log.actor_id is a uuid column, and a cast
// that fails inside a policy or a constraint is a 500 where a refusal belongs.
func actingAsUser(subject string) (actor, error) {
	if subject == "" {
		return actor{}, fmt.Errorf("acting as a user: %w", ErrNoActor)
	}

	if !IsUUIDShaped(subject) {
		return actor{}, fmt.Errorf("acting as %q: %w", subject, ErrInvalidSubject)
	}

	return actor{subject: subject}, nil
}

// actingAsJob attributes the transaction to a named system job — the reminder
// sweep, the invitation mailer — which has no principal behind it.
//
// The name is trimmed and a blank one is refused. It is the key the audit log
// groups by, and " reminder-sweep" beside "reminder-sweep" is two jobs in every
// report anybody runs.
func actingAsJob(name string) (actor, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return actor{}, fmt.Errorf("acting as a job: %w", ErrNoActor)
	}

	return actor{job: trimmed}, nil
}

// setting also names the setting that has to be cleared, because "exactly one"
// is a property of the transaction rather than of this value. The zero actor
// returns an empty value, which is what the guard in withService reads.
func (a actor) setting() (published, value, cleared string) {
	if a.subject != "" {
		return ActorIDSetting, a.subject, ActorJobSetting
	}

	return ActorJobSetting, a.job, ActorIDSetting
}

// WithService runs fn on the service pool on behalf of the caller the request
// was verified as.
//
// The actor is derived from auth.PrincipalFrom rather than taken as an argument,
// and that is the whole point of the shape: forging the actor survived two drafts
// of the proposal because it was an argument's value, where every call site
// looked right and one naming a different uuid would have looked right too. There
// is now nowhere to write one — the actor type is unexported and no exported
// function of this package takes a subject.
//
// A request with no verified caller is refused rather than attributed to a job.
// The two are the states the audit log cannot tell apart afterwards, and a
// service write on an unauthenticated path is a wiring mistake, not a fallback.
//
// Paths with no human use WithServiceJob.
func WithService(
	ctx context.Context, pool *pgxpool.Pool, fn func(context.Context, pgx.Tx) error,
) error {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return fmt.Errorf("opening the service seam: %w", ErrNoPrincipal)
	}

	acting, err := actingAsUser(principal.Subject)
	if err != nil {
		return fmt.Errorf("opening the service seam: %w", err)
	}

	return withService(ctx, pool, acting, fn)
}

// WithServiceJob runs fn on the service pool on behalf of a named system job —
// the reminder sweep, the invitation mailer, the push fan-out.
//
// A separate function rather than a nil actor, so that "this path has nobody to
// derive an actor from" is a decision somebody made once, at the one place it is
// true, instead of a check every call site could forget. The name lands in
// app.actor_job; a uuid passed here names a job after a uuid and attributes the
// action to nobody.
//
// A principal on the context is ignored, deliberately: what the audit row says is
// what the constructor said, so "exactly one of actor_id and actor_job" stays the
// seam's property rather than the caller's.
func WithServiceJob(
	ctx context.Context, pool *pgxpool.Pool, job string, fn func(context.Context, pgx.Tx) error,
) error {
	acting, err := actingAsJob(job)
	if err != nil {
		return fmt.Errorf("opening the service seam: %w", err)
	}

	return withService(ctx, pool, acting, fn)
}

// withService runs fn in a transaction on the service pool, as cadence_service.
//
// It is the path for what no policy lets a request do: creating a patient,
// writing another person's clinical fields, recording an invitation. Its
// authorization lives in Go rather than in a policy — the service-path policies
// carry no row predicate, save the one on profiles.role — and that is written
// down here rather than implied, because it is the one place in this codebase
// where the database is not the last word.
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
func withService(
	ctx context.Context, pool *pgxpool.Pool, acting actor, fn func(context.Context, pgx.Tx) error,
) error {
	published, value, cleared := acting.setting()
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

	// Cleared explicitly rather than relied upon to be absent: a fresh
	// transaction inherits nothing, but connections are pooled, and a statement
	// inside somebody's closure that issues SET without LOCAL leaves its value
	// behind for the next transaction to inherit. A leftover actor setting is
	// audit_log's "exactly one" broken at the moment the row is written.
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

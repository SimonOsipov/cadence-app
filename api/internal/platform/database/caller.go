package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The Postgres roles a request can run as. Table privileges are granted to them
// and the row level security policies will be written for them; the role the API
// connects with holds neither and cannot inherit them.
//
// They are Postgres roles rather than a value compared inside a policy, and
// that is the decision the whole access model rests on: privileges in Postgres
// are bound to a role, so while the product role was only a claim, a grant the
// admin needed reached the patient too and the escalation could be closed
// neither by a policy nor by a column.
const (
	patientRole = "cadence_patient"
	doctorRole  = "cadence_doctor"
	adminRole   = "cadence_admin"
)

// postgresRoles is closed on purpose, and not because of injection — the role
// travels to the database as a bound parameter, so a value carried by a token
// could not execute even if one reached here. It is closed because it is an
// allowlist: a token must not choose a Postgres role even as data, and a role
// this package does not know has to be a refusal rather than a default.
var postgresRoles = map[string]string{
	"patient": patientRole,
	"doctor":  doctorRole,
	"admin":   adminRole,
}

// ErrUnknownRole is returned for a caller whose role has no Postgres role. It is
// named so a handler can tell it from a database failure: the first is a 403,
// the second is a 500.
//
// There is deliberately no default. A fallback would serve a token nobody
// recognizes as somebody, and which somebody would be decided by whichever map
// entry happened to be reached first.
var ErrUnknownRole = errors.New("the caller's role has no Postgres role")

// claimsSetting is the connection setting the claims are published under.
//
// The name is GoTrue's, and deliberately so: a policy written as
// current_setting('request.jwt.claims')::json ->> 'sub' means the same thing
// here as it does in every example written for that ecosystem, so the policies
// are not a dialect of their own.
const claimsSetting = "request.jwt.claims"

// roleClaim is the key the product role travels under.
//
// Not the stock `role`: GoTrue puts the literal "authenticated" there for every
// user, and a key that already means something else is a key somebody will read
// by accident. It is an input for choosing a Postgres role in the seam and
// nothing a policy consults — the policies key on TO.
const roleClaim = "cadence_role"

// ErrNoSubject is returned for a caller with no identity. It is named so that a
// handler can tell it from a database failure: the first is a 401, the second
// is a 500.
var ErrNoSubject = errors.New("the caller has no subject")

// ErrInvalidSubject is returned for a subject that is not a UUID.
//
// Checked here rather than left to a policy. The idiom a policy is written with
// casts the claim to uuid, and a cast that fails raises 22P02 — a 500 for what
// is really a refusal, and one that happens inside a policy where no handler can
// tell what went wrong.
var ErrInvalidSubject = errors.New("the caller's subject is not a UUID")

// ErrNestedSeam is returned when either seam is opened inside the other.
//
// A nested call takes a second connection from a second pool and opens an
// independent transaction: it cannot see the outer one's uncommitted writes, and
// the outer rollback leaves its rows committed. The cost is named rather than
// hidden — checking a caller's rights under RLS and writing through the service
// path in one transaction is unavailable, and that is deferred, not solved.
var ErrNestedSeam = errors.New("a seam is already open on this context")

// seamKey marks a context that is already inside a seam. The type is unexported
// so nothing outside this package can clear the mark by writing the key itself.
type seamKey struct{}

// seam names which one, so the refusal can say what it collided with.
type seam string

const (
	callerSeam  seam = "WithCaller"
	serviceSeam seam = "WithService"
)

// enterSeam refuses a nested call in either order and returns the context the
// closure will run under.
func enterSeam(ctx context.Context, entering seam) (context.Context, error) {
	if open, ok := ctx.Value(seamKey{}).(seam); ok {
		return nil, fmt.Errorf("%s inside %s: %w", entering, open, ErrNestedSeam)
	}

	return context.WithValue(ctx, seamKey{}, entering), nil
}

// validSubject reports whether the subject is UUID-shaped in the canonical form
// Postgres writes.
//
// Round-tripped rather than merely parsed. pgx's parser switches on length and
// splices the hex out of positions 8, 13, 18 and 23 without checking that the
// separators are hyphens, so it accepts values uuid_in refuses — and a value
// this function passes and the database rejects is a 22P02 raised inside a
// policy, which is the exact failure ErrInvalidSubject exists to prevent.
func validSubject(subject string) bool {
	var parsed pgtype.UUID
	if err := parsed.Scan(subject); err != nil {
		return false
	}

	canonical, err := parsed.Value()
	if err != nil {
		return false
	}

	text, ok := canonical.(string)

	return ok && strings.EqualFold(text, subject)
}

// rollbackTimeout bounds the rollback of a transaction whose context is already
// finished. Without it, a Postgres that has stopped answering holds the pool
// slot for as long as the network takes to notice — and a handful of those is
// the whole API.
const rollbackTimeout = 5 * time.Second

// Caller is the verified identity a transaction runs as.
//
// It is declared here rather than taken from the authentication package on
// purpose. This package is the consumer, so it names what it needs — a subject
// and a role — and stays unaware of tokens, JWKS and HTTP. The conversion costs
// one struct literal at the call site and keeps the dependency pointing the
// right way.
//
// Neither field is trusted as given. Subject is required to be UUID-shaped,
// because the idiom a policy is written with casts the claim to uuid and a cast
// that fails raises 22P02 — a 500, from inside a policy, for what is really a
// refusal. Role is resolved against a closed map. Both are checked before a
// transaction opens, so a caller the seam cannot serve never takes a connection.
//
// A Caller is still constructible with neither, and that is the one asymmetry
// worth naming: the validity of this value is enforced by WithCaller rather than
// by the type, so a handler learns its caller is unusable only once it has
// reached the pool.
type Caller struct {
	// Subject is the user id every policy is keyed on.
	Subject string

	// Role is the role asserted by the caller's token.
	Role string
}

// WithCaller runs fn in a transaction that has taken on the caller's identity.
//
// Two things are true inside fn and false outside it: the effective role is the
// Postgres role of the caller's product role, and the caller's claims are
// readable through current_setting. Both are transaction-scoped, so they end
// when the transaction does — which matters because connections are pooled, and
// a role that outlived its transaction would be handed to the next request as
// somebody else's identity.
//
// The claims are passed as a bind parameter, never composed into the statement.
// The subject arrives from a token: it has passed a signature check, which says
// nothing about its contents, and it is otherwise attacker-controlled input
// heading for a SET.
//
// fn receives the transaction and must pass it down to whatever it calls.
// Calling WithCaller again from inside fn is not supported: it takes a second
// connection and opens an independent transaction that cannot see the outer
// one's uncommitted writes, and once the pool is exhausted it blocks on Acquire
// until the context ends. Under load that reads as an intermittent timeout
// rather than as the structural mistake it is.
func WithCaller(
	ctx context.Context, pool *pgxpool.Pool, caller Caller, fn func(context.Context, pgx.Tx) error,
) error {
	if caller.Subject == "" {
		// A policy comparing against an empty subject matches nothing in the
		// best case and everything in the worst. Neither is a caller.
		return fmt.Errorf("impersonating: %w", ErrNoSubject)
	}

	if !validSubject(caller.Subject) {
		return fmt.Errorf("impersonating %q: %w", caller.Subject, ErrInvalidSubject)
	}

	// Resolved before the transaction opens: there is no role to assume, so
	// there is nothing to open a transaction for.
	impersonatedRole, known := postgresRoles[caller.Role]
	if !known {
		return fmt.Errorf("impersonating %q: %w", caller.Role, ErrUnknownRole)
	}

	ctx, err := enterSeam(ctx, callerSeam)
	if err != nil {
		return err
	}

	// Both values have already been through a guard of their own, so neither can
	// be empty here — which is what "empty values are not published" amounts to
	// on this path. It is a property of the checks above rather than of a filter
	// below, and a third claim added later would need its own.
	claims, err := json.Marshal(map[string]string{
		"sub":     caller.Subject,
		roleClaim: caller.Role,
	})
	if err != nil {
		return fmt.Errorf("encoding the caller's claims: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning the transaction: %w", err)
	}

	// Rollback on every path that does not reach the commit, including a panic.
	// After a successful commit it is a no-op that returns pgx.ErrTxClosed.
	//
	// The cancellation is dropped because the usual reason to get here is a
	// context that has just been cancelled, and a rollback that inherits it
	// never reaches the server — leaving the connection to be discarded with
	// the transaction still open. The deadline is put back because this runs on
	// the request path: an unbounded rollback against an unresponsive Postgres
	// holds its pool slot for as long as the network takes to notice.
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
		defer cancel()

		_ = tx.Rollback(rollbackCtx)
	}()

	// Claims first, role second. set_config on a custom setting is allowed to
	// either role, but doing it in this order means the seam never depends on
	// what the impersonated role happens to be permitted.
	if _, err := tx.Exec(ctx, "SELECT set_config($1, $2, true)", claimsSetting, string(claims)); err != nil {
		return fmt.Errorf("publishing the caller's claims: %w", err)
	}

	// SET LOCAL ROLE, spelled as an assignment to the role parameter, which is
	// what the statement is defined as. Written this way because the statement
	// form takes an identifier and admits no placeholder, so the role name would
	// have to be concatenated in — and a SQL string assembled from a variable is
	// exactly the shape the authorship gate forbids, whatever the variable is
	// known to hold today. Here the statement is a constant and the role is a
	// bound parameter, so there is no shape left to get wrong.
	//
	// is_local is true: the role ends with the transaction. Connections are
	// pooled, and a role that outlived its transaction would be handed to the
	// next request as somebody else's identity.
	if _, err := tx.Exec(ctx, "SELECT set_config('role', $1, true)", impersonatedRole); err != nil {
		return fmt.Errorf("impersonating %s: %w", impersonatedRole, err)
	}

	if err := fn(ctx, tx); err != nil {
		return fmt.Errorf("running as %s: %w", impersonatedRole, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	return nil
}

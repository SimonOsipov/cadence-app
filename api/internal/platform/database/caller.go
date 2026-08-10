package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// impersonatedRole is the role a request runs as. Table privileges are granted
// to it and the row level security policies of M2 are written for it; the role
// the API connects with holds neither and cannot inherit them.
const impersonatedRole = "cadence_authenticated"

// claimsSetting is the connection setting the claims are published under.
//
// The name is Supabase's, and deliberately so: a policy written as
// current_setting('request.jwt.claims')::json ->> 'sub' means the same thing
// here as it does in every Supabase example, so the policies of M2 are not a
// dialect of their own.
const claimsSetting = "request.jwt.claims"

// ErrNoSubject is returned for a caller with no identity. It is named so that a
// handler can tell it from a database failure: the first is a 401, the second
// is a 500.
var ErrNoSubject = errors.New("the caller has no subject")

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
// Two limits are worth stating before M2 writes its first policy against this.
// Subject is checked for emptiness and for nothing else: the Supabase idiom
// casts sub to uuid, so a policy written that way will raise 22P02 — a 500,
// not a refusal — on a subject that is not one. And Role carries whatever the
// token asserted, which in a stock Supabase project is the literal
// "authenticated" for every user; the product roles arrive through a custom
// access token hook, and where that hook puts them is a decision M2 has to make
// rather than inherit.
type Caller struct {
	// Subject is the user id every policy is keyed on.
	Subject string

	// Role is the role asserted by the caller's token.
	Role string
}

// WithCaller runs fn in a transaction that has taken on the caller's identity.
//
// Two things are true inside fn and false outside it: the effective role is
// cadence_authenticated, and the caller's claims are readable through
// current_setting. Both are transaction-scoped, so they end when the
// transaction does — which matters because connections are pooled, and a role
// that outlived its transaction would be handed to the next request as somebody
// else's identity.
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

	claims, err := json.Marshal(map[string]string{"sub": caller.Subject, "role": caller.Role})
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

	// The role name is a constant of this package, not input, so there is
	// nothing here to parameterise — SET ROLE takes an identifier and no
	// placeholder is possible in one.
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+impersonatedRole); err != nil {
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

// Command bootstrap-admin creates the clinic's first administrator.
//
// Staff are created by an administrator and the first one has nobody to create them: migration 000006 refuses
// role='admin' from the service path whoever asks. So the first row is written from outside the request path
// entirely, under the migration role, once per environment.
//
// It records no invitation — app.invites carries no policy for the owner — and that is harmless because the command
// never claims an account. The account exists at the provider first, its identifier is the argument, so a failed run
// is repeated with the same identifier and one somebody lost is what the provisioner's lookup answers with.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/config"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

const usage = `usage: bootstrap-admin <account> "<full name>"

  account    the identifier the identity provider assigned to this person's
             account, which becomes app.profiles.user_id — the token issuance
             hook resolves the profile by it, so an invented one signs nobody in
  full name  as it should read in the interface, in Russian

The account has to exist at the provider before this runs: invite the address
through the provisioner, then pass the identifier it answers with.

The command refuses to run when the clinic already has an administrator. Doctors
after this one are created by an administrator through POST /v1/providers; that
route creates doctors only, so a second administrator has no path at all.

Environment:
  DATABASE_MIGRATION_URL  connection string of the migration role (required)
`

// ownerRole is assumed rather than inherited: a NOINHERIT member of it reaches 000009's policies only after SET ROLE.
const ownerRole = "cadence_owner"

// auditJob signs the audit row, and 000009's policy lets the owner sign no other way: a job, never a person.
const auditJob = "bootstrap-admin"

// auditAction is the bootstrap's own, not the staff route's provider.create: an administrator is not a provider.
const auditAction = "admin.bootstrap"

// errArguments lets the command answer a usage mistake before it asks for a connection string.
var errArguments = errors.New("bootstrap-admin takes an account identifier and a full name")

// errAdministratorExists: nothing on app.profiles is unique on role, so this check is all that stops a second run.
var errAdministratorExists = errors.New("the clinic already has an administrator")

type administrator struct {
	userID   string
	fullName string
}

// writer is the seam the argument handling is tested through: everything below the parse needs a database.
type writer func(ctx context.Context, databaseURL string, person administrator) error

func main() {
	if err := run(context.Background(), os.Args[1:], writeTheFirstAdministrator); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap-admin: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, write writer) error {
	person, err := parse(args)
	if err != nil {
		fmt.Fprint(os.Stderr, usage)

		return err
	}

	cfg, err := config.LoadMigration()
	if err != nil {
		return err
	}

	if err := write(ctx, cfg.URL, person); err != nil {
		return err
	}

	fmt.Printf("bootstrap-admin: %s is the clinic's administrator\n", person.userID)

	return nil
}

func parse(args []string) (administrator, error) {
	if len(args) != 2 {
		return administrator{}, fmt.Errorf("%w, got %d argument(s)", errArguments, len(args))
	}

	person := administrator{userID: args[0], fullName: strings.TrimSpace(args[1])}
	if !database.IsUUIDShaped(person.userID) {
		return administrator{}, fmt.Errorf("%w: %q is not an account identifier", errArguments, person.userID)
	}
	if person.fullName == "" {
		return administrator{}, fmt.Errorf("%w: the full name is empty", errArguments)
	}

	return person, nil
}

// writeTheFirstAdministrator writes the profile and the audit row, or neither.
//
// Nothing here serialises two simultaneous runs: both would find no administrator and both write one. This is a
// command a person runs once per environment rather than a path anything calls.
func writeTheFirstAdministrator(ctx context.Context, databaseURL string, person administrator) error {
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connecting as the migration role: %w", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning the transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	// set_config rather than SET ROLE: the statement form takes an identifier, which would have to be concatenated in.
	if _, err := tx.Exec(ctx, "SELECT set_config('role', $1, true)", ownerRole); err != nil {
		return fmt.Errorf("assuming %s: %w", ownerRole, err)
	}

	var exists bool
	if err := tx.QueryRow(
		ctx, `SELECT EXISTS (SELECT FROM app.profiles WHERE role = 'admin')`,
	).Scan(&exists); err != nil {
		return fmt.Errorf("looking for an administrator: %w", err)
	}
	if exists {
		return errAdministratorExists
	}

	// timezone is left out rather than guessed: it is captured at the person's first sign-in.
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.profiles (user_id, role, full_name) VALUES ($1, 'admin', $2)
	`, person.userID, person.fullName); err != nil {
		return fmt.Errorf("writing the administrator's profile: %w", err)
	}

	// In the profile's own transaction: a rollback takes the signature with the thing signed.
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.audit_log (actor_job, action, entity, entity_id)
		VALUES ($1, $2, 'profiles', $3)
	`, auditJob, auditAction, person.userID); err != nil {
		return fmt.Errorf("signing the audit row: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	return nil
}

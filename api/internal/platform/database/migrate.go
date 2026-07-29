package database

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // registers the postgres driver
	_ "github.com/golang-migrate/migrate/v4/source/file"       // registers the file source
)

// RunMigrations applies every pending migration from migrationsPath.
//
// The chain in the repository is the only owner of the schema and of the RLS
// policies — the Supabase dashboard never edits either. The composition root
// starts calling this once the chain has its first migration.
//
// It takes no context on purpose: golang-migrate's Up is not context-aware, and
// cancellation goes through m.GracefulStop instead. A migration that has to be
// interruptible on SIGTERM needs that wiring added here first.
func RunMigrations(databaseURL, migrationsPath string) error {
	return withChain(databaseURL, migrationsPath, func(m *migrate.Migrate) error {
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("applying migrations: %w", err)
		}

		return nil
	})
}

// MigrateDown rolls the chain back by steps migrations, or all the way when
// steps is zero or negative.
//
// Rolling back is not a symmetric operation and is never automatic: the down
// migrations exist so that a bad deploy can be undone deliberately, and so that
// the test harness can prove the chain is reversible. Nothing calls this on
// startup.
func MigrateDown(databaseURL, migrationsPath string, steps int) error {
	return withChain(databaseURL, migrationsPath, func(m *migrate.Migrate) error {
		// An already-empty database is asked about rather than inferred from the
		// error afterwards. golang-migrate reports "nothing is applied" with a
		// bare os.ErrNotExist, but it reports a version recorded in the database
		// whose file is missing from the source with the same sentinel — and that
		// second case must not be swallowed. Rolling the schema back from a
		// checkout that predates the applied migration would otherwise print
		// success and change nothing.
		if _, _, err := m.Version(); err != nil {
			if errors.Is(err, migrate.ErrNilVersion) {
				return nil
			}

			return fmt.Errorf("reading the applied version: %w", err)
		}

		var err error
		if steps > 0 {
			err = m.Steps(-steps)
		} else {
			err = m.Down()
		}

		if err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("rolling back migrations: %w", err)
		}

		return nil
	})
}

// MigrateForce clears the dirty flag by declaring which version the database is
// actually at, without running any migration.
//
// A migration that fails halfway leaves the version row marked dirty, and every
// later up or down refuses to run until someone says what the truth is. That
// someone has to be a person: the command exists so the recovery is a reviewed
// action against a known state, rather than hand-edited SQL against
// schema_migrations in production.
func MigrateForce(databaseURL, migrationsPath string, version int) error {
	return withChain(databaseURL, migrationsPath, func(m *migrate.Migrate) error {
		if err := m.Force(version); err != nil {
			return fmt.Errorf("forcing version %d: %w", version, err)
		}

		return nil
	})
}

// migrationsTable is where this chain records what it has applied.
//
// Not golang-migrate's default `schema_migrations`, and the reason is a
// collision rather than a preference: GoTrue shares this database — its `auth`
// schema is what `profiles` references, so the two cannot be separated — and it
// brings its own migrator, which claims exactly that name with a single
// `version` column. golang-migrate then reads it and fails with
// `column "dirty" does not exist`, and which of the two wins depends on the
// order two containers happened to start in.
//
// Set here rather than in the connection string so that it cannot be forgotten
// by whoever writes the next DATABASE_MIGRATION_URL.
const migrationsTable = "cadence_schema_migrations"

// withChain opens the chain, hands it to fn and closes it, so that the two
// entry points cannot drift on how the source and the advisory lock are
// released.
func withChain(databaseURL, migrationsPath string, fn func(*migrate.Migrate) error) (err error) {
	target, err := withMigrationsTable(databaseURL)
	if err != nil {
		return err
	}

	m, err := migrate.New("file://"+migrationsPath, target)
	if err != nil {
		return fmt.Errorf("opening migration chain at %s: %w", migrationsPath, err)
	}

	// Named return: a failure to release the source or the advisory lock has to
	// surface, but never at the cost of masking the migration error itself.
	defer func() {
		srcErr, dbErr := m.Close()
		if closeErr := errors.Join(srcErr, dbErr); closeErr != nil && err == nil {
			err = fmt.Errorf("closing migration chain: %w", closeErr)
		}
	}()

	return fn(m)
}

// withMigrationsTable puts migrationsTable into the connection string.
//
// It overwrites rather than defers to a value already there: a URL that names
// a different table is a URL that would split this chain's history in two, and
// silently — the second table simply looks like a database with nothing
// applied, and the chain runs from the beginning against a populated schema.
func withMigrationsTable(databaseURL string) (string, error) {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return "", fmt.Errorf("parsing the migration database URL: %w", err)
	}

	query := parsed.Query()
	query.Set("x-migrations-table", migrationsTable)
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

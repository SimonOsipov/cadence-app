package database

import (
	"errors"
	"fmt"

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

// withChain opens the chain, hands it to fn and closes it, so that the two
// entry points cannot drift on how the source and the advisory lock are
// released.
func withChain(databaseURL, migrationsPath string, fn func(*migrate.Migrate) error) (err error) {
	m, err := migrate.New("file://"+migrationsPath, databaseURL)
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

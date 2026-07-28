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
func RunMigrations(databaseURL, migrationsPath string) (err error) {
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

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("applying migrations: %w", err)
	}

	return nil
}

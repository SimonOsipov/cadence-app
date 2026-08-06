// Command migrate applies and rolls back the migration chain.
//
// It is a separate binary from the API on purpose. Migrations run under the
// role that owns the schema; the API runs under a role that owns nothing and
// could not apply one if it tried. Applying the chain at startup would also
// race between instances during a rolling deploy — the pipeline runs this once,
// before the new revision starts taking traffic.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/config"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
)

const usage = `usage: migrate <command>

  up             apply every pending migration
  down [n|all]   roll back n migrations (default 1), or the whole chain
  force <n>      declare the applied version and clear the dirty flag
  new <name>     create an empty up/down pair at the next sequence number

A migration that fails halfway leaves the version marked dirty, and every later
up and down refuses to run. Look at what actually landed in the database, then
say so with "force": 0 means nothing is applied.

Environment:
  DATABASE_MIGRATION_URL  connection string of the migration role (required)
  MIGRATIONS_PATH         directory holding the chain (default: migrations)
`

// migrator is what run needs of the chain: three operations, named by what the
// subcommands do rather than by which package implements them.
//
// It exists because the alternative was untestable. run called the database
// package directly, so no test could reach it without a live Postgres, and
// none did — the whole of run had zero coverage. Turning `up` into
// MigrateDown(cfg.URL, cfg.Path, 0) left `go test ./cmd/migrate` green, which
// is to say `make migrate-up` in a deploy could have unwound the entire schema
// and printed «chain applied».
//
// A struct of functions rather than an interface: there is one production
// implementation and one test double, and naming a type for that would add a
// file without removing a coupling. A database is the case the test-double
// exception is written for — external, slow, and destructive when wrong.
type migrator struct {
	up    func(url, path string) error
	down  func(url, path string, steps int) error
	force func(url, path string, version int) error
}

func chainMigrator() migrator {
	return migrator{
		up:    database.RunMigrations,
		down:  database.MigrateDown,
		force: database.MigrateForce,
	}
}

func main() {
	if err := run(os.Args[1:], chainMigrator()); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, chain migrator) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)

		return errors.New("no command given")
	}

	// `new` writes files and reaches no database, so it is handled before the
	// configuration that would demand a connection string.
	if args[0] == "new" {
		if len(args) != 2 {
			return errors.New("new requires exactly one name")
		}

		return createMigration(migrationsPath(), args[1])
	}

	cfg, err := config.LoadMigration()
	if err != nil {
		return err
	}

	switch args[0] {
	case "up":
		if err := chain.up(cfg.URL, cfg.Path); err != nil {
			return err
		}
		fmt.Println("migrate: chain applied")

	case "down":
		steps, err := downSteps(args[1:])
		if err != nil {
			return err
		}
		if err := chain.down(cfg.URL, cfg.Path, steps); err != nil {
			return err
		}
		fmt.Println("migrate: chain rolled back")

	case "force":
		version, err := forceVersion(args[1:])
		if err != nil {
			return err
		}
		if err := chain.force(cfg.URL, cfg.Path, version); err != nil {
			return err
		}
		fmt.Printf("migrate: version forced to %d\n", version)

	default:
		fmt.Fprint(os.Stderr, usage)

		return fmt.Errorf("unknown command %q", args[0])
	}

	return nil
}

// downSteps reads the argument of `down`. Zero means the whole chain, and it
// has to be asked for by name: a bare `down` that meant "everything" is the
// command someone runs against the wrong environment once.
func downSteps(args []string) (int, error) {
	if len(args) == 0 {
		return 1, nil
	}
	if len(args) > 1 {
		return 0, errors.New("down takes at most one argument")
	}
	if args[0] == "all" {
		return 0, nil
	}

	n, err := strconv.Atoi(args[0])
	if err != nil {
		return 0, fmt.Errorf("parsing the step count %q: %w", args[0], err)
	}
	if n < 1 {
		return 0, fmt.Errorf(`step count must be positive, got %d (use "all" for the whole chain)`, n)
	}

	return n, nil
}

// forceVersion reads the argument of `force`. There is no default: the command
// exists because someone had to look at the database and decide what is true,
// and a guess would defeat that.
func forceVersion(args []string) (int, error) {
	if len(args) != 1 {
		return 0, errors.New("force requires exactly one version, 0 meaning nothing applied")
	}

	n, err := strconv.Atoi(args[0])
	if err != nil {
		return 0, fmt.Errorf("parsing the version %q: %w", args[0], err)
	}
	if n < 0 {
		return 0, fmt.Errorf("version must not be negative, got %d", n)
	}

	return n, nil
}

func migrationsPath() string {
	if p := os.Getenv("MIGRATIONS_PATH"); p != "" {
		return p
	}

	return "migrations"
}

// createMigration writes an empty up/down pair. Both files, always: a migration
// whose down half was never created is the one that cannot be undone at the
// moment it has to be.
func createMigration(dir, name string) error {
	slug := slugify(name)
	if slug == "" {
		return fmt.Errorf("name %q leaves nothing usable after slugging", name)
	}

	next, err := nextSequence(dir)
	if err != nil {
		return err
	}

	for _, direction := range []string{"up", "down"} {
		path := filepath.Join(dir, fmt.Sprintf("%06d_%s.%s.sql", next, slug, direction))
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			return fmt.Errorf("creating %s: %w", path, err)
		}
		fmt.Println(path)
	}

	return nil
}

func slugify(name string) string {
	return strings.Trim(strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r == ' ' || r == '-' || r == '_':
			return '_'
		default:
			return -1
		}
	}, name), "_")
}

// nextSequence reads the numeric prefixes already in the directory. golang-migrate
// orders the chain by that prefix, so a new migration has to land above every
// existing one — including ones a branch added while this branch was open.
func nextSequence(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", dir, err)
	}

	highest := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		prefix, _, found := strings.Cut(name, "_")
		if !found {
			continue
		}

		n, err := strconv.Atoi(prefix)
		if err != nil {
			continue
		}
		if n > highest {
			highest = n
		}
	}

	return highest + 1, nil
}

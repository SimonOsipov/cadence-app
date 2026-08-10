package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDownStepsDefaultsToOne(t *testing.T) {
	steps, err := downSteps(nil)
	if err != nil {
		t.Fatalf("downSteps: %v", err)
	}
	if steps != 1 {
		t.Errorf("downSteps() = %d, want 1", steps)
	}
}

// Rolling the whole chain back has to be spelled out. A bare `down` that means
// "everything" is the command someone runs against the wrong environment.
func TestDownStepsRequiresAllToBeSpelledOut(t *testing.T) {
	steps, err := downSteps([]string{"all"})
	if err != nil {
		t.Fatalf("downSteps: %v", err)
	}
	if steps != 0 {
		t.Errorf("downSteps(all) = %d, want 0 meaning the whole chain", steps)
	}
}

func TestDownStepsReadsACount(t *testing.T) {
	steps, err := downSteps([]string{"3"})
	if err != nil {
		t.Fatalf("downSteps: %v", err)
	}
	if steps != 3 {
		t.Errorf("downSteps(3) = %d, want 3", steps)
	}
}

func TestDownStepsRejectsBadInput(t *testing.T) {
	for _, args := range [][]string{
		{"two"},
		{"0"},
		{"-1"},
		{"1", "2"},
	} {
		if _, err := downSteps(args); err == nil {
			t.Errorf("downSteps(%v): want error, got nil", args)
		}
	}
}

// Recovering from a half-applied migration means declaring which version the
// database is actually at. There is no default worth guessing here: the whole
// point is that a person looked and decided.
func TestForceVersionReadsTheVersion(t *testing.T) {
	version, err := forceVersion([]string{"7"})
	if err != nil {
		t.Fatalf("forceVersion: %v", err)
	}
	if version != 7 {
		t.Errorf("forceVersion(7) = %d, want 7", version)
	}
}

func TestForceVersionAcceptsZeroMeaningNothingApplied(t *testing.T) {
	version, err := forceVersion([]string{"0"})
	if err != nil {
		t.Fatalf("forceVersion: %v", err)
	}
	if version != 0 {
		t.Errorf("forceVersion(0) = %d, want 0", version)
	}
}

func TestForceVersionRejectsBadInput(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{},
		{"latest"},
		{"-1"},
		{"1", "2"},
	} {
		if _, err := forceVersion(args); err == nil {
			t.Errorf("forceVersion(%v): want error, got nil", args)
		}
	}
}

func TestNextSequenceFollowsTheHighestPrefix(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"000001_base.up.sql",
		"000001_base.down.sql",
		"000007_invitations.up.sql",
		"000007_invitations.down.sql",
		"README.md",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatalf("seeding %s: %v", name, err)
		}
	}

	next, err := nextSequence(dir)
	if err != nil {
		t.Fatalf("nextSequence: %v", err)
	}
	if next != 8 {
		t.Errorf("nextSequence = %d, want 8", next)
	}
}

func TestNextSequenceStartsAtOneInAnEmptyDirectory(t *testing.T) {
	next, err := nextSequence(t.TempDir())
	if err != nil {
		t.Fatalf("nextSequence: %v", err)
	}
	if next != 1 {
		t.Errorf("nextSequence = %d, want 1", next)
	}
}

// Both halves, always. A migration whose down file was never created is the one
// that cannot be undone at the moment it has to be.
func TestCreateMigrationWritesBothDirections(t *testing.T) {
	dir := t.TempDir()

	if err := createMigration(dir, "Add care team"); err != nil {
		t.Fatalf("createMigration: %v", err)
	}

	for _, want := range []string{"000001_add_care_team.up.sql", "000001_add_care_team.down.sql"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("expected %s: %v", want, err)
		}
	}
}

func TestCreateMigrationRejectsANameWithNothingUsable(t *testing.T) {
	if err := createMigration(t.TempDir(), "!!!"); err == nil {
		t.Fatal("createMigration: want error for a name that slugs to nothing, got nil")
	}
}

// recorder is a migrator that runs nothing and remembers everything.
type recorder struct {
	calls []string
}

func (r *recorder) chain() migrator {
	return migrator{
		up: func(url, path string) error {
			r.calls = append(r.calls, fmt.Sprintf("up(%s,%s)", url, path))

			return nil
		},
		down: func(url, path string, steps int) error {
			r.calls = append(r.calls, fmt.Sprintf("down(%s,%s,%d)", url, path, steps))

			return nil
		},
		force: func(url, path string, version int) error {
			r.calls = append(r.calls, fmt.Sprintf("force(%s,%s,%d)", url, path, version))

			return nil
		},
	}
}

func configured(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_MIGRATION_URL", "postgres://migrator@localhost/cadence")
	t.Setenv("MIGRATIONS_PATH", "chain")
}

// TestRunDispatchesEachSubcommandToItsOwnOperation is the test the whole of run
// was missing. Measured before the seam existed: replacing RunMigrations with
// MigrateDown(cfg.URL, cfg.Path, 0) under `up` left `go test ./cmd/migrate`
// green, so `make migrate-up` could have unwound the schema and reported
// success. Nothing here touches a database; what is asserted is which operation
// each word maps to, and with which arguments.
func TestRunDispatchesEachSubcommandToItsOwnOperation(t *testing.T) {
	const url = "postgres://migrator@localhost/cadence"

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "up", args: []string{"up"}, want: "up(" + url + ",chain)"},
		{name: "down defaults to one", args: []string{"down"}, want: "down(" + url + ",chain,1)"},
		{name: "down n", args: []string{"down", "3"}, want: "down(" + url + ",chain,3)"},
		// Zero is the whole chain, and it is the destructive one — so it is
		// asserted to come only from the word that asks for it.
		{name: "down all", args: []string{"down", "all"}, want: "down(" + url + ",chain,0)"},
		{name: "force", args: []string{"force", "7"}, want: "force(" + url + ",chain,7)"},
		{name: "force zero", args: []string{"force", "0"}, want: "force(" + url + ",chain,0)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configured(t)
			rec := &recorder{}

			if err := run(tc.args, rec.chain()); err != nil {
				t.Fatalf("run(%v): %v", tc.args, err)
			}

			if len(rec.calls) != 1 || rec.calls[0] != tc.want {
				t.Errorf("calls = %v, want exactly [%s]", rec.calls, tc.want)
			}
		})
	}
}

// TestRunTouchesNothingOnABadCommand covers the arms that must refuse. An
// unknown subcommand exiting zero is the failure mode that makes a typo in a
// deploy script look like a successful migration.
func TestRunTouchesNothingOnABadCommand(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "no command", args: nil},
		{name: "unknown command", args: []string{"upp"}},
		{name: "down with two arguments", args: []string{"down", "1", "2"}},
		{name: "down with a negative count", args: []string{"down", "-1"}},
		{name: "force with no version", args: []string{"force"}},
		{name: "force with a negative version", args: []string{"force", "-1"}},
		{name: "new with no name", args: []string{"new"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configured(t)
			rec := &recorder{}

			if err := run(tc.args, rec.chain()); err == nil {
				t.Fatalf("run(%v) returned no error", tc.args)
			}
			if len(rec.calls) != 0 {
				t.Errorf("calls = %v, want none", rec.calls)
			}
		})
	}
}

// TestRunReportsAFailedChain: the operation's error is the command's error, and
// swallowing it would make a half-applied migration exit zero.
func TestRunReportsAFailedChain(t *testing.T) {
	configured(t)
	sentinel := errors.New("dirty at version 3")
	chain := migrator{up: func(string, string) error { return sentinel }}

	if err := run([]string{"up"}, chain); !errors.Is(err, sentinel) {
		t.Errorf("run(up) = %v, want %v", err, sentinel)
	}
}

// TestRunNeedsNoConnectionStringForNew: `new` writes files and reaches no
// database, so it must work on a laptop with no migration URL set — which is
// the state the developer creating a migration is usually in.
func TestRunNeedsNoConnectionStringForNew(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MIGRATIONS_PATH", dir)
	t.Setenv("DATABASE_MIGRATION_URL", "")
	rec := &recorder{}

	if err := run([]string{"new", "add widgets"}, rec.chain()); err != nil {
		t.Fatalf("run(new): %v", err)
	}

	for _, name := range []string{"000001_add_widgets.up.sql", "000001_add_widgets.down.sql"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if len(rec.calls) != 0 {
		t.Errorf("calls = %v, want none", rec.calls)
	}
}

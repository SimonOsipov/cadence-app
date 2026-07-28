package main

import (
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

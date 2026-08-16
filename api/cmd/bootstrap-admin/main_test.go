package main

import (
	"context"
	"errors"
	"testing"
)

// The identifiers the account would have at the provider. Shared with the
// integration file, which runs the same command against a real database.
const (
	adminID      = "8a1f3b7c-0000-4000-8000-000000000001"
	otherAdminID = "8a1f3b7c-0000-4000-8000-000000000002"

	adminName      = "Пётр Аверин"
	otherAdminName = "Марина Крылова"
)

// recorder is a writer that reaches no database and remembers what it was asked
// to write.
type recorder struct {
	calls []administrator
	url   string
	fail  error
}

func (r *recorder) write(_ context.Context, databaseURL string, person administrator) error {
	r.url = databaseURL
	r.calls = append(r.calls, person)

	return r.fail
}

func configured(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_MIGRATION_URL", "postgres://migrator@localhost/cadence")
}

func TestRunPassesTheAdministratorAndTheMigrationURLToTheWrite(t *testing.T) {
	configured(t)
	rec := &recorder{}

	if err := run(t.Context(), []string{adminID, "  " + adminName + "  "}, rec.write); err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(rec.calls) != 1 {
		t.Fatalf("the write was called %d time(s), want once", len(rec.calls))
	}
	if rec.calls[0].userID != adminID {
		t.Errorf("the account is %q, want %q", rec.calls[0].userID, adminID)
	}
	// Trimmed, because the name is a value a person types and the space around
	// it would be part of it everywhere it is ever displayed.
	if rec.calls[0].fullName != adminName {
		t.Errorf("the name is %q, want it trimmed", rec.calls[0].fullName)
	}
	if rec.url != "postgres://migrator@localhost/cadence" {
		t.Errorf("the write was aimed at %q", rec.url)
	}
}

// The arms that must refuse, and the property that makes a refusal worth
// something: nothing is written. A command that half-runs on a mistyped
// identifier leaves a profile nobody can sign in as and an audit row saying it
// was deliberate.
func TestRunWritesNothingOnAnArgumentItRefuses(t *testing.T) {
	for name, args := range map[string][]string{
		"no arguments":               nil,
		"only an account":            {adminID},
		"a third argument":           {adminID, adminName, "admin"},
		"an account that is not one": {"пётр", adminName},
		// The shape IsUUIDShaped round-trips for rather than merely parsing:
		// pgx's parser splices the hex out of fixed positions without reading
		// the separators, the reason recorded at database.IsUUIDShaped.
		"an account punctuated wrongly": {"8a1f3b7c:0000:4000:8000:000000000001", adminName},
		"a name of nothing but space":   {adminID, "   "},
		"no name at all":                {adminID, ""},
	} {
		t.Run(name, func(t *testing.T) {
			configured(t)
			rec := &recorder{}

			if err := run(t.Context(), args, rec.write); err == nil {
				t.Fatal("accepted")
			}
			if len(rec.calls) != 0 {
				t.Errorf("the write was called with %v, want not at all", rec.calls)
			}
		})
	}
}

// A usage mistake is answered as a usage mistake. Reading the configuration
// first would tell somebody who typed the wrong number of arguments that their
// connection string is missing, which sends them to fix the wrong thing.
func TestRunNamesTheArgumentsBeforeItAsksForAConnectionString(t *testing.T) {
	t.Setenv("DATABASE_MIGRATION_URL", "")
	rec := &recorder{}

	err := run(t.Context(), nil, rec.write)
	if err == nil {
		t.Fatal("accepted")
	}
	if !errors.Is(err, errArguments) {
		t.Errorf("run answered %v, want the argument mistake", err)
	}
}

// The connection string has no default, and the command must not fall through
// to libpq's — which resolves to the local user and, in a container, to the
// superuser.
func TestRunRefusesToRunWithoutTheMigrationURL(t *testing.T) {
	t.Setenv("DATABASE_MIGRATION_URL", "")
	rec := &recorder{}

	if err := run(t.Context(), []string{adminID, adminName}, rec.write); err == nil {
		t.Fatal("accepted")
	}
	if len(rec.calls) != 0 {
		t.Errorf("the write was called with %v, want not at all", rec.calls)
	}
}

// The write's failure is the command's failure. Swallowing it exits zero on a
// clinic that has no administrator, and whoever reads the pipeline afterwards
// sees a step that reported success.
func TestRunReportsAFailedWrite(t *testing.T) {
	configured(t)
	sentinel := errors.New("the write failed")
	rec := &recorder{fail: sentinel}

	if err := run(t.Context(), []string{adminID, adminName}, rec.write); !errors.Is(err, sentinel) {
		t.Errorf("run = %v, want %v", err, sentinel)
	}
}

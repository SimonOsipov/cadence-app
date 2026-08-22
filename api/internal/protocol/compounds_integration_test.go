//go:build integration

package protocol_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

func resolving(t *testing.T) *pgxpool.Pool {
	t.Helper()

	db := cluster.NewDatabase(t)
	pool, err := database.NewPool(t.Context(), db.ServiceAppURL)
	if err != nil {
		t.Fatalf("opening the service pool: %v", err)
	}
	t.Cleanup(pool.Close)
	seedForShape(t, pool)

	return pool
}

func resolve(t *testing.T, pool *pgxpool.Pool, ref protocol.CompoundRef) (protocol.CompoundID, error) {
	t.Helper()

	var id protocol.CompoundID
	err := database.WithServiceJob(
		t.Context(), pool, shapeJob,
		func(ctx context.Context, tx pgx.Tx) error {
			var err error
			id, err = protocol.ResolveCompound(ctx, tx, ref)

			return err
		},
	)

	return id, err
}

// «Повтор по регистру возвращает существующий compoundId, а не создаёт второй» is an
// acceptance criterion of this feature, and the uniqueness that makes it enforceable is an
// index on lower(name_ru) — so the case is the database's business and the whitespace is not.
func TestANameAlreadyInTheDirectoryResolvesToTheRowItAlreadyHas(t *testing.T) {
	pool := resolving(t)
	existing := protocol.CompoundID(shapeCompound)

	for _, spelling := range []string{
		"Семаглутид",
		"СЕМАГЛУТИД",
		"семаглутид",
		// Trimmed here, because the index cannot: lower() leaves this different from
		// the stored name and a second row would be created.
		"  Семаглутид  ",
	} {
		got, err := resolve(t, pool, protocol.CompoundRef{
			New: &protocol.NewCompound{NameRU: spelling, DefaultUnit: protocol.MG, Route: "sc", Icon: "syringe"},
		})
		if err != nil {
			t.Errorf("%q: %v", spelling, err)

			continue
		}
		if got != existing {
			t.Errorf("%q resolved to %s, want the row already there", spelling, got)
		}
	}

	if count := compoundsNamed(t, pool, "семаглутид"); count != 1 {
		t.Errorf("the directory holds %d rows for one drug", count)
	}
}

// Runs of whitespace inside a name are one space, which only a multi-word name can show:
// the first attempt at this case broke a single word with a space and asserted the two were
// the same drug. They are not, and the resolver is right to say so.
func TestRunsOfSpaceInsideANameAreOneSpace(t *testing.T) {
	pool := resolving(t)

	spaced := func(name string) protocol.CompoundRef {
		return protocol.CompoundRef{New: &protocol.NewCompound{
			NameRU: name, DefaultUnit: protocol.MCG, Route: "sc", Icon: "vial",
		}}
	}

	first, err := resolve(t, pool, spaced("Гормон  роста"))
	if err != nil {
		t.Fatalf("entering a two-word drug: %v", err)
	}
	again, err := resolve(t, pool, spaced("Гормон роста"))
	if err != nil {
		t.Fatalf("naming it with one space: %v", err)
	}
	if again != first {
		t.Errorf("the collapse gave %s, want %s", again, first)
	}

	// And a word broken by a space is a different drug, not the same one respaced.
	other, err := resolve(t, pool, spaced("Гормонроста"))
	if err != nil {
		t.Fatalf("entering the unspaced word: %v", err)
	}
	if other == first {
		t.Error("a word and the same letters split by a space resolved to one drug")
	}
}

// The other half: a drug the clinic has not used before is entered rather than refused.
func TestANameTheDirectoryDoesNotHaveIsEntered(t *testing.T) {
	pool := resolving(t)

	first, err := resolve(t, pool, protocol.CompoundRef{
		New: &protocol.NewCompound{NameRU: "Тирзепатид", DefaultUnit: protocol.MG, Route: "sc", Icon: "syringe"},
	})
	if err != nil {
		t.Fatalf("entering a new drug: %v", err)
	}

	// And the second call finds it rather than making another, which is the same
	// property as the case-insensitive repeat and a different code path: the first
	// call inserts, this one does not.
	again, err := resolve(t, pool, protocol.CompoundRef{
		New: &protocol.NewCompound{NameRU: "тирзепатид", DefaultUnit: protocol.MCG, Route: "im", Icon: "pill"},
	})
	if err != nil {
		t.Fatalf("naming it again: %v", err)
	}
	if again != first {
		t.Errorf("the repeat gave %s, want %s", again, first)
	}

	// The second call's unit, route and icon are discarded, and that is the decision:
	// the service path holds no UPDATE on this table, so a name collision must never
	// rewrite a reference row other patients' courses already point at.
	unit, route := describe(t, pool, first)
	if unit != string(protocol.MG) || route != "sc" {
		t.Errorf("the directory row now reads %s/%s, want мг/sc", unit, route)
	}
}

// A course may name a drug from the directory instead of describing one, and an id that is
// not in it has to be refused here — the foreign key would refuse it later, with a message
// naming a constraint rather than the field the doctor filled in.
func TestAnIdentifierTheDirectoryDoesNotHaveIsRefused(t *testing.T) {
	pool := resolving(t)

	known := protocol.CompoundID(shapeCompound)
	if got, err := resolve(t, pool, protocol.CompoundRef{ID: &known}); err != nil || got != known {
		t.Errorf("a known id resolved to %s, %v", got, err)
	}

	stranger := protocol.CompoundID("7c3f3b7c-0000-4000-8000-00000000dead")
	if _, err := resolve(t, pool, protocol.CompoundRef{ID: &stranger}); !errors.Is(err, protocol.ErrNoSuchCompound) {
		t.Errorf("an unknown id gave %v, want ErrNoSuchCompound", err)
	}

	// Neither half named, and both — a reference that says nothing and one that says
	// two things are the same programmer error, and neither has a sensible default.
	for _, malformed := range []protocol.CompoundRef{
		{},
		{ID: &known, New: &protocol.NewCompound{NameRU: "Семаглутид", DefaultUnit: protocol.MG, Route: "sc", Icon: "syringe"}},
	} {
		if _, err := resolve(t, pool, malformed); !errors.Is(err, protocol.ErrCompoundRefUnclear) {
			t.Errorf("%+v gave %v, want ErrCompoundRefUnclear", malformed, err)
		}
	}

	// And a name that is nothing but space, which normalises to nothing at all.
	if _, err := resolve(t, pool, protocol.CompoundRef{
		New: &protocol.NewCompound{NameRU: "   ", DefaultUnit: protocol.MG, Route: "sc", Icon: "syringe"},
	}); !errors.Is(err, protocol.ErrCompoundRefUnclear) {
		t.Errorf("a blank name gave %v, want ErrCompoundRefUnclear", err)
	}
}

func compoundsNamed(t *testing.T, pool *pgxpool.Pool, lowered string) int {
	t.Helper()

	var count int
	if err := database.WithServiceJob(
		t.Context(), pool, shapeJob,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`SELECT count(*) FROM app.compounds WHERE lower(name_ru) = $1`, lowered).Scan(&count)
		},
	); err != nil {
		t.Fatalf("counting: %v", err)
	}

	return count
}

func describe(t *testing.T, pool *pgxpool.Pool, id protocol.CompoundID) (string, string) {
	t.Helper()

	var unit, route string
	if err := database.WithServiceJob(
		t.Context(), pool, shapeJob,
		func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx,
				`SELECT default_unit, route FROM app.compounds WHERE id = $1`, string(id)).Scan(&unit, &route)
		},
	); err != nil {
		t.Fatalf("describing: %v", err)
	}

	return unit, route
}

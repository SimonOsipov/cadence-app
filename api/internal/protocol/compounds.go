package protocol

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
)

var (
	// The identifier came from a directory the caller read, so its absence is a stale
	// client rather than a typo — but it reaches the doctor as a named field either way,
	// which the foreign key's 23503 would not.
	ErrNoSuchCompound = errors.New("no such compound in the directory")

	// A reference that names neither half, or both, or a name of nothing but space.
	ErrCompoundRefUnclear = errors.New("a compound is named by its identifier or by its name, and by one of them")
)

// CompoundRef is how a course names what is injected: by an identifier the doctor picked
// from the directory, or by a name they typed. Exactly one, and «both» is an error rather
// than a precedence rule — a precedence rule is a silent answer to a question the caller got
// wrong.
type CompoundRef struct {
	ID  *CompoundID
	New *NewCompound
}

// NewCompound is a drug the clinic has not prescribed before. The three besides the name are
// NOT NULL in the directory and have no sensible default: a unit guessed here is a dose
// rendered wrong on every screen.
type NewCompound struct {
	NameRU      string
	DefaultUnit DoseUnit
	Route       string
	Icon        string
}

// ResolveCompound turns a reference into an identifier, entering the drug into the directory
// if the clinic has not used it before. Case is the database's business — the index is on
// lower(name_ru) — and whitespace is not. DO NOTHING then SELECT rather than DO UPDATE …
// RETURNING: the service path holds no UPDATE here, so a collision cannot rewrite a row other
// courses point at.
// EnterNewDrugs inserts every drug a draft names by name, in one pass and in a fixed order.
//
// Sorted by normalised name, and the ordering is load-bearing across tenants: app.compounds is
// the one table every patient shares, so {X, Y} against {Y, X} deadlocks and one patient's
// write fails because of another's.
func EnterNewDrugs(ctx context.Context, tx pgx.Tx, items []DraftItem) error {
	names := make([]string, 0, len(items))
	described := make(map[string]NewCompound, len(items))

	for _, item := range items {
		if item.Compound.New == nil {
			continue
		}
		name := normaliseName(item.Compound.New.NameRU)
		if name == "" {
			return fmt.Errorf("the name is blank: %w", ErrCompoundRefUnclear)
		}
		if _, seen := described[name]; !seen {
			names = append(names, name)
			described[name] = *item.Compound.New
		}
	}

	slices.Sort(names)
	for _, name := range names {
		drug := described[name]
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.compounds (name_ru, default_unit, route, icon)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT DO NOTHING
		`, name, string(drug.DefaultUnit), drug.Route, drug.Icon); err != nil {
			return fmt.Errorf("entering %q into the directory: %w", name, err)
		}
	}

	return nil
}

func ResolveCompound(ctx context.Context, tx pgx.Tx, ref CompoundRef) (CompoundID, error) {
	named := 0
	if ref.ID != nil {
		named++
	}
	if ref.New != nil {
		named++
	}
	if named != 1 {
		return "", fmt.Errorf("%d halves named: %w", named, ErrCompoundRefUnclear)
	}

	if ref.ID != nil {
		return *ref.ID, requireCompound(ctx, tx, *ref.ID)
	}

	name := normaliseName(ref.New.NameRU)
	if name == "" {
		return "", fmt.Errorf("the name is blank: %w", ErrCompoundRefUnclear)
	}

	// The insert is EnterNewDrugs' business on the write path, which does it in a fixed
	// order for every transaction. Kept here for the callers that resolve one reference
	// on its own — the seed of step 11 — where there is no second name to order against.
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.compounds (name_ru, default_unit, route, icon)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING
	`, name, string(ref.New.DefaultUnit), ref.New.Route, ref.New.Icon); err != nil {
		return "", fmt.Errorf("entering %q into the directory: %w", name, err)
	}

	var id CompoundID
	if err := tx.QueryRow(ctx, `
		SELECT id::text FROM app.compounds WHERE lower(name_ru) = lower($1)
	`, name).Scan(&id); err != nil {
		return "", fmt.Errorf("reading %q back: %w", name, err)
	}

	return id, nil
}

func requireCompound(ctx context.Context, tx pgx.Tx, id CompoundID) error {
	var known bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT FROM app.compounds WHERE id = $1)`, string(id)).Scan(&known); err != nil {
		return fmt.Errorf("looking up compound %s: %w", id, err)
	}
	if !known {
		return fmt.Errorf("%s: %w", id, ErrNoSuchCompound)
	}

	return nil
}

// normaliseName trims and collapses runs of whitespace, and does nothing to the case: the
// stored name keeps the doctor's own capitalisation, and lower(name_ru) is what makes the
// repeat find it.
func normaliseName(name string) string { return strings.Join(strings.Fields(name), " ") }

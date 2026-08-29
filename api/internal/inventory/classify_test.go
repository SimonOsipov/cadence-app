package inventory

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/storage"
)

// Naming the refusal is the whole content of this function, and the transport suite cannot see
// it: every case there answers 422, so swapping two sentinels — or collapsing the switch into
// «any 23514 is an amount off range» — passes it whole.
//
// The code and the name together, never either alone: a foreign key violation and a check
// violation both arrive as constraint names on this table, and the same name under another
// code is a different failure.
func TestEachConstraintTheFormCanBreakIsNamed(t *testing.T) {
	for _, refusal := range []struct {
		name       string
		code       string
		constraint string
		want       error
	}{
		{"a drug nobody has", "23503", "vials_compound_id_fkey", ErrNoSuchCompound},
		{"a key under another prefix", "23514", "vials_photo_key_is_under_its_own_prefix", ErrKeyNotTheirs},
		{"an amount finer than a microgram", "23514", "vials_total_amount_scale_check", ErrAmountTooFine},
		{"an amount of nothing", "23514", "vials_total_amount_check", ErrAmountOffRange},
		{"an amount past the ceiling", "23514", "vials_total_amount_magnitude_check", ErrAmountOffRange},
		{"a day that moved backwards", "23514", "vials_disposed_after_opening", ErrDisposedTooEarly},
	} {
		t.Run(refusal.name, func(t *testing.T) {
			got := classifyWrite(&pgconn.PgError{Code: refusal.code, ConstraintName: refusal.constraint})
			if !errors.Is(got, refusal.want) {
				t.Errorf("got %v, want %v", got, refusal.want)
			}
		})
	}

	// And the other direction, which the table cannot supply: what this function does not
	// know stays itself, so a bug reaches the caller as a 500 rather than as the patient's
	// own form being wrong.
	for _, unknown := range []struct {
		name string
		err  error
	}{
		{
			// A real constraint this switch deliberately does not map: huma's enum is
			// the door for the unit, so reaching it means the door was bypassed.
			"a constraint nothing here maps",
			&pgconn.PgError{Code: "23514", ConstraintName: "vials_amount_unit_check"},
		},
		{
			// The same name under another code: a scale check cannot be a not-null
			// violation, and reading one as the other would name the wrong field.
			"a known name under another code",
			&pgconn.PgError{Code: "23502", ConstraintName: "vials_total_amount_scale_check"},
		},
		{"an error that is not the database's", errors.New("the pool is closed")},
	} {
		t.Run(unknown.name, func(t *testing.T) {
			got := classifyWrite(unknown.err)
			for _, named := range []error{
				ErrNoSuchCompound, ErrKeyNotTheirs, ErrAmountTooFine, ErrAmountOffRange,
				ErrDisposedTooEarly,
			} {
				if errors.Is(got, named) {
					t.Errorf("%v was read as %v", unknown.err, named)
				}
			}
			if !errors.Is(got, unknown.err) {
				t.Errorf("got %v, want the error itself", got)
			}
		})
	}
}

// The enum huma validates against and the set the store can mint a key for are one set written
// twice, so this reconciles them — the reason storage.ImageTypes is exported.
//
// Apart they fail in two silent ways: a type the store keeps and the tag omits is refused at
// the door although the API can hold it, and one the tag advertises and the store cannot mint
// reaches a handler that answers 422 from a branch nothing else reaches. The second is what
// this endpoint shipped with — image/webp, advertised in the contract and refused by every
// request that used it.
func TestTheAdvertisedLabelTypesAreTheOnesTheStoreCanKeep(t *testing.T) {
	field, ok := reflect.TypeOf(LabelUploadInput{}.Body).FieldByName("ContentType")
	if !ok {
		t.Fatal("LabelUploadInput has no ContentType field for the enum to sit on")
	}

	advertised := strings.Split(field.Tag.Get("enum"), ",")
	kept := storage.ImageTypes()

	for _, contentType := range kept {
		if !slices.Contains(advertised, contentType) {
			t.Errorf("the store keeps %s and the tag does not advertise it", contentType)
		}
	}
	// And the other direction by name rather than by count, so a divergence says which
	// end it is on: this is the half that shipped, and «the store keeps heic» would have
	// been a confusing way to report an advertised webp.
	for _, contentType := range advertised {
		if !slices.Contains(kept, contentType) {
			t.Errorf("the tag advertises %s and the store cannot keep it", contentType)
		}
	}
	// After the two, and not before them: a count checked first preempts both messages
	// and reports a cardinality where the name of the offending type is the useful half.
	if len(advertised) != len(kept) {
		t.Errorf("the tag advertises %v, the store keeps %v", advertised, kept)
	}
}

// The three string bounds are written twice — in the tags huma refuses at the door and in the
// CHECKs 000015 carries — and nothing reconciled them.
//
// Apart they fail the way the content type did: a tag wider than its CHECK sends the patient a
// 23514 this package does not map, which answerWrite turns into a 500 about their own form.
// Read out of the migration rather than repeated here, as storage's own key test reads its
// prefix rule.
func TestTheAdvertisedLengthsAreTheOnesTheSchemaHolds(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000015_inventory_tables.up.sql"))
	if err != nil {
		t.Fatalf("reading the migration: %v", err)
	}

	for _, bound := range []struct {
		field  string
		column string
	}{
		{"ConcentrationLabel", "concentration_label"},
		{"Lot", "lot"},
		{"LocationRU", "location_ru"},
	} {
		t.Run(bound.column, func(t *testing.T) {
			// Anchored on the length call alone: it names the column, so a leading
			// term could only move where the match starts, never what it captures.
			found := regexp.MustCompile(
				`length\(` + bound.column + `\) BETWEEN (\d+) AND (\d+)`,
			).FindSubmatch(source)
			if found == nil {
				t.Fatalf("000015 does not bound %s the way this test reads it", bound.column)
			}

			field, ok := reflect.TypeOf(NewVialBody{}).FieldByName(bound.field)
			if !ok {
				t.Fatalf("NewVialBody has no %s", bound.field)
			}
			if got, want := field.Tag.Get("minLength"), string(found[1]); got != want {
				t.Errorf("the tag admits from %q, the schema from %q", got, want)
			}
			if got, want := field.Tag.Get("maxLength"), string(found[2]); got != want {
				t.Errorf("the tag admits up to %q, the schema up to %q", got, want)
			}
		})
	}
}

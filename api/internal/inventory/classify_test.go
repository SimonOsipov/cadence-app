package inventory

import (
	"errors"
	"reflect"
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
			"a constraint nothing here maps",
			&pgconn.PgError{Code: "23514", ConstraintName: "vials_expires_on_check"},
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

	if len(advertised) != len(kept) {
		t.Fatalf("the tag advertises %v, the store keeps %v", advertised, kept)
	}
	for _, contentType := range kept {
		if !slices.Contains(advertised, contentType) {
			t.Errorf("the store keeps %s and the tag does not advertise it", contentType)
		}
	}
}

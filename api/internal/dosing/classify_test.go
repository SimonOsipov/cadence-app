package dosing

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// The constraints that answer a legitimate request, and only those. A catch-all here is how
// a bug becomes a refusal the client retries forever — and each of these three is reached in
// the one place a test cannot orchestrate, so the mapping is measured as a function.
func TestEachConstraintThatAnswersARequestIsNamed(t *testing.T) {
	for _, refusal := range []struct {
		name       string
		code       string
		constraint string
		want       error
	}{
		{"the slot another device took", "23505", "dose_events_one_per_slot", errLostTheSlot},
		{"a vial that is not theirs", "23503", "dose_events_drawn_from_their_own_vial", ErrNoSuchVial},
		{
			"a photo key outside their prefix", "23514",
			"dose_events_photo_key_is_under_its_own_prefix", ErrPhotoNotTheirs,
		},
		{
			"a dose finer than the unit's atom", "23514",
			"dose_events_dose_value_scale_check", ErrDoseTooFine,
		},
	} {
		t.Run(refusal.name, func(t *testing.T) {
			got := classify(&pgconn.PgError{Code: refusal.code, ConstraintName: refusal.constraint})
			if !errors.Is(got, refusal.want) {
				t.Errorf("got %v, want %v", got, refusal.want)
			}
		})
	}

	// The code and the name together, not either alone: the client key's uniqueness is
	// also a 23505 on this table, and it is not a lost slot.
	same := &pgconn.PgError{Code: "23505", ConstraintName: "dose_events_one_per_client_key"}
	if got := classify(same); errors.Is(got, errLostTheSlot) {
		t.Error("the client key's uniqueness was read as a lost slot")
	}

	// And everything else keeps its own error, so a bug is never a retryable refusal.
	stranger := errors.New("something nobody named")
	if got := classify(stranger); !errors.Is(got, stranger) {
		t.Errorf("an unnamed error became %v", got)
	}
	if got := classify(&pgconn.PgError{Code: "23502"}); errors.Is(got, ErrNoSuchVial) {
		t.Error("a NOT NULL violation was read as a missing vial")
	}
}

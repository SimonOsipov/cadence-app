package dosing

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/inventory"
	"github.com/SimonOsipov/cadence-app/api/internal/journal"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// ErrNoTimezone is a patient whose profile carries no zone. Every occurrence is generated in
// the patient's own day, so there is no safe default: the server's zone would put a dose on
// the wrong date for half the clinic, and refusing says which patient to fix.
var (
	ErrNoTimezone = errors.New("the patient's timezone is not recorded")

	// The same key with a different draft is a client error and not a repeat, and the
	// approved decision of 2026-08-15 says so: returning the first result silently would
	// hide the fault in the one path that exists for an unreliable network. The retry
	// queue sends what it saved, so a divergence means it saved the wrong thing.
	ErrRequestChanged = errors.New("this request key was used for a different dose")

	// The vial named is not one this patient holds. The composite key refuses it on every
	// path; this is the same refusal read as a field the caller filled in.
	ErrNoSuchVial = errors.New("no such vial in this patient's cabinet")

	// A prefix is where a key begins, and this constraint is about where it points.
	ErrPhotoNotTheirs = errors.New("the photo key is not under this patient's prefix")

	// A note of nothing. The transport drops one before it gets here, so this answers the
	// in-process callers — and it is named rather than left as a 500 because the offline
	// queue would re-send a request that always fails.
	ErrNoteSaysNothing = errors.New("a note is either absent or says something")

	// A dose finer than the unit's atom: 000021 bounds the scale because the vial
	// arithmetic is integer micrograms, and a value that rounds to zero there is a
	// vial that never empties. Named so the wizard sees a refusal rather than a 500.
	ErrDoseTooFine = errors.New("a dose is measured to the microgram, not finer")

	// The other end of the same bound, 000024. Named for the same reason: the count of
	// micrograms is int64, and a value past it saturates on one architecture and wraps
	// on the other, so the refusal has to happen before the row exists.
	ErrDoseTooLarge = errors.New("a dose is measured in milligrams, not grams")
)

// Logged is what the write answers. The identifiers are absent for every outcome but Written,
// because there is nothing to name.
type Logged struct {
	Outcome Outcome

	EventID string
	// §03's «one action, two facts». The entry has no id of its own — the day is its
	// identity — so the date is what names it.
	JournalDate civil.Date
	Dose        protocol.Dose
	VialID      *string
}

// Log records a dose and the day's diary entry in one transaction.
//
// The request seam and not the service one: a dose is the patient writing about themselves, so
// RLS answers and no audit row is written — their own row is the record, and a second under a
// service role would add only the name of a service to it.
//
// `now` is a parameter: the day is the patient's own, computed in their zone.
func Log(
	ctx context.Context, pool *pgxpool.Pool, caller database.Caller, now time.Time, draft Draft,
) (Logged, error) {
	// Lower-cased on this path; storage.NewKey does the same for the key it mints, and for
	// the same reason. IsUUIDShaped accepts a non-canonical spelling deliberately,
	// and the database answers patient_id::text in canonical lowercase — so an uppercase
	// subject would make journal.Merge's ownership comparison, which is a Go string
	// equality between the two, refuse the patient's own day.
	patient := civil.UserID(strings.ToLower(caller.Subject))

	// Before a connection is taken, and before the repeat lookup: differsFrom compares a
	// draft field by field, and a draft with no dose has nothing to compare — an
	// in-process caller reusing a key with a half-built draft would have panicked there.
	// Resolve keeps its own copy because it is pure and cannot take this path: the guard
	// belongs at the write's entry, before a connection is spent.
	if draft.incomplete() {
		return Logged{Outcome: Incomplete}, nil
	}

	var logged Logged
	err := database.WithCaller(ctx, pool, caller, func(ctx context.Context, tx pgx.Tx) error {
		// The repeat first, before anything is read: a retry from the offline queue must
		// answer the same thing without taking a second dose out of the vial, and it must
		// do so even when the plan has changed underneath it since.
		repeat, found, err := logOf(ctx, tx, patient, draft.ClientRequestID)
		if err != nil {
			return err
		}
		if found {
			if differs := repeat.differsFrom(draft); differs != "" {
				return fmt.Errorf("%s: %w", differs, ErrRequestChanged)
			}
			logged = repeat.Logged

			return nil
		}

		today, err := todayFor(ctx, tx, patient, now)
		if err != nil {
			return err
		}

		plan, running, err := protocol.ActivePlanFor(ctx, tx, patient)
		if err != nil {
			return err
		}
		if !running {
			logged = Logged{Outcome: NotScheduledToday}

			return nil
		}

		alreadyLogged, err := slotsLoggedOn(ctx, tx, patient, today)
		if err != nil {
			return err
		}

		slot, outcome := Resolve(plan, alreadyLogged, today, draft)
		if outcome != Written {
			logged = Logged{Outcome: outcome}

			return nil
		}

		// Inside a savepoint, and that is the whole of why: a constraint violation
		// aborts the transaction it happens in, so recovering from the slot race at
		// this level would answer the outcome and then fail the commit — measured,
		// pgx reports ErrTxCommitRollback, which is not a PgError and reaches the
		// caller as a 500. The nested transaction is the savepoint; rolling it back
		// leaves the outer one able to commit the nothing it has done.
		race, err := tx.Begin(ctx)
		if err != nil {
			return err
		}

		logged, err = record(ctx, race, patient, plan, slot, now, draft)
		if err != nil {
			// Joined and not replaced: a failing rollback would otherwise turn a 422
			// naming the caller's own field into an unexplained 500, and make the
			// slot race unrecoverable — the cause has to survive as the errors.Is
			// target whatever the savepoint does.
			if rollback := race.Rollback(ctx); rollback != nil {
				return errors.Join(err, rollback)
			}
			if errors.Is(err, errLostTheSlot) {
				// The other request wrote it between Resolve and the insert. The
				// reply is the one this request would have got a moment later.
				logged = Logged{Outcome: AlreadyLogged}

				return nil
			}

			return err
		}

		return race.Commit(ctx)
	})
	if err != nil {
		return Logged{}, fmt.Errorf("logging a dose for %s: %w", patient, err)
	}

	return logged, nil
}

// record writes both facts. One transaction or neither: a dose without its day loses the feed
// its «с дозой» mark, and a day without its dose is a note about an injection that is not in
// the record.
func record(
	ctx context.Context, tx pgx.Tx, patient civil.UserID,
	plan protocol.Plan, slot Slot, now time.Time, draft Draft,
) (Logged, error) {
	// The compound is the item's, resolved here and never taken from the request: a client
	// naming another drug would write an attribution nothing downstream could question.
	var compound *protocol.CompoundID
	for _, item := range plan.Items {
		if item.ID == slot.ItemID {
			compound = item.CompoundID

			break
		}
	}

	// The vial the patient named, or the one the cabinet resolves to. «Resolved, not
	// chosen» is the recorded invariant: with two open vials of one compound the server
	// leaves it empty rather than guessing, because the choice is the patient's and it
	// arrives in the request — the KMP client has drawn that picker since 2026-08-04.
	vial := draft.VialID
	switch {
	case vial != nil && compound != nil:
		// A named vial gets the same predicate the resolution uses, and not merely the
		// composite key's «is it theirs»: a dose charged to a thrown-away vial or to
		// another drug's is a wrong remaining count, which is the number the patient
		// reorders on.
		drawable, err := inventory.IsDrawableFor(ctx, tx, patient, string(*compound), *vial)
		if err != nil {
			return Logged{}, err
		}
		if !drawable {
			return Logged{}, fmt.Errorf("%s: %w", *vial, ErrNoSuchVial)
		}
	case vial == nil && compound != nil:
		resolved, err := inventory.OpenVialFor(ctx, tx, patient, string(*compound))
		if err != nil {
			return Logged{}, err
		}
		vial = resolved
	}

	sides := make([]string, 0, len(draft.Sides))
	for _, side := range draft.Sides {
		sides = append(sides, string(side))
	}

	var eventID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO app.dose_events
		    (patient_id, protocol_id, protocol_item_id, vial_id, compound_id,
		     scheduled_for_date, scheduled_for_time, injected_at,
		     dose_value, dose_unit, site_code, mood, side_effects, note, photo_path,
		     client_request_id)
		VALUES ($1, $2, $3, $4, $5, $6::date, $7::time, $8,
		        $9, $10, $11, $12, $13::text[], $14, $15, $16)
		RETURNING id::text
	`, string(patient), string(plan.Protocol.ID), string(slot.ItemID), vial, compound,
		slot.Date.String(), slotTime(slot), now, draft.Dose.Value, string(draft.Dose.Unit),
		siteCode(draft.Site), draft.Mood, sides, draft.Note, draft.PhotoPath,
		draft.ClientRequestID).Scan(&eventID); err != nil {
		return Logged{}, classify(err)
	}

	if err := writeTheDay(ctx, tx, patient, slot.Date, draft); err != nil {
		return Logged{}, err
	}

	return Logged{
		Outcome:     Written,
		EventID:     eventID,
		JournalDate: slot.Date,
		Dose:        *draft.Dose,
		VialID:      vial,
	}, nil
}

// writeTheDay is the second fact. The entry is written whatever the check-in says, including
// nothing at all: an injection is a fact on its own, which is why the schema exempts the dose
// path from «a day says something».
func writeTheDay(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, day civil.Date, draft Draft,
) error {
	existing, err := entryOn(ctx, tx, patient, day)
	if err != nil {
		return err
	}

	merged, err := journal.Merge(existing, patient, journal.CheckInDraft{
		EntryDate: day,
		Mood:      draft.Mood,
		Tags:      draft.Sides,
		Note:      draft.Note,
	}, journal.SourceDose)
	if err != nil {
		// Both of journal's refusals are programmer errors by its own account — the row
		// read and the patient written disagreeing means the read was wrong — so they
		// travel as themselves and the transport answers 500. Mapping them to a 4xx
		// would tell a patient their form is wrong about a bug in this process, and put
		// another patient's identifier in the reply on the way.
		return fmt.Errorf("merging the day: %w", err)
	}

	tags := make([]string, 0, len(merged.Tags))
	for _, tag := range merged.Tags {
		tags = append(tags, string(tag))
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO app.journal_entries
		    (patient_id, entry_date, mood, energy, sleep, tags, note, source)
		VALUES ($1, $2::date, $3, $4, $5, $6::text[], $7, $8)
		ON CONFLICT (patient_id, entry_date) DO UPDATE
		SET mood = excluded.mood, energy = excluded.energy, sleep = excluded.sleep,
		    tags = excluded.tags, note = excluded.note
	`, string(patient), day.String(), merged.Mood, merged.Energy, merged.Sleep,
		tags, merged.Note, string(merged.Source))

	return err
}

// theFirstTime is what a key already wrote, read back far enough to tell a repeat from a
// client that reused a key for a different dose.
type theFirstTime struct {
	Logged

	itemID protocol.ProtocolItemID
	site   *string
	mood   *int
	sides  []string
	note   *string
	photo  *string
}

// differsFrom names the field a repeat disagrees on, or the empty string when it is one.
//
// The draft's own meaning and not the request's bytes, which is what the decision asks for:
// a client that reordered its side effects or reformatted its JSON sent the same dose. The
// dose is compared too, and it is the field a patient is most likely to correct before the
// queue drains — a retry changing only the number would otherwise be accepted as a repeat and
// answered with the old one, which is the fault the conflict exists to stop hiding.
func (first theFirstTime) differsFrom(draft Draft) string {
	sides := make([]string, 0, len(draft.Sides))
	for _, side := range draft.Sides {
		sides = append(sides, string(side))
	}
	slices.Sort(sides)
	stored := slices.Clone(first.sides)
	slices.Sort(stored)

	switch {
	case first.itemID != draft.ItemID:
		return "protocol_item_id"
	case first.Dose != *draft.Dose:
		return "dose_value"
	// Only when this request names one. The row cannot tell «the client chose X» from
	// «the server resolved X», so a repeat that names nothing is asking for the same
	// resolution rather than disagreeing with it — comparing them unconditionally made
	// every ordinary retry a conflict, which is the fault this comparison exists to
	// avoid, one field over.
	case draft.VialID != nil && !samePointer(first.VialID, draft.VialID):
		return "vial_id"
	case !samePointer(first.site, siteCode(draft.Site)):
		return "site_code"
	case !samePointer(first.mood, draft.Mood):
		return "mood"
	case !slices.Equal(stored, sides):
		return "side_effects"
	case !samePointer(first.note, draft.Note):
		return "note"
	case !samePointer(first.photo, draft.PhotoPath):
		return "photo_path"
	default:
		return ""
	}
}

func samePointer[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return *a == *b
}

// logOf is the idempotency. The key is the client's own and unique per patient, so a repeat
// finds the row and answers what it answered before rather than resolving again.
func logOf(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, key string,
) (theFirstTime, bool, error) {
	var (
		first theFirstTime
		day   time.Time
	)
	err := tx.QueryRow(ctx, `
		SELECT id::text, scheduled_for_date, dose_value, dose_unit, vial_id::text,
		       protocol_item_id::text, site_code, mood, side_effects, note, photo_path
		FROM app.dose_events
		WHERE patient_id = $1 AND client_request_id = $2
	`, string(patient), key).Scan(
		&first.EventID, &day, &first.Dose.Value, &first.Dose.Unit, &first.VialID,
		&first.itemID, &first.site, &first.mood, &first.sides, &first.note, &first.photo,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return theFirstTime{}, false, nil
	}
	if err != nil {
		return theFirstTime{}, false, fmt.Errorf("looking up request %q: %w", key, err)
	}

	first.Outcome = Written
	first.JournalDate = civil.NewDate(day.Year(), day.Month(), day.Day())

	return first, true, nil
}

// InjectionsOf is the history the rotation reads: which zone, and when it went in.
//
// `injected_at` and never `scheduled_for_date`: a dose from the retry queue answers an
// occurrence months old, and charging its zone to that date would move the suggestion off the
// zone it should be on. `created_at` is wrong the same way, one field over.
//
// A ninety-day window, and the arithmetic is why it is safe: ten zones on a weekly injection
// come round in seventy days, so a zone inside the rotation never reads as never-used.
func InjectionsOf(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, since time.Time,
) ([]Injection, error) {
	rows, err := tx.Query(ctx, `
		SELECT site_code, injected_at
		FROM app.dose_events
		WHERE patient_id = $1 AND injected_at >= $2
		ORDER BY injected_at
	`, string(patient), since)
	if err != nil {
		return nil, fmt.Errorf("reading the injections of %s: %w", patient, err)
	}
	defer rows.Close()

	var history []Injection
	for rows.Next() {
		var (
			injection Injection
			code      *string
		)
		if err := rows.Scan(&code, &injection.At); err != nil {
			return nil, err
		}
		if code != nil {
			// Parsed and not cast: a value the schema accepts and Go does not is a
			// set that drifted, and dropping it silently would move the rotation.
			site, ok := parseSite(*code)
			if !ok {
				return nil, fmt.Errorf("a dose of %s names the zone %q", patient, *code)
			}
			injection.Site = &site
		}
		history = append(history, injection)
	}

	return history, rows.Err()
}

// RotationWindow is how far back InjectionsOf reads. Seventy days is a full ten-zone rotation
// of a weekly injection; ninety leaves room for a missed week without a zone falling out of
// the history and reading as never used.
const RotationWindow = 90 * 24 * time.Hour

// slotsLoggedOn reads what closes an occurrence, and it reads the *scheduled* slot rather
// than injected_at: a dose logged from the retry queue answers the occurrence it was taken
// for, not the moment the row arrived.
func slotsLoggedOn(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, day civil.Date,
) ([]protocol.LoggedSlot, error) {
	rows, err := tx.Query(ctx, `
		SELECT protocol_item_id::text, scheduled_for_date, scheduled_for_time
		FROM app.dose_events
		WHERE patient_id = $1 AND scheduled_for_date = $2::date
	`, string(patient), day.String())
	if err != nil {
		return nil, fmt.Errorf("reading the day's doses: %w", err)
	}
	defer rows.Close()

	var logged []protocol.LoggedSlot
	for rows.Next() {
		var (
			slot protocol.LoggedSlot
			on   time.Time
			at   *time.Time
		)
		if err := rows.Scan(&slot.ItemID, &on, &at); err != nil {
			return nil, err
		}
		slot.Date = civil.NewDate(on.Year(), on.Month(), on.Day())
		if at != nil {
			slot.Time = &civil.Slot{Hour: at.Hour(), Minute: at.Minute()}
		}
		logged = append(logged, slot)
	}

	return logged, rows.Err()
}

func entryOn(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, day civil.Date,
) (*journal.Entry, error) {
	var (
		entry  journal.Entry
		on     time.Time
		tags   []string
		source string
	)
	// FOR UPDATE because this is a read-modify-write: the merge happens in Go, and two
	// writes landing on one day — a twice-daily item, or a dose against a day being
	// written elsewhere — would both read the same row and the second would overwrite the
	// first's merge instead of merging on top of it. DO UPDATE makes that loss silent.
	err := tx.QueryRow(ctx, `
		SELECT patient_id::text, entry_date, mood, energy, sleep, tags, note, source
		FROM app.journal_entries
		WHERE patient_id = $1 AND entry_date = $2::date
		FOR UPDATE
	`, string(patient), day.String()).Scan(
		&entry.PatientID, &on, &entry.Mood, &entry.Energy, &entry.Sleep,
		&tags, &entry.Note, &source,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the day: %w", err)
	}

	entry.EntryDate = civil.NewDate(on.Year(), on.Month(), on.Day())

	// Parsed like the tags below it, and for the same reason. A stored value the schema
	// accepts and Go does not is a set that drifted; the two are reconciled by test, so
	// this cannot happen, and if it does it is louder than carrying an unknown provenance
	// into a merge that decides whether to keep it.
	parsed, ok := journal.ParseSource(source)
	if !ok {
		return nil, fmt.Errorf("the day of %v was born as %q", entry.EntryDate, source)
	}
	entry.Source = parsed

	for _, stored := range tags {
		// A value the schema accepts and Go does not is a set that drifted, and the two
		// are reconciled by test — so this cannot happen, and if it does it is louder
		// than dropping the tag and carrying on.
		tag, ok := journal.ParseTag(stored)
		if !ok {
			return nil, fmt.Errorf("the day of %v carries the tag %q", entry.EntryDate, stored)
		}
		entry.Tags = append(entry.Tags, tag)
	}

	return &entry, nil
}

// todayFor is the patient's own day, and there is no default. Every occurrence is generated
// in the patient's zone, so the server's would put a dose on the wrong date for half a clinic
// — and a patient with no zone recorded is a provisioning fault worth naming.
func todayFor(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, now time.Time,
) (civil.Date, error) {
	var zone *string
	if err := tx.QueryRow(ctx,
		`SELECT timezone FROM app.profiles WHERE user_id = $1`, string(patient)).Scan(&zone); err != nil {
		return civil.Date{}, fmt.Errorf("reading the timezone of %s: %w", patient, err)
	}
	if zone == nil || *zone == "" {
		return civil.Date{}, fmt.Errorf("%s: %w", patient, ErrNoTimezone)
	}

	where, err := time.LoadLocation(*zone)
	if err != nil {
		return civil.Date{}, fmt.Errorf("the zone %q of %s: %w", *zone, patient, err)
	}

	local := now.In(where)

	return civil.NewDate(local.Year(), local.Month(), local.Day()), nil
}

// classify turns the constraints that answer a legitimate request into named errors, and
// only those: a catch-all here is how a bug becomes a refusal the client retries forever.
//
// The slot's uniqueness is the race Resolve cannot see — two devices with different keys
// aiming at one occurrence — and the outcome set already has the word for it. Without this
// the loser got a 500 where already_logged is exactly what happened.
func classify(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}

	switch {
	case pgErr.Code == uniqueViolation && pgErr.ConstraintName == oneDosePerSlot:
		return errLostTheSlot
	case pgErr.Code == foreignKeyViolation && pgErr.ConstraintName == vialIsTheirOwn:
		return ErrNoSuchVial
	case pgErr.Code == checkViolation && pgErr.ConstraintName == photoIsUnderTheirPrefix:
		return ErrPhotoNotTheirs
	case pgErr.Code == checkViolation && pgErr.ConstraintName == noteSaysSomething:
		return ErrNoteSaysNothing
	case pgErr.Code == checkViolation && pgErr.ConstraintName == doseIsNotFinerThanTheAtom:
		return ErrDoseTooFine
	case pgErr.Code == checkViolation && pgErr.ConstraintName == doseIsUnderItsCeiling:
		return ErrDoseTooLarge
	default:
		return err
	}
}

const (
	uniqueViolation     = "23505"
	foreignKeyViolation = "23503"
	checkViolation      = "23514"

	doseIsNotFinerThanTheAtom = "dose_events_dose_value_scale_check"
	doseIsUnderItsCeiling     = "dose_events_dose_value_magnitude_check"
	oneDosePerSlot            = "dose_events_one_per_slot"
	vialIsTheirOwn            = "dose_events_drawn_from_their_own_vial"
	photoIsUnderTheirPrefix   = "dose_events_photo_key_is_under_its_own_prefix"
	noteSaysSomething         = "dose_events_note_check"
)

// errLostTheSlot never leaves this package: it is the race becoming the outcome that names
// it, and a caller has nothing to do about it that «already logged» does not already say.
var errLostTheSlot = errors.New("another request took this slot first")

func slotTime(slot Slot) *string {
	if slot.Time == nil {
		return nil
	}
	at := slot.Time.String()

	return &at
}

func siteCode(site *Site) *string {
	if site == nil {
		return nil
	}
	code := string(*site)

	return &code
}

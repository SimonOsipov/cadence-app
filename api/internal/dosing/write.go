package dosing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SimonOsipov/cadence-app/api/internal/journal"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/database"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// ErrNoTimezone is a patient whose profile carries no zone. Every occurrence is generated in
// the patient's own day, so there is no safe default: the server's zone would put a dose on
// the wrong date for half the clinic, and refusing says which patient to fix.
var ErrNoTimezone = errors.New("the patient's timezone is not recorded")

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
// The request seam and not the service one: a dose is the patient writing about themselves,
// so RLS answers and no audit row is written — the patient's own row is the record of the
// fact, and a second under a service role would add only the name of a service to it. That is
// a decision of this feature and it removed a line from the component's contract.
//
// `now` is a parameter for the reason the generator's `today` is: the day is the patient's
// own, computed in their zone, and a package that reads the clock cannot be run against a
// calendar.
func Log(
	ctx context.Context, pool *pgxpool.Pool, caller database.Caller, now time.Time, draft Draft,
) (Logged, error) {
	patient := civil.UserID(caller.Subject)

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
			logged = repeat

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

		logged, err = record(ctx, tx, patient, plan, slot, now, draft)

		return err
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
	`, string(patient), string(plan.Protocol.ID), string(slot.ItemID), draft.VialID, compound,
		slot.Date.String(), slotTime(slot), now, slot.Prescribed.Value, string(slot.Prescribed.Unit),
		siteCode(draft.Site), draft.Mood, sides, draft.Note, draft.PhotoPath,
		draft.ClientRequestID).Scan(&eventID); err != nil {
		return Logged{}, err
	}

	if err := writeTheDay(ctx, tx, patient, slot.Date, draft); err != nil {
		return Logged{}, err
	}

	return Logged{
		Outcome:     Written,
		EventID:     eventID,
		JournalDate: slot.Date,
		Dose:        *slot.Prescribed,
		VialID:      draft.VialID,
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

// logOf is the idempotency. The key is the client's own and unique per patient, so a repeat
// finds the row and answers what it answered before rather than resolving again.
func logOf(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, key string,
) (Logged, bool, error) {
	var (
		logged Logged
		day    time.Time
	)
	err := tx.QueryRow(ctx, `
		SELECT id::text, scheduled_for_date, dose_value, dose_unit, vial_id::text
		FROM app.dose_events
		WHERE patient_id = $1 AND client_request_id = $2
	`, string(patient), key).Scan(
		&logged.EventID, &day, &logged.Dose.Value, &logged.Dose.Unit, &logged.VialID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Logged{}, false, nil
	}
	if err != nil {
		return Logged{}, false, fmt.Errorf("looking up request %q: %w", key, err)
	}

	logged.Outcome = Written
	logged.JournalDate = civil.NewDate(day.Year(), day.Month(), day.Day())

	return logged, true, nil
}

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
		entry journal.Entry
		on    time.Time
		tags  []string
	)
	err := tx.QueryRow(ctx, `
		SELECT patient_id::text, entry_date, mood, energy, sleep, tags, note, source
		FROM app.journal_entries
		WHERE patient_id = $1 AND entry_date = $2::date
	`, string(patient), day.String()).Scan(
		&entry.PatientID, &on, &entry.Mood, &entry.Energy, &entry.Sleep,
		&tags, &entry.Note, &entry.Source)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the day: %w", err)
	}

	entry.EntryDate = civil.NewDate(on.Year(), on.Month(), on.Day())
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

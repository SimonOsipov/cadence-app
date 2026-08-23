package dosing

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// The two questions protocol asks this context, answered where the tables are.
//
// They live here and the interfaces live there, which is the direction the dependency has to
// run: dosing imports protocol for the plan and the occurrence, so protocol cannot import
// dosing back. What crosses the boundary is a zone and a list of slots — never a dose event.
type History struct {
	// The clock, injected for the reason it is everywhere in this feature: the rotation
	// reads a window back from now, and a package that reads the clock cannot be run
	// against a calendar.
	now func() time.Time
}

func NewHistory(now func() time.Time) *History { return &History{now: now} }

// SuggestNextSite is the zone the wizard's site step opens on, computed from what the patient
// logged rather than frozen — which is what the prototype ships and §03 asks to replace.
func (h *History) SuggestNextSite(
	ctx context.Context, tx pgx.Tx, patient civil.UserID,
) (string, error) {
	history, err := InjectionsOf(ctx, tx, patient, h.now().Add(-RotationWindow))
	if err != nil {
		return "", err
	}

	return string(SuggestNextSite(history)), nil
}

// LoggedSlotsIn is what closes an occurrence, over a window of days.
//
// The scheduled slot and not injected_at, which is the opposite of the rotation's read and
// deliberately so: an occurrence is closed by the dose taken *for* it, and a back-filled dose
// answers the day it was prescribed for rather than the day it arrived.
func (h *History) LoggedSlotsIn(
	ctx context.Context, tx pgx.Tx, patient civil.UserID, window civil.Range,
) ([]protocol.LoggedSlot, error) {
	rows, err := tx.Query(ctx, `
		SELECT protocol_item_id::text, scheduled_for_date, scheduled_for_time
		FROM app.dose_events
		WHERE patient_id = $1 AND scheduled_for_date BETWEEN $2::date AND $3::date
	`, string(patient), window.From.String(), window.Through.String())
	if err != nil {
		return nil, fmt.Errorf("reading the logged doses of %s: %w", patient, err)
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

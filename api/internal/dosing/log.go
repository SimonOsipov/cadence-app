package dosing

import (
	"github.com/SimonOsipov/cadence-app/api/internal/journal"
	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// SideEffect is journal's tag under the name §03 gives it here. One set and not two: the
// same seven reach the diary and the dose event from one patient action, and a second
// declaration is a second thing to forget to change. It lives with the write that first
// carries it — step 5 declared it early and left it unused, which review called out.
type SideEffect = journal.Tag

// Outcome is what a dose write can answer, and the set is closed. A nullable id would make
// «the day rolled over», «already logged» and «the form is not filled in» one answer, and
// they are three different things for a screen to say.
//
// They travel as a 200 with a body rather than as HTTP errors: the client already branches
// on them, so they are expected results and not failures. problem+json stays for real ones.
type Outcome string

const (
	Written           Outcome = "written"
	Incomplete        Outcome = "incomplete"
	NotScheduledToday Outcome = "not_scheduled_today"
	AlreadyLogged     Outcome = "already_logged"
)

// Draft is the wizard's payload after the transport has parsed it. Everything but the item,
// the dose and the client's key is optional — §03's step 4 is «всё по желанию».
type Draft struct {
	ItemID protocol.ProtocolItemID
	Dose   *protocol.Dose

	VialID    *string
	Site      *Site
	Mood      *int
	Sides     []SideEffect
	Note      *string
	PhotoPath *string

	// The client's own, and the whole of the idempotency: a retry from the offline queue
	// carries the key generated when the patient tapped save.
	ClientRequestID string
}

// Slot is the occurrence a draft resolved to, and what the course prescribes for it.
type Slot struct {
	ItemID protocol.ProtocolItemID
	Date   civil.Date

	// Never absent in practice: protocol_items_has_a_slot keeps the array non-empty and
	// the generator walks it, so every occurrence carries a time. A pointer because the
	// column is nullable and the row shape has to be able to say so.
	Time *civil.Slot

	// What the course prescribes for that occurrence. Not what is stored — the event
	// records what the patient took — and read by nothing in this package: it is here for
	// the aggregates of step 9, which show the prescription beside the entry. The
	// comparison the endpoint's text promises is made downstream, off the two tables.
	Prescribed *protocol.Dose
}

// Resolve decides which occurrence a draft answers, or why it answers none.
//
// Pure, and separate from the write for the reason protocol's generator is: the four outcomes
// are a decision about a plan and a history, and testing them should not need a database.
func Resolve(plan protocol.Plan, logged []protocol.LoggedSlot, today civil.Date, draft Draft) (Slot, Outcome) {
	if draft.ItemID == "" || draft.ClientRequestID == "" ||
		draft.Dose == nil || draft.Dose.Value <= 0 || draft.Dose.Unit == "" {
		return Slot{}, Incomplete
	}

	// loggable is asked here and not only where the item was chosen: a caller building a
	// draft by hand never passes through the chooser.
	loggable := false
	for _, item := range plan.Items {
		if item.ID == draft.ItemID && item.Loggable {
			loggable = true

			break
		}
	}
	if !loggable {
		return Slot{}, NotScheduledToday
	}

	var todays []protocol.Occurrence
	for _, occurrence := range protocol.OccurrencesFor(plan, logged, today, today) {
		if occurrence.ItemID == draft.ItemID {
			todays = append(todays, occurrence)
		}
	}
	if len(todays) == 0 {
		return Slot{}, NotScheduledToday
	}

	// The first slot still open, and not the first slot: §03 gives BPC-157 two daily
	// times, so «the first occurrence» would tell a patient the evening dose was already
	// recorded after they had logged only the morning one.
	for _, occurrence := range todays {
		if occurrence.Status == protocol.StatusDone {
			continue
		}

		at := occurrence.Time

		return Slot{
			ItemID:     occurrence.ItemID,
			Date:       occurrence.Date,
			Time:       &at,
			Prescribed: occurrence.Dose,
		}, Written
	}

	// §03 has one event per occurrence: re-tapping «Сохранить» must not take another dose
	// out of the vial.
	return Slot{}, AlreadyLogged
}

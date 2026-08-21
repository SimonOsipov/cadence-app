package protocol

// ProtocolRow is one line of the week's prescription strip. TodayStatus is nil when the item
// is not due today at all — a claim about nothing.
type ProtocolRow struct {
	ItemID ProtocolItemID
	Kind   ItemKind
	// Resolved here rather than left as an id: the Today row needs the name and is not
	// allowed a repository to look it up in.
	Compound    *Compound
	Dose        *Dose
	Times       []Slot
	Cadence     Cadence
	TodayStatus *OccurrenceStatus
	Loggable    bool
}

// WeekProtocolRows builds from the plan's items, not from the day's occurrences: the strip
// is the week's prescription, so the weekly injection belongs on it every day. Today's state
// comes from OccurrencesFor — the same call the schedule makes, so the two cannot disagree
// by construction.
func WeekProtocolRows(plan Plan, compounds []Compound, logged []LoggedSlot, today Date) []ProtocolRow {
	if plan.Protocol.Status == StatusCancelled {
		return nil
	}
	if _, ok := CycleWeek(plan.Protocol, today); !ok {
		return nil
	}
	todays := OccurrencesFor(plan, logged, today, today)

	rows := make([]ProtocolRow, 0, len(plan.Items))
	for _, item := range plan.Items {
		var mine []Occurrence
		for _, o := range todays {
			if o.ItemID == item.ID {
				mine = append(mine, o)
			}
		}
		rows = append(rows, ProtocolRow{
			ItemID:      item.ID,
			Kind:        item.Kind,
			Compound:    compoundOf(compounds, item.CompoundID),
			Dose:        PhaseDose(plan, item.ID, today),
			Times:       item.Times,
			Cadence:     item.Cadence,
			TodayStatus: leastComplete(mine),
			Loggable:    item.Loggable,
		})
	}
	return rows
}

func compoundOf(compounds []Compound, id *CompoundID) *Compound {
	if id == nil {
		return nil
	}
	for i := range compounds {
		if compounds[i].ID == *id {
			return &compounds[i]
		}
	}
	return nil
}

// Not the first slot: reading it collapsed a multi-slot item, and logging BPC-157's 08:00
// dose read «Сегодня · записано» while the 20:00 injection was still due. Done only when
// every slot is done, else whatever the first slot that is not says.
func leastComplete(occurrences []Occurrence) *OccurrenceStatus {
	if len(occurrences) == 0 {
		return nil
	}
	for _, o := range occurrences {
		if o.Status != StatusDone {
			status := o.Status
			return &status
		}
	}
	done := StatusDone
	return &done
}

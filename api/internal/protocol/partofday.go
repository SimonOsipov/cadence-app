package protocol

import "github.com/SimonOsipov/cadence-app/api/internal/platform/civil"

// PartOfDay is the half of «Воскресенье, утро» that is not the weekday: a rule about the
// clock, computed here rather than on the screen so both surfaces say the same word.
type PartOfDay string

const (
	Night     PartOfDay = "night"
	Morning   PartOfDay = "morning"
	Afternoon PartOfDay = "afternoon"
	Evening   PartOfDay = "evening"
)

// The boundaries Russian actually uses, and not even quarters: «ночь» to five, «утро» to
// noon, «день» to six, «вечер» to midnight. Ported from the KMP rather than rounded.
const (
	morningFrom   = 5
	afternoonFrom = 12
	eveningFrom   = 18
)

// PartOfDayAt is the word for a time of day. The slot is the patient's own — the caller
// computes it in their zone, the way every day in this feature is computed.
func PartOfDayAt(at civil.Slot) PartOfDay {
	switch {
	case at.Hour >= morningFrom && at.Hour < afternoonFrom:
		return Morning
	case at.Hour >= afternoonFrom && at.Hour < eveningFrom:
		return Afternoon
	case at.Hour >= eveningFrom:
		return Evening
	default:
		return Night
	}
}

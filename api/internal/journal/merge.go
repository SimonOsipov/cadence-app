package journal

import (
	"slices"
	"strings"

	"github.com/SimonOsipov/cadence-app/api/internal/protocol"
)

// Tag is §03's «tags[] (7 fixed)», and the seven are the side effects.
//
// The closed set lives here rather than in dosing, which is where the KMP puts it —
// there it is one enum with a typealias, because two sets that must never differ are
// one set, and Kotlin has no import direction to protect. Go does: one patient action
// writes a dose event and a diary entry, so dosing reads journal and not the reverse,
// and the set has to sit at the end the write reaches second.
type Tag string

const (
	TagNausea   Tag = "nausea"
	TagFatigue  Tag = "fatigue"
	TagHeadache Tag = "headache"
	TagBloating Tag = "bloating"
	TagInsomnia Tag = "insomnia"
	TagSite     Tag = "site"
	TagAppetite Tag = "appetite"
)

// Source is §03's `source manual|dose` — the dose wizard's check-in writes one of
// these, and the feed marks the entry by it.
type Source string

const (
	SourceManual Source = "manual"
	SourceDose   Source = "dose"
)

// Entry is one day, and the day is the identity: a write updates it rather than
// adding a second.
type Entry struct {
	PatientID protocol.UserID
	EntryDate protocol.Date
	Mood      *int
	Energy    *int
	Sleep     *int
	Tags      []Tag
	Note      *string
	Source    Source
}

// CheckInDraft carries no source: the path decides it, so a caller cannot sign its
// own write. Every field is optional — unnamed means «skipped», not «erase».
type CheckInDraft struct {
	EntryDate protocol.Date
	Mood      *int
	Energy    *int
	Sleep     *int
	Tags      []Tag
	Note      *string
}

// SaysNothing is what stops a day the patient never filled from reaching the feed and
// the heatmap. A note of spaces says nothing, like an unanswered reading.
func (d CheckInDraft) SaysNothing() bool {
	return d.Mood == nil && d.Energy == nil && d.Sleep == nil &&
		len(d.Tags) == 0 && blank(d.Note)
}

// ReadingsAreOnTheScale guards a loss that is otherwise silent: the client maps a
// number outside 1..5 to «no answer», so a stored 7 reads back as nothing said.
func (d CheckInDraft) ReadingsAreOnTheScale() bool {
	for _, reading := range []*int{d.Mood, d.Energy, d.Sleep} {
		if reading != nil && (*reading < 1 || *reading > 5) {
			return false
		}
	}

	return true
}

// Merge applies a draft to whatever the day already holds. Four rules, and each is a
// decision rather than an accident:
//
//   - a named reading overrides, an unnamed one keeps what was there;
//   - a blank note is a skipped note;
//   - tags accumulate without duplicates — a side effect reported this morning did
//     not stop being true by evening;
//   - provenance is set once, so «born of a dose» stays true of the day it was born
//     on however it is edited afterwards.
//
// Nothing is written through to `existing`: the caller holds the row it read, and a
// merge that edited it in place would leave the two disagreeing if the write failed.
func Merge(existing *Entry, patient protocol.UserID, draft CheckInDraft, bornAs Source) Entry {
	merged := Entry{
		PatientID: patient,
		EntryDate: draft.EntryDate,
		Mood:      draft.Mood,
		Energy:    draft.Energy,
		Sleep:     draft.Sleep,
		Tags:      slices.Clone(draft.Tags),
		Source:    bornAs,
	}
	if !blank(draft.Note) {
		merged.Note = draft.Note
	}

	if existing == nil {
		return merged
	}

	if merged.Mood == nil {
		merged.Mood = existing.Mood
	}
	if merged.Energy == nil {
		merged.Energy = existing.Energy
	}
	if merged.Sleep == nil {
		merged.Sleep = existing.Sleep
	}
	if merged.Note == nil {
		merged.Note = existing.Note
	}
	merged.Source = existing.Source

	// The earlier tags first and in their own order, so that two reads of one day
	// list them the same way. A set would lose that.
	tags := slices.Clone(existing.Tags)
	for _, tag := range draft.Tags {
		if !slices.Contains(tags, tag) {
			tags = append(tags, tag)
		}
	}
	merged.Tags = tags

	return merged
}

func blank(s *string) bool { return s == nil || strings.TrimSpace(*s) == "" }

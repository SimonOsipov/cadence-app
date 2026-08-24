package journal

import (
	"errors"
	"slices"
	"strings"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
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

// Merging into a day that is not the one being written is a programmer error and
// not a rejection: no caller has a sensible response to either.
var (
	ErrAnotherPatientsDay = errors.New("a draft merges only into its own patient's entry")
	ErrAnotherDay         = errors.New("a draft merges only into the entry for its own day")
)

// Entry is one day, and the day is the identity: a write updates it rather than
// adding a second.
type Entry struct {
	PatientID civil.UserID
	EntryDate civil.Date
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
	EntryDate civil.Date
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

// Merge applies a draft to whatever the day already holds. Two of its rules are not
// obvious from the code: tags accumulate without duplicates, because a side effect
// reported this morning did not stop being true by evening; and provenance is set
// once, so «born of a dose» stays true of the day however it is edited afterwards.
//
// `patient` is the token subject and never a body field: it is what the two refusals
// below compare against, so a body field would decide ownership from the same place
// that is trying to cross it. Nothing is written through to `existing`.
func Merge(existing *Entry, patient civil.UserID, draft CheckInDraft, bornAs Source) (Entry, error) {
	merged := Entry{
		PatientID: patient,
		EntryDate: draft.EntryDate,
		Mood:      draft.Mood,
		Energy:    draft.Energy,
		Sleep:     draft.Sleep,
		Source:    bornAs,
	}
	if !blank(draft.Note) {
		merged.Note = draft.Note
	}

	if existing == nil {
		// De-duplicated here too, and not only where the two lists meet. The Kotlin
		// runs distinct() over the whole concatenation, so it collapses a repeat
		// inside the draft as readily as one across the join; taking the draft's
		// slice whole would store «тошнота, тошнота» on a first check-in and keep it
		// forever, because every later merge finds the tag already present.
		merged.Tags = distinct(nil, draft.Tags)

		return merged, nil
	}

	// Ownership comes from `patient` and content from `existing`, so the two disagreeing
	// is a cross-tenant read laundered into a write nothing downstream can tell from a
	// legitimate one — and the service seam reads this table USING (true), so fetching the
	// day by date alone produces exactly that `existing`.
	//
	// Neither error carries the ids it compared: both are errors.Is-able, and a handler
	// mapping them to a 4xx would put the other patient's identifier in this one's reply.
	if existing.PatientID != patient {
		return Entry{}, ErrAnotherPatientsDay
	}
	if existing.EntryDate != draft.EntryDate {
		return Entry{}, ErrAnotherDay
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
	merged.Tags = distinct(existing.Tags, draft.Tags)

	return merged, nil
}

// distinct concatenates and keeps the first occurrence of each, which is what the
// Kotlin's distinct() does — including over a row that already holds a repeat,
// written before this rule was enforced.
func distinct(existing, draft []Tag) []Tag {
	// Empty rather than nil: pgx encodes a nil slice as SQL NULL, and the column is
	// NOT NULL DEFAULT '{}', so a day with only a mood would fail with 23502.
	tags := []Tag{}
	for _, tag := range slices.Concat(existing, draft) {
		if !slices.Contains(tags, tag) {
			tags = append(tags, tag)
		}
	}

	return tags
}

func blank(s *string) bool { return s == nil || strings.TrimSpace(*s) == "" }

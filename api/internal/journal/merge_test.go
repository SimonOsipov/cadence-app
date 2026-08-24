package journal

import (
	"errors"
	"testing"
	"time"

	"github.com/SimonOsipov/cadence-app/api/internal/platform/civil"
)

var (
	patient      = civil.UserID("p-1")
	otherPatient = civil.UserID("p-2")
	day          = civil.NewDate(2026, time.May, 31)
)

func text(s string) *string { return &s }

func number(n int) *int { return &n }

func draftWith(fill func(*CheckInDraft)) CheckInDraft {
	draft := CheckInDraft{EntryDate: day}
	fill(&draft)

	return draft
}

func TestADayThatSaysNothingIsRefused(t *testing.T) {
	// A written-empty entry would put a day the patient never filled into the feed
	// and into the heatmap.
	for _, c := range []struct {
		name  string
		draft CheckInDraft
		empty bool
	}{
		{"nothing at all", draftWith(func(*CheckInDraft) {}), true},
		{"a note of spaces", draftWith(func(d *CheckInDraft) { d.Note = text("   ") }), true},
		{"an empty tag list", draftWith(func(d *CheckInDraft) { d.Tags = []Tag{} }), true},
		{"a mood", draftWith(func(d *CheckInDraft) { d.Mood = number(3) }), false},
		{"a note", draftWith(func(d *CheckInDraft) { d.Note = text("тяжело") }), false},
		{"one tag", draftWith(func(d *CheckInDraft) { d.Tags = []Tag{TagNausea} }), false},
	} {
		if got := c.draft.SaysNothing(); got != c.empty {
			t.Errorf("%s: says nothing = %v, want %v", c.name, got, c.empty)
		}
	}
}

func TestAReadingOffTheScaleIsRefused(t *testing.T) {
	// Off the 1..5 scale the loss is silent: the client maps an unknown number to
	// «no answer», so a stored 7 reads back as nothing said.
	for _, c := range []struct {
		name    string
		draft   CheckInDraft
		onScale bool
	}{
		{"the bottom of the scale", draftWith(func(d *CheckInDraft) { d.Mood = number(1) }), true},
		{"the top of the scale", draftWith(func(d *CheckInDraft) { d.Mood = number(5) }), true},
		{"one below", draftWith(func(d *CheckInDraft) { d.Mood = number(0) }), false},
		{"one above", draftWith(func(d *CheckInDraft) { d.Mood = number(6) }), false},
		{"energy off the scale", draftWith(func(d *CheckInDraft) { d.Energy = number(9) }), false},
		{"sleep off the scale", draftWith(func(d *CheckInDraft) { d.Sleep = number(-1) }), false},
		{"nothing named at all", draftWith(func(*CheckInDraft) {}), true},
	} {
		if got := c.draft.ReadingsAreOnTheScale(); got != c.onScale {
			t.Errorf("%s: on the scale = %v, want %v", c.name, got, c.onScale)
		}
	}
}

func TestAnEmptyDayTakesTheDraftWhole(t *testing.T) {
	draft := draftWith(func(d *CheckInDraft) {
		d.Mood = number(2)
		d.Energy = number(3)
		d.Sleep = number(4)
		d.Tags = []Tag{TagNausea}
		d.Note = text("тяжело")
	})

	entry := merged(t, nil, patient, draft, SourceDose)

	if entry.PatientID != patient || entry.EntryDate != day {
		t.Errorf("the entry is %v on %v, want %v on %v", entry.PatientID, entry.EntryDate, patient, day)
	}
	if *entry.Mood != 2 || *entry.Energy != 3 || *entry.Sleep != 4 || *entry.Note != "тяжело" {
		t.Errorf("the readings did not arrive: %v", entry)
	}
	if len(entry.Tags) != 1 || entry.Tags[0] != TagNausea {
		t.Errorf("the tags did not arrive: %v", entry.Tags)
	}
	if entry.Source != SourceDose {
		t.Errorf("born as %v, want %v", entry.Source, SourceDose)
	}
}

func TestASecondCheckInOverwritesWhatItNamesAndKeepsWhatItDoesNot(t *testing.T) {
	// Step 4 of the wizard is «всё по желанию»: an unanswered field means «пропущено»,
	// not «erase what an earlier one said».
	morning := merged(t, nil, patient, draftWith(func(d *CheckInDraft) {
		d.Mood = number(2)
		d.Note = text("тяжело")
	}), SourceDose)

	evening := merged(t, &morning, patient, draftWith(func(d *CheckInDraft) {
		d.Mood = number(5)
	}), SourceManual)

	if *evening.Mood != 5 {
		t.Errorf("mood is %d, want the latest answer 5", *evening.Mood)
	}
	if evening.Note == nil || *evening.Note != "тяжело" {
		t.Errorf("note is %v, want what the morning said", evening.Note)
	}
}

func TestANoteOfNothingDoesNotEraseTheOneThatWasThere(t *testing.T) {
	// A blank note is «skipped», like an unanswered reading, and not «erase».
	morning := merged(t, nil, patient, draftWith(func(d *CheckInDraft) { d.Note = text("тяжело") }), SourceManual)

	for _, blank := range []*string{nil, text(""), text("   ")} {
		evening := merged(t, &morning, patient, draftWith(func(d *CheckInDraft) {
			d.Mood = number(3)
			d.Note = blank
		}), SourceManual)

		if evening.Note == nil || *evening.Note != "тяжело" {
			t.Errorf("a blank note erased the earlier one: %v", evening.Note)
		}
	}
}

func TestOneSideEffectReportedTwiceInADayIsOneTag(t *testing.T) {
	morning := merged(t, nil, patient, draftWith(func(d *CheckInDraft) {
		d.Tags = []Tag{TagNausea}
	}), SourceDose)

	evening := merged(t, &morning, patient, draftWith(func(d *CheckInDraft) {
		d.Tags = []Tag{TagNausea, TagFatigue}
	}), SourceDose)

	// Order matters as well as membership: the feed lists them, and a set that
	// reordered itself between two reads of one day would read as a change.
	want := []Tag{TagNausea, TagFatigue}
	if len(evening.Tags) != len(want) {
		t.Fatalf("tags are %v, want %v", evening.Tags, want)
	}
	for i, tag := range want {
		if evening.Tags[i] != tag {
			t.Errorf("tag %d is %v, want %v", i, evening.Tags[i], tag)
		}
	}
}

func TestASideEffectReportedThisMorningDidNotStopBeingTrueByEvening(t *testing.T) {
	morning := merged(t, nil, patient, draftWith(func(d *CheckInDraft) {
		d.Tags = []Tag{TagNausea}
	}), SourceDose)

	evening := merged(t, &morning, patient, draftWith(func(d *CheckInDraft) {
		d.Tags = []Tag{TagHeadache}
	}), SourceDose)

	if len(evening.Tags) != 2 || evening.Tags[0] != TagNausea || evening.Tags[1] != TagHeadache {
		t.Errorf("tags are %v, want both the morning's and the evening's", evening.Tags)
	}
}

func TestProvenanceIsSetOnceAndNeverRewritten(t *testing.T) {
	// «Born of a dose» stays true of the day it was born on: a later edit by hand does
	// not make it untrue, and the feed marks the entry by it.
	born := merged(t, nil, patient, draftWith(func(d *CheckInDraft) { d.Mood = number(2) }), SourceDose)

	edited := merged(t, &born, patient, draftWith(func(d *CheckInDraft) { d.Mood = number(4) }), SourceManual)

	if edited.Source != SourceDose {
		t.Errorf("source is %v, want the one it was born with", edited.Source)
	}

	// And the other direction, so this cannot pass on a function that always answers
	// «dose»: a day begun by hand stays manual when the wizard writes it later.
	byHand := merged(t, nil, patient, draftWith(func(d *CheckInDraft) { d.Mood = number(2) }), SourceManual)
	byWizard := merged(t, &byHand, patient, draftWith(func(d *CheckInDraft) { d.Mood = number(4) }), SourceDose)

	if byWizard.Source != SourceManual {
		t.Errorf("source is %v, want manual", byWizard.Source)
	}
}

func TestMergingDoesNotWriteThroughToTheDayItWasGiven(t *testing.T) {
	// The caller holds the row it read; a merge that edited it in place would leave
	// the two disagreeing if the write then failed.
	morning := merged(t, nil, patient, draftWith(func(d *CheckInDraft) {
		d.Mood = number(2)
		d.Tags = []Tag{TagNausea}
	}), SourceDose)

	merged(t, &morning, patient, draftWith(func(d *CheckInDraft) {
		d.Mood = number(5)
		d.Tags = []Tag{TagHeadache}
	}), SourceManual)

	if *morning.Mood != 2 {
		t.Errorf("the earlier entry now reads %d", *morning.Mood)
	}
	if len(morning.Tags) != 1 || morning.Tags[0] != TagNausea {
		t.Errorf("the earlier entry's tags are now %v", morning.Tags)
	}
}

func TestEveryReadingKeepsWhatTheDraftDoesNotName(t *testing.T) {
	// Mood and the note carried this rule alone, so energy and sleep were unmeasured:
	// deleting either one's carry-over left the suite green. Three readings, each
	// named on its own, so no one of them can pass for the others.
	morning := merged(t, nil, patient, draftWith(func(d *CheckInDraft) {
		d.Mood = number(2)
		d.Energy = number(3)
		d.Sleep = number(4)
	}), SourceManual)

	// Read back by value and not by presence. The morning's three are distinct on
	// purpose, so a carry-over wired to the wrong field — Energy taking Sleep — is
	// green against `!= nil` and fails here.
	type reading struct {
		name string
		of   func(Entry) *int
		was  int
	}
	mood := reading{"mood", func(e Entry) *int { return e.Mood }, 2}
	energy := reading{"energy", func(e Entry) *int { return e.Energy }, 3}
	sleep := reading{"sleep", func(e Entry) *int { return e.Sleep }, 4}

	for _, c := range []struct {
		name      string
		fill      func(*CheckInDraft)
		named     func(Entry) *int
		untouched []reading
	}{
		{"mood", func(d *CheckInDraft) { d.Mood = number(5) }, mood.of, []reading{energy, sleep}},
		{"energy", func(d *CheckInDraft) { d.Energy = number(5) }, energy.of, []reading{mood, sleep}},
		{"sleep", func(d *CheckInDraft) { d.Sleep = number(5) }, sleep.of, []reading{mood, energy}},
	} {
		evening := merged(t, &morning, patient, draftWith(c.fill), SourceManual)

		if got := c.named(evening); got == nil || *got != 5 {
			t.Errorf("%s named: got %v, want 5", c.name, got)
		}
		for _, untouched := range c.untouched {
			if got := untouched.of(evening); got == nil || *got != untouched.was {
				t.Errorf("%s named: %s reads %v, want the morning's %d",
					c.name, untouched.name, got, untouched.was)
			}
		}
	}
}

func TestTheEntryDoesNotShareItsTagsWithTheDraft(t *testing.T) {
	// The empty-day branch builds its own slice through distinct(); were it to take
	// the draft's whole, the caller's draft and the entry it got back would be one
	// array, and a caller that reuses a draft would change a row already written.
	draft := draftWith(func(d *CheckInDraft) { d.Tags = []Tag{TagNausea} })

	entry := merged(t, nil, patient, draft, SourceManual)
	draft.Tags[0] = TagHeadache

	if len(entry.Tags) != 1 || entry.Tags[0] != TagNausea {
		t.Errorf("the entry followed the draft: %v", entry.Tags)
	}
}

// merged is Merge with the programmer-error branch asserted away, so the tests below
// read as they did before the branch existed.
func merged(t *testing.T, existing *Entry, patient civil.UserID, draft CheckInDraft, bornAs Source) Entry {
	t.Helper()

	entry, err := Merge(existing, patient, draft, bornAs)
	if err != nil {
		t.Fatalf("merging: %v", err)
	}

	return entry
}

func TestMergingIntoSomebodyElsesDayIsRefused(t *testing.T) {
	// The ownership decision is made on `patient` and the content comes from
	// `existing`; the two disagreeing would launder a cross-tenant read into a
	// cross-tenant write, under an owner the policies accept. The service seam reads
	// this table with USING (true), so a repository fetching the day by date alone
	// would hand exactly this pair in.
	theirs := merged(t, nil, otherPatient, draftWith(func(d *CheckInDraft) {
		d.Note = text("их заметка")
		d.Tags = []Tag{TagNausea}
	}), SourceManual)

	entry, err := Merge(&theirs, patient, draftWith(func(d *CheckInDraft) {
		d.Mood = number(3)
	}), SourceManual)

	if !errors.Is(err, ErrAnotherPatientsDay) {
		t.Fatalf("got %v, want ErrAnotherPatientsDay", err)
	}
	// And nothing of theirs came back on the error path: an empty Entry, not a
	// half-merged one a careless caller might still write.
	if entry.Note != nil || len(entry.Tags) != 0 || entry.PatientID != "" {
		t.Errorf("the refused merge carried something out: %v", entry)
	}
}

func TestMergingYesterdayIntoTodayIsRefused(t *testing.T) {
	yesterday := merged(t, nil, patient, CheckInDraft{
		EntryDate: day.AddDays(-1),
		Mood:      number(2),
	}, SourceManual)

	if _, err := Merge(&yesterday, patient, draftWith(func(d *CheckInDraft) {
		d.Mood = number(4)
	}), SourceManual); !errors.Is(err, ErrAnotherDay) {
		t.Fatalf("got %v, want ErrAnotherDay", err)
	}
}

func TestADraftThatRepeatsATagStoresItOnce(t *testing.T) {
	// The empty-day path took the draft's slice whole, so a first check-in carrying
	// the same tag twice stored it twice — and kept it forever, because every later
	// merge finds it already present. The Kotlin's distinct() runs over the whole
	// concatenation and collapses this case too.
	entry := merged(t, nil, patient, draftWith(func(d *CheckInDraft) {
		d.Tags = []Tag{TagNausea, TagFatigue, TagNausea}
	}), SourceManual)

	want := []Tag{TagNausea, TagFatigue}
	if len(entry.Tags) != len(want) || entry.Tags[0] != want[0] || entry.Tags[1] != want[1] {
		t.Errorf("tags are %v, want %v", entry.Tags, want)
	}

	// And a row that already holds a repeat — written before the rule existed — is
	// collapsed rather than preserved.
	stale := Entry{PatientID: patient, EntryDate: day, Tags: []Tag{TagSite, TagSite}, Source: SourceManual}
	cleaned := merged(t, &stale, patient, draftWith(func(d *CheckInDraft) {
		d.Mood = number(3)
	}), SourceManual)

	if len(cleaned.Tags) != 1 || cleaned.Tags[0] != TagSite {
		t.Errorf("a stale repeat survived: %v", cleaned.Tags)
	}
}

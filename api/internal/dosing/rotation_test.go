package dosing

import (
	"slices"
	"testing"
	"time"
)

// scheduledFor and createdAt are frozen to one value for every event in the KMP suite, so
// that a rotation reaching for a neighbouring field sees one value across the history and
// fails. Here only injectedAt exists at all — the type carries nothing else — which makes the
// same point structurally: this function cannot read a field it was not given.

// The KMP writes these as named parameters with defaults, which is looser rather than tighter:
// at(month =, day =, hour =) compiles and takes minute = 0 silently. Here a half-given clock is
// refused instead: at(5, 20, 9) reads as nine o'clock and meant seven.
func at(month, day int, clock ...int) time.Time {
	hour, minute := 7, 0
	switch len(clock) {
	case 0:
	case 2:
		hour, minute = clock[0], clock[1]
	default:
		panic("at takes an hour and a minute, or neither")
	}

	return time.Date(2026, time.Month(month), day, hour, minute, 0, 0, time.UTC)
}

func on(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 7, 0, 0, 0, time.UTC)
}

func injected(site *Site, when time.Time) Injection {
	return Injection{Site: site, At: when}
}

func zone(site Site) *Site { return &site }

// Neither the set's order nor its reverse, on purpose: a function ignoring its input could
// only answer with a fixed constant, and the oldest zone here is Sites()[7]. The KMP's
// comment says the newest sits in the middle too — it does not, it is Sites()[0] — and the
// fixture works anyway because no test expects the newest zone in an answer.
var newestFirst = []Site{
	SiteLeftAbdomen,
	SiteRightAbdomen,
	SiteLeftDeltoid,
	SiteRightDeltoid,
	SiteLeftLowBack,
	SiteRightLowBack,
	SiteLeftGlute,
	SiteRightGlute,
	SiteLeftThigh,
	SiteRightThigh,
}

var (
	oldest       = newestFirst[len(newestFirst)-1]
	secondOldest = newestFirst[len(newestFirst)-2]
)

// everyZoneOnce injects each zone once, thirty days apart, in newestFirst order.
func everyZoneOnce() []Injection {
	history := make([]Injection, 0, len(newestFirst))
	for i, site := range newestFirst {
		history = append(history, injected(zone(site), on(2025, 10, 15).AddDate(0, 0, -i*30)))
	}

	return history
}

func TestTheSharedHistoryInjectsEveryZoneExactlyOnce(t *testing.T) {
	// Guards the fixture the rest of the file leans on: adding a zone to the set without
	// updating newestFirst would leave it unused throughout, silently.
	declared := slices.Clone(Sites())
	fixture := slices.Clone(newestFirst)
	slices.Sort(declared)
	slices.Sort(fixture)

	if !slices.Equal(declared, fixture) {
		t.Errorf("the fixture covers %v, the set is %v", fixture, declared)
	}
	if len(newestFirst) != len(Sites()) {
		t.Errorf("a zone repeats in the fixture: %d against %d", len(newestFirst), len(Sites()))
	}
}

// The order in full, and by literal. It is the rotation's tie-break, so it decides what a
// patient with no history is offered and which of two equally stale zones wins — and only
// the first two positions were pinned: measured, permuting the other eight left the whole
// package green. The fixture guard above sorts both sides, which erases exactly this.
func TestTheZonesAreInTheOrderTheTieBreakNeeds(t *testing.T) {
	// The KMP enum's declaration order, and the same order the CHECK in 000019 lists.
	// Not the prototype's front-six-then-back-four, which is its panels' drawing order.
	want := []Site{
		SiteLeftAbdomen, SiteRightAbdomen,
		SiteLeftDeltoid, SiteRightDeltoid,
		SiteLeftGlute, SiteRightGlute,
		SiteLeftThigh, SiteRightThigh,
		SiteLeftLowBack, SiteRightLowBack,
	}
	if !slices.Equal(Sites(), want) {
		t.Errorf("the zones are %v, want %v", Sites(), want)
	}
}

func TestWithNoHistoryTheSuggestionIsTheFirstZone(t *testing.T) {
	// The diagram has to open somewhere; the set's own order is the only tie-break.
	if got := SuggestNextSite(nil); got != SiteLeftAbdomen {
		t.Errorf("got %q, want the left abdomen", got)
	}
}

func TestAnUnusedZoneWinsOverEveryUsedOne(t *testing.T) {
	// The left abdomen used, so the answer cannot be the set's first element.
	if got := SuggestNextSite([]Injection{
		injected(zone(SiteLeftAbdomen), at(5, 1)),
	}); got != SiteRightAbdomen {
		t.Errorf("got %q, want the right abdomen", got)
	}

	// The mirror the frozen prototype documents: `lastUsed: ['r-abdomen']` beside
	// `suggested: 'l-abdomen'` in log-dose/data.ts. Holds only while mobile/ is frozen.
	if got := SuggestNextSite([]Injection{
		injected(zone(SiteRightAbdomen), at(5, 1)),
	}); got != SiteLeftAbdomen {
		t.Errorf("got %q, want the left abdomen", got)
	}
}

func TestTheZoneUsedLongestAgoWinsOnceEveryZoneHasBeenUsed(t *testing.T) {
	// Nine months of history: a rotation counting only a recent window would read zones
	// outside it as never injected.
	if got := SuggestNextSite(everyZoneOnce()); got != oldest {
		t.Errorf("got %q, want %q", got, oldest)
	}
}

func TestTheZoneUsedLastIsNeverSuggested(t *testing.T) {
	for _, justUsed := range Sites() {
		history := make([]Injection, 0, len(Sites())+1)
		for _, site := range Sites() {
			history = append(history, injected(zone(site), at(5, 1)))
		}
		history = append(history, injected(zone(justUsed), at(6, 1)))

		// Tie broken by the set's order: the first zone that is not the one just used.
		want := Sites()[0]
		if want == justUsed {
			want = Sites()[1]
		}
		if got := SuggestNextSite(history); got != want {
			t.Errorf("after injecting %q: got %q, want %q", justUsed, got, want)
		}
	}
}

func TestAZoneUsedTwiceCountsFromItsLatestUse(t *testing.T) {
	// The oldest zone injected again today: its latest use decides, not its first.
	history := append(everyZoneOnce(), injected(zone(oldest), at(6, 1)))

	if got := SuggestNextSite(history); got != secondOldest {
		t.Errorf("got %q, want %q", got, secondOldest)
	}
}

func TestRecencyIsTheTimestampAndNotThePositionInTheList(t *testing.T) {
	// Reading a zone's event by position rather than its latest timestamp answers
	// differently for the two orders. The two assertions are a pair: the first alone
	// would pass for any function ignoring its argument.
	oldestFirst := append(everyZoneOnce(), injected(zone(oldest), at(6, 1)))
	reversed := slices.Clone(oldestFirst)
	slices.Reverse(reversed)

	if SuggestNextSite(oldestFirst) != SuggestNextSite(reversed) {
		t.Errorf("the order of the list changed the answer: %q against %q",
			SuggestNextSite(oldestFirst), SuggestNextSite(reversed))
	}
	if got := SuggestNextSite(reversed); got != secondOldest {
		t.Errorf("got %q, want %q", got, secondOldest)
	}
}

func TestDosesOnOneDayAreSeparatedByTheClockAndNotByTheDate(t *testing.T) {
	// Ten events, not two: a rotation rounding to the day or the hour would see a ten-way
	// tie and answer with the set's first constant.
	history := make([]Injection, 0, len(newestFirst))
	for i, site := range newestFirst {
		history = append(history, injected(zone(site), at(5, 20, 9, 45-i*5)))
	}

	if got := SuggestNextSite(history); got != oldest {
		t.Errorf("got %q, want %q", got, oldest)
	}
}

func TestASiteLessEventIsIgnoredWhicheverZoneItWouldOtherwiseFallOn(t *testing.T) {
	// The loop is the point: one history only proves the site-less event is not charged to
	// the one zone it happens to answer with — an earlier version of the KMP test asserted
	// exactly that and went green when the fixture moved underneath it.
	for _, stalest := range Sites() {
		history := make([]Injection, 0, len(Sites())+1)
		for _, site := range Sites() {
			when := at(5, 1)
			if site == stalest {
				when = on(2025, 1, 1)
			}
			history = append(history, injected(zone(site), when))
		}
		history = append(history, injected(nil, at(6, 1)))

		if got := SuggestNextSite(history); got != stalest {
			t.Errorf("a site-less event moved the rotation off %q to %q", stalest, got)
		}
	}
}

func TestAHistoryOfOnlySiteLessEventsIsNoHistory(t *testing.T) {
	// Rows arrive, none carries a zone: the no-history answer, not a panic on an empty
	// group and not the zero zone by accident.
	history := []Injection{
		injected(nil, at(5, 1)),
		injected(nil, at(5, 8)),
	}

	if got, none := SuggestNextSite(history), SuggestNextSite(nil); got != none {
		t.Errorf("got %q, want the no-history answer %q", got, none)
	}
	if got := SuggestNextSite(history); got != SiteLeftAbdomen {
		t.Errorf("got %q, want the left abdomen", got)
	}
}

// Go's own case, absent from the KMP suite: kotlin.time has no zero Instant to confuse with
// «never used».
func TestAZoneInjectedAtTheZeroInstantIsStillAUsedZone(t *testing.T) {
	history := []Injection{{Site: zone(SiteLeftAbdomen), At: time.Time{}}}

	if got := SuggestNextSite(history); got != SiteRightAbdomen {
		t.Errorf("got %q, want the right abdomen — the left one was used", got)
	}
}

// The fixture's own arithmetic, asserted rather than assumed: thirty days apart in
// newestFirst order means the last element is the oldest, and everything downstream reads
// that. A fixture whose dates ran the other way would make every test above assert its own
// mirror image and pass.
func TestTheFixtureRunsFromNewestToOldest(t *testing.T) {
	history := everyZoneOnce()

	for i := 1; i < len(history); i++ {
		if !history[i].At.Before(history[i-1].At) {
			t.Fatalf("event %d (%v) is not older than event %d (%v)",
				i, history[i].At, i-1, history[i-1].At)
		}
	}
	if *history[len(history)-1].Site != oldest {
		t.Errorf("the last event is %q, want %q", *history[len(history)-1].Site, oldest)
	}
	if gap := history[0].At.Sub(history[1].At); gap != 30*24*time.Hour {
		t.Errorf("the gap is %v, want thirty days", gap)
	}
}

// parseSite has no unit test beside it, unlike journal's ParseTag and protocol's four — and
// over HTTP the schema's enum refuses an unknown zone before it is reached, so this is where
// it is measured at all.
func TestAZoneOffTheSetIsRefusedRatherThanRepresented(t *testing.T) {
	for _, refused := range []string{"l-flank", "L-ABDOMEN", "", " l-abdomen", "abdomen"} {
		if got, ok := parseSite(refused); ok {
			t.Errorf("parseSite(%q) gave %q", refused, got)
		}
	}

	for _, site := range Sites() {
		if got, ok := parseSite(string(site)); !ok || got != site {
			t.Errorf("parseSite(%q) gave %q, %v", site, got, ok)
		}
	}
}

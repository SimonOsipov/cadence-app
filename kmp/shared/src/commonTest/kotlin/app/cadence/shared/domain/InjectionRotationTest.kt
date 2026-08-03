package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.time.Duration.Companion.days
import kotlin.time.Instant

private val PATIENT = UserId("p-1")

/**
 * The two timestamps that sit beside `injectedAt` on the row, each frozen to
 * one value for every event in this file.
 *
 * `DoseEvent` carries three of them — `scheduledForDate` is the occurrence the
 * dose settles, `createdAt` is when the row reached the server, `injectedAt` is
 * when the needle went in — and only the last is a fact about the patient's
 * tissue. Freezing the other two is what makes an implementation that reaches
 * for a neighbour visible: it sees one value across the whole history, answers
 * by the set's order, and fails. Leaving them null would hide the same reach
 * instead of exposing it.
 *
 * The one case where `scheduledForDate` genuinely diverges has its own test.
 */
private val SCHEDULED = LocalDate(2026, 5, 1)
private val RECORDED = Instant.parse("2026-07-01T00:00:00Z")

private fun at(
    year: Int = 2026,
    month: Int,
    day: Int,
    hour: Int = 7,
    minute: Int = 0,
): Instant = Instant.parse("$year-${month.pad()}-${day.pad()}T${hour.pad()}:${minute.pad()}:00Z")

private fun Int.pad() = toString().padStart(2, '0')

private fun event(
    site: InjectionSite?,
    injectedAt: Instant,
    scheduledFor: LocalDate = SCHEDULED,
) = DoseEvent(
    id = DoseEventId("${site?.code ?: "no-site"}@$injectedAt"),
    patientId = PATIENT,
    protocolItemId = ProtocolItemId("sema"),
    vialId = null,
    scheduledForDate = scheduledFor,
    scheduledForTime = null,
    injectedAt = injectedAt,
    dose = Dose(0.25, DoseUnit.MG),
    site = site,
    mood = null,
    sideEffects = emptyList(),
    note = null,
    photoPath = null,
    createdAt = RECORDED,
)

/**
 * The ten zones in the order the shared history injects them, newest first.
 *
 * Neither the set's order nor its reverse, on purpose. A function that ignored
 * its input could only answer with a fixed constant, and the two constants it
 * would plausibly pick are the first and the last; if the oldest zone were
 * either of those, every assertion over this history would pass against such a
 * function. Here the oldest is the right thigh and the newest the left abdomen,
 * so the answers land in the middle of the set.
 */
private val NEWEST_FIRST =
    listOf(
        InjectionSite.LEFT_ABDOMEN,
        InjectionSite.RIGHT_ABDOMEN,
        InjectionSite.LEFT_DELTOID,
        InjectionSite.RIGHT_DELTOID,
        InjectionSite.LEFT_LOWER_BACK,
        InjectionSite.RIGHT_LOWER_BACK,
        InjectionSite.LEFT_GLUTE,
        InjectionSite.RIGHT_GLUTE,
        InjectionSite.LEFT_THIGH,
        InjectionSite.RIGHT_THIGH,
    )

private val OLDEST = NEWEST_FIRST.last()
private val SECOND_OLDEST = NEWEST_FIRST[NEWEST_FIRST.size - 2]

/** Every zone injected once, thirty days apart, in [NEWEST_FIRST] order. */
private fun everyZoneOnce() =
    NEWEST_FIRST.mapIndexed { index, site ->
        event(site, at(year = 2025, month = 10, day = 15) - (index * 30).days)
    }

class InjectionRotationTest {
    @Test
    fun theSharedHistoryInjectsEveryZoneExactlyOnce() {
        // Guards the fixture the rest of the file leans on. Add a zone to the
        // set and this fails loudly, instead of leaving the new zone unused in
        // every history below — which would quietly turn «the least recently
        // used zone» into «the first zone nobody has touched» throughout.
        assertEquals(InjectionSite.entries.toSet(), NEWEST_FIRST.toSet())
        assertEquals(InjectionSite.entries.size, NEWEST_FIRST.size, "a zone repeats in the fixture")
    }

    @Test
    fun withNoHistoryTheSuggestionIsTheFirstZone() {
        // A patient with no logged doses has no rotation, and the diagram has
        // to open somewhere; the set's own order is the only tie-break there is.
        assertEquals(InjectionSite.LEFT_ABDOMEN, suggestNextSite(emptyList()))
    }

    @Test
    fun anUnusedZoneWinsOverEveryUsedOne() {
        // The left abdomen used, so the answer cannot be `entries.first()`.
        assertEquals(
            InjectionSite.RIGHT_ABDOMEN,
            suggestNextSite(listOf(event(InjectionSite.LEFT_ABDOMEN, at(month = 5, day = 1)))),
        )

        // And the mirror of it, which is the one suggestion the frozen
        // prototype documents: `lastUsed: ['r-abdomen']` beside
        // `suggested: 'l-abdomen'` in `log-dose/data.ts`. Weak against a stub —
        // «l-abdomen» is also the first constant — but it is the fidelity
        // claim, and the rule reproduces it. It holds only while `mobile/` is
        // frozen; if that ever changes, this assertion goes with it.
        assertEquals(
            InjectionSite.LEFT_ABDOMEN,
            suggestNextSite(listOf(event(InjectionSite.RIGHT_ABDOMEN, at(month = 5, day = 1)))),
        )
    }

    @Test
    fun theZoneUsedLongestAgoWinsOnceEveryZoneHasBeenUsed() {
        // Nine months of history, deliberately: a rotation that only counted a
        // recent window would read the zones outside it as never injected and
        // answer with one of those instead of the genuinely oldest.
        assertEquals(OLDEST, suggestNextSite(everyZoneOnce()))
    }

    @Test
    fun theZoneUsedLastIsNeverSuggested() {
        InjectionSite.entries.forEach { justUsed ->
            // Every zone tied on one day, then one of them used again today.
            val history =
                InjectionSite.entries.map { event(it, at(month = 5, day = 1)) } +
                    event(justUsed, at(month = 6, day = 1))

            // The tie is broken by the set's order, so the answer is the first
            // constant that is not the one just used.
            assertEquals(
                InjectionSite.entries.first { it != justUsed },
                suggestNextSite(history),
                "wrong suggestion after injecting $justUsed",
            )
        }
    }

    @Test
    fun aZoneUsedTwiceCountsFromItsLatestUse() {
        // The oldest zone injected again today. Its first use is still the
        // oldest in the history; its latest is the newest, and that is what
        // decides — the tissue does not care that it was also used last year.
        val history = everyZoneOnce() + event(OLDEST, at(month = 6, day = 1))

        assertEquals(SECOND_OLDEST, suggestNextSite(history))
    }

    @Test
    fun recencyIsTheTimestampAndNotThePositionInTheList() {
        // The same history a repository handed back newest-first. Reading a
        // zone's first or last event by position rather than its latest
        // `injectedAt` answers differently for the two orders, and a query's
        // ORDER BY is not a clinical fact.
        //
        // The two assertions are a pair: the first alone would pass for any
        // function that ignores its argument, and the second is what pins the
        // answer. Neither is redundant.
        val oldestFirst = everyZoneOnce() + event(OLDEST, at(month = 6, day = 1))

        assertEquals(suggestNextSite(oldestFirst), suggestNextSite(oldestFirst.reversed()))
        assertEquals(SECOND_OLDEST, suggestNextSite(oldestFirst.reversed()))
    }

    @Test
    fun aBackDatedLogCountsFromWhenItWasInjectedNotWhatItWasScheduledFor() {
        // A dose injected today against an occurrence from eighteen months ago
        // — the shape a patient's back-fill takes. The zone is freshly used,
        // however old the occurrence it settles.
        val history =
            everyZoneOnce() +
                event(OLDEST, injectedAt = at(month = 6, day = 1), scheduledFor = LocalDate(2025, 1, 15))

        assertEquals(SECOND_OLDEST, suggestNextSite(history))
    }

    @Test
    fun dosesOnOneDayAreSeparatedByTheClockAndNotByTheDate() {
        // §03 allows an item twice a day — `BPC-157, п/к · 2× в день` in the
        // prototype's own compound list. Ten of them here rather than two,
        // because a resolution finer than the day can only be caught by more
        // zones than there are hours to confuse. A rotation that rounded to the
        // day, or to the hour, would see a ten-way tie and answer with the
        // first constant.
        val history =
            NEWEST_FIRST.mapIndexed { index, site ->
                event(site, at(month = 5, day = 20, hour = 9, minute = 45 - index * 5))
            }

        assertEquals(OLDEST, suggestNextSite(history))
    }

    @Test
    fun aSiteLessEventIsIgnoredWhicheverZoneItWouldOtherwiseFallOn() {
        // `DoseEvent.site` is nullable — an oral item, or a patient who skipped
        // the zone step. Such an event is not the most recent injection into
        // anywhere; it is not an injection into a zone at all.
        //
        // The loop is the point. One history only proves the site-less event is
        // not charged to the one zone that history happens to answer with — an
        // earlier version of this test asserted exactly that and went green
        // when the fixture moved underneath it. Running every zone in turn as
        // the answer leaves no zone it could be charged to unnoticed.
        InjectionSite.entries.forEach { oldest ->
            val history =
                InjectionSite.entries.map { site ->
                    val on = if (site == oldest) at(year = 2025, month = 1, day = 1) else at(month = 5, day = 1)
                    event(site, on)
                } + event(site = null, injectedAt = at(month = 6, day = 1))

            assertEquals(oldest, suggestNextSite(history), "a site-less event moved the rotation off $oldest")
        }
    }

    @Test
    fun aHistoryOfOnlySiteLessEventsIsNoHistory() {
        // The shape an upstream regression would take: rows arrive, none of
        // them carries a zone. The answer must be the no-history answer rather
        // than a crash on an empty group.
        val history =
            listOf(
                event(site = null, injectedAt = at(month = 5, day = 1)),
                event(site = null, injectedAt = at(month = 5, day = 8)),
            )

        assertEquals(suggestNextSite(emptyList()), suggestNextSite(history))
        assertEquals(InjectionSite.LEFT_ABDOMEN, suggestNextSite(history))
    }
}

package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.time.Duration.Companion.days
import kotlin.time.Instant

private val PATIENT = UserId("p-1")

/**
 * `scheduledForDate` and `createdAt` frozen to one value for every event here: only
 * `injectedAt` is a fact about the tissue, so freezing the other two makes a rotation that
 * reaches for a neighbour visible — it sees one value across the history and fails.
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
 * Neither the set's order nor its reverse, on purpose: a function ignoring its input could
 * only answer with a fixed constant (first or last), and here the oldest/newest zones sit in
 * the middle of the set so such a function would fail.
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
        // Guards the fixture the rest of the file leans on: adding a zone to the set without
        // updating NEWEST_FIRST would leave it unused throughout, silently.
        assertEquals(InjectionSite.entries.toSet(), NEWEST_FIRST.toSet())
        assertEquals(InjectionSite.entries.size, NEWEST_FIRST.size, "a zone repeats in the fixture")
    }

    @Test
    fun withNoHistoryTheSuggestionIsTheFirstZone() {
        // The diagram has to open somewhere; the set's own order is the only tie-break.
        assertEquals(InjectionSite.LEFT_ABDOMEN, suggestNextSite(emptyList()))
    }

    @Test
    fun anUnusedZoneWinsOverEveryUsedOne() {
        // The left abdomen used, so the answer cannot be `entries.first()`.
        assertEquals(
            InjectionSite.RIGHT_ABDOMEN,
            suggestNextSite(listOf(event(InjectionSite.LEFT_ABDOMEN, at(month = 5, day = 1)))),
        )

        // The mirror the frozen prototype documents: `lastUsed: ['r-abdomen']` beside
        // `suggested: 'l-abdomen'` in log-dose/data.ts. Holds only while mobile/ is frozen.
        assertEquals(
            InjectionSite.LEFT_ABDOMEN,
            suggestNextSite(listOf(event(InjectionSite.RIGHT_ABDOMEN, at(month = 5, day = 1)))),
        )
    }

    @Test
    fun theZoneUsedLongestAgoWinsOnceEveryZoneHasBeenUsed() {
        // Nine months of history: a rotation counting only a recent window would read zones
        // outside it as never injected.
        assertEquals(OLDEST, suggestNextSite(everyZoneOnce()))
    }

    @Test
    fun theZoneUsedLastIsNeverSuggested() {
        InjectionSite.entries.forEach { justUsed ->
            val history =
                InjectionSite.entries.map { event(it, at(month = 5, day = 1)) } +
                    event(justUsed, at(month = 6, day = 1))

            // Tie broken by the set's order: the first constant that isn't the one just used.
            assertEquals(
                InjectionSite.entries.first { it != justUsed },
                suggestNextSite(history),
                "wrong suggestion after injecting $justUsed",
            )
        }
    }

    @Test
    fun aZoneUsedTwiceCountsFromItsLatestUse() {
        // Oldest zone injected again today: its latest use decides, not its first.
        val history = everyZoneOnce() + event(OLDEST, at(month = 6, day = 1))

        assertEquals(SECOND_OLDEST, suggestNextSite(history))
    }

    @Test
    fun recencyIsTheTimestampAndNotThePositionInTheList() {
        // Reading a zone's event by position, not its latest injectedAt, answers differently
        // for the two orders. The two assertions are a pair: the first alone would pass for
        // any function ignoring its argument.
        val oldestFirst = everyZoneOnce() + event(OLDEST, at(month = 6, day = 1))

        assertEquals(suggestNextSite(oldestFirst), suggestNextSite(oldestFirst.reversed()))
        assertEquals(SECOND_OLDEST, suggestNextSite(oldestFirst.reversed()))
    }

    @Test
    fun aBackDatedLogCountsFromWhenItWasInjectedNotWhatItWasScheduledFor() {
        // A dose injected today against an occurrence eighteen months ago — a back-fill's shape.
        val history =
            everyZoneOnce() +
                event(OLDEST, injectedAt = at(month = 6, day = 1), scheduledFor = LocalDate(2025, 1, 15))

        assertEquals(SECOND_OLDEST, suggestNextSite(history))
    }

    @Test
    fun dosesOnOneDayAreSeparatedByTheClockAndNotByTheDate() {
        // Ten events, not two: a rotation rounding to the day or the hour would see a
        // ten-way tie and answer with the first constant.
        val history =
            NEWEST_FIRST.mapIndexed { index, site ->
                event(site, at(month = 5, day = 20, hour = 9, minute = 45 - index * 5))
            }

        assertEquals(OLDEST, suggestNextSite(history))
    }

    @Test
    fun aSiteLessEventIsIgnoredWhicheverZoneItWouldOtherwiseFallOn() {
        // The loop is the point: one history only proves the site-less event isn't charged
        // to the one zone it happens to answer with — an earlier version asserted exactly
        // that and went green when the fixture moved underneath it.
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
        // Rows arrive, none carries a zone: must be the no-history answer, not a crash on an
        // empty group.
        val history =
            listOf(
                event(site = null, injectedAt = at(month = 5, day = 1)),
                event(site = null, injectedAt = at(month = 5, day = 8)),
            )

        assertEquals(suggestNextSite(emptyList()), suggestNextSite(history))
        assertEquals(InjectionSite.LEFT_ABDOMEN, suggestNextSite(history))
    }
}

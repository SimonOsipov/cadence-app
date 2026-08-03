package app.cadence.shared.domain

import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue
import kotlin.time.Instant

// The prototype's three rows: a weekly injection, a daily one, and a nightly
// supplement — the shape the strip has to draw whatever day it is.

private val START = LocalDate(2026, 5, 10)
private val PID = ProtocolId("pr")
private val SEMA = ProtocolItemId("sema")
private val BPC = ProtocolItemId("bpc")
private val GLYCINE = ProtocolItemId("glycine")

private fun item(
    id: ProtocolItemId,
    kind: ProtocolItemKind,
    times: List<LocalTime>,
    days: List<DayOfWeek> = emptyList(),
) = ProtocolItem(
    id = id,
    protocolId = PID,
    kind = kind,
    compoundId = CompoundId(id.raw),
    cadence = if (days.isEmpty()) ProtocolCadence.DAILY else ProtocolCadence.WEEKLY,
    daysOfWeek = days,
    times = times,
    loggable = kind != ProtocolItemKind.SUPPLEMENT,
)

private val ITEMS =
    listOf(
        item(SEMA, ProtocolItemKind.INJECTION, listOf(LocalTime(7, 0)), listOf(DayOfWeek.SUNDAY)),
        item(BPC, ProtocolItemKind.INJECTION, listOf(LocalTime(8, 0), LocalTime(20, 0))),
        item(GLYCINE, ProtocolItemKind.SUPPLEMENT, listOf(LocalTime(21, 30))),
    )

private val PHASES =
    mapOf(
        SEMA to
            listOf(
                ProtocolPhase(1, 4, Dose(0.25, DoseUnit.MG)),
                ProtocolPhase(5, 8, Dose(0.5, DoseUnit.MG)),
                ProtocolPhase(9, 12, Dose(1.0, DoseUnit.MG)),
            ),
        BPC to listOf(ProtocolPhase(1, 12, Dose(250.0, DoseUnit.MCG))),
    )

private val COMPOUNDS =
    listOf(SEMA, BPC, GLYCINE).map {
        Compound(CompoundId(it.raw), it.raw, it.raw.uppercase(), DoseUnit.MG, "п/к", "beaker")
    }

private fun plan(status: ProtocolStatus = ProtocolStatus.ACTIVE) =
    ProtocolPlan(Protocol(PID, UserId("p"), START, 12, status, null, null), ITEMS, PHASES)

private fun rowsOn(
    today: LocalDate,
    events: List<DoseEvent> = emptyList(),
    status: ProtocolStatus = ProtocolStatus.ACTIVE,
) = weekProtocolRows(plan(status), COMPOUNDS, events, today)

private fun logged(
    item: ProtocolItemId,
    date: LocalDate,
    time: LocalTime,
) = DoseEvent(
    DoseEventId("e"),
    UserId("p"),
    item,
    null,
    date,
    time,
    Instant.parse("2026-05-31T07:05:00Z"),
    Dose(0.25, DoseUnit.MG),
    null,
    null,
    emptyList(),
    null,
    null,
    null,
)

class WeekProtocolTest {
    @Test
    fun everyItemOfTheProtocolGetsARowWhateverDayItIs() {
        // The strip is «Протокол этой недели», not «сегодня»: the weekly
        // injection is on it on a Wednesday too. A projection built from the
        // day's occurrences would drop it six days out of seven.
        val wednesday = LocalDate(2026, 5, 20)

        assertEquals(listOf(SEMA, BPC, GLYCINE), rowsOn(wednesday).map { it.itemId })
    }

    @Test
    fun theRowCarriesTheDoseOfTheWeekItIsIn() {
        // Not the protocol's first phase. The prototype's row reads «Семаглутид
        // · 0,25 мг» hardcoded, and stays 0,25 for all twelve weeks.
        assertEquals(Dose(0.25, DoseUnit.MG), rowsOn(LocalDate(2026, 5, 31)).first().dose)
        assertEquals(Dose(0.5, DoseUnit.MG), rowsOn(LocalDate(2026, 6, 10)).first().dose)
        assertEquals(Dose(1.0, DoseUnit.MG), rowsOn(LocalDate(2026, 7, 8)).first().dose)
    }

    @Test
    fun anItemWithNoPhasesHasNoDoseRatherThanAWrongOne() {
        // The supplement carries none in §03's model, and a row that borrowed
        // its neighbour's would be a number on screen with no source.
        assertEquals(null, rowsOn(LocalDate(2026, 5, 20)).first { it.itemId == GLYCINE }.dose)
    }

    @Test
    fun todaysStatusFollowsTheDayOccurrenceAndNotTheWeek() {
        val sunday = LocalDate(2026, 5, 31)

        val before = rowsOn(sunday).first { it.itemId == SEMA }
        val after = rowsOn(sunday, events = listOf(logged(SEMA, sunday, LocalTime(7, 0)))).first { it.itemId == SEMA }

        assertEquals(OccurrenceStatus.PENDING, before.todayStatus)
        assertEquals(OccurrenceStatus.DONE, after.todayStatus)
    }

    @Test
    fun anItemNotDueTodayHasNoStatusForToday() {
        // Wednesday has no semaglutide occurrence, so «сегодня · ждёт» would be
        // a claim about a day that does not have one.
        assertEquals(null, rowsOn(LocalDate(2026, 5, 20)).first { it.itemId == SEMA }.todayStatus)
        assertTrue(rowsOn(LocalDate(2026, 5, 20)).first { it.itemId == BPC }.todayStatus != null)
    }

    @Test
    fun aRowNamesItsCompoundRatherThanCarryingAnId() {
        // §11's Today row is «Семаглутид · 0,25 мг». A screen holding an id
        // would need a repository to turn it into a word, which is exactly
        // what a screen is not allowed to have.
        assertEquals("SEMA", rowsOn(LocalDate(2026, 5, 20)).first().compound?.nameRu)
    }

    @Test
    fun anItemWithNoCompoundHasNoneRatherThanTheWrongOne() {
        val noSuchCompound = ProtocolPlan(plan().protocol, ITEMS, PHASES)

        assertEquals(
            null,
            weekProtocolRows(noSuchCompound, emptyList(), emptyList(), LocalDate(2026, 5, 20)).first().compound,
        )
    }

    @Test
    fun aCancelledProtocolHasNoStrip() {
        assertTrue(rowsOn(LocalDate(2026, 5, 20), status = ProtocolStatus.CANCELLED).isEmpty())
    }

    @Test
    fun aDateOutsideTheCycleHasNoStrip() {
        assertTrue(rowsOn(LocalDate(2026, 5, 9)).isEmpty())
        assertTrue(rowsOn(LocalDate(2026, 8, 3)).isEmpty())
    }
}

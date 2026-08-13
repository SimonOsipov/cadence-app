package app.cadence.shared.domain

import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue
import kotlin.time.Instant

// The prototype's three rows: a weekly injection, a daily one, and a nightly supplement.

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
        // The strip is «Протокол этой недели», not «сегодня»: a projection built from the
        // day's occurrences would drop the weekly injection six days out of seven.
        val wednesday = LocalDate(2026, 5, 20)

        assertEquals(listOf(SEMA, BPC, GLYCINE), rowsOn(wednesday).map { it.itemId })
    }

    @Test
    fun theRowCarriesTheDoseOfTheWeekItIsIn() {
        // Not the protocol's first phase: the prototype's row is hardcoded «0,25 мг» for all
        // twelve weeks.
        assertEquals(Dose(0.25, DoseUnit.MG), rowsOn(LocalDate(2026, 5, 31)).first().dose)
        assertEquals(Dose(0.5, DoseUnit.MG), rowsOn(LocalDate(2026, 6, 10)).first().dose)
        assertEquals(Dose(1.0, DoseUnit.MG), rowsOn(LocalDate(2026, 7, 8)).first().dose)
    }

    @Test
    fun anItemWithNoPhasesHasNoDoseRatherThanAWrongOne() {
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
        assertEquals(null, rowsOn(LocalDate(2026, 5, 20)).first { it.itemId == SEMA }.todayStatus)
        assertTrue(rowsOn(LocalDate(2026, 5, 20)).first { it.itemId == BPC }.todayStatus != null)
    }

    @Test
    fun aRowNamesItsCompoundRatherThanCarryingAnId() {
        assertEquals("SEMA", rowsOn(LocalDate(2026, 5, 20)).first().compound?.nameRu)
    }

    @Test
    fun eachRowGetsItsOwnCompoundAndNotTheFirstOne() {
        // An earlier version passed an empty compound list, asserting "no compounds" rather
        // than the per-row mapping.
        assertEquals(
            listOf("SEMA", "BPC", "GLYCINE"),
            rowsOn(LocalDate(2026, 5, 20)).map { it.compound?.nameRu },
        )
    }

    @Test
    fun anItemReferencingAnUnknownCompoundHasNoneRatherThanTheWrongOne() {
        val onlyBpc = COMPOUNDS.filter { it.id.raw == BPC.raw }

        val rows = weekProtocolRows(plan(), onlyBpc, emptyList(), LocalDate(2026, 5, 20))

        assertEquals(null, rows.first { it.itemId == SEMA }.compound)
        assertEquals("BPC", rows.first { it.itemId == BPC }.compound?.nameRu)
    }

    @Test
    fun theRowCarriesTheKindTheStripDrawsItBy() {
        assertEquals(
            listOf(ProtocolItemKind.INJECTION, ProtocolItemKind.INJECTION, ProtocolItemKind.SUPPLEMENT),
            rowsOn(LocalDate(2026, 5, 20)).map { it.kind },
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

    @Test
    fun aTwiceDailyItemIsNotDoneUntilBothSlotsAre() {
        // `firstOrNull` was here once, and read the 08:00 slot for the whole day: logging
        // the morning injection said «Сегодня · записано» with the 20:00 one still due.
        val day = LocalDate(2026, 5, 20)

        fun bpcStatus(events: List<DoseEvent>) = rowsOn(day, events).first { it.itemId == BPC }.todayStatus

        assertEquals(
            OccurrenceStatus.PENDING,
            bpcStatus(listOf(logged(BPC, day, LocalTime(8, 0)))),
            "the evening injection is still due",
        )
        assertEquals(
            OccurrenceStatus.DONE,
            bpcStatus(listOf(logged(BPC, day, LocalTime(8, 0)), logged(BPC, day, LocalTime(20, 0)))),
            "both slots logged",
        )
        assertEquals(
            OccurrenceStatus.PENDING,
            bpcStatus(emptyList()),
            "nothing logged at all",
        )
    }

    @Test
    fun theStripCarriesWhichItemsCanBeLogged() {
        // Guarded here, not only through a simulator UI test: a mapping in `shared` should
        // fail in `shared`.
        val rows = rowsOn(LocalDate(2026, 5, 20))

        assertEquals(
            mapOf(ProtocolItemKind.INJECTION to true, ProtocolItemKind.SUPPLEMENT to false),
            rows.associate { it.kind to it.loggable },
        )
    }
}

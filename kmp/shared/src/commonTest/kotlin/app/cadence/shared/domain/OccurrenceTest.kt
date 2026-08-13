package app.cadence.shared.domain

import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlin.time.Instant

// The minimal protocol §03 describes: a 12-week cycle starting Sunday
// 10 May 2026, Семаглутид weekly on that weekday with three titration bands,
// BPC-157 daily at 08:00 and 20:00.

private val CYCLE_START = LocalDate(2026, 5, 10)
private val PATIENT = UserId("p-1")
private val PROTOCOL_ID = ProtocolId("pr-1")
private val SEMA = ProtocolItemId("sema")
private val BPC = ProtocolItemId("bpc")

private val PROTOCOL =
    Protocol(
        id = PROTOCOL_ID,
        patientId = PATIENT,
        startDate = CYCLE_START,
        weeks = 12,
        status = ProtocolStatus.ACTIVE,
        createdBy = null,
        notes = null,
    )

private val ITEMS =
    listOf(
        ProtocolItem(
            id = SEMA,
            protocolId = PROTOCOL_ID,
            kind = ProtocolItemKind.INJECTION,
            compoundId = CompoundId("sema"),
            cadence = ProtocolCadence.WEEKLY,
            daysOfWeek = listOf(DayOfWeek.SUNDAY),
            times = listOf(LocalTime(7, 0)),
            loggable = true,
        ),
        ProtocolItem(
            id = BPC,
            protocolId = PROTOCOL_ID,
            kind = ProtocolItemKind.INJECTION,
            compoundId = CompoundId("bpc"),
            cadence = ProtocolCadence.DAILY,
            daysOfWeek = emptyList(),
            times = listOf(LocalTime(8, 0), LocalTime(20, 0)),
            loggable = true,
        ),
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

private fun occurrencesOn(
    date: LocalDate,
    events: List<DoseEvent> = emptyList(),
    today: LocalDate = date,
): List<Occurrence> = occurrencesFor(ProtocolPlan(PROTOCOL, ITEMS, PHASES), events, date, today)

private fun semaDoseOn(date: LocalDate): Dose? = occurrencesOn(date).first { it.itemId == SEMA }.dose

private fun doseEventFor(
    item: ProtocolItemId,
    date: LocalDate,
    time: LocalTime? = null,
) = DoseEvent(
    id = DoseEventId("e-${item.raw}-$date"),
    patientId = PATIENT,
    protocolItemId = item,
    vialId = null,
    scheduledForDate = date,
    scheduledForTime = time,
    injectedAt = Instant.parse("2026-05-17T07:05:00Z"),
    dose = Dose(0.25, DoseUnit.MG),
    site = null,
    mood = null,
    sideEffects = emptyList(),
    note = null,
    photoPath = null,
    createdAt = null,
)

class OccurrenceTest {
    @Test
    fun theCycleWeekCountsFromTheProtocolStartAndNotFromAConstant() {
        assertEquals(1, cycleWeek(PROTOCOL, LocalDate(2026, 5, 10)))
        assertEquals(1, cycleWeek(PROTOCOL, LocalDate(2026, 5, 16)))
        assertEquals(2, cycleWeek(PROTOCOL, LocalDate(2026, 5, 17)))
        assertEquals(4, cycleWeek(PROTOCOL, LocalDate(2026, 5, 31)))
        assertEquals(12, cycleWeek(PROTOCOL, LocalDate(2026, 8, 1)))
    }

    @Test
    fun aDateOutsideTheCycleHasNoWeekAndNoOccurrences() {
        assertNull(cycleWeek(PROTOCOL, LocalDate(2026, 5, 9)), "the day before the protocol began")
        assertNull(cycleWeek(PROTOCOL, LocalDate(2026, 8, 2)), "the day after twelve weeks")
        assertTrue(occurrencesOn(LocalDate(2026, 5, 9)).isEmpty())
        assertTrue(occurrencesOn(LocalDate(2026, 8, 2)).isEmpty())
    }

    @Test
    fun aWeeklyItemFallsOnItsWeekdayAndNowhereElse() {
        assertEquals(1, occurrencesOn(LocalDate(2026, 5, 17)).count { it.itemId == SEMA })
        assertTrue(occurrencesOn(LocalDate(2026, 5, 18)).none { it.itemId == SEMA })
    }

    @Test
    fun aDailyItemEmitsEveryTimeItIsScheduledFor() {
        val bpc = occurrencesOn(LocalDate(2026, 5, 18)).filter { it.itemId == BPC }

        assertEquals(listOf(LocalTime(8, 0), LocalTime(20, 0)), bpc.map { it.time })
    }

    @Test
    fun theDoseFollowsTheTitrationPhaseForThatWeek() {
        // Boundary weeks are what an off-by-one gets wrong, so all six are asserted.
        assertEquals(Dose(0.25, DoseUnit.MG), semaDoseOn(LocalDate(2026, 5, 10)))
        assertEquals(Dose(0.25, DoseUnit.MG), semaDoseOn(LocalDate(2026, 5, 31)))
        assertEquals(Dose(0.5, DoseUnit.MG), semaDoseOn(LocalDate(2026, 6, 7)))
        assertEquals(Dose(0.5, DoseUnit.MG), semaDoseOn(LocalDate(2026, 6, 28)))
        assertEquals(Dose(1.0, DoseUnit.MG), semaDoseOn(LocalDate(2026, 7, 5)))
        // 26 July, not 1 August: the latter is a Saturday, and Семаглутид falls on the
        // cycle's start weekday.
        assertEquals(Dose(1.0, DoseUnit.MG), semaDoseOn(LocalDate(2026, 7, 26)))
    }

    @Test
    fun aLoggedEventMarksItsOccurrenceDoneAndLeavesTheOthersAlone() {
        val date = LocalDate(2026, 5, 17)

        val occurrences = occurrencesOn(date, events = listOf(doseEventFor(SEMA, date, LocalTime(7, 0))))

        assertEquals(OccurrenceStatus.DONE, occurrences.first { it.itemId == SEMA }.status)
        assertTrue(occurrences.filter { it.itemId == BPC }.none { it.status == OccurrenceStatus.DONE })
    }

    @Test
    fun anEventOnAnotherDayDoesNotSatisfyTodaysOccurrence() {
        // Matching on item alone would mark every Sunday done for the rest of the cycle.
        val logged = doseEventFor(SEMA, LocalDate(2026, 5, 17), LocalTime(7, 0))

        val later = occurrencesOn(LocalDate(2026, 5, 24), events = listOf(logged))

        assertTrue(later.first { it.itemId == SEMA }.status != OccurrenceStatus.DONE)
    }

    @Test
    fun aPastOccurrenceWithNoEventIsMissedAFutureOneScheduledAndTodaysPending() {
        val today = LocalDate(2026, 5, 31)

        assertTrue(occurrencesOn(LocalDate(2026, 5, 17), today = today).all { it.status == OccurrenceStatus.MISSED })
        assertTrue(occurrencesOn(LocalDate(2026, 6, 7), today = today).all { it.status == OccurrenceStatus.SCHEDULED })
        assertTrue(occurrencesOn(today, today = today).all { it.status == OccurrenceStatus.PENDING })
    }

    @Test
    fun anEventWithoutASlotSatisfiesNoOccurrence() {
        // Measured: one BPC event with a null scheduledForTime once marked both 08:00 and
        // 20:00 DONE, telling a patient the evening injection was already done.
        val date = LocalDate(2026, 5, 18)

        val bpc =
            occurrencesOn(date, events = listOf(doseEventFor(BPC, date, time = null)))
                .filter { it.itemId == BPC }

        assertTrue(bpc.none { it.status == OccurrenceStatus.DONE }, "a slotless event closed a slot")
    }

    @Test
    fun anEventMatchesOnlyTheSlotItWasLoggedAgainst() {
        val date = LocalDate(2026, 5, 18)

        val bpc =
            occurrencesOn(date, events = listOf(doseEventFor(BPC, date, LocalTime(8, 0))))
                .filter { it.itemId == BPC }

        assertEquals(OccurrenceStatus.DONE, bpc.first { it.time == LocalTime(8, 0) }.status)
        assertTrue(bpc.first { it.time == LocalTime(20, 0) }.status != OccurrenceStatus.DONE)
    }

    @Test
    fun anItemScheduledSeveralDaysAWeekFallsOnEachOfThem() {
        // N_PER_WEEK shares a branch with WEEKLY correctly, but nothing exercised it, so
        // deleting the case would have made «трижды в неделю» vanish with the gate green.
        val thrice =
            ProtocolItem(
                id = ProtocolItemId("tb"),
                protocolId = PROTOCOL_ID,
                kind = ProtocolItemKind.INJECTION,
                compoundId = CompoundId("tb"),
                cadence = ProtocolCadence.N_PER_WEEK,
                daysOfWeek = listOf(DayOfWeek.MONDAY, DayOfWeek.WEDNESDAY, DayOfWeek.FRIDAY),
                times = listOf(LocalTime(9, 0)),
                loggable = true,
            )
        val plan = ProtocolPlan(PROTOCOL, listOf(thrice), mapOf(thrice.id to PHASES.getValue(BPC)))

        val hit =
            (18..24)
                .map { LocalDate(2026, 5, it) }
                .filter { occurrencesFor(plan, emptyList(), it, it).isNotEmpty() }

        assertEquals(listOf(LocalDate(2026, 5, 18), LocalDate(2026, 5, 20), LocalDate(2026, 5, 22)), hit)
    }

    @Test
    fun aCancelledProtocolGeneratesNothing() {
        // Measured: a stopped course once produced a full schedule and kept telling the
        // patient to inject.
        val date = LocalDate(2026, 5, 18)
        val cancelled = ProtocolPlan(PROTOCOL.copy(status = ProtocolStatus.CANCELLED), ITEMS, PHASES)

        assertTrue(occurrencesFor(cancelled, emptyList(), date, date).isEmpty())
    }

    @Test
    fun aCompletedProtocolKeepsItsHistory() {
        // An earlier guard blanked COMPLETED too, and every patient reaches it after twelve
        // weeks — their calendar would empty retroactively.
        val date = LocalDate(2026, 5, 18)
        val done = ProtocolPlan(PROTOCOL.copy(status = ProtocolStatus.COMPLETED), ITEMS, PHASES)

        assertEquals(
            occurrencesFor(ProtocolPlan(PROTOCOL, ITEMS, PHASES), emptyList(), date, date),
            occurrencesFor(done, emptyList(), date, date),
        )
    }

    @Test
    fun theWeeklyRateFollowsTheCadenceAndTheTimes() {
        // `dosesPerWeek` had no test: replacing its body with a constant left the whole
        // suite green and the reorder hint would have quietly vanished.
        val weekly = ITEMS.first { it.id == SEMA }
        val daily = ITEMS.first { it.id == BPC }
        val thrice =
            weekly.copy(
                cadence = ProtocolCadence.N_PER_WEEK,
                daysOfWeek = listOf(DayOfWeek.MONDAY, DayOfWeek.WEDNESDAY, DayOfWeek.FRIDAY),
            )

        assertEquals(1.0, weekly.dosesPerWeek(), "one day, one time")
        assertEquals(14.0, daily.dosesPerWeek(), "seven days, two times")
        assertEquals(3.0, thrice.dosesPerWeek(), "three days, one time")
    }

    @Test
    fun aDoseLoggedForOneItemLeavesAnotherItemInTheSameSlotAlone() {
        // Measured: deleting `event.protocolItemId == item.id` from `statusOf` left the
        // suite green, because no fixture put two items in the same (date, slot) before this
        // one. Without the check, logging one of two same-slot injections greys both.
        val sunday = LocalDate(2026, 5, 17)
        val slot = LocalTime(8, 0)
        val collide =
            ITEMS.map { if (it.id == SEMA) it.copy(times = listOf(slot)) else it }
        val plan = ProtocolPlan(PROTOCOL, collide, PHASES)
        val logged = listOf(doseEventFor(SEMA, sunday, slot))

        val eight =
            occurrencesFor(plan, logged, sunday, sunday)
                .filter { it.time == slot }
        assertEquals(2, eight.size, "the fixture has to put both items in one slot")

        assertEquals(OccurrenceStatus.DONE, eight.first { it.itemId == SEMA }.status)
        assertEquals(
            OccurrenceStatus.PENDING,
            eight.first { it.itemId == BPC }.status,
            "BPC-157 was not logged; only Семаглутид was",
        )
    }
}

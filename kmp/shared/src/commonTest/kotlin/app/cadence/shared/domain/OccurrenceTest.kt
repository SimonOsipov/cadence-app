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
        // The prototype hardcodes «week 4» against a frozen 31 May. §03 derives
        // it, which is why a protocol that started elsewhere gives a different
        // answer for the same date.
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
        // Six of the seven days must be free of it, which an implementation
        // emitting every day would fail.
        assertEquals(1, occurrencesOn(LocalDate(2026, 5, 17)).count { it.itemId == SEMA })
        assertTrue(occurrencesOn(LocalDate(2026, 5, 18)).none { it.itemId == SEMA })
    }

    @Test
    fun aDailyItemEmitsEveryTimeItIsScheduledFor() {
        // §03's `times[]` — and the reason an occurrence is keyed by (item,
        // date, time) rather than by (item, date).
        val bpc = occurrencesOn(LocalDate(2026, 5, 18)).filter { it.itemId == BPC }

        assertEquals(listOf(LocalTime(8, 0), LocalTime(20, 0)), bpc.map { it.time })
    }

    @Test
    fun theDoseFollowsTheTitrationPhaseForThatWeek() {
        // 0,25 in weeks 1–4, 0,5 in 5–8, 1,0 in 9–12. The boundary weeks are
        // what an off-by-one gets wrong, so all six are asserted.
        assertEquals(Dose(0.25, DoseUnit.MG), semaDoseOn(LocalDate(2026, 5, 10)))
        assertEquals(Dose(0.25, DoseUnit.MG), semaDoseOn(LocalDate(2026, 5, 31)))
        assertEquals(Dose(0.5, DoseUnit.MG), semaDoseOn(LocalDate(2026, 6, 7)))
        assertEquals(Dose(0.5, DoseUnit.MG), semaDoseOn(LocalDate(2026, 6, 28)))
        assertEquals(Dose(1.0, DoseUnit.MG), semaDoseOn(LocalDate(2026, 7, 5)))
        // 26 July, not 1 August: the latter is inside week 12 but is a
        // Saturday, and Семаглутид falls on the cycle's start weekday.
        assertEquals(Dose(1.0, DoseUnit.MG), semaDoseOn(LocalDate(2026, 7, 26)))
    }

    @Test
    fun aLoggedEventMarksItsOccurrenceDoneAndLeavesTheOthersAlone() {
        // The status is computed by comparing generated occurrences against
        // dose events — §03's missed-dose sweep, client-side. Storing it is
        // what «nothing derived is stored» forbids.
        val date = LocalDate(2026, 5, 17)

        val occurrences = occurrencesOn(date, events = listOf(doseEventFor(SEMA, date, LocalTime(7, 0))))

        assertEquals(OccurrenceStatus.DONE, occurrences.first { it.itemId == SEMA }.status)
        assertTrue(occurrences.filter { it.itemId == BPC }.none { it.status == OccurrenceStatus.DONE })
    }

    @Test
    fun anEventOnAnotherDayDoesNotSatisfyTodaysOccurrence() {
        // Matching on the item alone would mark every Sunday done for the rest
        // of the cycle after the first injection.
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
        // Measured before it was fixed: one BPC event with a null
        // `scheduledForTime` marked both 08:00 and 20:00 DONE, so a patient who
        // had taken the morning injection was told the evening one was already
        // done. The same class of bug the date-matching comment warns about,
        // one level down.
        //
        // §03 stores `scheduled_for (date + slot)`, so a dose logged outside the
        // schedule did not fulfil a scheduled occurrence and must not claim to.
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
        // N_PER_WEEK shares a branch with WEEKLY, which is right — §03 gives
        // both `days_of_week[]` — but nothing exercised it, so deleting the
        // case would have made «трижды в неделю» vanish from the calendar with
        // the gate green.
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
    fun aProtocolThatIsNoLongerActiveGeneratesNothing() {
        // Measured before it was fixed: a CANCELLED protocol produced a full
        // schedule. §03 gives `protocols.status` three values and nothing was
        // reading it, so a patient whose course had been stopped kept being
        // told to inject.
        val date = LocalDate(2026, 5, 18)

        listOf(ProtocolStatus.CANCELLED, ProtocolStatus.COMPLETED).forEach { status ->
            val stopped = ProtocolPlan(PROTOCOL.copy(status = status), ITEMS, PHASES)
            assertTrue(occurrencesFor(stopped, emptyList(), date, date).isEmpty(), "$status still generated")
        }
    }
}

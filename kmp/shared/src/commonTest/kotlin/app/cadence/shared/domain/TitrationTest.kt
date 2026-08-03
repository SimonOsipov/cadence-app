package app.cadence.shared.domain

import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

private val START = LocalDate(2026, 5, 10)
private val PID = ProtocolId("pr")
private val SEMA = ProtocolItemId("sema")

private val PLAN =
    ProtocolPlan(
        protocol = Protocol(PID, UserId("p"), START, 12, ProtocolStatus.ACTIVE, null, null),
        items =
            listOf(
                ProtocolItem(
                    id = SEMA,
                    protocolId = PID,
                    kind = ProtocolItemKind.INJECTION,
                    compoundId = CompoundId("sema"),
                    cadence = ProtocolCadence.WEEKLY,
                    daysOfWeek = listOf(DayOfWeek.SUNDAY),
                    times = listOf(LocalTime(7, 0)),
                    loggable = true,
                ),
            ),
        phases =
            mapOf(
                SEMA to
                    // Deliberately out of order: §03 does not promise the
                    // server sends phases sorted, and a fixture that is already
                    // sorted lets a missing `sortedBy` through.
                    listOf(
                        ProtocolPhase(5, 8, Dose(0.5, DoseUnit.MG)),
                        ProtocolPhase(9, 12, Dose(1.0, DoseUnit.MG)),
                        ProtocolPhase(1, 4, Dose(0.25, DoseUnit.MG)),
                    ),
            ),
    )

class TitrationTest {
    @Test
    fun theNextStepIsTheOneAfterTheWeekYouAreIn() {
        // The prototype writes TITRATION_STEPS as a literal pair with literal
        // dates. Both are functions of the phases and the protocol's start.
        val step = titrationStepAfter(PLAN, SEMA, LocalDate(2026, 5, 31))

        assertEquals(Dose(0.25, DoseUnit.MG), step?.from)
        assertEquals(Dose(0.5, DoseUnit.MG), step?.to)
        // Week 5 begins seven days after week 4's Sunday.
        assertEquals(LocalDate(2026, 6, 7), step?.date)
    }

    @Test
    fun theStepMovesOnOnceItIsPassed() {
        val step = titrationStepAfter(PLAN, SEMA, LocalDate(2026, 6, 10))

        assertEquals(Dose(0.5, DoseUnit.MG), step?.from)
        assertEquals(Dose(1.0, DoseUnit.MG), step?.to)
        assertEquals(LocalDate(2026, 7, 5), step?.date)
    }

    @Test
    fun theLastBandHasNoNextStep() {
        // Week 9 onward is the top dose; «доза растёт» would be a promise the
        // protocol does not make.
        assertNull(titrationStepAfter(PLAN, SEMA, LocalDate(2026, 7, 20)))
    }

    @Test
    fun anItemWithOnePhaseNeverTitrates() {
        val flat = PLAN.copy(phases = mapOf(SEMA to listOf(ProtocolPhase(1, 12, Dose(250.0, DoseUnit.MCG)))))

        assertNull(titrationStepAfter(flat, SEMA, LocalDate(2026, 5, 31)))
    }

    @Test
    fun aDateOutsideTheCycleHasNoStep() {
        assertNull(titrationStepAfter(PLAN, SEMA, LocalDate(2026, 5, 9)))
    }
}

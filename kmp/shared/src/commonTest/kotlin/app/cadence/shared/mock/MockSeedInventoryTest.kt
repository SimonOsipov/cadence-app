package app.cadence.shared.mock

import app.cadence.shared.domain.DoseEvent
import app.cadence.shared.domain.InjectionSite
import app.cadence.shared.domain.OccurrenceStatus
import app.cadence.shared.domain.VialId
import app.cadence.shared.domain.VialStatus
import app.cadence.shared.domain.occurrencesFor
import app.cadence.shared.domain.remainingDoses
import app.cadence.shared.domain.vialStatus
import kotlinx.datetime.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/** The day the whole seed is written against — a Sunday, week 4 of the cycle. */
private val TODAY = LocalDate(2026, 5, 31)

private fun remaining(id: String) = remainingDoses(MockSeed.vials.first { it.id == VialId(id) }, MockSeed.history)

private fun status(id: String) = vialStatus(MockSeed.vials.first { it.id == VialId(id) }, MockSeed.history, TODAY)

class MockSeedInventoryTest {
    @Test
    fun noSeededVialCarriesARemainingCountThatWasTyped() {
        // Asserted against the seed, not the arithmetic: `Vial` has no field to store a
        // remaining count, so the only way one can be wrong is for the history to be missing.
        assertEquals(1, remaining("vial-sema-1"), "three Sundays taken of four doses")
        assertEquals(6, remaining("vial-bpc-1"), "thrown out with six still in it")
        assertEquals(2, remaining("vial-bpc-2"), "twelve of fourteen taken")
        assertEquals(4, remaining("vial-bpc-3"), "six of ten taken")
        assertEquals(30, remaining("vial-bpc-4"), "sealed, nothing drawn")
    }

    @Test
    fun everyVialStatusIsReachableFromTheSeed() {
        // Named, not counted: `vialFor`'s disposal filter had no case to filter until this
        // seed existed.
        assertEquals(
            mapOf(
                VialStatus.ACTIVE to "vial-sema-1",
                VialStatus.DISPOSED to "vial-bpc-1",
                VialStatus.LOW to "vial-bpc-2",
                VialStatus.EXPIRING to "vial-bpc-3",
                VialStatus.SEALED to "vial-bpc-4",
            ),
            MockSeed.vials.associate { vialStatus(it, MockSeed.history, TODAY) to it.id.raw },
        )
    }

    @Test
    fun theSemaglutideVialIsNearlyOutBecauseThePatientTookTheDoses() {
        // The reorder hint fires because the patient is nearly out, not because the seed was
        // tuned to fire it — the state this replaces was four doses left with nothing logged.
        val taken = MockSeed.history.count { it.vialId == VialId("vial-sema-1") }

        assertEquals(3, taken)
        assertEquals(listOf(LocalDate(2026, 5, 10), LocalDate(2026, 5, 17), LocalDate(2026, 5, 24)), semaDates())
        assertEquals(VialStatus.ACTIVE, status("vial-sema-1"))
    }

    @Test
    fun todaysDoseIsNotInTheHistory() {
        // The app opens on an unlogged day; a seeded history reaching today would take that away.
        assertTrue(MockSeed.history.none { it.scheduledForDate == TODAY })
    }

    @Test
    fun theHistoryFillsThePastAndStopsAtYesterday() {
        // Asserted through the generator the calendar reads, so a missed slot shows up as a
        // day the patient didn't finish.
        val past = generateSequence(MockSeed.cycleStart) { it.plusDay() }.takeWhile { it < TODAY }

        past.forEach { date ->
            val bpc =
                occurrencesFor(MockSeed.plan, MockSeed.history, date, TODAY).filter {
                    it.itemId ==
                        MockSeed.bpcItemId
                }

            assertTrue(
                bpc.isNotEmpty() && bpc.all { it.status == OccurrenceStatus.DONE },
                "BPC on $date is not fully logged: ${bpc.map { it.status }}",
            )
        }
    }

    @Test
    fun theSeededZonesFollowTheRotationRatherThanAList() {
        // Forty-five hand-picked zones would be forty-five chances to contradict `suggestNextSite`.
        val sited = MockSeed.history.filter { it.site != null }

        assertEquals(MockSeed.history.size, sited.size, "an injection with no zone")
        assertTrue(
            MockSeed.history.zipWithNext().all { (a, b) -> a.injectedAt <= b.injectedAt },
            "the history is not in the order it happened",
        )
        // Properties of the rule, not the rule restated: reaches every zone, no two in a row
        // repeat. A constant satisfies neither.
        assertEquals(
            InjectionSite.entries.toSet(),
            MockSeed.history.mapNotNull { it.site }.toSet(),
            "the rotation never reached some zones",
        )
        assertTrue(
            MockSeed.history.zipWithNext().none { (a, b) -> a.site == b.site },
            "two doses in a row went into the same zone",
        )
    }

    private fun semaDates(): List<LocalDate> =
        MockSeed.history
            .filter { it.vialId == VialId("vial-sema-1") }
            .map(DoseEvent::scheduledForDate)
}

private fun LocalDate.plusDay() = LocalDate.fromEpochDays(toEpochDays() + 1)

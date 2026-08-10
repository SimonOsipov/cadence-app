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
        // The subtask's prohibition, asserted against the seed rather than
        // against the arithmetic: `Vial` has no field to store a remaining
        // count in, so the only way one can be wrong is for the history behind
        // it to be missing. Every number here is `totalDoses` minus events.
        assertEquals(1, remaining("vial-sema-1"), "three Sundays taken of four doses")
        assertEquals(6, remaining("vial-bpc-1"), "thrown out with six still in it")
        assertEquals(2, remaining("vial-bpc-2"), "twelve of fourteen taken")
        assertEquals(4, remaining("vial-bpc-3"), "six of ten taken")
        assertEquals(30, remaining("vial-bpc-4"), "sealed, nothing drawn")
    }

    @Test
    fun everyVialStatusIsReachableFromTheSeed() {
        // Named, not counted. A cabinet that could only ever show one state is
        // a cabinet whose statuses nothing exercises — and `vialFor`'s disposal
        // filter had no case to filter until this seed existed.
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
        // Three of four, in week four of a weekly protocol. The reorder hint
        // now fires because the patient is nearly out rather than because the
        // seed was tuned to make it fire — the state this replaces was four
        // doses left with nothing ever logged.
        val taken = MockSeed.history.count { it.vialId == VialId("vial-sema-1") }

        assertEquals(3, taken)
        assertEquals(listOf(LocalDate(2026, 5, 10), LocalDate(2026, 5, 17), LocalDate(2026, 5, 24)), semaDates())
        assertEquals(VialStatus.ACTIVE, status("vial-sema-1"))
    }

    @Test
    fun todaysDoseIsNotInTheHistory() {
        // The whole app opens on an unlogged day: the hero offers «Записать →»
        // and `doseLoggedToday` is false. A seeded history that reached today
        // would take that state away.
        assertTrue(MockSeed.history.none { it.scheduledForDate == TODAY })
    }

    @Test
    fun theHistoryFillsThePastAndStopsAtYesterday() {
        // Every BPC slot from the protocol's start to yesterday, both times a
        // day. Asserted through the generator the calendar reads, so a history
        // that missed a slot shows up as a day the patient did not finish.
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
        // Forty-five hand-picked zones would be forty-five chances to
        // contradict `suggestNextSite`. The seed walks it, so the history is an
        // example of the rule the app follows rather than a second opinion.
        val sited = MockSeed.history.filter { it.site != null }

        assertEquals(MockSeed.history.size, sited.size, "an injection with no zone")
        assertTrue(
            MockSeed.history.zipWithNext().all { (a, b) -> a.injectedAt <= b.injectedAt },
            "the history is not in the order it happened",
        )
        // Two properties of the rule rather than the rule restated: forty-five
        // doses under a least-recently-used rotation reach every zone, and no
        // two in a row land in the same one. A constant satisfies neither.
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

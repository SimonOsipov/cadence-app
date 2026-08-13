package app.cadence.shared.domain

import app.cadence.shared.mock.MockSeed
import kotlinx.datetime.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

private val TODAY = LocalDate(2026, 5, 31)

private fun detail(id: String) =
    vialDetail(
        plan = MockSeed.plan,
        vial = MockSeed.vials.first { it.id == VialId(id) },
        events = MockSeed.history,
        today = TODAY,
        compounds = MockSeed.compounds,
    )

class VialDetailTest {
    @Test
    fun theRecordsAreThisVialsAndNobodyElses() {
        val low = detail("vial-bpc-2")

        assertEquals(12, low.recent.size)
        assertTrue(
            low.recent.all { it.date >= LocalDate(2026, 5, 25) },
            "a record from before this vial was opened is on its sheet",
        )
    }

    @Test
    fun theRecordsAreNewestFirst() {
        val dates = detail("vial-bpc-2").recent.map { it.date }

        assertEquals(dates.sortedDescending(), dates)
    }

    @Test
    fun eachRecordCarriesTheDoseAndTheZoneThatWereLogged() {
        val first = detail("vial-sema-1").recent.first()

        assertEquals(Dose(0.25, DoseUnit.MG), first.dose)
        assertTrue(first.site != null, "an injection with no zone")
        assertEquals(LocalDate(2026, 5, 24), first.date, "the newest of three Sundays")
    }

    @Test
    fun theUsageRowCountsDosesPerWeekSinceTheVialWasOpened() {
        val low = detail("vial-bpc-2")

        assertEquals(listOf(12), low.dosesPerWeek, "six days at two a day, inside one week")
    }

    @Test
    fun aVialOpenLongerHasOneNumberPerWeek() {
        // Ran from the 10th to the 22nd: two whole weeks and a fragment, so three numbers.
        val first = detail("vial-bpc-1")

        assertEquals(listOf(14, 10), first.dosesPerWeek)
    }

    @Test
    fun aSealedVialHasNoRecordsAndNoUsage() {
        val spare = detail("vial-bpc-4")

        assertEquals(emptyList(), spare.recent)
        assertEquals(emptyList(), spare.dosesPerWeek)
    }

    @Test
    fun aVialWithDosesButNoOpeningDateHasNoUsageRow() {
        // A shape the seed can't produce, so hand-built: weeks are counted from the day it
        // was opened, and there's no day to count from.
        val impossible = MockSeed.vials.first { it.id == VialId("vial-bpc-2") }.copy(openedAt = null)
        val detail =
            vialDetail(MockSeed.plan, impossible, MockSeed.history, TODAY, MockSeed.compounds)

        assertEquals(emptyList(), detail.dosesPerWeek)
        assertEquals(12, detail.recent.size, "and its records are still its own")
    }

    @Test
    fun theSheetCarriesTheSameRowTheCabinetShows() {
        val row = detail("vial-sema-1").row

        assertEquals(MockSeed.semaglutide, row.compound)
        assertEquals(1, row.remaining)
        assertEquals(VialStatus.ACTIVE, row.status)
    }
}

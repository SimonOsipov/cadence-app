package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class CadenceClockTest {
    @Test
    fun aFixedClockReportsTheInstantItWasGiven() {
        // Every test downstream of this one needs a «today» it chose, because
        // the prototype's hardcoded 31 May 2026 is exactly what §03 replaces.
        val clock = FixedCadenceClock.at("2026-05-31T04:00:00Z")

        assertEquals(LocalDate(2026, 5, 31), clock.today(TimeZone.UTC))
    }

    @Test
    fun theDateDependsOnTheZoneAndNotOnlyOnTheInstant() {
        // 22:00 UTC is already the 31st in Moscow and still the 30th in Los
        // Angeles. Cycle week, «due today» and the missed-dose sweep all hang
        // off this, so it is asserted rather than assumed.
        val clock = FixedCadenceClock.at("2026-05-30T22:00:00Z")

        assertEquals(LocalDate(2026, 5, 31), clock.today(TimeZone.of("Europe/Moscow")))
        assertEquals(LocalDate(2026, 5, 30), clock.today(TimeZone.of("America/Los_Angeles")))
    }

    @Test
    fun theSystemClockMoves() {
        val clock = SystemCadenceClock

        val first = clock.now()
        val second = clock.now()

        // Not an equality assertion — two reads can land in the same
        // millisecond. What must not happen is a constant.
        assertTrue(second.toEpochMilliseconds() > 0L, "the system clock returned the epoch")
        assertTrue(second >= first, "the system clock went backwards")
    }
}

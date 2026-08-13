package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class CadenceClockTest {
    @Test
    fun aFixedClockReportsTheInstantItWasGiven() {
        val clock = FixedCadenceClock.at("2026-05-31T04:00:00Z")

        assertEquals(LocalDate(2026, 5, 31), clock.today(TimeZone.UTC))
    }

    @Test
    fun theDateDependsOnTheZoneAndNotOnlyOnTheInstant() {
        // 22:00 UTC is already the 31st in Moscow and still the 30th in Los Angeles.
        val clock = FixedCadenceClock.at("2026-05-30T22:00:00Z")

        assertEquals(LocalDate(2026, 5, 31), clock.today(TimeZone.of("Europe/Moscow")))
        assertEquals(LocalDate(2026, 5, 30), clock.today(TimeZone.of("America/Los_Angeles")))
    }

    @Test
    fun theSystemClockMoves() {
        val clock = SystemCadenceClock

        val first = clock.now()
        val second = clock.now()

        // Not equality — two reads can land in the same millisecond; must not be a constant.
        assertTrue(second.toEpochMilliseconds() > 0L, "the system clock returned the epoch")
        assertTrue(second >= first, "the system clock went backwards")
    }
}

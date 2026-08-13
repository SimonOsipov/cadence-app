package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlinx.datetime.toLocalDateTime
import kotlin.time.Clock
import kotlin.time.Instant

/**
 * An interface, not a system-clock call: §03 replaces the prototype's three disagreeing
 * «todays» with real clocks, and a real clock is exactly what a test can't assert against.
 * Every derived thing in this module reads this, never the system clock directly.
 * `kotlin.time.Clock`, not `kotlinx.datetime.Clock`: kotlinx-datetime 0.7 moved both it and
 * `Instant` into the standard library and deprecated its own.
 */
interface CadenceClock {
    fun now(): Instant
}

/** The clock the app runs on. */
object SystemCadenceClock : CadenceClock {
    override fun now(): Instant = Clock.System.now()
}

/** The clock the tests run on. */
class FixedCadenceClock(
    private val instant: Instant,
) : CadenceClock {
    override fun now(): Instant = instant

    companion object {
        fun at(iso: String): FixedCadenceClock = FixedCadenceClock(Instant.parse(iso))
    }
}

/**
 * Takes the zone rather than assuming the device's: §03 sends the device timezone to the
 * server at sign-in so occurrences, reminders and the server sweep agree.
 */
fun CadenceClock.today(zone: TimeZone): LocalDate = now().toLocalDateTime(zone).date

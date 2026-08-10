package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlinx.datetime.toLocalDateTime
import kotlin.time.Clock
import kotlin.time.Instant

/**
 * Where «now» comes from.
 *
 * An interface rather than a call to the system clock, because §03 replaces the
 * prototype's three disagreeing «todays» — 29 May on the web, 31 May on mobile,
 * 28 May in inventory — with real clocks, and a real clock is exactly what a
 * test cannot assert against. Every derived thing in this module (cycle week,
 * «due today», a vial's days to expiry) is a function of this and of stored
 * dates, and none of them may read the system clock directly.
 *
 * `kotlin.time.Clock` rather than `kotlinx.datetime.Clock`: kotlinx-datetime
 * 0.7 moved both it and `Instant` into the standard library and deprecated its
 * own. The seam is this interface, not which stdlib type it wraps.
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
 * The patient's calendar date.
 *
 * Takes the zone rather than assuming the device's. §03 sends the device
 * timezone to the server at sign-in precisely so that occurrences, reminders
 * and the server-side sweep agree; a client that quietly used its own default
 * would be the one disagreeing.
 */
fun CadenceClock.today(zone: TimeZone): LocalDate = now().toLocalDateTime(zone).date

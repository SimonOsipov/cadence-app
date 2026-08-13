package app.cadence.screens.schedule

import app.cadence.shared.domain.TitrationStep
import app.cadence.shared.repository.ScheduleDay
import kotlinx.datetime.LocalDate

/**
 * Everything «График» draws, in one value. Taken rather than a repository for the
 * same reason every other screen does: a test has to hand it a month it chose, and
 * the Ktor client has to replace the mock without this file changing.
 */
data class ScheduleState(
    val today: LocalDate,
    val cycleWeek: Int?,
    val cycleWeeks: Int,
    val days: List<ScheduleDay>,
    val titration: TitrationStep?,
)

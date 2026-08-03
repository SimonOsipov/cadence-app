package app.cadence.shared.mock

import app.cadence.shared.domain.CadenceClock
import app.cadence.shared.domain.DoseEvent
import app.cadence.shared.domain.DoseEventId
import app.cadence.shared.domain.InjectionSite
import app.cadence.shared.domain.OccurrenceStatus
import app.cadence.shared.domain.ProtocolItemId
import app.cadence.shared.domain.SystemCadenceClock
import app.cadence.shared.domain.cycleWeek
import app.cadence.shared.domain.occurrencesFor
import app.cadence.shared.domain.remainingDoses
import app.cadence.shared.domain.reorderHint
import app.cadence.shared.domain.today
import app.cadence.shared.repository.DoseLogRepository
import app.cadence.shared.repository.ScheduleDay
import app.cadence.shared.repository.ScheduleRepository
import app.cadence.shared.repository.TodayRepository
import app.cadence.shared.repository.TodaySummary
import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.Month
import kotlinx.datetime.TimeZone
import kotlinx.datetime.plus

/**
 * Everything a screen can ask for, and one place it is assembled.
 *
 * The point of the whole subtask: a screen takes a `TodayRepository`, not a
 * `MockTodayRepository`, so replacing this object with the Ktor client in
 * M3–M10 is a change to this file and to nothing else.
 *
 * The dose events live here rather than inside either repository, because they
 * are one fact stream that both read — a mock where Today kept its own copy
 * would pass a test that logged and read through the same object and fail the
 * moment two screens were open.
 */
class CadenceMocks(
    private val clock: CadenceClock = SystemCadenceClock,
    private val zone: TimeZone = TimeZone.currentSystemDefault(),
) {
    private val events = mutableListOf<DoseEvent>()
    private var nextEventId = 0

    val today: TodayRepository = MockTodayRepository()
    val schedule: ScheduleRepository = MockScheduleRepository()
    val dosing: DoseLogRepository = MockDoseLogRepository()

    private fun currentDate(): LocalDate = clock.today(zone)

    private inner class MockTodayRepository : TodayRepository {
        override suspend fun today(): TodaySummary {
            val date = currentDate()
            val todays = occurrencesFor(MockSeed.plan, events, date, date)
            val vial = MockSeed.vials.first()

            return TodaySummary(
                cycleWeek = cycleWeek(MockSeed.plan.protocol, date),
                // The weekly injection is what «next dose» means on this
                // screen; the daily item is a strip, not a call to action.
                nextDose = todays.firstOrNull { it.itemId == MockSeed.semaItemId },
                doseLoggedToday =
                    todays.any {
                        it.itemId == MockSeed.semaItemId && it.status == OccurrenceStatus.DONE
                    },
                mealCount = MockSeed.meals.size,
                mealKcal = MockSeed.meals.sumOf { it.totals.kcal },
                targets = MockSeed.targets,
                weightKg = MockSeed.measurements.lastOrNull()?.value,
                targetWeightKg = MockSeed.profile.targetWeightKg,
                vialDosesLeft = remainingDoses(vial, events),
                reorder = reorderHint(MockSeed.vials, events, MockSeed.SEMA_DOSES_PER_WEEK),
            )
        }
    }

    private inner class MockScheduleRepository : ScheduleRepository {
        override suspend fun month(anyDateInMonth: LocalDate): List<ScheduleDay> {
            val today = currentDate()
            val first = LocalDate(anyDateInMonth.year, anyDateInMonth.month, 1)

            return generateSequence(first) { it.plus(DatePeriod(days = 1)) }
                .takeWhile { it.month == first.month }
                .map { date ->
                    val occurrences = occurrencesFor(MockSeed.plan, events, date, today)
                    ScheduleDay(
                        date = date,
                        cycleWeek = cycleWeek(MockSeed.plan.protocol, date),
                        hasInjection = occurrences.any { it.itemId == MockSeed.semaItemId },
                        anyPending = occurrences.any { it.status == OccurrenceStatus.PENDING },
                        allDone = occurrences.isNotEmpty() && occurrences.all { it.status == OccurrenceStatus.DONE },
                    )
                }.toList()
        }

        override suspend fun day(date: LocalDate) = occurrencesFor(MockSeed.plan, events, date, currentDate())
    }

    private inner class MockDoseLogRepository : DoseLogRepository {
        override suspend fun logDose(
            itemId: ProtocolItemId,
            site: InjectionSite?,
        ): DoseEventId {
            val date = currentDate()
            val occurrence =
                occurrencesFor(MockSeed.plan, events, date, date).first { it.itemId == itemId }
            val id = DoseEventId("mock-event-${nextEventId++}")

            events +=
                DoseEvent(
                    id = id,
                    patientId = MockSeed.patientId,
                    protocolItemId = itemId,
                    // §03's third correction is only real if the write carries
                    // the vial: without it the subtraction has nothing to
                    // subtract, which is the prototype's bug exactly.
                    vialId = MockSeed.vials.first().id,
                    scheduledForDate = date,
                    scheduledForTime = occurrence.time,
                    injectedAt = clock.now(),
                    dose = occurrence.dose ?: error("a loggable occurrence with no dose: $itemId"),
                    site = site,
                    mood = null,
                    sideEffects = emptyList(),
                    note = null,
                    photoPath = null,
                    createdAt = clock.now(),
                )
            return id
        }
    }
}

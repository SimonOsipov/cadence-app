package app.cadence.shared.mock

import app.cadence.shared.domain.CadenceClock
import app.cadence.shared.domain.DoseEvent
import app.cadence.shared.domain.DoseEventId
import app.cadence.shared.domain.InjectionSite
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.OccurrenceStatus
import app.cadence.shared.domain.ProtocolItemId
import app.cadence.shared.domain.ProtocolStatus
import app.cadence.shared.domain.SystemCadenceClock
import app.cadence.shared.domain.cycleWeek
import app.cadence.shared.domain.dosesPerWeek
import app.cadence.shared.domain.occurrencesFor
import app.cadence.shared.domain.partOfDay
import app.cadence.shared.domain.remainingDoses
import app.cadence.shared.domain.reorderHint
import app.cadence.shared.domain.today
import app.cadence.shared.repository.DoseLogRepository
import app.cadence.shared.repository.MeasurementsRepository
import app.cadence.shared.repository.MetricSeries
import app.cadence.shared.repository.ScheduleDay
import app.cadence.shared.repository.ScheduleRepository
import app.cadence.shared.repository.TodayRepository
import app.cadence.shared.repository.TodaySummary
import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.Month
import kotlinx.datetime.TimeZone
import kotlinx.datetime.plus
import kotlinx.datetime.toLocalDateTime

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
    // The device's zone, and that is temporary: CadenceClock's own KDoc calls
    // reading a default the wrong answer, and §03 derives cycle position from
    // «protocols.start_date + patient timezone». There is nowhere better yet —
    // §03 puts the zone on `profiles` and the seed has no `Profile`, only a
    // `PatientProfile` — so until sign-in says whose profile it is, a patient
    // in Kaliningrad with a phone on Moscow time sees a different week than the
    // server does.
    private val zone: TimeZone = TimeZone.currentSystemDefault(),
) {
    private val events = mutableListOf<DoseEvent>()
    private var nextEventId = 0

    val today: TodayRepository = MockTodayRepository()
    val schedule: ScheduleRepository = MockScheduleRepository()
    val dosing: DoseLogRepository = MockDoseLogRepository()
    val measurements: MeasurementsRepository = MockMeasurementsRepository()

    private fun currentDate(): LocalDate = clock.today(zone)

    private inner class MockTodayRepository : TodayRepository {
        override suspend fun today(): TodaySummary {
            val date = currentDate()
            val todays = occurrencesFor(MockSeed.plan, events, date, date)
            val vial = MockSeed.vials.first()
            val weightSeries =
                MockSeed.measurements
                    .filter { it.metric == Metric.WEIGHT }
                    .sortedBy { it.measuredAt }
                    .takeLast(MeasurementsRepository.DEFAULT_POINTS)
            val todaysMeals = MockSeed.meals.filter { it.eatenAt.toLocalDateTime(zone).date == date }

            return TodaySummary(
                date = date,
                partOfDay = partOfDay(clock.now().toLocalDateTime(zone).time),
                // Null for a cancelled course, like its occurrences: «Неделя 4»
                // over an empty calendar is worse than saying nothing.
                cycleWeek =
                    cycleWeek(MockSeed.plan.protocol, date)
                        ?.takeIf { MockSeed.plan.protocol.status != ProtocolStatus.CANCELLED },
                // The weekly injection is what «next dose» means on this
                // screen; the daily item is a strip, not a call to action.
                nextDose = todays.firstOrNull { it.itemId == MockSeed.semaItemId },
                doseLoggedToday =
                    todays.any {
                        it.itemId == MockSeed.semaItemId && it.status == OccurrenceStatus.DONE
                    },
                // Today's meals, not every meal ever seeded. Without the
                // filter the app shows a breakfast eaten three weeks ago as
                // eaten today — and a mutation replacing this with the seed's
                // own numbers survived, because on the seeded day the two
                // happen to agree.
                mealCount = todaysMeals.size,
                mealKcal = todaysMeals.sumOf { it.totals.kcal },
                targets = MockSeed.targets,
                // «Latest reading wins», §03 — by `measuredAt` and for this metric,
                // not by position in a list. The same defect as the unfiltered
                // meals, hidden by a seed holding exactly one measurement.
                weightSeries = weightSeries.map { it.value },
                weightKg =
                    MockSeed.measurements
                        .filter { it.metric == Metric.WEIGHT }
                        .maxByOrNull { it.measuredAt }
                        ?.value,
                targetWeightKg = MockSeed.profile.targetWeightKg,
                vialDosesLeft = remainingDoses(vial, events),
                reorder =
                    reorderHint(
                        item = MockSeed.plan.items.first { it.id == MockSeed.semaItemId },
                        vials = MockSeed.vials,
                        events = events,
                    ),
            )
        }
    }

    private inner class MockMeasurementsRepository : MeasurementsRepository {
        override suspend fun series(
            metric: Metric,
            points: Int,
        ): MetricSeries =
            MetricSeries(
                metric = metric,
                // takeLast, not take: a chart of the patient's first seven
                // weeks would never move again.
                points =
                    MockSeed.measurements
                        .filter { it.metric == metric }
                        .sortedBy { it.measuredAt }
                        .takeLast(points),
            )
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
        ): DoseEventId? {
            val date = currentDate()
            // firstOrNull, not first: `summary` is a snapshot, and an app left
            // open across midnight taps «Записать» against yesterday's
            // occurrence. Throwing inside `scope.launch` with no handler is the
            // failure this mock is not supposed to have.
            val occurrence =
                occurrencesFor(MockSeed.plan, events, date, date).firstOrNull { it.itemId == itemId }
                    ?: return null
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

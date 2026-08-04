package app.cadence.shared.mock

import app.cadence.shared.domain.CadenceClock
import app.cadence.shared.domain.Dose
import app.cadence.shared.domain.DoseDraft
import app.cadence.shared.domain.DoseEvent
import app.cadence.shared.domain.DoseEventId
import app.cadence.shared.domain.JournalEntry
import app.cadence.shared.domain.JournalSource
import app.cadence.shared.domain.Macros
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.OccurrenceStatus
import app.cadence.shared.domain.ProtocolItemId
import app.cadence.shared.domain.ProtocolStatus
import app.cadence.shared.domain.SystemCadenceClock
import app.cadence.shared.domain.Vial
import app.cadence.shared.domain.cycleWeek
import app.cadence.shared.domain.dosesPerWeek
import app.cadence.shared.domain.occurrencesFor
import app.cadence.shared.domain.partOfDay
import app.cadence.shared.domain.remainingDoses
import app.cadence.shared.domain.reorderHint
import app.cadence.shared.domain.suggestNextSite
import app.cadence.shared.domain.titrationStepAfter
import app.cadence.shared.domain.today
import app.cadence.shared.domain.weekProtocolRows
import app.cadence.shared.repository.DoseLogRepository
import app.cadence.shared.repository.DoseLogResult
import app.cadence.shared.repository.JournalRepository
import app.cadence.shared.repository.MeasurementsRepository
import app.cadence.shared.repository.MetricSeries
import app.cadence.shared.repository.ScheduleDay
import app.cadence.shared.repository.ScheduleRepository
import app.cadence.shared.repository.TodayRepository
import app.cadence.shared.repository.TodaySummary
import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime
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
    // Seeded, not empty. What the patient has already done is half of every
    // number the cabinet shows — a remaining count is `totalDoses` minus these,
    // and an empty history means five full vials and a rotation with nothing to
    // rotate away from.
    private val events = MockSeed.history.toMutableList()
    private var nextEventId = 0

    // §03 keys `journal_entries` `UNIQUE(patient, date)`, so a map by date is
    // the table's own constraint rather than a convenience — there is nowhere
    // for a second entry on one day to go.
    private val journalByDate = mutableMapOf<LocalDate, JournalEntry>()

    val today: TodayRepository = MockTodayRepository()
    val schedule: ScheduleRepository = MockScheduleRepository()
    val dosing: DoseLogRepository = MockDoseLogRepository()
    val journal: JournalRepository = MockJournalRepository()
    val measurements: MeasurementsRepository = MockMeasurementsRepository()

    private fun currentDate(): LocalDate = clock.today(zone)

    private inner class MockTodayRepository : TodayRepository {
        override suspend fun today(): TodaySummary {
            val date = currentDate()
            val todays = occurrencesFor(MockSeed.plan, events, date, date)
            // The open vial of the item's compound, not the first in the
            // list. With one vial the two agreed; with five they do not, and
            // «doses left» would have counted a sealed spare's.
            val vial = vialFor(MockSeed.semaItemId)
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
                nextDoseCompound = MockSeed.semaglutide,
                suggestedSite = suggestNextSite(events),
                weekProtocol = weekProtocolRows(MockSeed.plan, MockSeed.compounds, events, date),
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
                mealMacros =
                    todaysMeals.fold(Macros(0, 0, 0, 0)) { acc, meal ->
                        Macros(
                            kcal = acc.kcal + meal.totals.kcal,
                            proteinG = acc.proteinG + meal.totals.proteinG,
                            carbsG = acc.carbsG + meal.totals.carbsG,
                            fatG = acc.fatG + meal.totals.fatG,
                        )
                    },
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
                vialDosesLeft = vial?.let { remainingDoses(it, events) } ?: 0,
                nextTitration = titrationStepAfter(MockSeed.plan, MockSeed.semaItemId, date),
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

    private inner class MockJournalRepository : JournalRepository {
        override suspend fun entry(date: LocalDate): JournalEntry? = journalByDate[date]
    }

    private inner class MockDoseLogRepository : DoseLogRepository {
        override suspend fun submit(draft: DoseDraft): DoseLogResult {
            val itemId = draft.itemId
            val dose = draft.dose
            // `canSubmit` and these two reads answer the same question, and the
            // reads are what make the answer a non-null `ProtocolItemId` and
            // `Dose` rather than a `!!` at the write.
            if (!draft.canSubmit() || itemId == null || dose == null) return DoseLogResult.Incomplete

            val date = currentDate()
            // firstOrNull, not first: `summary` is a snapshot, and an app left
            // open across midnight taps «Записать» against yesterday's
            // occurrence. Throwing inside `scope.launch` with no handler is the
            // failure this mock is not supposed to have.
            // `loggable`, on the write rather than only in `selectItem`: the
            // rule belongs where the row is created, because a caller building
            // a draft with the constructor never passes through the chooser.
            val loggable = MockSeed.plan.items.any { it.id == itemId && it.loggable }
            val todays =
                occurrencesFor(MockSeed.plan, events, date, date)
                    .filter { loggable && it.itemId == itemId }
            // The first slot still open, not the first slot. §03 gives BPC-157
            // `times = [08:00, 20:00]`, so «the first occurrence» would let a
            // patient log the morning dose and then be told the evening one was
            // already recorded — the same defect `statusOf` was fixed for.
            val open = todays.firstOrNull { it.status != OccurrenceStatus.DONE }

            return when {
                todays.isEmpty() -> DoseLogResult.NotScheduledToday

                // §03 has one event per occurrence. Re-tapping «Сохранить» — or
                // a retry after a lost response — must not take another dose
                // out of the vial.
                open == null -> DoseLogResult.AlreadyLogged

                else -> record(draft, itemId, dose, date, open.time)
            }
        }
    }

    /** Both facts, written together — the whole point of one call. */
    private fun record(
        draft: DoseDraft,
        itemId: ProtocolItemId,
        dose: Dose,
        date: LocalDate,
        time: LocalTime,
    ): DoseLogResult.Written {
        val id = DoseEventId("mock-event-${nextEventId++}")
        val event =
            DoseEvent(
                id = id,
                patientId = MockSeed.patientId,
                protocolItemId = itemId,
                // §03's third correction is only real if the write carries the
                // vial: without it the subtraction has nothing to subtract,
                // which is the prototype's bug exactly. Null when the patient
                // has no vial of this compound — the honest answer, where
                // decrementing whatever vial came first in the list is the same
                // disconnected inventory wearing a new shape.
                vialId = vialFor(itemId)?.id,
                scheduledForDate = date,
                scheduledForTime = time,
                injectedAt = clock.now(),
                dose = dose,
                site = draft.site,
                mood = draft.mood,
                sideEffects = draft.sideEffects,
                note = draft.note,
                // The upload lands with the storage work: §03 routes photos
                // straight to object storage under a path convention, and the
                // wizard's slot only reports a tap.
                photoPath = null,
                createdAt = clock.now(),
            )

        events += event
        // From the event, not from the draft a second time: «one action, two
        // facts» is only true if the two cannot disagree, and reading the
        // draft twice is two chances to carry a different answer.
        journalByDate[date] = mergedJournalEntry(date, event)

        return DoseLogResult.Written(eventId = id, journalDate = date, dose = event.dose, vialId = event.vialId)
    }

    /**
     * The patient's vial of this item's compound, if they have one.
     *
     * Open and not disposed: a vial that ran out and was replaced is still on
     * the shelf as history, and drawing from it would put a dose into a vial
     * the patient has thrown away.
     *
     * No filter on `disposedAt`, deliberately: the seed holds one vial per
     * compound, so a disposal branch here would be a line no test can reach.
     * Choosing between several vials of one compound is the Аптечка section's
     * problem, and it arrives with the picker this block still owes —
     * `docs/prototype-divergences.md`.
     */
    private fun vialFor(itemId: ProtocolItemId): Vial? {
        val compoundId =
            MockSeed.plan.items
                .firstOrNull { it.id == itemId }
                ?.compoundId ?: return null
        return MockSeed.vials
            .filter { it.compoundId == compoundId && it.disposedAt == null && it.openedAt != null }
            // The fullest open vial. A patient may have two open at once — §03
            // allows it and this seed contains it — and until the picker lets
            // them say which, drawing from the one with the most left is at
            // least deterministic and at most wrong by one vial.
            .maxByOrNull { remainingDoses(it, events) }
    }

    /**
     * The day's journal entry after this check-in — §03's `UNIQUE(patient,
     * date)` means there is one, so a second dose updates it rather than
     * adding.
     *
     * Fields the patient skipped keep what an earlier check-in recorded: step 4
     * is «всё по желанию», so an unanswered question means «пропущено» and not
     * «сотрите то, что я сказал утром». Tags accumulate for the same reason — a
     * side effect reported this morning did not stop being true by evening.
     */
    private fun mergedJournalEntry(
        date: LocalDate,
        event: DoseEvent,
    ): JournalEntry {
        val existing = journalByDate[date]

        return JournalEntry(
            patientId = MockSeed.patientId,
            entryDate = date,
            mood = event.mood ?: existing?.mood,
            // Neither is asked by the dose wizard; they belong to the diary's
            // own check-in, which arrives in step 7. Both are the placeholder
            // for that writer — nothing can make either non-null yet, and they
            // are kept rather than deleted so step 7 has the merge already
            // shaped.
            energy = existing?.energy,
            sleep = existing?.sleep,
            tags = ((existing?.tags ?: emptyList()) + event.sideEffects).distinct(),
            note = event.note ?: existing?.note,
            // Unconditional, and that is deferred rather than decided: once
            // step 7 writes a MANUAL entry, a dose check-in afterwards will
            // keep the patient's own note and relabel its provenance. The two
            // columns §03 keeps apart need a rule then — probably that the
            // first writer owns `source`, or a third value for both.
            source = JournalSource.DOSE,
        )
    }
}

package app.cadence.shared.mock

import app.cadence.shared.domain.CadenceClock
import app.cadence.shared.domain.Dose
import app.cadence.shared.domain.DoseDraft
import app.cadence.shared.domain.DoseEvent
import app.cadence.shared.domain.DoseEventId
import app.cadence.shared.domain.Ingredient
import app.cadence.shared.domain.InventorySummary
import app.cadence.shared.domain.JournalEntry
import app.cadence.shared.domain.JournalSource
import app.cadence.shared.domain.Meal
import app.cadence.shared.domain.MealDraft
import app.cadence.shared.domain.MealId
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.MetricTrend
import app.cadence.shared.domain.OccurrenceStatus
import app.cadence.shared.domain.ProtocolItemId
import app.cadence.shared.domain.ProtocolStatus
import app.cadence.shared.domain.Recipe
import app.cadence.shared.domain.RecipeId
import app.cadence.shared.domain.SystemCadenceClock
import app.cadence.shared.domain.TrendRange
import app.cadence.shared.domain.TrendWindow
import app.cadence.shared.domain.TrendsOverview
import app.cadence.shared.domain.Vial
import app.cadence.shared.domain.VialDetail
import app.cadence.shared.domain.VialDraft
import app.cadence.shared.domain.VialId
import app.cadence.shared.domain.cycleWeek
import app.cadence.shared.domain.dayTotals
import app.cadence.shared.domain.doseBands
import app.cadence.shared.domain.dosesPerWeek
import app.cadence.shared.domain.filteredByTypeAndTag
import app.cadence.shared.domain.inventorySummary
import app.cadence.shared.domain.meta
import app.cadence.shared.domain.occurrencesFor
import app.cadence.shared.domain.partOfDay
import app.cadence.shared.domain.protocolMarks
import app.cadence.shared.domain.rangeOn
import app.cadence.shared.domain.remainingDoses
import app.cadence.shared.domain.reorderHint
import app.cadence.shared.domain.suggestNextSite
import app.cadence.shared.domain.titrationStepAfter
import app.cadence.shared.domain.today
import app.cadence.shared.domain.trendSeries
import app.cadence.shared.domain.vialDetail
import app.cadence.shared.domain.weekProtocolRows
import app.cadence.shared.repository.AddVialResult
import app.cadence.shared.repository.DoseLogRepository
import app.cadence.shared.repository.DoseLogResult
import app.cadence.shared.repository.InventoryRepository
import app.cadence.shared.repository.JournalRepository
import app.cadence.shared.repository.MealLogResult
import app.cadence.shared.repository.MeasurementsRepository
import app.cadence.shared.repository.MetricDetail
import app.cadence.shared.repository.MetricSeries
import app.cadence.shared.repository.NUTRITION_WEEK_DAYS
import app.cadence.shared.repository.NutritionDay
import app.cadence.shared.repository.NutritionRepository
import app.cadence.shared.repository.NutritionWeek
import app.cadence.shared.repository.NutritionWeekDay
import app.cadence.shared.repository.RecipeDraft
import app.cadence.shared.repository.RecipeFilter
import app.cadence.shared.repository.RecipeList
import app.cadence.shared.repository.RecipeRepository
import app.cadence.shared.repository.RecipeSaveResult
import app.cadence.shared.repository.ScheduleDay
import app.cadence.shared.repository.ScheduleRepository
import app.cadence.shared.repository.TodayRepository
import app.cadence.shared.repository.TodaySummary
import app.cadence.shared.repository.TrendsRepository
import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime
import kotlinx.datetime.Month
import kotlinx.datetime.TimeZone
import kotlinx.datetime.minus
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
    //
    // Public, not private: `TodayScreen.kt`'s recent-meal row renders a
    // `Meal.eatenAt` through the same zone this class reads its own dates by
    // (`NutritionScreen.kt:94-98` states the same rule for its own screen), and
    // a composition root that built its own default would agree with this one
    // only in production, not in a test that winds a fixed zone here.
    val zone: TimeZone = TimeZone.currentSystemDefault(),
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

    // One list, read by both Today and Nutrition — the same shape as [events]
    // above, and for the same reason: a nutrition repository holding its own
    // copy would agree with Today only by accident, the moment two screens
    // were open at once (see `NutritionRepository`'s KDoc).
    private val meals = MockSeed.meals.toMutableList()
    private var nextMealId = 0

    // The patient's own recipes, empty until `save` writes to it — the
    // library (`MockSeed.recipes`) is the clinic's and stays immutable. One
    // list rather than a private copy inside the repository's mock, matching
    // [meals] above: `RecipeRepository` is the only reader either way, but a
    // second mutable list next to this one would be a second place «own
    // recipe» could mean something.
    private val ownRecipes = mutableListOf<Recipe>()
    private var nextRecipeId = 0

    val today: TodayRepository = MockTodayRepository()
    val schedule: ScheduleRepository = MockScheduleRepository()
    val dosing: DoseLogRepository = MockDoseLogRepository()
    val journal: JournalRepository = MockJournalRepository()
    val inventory: InventoryRepository = MockInventoryRepository()
    val measurements: MeasurementsRepository = MockMeasurementsRepository()
    val trends: TrendsRepository = MockTrendsRepository()
    val nutrition: NutritionRepository = MockNutritionRepository()
    val recipes: RecipeRepository = MockRecipeRepository()

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
            val todaysMeals = meals.filter { it.eatenAt.toLocalDateTime(zone).date == date }

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
                // Exact fold across the day's meals, then the one rounding to
                // whole grams — the spec's boundary. `dayTotals()` is also
                // what `MockNutritionRepository.day` folds through, so this
                // and the Nutrition screen cannot round the same day two
                // different ways.
                mealMacros = todaysMeals.dayTotals(),
                targets = MockSeed.targets.macros,
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
                        vials = allVials(),
                        events = events,
                        today = date,
                    ),
            )
        }
    }

    private inner class MockNutritionRepository : NutritionRepository {
        override suspend fun day(date: LocalDate): NutritionDay {
            val dayMeals = mealsOn(date)
            return NutritionDay(
                date = date,
                meals = dayMeals,
                totals = dayMeals.dayTotals(),
                targets = MockSeed.targets,
            )
        }

        override suspend fun week(endingOn: LocalDate): NutritionWeek {
            val days =
                (NUTRITION_WEEK_DAYS - 1 downTo 0).map { daysAgo ->
                    val date = endingOn.minus(DatePeriod(days = daysAgo))
                    val totals = mealsOn(date).dayTotals()
                    NutritionWeekDay(date = date, kcal = totals.kcal, proteinG = totals.proteinG)
                }
            return NutritionWeek(days = days)
        }

        override suspend fun log(draft: MealDraft): MealLogResult {
            val name = draft.name
            val source = draft.source
            if (!draft.canLog() || name == null || source == null) return MealLogResult.Rejected

            // One reading, used for both the stored meal and the totals below —
            // two reads near midnight could otherwise put the meal on one day
            // and the confirming total on the next.
            val loggedAt = clock.now()
            val id = MealId("meal-added-${nextMealId++}")
            meals +=
                Meal(
                    id = id,
                    patientId = MockSeed.patientId,
                    eatenAt = loggedAt,
                    name = name,
                    source = source,
                    recipeId = draft.recipeId,
                    items = draft.items,
                )

            // A fresh read, not the day computed before the write — the
            // written meal must be inside the sum it confirms.
            val loggedDate = loggedAt.toLocalDateTime(zone).date
            return MealLogResult.Written(id = id, dayTotals = mealsOn(loggedDate).dayTotals())
        }

        private fun mealsOn(date: LocalDate): List<Meal> =
            meals.filter { it.eatenAt.toLocalDateTime(zone).date == date }
    }

    private inner class MockRecipeRepository : RecipeRepository {
        override suspend fun library(filter: RecipeFilter): RecipeList {
            // `ownRecipes` is written oldest-first (`+=`); `asReversed()` puts
            // the most recently saved one first, matching `AppState.tsx:163`'s
            // `[recipe, ...prev]`. The library sits after all of them, per
            // `AppState.tsx:166`'s `[...userRecipes, ...RECIPES.STARTERS]`.
            val ordered = ownRecipes.asReversed() + MockSeed.recipes
            return RecipeList(recipes = ordered.filteredByTypeAndTag(filter.mealType, filter.tag))
        }

        override suspend fun recipe(id: RecipeId): Recipe? =
            ownRecipes.firstOrNull { it.id == id } ?: MockSeed.recipes.firstOrNull { it.id == id }

        override suspend fun ingredients(query: String): List<Ingredient> {
            // Trimmed, matching `RecipeBuilderScreen.tsx:127`'s
            // `q.trim().toLowerCase()` — a trailing space off a mobile
            // keyboard (`"тво "`) must still find «Творог 5%», not zero
            // results.
            val trimmed = query.trim()
            return MockSeed.ingredients.filter { it.nameRu.contains(trimmed, ignoreCase = true) }
        }

        override suspend fun save(draft: RecipeDraft): RecipeSaveResult {
            val name = draft.name
            if (!draft.canSave() || name == null) return RecipeSaveResult.Rejected

            val id = RecipeId("recipe-added-${nextRecipeId++}")
            ownRecipes +=
                Recipe(
                    id = id,
                    ownerId = MockSeed.patientId,
                    name = name,
                    mealType = draft.mealType,
                    tags = draft.tags,
                    servings = draft.servings,
                    prepMin = draft.prepMin,
                    cookMin = draft.cookMin,
                    dek = draft.dek,
                    ingredients = draft.ingredients,
                    steps = draft.steps,
                )

            return RecipeSaveResult.Saved(id)
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

    private inner class MockTrendsRepository : TrendsRepository {
        override suspend fun overview(window: TrendWindow): TrendsOverview {
            val range = rangeFor(window)
            return TrendsOverview(
                window = window,
                range = range,
                // Every metric of §03, in the enum's order, including the ones
                // with nothing to show: the list is where a patient finds out
                // that a metric is unmeasured.
                metrics = Metric.entries.map { MetricTrend(it.meta, seriesIn(it, range)) },
            )
        }

        override suspend fun metric(
            metric: Metric,
            window: TrendWindow,
        ): MetricDetail {
            val range = rangeFor(window)
            return MetricDetail(
                trend = MetricTrend(metric.meta, seriesIn(metric, range)),
                // The injection item's bands, not every item's: the strip under
                // a chart answers «what was I prescribed», and the daily
                // supplement's flat band would say nothing while hiding the one
                // that titrates.
                bands = doseBands(MockSeed.plan, MockSeed.semaItemId, range),
                marks = protocolMarks(MockSeed.plan, MockSeed.semaItemId, range),
            )
        }

        /**
         * The window's days, or the whole course when it has not begun.
         *
         * `rangeOn` answers null for a course starting in the future, and a
         * repository has to answer something: the empty series that falls out
         * of a one-day range is the honest one — no readings, no bands — and
         * every screen already renders it.
         */
        private fun rangeFor(window: TrendWindow): TrendRange =
            window.rangeOn(MockSeed.plan, currentDate())
                ?: TrendRange(currentDate(), currentDate())

        private fun seriesIn(
            metric: Metric,
            range: TrendRange,
        ) = trendSeries(MockSeed.measurements, metric, range, zone)
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

    /**
     * The vials the patient has, seeded plus whatever they have added.
     *
     * One list, not two. §03's third correction is that the prototype ships a
     * cabinet dataset and a dose-logging dataset that disagree; a vial added
     * here is drawn from by the dose write on the next read, because both ask
     * this.
     */
    private val addedVials = mutableListOf<Vial>()
    private var nextVialId = 0

    private fun allVials(): List<Vial> = MockSeed.vials + addedVials

    private inner class MockInventoryRepository : InventoryRepository {
        override suspend fun cabinet(): InventorySummary =
            inventorySummary(MockSeed.plan, allVials(), events, currentDate(), MockSeed.compounds)

        override suspend fun vial(id: VialId): VialDetail? =
            allVials()
                .firstOrNull { it.id == id }
                ?.let { vialDetail(MockSeed.plan, it, events, currentDate(), MockSeed.compounds) }

        override suspend fun addVial(draft: VialDraft): AddVialResult {
            val today = currentDate()
            val compoundId = draft.compoundId
            // `canSave` and this read ask the same question; the read is what
            // makes the answer a non-null id rather than a `!!` at the write.
            if (!draft.canSave(today) || compoundId == null) return AddVialResult.Rejected

            val id = VialId("vial-added-${nextVialId++}")
            addedVials +=
                Vial(
                    id = id,
                    patientId = MockSeed.patientId,
                    compoundId = compoundId,
                    concentrationLabel = draft.concentrationLabel.orEmpty(),
                    totalDoses = draft.totalDoses,
                    // Sealed. A vial is not open until somebody opens it, and
                    // there is no remaining count to set: it is `totalDoses`
                    // minus the events, of which there are none.
                    openedAt = null,
                    expiresOn = draft.expiresOn ?: today,
                    lot = draft.lot,
                    locationRu = draft.locationRu,
                    disposedAt = null,
                    labelPhotoPath = null,
                )

            return AddVialResult.Added(id)
        }
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
                // The patient's choice when they made one; otherwise the
                // fullest open vial, which is the app choosing for them.
                vialId = draft.vialId ?: vialFor(itemId)?.id,
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
        return allVials()
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

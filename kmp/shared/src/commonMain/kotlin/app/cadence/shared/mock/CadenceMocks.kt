package app.cadence.shared.mock

import app.cadence.shared.domain.CadenceClock
import app.cadence.shared.domain.CheckInDraft
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
import app.cadence.shared.repository.JournalSaveResult
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
import kotlinx.datetime.LocalDateTime
import kotlinx.datetime.LocalTime
import kotlinx.datetime.Month
import kotlinx.datetime.TimeZone
import kotlinx.datetime.minus
import kotlinx.datetime.plus
import kotlinx.datetime.toLocalDateTime

/**
 * A screen takes a `TodayRepository`, not a `MockTodayRepository`, so replacing this object
 * with the Ktor client in M3–M10 is a change to this file and nothing else. Dose events live
 * here rather than inside either repository, since they're one fact stream both read.
 */
class CadenceMocks(
    private val clock: CadenceClock = SystemCadenceClock,
    // Temporary: §03 derives cycle position from `protocols.start_date + patient timezone`,
    // but the seed has no `Profile` to put a zone on yet, only `PatientProfile`. Public, not
    // private, because `TodayScreen.kt`'s recent-meal row renders through this same zone
    // (NutritionScreen.kt:94-98 states the same rule) and a composition root building its own
    // default would agree with this one only in production, not in a test with a fixed zone.
    val zone: TimeZone = TimeZone.currentSystemDefault(),
) {
    // Seeded, not empty: an empty history means five full vials and a rotation with nothing
    // to rotate away from.
    private val events = MockSeed.history.toMutableList()
    private var nextEventId = 0

    // §03 keys `journal_entries` UNIQUE(patient, date), so a map by date is the table's own
    // constraint, not a convenience.
    private val journalByDate = mutableMapOf<LocalDate, JournalEntry>()

    // Read by both Today and Nutrition: a nutrition repository holding its own copy would
    // agree with Today only by accident, the moment two screens were open at once.
    private val meals = MockSeed.meals.toMutableList()
    private var nextMealId = 0

    // Empty until `save` writes to it — `MockSeed.recipes` is the clinic's library and stays
    // immutable.
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

    /**
     * The clock's reading in [zone]. Public because a screen that stamps «сейчас» — the meal
     * wizard's header — must read the same clock the write does, and the composition root has
     * no other way to reach it. The prototype's own two hardcoded times (`08:42`, `13:14`) are
     * exactly what having no such reading produced.
     */
    fun nowLocal(): LocalDateTime = clock.now().toLocalDateTime(zone)

    private fun currentDate(): LocalDate = clock.today(zone)

    private inner class MockTodayRepository : TodayRepository {
        override suspend fun today(): TodaySummary {
            val date = currentDate()
            val todays = occurrencesFor(MockSeed.plan, events, date, date)
            // The open vial, not the first in the list: with five vials, "doses left" would
            // have counted a sealed spare's.
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
                // Null for a cancelled course: «Неделя 4» over an empty calendar is worse than
                // saying nothing.
                cycleWeek =
                    cycleWeek(MockSeed.plan.protocol, date)
                        ?.takeIf { MockSeed.plan.protocol.status != ProtocolStatus.CANCELLED },
                // The weekly injection is "next dose" on this screen; the daily item is a
                // strip, not a call to action.
                nextDose = todays.firstOrNull { it.itemId == MockSeed.semaItemId },
                nextDoseCompound = MockSeed.semaglutide,
                suggestedSite = suggestNextSite(events),
                weekProtocol = weekProtocolRows(MockSeed.plan, MockSeed.compounds, events, date),
                doseLoggedToday =
                    todays.any {
                        it.itemId == MockSeed.semaItemId && it.status == OccurrenceStatus.DONE
                    },
                // Today's meals, not every meal seeded: without the filter a breakfast eaten
                // three weeks ago shows as eaten today — a mutation doing that survived
                // because the seeded day happens to agree.
                mealCount = todaysMeals.size,
                // `dayTotals()` is also what `MockNutritionRepository.day` folds through, so
                // this and the Nutrition screen can't round the same day two different ways.
                mealMacros = todaysMeals.dayTotals(),
                targets = MockSeed.targets.macros,
                // «Latest reading wins», §03 — by measuredAt, not list position.
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

            // One reading for both the stored meal and the totals below: two reads near
            // midnight could otherwise put the meal on one day and the confirming total on the next.
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

            // A fresh read, not the day computed before the write: the written meal must be
            // inside the sum it confirms.
            val loggedDate = loggedAt.toLocalDateTime(zone).date
            return MealLogResult.Written(id = id, dayTotals = mealsOn(loggedDate).dayTotals())
        }

        private fun mealsOn(date: LocalDate): List<Meal> =
            meals.filter { it.eatenAt.toLocalDateTime(zone).date == date }
    }

    private inner class MockRecipeRepository : RecipeRepository {
        override suspend fun library(filter: RecipeFilter): RecipeList {
            // ownRecipes is written oldest-first; asReversed() puts the most recent first,
            // matching AppState.tsx's `[recipe, ...prev]`, library after per `[...userRecipes, ...RECIPES.STARTERS]`.
            val ordered = ownRecipes.asReversed() + MockSeed.recipes
            return RecipeList(recipes = ordered.filteredByTypeAndTag(filter.mealType, filter.tag))
        }

        override suspend fun recipe(id: RecipeId): Recipe? =
            ownRecipes.firstOrNull { it.id == id } ?: MockSeed.recipes.firstOrNull { it.id == id }

        override suspend fun ingredients(query: String): List<Ingredient> {
            // Trimmed, matching RecipeBuilderScreen.tsx: a trailing space off a mobile
            // keyboard must still find «Творог 5%», not zero results.
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
                // takeLast, not take: a chart of the patient's first seven weeks would never move again.
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
                // Every metric of §03, including ones with nothing to show: the list is where
                // a patient finds out a metric is unmeasured.
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
                // The injection item's bands, not every item's: the daily supplement's flat
                // band would hide the one that titrates.
                bands = doseBands(MockSeed.plan, MockSeed.semaItemId, range),
                marks = protocolMarks(MockSeed.plan, MockSeed.semaItemId, range),
            )
        }

        /**
         * `rangeOn` answers null for a course starting in the future; the one-day range's
         * empty series is the honest answer, and every screen already renders it.
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
     * One list, not two: the prototype ships a cabinet dataset and a dose-logging dataset
     * that disagree, but a vial added here is drawn from by the dose write on the next read.
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
            // `canSave` and this read ask the same question; the read makes the answer a
            // non-null id rather than a `!!` at the write.
            if (!draft.canSave(today) || compoundId == null) return AddVialResult.Rejected

            val id = VialId("vial-added-${nextVialId++}")
            addedVials +=
                Vial(
                    id = id,
                    patientId = MockSeed.patientId,
                    compoundId = compoundId,
                    concentrationLabel = draft.concentrationLabel.orEmpty(),
                    totalDoses = draft.totalDoses,
                    // Sealed: not open until somebody opens it.
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

        override suspend fun save(draft: CheckInDraft): JournalSaveResult =
            when {
                draft.saysNothing -> JournalSaveResult.Rejected.Empty
                !draft.readingsAreOnTheScale -> JournalSaveResult.Rejected.OffTheScale
                else -> JournalSaveResult.Written(write(draft, JournalSource.MANUAL))
            }
    }

    private inner class MockDoseLogRepository : DoseLogRepository {
        override suspend fun submit(draft: DoseDraft): DoseLogResult {
            val itemId = draft.itemId
            val dose = draft.dose
            // `canSubmit` and these two reads answer the same question, making it a non-null
            // `ProtocolItemId`/`Dose` rather than a `!!` at the write.
            if (!draft.canSubmit() || itemId == null || dose == null) return DoseLogResult.Incomplete

            val date = currentDate()
            // firstOrNull, not first: `summary` is a snapshot, and an app left open across
            // midnight taps against yesterday's occurrence.
            // `loggable` checked on the write, not only in `selectItem`: a caller building a
            // draft with the constructor never passes through the chooser.
            val loggable = MockSeed.plan.items.any { it.id == itemId && it.loggable }
            val todays =
                occurrencesFor(MockSeed.plan, events, date, date)
                    .filter { loggable && it.itemId == itemId }
            // The first slot still open, not the first slot: §03 gives BPC-157 two daily
            // times, so "the first occurrence" would tell a patient the evening dose was
            // already recorded after only logging the morning one.
            val open = todays.firstOrNull { it.status != OccurrenceStatus.DONE }

            return when {
                todays.isEmpty() -> DoseLogResult.NotScheduledToday

                // §03 has one event per occurrence: re-tapping «Сохранить» must not take
                // another dose out of the vial.
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
                // Null when the patient has no vial of this compound — the honest answer;
                // decrementing whatever came first in the list would be the prototype's bug
                // wearing a new shape. The patient's choice when made, else the fullest open vial.
                vialId = draft.vialId ?: vialFor(itemId)?.id,
                scheduledForDate = date,
                scheduledForTime = time,
                injectedAt = clock.now(),
                dose = dose,
                site = draft.site,
                mood = draft.mood,
                sideEffects = draft.sideEffects,
                note = draft.note,
                // Upload lands with the storage work; the wizard's slot only reports a tap for now.
                photoPath = null,
                createdAt = clock.now(),
            )

        events += event
        // From the event, not the draft a second time: reading the draft twice is two
        // chances to carry a different answer.
        write(event.asCheckIn(date), JournalSource.DOSE)

        return DoseLogResult.Written(eventId = id, journalDate = date, dose = event.dose, vialId = event.vialId)
    }

    /**
     * Open and not disposed: a disposed vial is still on the shelf as history, and drawing
     * from it would put a dose into a vial the patient threw away.
     */
    private fun vialFor(itemId: ProtocolItemId): Vial? {
        val compoundId =
            MockSeed.plan.items
                .firstOrNull { it.id == itemId }
                ?.compoundId ?: return null
        return allVials()
            .filter { it.compoundId == compoundId && it.disposedAt == null && it.openedAt != null }
            // Fullest open vial: with two open at once and no picker yet, this is at least
            // deterministic and at most wrong by one vial.
            .maxByOrNull { remainingDoses(it, events) }
    }

    /** The wizard's step 4 is a check-in like any other; only the path differs. */
    private fun DoseEvent.asCheckIn(date: LocalDate) =
        CheckInDraft(entryDate = date, mood = mood, tags = sideEffects, note = note)

    /** Provenance is set once: [bornAs] applies only to an empty day, so «с дозой» means born of a dose. */
    private fun write(
        draft: CheckInDraft,
        bornAs: JournalSource,
    ): JournalEntry {
        val existing = journalByDate[draft.entryDate]

        val merged =
            JournalEntry(
                patientId = MockSeed.patientId,
                entryDate = draft.entryDate,
                mood = draft.mood ?: existing?.mood,
                energy = draft.energy ?: existing?.energy,
                sleep = draft.sleep ?: existing?.sleep,
                tags = ((existing?.tags ?: emptyList()) + draft.tags).distinct(),
                note = draft.note?.takeUnless { it.isBlank() } ?: existing?.note,
                source = existing?.source ?: bornAs,
            )

        journalByDate[draft.entryDate] = merged
        return merged
    }
}

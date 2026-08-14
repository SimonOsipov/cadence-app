package app.cadence.shell

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavGraphBuilder
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.toRoute
import app.cadence.design.CadenceDestination
import app.cadence.design.CadenceSheet
import app.cadence.format.formatDose
import app.cadence.screens.dose.DoseOption
import app.cadence.screens.dose.DoseWizard
import app.cadence.screens.inventory.AddVialScreen
import app.cadence.screens.inventory.VialDetailSheet
import app.cadence.screens.inventory.VialsScreen
import app.cadence.screens.nutrition.LogMealScreen
import app.cadence.screens.schedule.ScheduleScreen
import app.cadence.screens.schedule.ScheduleState
import app.cadence.screens.today.TodayScreen
import app.cadence.screens.trends.TrendDetailScreen
import app.cadence.screens.trends.TrendsScreen
import app.cadence.shared.domain.DoseDraft
import app.cadence.shared.domain.DoseStep
import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.domain.Ingredient
import app.cadence.shared.domain.InjectionSite
import app.cadence.shared.domain.InventorySummary
import app.cadence.shared.domain.Meal
import app.cadence.shared.domain.MealDraft
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.ProtocolCadence
import app.cadence.shared.domain.ProtocolRow
import app.cadence.shared.domain.TrendWindow
import app.cadence.shared.domain.TrendsOverview
import app.cadence.shared.domain.VialDetail
import app.cadence.shared.domain.VialDraft
import app.cadence.shared.domain.VialId
import app.cadence.shared.mock.CadenceMocks
import app.cadence.shared.mock.MockSeed
import app.cadence.shared.parsing.MealParseResult
import app.cadence.shared.parsing.MockMealParser
import app.cadence.shared.repository.DoseLogResult
import app.cadence.shared.repository.MealLogResult
import app.cadence.shared.repository.MetricDetail
import app.cadence.shared.repository.NutritionDay
import app.cadence.shared.repository.NutritionWeek
import app.cadence.shared.repository.RecipeDraft
import app.cadence.shared.repository.RecipeList
import app.cadence.shared.repository.RecipeSaveResult
import app.cadence.shared.repository.TodaySummary
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalDateTime
import kotlinx.datetime.TimeZone

/** Until sign-in says whose app this is — block 7. */
private const val PATIENT_NAME = "Марина"
private const val PRIMARY_THREAD = "ksenia"

/** §03: `protocols.weeks (12)`. Read from the protocol once one is fetched. */
private const val CYCLE_WEEKS = 12

/**
 * Held above the trends screen, not inside it: list and detail both read it, so a window
 * remembered in one would reset on return from the other.
 */
private class TrendsUiState {
    var window by mutableStateOf(TrendWindow.THREE_MONTHS)
    var overview by mutableStateOf<TrendsOverview?>(null)
}

/** Keyed on both [reloads] and the window: keying on [reloads] alone would leave the chips changing nothing. */
@Composable
private fun rememberTrendsState(
    mocks: CadenceMocks,
    reloads: Int,
): TrendsUiState {
    val state = remember { TrendsUiState() }
    LaunchedEffect(state.window, reloads) {
        state.overview = mocks.trends.overview(state.window)
    }
    return state
}

/**
 * Everything one `reloads` bump re-reads, held together for the same reason [TrendsUiState]
 * is: eight `remember`s in [CadenceApp] and one effect that assigns them all is one fact —
 * "what the repositories last said" — spelled eight times.
 */
private class DayUiState {
    var summary by mutableStateOf<TodaySummary?>(null)

    // TodaySummary carries only the count and the fold — this is TodayMeals' own list.
    var todayMeals by mutableStateOf<List<Meal>>(emptyList())
    var schedule by mutableStateOf<ScheduleState?>(null)
    var cabinet by mutableStateOf<InventorySummary?>(null)
}

@Composable
private fun rememberDayState(
    mocks: CadenceMocks,
    reloads: Int,
): DayUiState {
    val state = remember { DayUiState() }
    LaunchedEffect(reloads) {
        val today = mocks.today.today()
        state.summary = today
        state.schedule = mocks.scheduleFor(today)
        state.cabinet = mocks.inventory.cabinet()
        state.todayMeals = mocks.nutrition.day(today.date).meals
    }
    return state
}

/**
 * The confirmation toast and the counter that restarts its timer, together — they are one
 * fact and were two `remember`s.
 *
 * Keyed on a counter, not the toast itself: [ConfirmToastState] is a data class and
 * `mutableStateOf` compares structurally, so an equal toast raised inside the window would
 * be no assignment at all — no restart, and the new confirmation would inherit the old
 * timer. No UI test reaches the identical-toast case directly (the overlay blocks a second
 * tap until the first toast clears — `ConfirmToastTest.theToastSwallowsEveryTouchWhileItIsUp`),
 * but the counter stays for any future non-tap trigger.
 */
internal class ToastUiState {
    var state by mutableStateOf<ConfirmToastState?>(null)
        private set
    var raisedAt by mutableStateOf(0)
        private set

    fun raise(
        mealName: String,
        dayKcal: Int,
    ) {
        state = ConfirmToastState(mealName, dayKcal)
        raisedAt++
    }

    fun clear() {
        state = null
    }
}

/** The month around [today], in the shape «График» draws. */
private suspend fun CadenceMocks.scheduleFor(today: TodaySummary): ScheduleState =
    ScheduleState(
        today = today.date,
        cycleWeek = today.cycleWeek,
        cycleWeeks = CYCLE_WEEKS,
        days = schedule.month(today.date),
        titration = today.nextTitration,
    )

/**
 * No failure path yet: the mock cannot throw on any reachable path, but the Ktor
 * client can, and this composable — not just the repositories — will need one.
 */
@Composable
fun CadenceApp(
    navController: NavHostController = rememberNavController(),
    modifier: Modifier = Modifier,
    // Wound to the seed's own day (MockSeed.DEMO_NOW), not the system clock: the fixture is
    // a course with an end date, and the real clock would empty every screen once it passes.
    mocks: CadenceMocks = remember { CadenceMocks(FixedCadenceClock.at(MockSeed.DEMO_NOW)) },
) {
    val scope = rememberCoroutineScope()
    // Bumped by every write so the next read goes back to the repository, not a stale snapshot.
    var reloads by remember { mutableStateOf(0) }

    val parser = remember { MockMealParser() }
    val day = rememberDayState(mocks, reloads)
    val trendsState = rememberTrendsState(mocks, reloads)
    val summary = day.summary

    var openVial by remember { mutableStateOf<VialDetail?>(null) }
    var actionsOpen by remember { mutableStateOf(false) }
    val toast = remember { ToastUiState() }

    ToastLifetime(toast.raisedAt, toast.state, toast::clear)

    Box(modifier.fillMaxSize()) {
        CadenceShell(
            navController = navController,
            data =
                CadenceShellData(
                    summary = summary,
                    todayMeals = day.todayMeals,
                    schedule = day.schedule,
                    cabinet = day.cabinet,
                    trends = trendsState.overview,
                    trendWindow = trendsState.window,
                    now = mocks.nowLocal(),
                    zone = mocks.zone,
                ),
            actions =
                CadenceShellActions(
                    onOpenActions = { actionsOpen = true },
                    onDoseLogged = { draft ->
                        val result = mocks.dosing.submit(draft)
                        reloads++
                        result
                    },
                    onOpenVial = { scope.launch { openVial = mocks.inventory.vial(it) } },
                    onAddVial = { draft ->
                        scope.launch {
                            mocks.inventory.addVial(draft)
                            reloads++
                        }
                    },
                    onTrendWindow = { trendsState.window = it },
                    onLoadMetric = { metric, window -> mocks.trends.metric(metric, window) },
                    parseMeal = { text -> parser.parse(text) },
                    onMealLogged = { draft -> logMeal(mocks, toast, draft) { reloads++ } },
                ),
        )

        Actions(summary, actionsOpen, navController, onDismiss = { actionsOpen = false })

        VialSheet(openVial, today = summary?.date, onDismiss = { openVial = null }, onLogDose = { vialId ->
            openVial = null
            // Named, not dropped: a null draft here let the picker silently default to the
            // fullest open vial, so a half-empty vial's dose got recorded against the wrong one.
            navController.openRoute(CadenceRoute.LogDose(vialId.raw))
        })

        ConfirmToast(state = toast.state, targetKcal = summary?.targets?.kcal ?: 0)
    }
}

/** A raised toast clears itself. Its own composable so `CadenceApp` stays readable. */
@Composable
private fun ToastLifetime(
    raisedAt: Int,
    toast: ConfirmToastState?,
    onClear: () -> Unit,
) {
    LaunchedEffect(raisedAt) {
        if (toast != null) {
            delay(CADENCE_CONFIRM_TOAST_MS)
            onClear()
        }
    }
}

/**
 * Everything the shell has been *told*, in one value. Every field defaults to its
 * "not read yet" state, so a test builds only the field its case turns on.
 *
 * A null on any of the nullable fields means the first read has not landed; a route gated
 * on one draws its placeholder rather than composing a screen out of zeroes, which would
 * be a false statement about the patient rather than an honest one about the fetch.
 */
data class CadenceShellData(
    val summary: TodaySummary? = null,
    val todayMeals: List<Meal> = emptyList(),
    val schedule: ScheduleState? = null,
    val cabinet: InventorySummary? = null,
    val trends: TrendsOverview? = null,
    val trendWindow: TrendWindow = TrendWindow.THREE_MONTHS,
    /** The clock's own reading, for the one screen that stamps «сейчас» on what it writes. */
    val now: LocalDateTime? = null,
    val nutritionDay: NutritionDay? = null,
    val nutritionWeek: NutritionWeek? = null,
    val recipes: RecipeList? = null,
    val ingredients: List<Ingredient> = emptyList(),
    val zone: TimeZone = TimeZone.currentSystemDefault(),
)

/**
 * Everything the shell can *do*, in one value — each defaulting to a no-op so a test names
 * only the event its case asserts on.
 *
 * Split from [CadenceShellData] rather than folded in with it: data is replaced on every
 * `reloads` bump while these stay put for the app's lifetime, and `CadenceNavigationTest`'s
 * own call sites show the split is the one tests want (three pass neither, one passes a
 * single lambda). Writes return their repository's own sealed result, like `onDoseLogged`
 * already does, so a route can tell a written meal from a rejected one without a second
 * read.
 */
data class CadenceShellActions(
    val onOpenActions: () -> Unit = { },
    val onDoseLogged: suspend (DoseDraft) -> DoseLogResult = { DoseLogResult.Incomplete },
    val onMealLogged: suspend (MealDraft) -> MealLogResult = { MealLogResult.Rejected },
    val onAddVial: (VialDraft) -> Unit = { },
    val onOpenVial: (VialId) -> Unit = { },
    val onTrendWindow: (TrendWindow) -> Unit = { },
    val onLoadMetric: suspend (Metric, TrendWindow) -> MetricDetail? = { _, _ -> null },
    val parseMeal: suspend (String) -> MealParseResult = { MealParseResult.Unavailable },
    val searchIngredients: suspend (String) -> List<Ingredient> = { emptyList() },
    val onRecipeSaved: suspend (RecipeDraft) -> RecipeSaveResult = { RecipeSaveResult.Rejected },
)

/**
 * Screens get named lambdas and never the controller, so each stays testable without a
 * navigator; `onOpenActions` is the one callback that isn't navigation — the `+` opens a sheet.
 * The controller is a parameter so a test can assert on the back stack.
 */
@Composable
fun CadenceShell(
    navController: NavHostController = rememberNavController(),
    modifier: Modifier = Modifier,
    data: CadenceShellData = CadenceShellData(),
    actions: CadenceShellActions = CadenceShellActions(),
) {
    NavHost(
        navController = navController,
        startDestination = CADENCE_ROOT,
        modifier = modifier.fillMaxSize(),
        // Transitions live on the NavHost, not the modal: Compose reads each side from the
        // destination it belongs to, so overrides on the modal itself fire on the wrong event.
        enterTransition = { if (targetState.destination.isModal()) modalEnter() else pushEnter() },
        exitTransition = { if (targetState.destination.isModal()) modalUnderlayExit() else pushExit() },
        popEnterTransition = { if (initialState.destination.isModal()) modalUnderlayEnter() else popEnter() },
        popExitTransition = { if (initialState.destination.isModal()) modalExit() else popExit() },
    ) {
        tabRoutes(navController, data, actions)
        pushedRoutes(navController, data, actions)
        modalRoutes(navController, data, actions)
    }
}

/**
 * Written as a `when` over the enum so a fifth destination fails to compile here too, not
 * only in [CadenceDestination.route].
 */
private fun NavGraphBuilder.tabRoutes(
    nav: NavHostController,
    data: CadenceShellData,
    actions: CadenceShellActions,
) {
    val onOpenActions = actions.onOpenActions

    CadenceDestination.entries.forEach { destination ->
        val body: @Composable () -> Unit = {
            PlaceholderScreen(
                title = destination.label,
                destination = destination,
                // Today is the root, nothing to go back to; the other three use goBack, which
                // stops being equivalent to popToTop once a screen reaches a tab from depth > 1.
                onBack = if (destination == CadenceDestination.TODAY) null else back(nav),
                onSelectTab = nav::selectDestination,
                onLog = onOpenActions,
            )
        }
        when (destination) {
            CadenceDestination.TODAY -> {
                composable<CadenceRoute.Today> {
                    TodayRoute(nav, data.summary, data.todayMeals, data.zone, onOpenActions, body)
                }
            }

            CadenceDestination.INVENTORY -> {
                composable<CadenceRoute.Vials> {
                    // Gated on the read: composed against no data this would draw «0 флаконов», a
                    // false statement about the patient rather than an honest one about the fetch.
                    val cabinet = data.cabinet
                    if (cabinet == null) {
                        body()
                    } else {
                        VialsScreen(
                            cabinet = cabinet,
                            onOpenVial = actions.onOpenVial,
                            onAddVial = { nav.openRoute(CadenceRoute.AddVial) },
                            onSelectTab = nav::selectDestination,
                            onOpenActions = onOpenActions,
                        )
                    }
                }
            }

            CadenceDestination.TRENDS -> {
                composable<CadenceRoute.Trends> {
                    // Gated on the read, same reason as Vials above.
                    val trends = data.trends
                    if (trends == null) {
                        body()
                    } else {
                        TrendsScreen(
                            overview = trends,
                            onOpenMetric = { nav.openRoute(CadenceRoute.TrendDetail(it.code)) },
                            onWindowChange = actions.onTrendWindow,
                            onOpenJournal = { nav.openRoute(CadenceRoute.Journal) },
                            onOpenBody = { nav.openRoute(CadenceRoute.Body) },
                            onSelectTab = nav::selectDestination,
                            onOpenActions = onOpenActions,
                        )
                    }
                }
            }

            CadenceDestination.NUTRITION -> {
                composable<CadenceRoute.Nutrition> { body() }
            }
        }
    }
}

/**
 * One definition rather than one per graph section: `popBackStack()` inline in three places
 * is three chances for one to become `{ }` and strand a screen.
 */
private fun back(nav: NavHostController): () -> Unit = { nav.popBackStack() }

private fun NavGraphBuilder.pushedRoutes(
    nav: NavHostController,
    data: CadenceShellData,
    actions: CadenceShellActions,
) {
    val back = back(nav)
    val trendWindow = data.trendWindow

    composable<CadenceRoute.TrendDetail> { entry ->
        // The route carries a String; null reaches the screen as «такой метрики нет» —
        // the prototype has a `thigh` and a `bmi` the data model doesn't, and a deep
        // link can carry anything.
        val code = entry.toRoute<CadenceRoute.TrendDetail>().biomarkerId
        val metric = Metric.fromCode(code)
        // Keyed on the code alone, not the window: stale-while-revalidate, so a window
        // change doesn't wipe the chart and chip row while the new read is in flight.
        var detail by remember(code) { mutableStateOf<MetricDetail?>(null) }
        // Reset with metric and window so a scrub position never lands on another's readings.
        var scrubIndex by remember(code, trendWindow) { mutableStateOf<Int?>(null) }

        LaunchedEffect(code, trendWindow) {
            detail = metric?.let { actions.onLoadMetric(it, trendWindow) }
        }

        // A null `detail` here means «ещё не пришло», not «такой метрики нет» (metric == null) —
        // conflating them would answer «no such metric» to a load that's just in flight.
        if (metric != null && detail == null) {
            PlaceholderScreen("Метрика", onBack = back)
        } else {
            TrendDetailScreen(
                detail = detail,
                window = trendWindow,
                onWindowChange = actions.onTrendWindow,
                onBack = back,
                scrubIndex = scrubIndex,
                onScrub = { index, _ -> scrubIndex = index },
            )
        }
    }
    composable<CadenceRoute.Schedule> {
        val schedule = data.schedule
        if (schedule == null) {
            PlaceholderScreen("Расписание", onBack = back)
        } else {
            ScheduleScreen(state = schedule, onBack = back)
        }
    }
    composable<CadenceRoute.Learn> { PlaceholderScreen("Обучение", onBack = back) }
    composable<CadenceRoute.Article> { entry ->
        PlaceholderScreen("Статья · ${entry.toRoute<CadenceRoute.Article>().articleId}", onBack = back)
    }
    composable<CadenceRoute.Journal> { PlaceholderScreen("Дневник", onBack = back) }
    composable<CadenceRoute.Body> { PlaceholderScreen("Тело", onBack = back) }
    composable<CadenceRoute.Recipes> { PlaceholderScreen("Рецепты", onBack = back) }
    composable<CadenceRoute.RecipeDetail> { entry ->
        PlaceholderScreen("Рецепт · ${entry.toRoute<CadenceRoute.RecipeDetail>().recipeId}", onBack = back)
    }
    composable<CadenceRoute.Profile> { PlaceholderScreen("Профиль", onBack = back) }
    composable<CadenceRoute.ChatList> { PlaceholderScreen("Чаты", onBack = back) }
    composable<CadenceRoute.ChatThread> { entry ->
        PlaceholderScreen("Переписка · ${entry.toRoute<CadenceRoute.ChatThread>().threadId}", onBack = back)
    }
}

/** The prototype's `Stack.Group` with `presentation: 'fullScreenModal'`: slides up, not in. */
private fun NavGraphBuilder.modalRoutes(
    nav: NavHostController,
    data: CadenceShellData,
    actions: CadenceShellActions,
) {
    val back = back(nav)
    val today = data.summary

    modal<CadenceRoute.LogDose> { entry ->
        LogDoseModal(
            summary = data.summary,
            cabinet = data.cabinet,
            openedVial = entry.toRoute<CadenceRoute.LogDose>().vialId?.let(::VialId),
            onDoseLogged = actions.onDoseLogged,
            back = back,
        )
    }
    modal<CadenceRoute.LogMeal> {
        LogMealModal(summary = data.summary, now = data.now, actions = actions, back = back)
    }
    modal<CadenceRoute.AddVial> {
        // Gated on the read like its siblings: substituting MockSeed.cycleStart for a
        // summary that hadn't landed let a vial expiring months ago save with no warning
        // (VialDraft.canSave guards on expiresOn >= today, using that fallback date).
        val day = today?.date

        if (day == null) {
            PlaceholderScreen(title = "Добавить флакон", onBack = back)
        } else {
            AddVialScreen(
                compounds = MockSeed.compounds,
                today = day,
                onSave = {
                    actions.onAddVial(it)
                    back()
                },
                onCancel = back,
            )
        }
    }
    modal<CadenceRoute.RecipeBuilder> { PlaceholderScreen("Новый рецепт", onBack = back) }
}

/**
 * Gated on the read like the other ported routes: composed against a null summary the
 * wizard draws no compounds, and «Дальше» is dead on step 1.
 * [openedVial] seeds the draft; the picker on step 2 can still override it.
 */
@Composable
private fun LogDoseModal(
    summary: TodaySummary?,
    cabinet: InventorySummary?,
    openedVial: VialId?,
    onDoseLogged: suspend (DoseDraft) -> DoseLogResult,
    back: () -> Unit,
) {
    if (summary == null) {
        PlaceholderScreen(title = "Записать дозу", onBack = back)
        return
    }

    // Draft and step live here, not hoisted into the shell: neither is a fact about the
    // patient until «Сохранить дозу», and a hoisted draft would outlive a cancel.
    // Opens on `nextDose` — the same field the Today hero and action sheet build their
    // "waiting" line from, so three surfaces can't disagree about what's due, and on a
    // two-items-due day it's the one thing that already picks between them.
    var draft by remember {
        mutableStateOf(
            summary.nextDose?.let {
                DoseDraft(itemId = it.itemId, kind = it.kind, dose = it.dose, vialId = openedVial)
            } ?: DoseDraft(vialId = openedVial),
        )
    }
    var step by remember { mutableStateOf(DoseStep.COMPOUND) }
    var refusal by remember { mutableStateOf<String?>(null) }
    // Guards one write per wizard: two taps in the same frame both pass canSubmit() (a pure
    // function of the draft) and queue two coroutines, so without this the second submit ran
    // before the pop tore the scope down and logged a second, unreal occurrence.
    var submitting by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    DoseWizard(
        draft = draft,
        step = step,
        options = summary.doseOptions(cabinet),
        suggestedSite = summary.suggestedSite,
        notice = refusal,
        onDraft = {
            draft = it
            refusal = null
        },
        onStep = { step = it },
        onSubmit = {
            if (!submitting) {
                submitting = true
                scope.launch {
                    try {
                        // Pop only on a write: a wizard that closed on a refusal would throw
                        // away five steps of the patient's answers and say nothing at all.
                        when (val result = onDoseLogged(draft)) {
                            is DoseLogResult.Written -> back()
                            else -> refusal = result.reasonRu()
                        }
                    } finally {
                        submitting = false
                    }
                }
            }
        },
        onCancel = back,
    )
}

/**
 * Named so the four modal entries carry the transition by membership, not by each repeating
 * the same overrides — where one omission slides the wrong way with no test to catch it.
 */
private inline fun <reified T : CadenceRoute.Modal> NavGraphBuilder.modal(
    noinline content: @Composable (NavBackStackEntry) -> Unit,
) {
    // No transition overrides here — those live on the NavHost, the only place that sees
    // both sides of a transition. This buys the type constraint: a modal registered as an
    // ordinary push (or vice versa) doesn't compile.
    composable<T> { entry -> content(entry) }
}

/** Exhaustive with no `else`: a fifth `DoseLogResult` answer must be given words here or it won't compile. */
private fun DoseLogResult.reasonRu(): String? =
    when (this) {
        is DoseLogResult.Written -> null
        DoseLogResult.AlreadyLogged -> "Эта доза уже записана сегодня."
        DoseLogResult.NotScheduledToday -> "На сегодня эта доза не запланирована."
        DoseLogResult.Incomplete -> "Не хватает данных — вернитесь на шаг назад."
    }

/**
 * `weekProtocol` already resolves compound, dose and today's status, so the wizard can't
 * disagree with the strip. Only loggable items: a weigh-in is on the protocol but isn't a dose.
 * `syringeUnits` is null until a vial says the concentration — see docs/prototype-divergences.md.
 */
internal fun TodaySummary?.doseOptions(cabinet: InventorySummary? = null): List<DoseOption> =
    this
        ?.weekProtocol
        .orEmpty()
        .filter { it.loggable && it.dose != null }
        .map { row ->
            DoseOption(
                itemId = row.itemId,
                nameRu = row.compound?.nameRu ?: "—",
                kind = row.kind,
                dose = row.dose,
                syringeUnits = null,
                modeRu = modeRu(row),
                dueToday = row.todayStatus != null,
                // Fullest first: the picker draws nothing for one vial; two is the case §03
                // allows and only the patient can settle.
                vials =
                    cabinet
                        ?.active
                        .orEmpty()
                        .filter { it.compound?.id == row.compound?.id }
                        .sortedByDescending { it.remaining },
            )
        }

/**
 * Built from the same `nextDose`/`nextDoseCompound` fields the Today hero draws, so the
 * sheet and card can't disagree about what's waiting. Null when nothing is due.
 */
private fun TodaySummary.doseDueLine(): String? {
    val name = nextDoseCompound?.nameRu ?: return null
    val dose = nextDose?.dose ?: return null
    val (value, unit) = formatDose(dose)

    return "$name · $value $unit ждёт"
}

/** «п/к · еженедельно» — the route the clinic wrote, and the cadence. */
internal fun modeRu(row: ProtocolRow): String =
    listOfNotNull(
        row.compound?.route,
        when (row.cadence) {
            ProtocolCadence.WEEKLY -> "еженедельно"
            ProtocolCadence.DAILY -> if (row.times.size > 1) "${row.times.size}× в день" else "ежедневно"
            ProtocolCadence.N_PER_WEEK -> "${row.times.size}× в неделю"
        },
    ).joinToString(" · ")

/**
 * A closer look at the card behind it — a sheet rather than a route, like the
 * prototype's, so the cabinet stays where the patient left it.
 */
@Composable
private fun VialSheet(
    detail: VialDetail?,
    today: LocalDate?,
    onDismiss: () -> Unit,
    onLogDose: (VialId) -> Unit,
) {
    if (detail == null) return

    CadenceSheet(open = true, onDismiss = onDismiss) {
        VialDetailSheet(detail = detail, today = today ?: MockSeed.cycleStart, onLogDose = onLogDose)
    }
}

@Composable
private fun TodayRoute(
    nav: NavHostController,
    summary: TodaySummary?,
    todayMeals: List<Meal>,
    zone: TimeZone,
    onOpenActions: () -> Unit,
    body: @Composable () -> Unit,
) {
    if (summary == null) {
        body()
    } else {
        TodayScreen(
            summary = summary,
            patientName = PATIENT_NAME,
            meals = todayMeals,
            zone = zone,
            onLogDose = { nav.openRoute(CadenceRoute.LogDose()) },
            onOpenJournal = { nav.openRoute(CadenceRoute.Journal) },
            onOpenQuickFeel = { nav.openRoute(CadenceRoute.Journal) },
            onOpenVials = { nav.selectDestination(CadenceDestination.INVENTORY) },
            onOpenTrends = { nav.selectDestination(CadenceDestination.TRENDS) },
            onOpenChat = { nav.openRoute(CadenceRoute.ChatThread(PRIMARY_THREAD)) },
            onOpenSchedule = { nav.openRoute(CadenceRoute.Schedule) },
            onOpenLearn = { nav.openRoute(CadenceRoute.Learn) },
            onOpenProfile = { nav.openRoute(CadenceRoute.Profile) },
            onLogMeal = { nav.openRoute(CadenceRoute.LogMeal) },
            onOpenRecipes = { nav.openRoute(CadenceRoute.Recipes) },
            onOpenNutrition = { nav.selectDestination(CadenceDestination.NUTRITION) },
            onSelectTab = nav::selectDestination,
            onOpenActions = onOpenActions,
        )
    }
}

/** The sheet the `+` opens — four values it takes as parameters, and two taps. */
@Composable
private fun Actions(
    summary: TodaySummary?,
    open: Boolean,
    nav: NavHostController,
    onDismiss: () -> Unit,
) {
    ActionChooserSheet(
        open = open,
        doseLogged = summary?.doseLoggedToday == true,
        doseDue = summary?.doseDueLine(),
        mealCount = summary?.mealCount ?: 0,
        mealKcal = summary?.mealMacros?.kcal ?: 0,
        onDismiss = onDismiss,
        onPickDose = {
            onDismiss()
            nav.openRoute(CadenceRoute.LogDose())
        },
        onPickMeal = {
            onDismiss()
            nav.openRoute(CadenceRoute.LogMeal)
        },
    )
}

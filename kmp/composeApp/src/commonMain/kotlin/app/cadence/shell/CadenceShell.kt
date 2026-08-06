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
import app.cadence.screens.schedule.ScheduleScreen
import app.cadence.screens.schedule.ScheduleState
import app.cadence.screens.today.TodayScreen
import app.cadence.screens.trends.TrendDetailScreen
import app.cadence.screens.trends.TrendsScreen
import app.cadence.shared.domain.DoseDraft
import app.cadence.shared.domain.DoseStep
import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.domain.InjectionSite
import app.cadence.shared.domain.InventorySummary
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
import app.cadence.shared.repository.DoseLogResult
import app.cadence.shared.repository.MetricDetail
import app.cadence.shared.repository.TodaySummary
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.datetime.LocalDate

/**
 * The one stand-in left, until the meal wizard lands in step 8 of the block.
 *
 * The day's totals and target are no longer constants — they come from the
 * repository. What the wizard still owes is which meal was logged, and that
 * needs the wizard.
 */
private const val PLACEHOLDER_MEAL_NAME = "Обед"

/** Until sign-in says whose app this is — block 7. */
private const val PATIENT_NAME = "Марина"
private const val PRIMARY_THREAD = "ksenia"

/** §03: `protocols.weeks (12)`. Read from the protocol once one is fetched. */
private const val CYCLE_WEEKS = 12

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
 * The whole after-sign-in surface: the graph, the sheet the `+` opens, and the
 * card a logged meal raises.
 *
 * Two pieces of state, both about what is on screen right now and neither
 * derived from anything: whether the sheet is open, and what the toast is
 * showing. Both are the prototype's — `actionSheetOpen` in the navigator,
 * `confirmSheet` in the app state.
 *
 * The timer lives here rather than in [ConfirmToast] because how long something
 * stays on screen is a property of the screen. `showConfirm` in the prototype
 * puts it in the same place, for the same reason.
 *
 * **There is no failure path yet, and this is where it will go.** Neither the
 * read nor the write handles an exception, because the mock cannot throw on any
 * reachable path. The Ktor client can, so «swapping the mock is a change to one
 * file» is true of the repositories and not of this composable: a *screen* will
 * still not change, but this will.
 */
@Composable
fun CadenceApp(
    navController: NavHostController = rememberNavController(),
    modifier: Modifier = Modifier,
    // Wound to the seed's own day, not to the system clock. `MockSeed.DEMO_NOW`
    // carries why: the fixture is a course with an end date, and reading the
    // real clock empties every screen the moment that date passes.
    mocks: CadenceMocks = remember { CadenceMocks(FixedCadenceClock.at(MockSeed.DEMO_NOW)) },
) {
    val scope = rememberCoroutineScope()
    var summary by remember { mutableStateOf<TodaySummary?>(null) }
    // Bumped by every write, so the next read goes back to the repository
    // rather than to a snapshot taken before it. The real client will invalidate
    // the same way; nothing about this line knows it is talking to a mock.
    var reloads by remember { mutableStateOf(0) }

    // «3 месяца», the prototype's own initial timeframe. Held here rather than
    // on either screen because both read it: the list and the detail share the
    // choice, and a window remembered inside one would reset every time the
    // patient came back from the other.
    var trendWindow by remember { mutableStateOf(TrendWindow.THREE_MONTHS) }
    var trends by remember { mutableStateOf<TrendsOverview?>(null) }

    var schedule by remember { mutableStateOf<ScheduleState?>(null) }
    var cabinet by remember { mutableStateOf<InventorySummary?>(null) }
    var openVial by remember { mutableStateOf<VialDetail?>(null) }

    LaunchedEffect(reloads) {
        val today = mocks.today.today()
        summary = today
        schedule = mocks.scheduleFor(today)
        cabinet = mocks.inventory.cabinet()
    }

    // Keyed on the window as well as on the reload counter: switching «7 дней»
    // has to re-read, and a single effect keyed on `reloads` alone would leave
    // the chips changing nothing.
    LaunchedEffect(trendWindow, reloads) {
        trends = mocks.trends.overview(trendWindow)
    }

    var actionsOpen by remember { mutableStateOf(false) }
    var toast by remember { mutableStateOf<ConfirmToastState?>(null) }
    // Keyed on a counter rather than on the toast itself. ConfirmToastState is a
    // data class and mutableStateOf compares structurally, so raising an equal
    // toast inside the window would be no assignment at all: no state change,
    // no restart, and the second confirmation would inherit the remainder of
    // the first one's life. The prototype clears its timeout before re-arming,
    // for the same reason.
    //
    // No UI test reaches this, and that is not an oversight: the overlay
    // swallows touches for the whole window, so a second meal cannot be logged
    // by tapping until the first toast is gone — see
    // ConfirmToastTest.theToastSwallowsEveryTouchWhileItIsUp. The counter stays
    // because the moment anything raises a toast without a tap behind it — a
    // repository push, a recipe added to the day — the equality trap is live
    // again, and it costs one Int.
    var raisedAt by remember { mutableStateOf(0) }

    ToastLifetime(raisedAt, toast) { toast = null }

    Box(modifier.fillMaxSize()) {
        CadenceShell(
            navController = navController,
            onOpenActions = { actionsOpen = true },
            onDoseLogged = { draft ->
                val result = mocks.dosing.submit(draft)
                reloads++
                result
            },
            summary = summary,
            schedule = schedule,
            cabinet = cabinet,
            onOpenVial = { scope.launch { openVial = mocks.inventory.vial(it) } },
            onAddVial = { draft ->
                scope.launch {
                    mocks.inventory.addVial(draft)
                    reloads++
                }
            },
            trends = trends,
            trendWindow = trendWindow,
            onTrendWindow = { trendWindow = it },
            onLoadMetric = { metric, window -> mocks.trends.metric(metric, window) },
            onMealLogged = { name ->
                // The day's running total, from the repository — §03 puts
                // exactly that in the toast, not the meal's own figure.
                toast = ConfirmToastState(name, summary?.mealMacros?.kcal ?: 0)
                raisedAt++
            },
        )

        Actions(summary, actionsOpen, navController, onDismiss = { actionsOpen = false })

        VialSheet(openVial, today = summary?.date, onDismiss = { openVial = null }, onLogDose = { vialId ->
            openVial = null
            // Named, not dropped. `VialDetailSheet` has always reported which
            // vial the button belongs to; the shell threw it away, so the
            // wizard opened with a null draft and the picker defaulted to the
            // *fullest* open vial of that compound. A patient who opened the
            // half-empty one and stepped through without noticing the picker
            // wrote the dose against the other, leaving both counts wrong and
            // nothing to reconcile them against.
            navController.openRoute(CadenceRoute.LogDose(vialId.raw))
        })

        ConfirmToast(state = toast, targetKcal = summary?.targets?.kcal ?: 0)
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
 * The after-sign-in host: the screen graph and the overlays above it.
 *
 * Screens get named lambdas and never the controller, which is how the
 * prototype wires its `Stack.Screen` children and what keeps a screen testable
 * without a navigator. `onOpenActions` is the one callback that is not
 * navigation: the `+` in the bar opens a sheet, and a sheet is not a place.
 *
 * Takes its controller as a parameter so a test can assert on the back stack —
 * «did that tap navigate» is not answerable from what is on screen when two
 * routes render the same word.
 */
@Composable
fun CadenceShell(
    navController: NavHostController = rememberNavController(),
    modifier: Modifier = Modifier,
    onOpenActions: () -> Unit = { },
    onMealLogged: (String) -> Unit = { },
    onDoseLogged: suspend (DoseDraft) -> DoseLogResult = { DoseLogResult.Incomplete },
    onAddVial: (VialDraft) -> Unit = { },
    onOpenVial: (VialId) -> Unit = { },
    cabinet: InventorySummary? = null,
    trends: TrendsOverview? = null,
    trendWindow: TrendWindow = TrendWindow.THREE_MONTHS,
    onTrendWindow: (TrendWindow) -> Unit = { },
    onLoadMetric: suspend (Metric, TrendWindow) -> MetricDetail? = { _, _ -> null },
    // The sections that are ported. The rest of the graph still draws
    // placeholders, and a null summary means the first read has not landed —
    // the screen is not composed until it has, rather than being shown a day
    // made of zeroes.
    summary: TodaySummary? = null,
    schedule: ScheduleState? = null,
) {
    NavHost(
        navController = navController,
        startDestination = CADENCE_ROOT,
        modifier = modifier.fillMaxSize(),
        // The transitions live here, on the NavHost, because Compose reads
        // each side from the destination it belongs to: a screen's exit is
        // taken from the screen being *left*, not from the one arriving. Two
        // overrides on the modal itself therefore could not hold the screen
        // beneath it still — they fired when leaving the modal forward, which
        // is not the same event and not the one that drifts.
        enterTransition = { if (targetState.destination.isModal()) modalEnter() else pushEnter() },
        exitTransition = { if (targetState.destination.isModal()) modalUnderlayExit() else pushExit() },
        popEnterTransition = { if (initialState.destination.isModal()) modalUnderlayEnter() else popEnter() },
        popExitTransition = { if (initialState.destination.isModal()) modalExit() else popExit() },
    ) {
        tabRoutes(navController, onOpenActions, summary, cabinet, onOpenVial, trends, onTrendWindow)
        pushedRoutes(navController, schedule, trendWindow, onTrendWindow, onLoadMetric)
        modalRoutes(navController, onMealLogged, onDoseLogged, onAddVial, summary, cabinet)
    }
}

/**
 * The four the bottom bar reaches. Only these carry the bar.
 *
 * Written as a `when` over the enum rather than four literal `composable<…>`
 * calls so that adding a fifth destination fails to compile here too, not only
 * in [CadenceDestination.route].
 */
@Suppress("LongParameterList")
private fun NavGraphBuilder.tabRoutes(
    nav: NavHostController,
    onOpenActions: () -> Unit,
    summary: TodaySummary?,
    cabinet: InventorySummary?,
    onOpenVial: (VialId) -> Unit,
    trends: TrendsOverview?,
    onTrendWindow: (TrendWindow) -> Unit,
) {
    CadenceDestination.entries.forEach { destination ->
        val body: @Composable () -> Unit = {
            PlaceholderScreen(
                title = destination.label,
                destination = destination,
                // Today is the root: there is nothing behind it to go back to.
                // The other three use goBack, as the prototype does — identical
                // to popToTop only while a tab always sits at depth 1, which
                // stops being true the moment a real screen reaches one from
                // deeper (RecipeDetail → Nutrition).
                onBack = if (destination == CadenceDestination.TODAY) null else back(nav),
                onSelectTab = nav::selectDestination,
                onLog = onOpenActions,
            )
        }
        when (destination) {
            CadenceDestination.TODAY -> {
                composable<CadenceRoute.Today> {
                    TodayRoute(nav, summary, onOpenActions, body)
                }
            }

            CadenceDestination.INVENTORY -> {
                composable<CadenceRoute.Vials> {
                    // Gated on the read like Today and the schedule: a cabinet
                    // composed against no data draws «0 флаконов», which is a
                    // sentence about the patient rather than about the fetch.
                    if (cabinet == null) {
                        body()
                    } else {
                        VialsScreen(
                            cabinet = cabinet,
                            onOpenVial = onOpenVial,
                            onAddVial = { nav.openRoute(CadenceRoute.AddVial) },
                        )
                    }
                }
            }

            CadenceDestination.TRENDS -> {
                composable<CadenceRoute.Trends> {
                    // Gated on the read like Today and the cabinet: a list
                    // composed against no data draws «нет данных» on all eight
                    // cards, which is a sentence about the patient rather than
                    // about the fetch.
                    if (trends == null) {
                        body()
                    } else {
                        TrendsScreen(
                            overview = trends,
                            onOpenMetric = { nav.openRoute(CadenceRoute.TrendDetail(it.code)) },
                            onWindowChange = onTrendWindow,
                            onOpenJournal = { nav.openRoute(CadenceRoute.Journal) },
                            onOpenBody = { nav.openRoute(CadenceRoute.Body) },
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
 * The way out of every screen that is not the root.
 *
 * One definition rather than one per graph section: `popBackStack()` spelled
 * inline in three places is three chances for one of them to become `{ }` and
 * strand a screen, which is what the tests could not see until they clicked it.
 */
private fun back(nav: NavHostController): () -> Unit = { nav.popBackStack() }

/** Everything reached by a push: slides in from the right, backed out of. */
@Suppress("LongParameterList")
private fun NavGraphBuilder.pushedRoutes(
    nav: NavHostController,
    schedule: ScheduleState?,
    trendWindow: TrendWindow,
    onTrendWindow: (TrendWindow) -> Unit,
    onLoadMetric: suspend (Metric, TrendWindow) -> MetricDetail?,
) {
    val back = back(nav)

    composable<CadenceRoute.TrendDetail> { entry ->
        // The route carries a `String`, so this is where it becomes a metric or
        // fails to. `null` reaches the screen as «такой метрики нет» — the
        // prototype has a `thigh` and a `bmi` the data model does not, and a
        // deep link can carry anything at all.
        val code = entry.toRoute<CadenceRoute.TrendDetail>().biomarkerId
        val metric = Metric.fromCode(code)
        // Keyed on the code alone, not on the window. A window change re-reads
        // — the effect below is keyed on both — but the previous chart and the
        // chip row stay on screen until the new answer lands, which is the same
        // stale-while-revalidate the list does: `CadenceApp` never nulls
        // `trends` when the window changes. Nulling here took the control the
        // patient had just used away from them mid-tap.
        var detail by remember(code) { mutableStateOf<MetricDetail?>(null) }
        // Reset with the metric and the window, so a position scrubbed on one
        // chart never lands on another's readings.
        var scrubIndex by remember(code, trendWindow) { mutableStateOf<Int?>(null) }

        LaunchedEffect(code, trendWindow) {
            detail = metric?.let { onLoadMetric(it, trendWindow) }
        }

        // «Ещё не пришло» is not «такой метрики нет». They are one value on
        // the screen — a null `detail` — and two different facts here, so the
        // graph tells them apart the way the schedule route does. Without this,
        // tapping a window chip on the detail wipes the chart *and* the chip
        // row and answers «Такой метрики нет» until the read lands: the patient
        // loses the data and the control they just used.
        if (metric != null && detail == null) {
            PlaceholderScreen("Метрика", onBack = back)
        } else {
            TrendDetailScreen(
                detail = detail,
                window = trendWindow,
                onWindowChange = onTrendWindow,
                onBack = back,
                scrubIndex = scrubIndex,
                onScrub = { index, _ -> scrubIndex = index },
            )
        }
    }
    composable<CadenceRoute.Schedule> {
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

/**
 * The prototype's `Stack.Group` with `presentation: 'fullScreenModal'`: these
 * slide up rather than in, and every one of them ends by dismissing itself.
 */
@Suppress("LongParameterList")
private fun NavGraphBuilder.modalRoutes(
    nav: NavHostController,
    onMealLogged: (String) -> Unit,
    onDoseLogged: suspend (DoseDraft) -> DoseLogResult,
    onAddVial: (VialDraft) -> Unit,
    summary: TodaySummary?,
    cabinet: InventorySummary?,
) {
    val back = back(nav)
    val today = summary

    modal<CadenceRoute.LogDose> { entry ->
        LogDoseModal(
            summary = summary,
            cabinet = cabinet,
            openedVial = entry.toRoute<CadenceRoute.LogDose>().vialId?.let(::VialId),
            onDoseLogged = onDoseLogged,
            back = back,
        )
    }
    modal<CadenceRoute.LogMeal> {
        PlaceholderScreen(
            title = "Записать приём пищи",
            onBack = back,
            // The prototype's LogMeal hands the meal to the app, which raises
            // the toast and dismisses. The placeholder hands over a fixed one
            // so the toast is reachable before the wizard is ported.
            action =
                "Записать" to {
                    onMealLogged(PLACEHOLDER_MEAL_NAME)
                    back()
                },
        )
    }
    modal<CadenceRoute.AddVial> {
        // Gated on the read like its three siblings, which this one was not.
        // `today` is the only thing standing between the form and expired
        // stock — `VialDraft.canSave` guards on `expiresOn >= today`, and the
        // screen shows «Срок уже истёк» from the same value — so substituting
        // `MockSeed.cycleStart` for a summary that had not landed let a vial
        // expiring three months ago save with no warning. The repository
        // re-checked against the real date and returned `Rejected`, which the
        // caller discarded, so the screen popped as though it had saved.
        val day = today?.date

        if (day == null) {
            PlaceholderScreen(title = "Добавить флакон", onBack = back)
        } else {
            AddVialScreen(
                compounds = MockSeed.compounds,
                today = day,
                onSave = {
                    onAddVial(it)
                    back()
                },
                onCancel = back,
            )
        }
    }
    modal<CadenceRoute.RecipeBuilder> { PlaceholderScreen("Новый рецепт", onBack = back) }
}

/**
 * The dose wizard, and the two things the shell owes it.
 *
 * Gated on the read, like the other ported routes. Composed against a null
 * summary the wizard draws no compounds, so «Дальше» is dead on step 1 and the
 * only way out is «Отмена» — a full-screen dead end, one frame wide against the
 * mock and a network round trip against the client this file anticipates.
 *
 * [openedVial] is the vial the patient had open when they tapped, or null when
 * they came from the tab bar or the Today hero. It seeds the draft; the picker
 * on step 2 can still override it.
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

    // The wizard holds its own draft and step for as long as it is on screen:
    // neither is a fact about the patient until «Сохранить дозу», and a draft
    // hoisted into the shell would outlive a cancel.
    var draft by remember { mutableStateOf(DoseDraft(vialId = openedVial)) }
    var step by remember { mutableStateOf(DoseStep.COMPOUND) }
    var refusal by remember { mutableStateOf<String?>(null) }
    // One write per wizard. `canSubmit()` is a pure function of the draft, so
    // it does not change while the write is in flight: two taps in the same
    // frame queued two coroutines, the first closed the 08:00 slot and popped,
    // and the second — running before the pop tore the scope down — found the
    // *20:00* occurrence still open and recorded that too. An evening injection
    // marked done that the patient never took, and a vial short by one. The
    // mock makes the window a frame wide; the Ktor client makes it seconds.
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
                        // Pop only on a write. A repository that refuses — the
                        // day rolled over, the occurrence is already logged —
                        // and a wizard that closed anyway would throw away five
                        // steps of the patient's answers and say nothing at all.
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
 * A route in the modal group: up rather than in, and back down again.
 *
 * Named so the four entries carry the transition by membership, the way the
 * prototype's `Stack.Group` does, instead of each repeating the same two
 * overrides — where one omission would be a screen that slides the wrong way
 * and no test would say so.
 */
private inline fun <reified T : CadenceRoute.Modal> NavGraphBuilder.modal(
    noinline content: @Composable (NavBackStackEntry) -> Unit,
) {
    // No transition overrides: they belong on the NavHost, which is the only
    // place that can see both sides of a transition at once. What this builder
    // still buys is the type constraint — registering an ordinary route as a
    // modal, or a modal as an ordinary push, does not compile.
    //
    // The entry is handed on because `LogDose` carries an argument: which vial
    // the patient opened before tapping «Записать дозу».
    composable<T> { entry -> content(entry) }
}

/**
 * What the wizard says when the repository refuses.
 *
 * Exhaustive with no `else`: a fifth answer added to `DoseLogResult` has to be
 * given words here or it will not compile, which is the whole reason the result
 * is a sealed type rather than a nullable id.
 */
private fun DoseLogResult.reasonRu(): String? =
    when (this) {
        is DoseLogResult.Written -> null
        DoseLogResult.AlreadyLogged -> "Эта доза уже записана сегодня."
        DoseLogResult.NotScheduledToday -> "На сегодня эта доза не запланирована."
        DoseLogResult.Incomplete -> "Не хватает данных — вернитесь на шаг назад."
    }

/**
 * What «Что вы приняли?» offers, from the week's protocol.
 *
 * `weekProtocol` already resolves the compound, the dose in force and today's
 * status, so the wizard needs no repository of its own — and cannot disagree
 * with the strip about what today's dose is. Only loggable items: a weigh-in is
 * on the protocol and is not a dose.
 *
 * `syringeUnits` is null until a vial says what the concentration is — see
 * `docs/prototype-divergences.md`.
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
                // The open vials of this compound, fullest first. The picker
                // draws nothing when there is one; two is the case §03 allows
                // and only the patient can settle.
                vials =
                    cabinet
                        ?.active
                        .orEmpty()
                        .filter { it.compound?.id == row.compound?.id }
                        .sortedByDescending { it.remaining },
            )
        }

/**
 * «Семаглутид · 0,25 мг ждёт» — the next dose, in the protocol's own words.
 *
 * Null when the day prescribes nothing: the sheet says so rather than naming a
 * compound that is not due. Built from `nextDose` and `nextDoseCompound`, the
 * same two fields the Today hero draws, so the sheet and the card behind it
 * cannot disagree about what is waiting.
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
    onOpenActions: () -> Unit,
    body: @Composable () -> Unit,
) {
    if (summary == null) {
        body()
    } else {
        TodayScreen(
            summary = summary,
            patientName = PATIENT_NAME,
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

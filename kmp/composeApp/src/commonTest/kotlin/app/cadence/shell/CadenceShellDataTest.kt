package app.cadence.shell

import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.onAllNodesWithContentDescription
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavGraph
import androidx.navigation.NavHostController
import androidx.navigation.compose.rememberNavController
import androidx.navigation.toRoute
import app.cadence.design.CadenceTheme
import app.cadence.screens.inventory.ADD_VIAL_SAVE_TAG
import app.cadence.screens.inventory.addVialFieldTag
import app.cadence.screens.nutrition.CADENCE_NUTRITION_RECIPES_LINK_TAG
import app.cadence.screens.nutrition.CADENCE_NUTRITION_TAG
import app.cadence.screens.nutrition.LOG_MEAL_CHAT_FIELD_TAG
import app.cadence.screens.nutrition.LOG_MEAL_SAVE_TAG
import app.cadence.screens.recipes.CADENCE_RECIPES_TAG
import app.cadence.screens.recipes.CADENCE_RECIPE_DETAIL_TAG
import app.cadence.screens.trends.CADENCE_TRENDS_HERO_TAG
import app.cadence.screens.trends.CADENCE_TREND_DETAIL_STATS_TAG
import app.cadence.screens.trends.cadenceTrendCardTag
import app.cadence.screens.trends.cadenceTrendWindowTag
import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.ProtocolCadence
import app.cadence.shared.domain.ProtocolItemId
import app.cadence.shared.domain.ProtocolItemKind
import app.cadence.shared.domain.ProtocolRow
import app.cadence.shared.domain.TrendWindow
import app.cadence.shared.mock.CadenceMocks
import app.cadence.shared.mock.MockSeed
import app.cadence.shared.parsing.MockMealParser
import app.cadence.shared.repository.MealLogResult
import app.cadence.shared.repository.TodaySummary
import kotlinx.coroutines.test.runTest
import kotlinx.datetime.LocalTime
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

private val ZONE = TimeZone.of("Europe/Moscow")

/** The app on a day the test chose, reading a repository the test wound. */
@OptIn(ExperimentalTestApi::class)
private fun mocks(iso: String = "2026-05-31T09:00:00Z") = CadenceMocks(FixedCadenceClock.at(iso), ZONE)

/** A slot time for a hand-built row; nothing reads the hour. */
private val MORNING = LocalTime.parse("08:00")

@OptIn(ExperimentalTestApi::class)
class CadenceShellDataTest {
    /**
     * The tab, not the word: «Питание» is also the eyebrow of Today's own meals card, so a
     * text query matches two nodes and clicks neither.
     */
    private fun ComposeUiTest.nutritionTab() = onNodeWithContentDescription("Питание")

    /** A row with the cadence and slot count a case needs, and nothing else real. */
    private fun row(
        cadence: ProtocolCadence,
        times: Int,
    ) = ProtocolRow(
        itemId = ProtocolItemId("item"),
        kind = ProtocolItemKind.SUPPLEMENT,
        compound = MockSeed.glycine,
        dose = null,
        times = List(times) { MORNING },
        cadence = cadence,
        todayStatus = null,
        loggable = true,
    )

    /** The zone «Предложено: X — следующее в ротации» names, on the site step. */
    private fun ComposeUiTest.suggestedZone(): String =
        onNodeWithText("Предложено:", substring = true)
            .fetchSemanticsNode()
            .config
            .getOrNull(SemanticsProperties.Text)
            ?.firstOrNull()
            ?.text
            .orEmpty()
            .substringAfter("Предложено: ")
            .substringBefore(" —")

    @Test
    fun theSheetShowsTheDayTheRepositoryReportsAndNotAConstant() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()

            // Two seeded meals of 320 and 520 kcal, formatted on the way to the screen — «840»
            // is not a string anything upstream holds.
            onNodeWithText("2 приёма сегодня · 840 ккал").assertIsDisplayed()
            onNodeWithText("Семаглутид · 0,25 мг ждёт").assertIsDisplayed()
        }

    @Test
    fun theRecentMealsListOnTheRealTodayScreenReadsTheRepositoryAndTheMocksZone() =
        runComposeUiTest {
            // `TodayScreenTest`/`CadenceFormatTest` drive `TodayScreen` directly with hand-built
            // args, so neither exercises `CadenceApp`'s own todayMeals/zone lines — mutating
            // either to a constant left every other gate green.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            // Seeded breakfast, 06:30 UTC — 09:30 in Moscow via mocks().zone.
            onNodeWithText("09:30 · 1 позиция").performScrollTo().assertIsDisplayed()
        }

    @Test
    fun theAppOpensOnTheRealTodayScreen() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            // Placeholder is gone; the ported screen is what the tab bar lands on.
            onNodeWithText("Воскресенье, день · 4-я неделя").assertIsDisplayed()
            assertTrue(onAllNodesWithText("Экран «Сегодня»").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun theScheduleIsReachedFromTodayAndAgreesWithIt() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = mocks()) }
            }

            onNodeWithText("Весь график").performScrollTo().performClick()
            waitForIdle()

            // The same week, from the same generator, on the other screen.
            onNodeWithText("неделя 4 из 12").assertIsDisplayed()
            assertTrue(onAllNodesWithText("Экран «Расписание»").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun theWizardIsOfferedTheProtocolsLoggableItemsAndNothingElse() =
        runTest {
            // Tests the mapping directly: the seed has no loggable non-injection item, so
            // through the screen a wizard that stamped INJECTION on everything is invisible.
            val options = mocks().today.today().doseOptions()

            assertEquals(
                listOf("Семаглутид", "BPC-157"),
                options.map { it.nameRu },
                "the wizard offers something the protocol does not mark loggable",
            )
            // No kind assertion: both seeded items are injections, so it'd guard nothing —
            // see DoseWizardTest.anItemThatIsNotAnInjectionNeedsNoZoneToGetPast.
            assertEquals(listOf(true, true), options.map { it.dueToday })
            assertEquals("п/к · еженедельно", options.first().modeRu)
            assertEquals("п/к · 2× в день", options.last().modeRu)
        }

    @Test
    fun onlyTheItemsDueTodayCarryTheBadge() =
        runTest {
            // A Tuesday: daily injection due, weekly not. Earlier assertions ran on the seeded
            // Sunday, where both are due, so a `dueToday` hardcoded true was invisible.
            val options = mocks("2026-06-02T09:00:00Z").today.today().doseOptions()

            assertEquals(listOf("Семаглутид", "BPC-157"), options.map { it.nameRu })
            assertEquals(listOf(false, true), options.map { it.dueToday })
        }

    @Test
    fun theModeNamesEveryCadenceTheProtocolCanHave() =
        runTest {
            // Two of three branches are unreachable from the seed (its only daily item has two
            // times, nothing is N-per-week), hence testing them directly.
            assertEquals("внутрь · ежедневно", modeRu(row(ProtocolCadence.DAILY, times = 1)))
            assertEquals("внутрь · 3× в день", modeRu(row(ProtocolCadence.DAILY, times = 3)))
            assertEquals("внутрь · 2× в неделю", modeRu(row(ProtocolCadence.N_PER_WEEK, times = 2)))
        }

    @Test
    fun theWizardOpensOnAZoneTheRotationHasNotJustUsed() =
        runComposeUiTest {
            // Read from the screen rather than computed: computing here would only be the
            // production rule asked twice.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Записать →").performClick()
            waitForIdle()
            onNodeWithText("Семаглутид").performClick()
            waitForIdle()
            repeat(2) {
                onNodeWithText("Дальше").performClick()
                waitForIdle()
            }
            val suggested = suggestedZone()

            // May be on the back of the body: with real history the rotation moved off the front.
            if (onAllNodesWithContentDescription(suggested).fetchSemanticsNodes().isEmpty()) {
                onNodeWithText("Сзади").performClick()
                waitForIdle()
            }
            onNodeWithContentDescription(suggested).performScrollTo().performClick()
            waitForIdle()
            repeat(2) {
                onNodeWithText("Дальше").performClick()
                waitForIdle()
            }
            onNodeWithText("Сохранить дозу").performClick()
            waitForIdle()

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()
            onNodeWithText("Записать дозу").performClick()
            waitForIdle()
            onNodeWithText("Семаглутид").performClick()
            waitForIdle()
            repeat(2) {
                onNodeWithText("Дальше").performClick()
                waitForIdle()
            }

            assertNotEquals(suggested, suggestedZone(), "the wizard suggests the zone just injected")
        }

    @Test
    fun theCabinetTabDrawsTheVialsRatherThanAPlaceholder() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Аптечка").performClick()
            waitForIdle()

            onNodeWithText("Ваша аптечка").assertExists()
            onNodeWithText("4 флакона в холодильнике").assertExists()
        }

    // The three below walk the bar on the *ported* screens: `CadenceNavigationTest
    // .onlyTheFourBarDestinationsCarryTheBar` starts a shell with no data, so all four tabs
    // are placeholders drawing their own bar — live screens shipped without one once, a tab
    // a patient can't leave on iOS.

    @Test
    fun theCabinetTabKeepsTheBarThatReachesTheOtherTabs() =
        runComposeUiTest {
            // Mutation: drop `CadenceTabBar` from `VialsScreen`.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithContentDescription("Аптечка").performClick()
            waitForIdle()
            onNodeWithText("Ваша аптечка").assertExists()

            onNodeWithContentDescription("Тренды").performClick()
            waitForIdle()

            onNodeWithTag(CADENCE_TRENDS_HERO_TAG).assertExists()
        }

    @Test
    fun theTrendsTabKeepsTheBarThatReachesTheOtherTabs() =
        runComposeUiTest {
            // Mutation: drop `CadenceTabBar` from `TrendsScreen`.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithContentDescription("Тренды").performClick()
            waitForIdle()
            onNodeWithTag(CADENCE_TRENDS_HERO_TAG).assertExists()

            onNodeWithContentDescription("Сегодня").performClick()
            waitForIdle()

            onNodeWithText("Воскресенье, день · 4-я неделя").assertExists()
        }

    @Test
    fun theBarOnAPortedScreenMarksTheTabThatScreenIs() =
        runComposeUiTest {
            // Mutation: pass a constant (`active = CadenceDestination.TODAY`) to either bar.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithContentDescription("Аптечка").performClick()
            waitForIdle()
            onNodeWithContentDescription("Аптечка").assertIsSelected()
            onNodeWithContentDescription("Сегодня").assertIsNotSelected()

            onNodeWithContentDescription("Тренды").performClick()
            waitForIdle()
            onNodeWithContentDescription("Тренды").assertIsSelected()
            onNodeWithContentDescription("Аптечка").assertIsNotSelected()
        }

    @Test
    fun theWizardOpensOnTheVialTheSheetWasAbout() =
        runComposeUiTest {
            // The shell once dropped the vial id (`onLogDose = { onLogDose() }`), so the
            // picker defaulted to the fullest open vial and a dose got recorded against
            // the wrong one. B-2510 deliberately: it's the emptier of the two, so the
            // default would *not* pick it — opening the fullest would pass either way.
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = mocks()) }
            }

            // Driven by the route, not by walking the cabinet: the sheet's own suite already
            // covers the one-line wiring that hands the id to the route.
            runOnIdle { nav.navigate(CadenceRoute.LogDose("vial-bpc-2")) }
            waitForIdle()

            onNodeWithText("BPC-157").performClick()
            waitForIdle()
            onNodeWithText("Дальше").performClick()
            waitForIdle()

            // vial-bpc-2 is B-2510, the emptier of the two open BPC vials, so the picker's
            // "fullest open vial" default wouldn't pick it — opening the fullest would pass either way.
            onNodeWithText("B-2510", substring = true).performScrollTo().assertIsSelected()
            onNodeWithText("B-2601", substring = true).performScrollTo().assertIsNotSelected()
        }

    @Test
    fun theVialSheetHandsItsOwnIdToTheWizardRoute() =
        runComposeUiTest {
            // The other half of the seam: `theWizardOpensOnTheVialTheSheetWasAbout` drives the
            // route directly, so it can't see the shell dropping the id on the way *to* the
            // route — which it once did: `onLogDose = { onLogDose() }`, argument discarded.
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = mocks()) }
            }

            onNodeWithText("Аптечка").performClick()
            waitForIdle()
            onNodeWithText("B-2510", substring = true).performScrollTo().performClick()
            waitForIdle()
            assertEquals(
                1,
                onAllNodesWithText("Записать дозу").fetchSemanticsNodes().size,
                "the sheet has to be open, and be the only thing offering this",
            )
            onNodeWithText("Записать дозу").performScrollTo().performClick()
            waitForIdle()

            // Read off the entry, not inferred from the screen: "which vial" isn't drawn on step 1.
            val entry = nav.currentBackStackEntry
            assertEquals(
                "LogDose",
                entry
                    ?.destination
                    ?.route
                    ?.substringBefore('/')
                    ?.substringBefore('?')
                    ?.substringAfterLast('.'),
            )
            assertEquals("vial-bpc-2", entry?.toRoute<CadenceRoute.LogDose>()?.vialId)
        }

    @Test
    fun theActionSheetNamesTheDoseThatIsActuallyDue() =
        runComposeUiTest {
            // Week 5, where the course titrates to 0,5 мг. This row regressed to a literal
            // 0,25 мг for three blocks — every earlier test ran on the seeded week-4 Sunday,
            // the one week the literal happened to agree with the truth.
            setContent { CadenceTheme { CadenceApp(mocks = mocks("2026-06-07T09:00:00Z")) } }

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()

            onNodeWithText("Семаглутид · 0,5 мг ждёт").assertIsDisplayed()
            assertEquals(
                0,
                onAllNodesWithText("Семаглутид · 0,25 мг ждёт").fetchSemanticsNodes().size,
                "the literal this row used to carry",
            )
        }

    @Test
    fun addingAVialFromTheCabinetPutsItInTheCabinet() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Аптечка").performClick()
            waitForIdle()
            onNodeWithContentDescription("Добавить флакон").performClick()
            waitForIdle()

            onNodeWithText("Семаглутид").performScrollTo().performClick()
            waitForIdle()
            onNodeWithTag(addVialFieldTag("total"), useUnmergedTree = true).performTextReplacement("20")
            onNodeWithTag(addVialFieldTag("expires"), useUnmergedTree = true).performTextReplacement("2027-01-31")
            waitForIdle()
            onNodeWithTag(ADD_VIAL_SAVE_TAG, useUnmergedTree = true).performClick()
            waitForIdle()

            // Count moved: the write went through the repository and the next read reflected it.
            onNodeWithText("5 флаконов в холодильнике").assertExists()
        }

    @Test
    fun theWizardIsOfferedTheCabinetsOwnOpenVials() =
        runComposeUiTest {
            // Through the shell, not a hand-built option list: the cabinet-to-picker wiring
            // is where six mutants lived, invisible to a wizard test with its own vials.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()
            onNodeWithText("Записать дозу").performClick()
            waitForIdle()
            onNodeWithText("BPC-157").performClick()
            waitForIdle()
            onNodeWithText("Дальше").performClick()
            waitForIdle()

            // The two open BPC vials, fullest first, and no sealed spare.
            onNodeWithText("B-2601").performScrollTo().assertExists()
            onNodeWithText("B-2510").performScrollTo().assertExists()
            assertEquals(0, onAllNodesWithText("B-2610").fetchSemanticsNodes().size, "sealed stock is offered")
            assertEquals(
                0,
                onAllNodesWithText("A-2261").fetchSemanticsNodes().size,
                "another compound's vial is offered",
            )

            // Fullest first: B-2601 has four doses left, B-2510 has two. Read off the rows'
            // lots, since each row merges lot and count into one node led by the lot.
            val order =
                onAllNodesWithText("доз", substring = true)
                    .fetchSemanticsNodes()
                    .mapNotNull {
                        it.config
                            .getOrNull(SemanticsProperties.Text)
                            ?.firstOrNull()
                            ?.text
                    }
            assertEquals(listOf("B-2601", "B-2510"), order)
        }

    @Test
    fun aSingleOpenVialIsNotAChoiceAndIsNotDrawn() =
        runComposeUiTest {
            // Semaglutide has one open vial: a picker with one option is not a picker.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Записать →").performClick()
            waitForIdle()
            onNodeWithText("Семаглутид").performClick()
            waitForIdle()
            onNodeWithText("Дальше").performClick()
            waitForIdle()

            assertEquals(0, onAllNodesWithText("Из вашей аптечки", ignoreCase = true).fetchSemanticsNodes().size)
        }

    @Test
    fun theHeroOpensTheDoseWizard() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = mocks()) }
            }

            onNodeWithText("Записать →").performClick()
            waitForIdle()

            onNodeWithText("Шаг 1 · Препарат", ignoreCase = true).assertIsDisplayed()
            onNodeWithText("Шаг 1 из 5").assertIsDisplayed()
        }

    @Test
    fun theWizardOffersTheProtocolsOwnItemsWithTodaysDose() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Записать →").performClick()
            waitForIdle()

            // A wizard built from a literal list would name whatever the prototype's
            // `COMPOUNDS` held instead of the seeded protocol's two injections.
            onNodeWithText("Семаглутид").assertIsDisplayed()
            onNodeWithText("BPC-157").assertIsDisplayed()
            onNodeWithText("0,25 мг", substring = true).assertIsDisplayed()
        }

    @Test
    fun theWizardOpensOnTheDoseTheDayIsWaitingFor() =
        runComposeUiTest {
            // Mutation: build the draft without an item (`DoseDraft(vialId = openedVial)`,
            // which shipped once) — step 1's "already selected" copy then sits above two
            // empty radios. Asserted through step 2's subtitle, built from the draft's
            // actual option, rather than a checked radio.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Записать →").performClick()
            waitForIdle()

            // Deliberately nothing tapped on step 1.
            onNodeWithText("Дальше").performClick()
            waitForIdle()

            onNodeWithText("По умолчанию для Семаглутид", substring = true).assertIsDisplayed()
        }

    @Test
    fun onADayTheHeroHasNoDoseTheWizardStillPreselectsNothing() =
        runComposeUiTest {
            // Not the wanted behaviour, just the actual one, pinned so the gap is measured
            // rather than assumed away. `TodaySummary.nextDose` is only the weekly injection's
            // occurrence (CadenceMocks.kt:133); on this Tuesday BPC-157 is due twice and
            // semaglutide not at all, so there's nothing to open on. Deciding what *should*
            // preselect here is a product rule this port doesn't have — see
            // docs/prototype-divergences.md.
            setContent { CadenceTheme { CadenceApp(mocks = mocks("2026-06-02T09:00:00Z")) } }

            // Through the sheet, not the hero: the hero draws no «Записать →» when the
            // weekly injection isn't due, which is the same fact this test turns on.
            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()
            onNodeWithText("Записать дозу").performClick()
            waitForIdle()

            onNodeWithText("Дальше").performClick()
            waitForIdle()

            assertTrue(
                onAllNodesWithText("Шаг 2", substring = true, ignoreCase = true)
                    .fetchSemanticsNodes()
                    .isEmpty(),
                "the wizard advanced, so something was preselected after all — " +
                    "update this test and the divergence note together",
            )
        }

    @Test
    fun finishingTheWizardLogsTheDoseAndReturnsToToday() =
        runComposeUiTest {
            // The hero opens the wizard, the wizard writes through the repository, and
            // Today reads the write back.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Записать →").performClick()
            waitForIdle()
            onNodeWithText("Семаглутид").performClick()
            waitForIdle()
            onNodeWithText("Дальше").performClick()
            waitForIdle()
            onNodeWithText("Дальше").performClick()
            waitForIdle()
            onNodeWithContentDescription("Правое плечо").performScrollTo().performClick()
            waitForIdle()
            onNodeWithText("Дальше").performClick()
            waitForIdle()
            onNodeWithText("Дальше").performClick()
            waitForIdle()
            onNodeWithText("Сохранить дозу").performClick()
            waitForIdle()

            // `onAllNodesWithText` is case-sensitive and the strip's word is lowercase where
            // the hero's isn't — one query looked like it covered both and matched only the hero.
            assertTrue(
                onAllNodesWithText("Записано", substring = true).fetchSemanticsNodes().isNotEmpty(),
                "the hero does not say the dose was recorded",
            )
            assertTrue(
                onAllNodesWithText("записано", substring = true).fetchSemanticsNodes().isNotEmpty(),
                "the protocol strip does not say the dose was recorded",
            )
            assertTrue(
                onAllNodesWithText("Записать →").fetchSemanticsNodes().isEmpty(),
                "the hero still offers a dose it already has",
            )
            // Vial decrement is asserted through the repository in
            // MockRepositoryTest.aLoggedDoseComesOutOfTheVialItWasDrawnFrom.
        }

    @Test
    fun anotherDayGivesTheSheetAnotherAnswer() =
        runComposeUiTest {
            // Without this, the shell could hold the seeded day's numbers as literals and
            // every assertion above would still pass — a mutation doing exactly that survived
            // until this test was added.
            setContent { CadenceTheme { CadenceApp(mocks = mocks("2026-06-07T09:00:00Z")) } }

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()

            onNodeWithText("Пока ничего сегодня · начнём ритм").assertIsDisplayed()
        }

    @Test
    fun loggingADoseChangesWhatTheSheetSaysNextTime() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()
            onNodeWithText("Записать дозу").performClick()
            waitForIdle()

            onNodeWithText("Шаг 1 · Препарат", ignoreCase = true).assertIsDisplayed()
            onNodeWithText("Семаглутид").performClick()
            waitForIdle()
            repeat(2) {
                onNodeWithText("Дальше").performClick()
                waitForIdle()
            }
            onNodeWithContentDescription("Правое плечо").performScrollTo().performClick()
            waitForIdle()
            repeat(2) {
                onNodeWithText("Дальше").performClick()
                waitForIdle()
            }
            onNodeWithText("Сохранить дозу").performClick()
            waitForIdle()

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()

            onNodeWithText("Уже записано сегодня · открыть или поправить").assertIsDisplayed()
            assertTrue(
                onAllNodesWithText("Семаглутид · 0,25 мг ждёт").fetchSemanticsNodes().isEmpty(),
                "the sheet is still reading its old answer",
            )
        }

    @Test
    fun theTrendsTabDrawsTheListItReadRatherThanAPlaceholder() =
        runComposeUiTest {
            // The only place this is measured against the *shipped* wiring: `CadenceNavigationTest`
            // hands `CadenceShell` its own overview, proving the test's replica, not `CadenceApp`'s read.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Тренды").performClick()
            waitForIdle()

            onNodeWithTag(CADENCE_TRENDS_HERO_TAG, useUnmergedTree = true).assertExists()
            assertTrue(onAllNodesWithText("Экран «Тренды»").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun aChipOnTheTrendsTabChangesWhatTheListReads() =
        runComposeUiTest {
            // An effect keyed on the reload counter alone would leave the chips changing
            // nothing — until this test nothing measured that.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Тренды").performClick()
            waitForIdle()

            // Default «3 месяца»: weight has eight readings and a delta; a week has one and none.
            val quarter = onAllNodesWithText("↓", substring = true).fetchSemanticsNodes().size
            assertTrue(quarter > 0, "the three-month list carries deltas")

            onNodeWithTag(cadenceTrendWindowTag(TrendWindow.WEEK)).performClick()
            waitForIdle()

            assertTrue(
                onAllNodesWithText("↓", substring = true).fetchSemanticsNodes().size < quarter,
                "a week holds fewer readings, so fewer metrics have moved",
            )
            onNodeWithTag(CADENCE_TRENDS_HERO_TAG, useUnmergedTree = true).assertExists()
        }

    @Test
    fun tappingAMetricOpensItWithTheDataTheAppRead() =
        runComposeUiTest {
            // The other half of the shipped wiring: `onLoadMetric` reaching the repository.
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = mocks()) }
            }

            onNodeWithText("Тренды").performClick()
            waitForIdle()
            onNodeWithTag(cadenceTrendCardTag(Metric.WEIGHT)).performClick()
            waitForIdle()

            assertEquals(
                listOf("Today", "Trends", "TrendDetail"),
                nav.currentBackStack.value
                    .filter { it.destination !is NavGraph }
                    .mapNotNull { it.destination.route }
                    .map { it.substringBefore('/').substringBefore('?').substringAfterLast('.') },
            )
            // Only exists once the read landed — a loader stuck on null leaves the placeholder here.
            onNodeWithTag(CADENCE_TREND_DETAIL_STATS_TAG, useUnmergedTree = true).assertExists()
        }

    /**
     * The write's own answer decides whether the wizard closes. Unreachable through the mock,
     * whose parse always names a meal — so the case is driven by handing the shell an action
     * that rejects, which is what M9's server will do when a draft doesn't survive the round
     * trip. Without this, «close on any answer» passes every other test in the suite.
     */
    @Test
    fun aRejectedMealLeavesTheWizardOpenWithWhatWasTyped() =
        runComposeUiTest {
            val mocks = mocks()
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                var summary by remember { mutableStateOf<TodaySummary?>(null) }
                LaunchedEffect(Unit) { summary = mocks.today.today() }
                CadenceTheme {
                    CadenceShell(
                        navController = nav,
                        data = CadenceShellData(summary = summary, now = mocks.nowLocal()),
                        actions =
                            CadenceShellActions(
                                parseMeal = { text -> MockMealParser().parse(text) },
                                onMealLogged = { MealLogResult.Rejected },
                            ),
                    )
                }
            }
            waitForIdle()

            runOnUiThread { nav.openRoute(CadenceRoute.LogMeal) }
            waitForIdle()
            onNodeWithTag(LOG_MEAL_CHAT_FIELD_TAG, useUnmergedTree = true).performTextReplacement("курица с рисом")
            waitForIdle()
            onNodeWithText("Разобрать →").performClick()
            waitForIdle()
            onNodeWithTag(LOG_MEAL_SAVE_TAG).performScrollTo().performClick()
            waitForIdle()

            assertTrue(
                nav.currentBackStack.value.any { it.destination.hasRoute<CadenceRoute.LogMeal>() },
                "a rejected draft dismissed the wizard as if it had been recorded",
            )
            onNodeWithText("Куриная грудка", substring = true).assertExists()
        }

    @Test
    fun theNutritionTabDrawsItsOwnScreenAndKeepsTheTabBar() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            nutritionTab().performClick()
            waitForIdle()

            onNodeWithTag(CADENCE_NUTRITION_TAG).assertExists()
            // The tab bar is the reason this is a tab and not a pushed route: the placeholder
            // drew one too, so the screen alone would not tell the two apart.
            onNodeWithContentDescription("Аптечка").assertExists()
            // From the repository, not a constant: the seeded day is 840 kcal against 1800.
            onNodeWithText("840", substring = true).assertExists()
        }

    @Test
    fun theRecipesRouteOpensFromNutritionAndListsTheLibrary() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = mocks()) }
            }

            nutritionTab().performClick()
            waitForIdle()
            onNodeWithTag(CADENCE_NUTRITION_RECIPES_LINK_TAG).performScrollTo().performClick()
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPES_TAG).assertExists()
            assertTrue(
                nav.currentBackStack.value.any { it.destination.hasRoute<CadenceRoute.Recipes>() },
                "the card did not push the recipe library",
            )
            // A seeded recipe by name, so «the screen rendered» cannot pass on an empty library.
            onNodeWithText("Лосось с киноа и брокколи").performScrollTo().assertExists()
        }

    @Test
    fun openingARecipeShowsThatRecipesOwnCard() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            nutritionTab().performClick()
            waitForIdle()
            onNodeWithTag(CADENCE_NUTRITION_RECIPES_LINK_TAG).performScrollTo().performClick()
            waitForIdle()
            // Not the first row and not the featured card: those two would also be reached by a
            // detail route that ignores the id it was given.
            onNodeWithText("Тёплая чечевица с индейкой").performScrollTo().performClick()
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPE_DETAIL_TAG).assertExists()
            onNodeWithText("Тёплая чечевица с индейкой").assertExists()
        }
}

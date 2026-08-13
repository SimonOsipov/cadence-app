package app.cadence.screens.nutrition

import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CADENCE_RINGS_TAG
import app.cadence.design.CadenceTheme
import app.cadence.shared.domain.Macros
import app.cadence.shared.domain.NutritionTargets
import app.cadence.shared.domain.UserId
import app.cadence.shared.repository.NutritionDay
import kotlinx.datetime.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/** `MockSeed.targets` — the same 1800/140/200/60 the spec puts in exactly one place. */
private val TARGETS =
    NutritionTargets(patientId = UserId("patient-1"), macros = Macros(1800, 140, 200, 60), waterMl = null)
private val TODAY = LocalDate(2026, 5, 31)

private fun dayOf(totals: Macros) = NutritionDay(date = TODAY, meals = emptyList(), totals = totals, targets = TARGETS)

@OptIn(ExperimentalTestApi::class)
class NutritionScreenTest {
    /**
     * "Пустой день рисует **оба** приглашения, и нажатие на ссылку в ленте
     * открывает запись приёма" (spec step-6). Two separate invitations —
     * the hero's italic line and the feed's empty card — so a mutation that
     * drops either one on its own still reddens this test; and the
     * emphasised link is its own pressable node, proven by actually clicking
     * it rather than only asserting it exists.
     */
    @Test
    fun anEmptyDayDrawsBothInvitationsAndTheLinkOpensMealLogging() =
        runComposeUiTest {
            var opened = false
            setContent {
                CadenceTheme {
                    NutritionScreen(day = dayOf(Macros(0, 0, 0, 0)), onLogMeal = { opened = true })
                }
            }

            // The hero's own invitation — `NutritionHero`'s empty branch.
            onNodeWithText("Пока ничего — начнём, когда будете готовы.").assertExists()
            // The feed's own invitation — `EmptyFeedCard`.
            onNodeWithText("Сегодня пока ничего.").assertExists()

            onNodeWithText("Запишите первый приём").performClick()
            waitForIdle()

            assertTrue(opened, "the emphasised link in the empty feed did not open meal logging")
        }

    /**
     * The wiring test for [NutritionRingsCard]'s call into `CadenceRings`:
     * against the same lopsided fixture `CadenceRingsTest.kt` uses at the
     * primitive level (900/1800 kcal = 50%, 35/140 g protein = 25%), this
     * proves the *screen* threads `day.totals`/`day.targets.macros` into the
     * right slots rather than, say, passing kcal into both. Swapping the
     * protein arguments for the kcal ones would read «Белок 900 из 1800 г ·
     * 50%» here — the mutation this test exists to fail against.
     */
    @Test
    fun theProteinRingDrawsADifferentArcThanTheCalorieRing() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    NutritionScreen(day = dayOf(Macros(kcal = 900, proteinG = 35, carbsG = 0, fatG = 0)))
                }
            }

            val described =
                onNodeWithTag(CADENCE_RINGS_TAG, useUnmergedTree = true)
                    .fetchSemanticsNode()
                    .config
                    .getOrNull(SemanticsProperties.ContentDescription)
                    ?.firstOrNull()

            assertEquals("Калории 900 из 1800 ккал · 50% · Белок 35 из 140 г · 25%", described)
        }
}

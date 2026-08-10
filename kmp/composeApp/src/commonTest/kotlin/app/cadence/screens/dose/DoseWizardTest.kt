package app.cadence.screens.dose

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CadenceTheme
import app.cadence.shared.domain.Dose
import app.cadence.shared.domain.DoseDraft
import app.cadence.shared.domain.DoseStep
import app.cadence.shared.domain.DoseUnit
import app.cadence.shared.domain.InjectionSite
import app.cadence.shared.domain.ProtocolItemId
import app.cadence.shared.domain.ProtocolItemKind
import app.cadence.shared.domain.SideEffect
import app.cadence.shared.domain.VialId
import app.cadence.shared.domain.inventorySummary
import app.cadence.shared.mock.MockSeed
import kotlinx.datetime.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

private val SEMA = ProtocolItemId("item-sema")
private val BPC = ProtocolItemId("item-bpc")
private val GLYCINE = ProtocolItemId("item-glycine")

/** The seed's two open BPC vials, fullest first, as the picker offers them. */
private fun openBpcVials() =
    inventorySummary(MockSeed.plan, MockSeed.vials, MockSeed.history, LocalDate(2026, 5, 31), MockSeed.compounds)
        .active
        .filter { it.compound?.id == MockSeed.bpc.id }
        .sortedByDescending { it.remaining }

private val OPTIONS =
    listOf(
        DoseOption(
            itemId = SEMA,
            nameRu = "Семаглутид",
            kind = ProtocolItemKind.INJECTION,
            dose = Dose(0.25, DoseUnit.MG),
            modeRu = "п/к · еженедельно",
            dueToday = true,
        ),
        DoseOption(
            itemId = BPC,
            nameRu = "BPC-157",
            kind = ProtocolItemKind.INJECTION,
            dose = Dose(250.0, DoseUnit.MCG),
            modeRu = "п/к · 2× в день",
            dueToday = false,
            // Two open vials of one compound — the case the picker is for.
            vials = openBpcVials(),
        ),
        DoseOption(
            itemId = GLYCINE,
            nameRu = "Глицин",
            kind = ProtocolItemKind.SUPPLEMENT,
            dose = Dose(1.0, DoseUnit.MG),
            modeRu = "внутрь · ежедневно",
            dueToday = false,
        ),
    )

/** A draft that has satisfied every step before the review. */
private fun complete() =
    DoseDraft(
        itemId = SEMA,
        kind = ProtocolItemKind.INJECTION,
        dose = Dose(0.25, DoseUnit.MG),
        site = InjectionSite.LEFT_THIGH,
    )

/** «Шаг 3 из 5» — the header's counter, which is not uppercased. */
private fun counterFor(step: DoseStep) = "Шаг ${step.ordinal + 1} из ${DoseStep.entries.size}"

@OptIn(ExperimentalTestApi::class)
class DoseWizardTest {
    /** The wizard driving its own state, the way the shell will hold it. */
    private fun ComposeUiTest.wizard(
        initial: DoseDraft = DoseDraft(),
        onSubmit: (DoseDraft) -> Unit = { },
        onCancel: () -> Unit = { },
    ) {
        setContent {
            CadenceTheme {
                var draft by remember { mutableStateOf(initial) }
                var step by remember { mutableStateOf(DoseStep.COMPOUND) }

                DoseWizard(
                    draft = draft,
                    step = step,
                    options = OPTIONS,
                    suggestedSite = InjectionSite.LEFT_ABDOMEN,
                    onDraft = { draft = it },
                    onStep = { step = it },
                    onSubmit = { onSubmit(draft) },
                    onCancel = onCancel,
                )
            }
        }
    }

    /**
     * «Дальше» until the wizard is on [target].
     *
     * Bounded, and by the number of steps rather than by hope: an unbounded
     * loop against a button that may be disabled does not fail, it hangs, and a
     * hung suite says nothing at all. The counter is what the header shows, not
     * the eyebrow — `CadenceEyebrow` renders its text uppercased.
     */
    private fun ComposeUiTest.walkTo(target: DoseStep) {
        repeat(DoseStep.entries.size) {
            if (onAllNodesWithText(counterFor(target)).fetchSemanticsNodes().isNotEmpty()) return
            onNodeWithText("Дальше").performClick()
            waitForIdle()
        }
        onNodeWithText(counterFor(target)).assertIsDisplayed()
    }

    @Test
    fun everyStepNamesItselfInThePrototypesWords() =
        runComposeUiTest {
            // «Шаг 1 · Препарат» and the rest, against `stepDefs` in
            // `LogDoseScreen.tsx`. A step whose eyebrow drifted would still
            // render, and only this says which one it is.
            assertEquals("Шаг 1 · Препарат", DoseStep.COMPOUND.eyebrowRu())
            assertEquals("Шаг 2 · Доза", DoseStep.DOSE.eyebrowRu())
            assertEquals("Шаг 3 · Место", DoseStep.SITE.eyebrowRu())
            assertEquals("Шаг 4 · Контекст", DoseStep.CONTEXT.eyebrowRu())
            assertEquals("Шаг 5 · Проверка", DoseStep.REVIEW.eyebrowRu())
        }

    @Test
    fun theFirstStepShowsItsEyebrowItsTitleAndTheCounter() =
        runComposeUiTest {
            wizard()

            // Uppercased on screen: `CadenceEyebrow` is a small-caps run.
            onNodeWithText(DoseStep.COMPOUND.eyebrowRu(), ignoreCase = true).assertIsDisplayed()
            onNodeWithText("Что вы приняли?").assertIsDisplayed()
            onNodeWithText("Шаг 1 из 5").assertIsDisplayed()
        }

    @Test
    fun nextIsDeadUntilTheStepsRuleIsMetAndLiveOnce() =
        runComposeUiTest {
            wizard()

            onNodeWithText("Дальше").assertIsNotEnabled()

            onNodeWithText("Семаглутид").performClick()
            waitForIdle()

            onNodeWithText("Дальше").assertIsEnabled()
        }

    @Test
    fun choosingACompoundPutsThatCompoundAndItsDoseInTheDraft() =
        runComposeUiTest {
            // The second option, deliberately: tapping the first and checking
            // «Дальше» went live proves only that *something* was chosen. The
            // review is where the draft becomes visible, and BPC-157 differs
            // from the default in both the name and the unit.
            var submitted: DoseDraft? = null
            wizard(onSubmit = { submitted = it })

            onNodeWithText("BPC-157").performClick()
            waitForIdle()
            onNodeWithText("Дальше").performClick()
            waitForIdle()
            onNodeWithText("Дальше").performClick()
            waitForIdle()
            onNodeWithContentDescription("Правое плечо").performClick()
            waitForIdle()

            walkTo(DoseStep.REVIEW)

            onNodeWithText("BPC-157").assertIsDisplayed()
            onNodeWithText("250").assertIsDisplayed()
            onNodeWithText("мкг").assertIsDisplayed()

            onNodeWithText("Сохранить дозу").performClick()
            waitForIdle()

            assertEquals(BPC, submitted?.itemId)
            assertEquals(Dose(250.0, DoseUnit.MCG), submitted?.dose)
        }

    @Test
    fun theLastStepSavesRatherThanAdvancing() =
        runComposeUiTest {
            var submitted: DoseDraft? = null
            wizard(initial = complete(), onSubmit = { submitted = it })

            walkTo(DoseStep.REVIEW)

            onNodeWithText("Сохранить дозу").assertIsEnabled()
            assertEquals(0, onAllNodesWithText("Дальше").fetchSemanticsNodes().size, "the review still advances")

            onNodeWithText("Сохранить дозу").performClick()
            waitForIdle()

            assertEquals(complete(), submitted)
        }

    @Test
    fun backReturnsToThePreviousStepAndKeepsWhatWasEntered() =
        runComposeUiTest {
            wizard(initial = complete())

            walkTo(DoseStep.DOSE)
            onNodeWithContentDescription("Увеличить дозу").performClick()
            waitForIdle()
            onNodeWithText("0,3").assertIsDisplayed()

            onNodeWithText("Дальше").performClick()
            waitForIdle()
            onNodeWithText("Шаг 3 из 5").assertIsDisplayed()

            onNodeWithText("Назад").performClick()
            waitForIdle()

            onNodeWithText("Шаг 2 из 5").assertIsDisplayed()
            onNodeWithText("0,3").assertIsDisplayed()
        }

    @Test
    fun cancelInTheHeaderLeavesTheWizard() =
        runComposeUiTest {
            var cancelled = false
            wizard(onCancel = { cancelled = true })

            onNodeWithText("Отмена").performClick()
            waitForIdle()

            assertTrue(cancelled)
        }

    @Test
    fun aZoneOnTheBackOfTheBodyCanBeChosen() =
        runComposeUiTest {
            // Four of the ten zones live behind the toggle, and the rotation
            // suggests all ten. A toggle that draws and does nothing means the
            // wizard proposes a glute and then refuses to take it — and the
            // patient records an injection somewhere they did not make one.
            var submitted: DoseDraft? = null
            wizard(initial = complete().copy(site = null), onSubmit = { submitted = it })

            walkTo(DoseStep.SITE)

            onNodeWithText("Сзади").performClick()
            waitForIdle()
            // Scrolled to first: the diagram is taller than the window, and a
            // click on a target outside the viewport lands nowhere.
            onNodeWithContentDescription("Левая поясница").performScrollTo().performClick()
            waitForIdle()

            walkTo(DoseStep.REVIEW)
            onNodeWithText("Сохранить дозу").performClick()
            waitForIdle()

            assertEquals(InjectionSite.LEFT_LOWER_BACK, submitted?.site)
        }

    @Test
    fun anItemThatIsNotAnInjectionNeedsNoZoneToGetPast() =
        runComposeUiTest {
            // `DoseEvent.site` is nullable because a supplement has no zone,
            // and `DoseDraft` carries the kind for exactly this. A compound
            // step that stamped INJECTION on everything would demand a body
            // zone for a capsule taken by mouth, and nothing could be logged.
            var submitted: DoseDraft? = null
            wizard(onSubmit = { submitted = it })

            onNodeWithText("Глицин").performClick()
            waitForIdle()

            walkTo(DoseStep.SITE)
            onNodeWithText("Дальше").assertIsEnabled()

            walkTo(DoseStep.REVIEW)
            onNodeWithText("Сохранить дозу").performClick()
            waitForIdle()

            assertEquals(GLYCINE, submitted?.itemId)
            assertEquals(ProtocolItemKind.SUPPLEMENT, submitted?.kind)
            assertEquals(null, submitted?.site)
        }

    @Test
    fun theDoseStepOffersTheOpenVialsAndDefaultsToTheFullest() =
        runComposeUiTest {
            // The debt the wizard was left owing. Two open vials of one
            // compound is a state §03 allows and the seed now contains, and
            // until this the write picked one for the patient without asking.
            var submitted: DoseDraft? = null
            wizard(onSubmit = { submitted = it })

            // BPC-157, because it is the compound with two open vials — the
            // semaglutide draft `complete()` builds has one, and a picker with
            // one option is not a picker.
            onNodeWithText("BPC-157").performClick()
            waitForIdle()
            walkTo(DoseStep.DOSE)

            onNodeWithText("B-2601").assertExists()
            onNodeWithText("B-2510").assertExists()
            // Sealed stock is not offered: a vial nobody has opened is not one
            // this dose came out of.
            assertEquals(0, onAllNodesWithText("B-2610").fetchSemanticsNodes().size)

            onNodeWithText("Дальше").performClick()
            waitForIdle()
            onNodeWithContentDescription("Правое плечо").performScrollTo().performClick()
            waitForIdle()
            walkTo(DoseStep.REVIEW)
            onNodeWithText("Сохранить дозу").performClick()
            waitForIdle()

            // The fullest open one, chosen for them until they say otherwise.
            assertEquals(VialId("vial-bpc-3"), submitted?.vialId)
        }

    @Test
    fun theVialThePatientPicksIsTheOneRecorded() =
        runComposeUiTest {
            var submitted: DoseDraft? = null
            wizard(onSubmit = { submitted = it })

            onNodeWithText("BPC-157").performClick()
            waitForIdle()
            walkTo(DoseStep.DOSE)
            onNodeWithText("B-2510").performScrollTo().performClick()
            waitForIdle()

            onNodeWithText("Дальше").performClick()
            waitForIdle()
            onNodeWithContentDescription("Правое плечо").performScrollTo().performClick()
            waitForIdle()
            walkTo(DoseStep.REVIEW)
            onNodeWithText("Сохранить дозу").performClick()
            waitForIdle()

            assertEquals(VialId("vial-bpc-2"), submitted?.vialId)
        }

    @Test
    fun theCheckInStepPutsWhatWasTappedIntoTheDraft() =
        runComposeUiTest {
            // The input half of «one action, two facts». Every earlier test
            // seeded mood, chips and note into the initial draft and read them
            // back at the review, so a slider wired to nothing was invisible —
            // and `JournalEntry.tags` would have been silently empty forever.
            var submitted: DoseDraft? = null
            wizard(initial = complete(), onSubmit = { submitted = it })

            walkTo(DoseStep.CONTEXT)

            onNodeWithContentDescription("Самочувствие 4 из 5").performClick()
            waitForIdle()
            onNodeWithText("Тошнота").performClick()
            waitForIdle()
            onNodeWithText("Усталость").performClick()
            waitForIdle()
            // On and off again: a chip row that only ever added would pass a
            // test that only ever tapped once.
            onNodeWithText("Усталость").performClick()
            waitForIdle()
            onNodeWithText("Добавить фото").performClick()
            waitForIdle()

            walkTo(DoseStep.REVIEW)
            onNodeWithText("Сохранить дозу").performClick()
            waitForIdle()

            assertEquals(4, submitted?.mood)
            assertEquals(listOf(SideEffect.NAUSEA), submitted?.sideEffects)
            assertEquals(true, submitted?.photoAttached)
        }

    @Test
    fun theReviewNamesTheCompoundTheDoseAsTwoRunsAndTheZoneInRussian() =
        runComposeUiTest {
            wizard(
                initial =
                    complete().copy(
                        dose = Dose(0.5, DoseUnit.MG),
                        site = InjectionSite.RIGHT_ABDOMEN,
                        mood = 4,
                        sideEffects = listOf(SideEffect.NAUSEA),
                        note = "чуть тянет",
                    ),
            )

            walkTo(DoseStep.REVIEW)

            // The eyebrow on a step that is not the first: walking by the
            // header's counter leaves «every step shows step 1's eyebrow»
            // invisible, and that is the whole heading being wrong.
            onNodeWithText(DoseStep.REVIEW.eyebrowRu(), ignoreCase = true).assertIsDisplayed()
            onNodeWithText("Семаглутид").assertIsDisplayed()
            // Two runs: the number and the unit are separate, because the
            // prototype sets them in different faces and because a dose is
            // never one string.
            onNodeWithText("0,5").assertIsDisplayed()
            onNodeWithText("мг").assertIsDisplayed()
            onNodeWithText("Правый живот").assertIsDisplayed()
            onNodeWithText("Хорошо").assertIsDisplayed()
            onNodeWithText("Тошнота").assertIsDisplayed()
            onNodeWithText("чуть тянет").assertIsDisplayed()
        }

    @Test
    fun theReviewReadsTheDraftAndNotTheCompoundsOwnDefault() =
        runComposeUiTest {
            // The defect this guards: the review renders `c.default` — the
            // prescription — rather than the number the patient stepped to.
            wizard(initial = complete().copy(dose = Dose(0.2, DoseUnit.MG)))

            walkTo(DoseStep.REVIEW)

            onNodeWithText("0,2").assertIsDisplayed()
            assertEquals(0, onAllNodesWithText("0,25").fetchSemanticsNodes().size, "the review shows the plan's dose")
        }

    @Test
    fun anEmptyCheckInReadsAsEmptyRatherThanAsSomething() =
        runComposeUiTest {
            // Step 4 is optional, so the review has to be able to say «—»
            // rather than invent a mood or a side effect.
            wizard(initial = complete())

            walkTo(DoseStep.REVIEW)

            assertEquals(0, onAllNodesWithText("Ровно").fetchSemanticsNodes().size, "a mood nobody chose")
            // Two rows say «—»: the energy nobody rated and the complaints
            // nobody made. Both are «пропущено», and both have to read as it.
            assertEquals(2, onAllNodesWithText("—").fetchSemanticsNodes().size)
        }

    @Test
    fun theSiteStepOpensOnTheSuggestionAndTheContextStepAlwaysAdvances() =
        runComposeUiTest {
            wizard(initial = complete().copy(site = null))

            walkTo(DoseStep.SITE)

            // `assertExists`: the diagram is taller than the test window, and
            // the caption sits under it.
            onNodeWithText("Предложено: Левый живот — следующее в ротации", substring = true).assertExists()
            onNodeWithText("Дальше").assertIsNotEnabled()

            // A shoulder rather than a thigh: the diagram sits in a scroll,
            // and a target near the bottom of it is outside the viewport in
            // a test window, where a click lands nowhere.
            onNodeWithContentDescription("Правое плечо").performClick()
            waitForIdle()

            onNodeWithText("Дальше").assertIsEnabled()
            onNodeWithText("Дальше").performClick()
            waitForIdle()

            onNodeWithText("Шаг 4 из 5").assertIsDisplayed()
            onNodeWithText("Дальше").assertIsEnabled()
        }
}

package app.cadence.screens.inventory

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CadenceTheme
import app.cadence.format.relativeDay
import app.cadence.shared.domain.VialId
import app.cadence.shared.domain.vialDetail
import app.cadence.shared.mock.MockSeed
import kotlinx.datetime.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals

private val TODAY = LocalDate(2026, 5, 31)

private fun detail(id: String) =
    vialDetail(
        plan = MockSeed.plan,
        vial = MockSeed.vials.first { it.id == VialId(id) },
        events = MockSeed.history,
        today = TODAY,
        compounds = MockSeed.compounds,
    )

@OptIn(ExperimentalTestApi::class)
class VialDetailSheetTest {
    @Test
    fun aRelativeDayReadsTheWayAPatientWouldSayIt() {
        // «3 дн назад», not «3 дня назад»: the prototype's own abbreviation,
        // and the boundaries are where an off-by-one lives.
        assertEquals("сегодня", relativeDay(TODAY, TODAY))
        assertEquals("вчера", relativeDay(LocalDate(2026, 5, 30), TODAY))
        assertEquals("3 дн назад", relativeDay(LocalDate(2026, 5, 28), TODAY))
        assertEquals("6 дн назад", relativeDay(LocalDate(2026, 5, 25), TODAY))
        assertEquals("1 нед назад", relativeDay(LocalDate(2026, 5, 24), TODAY))
        assertEquals("2 нед назад", relativeDay(LocalDate(2026, 5, 17), TODAY))
        // Eleven days is where rounding and truncation part company: a patient
        // placing a dose reads «2 нед назад», and «1 нед назад» is a week out.
        assertEquals("2 нед назад", relativeDay(LocalDate(2026, 5, 20), TODAY))
    }

    @Test
    fun theSheetNamesTheCompoundAndTheDoseAsTwoRuns() =
        runComposeUiTest {
            setContent { CadenceTheme { VialDetailSheet(detail = detail("vial-sema-1"), today = TODAY) } }

            onNodeWithText("Семаглутид").assertExists()
            // Two runs, because a dose is never one rendered string.
            onNodeWithText("0,25").assertExists()
            onNodeWithText("мг").assertExists()
        }

    @Test
    fun theFourFactsAreOpenedExpiresLotAndWhereItIsKept() =
        runComposeUiTest {
            setContent { CadenceTheme { VialDetailSheet(detail = detail("vial-sema-1"), today = TODAY) } }

            onNodeWithText("Открыт", ignoreCase = true).assertExists()
            // Substring: the opened fact reads «10 мая · 3 нед назад».
            onNodeWithText("10 мая", substring = true).assertExists()
            onNodeWithText("Истекает", ignoreCase = true).assertExists()
            onNodeWithText("1 сентября", substring = true).assertExists()
            onNodeWithText("A-2261").assertExists()
            onNodeWithText("Холодильник, полка 2").assertExists()
        }

    @Test
    fun aSealedVialSaysItIsUnopenedRatherThanShowingABlankDate() =
        runComposeUiTest {
            setContent { CadenceTheme { VialDetailSheet(detail = detail("vial-bpc-4"), today = TODAY) } }

            onNodeWithText("Не вскрыт").assertExists()
        }

    @Test
    fun theRecordsAreThisVialsWithTheirZoneAndTheirDay() =
        runComposeUiTest {
            setContent { CadenceTheme { VialDetailSheet(detail = detail("vial-sema-1"), today = TODAY) } }

            // Three Sundays, each line naming the dose and the zone. Counted
            // rather than named one by one: the zones come from the rotation,
            // so which three they are is the rule's answer and not the test's.
            onNodeWithText("Последние записи · 3", ignoreCase = true).assertExists()
            assertEquals(
                3,
                onAllNodesWithText("0,25 мг · ", substring = true).fetchSemanticsNodes().size,
                "a record is missing its dose or its zone",
            )
            onNodeWithText("1 нед назад").assertExists()
        }

    @Test
    fun aVialWithNoRecordsSaysSoRatherThanDrawingAnEmptyList() =
        runComposeUiTest {
            setContent { CadenceTheme { VialDetailSheet(detail = detail("vial-bpc-4"), today = TODAY) } }

            onNodeWithText("Из этого флакона ещё не кололи").assertExists()
            assertEquals(0, onAllNodesWithText("нед назад", substring = true).fetchSemanticsNodes().size)
        }

    @Test
    fun loggingADoseFromTheSheetCarriesThisVial() =
        runComposeUiTest {
            var logged: VialId? = null
            setContent {
                CadenceTheme {
                    VialDetailSheet(detail = detail("vial-bpc-2"), today = TODAY, onLogDose = { logged = it })
                }
            }

            onNodeWithText("Записать дозу").performScrollTo().performClick()

            assertEquals(VialId("vial-bpc-2"), logged)
        }

    @Test
    fun theActionsThatAreNotBuiltSaySoInsteadOfDoingNothing() =
        runComposeUiTest {
            // A row that looks live and does nothing is worse than one that
            // says it is not ready. Both are recorded as owed.
            setContent { CadenceTheme { VialDetailSheet(detail = detail("vial-sema-1"), today = TODAY) } }

            onNodeWithText("Прикрепить фото").performScrollTo().assertIsNotEnabled()
            onNodeWithText("Перенести в запас").performScrollTo().assertIsNotEnabled()
        }
}

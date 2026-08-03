package app.cadence.screens.today

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CadenceTheme
import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.domain.Metric
import app.cadence.shared.mock.CadenceMocks
import app.cadence.shared.repository.MetricSeries
import kotlinx.coroutines.runBlocking
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

private fun series(metric: Metric): MetricSeries =
    runBlocking {
        CadenceMocks(FixedCadenceClock.at("2026-05-31T04:00:00Z"), TimeZone.of("Europe/Moscow"))
            .measurements
            .series(metric)
    }

@OptIn(ExperimentalTestApi::class)
class BiomarkerSheetTest {
    @Test
    fun theSheetNamesTheMetricAndItsLatestReading() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    BiomarkerSheet(open = true, title = "Вес", series = series(Metric.WEIGHT), onDismiss = {})
                }
            }

            onNodeWithText("Вес").assertIsDisplayed()
            // Value and unit as separate runs — a measurement is {value, unit}
            // for the same reason a dose is.
            onNodeWithText("98,4").assertIsDisplayed()
            onNodeWithText("кг").assertIsDisplayed()
        }

    @Test
    fun theSheetSaysHowFarTheReadingMoved() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    BiomarkerSheet(open = true, title = "Вес", series = series(Metric.WEIGHT), onDismiss = {})
                }
            }

            onNodeWithText("↓ 0,4 кг").assertIsDisplayed()
            onNodeWithText("к прошлой неделе").assertIsDisplayed()
        }

    @Test
    fun aMetricWithNoReadingsSaysSoRatherThanDrawingAnEmptyChart() =
        runComposeUiTest {
            // A blank chart beside a caption reads as «failed to load», and a
            // patient waits instead of measuring.
            setContent {
                CadenceTheme {
                    BiomarkerSheet(open = true, title = "Грудь", series = series(Metric.CHEST), onDismiss = {})
                }
            }

            onNodeWithText("Пока нет измерений").assertIsDisplayed()
            assertTrue(onAllNodesWithText("к прошлой неделе").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun aClosedSheetComposesNothing() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    BiomarkerSheet(open = false, title = "Вес", series = series(Metric.WEIGHT), onDismiss = {})
                }
            }

            assertTrue(onAllNodesWithText("Вес").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun theSheetLeadsOnIntoTheTrend() =
        runComposeUiTest {
            var opened = 0
            setContent {
                CadenceTheme {
                    BiomarkerSheet(
                        open = true,
                        title = "Вес",
                        series = series(Metric.WEIGHT),
                        onDismiss = {},
                        onOpenTrend = { opened++ },
                    )
                }
            }

            onNodeWithText("Открыть детали тренда").performClick()

            assertEquals(1, opened)
        }
}

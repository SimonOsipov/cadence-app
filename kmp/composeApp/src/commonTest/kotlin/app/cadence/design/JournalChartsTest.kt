package app.cadence.design

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import app.cadence.shared.domain.MoodLevel
import app.cadence.shared.domain.TrendRange
import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.plus
import kotlin.math.abs
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

internal const val COURSE_LAST_DAY = 83

private const val TOLERANCE = 1e-3f

private val INSET = ChartInset(left = 10f, right = 10f, top = 20f, bottom = 20f)
private const val CANVAS_WIDTH = 350f
private const val CANVAS_HEIGHT = 200f

/** A Sunday: the start weekday that costs the prototype's grid its last six days. */
private val SUNDAY_START = LocalDate(2026, 5, 24)

private val COURSE = TrendRange(SUNDAY_START, SUNDAY_START.plus(DatePeriod(days = COURSE_LAST_DAY)))

private fun day(index: Int): LocalDate = SUNDAY_START.plus(DatePeriod(days = index))

class JournalChartsTest {
    @Test
    fun aReadingIsPlacedByItsDateRatherThanItsPositionInTheList() {
        // Days 0, 1 and 4: adjacent, then a three-day gap. Laid out by index the
        // three would be evenly spaced and both distances would be equal.
        val points =
            moodPoints(
                readings =
                    listOf(0, 1, 4).map {
                        MoodReading(date = day(it), level = MoodLevel.EVEN, dosed = false)
                    },
                course = COURSE,
                canvasWidth = CANVAS_WIDTH,
                canvasHeight = CANVAS_HEIGHT,
                inset = INSET,
            )

        val adjacent = points[1].x - points[0].x
        val acrossTheGap = points[2].x - points[1].x

        assertTrue(adjacent > 0f, "the axis runs left to right, got $adjacent")
        assertEquals(3f, acrossTheGap / adjacent, TOLERANCE)
    }

    @Test
    fun theFirstColumnIsMondayWhenTheCourseOpensOnSunday() {
        val weeks = heatmapWeeks(COURSE, today = day(0), moodByDate = emptyMap(), titrations = emptySet())

        val firstRow = weeks.first()

        // Six blanks, then the Sunday the course opens on, in the seventh column.
        assertTrue(firstRow.take(6).all { it == null }, "the week before the start is not blank")
        assertEquals(SUNDAY_START, assertNotNull(firstRow[6]).date)
    }

    @Test
    fun theGridReachesTheLastDayOfTheCourse() {
        val weeks = heatmapWeeks(COURSE, today = day(0), moodByDate = emptyMap(), titrations = emptySet())

        val drawn = weeks.flatten().filterNotNull().map { it.date }

        assertEquals(COURSE.days, drawn.size, "the grid draws a cell per course day and no more")
        assertTrue(day(COURSE_LAST_DAY) in drawn, "the last day of the course is missing from the grid")
    }

    @Test
    fun aDayStillToComeIsNotTheSameAsADayThatPassedUnwritten() {
        val weeks = heatmapWeeks(COURSE, today = day(10), moodByDate = emptyMap(), titrations = emptySet())
        val cells = weeks.flatten().filterNotNull().associateBy { it.date }

        val past = assertNotNull(cells[day(9)])
        val current = assertNotNull(cells[day(10)])
        val ahead = assertNotNull(cells[day(11)])

        // All three are unwritten; only the standing tells them apart.
        assertNull(past.mood)
        assertNull(ahead.mood)
        assertEquals(DayStanding.PAST, past.standing)
        assertEquals(DayStanding.TODAY, current.standing)
        assertEquals(DayStanding.FUTURE, ahead.standing)
    }

    @Test
    fun bothTitrationDaysAreMarkedOnTheGrid() {
        val titrations = setOf(day(28), day(56))

        val weeks = heatmapWeeks(COURSE, today = day(0), moodByDate = emptyMap(), titrations = titrations)

        val marked =
            weeks
                .flatten()
                .filterNotNull()
                .filter { it.titration }
                .map { it.date }
                .toSet()

        assertEquals(titrations, marked)
    }

    @Test
    fun theWeekNumberFollowsTheCalendarRowRatherThanDaysSinceTheStart() {
        val weeks = heatmapWeeks(COURSE, today = day(0), moodByDate = emptyMap(), titrations = emptySet())
        val cells = weeks.flatten().filterNotNull().associateBy { it.date }

        // The course opens on a Sunday, so the Monday after it is already the
        // second row. Divided by seven, day 1 would still answer week 1.
        assertEquals(1, assertNotNull(cells[day(0)]).week)
        assertEquals(2, assertNotNull(cells[day(1)]).week)
        assertEquals(13, assertNotNull(cells[day(COURSE_LAST_DAY)]).week)
    }

    @Test
    fun aMoodIsCarriedOntoTheDayItWasWrittenFor() {
        val weeks =
            heatmapWeeks(
                COURSE,
                today = day(10),
                moodByDate = mapOf(day(3) to MoodLevel.BRIGHT),
                titrations = emptySet(),
            )
        val cells = weeks.flatten().filterNotNull().associateBy { it.date }

        assertEquals(MoodLevel.BRIGHT, assertNotNull(cells[day(3)]).mood)
        assertNull(assertNotNull(cells[day(4)]).mood)
    }

    @Test
    fun theFiveGridLinesStandOneMoodLevelApart() {
        val lines = moodGridLines(CANVAS_HEIGHT, INSET)

        assertEquals(MoodLevel.entries.size, lines.size)

        val gaps = lines.zipWithNext { above, below -> below - above }
        assertTrue(gaps.all { abs(it - gaps.first()) < TOLERANCE }, "the levels are not evenly spaced: $gaps")
    }

    @Test
    fun theBrightestMoodSitsAboveTheHardest() {
        val hard =
            moodPoints(
                listOf(MoodReading(day(0), MoodLevel.HARD, dosed = false)),
                COURSE,
                CANVAS_WIDTH,
                CANVAS_HEIGHT,
                INSET,
            )
        val bright =
            moodPoints(
                listOf(MoodReading(day(0), MoodLevel.BRIGHT, dosed = false)),
                COURSE,
                CANVAS_WIDTH,
                CANVAS_HEIGHT,
                INSET,
            )

        // Smaller y is higher on a canvas.
        assertTrue(
            bright.single().y < hard.single().y,
            "«Светло» drew at ${bright.single().y}, «Тяжело» at ${hard.single().y}",
        )
    }

    @Test
    fun aGridRowIsAddedForTheLeadInRatherThanFixedAtThirteen() {
        // A Sunday start spends six cells before the course opens, so 84 days need
        // thirteen rows; a Monday start fits the same course in twelve, and drawing
        // thirteen would leave an empty row under it.
        assertEquals(13, heatmapRows(lead = 6, days = 84))
        assertEquals(12, heatmapRows(lead = 0, days = 84))
        // A course is not always twelve weeks: sixteen must not be truncated.
        assertEquals(17, heatmapRows(lead = 6, days = 112))
    }

    @Test
    fun theHeatmapTellsTodayFromWhatIsAheadAndWhatIsPast() {
        val palette = CadenceLightPalette

        assertEquals(CadenceColors.forest700, assertNotNull(heatmapBorder(DayStanding.TODAY, palette)).color)
        assertEquals(palette.border, assertNotNull(heatmapBorder(DayStanding.FUTURE, palette)).color)
        assertNull(heatmapBorder(DayStanding.PAST, palette), "a past day carries no ring")

        // Today's ring has to win against a filled neighbour, so it is the heavier —
        // pinned by width and not only by direction, or 1.1dp would pass for 1.5dp.
        assertEquals(1.5.dp, assertNotNull(heatmapBorder(DayStanding.TODAY, palette)).width)
        assertEquals(1.dp, assertNotNull(heatmapBorder(DayStanding.FUTURE, palette)).width)
    }

    @Test
    fun anUnwrittenDayAheadIsEmptyAndOneBehindIsNot() {
        val palette = CadenceLightPalette
        val cells =
            heatmapWeeks(COURSE, today = day(10), moodByDate = emptyMap(), titrations = emptySet())
                .flatten()
                .filterNotNull()
                .associateBy { it.date }

        assertEquals(Color.Transparent, heatmapFill(assertNotNull(cells[day(11)]), palette))
        assertNotEquals(Color.Transparent, heatmapFill(assertNotNull(cells[day(9)]), palette))
    }

    @Test
    fun aMoodCellCarriesItsOwnToneRatherThanTheGroundBehindIt() {
        val palette = CadenceLightPalette
        val cells =
            heatmapWeeks(COURSE, day(10), mapOf(day(3) to MoodLevel.HARD), emptySet())
                .flatten()
                .filterNotNull()
                .associateBy { it.date }

        assertEquals(
            assertNotNull(CadenceMoodTone.of(MoodLevel.HARD)).color,
            heatmapFill(assertNotNull(cells[day(3)]), palette),
        )
    }

    @Test
    fun aDosedReadingIsSolidAndAHandWrittenOneIsHollow() {
        val palette = CadenceLightPalette

        val dosed = moodDot(MoodReading(day(0), MoodLevel.GOOD, dosed = true), palette)
        val byHand = moodDot(MoodReading(day(0), MoodLevel.GOOD, dosed = false), palette)

        assertEquals(CadenceColors.sand500, dosed.fill, "a dose's dot is filled sand")
        assertEquals(palette.paper, byHand.fill, "a hand-written dot is hollow on the paper")

        // The same outline on both: the difference a reader catches is fill and size.
        assertEquals(CadenceColors.forest700, dosed.stroke)
        assertEquals(CadenceColors.forest700, byHand.stroke)

        assertEquals(4.5.dp, dosed.radius)
        assertEquals(3.2.dp, byHand.radius)
    }

    @Test
    fun aTitrationHairlineIsNotDrawnLikeTheDayTheUserStandsOn() {
        val titration = moodMarkStyle(MoodMarkKind.TITRATION)
        val today = moodMarkStyle(MoodMarkKind.TODAY)

        assertEquals(CadenceColors.sand500, titration.color)
        assertEquals(3.dp, titration.dashOn)
        assertEquals(3.dp, assertNotNull(titration.cap), "a titration is capped")

        assertEquals(CadenceColors.forest700.copy(alpha = 0.6f), today.color)
        assertEquals(2.dp, today.dashOn)
        assertNull(today.cap, "today carries no cap — the cap is what tells the two apart")
    }

    @Test
    fun theFutureIsShadedFromTodayToTheEndOfTheCourse() {
        val quarterIn = day(COURSE.days / 4)

        val shaded = futureFraction(COURSE, today = quarterIn)

        // A quarter of the course has passed, so three quarters lie ahead.
        assertEquals(0.75f, shaded, 0.02f)
    }

    @Test
    fun aCourseWithNothingLeftShadesNothing() {
        assertEquals(0f, futureFraction(COURSE, today = day(COURSE_LAST_DAY)), TOLERANCE)
    }
}

package app.cadence.shared.domain

import app.cadence.shared.mock.MockSeed
import kotlinx.datetime.DatePeriod
import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime
import kotlinx.datetime.daysUntil
import kotlinx.datetime.plus
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

private val START = LocalDate(2026, 5, 10)
private val PID = ProtocolId("pr")
private val SEMA = ProtocolItemId("sema")
private val WEIGH_IN = ProtocolItemId("weigh")

private val QUARTER = Dose(0.25, DoseUnit.MG)
private val HALF = Dose(0.5, DoseUnit.MG)
private val WHOLE = Dose(1.0, DoseUnit.MG)

/**
 * Out of order on purpose: §03 doesn't promise the server sorts them, and a sorted fixture
 * lets a missing `sortedBy` through.
 */
private val SHUFFLED_PHASES =
    listOf(
        ProtocolPhase(5, 8, HALF),
        ProtocolPhase(9, 12, WHOLE),
        ProtocolPhase(1, 4, QUARTER),
    )

/** Four weeks on, four weeks off, four weeks on — a deliberate wash-out. */
private val BROKEN_PHASES = listOf(ProtocolPhase(1, 4, QUARTER), ProtocolPhase(9, 12, WHOLE))

private fun plan(
    status: ProtocolStatus = ProtocolStatus.ACTIVE,
    phases: List<ProtocolPhase> = SHUFFLED_PHASES,
): ProtocolPlan =
    ProtocolPlan(
        protocol = Protocol(PID, UserId("p"), START, 12, status, null, null),
        items =
            listOf(
                ProtocolItem(
                    id = SEMA,
                    protocolId = PID,
                    kind = ProtocolItemKind.INJECTION,
                    compoundId = CompoundId("sema"),
                    cadence = ProtocolCadence.WEEKLY,
                    daysOfWeek = listOf(DayOfWeek.SUNDAY),
                    times = listOf(LocalTime(7, 0)),
                    loggable = true,
                ),
            ),
        phases = mapOf(SEMA to phases),
    )

private val PLAN = plan()

/** The whole course and a little either side, so nothing is clipped by accident. */
private val WHOLE_COURSE = TrendRange(LocalDate(2026, 5, 1), LocalDate(2026, 8, 31))

private fun bandsOf(
    range: TrendRange,
    p: ProtocolPlan = PLAN,
) = doseBands(p, SEMA, range).map { Triple(it.dose, it.range.from, it.range.through) }

class DoseBandsTest {
    @Test
    fun eachPhaseIsOneBandRunningFromItsFirstDayToItsLast() {
        // Week N begins (N-1)×7 days after start, matching schedule/data.ts. The trends
        // module's DOSE_SPANS disagrees — see theSeedTitratesOnTheDaysThePrototypesScheduleDoesAndNotItsChart.
        assertEquals(
            listOf(
                Triple(QUARTER, LocalDate(2026, 5, 10), LocalDate(2026, 6, 6)),
                Triple(HALF, LocalDate(2026, 6, 7), LocalDate(2026, 7, 4)),
                Triple(WHOLE, LocalDate(2026, 7, 5), LocalDate(2026, 8, 1)),
            ),
            bandsOf(WHOLE_COURSE),
        )
    }

    @Test
    fun theBandsMeetWithoutOverlappingAndCoverTheCourseExactly() {
        val bands = doseBands(PLAN, SEMA, WHOLE_COURSE)

        assertEquals(START, bands.first().range.from)
        assertEquals(LocalDate(2026, 8, 1), bands.last().range.through, "day 83, the last prescribed day")
        bands.zipWithNext().forEach { (earlier, later) ->
            assertEquals(
                1,
                earlier.range.through.daysUntil(later.range.from),
                "one band ends the day before the next begins",
            )
        }
        assertEquals(84, bands.sumOf { it.range.days }, "twelve weeks, no day counted twice")
    }

    @Test
    fun theBandCoveringADayCarriesTheDoseThatDayWasPrescribed() {
        // Asserted day by day, not at four dates a literal test happens to name. Run over
        // the gapped course too: on a contiguous course `phaseDose` is never null, so the
        // "no band over an unprescribed day" half isn't exercised by PLAN alone.
        listOf(PLAN, plan(phases = BROKEN_PHASES)).forEach { p ->
            val start = p.protocol.startDate
            val bands = doseBands(p, SEMA, WHOLE_COURSE)
            (0..83).forEach { offset ->
                val day = start.plus(DatePeriod(days = offset))
                assertEquals(
                    listOfNotNull(phaseDose(p, SEMA, day)),
                    bands.filter { day in it.range }.map { it.dose },
                    "one band, carrying the prescribed dose, on $day",
                )
            }
        }
    }

    @Test
    fun aGapBetweenTwoPhasesIsAGapBetweenTwoBands() {
        // Makes `phase.toWeek` observable: a band closing where the *next* phase opens,
        // instead of where its own ends, draws the same spans for a contiguous course — only
        // a gap separates the two implementations. Do not let it become contiguous.
        val broken = plan(phases = BROKEN_PHASES)

        assertEquals(
            listOf(
                Triple(QUARTER, LocalDate(2026, 5, 10), LocalDate(2026, 6, 6)),
                Triple(WHOLE, LocalDate(2026, 7, 5), LocalDate(2026, 8, 1)),
            ),
            bandsOf(WHOLE_COURSE, broken),
        )
        // Weeks 5–8 are prescribed nothing, and nothing is drawn over them.
        assertEquals(emptyList(), bandsOf(TrendRange(LocalDate(2026, 6, 7), LocalDate(2026, 7, 4)), broken))
    }

    @Test
    fun aBandIsWhatWasPrescribedAndNotWhatWasTaken() {
        // No `DoseEvent` reaches this function: the seed's history stops at seededThrough
        // holding one dose, but the bands run two months further with three different doses.
        val bands = doseBands(MockSeed.plan, MockSeed.semaItemId, WHOLE_COURSE)

        assertEquals(listOf(QUARTER, HALF, WHOLE), bands.map { it.dose }, "three prescribed doses")
        assertEquals(
            setOf(QUARTER),
            MockSeed.history
                .filter { it.protocolItemId == MockSeed.semaItemId }
                .map { it.dose }
                .toSet(),
            "one dose ever logged — the patient is still in the first band",
        )
        assertTrue(
            bands.last().range.through > MockSeed.seededThrough,
            "the prescription outruns the history it was measured against",
        )
    }

    @Test
    fun aWindowInsideOnePhaseSeesThatPhaseClippedToIt() {
        assertEquals(
            listOf(Triple(QUARTER, LocalDate(2026, 5, 25), LocalDate(2026, 5, 31))),
            bandsOf(TrendRange(LocalDate(2026, 5, 25), LocalDate(2026, 5, 31))),
        )
    }

    @Test
    fun aWindowStraddlingATitrationSeesBothBandsAndTheJoinBetweenThem() {
        val bands = bandsOf(TrendRange(LocalDate(2026, 6, 4), LocalDate(2026, 6, 10)))

        assertEquals(
            listOf(
                Triple(QUARTER, LocalDate(2026, 6, 4), LocalDate(2026, 6, 6)),
                Triple(HALF, LocalDate(2026, 6, 7), LocalDate(2026, 6, 10)),
            ),
            bands,
        )
    }

    @Test
    fun aWindowThatMissesTheCourseEntirelyHasNoBands() {
        // Before it, and after it.
        assertEquals(emptyList(), bandsOf(TrendRange(LocalDate(2026, 3, 1), LocalDate(2026, 5, 9))))
        assertEquals(emptyList(), bandsOf(TrendRange(LocalDate(2026, 8, 2), LocalDate(2026, 9, 1))))
    }

    @Test
    fun aWindowTouchingTheCourseByOneDaySeesOneDayOfIt() {
        // A one-day band is what «весь цикл» asks for on the day a course begins
        // (TrendWindow.CYCLE returns exactly (start, start)); an overlap test written
        // `from >= through` would leave a patient's first day with no prescription under it.
        assertEquals(
            listOf(Triple(QUARTER, START, START)),
            bandsOf(TrendRange(LocalDate(2026, 3, 1), START)),
        )
        assertEquals(
            listOf(Triple(WHOLE, LocalDate(2026, 8, 1), LocalDate(2026, 8, 1))),
            bandsOf(TrendRange(LocalDate(2026, 8, 1), LocalDate(2026, 9, 1))),
        )
    }

    @Test
    fun theWeeksAreCountedFromWhicheverDayTheCourseBeganOn() {
        // Every other fixture here starts on 10 May 2026, so a baked-in date would pass them
        // all. This one crosses a new year and a leap February — measured, not assumed.
        val leapYear =
            ProtocolPlan(
                protocol = Protocol(PID, UserId("p"), LocalDate(2027, 12, 20), 12, ProtocolStatus.ACTIVE, null, null),
                items = emptyList(),
                phases = mapOf(SEMA to SHUFFLED_PHASES),
            )

        assertEquals(
            listOf(
                // Day 0 → 27, across the new year.
                Triple(QUARTER, LocalDate(2027, 12, 20), LocalDate(2028, 1, 16)),
                // Day 28 → 55.
                Triple(HALF, LocalDate(2028, 1, 17), LocalDate(2028, 2, 13)),
                // Day 56 → 83, across 29 February: 2028 is a leap year, so this band moves
                // under a 365-day assumption.
                Triple(WHOLE, LocalDate(2028, 2, 14), LocalDate(2028, 3, 12)),
            ),
            doseBands(leapYear, SEMA, TrendRange(LocalDate(2027, 1, 1), LocalDate(2029, 1, 1)))
                .map { Triple(it.dose, it.range.from, it.range.through) },
        )
    }

    @Test
    fun aCancelledCourseDrawsNoBandsAtAll() {
        // Same answer `phaseDose` gives, for the same reason.
        assertEquals(emptyList(), bandsOf(WHOLE_COURSE, plan(status = ProtocolStatus.CANCELLED)))
        assertEquals(
            emptyList(),
            protocolMarks(plan(status = ProtocolStatus.CANCELLED), SEMA, WHOLE_COURSE),
        )
    }

    @Test
    fun anItemWithNoPhasesHasNoBands() {
        // Two different absences, different branches: unheard-of in the phase map (weigh-in)
        // vs. present with an empty list.
        assertEquals(emptyList(), doseBands(PLAN, WEIGH_IN, WHOLE_COURSE))
        assertEquals(emptyList(), protocolMarks(PLAN, WEIGH_IN, WHOLE_COURSE))

        val dosedNothing = plan(phases = emptyList())
        assertEquals(emptyList(), doseBands(dosedNothing, SEMA, WHOLE_COURSE))
        assertEquals(emptyList(), protocolMarks(dosedNothing, SEMA, WHOLE_COURSE))
    }

    @Test
    fun aPhaseReachingPastTheCourseIsCutAtItsLastDay() {
        // Malformed, not impossible: §03 leaves `to_week`/`weeks` unjoined, so week 20 of a
        // twelve-week course is representable.
        val overrun = plan(phases = listOf(ProtocolPhase(1, 20, QUARTER)))

        assertEquals(
            listOf(Triple(QUARTER, START, LocalDate(2026, 8, 1))),
            bandsOf(WHOLE_COURSE, overrun),
        )
    }
}

class ProtocolMarksTest {
    @Test
    fun theCourseIsMarkedWhereItStartedAndWhereEachDoseWentUp() {
        val marks = protocolMarks(PLAN, SEMA, WHOLE_COURSE)

        assertEquals(
            listOf(
                ProtocolMark(ProtocolMarkKind.START, START, from = null, to = QUARTER),
                ProtocolMark(ProtocolMarkKind.TITRATION, LocalDate(2026, 6, 7), from = QUARTER, to = HALF),
                ProtocolMark(ProtocolMarkKind.TITRATION, LocalDate(2026, 7, 5), from = HALF, to = WHOLE),
            ),
            marks,
        )
    }

    @Test
    fun aMarkSitsOnTheFirstDayOfTheBandItOpens() {
        val bands = doseBands(PLAN, SEMA, WHOLE_COURSE)
        val titrations = protocolMarks(PLAN, SEMA, WHOLE_COURSE).filter { it.kind == ProtocolMarkKind.TITRATION }

        assertEquals(bands.drop(1).map { it.range.from }, titrations.map { it.date })
        assertEquals(bands.drop(1).map { it.dose }, titrations.map { it.to })
    }

    @Test
    fun marksOutsideTheWindowAreLeftOut() {
        // The seeded «весь цикл» window: three weeks lived, so the start is in and both
        // titrations are still ahead.
        val cycle = requireNotNull(TrendWindow.CYCLE.rangeOn(MockSeed.plan, LocalDate(2026, 5, 31)))
        val marks = protocolMarks(MockSeed.plan, MockSeed.semaItemId, cycle)

        assertEquals(listOf(ProtocolMarkKind.START), marks.map { it.kind })
        assertEquals(MockSeed.cycleStart, marks.single().date)
    }

    @Test
    fun bothEdgesOfTheWindowKeepTheirMarks() {
        // A «7 дней» window can close exactly on the day a dose went up. Left edge is
        // measured by the test above, where the start sits on `range.from`.
        val marks = protocolMarks(PLAN, SEMA, TrendRange(LocalDate(2026, 6, 1), LocalDate(2026, 6, 7)))

        assertEquals(listOf(LocalDate(2026, 6, 7)), marks.map { it.date })
    }

    @Test
    fun aMarkPastTheEndOfTheCourseIsDroppedLikeTheBandUnderIt() {
        // Second phase entirely outside a twelve-week course: its band is cut away, so a
        // dashed line left behind would stand over nothing.
        val overrun = plan(phases = listOf(ProtocolPhase(1, 12, QUARTER), ProtocolPhase(13, 20, HALF)))

        assertEquals(
            listOf(Triple(QUARTER, START, LocalDate(2026, 8, 1))),
            bandsOf(WHOLE_COURSE, overrun),
        )
        assertEquals(
            listOf(ProtocolMarkKind.START),
            protocolMarks(overrun, SEMA, WHOLE_COURSE).map { it.kind },
        )
        // The third reader agrees: `titrationStepAfter` is what `CadenceMocks` hands the
        // schedule screen, and a step past the course's end would have the schedule promise
        // «Доза растёт» after the chart drops the line. The clip lives in `titrationSteps`
        // so all three inherit it.
        assertEquals(emptyList(), titrationSteps(overrun, SEMA))
        assertNull(titrationStepAfter(overrun, SEMA, LocalDate(2026, 5, 31)))
    }

    @Test
    fun twoPhasesHoldingTheSameDoseAreNotATitration() {
        // §03 lets a course split its phase table without changing the dose. Two bands, one
        // dose, no dashed line: «0,5 мг → 0,5 мг» is a change that didn't happen.
        val held = plan(phases = listOf(ProtocolPhase(1, 4, HALF), ProtocolPhase(5, 8, HALF)))

        assertEquals(2, doseBands(held, SEMA, WHOLE_COURSE).size)
        assertEquals(listOf(ProtocolMarkKind.START), protocolMarks(held, SEMA, WHOLE_COURSE).map { it.kind })
        assertEquals(emptyList(), titrationSteps(held, SEMA))
    }

    @Test
    fun aCancelledCourseHasNoNextStepEither() {
        // The third reader of the same phases: a withdrawn prescription must not still
        // promise «Доза растёт».
        val cancelled = plan(status = ProtocolStatus.CANCELLED)

        assertEquals(emptyList(), titrationSteps(cancelled, SEMA))
        assertNull(titrationStepAfter(cancelled, SEMA, LocalDate(2026, 5, 31)))
    }

    @Test
    fun aWindowOpeningMidCourseCarriesNoStartMark() {
        val marks = protocolMarks(PLAN, SEMA, TrendRange(LocalDate(2026, 6, 1), LocalDate(2026, 7, 31)))

        assertEquals(
            listOf(LocalDate(2026, 6, 7), LocalDate(2026, 7, 5)),
            marks.map { it.date },
        )
        assertTrue(marks.none { it.kind == ProtocolMarkKind.START })
    }

    @Test
    fun anItemDosedFromTheSecondWeekIsMarkedWhereItsFirstBandOpens() {
        // §03 lets a phase begin after week 1 (a wash-in). The start belongs on the day the
        // item was first prescribed, not the protocol's own start date.
        val washIn = plan(phases = listOf(ProtocolPhase(2, 4, QUARTER), ProtocolPhase(5, 8, HALF)))
        val marks = protocolMarks(washIn, SEMA, WHOLE_COURSE)

        assertEquals(
            listOf(
                // Day 7, not day 0.
                ProtocolMark(ProtocolMarkKind.START, LocalDate(2026, 5, 17), from = null, to = QUARTER),
                ProtocolMark(ProtocolMarkKind.TITRATION, LocalDate(2026, 6, 7), from = QUARTER, to = HALF),
            ),
            marks,
        )
        // Bands under it: first opens the same day as the mark, last closes at its own
        // phase's end, not running on to the end of a course it doesn't cover.
        val bands = doseBands(washIn, SEMA, WHOLE_COURSE)
        assertEquals(LocalDate(2026, 5, 17), bands.first().range.from)
        assertEquals(LocalDate(2026, 7, 4), bands.last().range.through)
    }

    @Test
    fun anItemWithOneBandStartsAndNeverTitrates() {
        val marks = protocolMarks(MockSeed.plan, MockSeed.bpcItemId, WHOLE_COURSE)

        assertEquals(listOf(ProtocolMarkKind.START), marks.map { it.kind })
        assertEquals(Dose(250.0, DoseUnit.MCG), marks.single().to)
    }

    @Test
    fun theSeedTitratesOnTheDaysThePrototypesScheduleDoesAndNotItsChart() {
        // The prototype disagrees with itself: schedule/data.ts puts the steps at days 28
        // and 56, trends/data.ts draws the second at day 70. §03's phases give 28 and 56, so
        // the chart is wrong — porting it would draw a titration two weeks late.
        val marks =
            protocolMarks(MockSeed.plan, MockSeed.semaItemId, WHOLE_COURSE)
                .filter { it.kind == ProtocolMarkKind.TITRATION }

        assertEquals(listOf(LocalDate(2026, 6, 7), LocalDate(2026, 7, 5)), marks.map { it.date })
        assertEquals(
            listOf(28, 56),
            marks.map {
                MockSeed.plan.protocol.startDate
                    .daysUntil(it.date)
            },
        )
    }
}

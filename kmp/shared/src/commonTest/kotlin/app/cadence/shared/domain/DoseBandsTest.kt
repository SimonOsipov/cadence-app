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
 * The seed's three bands, out of order on purpose: §03 does not promise the
 * server sorts them, and a fixture already sorted lets a missing `sortedBy`
 * through.
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
        // Week N begins (N-1)×7 days after the start, so weeks 1–4 are days
        // 0–27 and week 5 opens on day 28 — §03's titration table read against
        // `protocol.startDate`, and the same convention
        // `mobile/src/features/schedule/data.ts:113-115` uses. (The trends
        // module's `DOSE_SPANS` disagrees; see
        // `theSeedTitratesOnTheDaysThePrototypesScheduleDoesAndNotItsChart`.)
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
        // Day 83 — the last prescribed day of a twelve-week course.
        assertEquals(LocalDate(2026, 8, 1), bands.last().range.through)
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
        // The contract between the strip and the dots, asserted day by day
        // rather than at the four dates a literal test happens to name:
        // `phaseDose` is what the calendar stamps on an occurrence, and a strip
        // that disagreed with it would draw a dose the patient was never asked
        // to take.
        //
        // Run over the gapped course as well as the contiguous one, and that is
        // where the invariant bites: on a contiguous course `phaseDose` is
        // never null, so the «no band over an unprescribed day» half is not
        // exercised by `PLAN` alone.
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
        // A deliberate break — four weeks on, four weeks off, four weeks on.
        // §03 constrains nothing about phases being contiguous, and a strip
        // that ran straight through would tell a patient they were on drug
        // during a wash-out.
        //
        // This is the fixture that makes `phase.toWeek` observable: a band that
        // closed where the *next* phase opens instead of where its own ends
        // draws the same three spans for a contiguous course, and only a gap
        // separates the two implementations. Do not let it become contiguous.
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
        // No `DoseEvent` reaches this function, and the seed is where that
        // shows: its history stops at `seededThrough` (30 May) and holds one
        // dose, 0,25 мг, because that is all the patient has reached. The bands
        // run two months further and carry three different doses.
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
        // What «7 дней» asks for on almost any day of a course.
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
        // Before it, and after it. «3 месяца» on a patient in week two reaches
        // back past the start; a finished course sits behind a later window.
        assertEquals(emptyList(), bandsOf(TrendRange(LocalDate(2026, 3, 1), LocalDate(2026, 5, 9))))
        assertEquals(emptyList(), bandsOf(TrendRange(LocalDate(2026, 8, 2), LocalDate(2026, 9, 1))))
    }

    @Test
    fun aWindowTouchingTheCourseByOneDaySeesOneDayOfIt() {
        // The failing side above is one day either way from these. A one-day
        // band is what «весь цикл» asks for on the day a course begins — the
        // range `TrendWindow.CYCLE` returns is exactly `(start, start)` — so an
        // overlap test written `from >= through` would leave a patient's first
        // day with an axis and no prescription under it.
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
        // Every other fixture here and in `TitrationTest` starts on 10 May
        // 2026, so an implementation that had baked that date in would pass
        // them all. This one crosses a new year and a leap February — the
        // arithmetic is measured rather than assumed.
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
                // Day 56 → 83, across 29 February — 2028 is a leap year, so
                // this band is the one whose arithmetic a 365-day assumption
                // would move.
                Triple(WHOLE, LocalDate(2028, 2, 14), LocalDate(2028, 3, 12)),
            ),
            doseBands(leapYear, SEMA, TrendRange(LocalDate(2027, 1, 1), LocalDate(2029, 1, 1)))
                .map { Triple(it.dose, it.range.from, it.range.through) },
        )
    }

    @Test
    fun aCancelledCourseDrawsNoBandsAtAll() {
        // The same answer `phaseDose` gives, and for its reason: a cancelled
        // course has no defensible prescription to draw, and §03 gives
        // `protocols` no `cancelled_at` to bound one with.
        assertEquals(emptyList(), bandsOf(WHOLE_COURSE, plan(status = ProtocolStatus.CANCELLED)))
        assertEquals(
            emptyList(),
            protocolMarks(plan(status = ProtocolStatus.CANCELLED), SEMA, WHOLE_COURSE),
        )
    }

    @Test
    fun anItemWithNoPhasesHasNoBands() {
        // Two different absences, and they take different branches. An item
        // the phase map has never heard of — a weigh-in, which is prescribed
        // but not dosed — and an item present with an empty list.
        assertEquals(emptyList(), doseBands(PLAN, WEIGH_IN, WHOLE_COURSE))
        assertEquals(emptyList(), protocolMarks(PLAN, WEIGH_IN, WHOLE_COURSE))

        val dosedNothing = plan(phases = emptyList())
        assertEquals(emptyList(), doseBands(dosedNothing, SEMA, WHOLE_COURSE))
        assertEquals(emptyList(), protocolMarks(dosedNothing, SEMA, WHOLE_COURSE))
    }

    @Test
    fun aPhaseReachingPastTheCourseIsCutAtItsLastDay() {
        // Malformed rather than impossible: §03 stores `to_week` per phase and
        // `weeks` per protocol, and nothing joins them. Drawing to week 20 of a
        // twelve-week course would put bands under an axis `cycleWeek` calls
        // outside the protocol.
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
        // The join the chart draws its dashed line on: every titration mark
        // lands on a band's opening day, and never between two.
        val bands = doseBands(PLAN, SEMA, WHOLE_COURSE)
        val titrations = protocolMarks(PLAN, SEMA, WHOLE_COURSE).filter { it.kind == ProtocolMarkKind.TITRATION }

        assertEquals(bands.drop(1).map { it.range.from }, titrations.map { it.date })
        assertEquals(bands.drop(1).map { it.dose }, titrations.map { it.to })
    }

    @Test
    fun marksOutsideTheWindowAreLeftOut() {
        // The seeded «весь цикл» window on the day the app opens: three weeks
        // lived, so the start is in and both titrations are still ahead.
        val cycle = requireNotNull(TrendWindow.CYCLE.rangeOn(MockSeed.plan, LocalDate(2026, 5, 31)))
        val marks = protocolMarks(MockSeed.plan, MockSeed.semaItemId, cycle)

        assertEquals(listOf(ProtocolMarkKind.START), marks.map { it.kind })
        assertEquals(MockSeed.cycleStart, marks.single().date)
    }

    @Test
    fun bothEdgesOfTheWindowKeepTheirMarks() {
        // A «7 дней» window can close exactly on the day a dose went up, and
        // that is the day the mark exists to show. The left edge is measured by
        // the test above, where the start sits on `range.from`.
        val marks = protocolMarks(PLAN, SEMA, TrendRange(LocalDate(2026, 6, 1), LocalDate(2026, 6, 7)))

        assertEquals(listOf(LocalDate(2026, 6, 7)), marks.map { it.date })
    }

    @Test
    fun aMarkPastTheEndOfTheCourseIsDroppedLikeTheBandUnderIt() {
        // Two phases where the second is entirely outside a twelve-week course.
        // Its band is cut away; a dashed «доза выросла» line left behind would
        // stand over nothing, on a date `cycleWeek` calls outside the protocol.
        // Reachable: the three length windows close on `today`, not on the
        // course's last day.
        val overrun = plan(phases = listOf(ProtocolPhase(1, 12, QUARTER), ProtocolPhase(13, 20, HALF)))

        assertEquals(
            listOf(Triple(QUARTER, START, LocalDate(2026, 8, 1))),
            bandsOf(WHOLE_COURSE, overrun),
        )
        assertEquals(
            listOf(ProtocolMarkKind.START),
            protocolMarks(overrun, SEMA, WHOLE_COURSE).map { it.kind },
        )
        // And the third reader of the same phases agrees. `titrationStepAfter`
        // is what `CadenceMocks` hands the schedule screen as `nextTitration`:
        // a step on 2 August, one day past a course that ends on the 1st, would
        // have the chart drop the dashed line while the schedule still promised
        // «Доза растёт». The clip lives in `titrationSteps` so that all three
        // inherit it.
        assertEquals(emptyList(), titrationSteps(overrun, SEMA))
        assertNull(titrationStepAfter(overrun, SEMA, LocalDate(2026, 5, 31)))
    }

    @Test
    fun twoPhasesHoldingTheSameDoseAreNotATitration() {
        // §03 lets a course split its phase table without changing the dose —
        // «hold 0,5 for another four weeks». Two bands, one dose, and no
        // dashed line, because «0,5 мг → 0,5 мг» is a change that did not
        // happen.
        val held = plan(phases = listOf(ProtocolPhase(1, 4, HALF), ProtocolPhase(5, 8, HALF)))

        assertEquals(2, doseBands(held, SEMA, WHOLE_COURSE).size)
        assertEquals(listOf(ProtocolMarkKind.START), protocolMarks(held, SEMA, WHOLE_COURSE).map { it.kind })
        assertEquals(emptyList(), titrationSteps(held, SEMA))
    }

    @Test
    fun aCancelledCourseHasNoNextStepEither() {
        // `doseBands` and `protocolMarks` already answer nothing for a
        // cancelled course. `titrationStepAfter` is the third reader of the same
        // phases — it is what `CadenceMocks` hands the schedule screen as
        // `nextTitration` — and a withdrawn prescription must not still promise
        // «Доза растёт: 0,25 мг → 0,5 мг».
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
        // §03 lets a phase begin after week 1, which is how a course writes a
        // wash-in. The start belongs on the day the item was first prescribed —
        // not on the protocol's own start date, where nothing had been given
        // yet, and not nowhere, which would leave the band unexplained.
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
        // And the bands under it: the first opens on the same day the mark
        // does, and the last closes where its own phase ends — week 8, day 55 —
        // rather than running on to the end of a course it does not cover.
        val bands = doseBands(washIn, SEMA, WHOLE_COURSE)
        assertEquals(LocalDate(2026, 5, 17), bands.first().range.from)
        assertEquals(LocalDate(2026, 7, 4), bands.last().range.through)
    }

    @Test
    fun anItemWithOneBandStartsAndNeverTitrates() {
        // BPC-157 in the seed: one phase across all twelve weeks.
        val marks = protocolMarks(MockSeed.plan, MockSeed.bpcItemId, WHOLE_COURSE)

        assertEquals(listOf(ProtocolMarkKind.START), marks.map { it.kind })
        assertEquals(Dose(250.0, DoseUnit.MCG), marks.single().to)
    }

    @Test
    fun theSeedTitratesOnTheDaysThePrototypesScheduleDoesAndNotItsChart() {
        // The prototype disagrees with itself. `schedule/data.ts:114-115` puts
        // the two steps at `CYCLE_START + 4×7` and `+ 8×7` — days 28 and 56 —
        // while `trends/data.ts:21` draws the second at day 70. §03's phases
        // give 28 and 56, so the chart is the one that is wrong, and porting it
        // would draw a titration two weeks after the dose actually changed.
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

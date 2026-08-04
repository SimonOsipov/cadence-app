package app.cadence.shared.mock

import app.cadence.shared.domain.Dose
import app.cadence.shared.domain.DoseDraft
import app.cadence.shared.domain.DoseUnit
import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.domain.InjectionSite
import app.cadence.shared.domain.JournalSource
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.OccurrenceStatus
import app.cadence.shared.domain.PartOfDay
import app.cadence.shared.domain.ProtocolItemId
import app.cadence.shared.domain.ProtocolItemKind
import app.cadence.shared.domain.SideEffect
import app.cadence.shared.domain.VialId
import app.cadence.shared.domain.selectItem
import app.cadence.shared.repository.DoseLogResult
import app.cadence.shared.repository.MeasurementsRepository
import kotlinx.coroutines.test.runTest
import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertNotEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

private val ZONE = TimeZone.of("Europe/Moscow")

/** The day the seed's weekly injection falls on, in [ZONE]. */
private val SEEDED_SUNDAY = LocalDate(2026, 5, 31)

/** The mock wound to a moment, so «today» is the test's choice and not the machine's. */
private fun mocks(iso: String = "2026-05-31T09:00:00Z") = CadenceMocks(clock = FixedCadenceClock.at(iso), zone = ZONE)

/**
 * A draft a patient could actually have finished: item chosen from the plan, so
 * the dose comes from the phase in force, and a zone, because an injection
 * needs one.
 */
private fun injectionDraft(
    itemId: ProtocolItemId,
    on: LocalDate = SEEDED_SUNDAY,
) = DoseDraft().selectItem(MockSeed.plan, itemId, on).copy(site = InjectionSite.LEFT_THIGH)

class MockRepositoryTest {
    @Test
    fun todayReportsTheNextDoseAsDataRatherThanAsAString() =
        runTest {
            val summary = mocks().today.today()

            val next = assertNotNull(summary.nextDose, "the seeded protocol has a dose due today")
            assertEquals(0.25, next.dose?.value)
            assertEquals(DoseUnit.MG, next.dose?.unit)
        }

    @Test
    fun theSummaryCarriesTheDayItIsAboutAndTheHourOfIt() =
        runTest {
            // The greeting is «Воскресенье, утро · 4-я неделя» in the prototype,
            // frozen. Both halves are facts about the clock, and the zone is
            // the patient's: 09:00 UTC is noon in Moscow, which is «день».
            val morning = mocks("2026-05-31T04:00:00Z").today.today()
            val afternoon = mocks("2026-05-31T09:00:00Z").today.today()

            assertEquals(LocalDate(2026, 5, 31), morning.date)
            assertEquals(PartOfDay.MORNING, morning.partOfDay)
            assertEquals(PartOfDay.AFTERNOON, afternoon.partOfDay)
        }

    @Test
    fun theCycleWeekMovesWhenTheClockDoes() =
        runTest {
            // The whole point of the clock seam: nothing here is pinned to
            // 31 May the way the prototype's three «todays» are.
            assertEquals(4, mocks("2026-05-31T09:00:00Z").today.today().cycleWeek)
            assertEquals(5, mocks("2026-06-07T09:00:00Z").today.today().cycleWeek)
        }

    @Test
    fun loggingADoseThroughTheInterfaceIsVisibleThroughTheInterface() =
        runTest {
            // The block's acceptance criterion in one test: a write goes in
            // through the repository and comes back out of it, with no screen
            // involved and nothing shared but the interface.
            val m = mocks()
            val before = m.today.today()
            assertFalse(before.doseLoggedToday, "the seeded day starts unlogged")

            m.dosing.submit(injectionDraft(assertNotNull(before.nextDose).itemId))

            assertTrue(m.today.today().doseLoggedToday)
        }

    @Test
    fun aLoggedDoseComesOutOfTheVialItWasDrawnFrom() =
        runTest {
            // §03's third correction, end to end through the seam rather than
            // in the arithmetic alone.
            val m = mocks()
            val before = m.today.today().vialDosesLeft

            m.dosing.submit(injectionDraft(assertNotNull(m.today.today().nextDose).itemId))

            assertEquals(before - 1, m.today.today().vialDosesLeft)
        }

    @Test
    fun oneCheckInWritesADoseEventAndAJournalEntryForTheSameDay() =
        runTest {
            // §03's «one action, two facts». Two calls from a screen can
            // half-fail, and a dose recorded without the side effect the
            // patient reported is worse than a check-in that failed outright.
            val m = mocks()
            val draft =
                injectionDraft(assertNotNull(m.today.today().nextDose).itemId)
                    .copy(
                        mood = 4,
                        sideEffects = listOf(SideEffect.NAUSEA, SideEffect.FATIGUE),
                        note = "чуть тянет",
                    )

            val written = assertIs<DoseLogResult.Written>(m.dosing.submit(draft))

            assertEquals(SEEDED_SUNDAY, written.journalDate)
            val entry = assertNotNull(m.journal.entry(written.journalDate), "the second fact")
            assertEquals(JournalSource.DOSE, entry.source)
            assertEquals(4, entry.mood)
            assertEquals(listOf(SideEffect.NAUSEA, SideEffect.FATIGUE), entry.tags)
            assertEquals("чуть тянет", entry.note)
            assertEquals(written.journalDate, entry.entryDate)
        }

    @Test
    fun submittingTheSameOccurrenceTwiceWritesOneEventAndTakesOneDose() =
        runTest {
            val m = mocks()
            val itemId = assertNotNull(m.today.today().nextDose).itemId
            val before = m.today.today().vialDosesLeft

            assertIs<DoseLogResult.Written>(m.dosing.submit(injectionDraft(itemId)))
            val second = m.dosing.submit(injectionDraft(itemId))

            // Named, not merely refused: the screen has to be able to say «эта
            // доза уже записана» rather than «что-то пошло не так».
            assertEquals(DoseLogResult.AlreadyLogged, second)
            assertEquals(before - 1, m.today.today().vialDosesLeft, "the vial paid once")
        }

    @Test
    fun anItemDosedTwiceADayCanBeLoggedTwiceAndThenNoMore() =
        runTest {
            // §03 gives BPC-157 `times = [08:00, 20:00]`. Resolving «the first
            // occurrence» rather than «the first one still open» would let the
            // patient log the morning dose and then be told the evening one was
            // already recorded — the defect `statusOf` was fixed for, one layer
            // up.
            val m = mocks()

            val morning = assertIs<DoseLogResult.Written>(m.dosing.submit(injectionDraft(MockSeed.bpcItemId)))
            val evening = assertIs<DoseLogResult.Written>(m.dosing.submit(injectionDraft(MockSeed.bpcItemId)))
            val third = m.dosing.submit(injectionDraft(MockSeed.bpcItemId))

            assertNotEquals(morning.eventId, evening.eventId, "two slots, two events")
            assertEquals(DoseLogResult.AlreadyLogged, third)
        }

    @Test
    fun theEventCarriesTheZoneThePatientChoseAndNotTheOneItSuggested() =
        runTest {
            // The one path on which `DoseEvent.site` is observable today, and
            // the reason to have it: the rotation reads the events back.
            //
            // The chosen zone is deliberately *not* the suggested one. A write
            // that stamped `suggestNextSite(events)` instead of the draft's
            // zone would pass a test that injected into the suggestion, and it
            // is the same defect as recording the planned dose instead of the
            // taken one.
            // Two check-ins, in two mocks, because one does not separate the
            // two ways this can be wrong — and neither assertion computes the
            // rotation's answer, which would only be the production rule asked
            // twice.
            //
            // Injecting into the *suggested* zone must move the suggestion: its
            // recency is what made it the answer. Injecting into any *other*
            // zone must leave the suggestion exactly where it was, because the
            // suggested zone was not touched — which is false for a write that
            // stamps its own suggestion instead of the patient's choice.
            val intoTheSuggestion = mocks()
            val suggested = intoTheSuggestion.today.today().suggestedSite

            intoTheSuggestion.dosing.submit(
                injectionDraft(assertNotNull(intoTheSuggestion.today.today().nextDose).itemId)
                    .copy(site = suggested),
            )

            assertNotEquals(suggested, intoTheSuggestion.today.today().suggestedSite, "the rotation did not move")

            val intoAnother = mocks()
            val chosen = InjectionSite.entries.first { it != suggested }

            intoAnother.dosing.submit(
                injectionDraft(assertNotNull(intoAnother.today.today().nextDose).itemId).copy(site = chosen),
            )

            assertEquals(
                suggested,
                intoAnother.today.today().suggestedSite,
                "injecting elsewhere moved a suggestion that nothing touched",
            )
        }

    @Test
    fun aDoseIsDrawnFromTheFullestOpenVialOfItsCompound() =
        runTest {
            // BPC-157 has four vials: one thrown out with six doses still in
            // it, two open, one sealed. Every wrong answer is a different vial,
            // which is the point of seeding it this way — with one vial per
            // compound «the fullest open one» and «the first in the list» are
            // the same vial and neither can be wrong.
            val m = mocks()

            val written = assertIs<DoseLogResult.Written>(m.dosing.submit(injectionDraft(MockSeed.bpcItemId)))

            assertEquals(VialId("vial-bpc-3"), written.vialId)
        }

    @Test
    fun theWriteDrawsFromTheVialTheDraftNamesRatherThanItsOwnChoice() =
        runTest {
            // The picker's whole purpose. Its default and the write's default
            // are the same rule — «the fullest open vial» — so a write that
            // ignored the draft would agree with the screen right up until the
            // patient chose the other one.
            val m = mocks()
            val chosen = VialId("vial-bpc-2")

            val written =
                assertIs<DoseLogResult.Written>(
                    m.dosing.submit(injectionDraft(MockSeed.bpcItemId).copy(vialId = chosen)),
                )

            assertEquals(chosen, written.vialId)
        }

    @Test
    fun theEventCarriesTheDoseTheDraftHeldAndNotThePlansOwnNumber() =
        runTest {
            // A patient who steps the dose down records what they took. A write
            // that read `phaseDose` again would stamp the prescription over the
            // fact, which is the prototype's «re-apply comp.default» bug moved
            // one layer down.
            val m = mocks()
            val stepped = Dose(0.2, DoseUnit.MG)
            val draft = injectionDraft(assertNotNull(m.today.today().nextDose).itemId).copy(dose = stepped)

            val written = assertIs<DoseLogResult.Written>(m.dosing.submit(draft))

            assertEquals(stepped, written.dose)
        }

    @Test
    fun anItemTheProtocolDoesNotMarkLoggableIsRefusedByTheWriteItself() =
        runTest {
            // The rule lives in `selectItem` too, but a caller that built its
            // draft with the constructor never went through the chooser — and
            // the app's own placeholder does exactly that.
            val m = mocks()
            val draft =
                DoseDraft(
                    itemId = MockSeed.glycineItemId,
                    kind = ProtocolItemKind.SUPPLEMENT,
                    dose = Dose(1.0, DoseUnit.MG),
                )

            assertEquals(DoseLogResult.NotScheduledToday, m.dosing.submit(draft))
            assertNull(m.journal.entry(SEEDED_SUNDAY))
        }

    @Test
    fun oneSideEffectReportedTwiceInADayIsOneTag() =
        runTest {
            val m = mocks()
            m.dosing.submit(injectionDraft(MockSeed.bpcItemId).copy(sideEffects = listOf(SideEffect.NAUSEA)))

            val second =
                assertIs<DoseLogResult.Written>(
                    m.dosing.submit(
                        injectionDraft(assertNotNull(m.today.today().nextDose).itemId)
                            .copy(sideEffects = listOf(SideEffect.NAUSEA, SideEffect.FATIGUE)),
                    ),
                )

            assertEquals(
                listOf(SideEffect.NAUSEA, SideEffect.FATIGUE),
                assertNotNull(m.journal.entry(second.journalDate)).tags,
            )
        }

    @Test
    fun theJournalAnswersForTheDayItWasAskedAbout() =
        runTest {
            // §03 keys the entry by date. A read that returned whatever entry
            // existed would look right for as long as there was only one.
            val m = mocks()
            val written =
                assertIs<DoseLogResult.Written>(
                    m.dosing.submit(injectionDraft(assertNotNull(m.today.today().nextDose).itemId)),
                )

            assertNotNull(m.journal.entry(written.journalDate))
            assertNull(m.journal.entry(LocalDate(2026, 5, 30)), "yesterday has no entry")
            assertNull(m.journal.entry(LocalDate(2026, 6, 1)), "nor does tomorrow")
        }

    @Test
    fun aSecondCheckInOnTheSameDayUpdatesTheOneJournalEntry() =
        runTest {
            // §03 keys the journal `UNIQUE(patient, date)`. The seeded Sunday
            // carries both the weekly injection and the twice-daily one, so two
            // check-ins on one day is the ordinary case and not an edge.
            val m = mocks()
            val morning = injectionDraft(MockSeed.bpcItemId).copy(mood = 2, sideEffects = listOf(SideEffect.NAUSEA))
            val later =
                injectionDraft(assertNotNull(m.today.today().nextDose).itemId)
                    .copy(mood = 5, sideEffects = listOf(SideEffect.HEADACHE))

            val first = assertIs<DoseLogResult.Written>(m.dosing.submit(morning))
            val second = assertIs<DoseLogResult.Written>(m.dosing.submit(later))

            assertEquals(first.journalDate, second.journalDate, "one entry, not two")
            val entry = assertNotNull(m.journal.entry(second.journalDate))
            assertEquals(5, entry.mood, "the latest answer to «как вы себя чувствуете»")
            assertEquals(
                listOf(SideEffect.NAUSEA, SideEffect.HEADACHE),
                entry.tags,
                "a side effect reported this morning did not stop being true by evening",
            )
        }

    @Test
    fun aCheckInThatSkipsTheContextDoesNotEraseWhatAnEarlierOneSaid() =
        runTest {
            // Step 4 is «всё по желанию», so an unanswered field means
            // «пропущено» and not «сотрите то, что я сказал утром».
            val m = mocks()
            m.dosing.submit(injectionDraft(MockSeed.bpcItemId).copy(mood = 2, note = "тяжело"))

            val second =
                assertIs<DoseLogResult.Written>(
                    m.dosing.submit(injectionDraft(assertNotNull(m.today.today().nextDose).itemId)),
                )

            val entry = assertNotNull(m.journal.entry(second.journalDate))
            assertEquals(2, entry.mood)
            assertEquals("тяжело", entry.note)
        }

    @Test
    fun anIncompleteDraftWritesNeitherFact() =
        runTest {
            val m = mocks()
            val noSite = injectionDraft(assertNotNull(m.today.today().nextDose).itemId).copy(site = null)

            assertEquals(DoseLogResult.Incomplete, m.dosing.submit(noSite))
            assertFalse(m.today.today().doseLoggedToday)
            assertNull(m.journal.entry(SEEDED_SUNDAY), "no half-write")
        }

    @Test
    fun anItemWithNoOccurrenceTodayWritesNeitherFact() =
        runTest {
            // The weekly injection falls on Sunday; this is the Monday after.
            val m = mocks("2026-06-01T09:00:00Z")

            assertEquals(DoseLogResult.NotScheduledToday, m.dosing.submit(injectionDraft(MockSeed.semaItemId)))
            assertNull(m.journal.entry(LocalDate(2026, 6, 1)))
        }

    @Test
    fun aCompoundWithNoVialIsStillLoggedAndTakesNothingFromAnotherCompoundsVial() =
        runTest {
            // The seed stocks semaglutide and nothing else. A dose event whose
            // `vialId` is null is honest; one that decremented whatever vial
            // came first in the list is the prototype's disconnected inventory
            // wearing a different shape.
            val m = mocks()
            val before = m.today.today().vialDosesLeft

            assertIs<DoseLogResult.Written>(m.dosing.submit(injectionDraft(MockSeed.bpcItemId)))

            assertEquals(before, m.today.today().vialDosesLeft)
        }

    @Test
    fun theScheduleAndTodayAgreeAboutTheSameDay() =
        runTest {
            // §03's seventh correction: the Today strip and the Schedule screen
            // render the same generated occurrences. The prototype's two
            // disagree because each has its own hardcoded copy.
            val m = mocks()

            val fromSchedule = m.schedule.day(LocalDate(2026, 5, 31))
            val fromToday = m.today.today().nextDose

            assertEquals(
                fromSchedule.first { it.status == OccurrenceStatus.PENDING }.itemId,
                fromToday?.itemId,
            )
        }

    @Test
    fun aMonthOfDaysCarriesTheDotsTheCalendarDraws() =
        runTest {
            val days = mocks().schedule.month(LocalDate(2026, 5, 1))

            assertEquals(31, days.size, "May has 31 days")
            assertTrue(days.any { it.hasInjection }, "the seeded protocol injects in May")
            assertTrue(days.none { it.date.month != LocalDate(2026, 5, 1).month })
        }

    @Test
    fun theDayDotsSayWhichDayIsWhich() =
        runTest {
            // Three of ScheduleDay's five fields had no assertion anywhere, and
            // dropping the emptiness guard from `allDone` painted «всё
            // выполнено» over the nine days before the protocol began.
            val m = mocks()
            val days = m.schedule.month(LocalDate(2026, 5, 1))
            val beforeTheCycle = days.first { it.date == LocalDate(2026, 5, 1) }
            val today = days.first { it.date == LocalDate(2026, 5, 31) }

            assertEquals(null, beforeTheCycle.cycleWeek)
            assertFalse(beforeTheCycle.allDone, "a day outside the protocol cannot be done")
            assertFalse(beforeTheCycle.anyPending)

            assertEquals(4, today.cycleWeek)
            assertTrue(today.anyPending, "today has occurrences nobody has logged")
        }

    @Test
    fun aSeriesIsOldestFirstAndKeepsTheMostRecentPoints() =
        runTest {
            // Ordered because a chart is drawn left to right, and truncated
            // from the *end*: `take` instead of `takeLast` would draw the
            // patient's first seven weeks forever.
            val series = mocks().measurements.series(Metric.WEIGHT, points = 3)

            assertEquals(3, series.points.size)
            assertEquals(series.points.sortedBy { it.measuredAt }, series.points)
            assertEquals(series.points.last(), series.latest)
            assertEquals(98.4, series.latest?.value)
        }

    @Test
    fun aMetricWithNoReadingsIsEmptyRatherThanAbsent() =
        runTest {
            // A blank chart beside a caption reads as «failed to load», so the
            // caller has to be able to tell «nothing measured» apart from an
            // error — which means not throwing.
            val series = mocks().measurements.series(Metric.CHEST)

            assertTrue(series.points.isEmpty())
            assertEquals(null, series.latest)
        }

    @Test
    fun theWeightSeriesOnTodayIsTheSameSevenPoints() =
        runTest {
            val m = mocks()

            assertEquals(
                m.measurements
                    .series(Metric.WEIGHT)
                    .points
                    .map { it.value },
                m.today.today().weightSeries,
            )
            // And it is seven, not «whatever the default happens to be»: the
            // seed holds eight readings, so the constant is load-bearing, and
            // comparing two calls that both use it proved nothing about it.
            assertEquals(
                MeasurementsRepository.DEFAULT_POINTS,
                m.today
                    .today()
                    .weightSeries.size,
            )
        }

    @Test
    fun theReorderHintReachesTheScreenPayload() =
        runTest {
            // The only thing wiring `dosesPerWeek()` to what a patient sees.
            // Nothing asserted `reorder` at all, so the hint could disappear
            // for every patient with the gate green.
            val hint = mocks().today.today().reorder

            assertEquals(MockSeed.semaglutide.id, hint?.compoundId)
            // One dose left of four, at one a week. The patient has taken
            // three Sundays; the hint fires because they are nearly out.
            assertEquals(1, hint?.weeksLeft, "one dose left at one a week")
        }

    @Test
    fun theLatestWeightIsTheLatestWeightAndNotTheLatestReading() =
        runTest {
            // The seed's most recent measurement is an HRV of 58. «Latest
            // reading wins» is per metric, and a lookup that took the last row
            // would put 58 kg on the Today screen.
            assertEquals(98.4, mocks().today.today().weightKg)
        }

    @Test
    fun daysBeforeTheProtocolBeganCarryNothing() =
        runTest {
            // The calendar is drawn for whole months, and the cycle starts on
            // the 10th — the first nine days must come back empty rather than
            // inheriting the protocol.
            val days = mocks().schedule.month(LocalDate(2026, 5, 1))

            assertTrue(days.take(9).none { it.hasInjection })
            assertTrue(days.drop(9).any { it.hasInjection })
        }

    @Test
    fun theDaySummaryCountsMealsAndTheirEnergy() =
        runTest {
            val summary = mocks().today.today()

            // Numbers, not «1 240 ккал» — the formatting lives on the UI side
            // and this is what it is handed.
            assertTrue(summary.mealCount > 0)
            assertTrue(summary.mealMacros.kcal > 0)
            assertTrue(summary.targets.kcal > summary.mealMacros.kcal)
        }

    @Test
    fun anotherDayHasItsOwnMealsAndNotTheSeededDays() =
        runTest {
            // A day the seed has no meals for. Without a date filter the app
            // reports a breakfast eaten three weeks earlier as eaten today,
            // and «сегодня» stops meaning anything.
            val summary = mocks("2026-06-07T09:00:00Z").today.today()

            assertEquals(0, summary.mealCount)
            assertEquals(0, summary.mealMacros.kcal)
        }
}

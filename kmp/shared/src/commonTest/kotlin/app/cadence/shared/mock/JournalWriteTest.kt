package app.cadence.shared.mock

import app.cadence.shared.domain.CheckInDraft
import app.cadence.shared.domain.DoseDraft
import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.domain.InjectionSite
import app.cadence.shared.domain.JournalSource
import app.cadence.shared.domain.ProtocolItemId
import app.cadence.shared.domain.SideEffect
import app.cadence.shared.domain.selectItem
import app.cadence.shared.repository.DoseLogResult
import app.cadence.shared.repository.JournalSaveResult
import kotlinx.coroutines.test.runTest
import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

private val ZONE = TimeZone.of("Europe/Moscow")

/** The seeded Sunday the dose wizard's own tests write against. */
private val DAY = LocalDate(2026, 5, 31)

private fun mocks() = CadenceMocks(clock = FixedCadenceClock.at("2026-05-31T09:00:00Z"), zone = ZONE)

/** An injection needs an item from the plan and a zone; the dose it carries is the phase's. */
private fun injectionDraft(itemId: ProtocolItemId) =
    DoseDraft().selectItem(MockSeed.plan, itemId, DAY).copy(site = InjectionSite.LEFT_THIGH)

private fun checkIn(
    mood: Int? = null,
    energy: Int? = null,
    sleep: Int? = null,
    tags: List<SideEffect> = emptyList(),
    note: String? = null,
) = CheckInDraft(entryDate = DAY, mood = mood, energy = energy, sleep = sleep, tags = tags, note = note)

class JournalWriteTest {
    @Test
    fun anEntryBornByHandStaysManualWhenADoseFollowsIt() =
        runTest {
            val m = mocks()

            assertIs<JournalSaveResult.Written>(m.journal.save(checkIn(mood = 4, energy = 3)))
            val dose = assertIs<DoseLogResult.Written>(m.dosing.submit(injectionDraft(MockSeed.semaItemId)))

            // Same day, or the two writes never met and this proves nothing.
            assertEquals(DAY, dose.journalDate)

            val entry = assertNotNull(m.journal.entry(DAY))

            // The mark «с дозой» means born of an injection, not written last.
            assertEquals(JournalSource.MANUAL, entry.source)
            assertEquals(4, entry.mood, "the morning's mood did not survive the injection")
        }

    @Test
    fun anEntryBornOfADoseStaysDoseWhenAHandEditFollowsIt() =
        runTest {
            val m = mocks()

            assertIs<DoseLogResult.Written>(
                m.dosing.submit(injectionDraft(MockSeed.semaItemId)),
            )
            assertIs<JournalSaveResult.Written>(m.journal.save(checkIn(sleep = 2)))

            // The other order, which breaks symmetrically: an evening edit by hand
            // would otherwise take «с дозой» off and the link to the injection with it.
            assertEquals(JournalSource.DOSE, assertNotNull(m.journal.entry(DAY)).source)
        }

    @Test
    fun aSecondCheckInUpdatesTheDaysEntryRatherThanAddingOne() =
        runTest {
            val m = mocks()

            m.journal.save(checkIn(mood = 2, tags = listOf(SideEffect.NAUSEA)))
            m.journal.save(checkIn(energy = 5, tags = listOf(SideEffect.HEADACHE)))

            val entry = assertNotNull(m.journal.entry(DAY))

            // §03 keys the entry UNIQUE(patient, date): the day is the identity.
            assertEquals(2, entry.mood, "the morning's mood was erased by an evening that did not mention it")
            assertEquals(5, entry.energy)
            assertEquals(listOf(SideEffect.NAUSEA, SideEffect.HEADACHE), entry.tags)
        }

    @Test
    fun aTagNamedTwiceIsCarriedOnce() =
        runTest {
            val m = mocks()

            m.journal.save(checkIn(tags = listOf(SideEffect.NAUSEA)))
            m.journal.save(checkIn(tags = listOf(SideEffect.NAUSEA, SideEffect.FATIGUE)))

            assertEquals(listOf(SideEffect.NAUSEA, SideEffect.FATIGUE), assertNotNull(m.journal.entry(DAY)).tags)
        }

    @Test
    fun aCheckInThatSaysNothingIsRefusedRatherThanWrittenEmpty() =
        runTest {
            val m = mocks()

            assertEquals(JournalSaveResult.Rejected.Empty, m.journal.save(checkIn()))

            // Refused, and nothing left behind: a written-then-empty entry would put a
            // day into the feed and the heatmap that the patient never filled in.
            assertTrue(m.journal.entry(DAY) == null, "an empty check-in still created an entry")
        }

    @Test
    fun aReadingOffTheScaleIsRefusedRatherThanStoredToReadBackAsNothing() =
        runTest {
            val m = mocks()

            // MoodLevel.of answers null outside 1..5, so a stored 7 comes back as «no
            // mood at all» — the loss is silent, which is why it is refused at the door.
            assertEquals(JournalSaveResult.Rejected.OffTheScale, m.journal.save(checkIn(mood = 7)))
            assertEquals(JournalSaveResult.Rejected.OffTheScale, m.journal.save(checkIn(energy = 0)))
            assertEquals(JournalSaveResult.Rejected.OffTheScale, m.journal.save(checkIn(sleep = -1)))

            // Every reading, not any: the sheet always sends a mood, so a bad energy
            // arrives beside a good mood and `all` collapsing to `any` would let it in.
            assertEquals(JournalSaveResult.Rejected.OffTheScale, m.journal.save(checkIn(mood = 3, energy = 9)))

            // Both edges of the range, or 1..5 widening to 1..6 or narrowing to 2..5
            // passes: neither 1 nor 6 is named above.
            assertEquals(JournalSaveResult.Rejected.OffTheScale, m.journal.save(checkIn(mood = 6)))
            assertTrue(m.journal.entry(DAY) == null, "a refused check-in still created an entry")

            assertIs<JournalSaveResult.Written>(m.journal.save(checkIn(mood = 1, sleep = 5)))
        }

    @Test
    fun aBlankNoteIsSilenceRatherThanAnAnswer() =
        runTest {
            val m = mocks()

            m.journal.save(checkIn(note = "тяжело"))
            m.journal.save(checkIn(mood = 4, note = "   "))

            // `saysNothing` treats a blank note as unnamed, so the merge has to agree:
            // read as a value it would erase the morning's, which is what step-6's own
            // trimming makes easy to send.
            assertEquals("тяжело", assertNotNull(m.journal.entry(DAY)).note)
        }

    @Test
    fun anInjectionWithNoContextStillWritesItsDay() =
        runTest {
            val m = mocks()

            // The guards belong to the hand-written door: an injection is a fact on its
            // own, so it writes a day whose fields are all empty. Deliberate, and pinned
            // so the next reader does not mistake it for the guards leaking.
            assertIs<DoseLogResult.Written>(m.dosing.submit(injectionDraft(MockSeed.semaItemId)))

            val entry = assertNotNull(m.journal.entry(DAY))

            assertEquals(JournalSource.DOSE, entry.source)
            assertNull(entry.mood)
            assertEquals(emptyList(), entry.tags)
        }

    @Test
    fun theWrittenAnswerCarriesTheEntryItWrote() =
        runTest {
            val m = mocks()

            val written = assertIs<JournalSaveResult.Written>(m.journal.save(checkIn(mood = 5, note = "ровно")))

            // The screen reads what was stored, not what it sent: the merge is where the
            // two differ, and a result that echoed the draft would hide it.
            assertEquals(5, written.entry.mood)
            assertEquals("ровно", written.entry.note)
            assertEquals(JournalSource.MANUAL, written.entry.source)
        }
}

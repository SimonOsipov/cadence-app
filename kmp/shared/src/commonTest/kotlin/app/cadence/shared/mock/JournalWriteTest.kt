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
            assertIs<DoseLogResult.Written>(
                m.dosing.submit(injectionDraft(MockSeed.semaItemId)),
            )

            // The mark «с дозой» means «this entry was born of an injection», not
            // «an injection wrote last».
            assertEquals(JournalSource.MANUAL, assertNotNull(m.journal.entry(DAY)).source)
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

            assertTrue(m.journal.entry(DAY) == null)
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

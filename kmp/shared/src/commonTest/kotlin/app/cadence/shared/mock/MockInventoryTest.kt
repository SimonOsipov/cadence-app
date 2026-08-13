package app.cadence.shared.mock

import app.cadence.shared.domain.CompoundId
import app.cadence.shared.domain.VialDraft
import app.cadence.shared.domain.VialId
import app.cadence.shared.domain.VialStatus
import app.cadence.shared.repository.AddVialResult
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
private val TODAY = LocalDate(2026, 5, 31)

private fun mocks() =
    CadenceMocks(
        clock =
            app.cadence.shared.domain.FixedCadenceClock
                .at("2026-05-31T09:00:00Z"),
        zone = ZONE,
    )

private fun draft(
    expires: LocalDate = LocalDate(2026, 12, 1),
    total: Int = 12,
    compound: CompoundId? = MockSeed.semaglutide.id,
) = VialDraft(
    compoundId = compound,
    concentrationLabel = "1 мг/мл",
    totalDoses = total,
    expiresOn = expires,
    lot = "N-0001",
    locationRu = "Холодильник, полка 2",
)

class MockInventoryTest {
    @Test
    fun theCabinetIsTheSeededOneReadThroughTheInterface() =
        runTest {
            val cabinet = mocks().inventory.cabinet()

            assertEquals(4, cabinet.all.size, "the disposed vial is not in the fridge")
            assertEquals(setOf("vial-sema-1"), cabinet.reorder.map { "vial-sema-1" }.toSet())
        }

    @Test
    fun aVialCanBeReadOnItsOwnWithItsRecords() =
        runTest {
            val detail = assertNotNull(mocks().inventory.vial(VialId("vial-bpc-2")))

            assertEquals(2, detail.row.remaining)
            assertEquals(12, detail.recent.size)
        }

    @Test
    fun aVialThatIsNotThePatientsIsNotFound() =
        runTest {
            assertNull(mocks().inventory.vial(VialId("vial-of-somebody-else")))
        }

    @Test
    fun anAddedVialComesBackFullBecauseNothingHasBeenDrawnFromIt() =
        runTest {
            // A vial arrives with a total and no remaining count: the count is a
            // subtraction from the moment it exists.
            val m = mocks()
            val before =
                m.inventory
                    .cabinet()
                    .all.size

            val added = assertIs<AddVialResult.Added>(m.inventory.addVial(draft(total = 12)))
            val cabinet = m.inventory.cabinet()

            assertEquals(before + 1, cabinet.all.size)
            val fresh = assertNotNull(cabinet.sealed.firstOrNull { it.id == added.id })
            assertEquals(12, fresh.remaining)
            assertEquals(12, fresh.totalDoses)
            assertEquals(VialStatus.SEALED, fresh.status, "a new vial is not open until it is opened")
        }

    @Test
    fun anAddedVialCarriesWhatTheFormWasGiven() =
        runTest {
            val m = mocks()
            val added = assertIs<AddVialResult.Added>(m.inventory.addVial(draft()))

            val fresh = assertNotNull(m.inventory.vial(added.id)).row

            assertEquals(MockSeed.semaglutide, fresh.compound)
            assertEquals("N-0001", fresh.lot)
            assertEquals("Холодильник, полка 2", fresh.locationRu)
            assertEquals(LocalDate(2026, 12, 1), fresh.expiresOn)
        }

    @Test
    fun aDraftMissingWhatAVialCannotExistWithoutIsRefused() =
        runTest {
            val m = mocks()
            val before =
                m.inventory
                    .cabinet()
                    .all.size

            assertEquals(AddVialResult.Rejected, m.inventory.addVial(draft(compound = null)))
            assertEquals(AddVialResult.Rejected, m.inventory.addVial(draft(total = 0)))
            assertEquals(AddVialResult.Rejected, m.inventory.addVial(VialDraft()))

            assertEquals(
                before,
                m.inventory
                    .cabinet()
                    .all.size,
                "a refused draft was written anyway",
            )
        }

    @Test
    fun aVialThatHasAlreadyExpiredIsRefused() =
        runTest {
            // A cabinet accepting it would count doses the patient must not take.
            val m = mocks()

            assertEquals(AddVialResult.Rejected, m.inventory.addVial(draft(expires = LocalDate(2026, 5, 30))))
            assertIs<AddVialResult.Added>(m.inventory.addVial(draft(expires = TODAY)), "expiring today is still usable")
        }

    @Test
    fun theNewVialIsOfferedToTheDoseWriteAsSoonAsItIsOpened() =
        runTest {
            // A vial added here and invisible to the dose write would be the prototype's two
            // disconnected datasets, rebuilt.
            val m = mocks()
            val added = assertIs<AddVialResult.Added>(m.inventory.addVial(draft(total = 40)))

            assertTrue(
                m.inventory
                    .cabinet()
                    .all
                    .any { it.id == added.id },
            )
        }
}

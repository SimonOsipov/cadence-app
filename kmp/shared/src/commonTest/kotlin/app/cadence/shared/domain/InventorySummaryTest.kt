package app.cadence.shared.domain

import app.cadence.shared.mock.MockSeed
import kotlinx.datetime.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

private val TODAY = LocalDate(2026, 5, 31)

private fun summary(
    vials: List<Vial> = MockSeed.vials,
    events: List<DoseEvent> = MockSeed.history,
    today: LocalDate = TODAY,
) = inventorySummary(MockSeed.plan, vials, events, today, MockSeed.compounds)

private fun ids(rows: List<VialRow>) = rows.map { it.id.raw }.toSet()

class InventorySummaryTest {
    @Test
    fun theGroupsAreTheProtototypesAndTheyOverlap() {
        // Named, deliberately not disjoint: the prototype puts an opened expiring vial in
        // both «Активные» and «Истекают».
        val sum = summary()

        assertEquals(setOf("vial-bpc-2", "vial-bpc-3", "vial-sema-1"), ids(sum.active))
        assertEquals(setOf("vial-bpc-4"), ids(sum.sealed))
        assertEquals(setOf("vial-bpc-3"), ids(sum.expiring))
        assertEquals(setOf("vial-bpc-2"), ids(sum.low))

        assertTrue("vial-bpc-3" in ids(sum.active) && "vial-bpc-3" in ids(sum.expiring))
    }

    @Test
    fun aDisposedVialIsInNoGroupAtAll() {
        val sum = summary()

        assertTrue(
            listOf(sum.active, sum.sealed, sum.expiring, sum.low).none { "vial-bpc-1" in ids(it) },
            "a thrown-away vial is still in the cabinet",
        )
    }

    @Test
    fun anUnopenedVialAboutToExpireIsSealedAndExpiringAtOnce() {
        val soon = MockSeed.vials.first { it.id == VialId("vial-bpc-4") }.copy(expiresOn = LocalDate(2026, 6, 5))
        val sum = summary(vials = MockSeed.vials.map { if (it.id == soon.id) soon else it })

        assertTrue("vial-bpc-4" in ids(sum.sealed))
        assertTrue("vial-bpc-4" in ids(sum.expiring))
    }

    @Test
    fun theReorderHintFiresForTheCompoundWithNoSpareAndForNoOther() {
        // BPC-157 would also read low on stock, but it has a spare and §03 requires both
        // conditions.
        val sum = summary()

        assertEquals(listOf(MockSeed.semaglutide.id), sum.reorder.map { it.compoundId })
        assertEquals(1, sum.reorder.single().weeksLeft)
    }

    @Test
    fun aCompoundTheProtocolDoesNotPrescribeIsNotCounted() {
        val stray =
            MockSeed.vials.first().copy(
                id = VialId("vial-stray"),
                compoundId = CompoundId("nothing-prescribed"),
                openedAt = TODAY,
                disposedAt = null,
            )
        val sum = summary(vials = MockSeed.vials + stray)

        assertEquals(listOf(MockSeed.semaglutide.id), sum.reorder.map { it.compoundId })
        assertTrue("vial-stray" in ids(sum.active), "and it is still in the cabinet")
    }

    @Test
    fun theReorderHintIsSilentAboveFourWeeksAndSpeaksAtFour() {
        // The seed's semaglutide fires at any threshold, so the boundary needs stock the
        // seed doesn't have: nine less three taken is six weeks (quiet); seven less three is
        // four, and the condition is "≤ 4".
        fun weeksOf(total: Int) =
            summary(
                vials = MockSeed.vials.map { if (it.id == VialId("vial-sema-1")) it.copy(totalDoses = total) else it },
            ).reorder.singleOrNull { it.compoundId == MockSeed.semaglutide.id }?.weeksLeft

        assertEquals(null, weeksOf(total = 9), "six weeks of supply is not a reason to reorder")
        assertEquals(4, weeksOf(total = 7))
        assertEquals(1, weeksOf(total = 4))
    }

    @Test
    fun aVialIsExpiringInsideAFortnightAndNotBefore() {
        // The seed's two dates (ten days, five months) sit nowhere near this boundary.
        fun expiringOn(day: Int) =
            summary(
                vials =
                    MockSeed.vials.map {
                        if (it.id == VialId("vial-bpc-4")) it.copy(expiresOn = LocalDate(2026, 6, day)) else it
                    },
            ).expiring.map { it.id.raw }

        assertTrue("vial-bpc-4" !in expiringOn(day = 20), "twenty days out is not expiring")
        assertTrue("vial-bpc-4" in expiringOn(day = 14), "a fortnight out is")
    }

    @Test
    fun oneCompoundGetsOneHintEvenWhenTheProtocolPrescribesItTwice() {
        // A compound taken morning and evening is two items; asking per item without
        // collapsing them tells the patient to reorder the same compound twice.
        val twice =
            MockSeed.plan.items
                .first { it.id == MockSeed.semaItemId }
                .copy(id = ProtocolItemId("item-sema-evening"))
        val sum =
            inventorySummary(
                MockSeed.plan.copy(items = MockSeed.plan.items + twice),
                MockSeed.vials,
                MockSeed.history,
                TODAY,
                MockSeed.compounds,
            )

        assertEquals(listOf(MockSeed.semaglutide.id), sum.reorder.map { it.compoundId })
    }

    @Test
    fun anEmptyCabinetHasEmptyGroupsRatherThanAnException() {
        val sum = summary(vials = emptyList(), events = emptyList())

        assertEquals(emptyList(), sum.active)
        assertEquals(emptyList(), sum.sealed)
        assertEquals(emptyList(), sum.expiring)
        assertEquals(emptyList(), sum.low)
        assertEquals(emptyList(), sum.reorder)
    }

    @Test
    fun theGroupsFollowTheDayTheyAreAskedAbout() {
        // Two months on, the sealed spare is past its date: the answer moves because the
        // question carries a day, not because anything was re-stored.
        val sum = summary(today = LocalDate(2026, 10, 10))

        assertTrue("vial-bpc-4" in ids(sum.expiring), "the October vial is not flagged in October")
    }
}

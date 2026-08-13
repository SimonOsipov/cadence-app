package app.cadence.shared.domain

import kotlinx.datetime.DatePeriod
import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime
import kotlinx.datetime.minus
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.time.Instant

private val TODAY = LocalDate(2026, 5, 31)
private val PATIENT = UserId("p-1")
private val SEMA = CompoundId("sema")
private var nextId = 0

private fun vial(
    totalDoses: Int = 4,
    openedAt: LocalDate? = null,
    expiresOn: LocalDate = LocalDate(2026, 12, 31),
) = Vial(
    id = VialId("v-${nextId++}"),
    patientId = PATIENT,
    compoundId = SEMA,
    concentrationLabel = "1 мг/мл",
    totalDoses = totalDoses,
    openedAt = openedAt,
    expiresOn = expiresOn,
    lot = null,
    locationRu = null,
    disposedAt = null,
    labelPhotoPath = null,
)

/** An item whose cadence gives the weekly rate each case needs. */
private fun weeklyItem(
    compound: CompoundId = SEMA,
    daysAWeek: Int = 1,
) = ProtocolItem(
    id = ProtocolItemId("item-${compound.raw}"),
    protocolId = ProtocolId("pr"),
    kind = ProtocolItemKind.INJECTION,
    compoundId = compound,
    cadence = ProtocolCadence.WEEKLY,
    daysOfWeek = DayOfWeek.entries.take(daysAWeek),
    times = listOf(LocalTime(7, 0)),
    loggable = true,
)

private fun doseFrom(v: Vial) =
    DoseEvent(
        id = DoseEventId("e-${nextId++}"),
        patientId = PATIENT,
        protocolItemId = ProtocolItemId("sema"),
        vialId = v.id,
        scheduledForDate = TODAY,
        scheduledForTime = null,
        injectedAt = Instant.parse("2026-05-31T07:00:00Z"),
        dose = Dose(0.25, DoseUnit.MG),
        site = null,
        mood = null,
        sideEffects = emptyList(),
        note = null,
        photoPath = null,
        createdAt = null,
    )

class InventoryMathTest {
    @Test
    fun loggingADoseDecrementsTheVialItCameFrom() {
        val v = vial(totalDoses = 4)

        assertEquals(4, remainingDoses(v, events = emptyList()))
        assertEquals(2, remainingDoses(v, events = List(2) { doseFrom(v) }))
        assertEquals(0, remainingDoses(v, events = List(4) { doseFrom(v) }))
    }

    @Test
    fun eventsFromOtherVialsDoNotCount() {
        // A subtraction ignoring `vialId` would drain both at once.
        val mine = vial(totalDoses = 4)
        val other = vial(totalDoses = 4)

        assertEquals(4, remainingDoses(mine, events = List(3) { doseFrom(other) }))
    }

    @Test
    fun remainingNeverGoesNegative() {
        val v = vial(totalDoses = 2)

        assertEquals(0, remainingDoses(v, events = List(5) { doseFrom(v) }))
    }

    @Test
    fun anUnopenedVialIsSealedAndAnOpenedOneIsActive() {
        assertEquals(VialStatus.SEALED, vialStatus(vial(openedAt = null), emptyList(), TODAY))
        assertEquals(
            VialStatus.ACTIVE,
            vialStatus(vial(openedAt = LocalDate(2026, 5, 1)), emptyList(), TODAY),
        )
    }

    @Test
    fun aDisposedVialSaysSoWhateverElseIsTrueOfIt() {
        val v = vial(openedAt = LocalDate(2026, 5, 1)).copy(disposedAt = LocalDate(2026, 5, 20))

        assertEquals(VialStatus.DISPOSED, vialStatus(v, emptyList(), TODAY))
    }

    @Test
    fun aVialBelowAQuarterIsLow() {
        // §03's «low <25%», strict: exactly a quarter is not low. Eight doses, not four:
        // four's remainders land on 25% and 0% with nothing in between.
        val v = vial(totalDoses = 8, openedAt = LocalDate(2026, 5, 1))

        assertEquals(VialStatus.ACTIVE, vialStatus(v, List(5) { doseFrom(v) }, TODAY), "three of eight")
        assertEquals(VialStatus.ACTIVE, vialStatus(v, List(6) { doseFrom(v) }, TODAY), "exactly a quarter")
        assertEquals(VialStatus.LOW, vialStatus(v, List(7) { doseFrom(v) }, TODAY), "one of eight")
    }

    @Test
    fun expiryOutranksLowStock() {
        val soon = vial(totalDoses = 8, openedAt = LocalDate(2026, 5, 1), expiresOn = LocalDate(2026, 6, 10))
        val later = vial(totalDoses = 8, openedAt = LocalDate(2026, 5, 1), expiresOn = LocalDate(2026, 7, 20))

        assertEquals(VialStatus.EXPIRING, vialStatus(soon, List(7) { doseFrom(soon) }, TODAY))
        assertEquals(VialStatus.LOW, vialStatus(later, List(7) { doseFrom(later) }, TODAY))
    }

    @Test
    fun aSealedVialAboutToExpireSaysSo() {
        // Measured: it read SEALED. Unopened stock about to be wasted is exactly the vial
        // worth warning about.
        val v = vial(openedAt = null, expiresOn = LocalDate(2026, 6, 3))

        assertEquals(VialStatus.EXPIRING, vialStatus(v, emptyList(), TODAY))
    }

    @Test
    fun aHintIsAboutOneCompoundAndCountsOnlyItsVials() {
        // Measured: an unopened BPC vial once suppressed the semaglutide hint entirely.
        val sema = vial(totalDoses = 4, openedAt = LocalDate(2026, 5, 1))
        val bpcSpare =
            vial(totalDoses = 30).copy(id = VialId("v-bpc"), compoundId = CompoundId("bpc"))

        val alone = reorderHint(weeklyItem(), listOf(sema), emptyList(), TODAY)
        val withOtherCompound = reorderHint(weeklyItem(), listOf(sema, bpcSpare), emptyList(), TODAY)

        assertEquals(alone, withOtherCompound, "a vial of another compound changed the answer")
        assertEquals(SEMA, alone?.compoundId)
    }

    @Test
    fun theReorderHintNeedsBothNoSparesAndLittleSupply() {
        // §03: «0 sealed spares & ≤4 weeks supply». Either alone is not a
        // reason to tell a patient to order more.
        val open = vial(totalDoses = 4, openedAt = LocalDate(2026, 5, 1))
        // A *small* spare, deliberately: a full one would put total supply over four weeks
        // anyway, passing without the spare check ever running.
        val spare = vial(totalDoses = 1)

        assertNull(
            reorderHint(weeklyItem(), listOf(open, spare), List(3) { doseFrom(open) }, TODAY),
            "a sealed spare is exactly what makes reordering unnecessary",
        )
        assertNull(
            reorderHint(weeklyItem(daysAWeek = 0), listOf(open), emptyList(), TODAY),
            "four doses at one every five weeks is twenty weeks of supply",
        )

        val hint = reorderHint(weeklyItem(), listOf(open), List(1) { doseFrom(open) }, TODAY)

        assertEquals(3, hint?.weeksLeft)
        assertEquals(SEMA, hint?.compoundId)
    }

    @Test
    fun anExpiredVialIsNeitherASpareNorSupply() {
        // Measured: an expired sealed vial made `hasSealedSpare` true, so a patient two
        // doses from nothing saw no reorder card at all.
        val open = vial(totalDoses = 4, openedAt = LocalDate(2026, 5, 1))
        val dead = vial(totalDoses = 4, expiresOn = TODAY.minus(DatePeriod(days = 1)))

        val hint = reorderHint(weeklyItem(), listOf(open, dead), List(2) { doseFrom(open) }, TODAY)

        assertEquals(2, hint?.weeksLeft, "the expired vial is not two more weeks of supply")

        // Boundary: a vial expiring *today* is still usable and still suppresses the hint.
        assertNull(
            reorderHint(
                weeklyItem(),
                listOf(open, vial(totalDoses = 4, expiresOn = TODAY)),
                List(2) { doseFrom(open) },
                TODAY,
            ),
            "stock that expires today has not expired yet",
        )
    }

    @Test
    fun aDisposedVialIsNeitherASpareNorSupply() {
        val open = vial(totalDoses = 4, openedAt = LocalDate(2026, 5, 1))
        val binned = vial(totalDoses = 4).copy(disposedAt = LocalDate(2026, 5, 2))

        val hint = reorderHint(weeklyItem(), listOf(open, binned), List(1) { doseFrom(open) }, TODAY)

        assertEquals(3, hint?.weeksLeft)
    }
}

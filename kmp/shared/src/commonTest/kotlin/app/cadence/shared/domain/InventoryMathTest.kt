package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
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
        // The prototype's central inventory bug: logging never touched stock.
        // There is no stored counter to go wrong here — the count *is* the
        // subtraction, every time it is asked for.
        val v = vial(totalDoses = 4)

        assertEquals(4, remainingDoses(v, events = emptyList()))
        assertEquals(2, remainingDoses(v, events = List(2) { doseFrom(v) }))
        assertEquals(0, remainingDoses(v, events = List(4) { doseFrom(v) }))
    }

    @Test
    fun eventsFromOtherVialsDoNotCount() {
        // Two vials of one compound is the ordinary case, and a subtraction
        // that ignored `vialId` would drain both at once.
        val mine = vial(totalDoses = 4)
        val other = vial(totalDoses = 4)

        assertEquals(4, remainingDoses(mine, events = List(3) { doseFrom(other) }))
    }

    @Test
    fun remainingNeverGoesNegative() {
        // More events than doses is data corruption, not a number to render.
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
        // §03's «low <25%», read strictly: a quarter left is not low, less
        // than a quarter is. Eight doses rather than four because four cannot
        // express the boundary — its remainders land on 25% and 0% with
        // nothing in between, which is what the first draft of this test got
        // wrong.
        val v = vial(totalDoses = 8, openedAt = LocalDate(2026, 5, 1))

        assertEquals(VialStatus.ACTIVE, vialStatus(v, List(5) { doseFrom(v) }, TODAY), "three of eight")
        assertEquals(VialStatus.ACTIVE, vialStatus(v, List(6) { doseFrom(v) }, TODAY), "exactly a quarter")
        assertEquals(VialStatus.LOW, vialStatus(v, List(7) { doseFrom(v) }, TODAY), "one of eight")
    }

    @Test
    fun expiryOutranksLowStock() {
        // «expiring ≤14 d». A vial that is both low and expiring reads as
        // expiring, because that is the one with a deadline attached.
        val soon = vial(totalDoses = 8, openedAt = LocalDate(2026, 5, 1), expiresOn = LocalDate(2026, 6, 10))
        val later = vial(totalDoses = 8, openedAt = LocalDate(2026, 5, 1), expiresOn = LocalDate(2026, 7, 20))

        assertEquals(VialStatus.EXPIRING, vialStatus(soon, List(7) { doseFrom(soon) }, TODAY))
        assertEquals(VialStatus.LOW, vialStatus(later, List(7) { doseFrom(later) }, TODAY))
    }

    @Test
    fun theReorderHintNeedsBothNoSparesAndLittleSupply() {
        // §03: «0 sealed spares & ≤4 weeks supply». Either alone is not a
        // reason to tell a patient to order more.
        val open = vial(totalDoses = 4, openedAt = LocalDate(2026, 5, 1))
        // A *small* spare, deliberately. With a full one the total supply is
        // over four weeks anyway and the assertion passes without the spare
        // check ever running — which is how the first version of this test let
        // a mutation through: it asserted the right answer for the wrong
        // reason.
        val spare = vial(totalDoses = 1)

        assertNull(
            reorderHint(listOf(open, spare), List(3) { doseFrom(open) }, dosesPerWeek = 1.0),
            "a sealed spare is exactly what makes reordering unnecessary",
        )
        assertNull(
            reorderHint(listOf(open), emptyList(), dosesPerWeek = 0.2),
            "four doses at one every five weeks is twenty weeks of supply",
        )

        val hint = reorderHint(listOf(open), List(1) { doseFrom(open) }, dosesPerWeek = 1.0)

        assertEquals(3, hint?.weeksLeft)
        assertEquals(SEMA, hint?.compoundId)
    }

    @Test
    fun aDisposedVialIsNeitherASpareNorSupply() {
        val open = vial(totalDoses = 4, openedAt = LocalDate(2026, 5, 1))
        val binned = vial(totalDoses = 4).copy(disposedAt = LocalDate(2026, 5, 2))

        // The binned vial must not count as the sealed spare that suppresses
        // the hint, and its doses must not count as supply.
        val hint = reorderHint(listOf(open, binned), List(1) { doseFrom(open) }, dosesPerWeek = 1.0)

        assertEquals(3, hint?.weeksLeft)
    }
}

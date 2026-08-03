package app.cadence.shared.domain

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class DomainTest {
    @Test
    fun aDoseKeepsItsValueAndUnitApart() {
        // §03: «Mobile's structured {value, unit} wins over the dashboard's
        // display string». The whole correction is that nothing downstream can
        // be handed «0,25 мг» and have to parse it back out.
        val dose = Dose(value = 0.25, unit = DoseUnit.MG)

        assertEquals(0.25, dose.value)
        assertEquals(DoseUnit.MG, dose.unit)
    }

    @Test
    fun theTenInjectionSitesAreAllThere() {
        // §03 names ten zones. The wizard's rotation suggestion is meaningless
        // if the set is short.
        assertEquals(10, InjectionSite.entries.size)
    }

    @Test
    fun theSevenSideEffectsAreTheOnesTheProtocolTracks() {
        assertEquals(
            setOf("nausea", "fatigue", "headache", "bloating", "insomnia", "site", "appetite"),
            SideEffect.entries.map { it.code }.toSet(),
        )
    }

    @Test
    fun theEightMetricsCoverBothSurfaces() {
        // Mobile shows all eight, the dashboard a subset — §03's «same rows,
        // different projections». A missing metric is a screen that cannot be
        // built.
        assertEquals(
            setOf("weight", "hrv", "rhr", "sleep", "bodyfat", "waist", "hip", "chest"),
            Metric.entries.map { it.code }.toSet(),
        )
    }

    @Test
    fun aPatientHasOneTargetWeight() {
        // §03's first correction: the prototype carries 100 kg in trends and
        // 102 kg in body. There is one field, so there is one answer.
        val profile =
            PatientProfile(
                userId = UserId("p-1"),
                dateOfBirth = null,
                sex = null,
                heightCm = 188,
                targetWeightKg = 92.0,
                joinedAt = null,
            )

        assertEquals(92.0, profile.targetWeightKg)
    }

    @Test
    fun aProtocolPhaseKnowsWhichWeeksItCovers() {
        val phase = ProtocolPhase(fromWeek = 1, toWeek = 4, dose = Dose(0.25, DoseUnit.MG))

        assertTrue(phase.covers(week = 1), "the first week of the band is in it")
        assertTrue(phase.covers(week = 4), "and so is the last")
        assertFalse(phase.covers(week = 5))
        assertFalse(phase.covers(week = 0))
    }

    @Test
    fun theSevenJournalTagsAreFixed() {
        // §03: «tags[] (7 fixed)». An open set would let a client invent one.
        assertEquals(7, JournalTag.entries.size)
    }
}

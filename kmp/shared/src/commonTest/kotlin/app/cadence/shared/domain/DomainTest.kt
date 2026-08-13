package app.cadence.shared.domain

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class DomainTest {
    @Test
    fun aDoseKeepsItsValueAndUnitApart() {
        // §03: mobile's structured {value, unit} wins over the dashboard's display string.
        val dose = Dose(value = 0.25, unit = DoseUnit.MG)

        assertEquals(0.25, dose.value)
        assertEquals(DoseUnit.MG, dose.unit)
    }

    @Test
    fun theTenInjectionSitesAreThePrototypesOwn() {
        // Codes, not a count: `entries.size == 10` was green while two of the ten were
        // invented ("l-flank", "r-flank"), matching no zone the body map draws.
        assertEquals(
            setOf(
                "r-delt",
                "l-delt",
                "r-abdomen",
                "l-abdomen",
                "r-thigh",
                "l-thigh",
                "l-lback",
                "r-lback",
                "l-glute",
                "r-glute",
            ),
            InjectionSite.entries.map { it.code }.toSet(),
        )
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
        assertEquals(
            setOf("weight", "hrv", "rhr", "sleep", "bodyfat", "waist", "hip", "chest"),
            Metric.entries.map { it.code }.toSet(),
        )
    }

    @Test
    fun aPatientHasOneTargetWeight() {
        // §03's first correction: the prototype carries 100kg in trends and 102kg in body.
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
    fun theJournalTagsAreTheSideEffectsUnderAnotherName() {
        // §03's "one action, two facts" needs the two vocabularies to be the same one;
        // counting to seven wouldn't notice an enum invented from scratch.
        assertEquals(
            SideEffect.entries.map { it.code }.toSet(),
            JournalTag.entries.map { it.code }.toSet(),
        )
    }
}

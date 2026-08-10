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
    fun theTenInjectionSitesAreThePrototypesOwn() {
        // Codes, not a count. This test asserted `entries.size == 10` and was
        // green while two of the ten were invented — «l-flank» and «r-flank»,
        // which match no zone the body map draws. The same file was already
        // comparing code sets for SideEffect and Metric; the pattern was
        // applied selectively and the gap is exactly where the drift landed.
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
    fun theJournalTagsAreTheSideEffectsUnderAnotherName() {
        // §03: one dose-wizard check-in writes both a dose event and a journal
        // entry — «one action, two facts». That is only possible if the two
        // vocabularies are the same one, and the prototype's own comment says
        // so: «tags — side-effect ids». Counting to seven did not notice that
        // this enum had been invented from scratch.
        assertEquals(
            SideEffect.entries.map { it.code }.toSet(),
            JournalTag.entries.map { it.code }.toSet(),
        )
    }
}

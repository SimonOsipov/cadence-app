package app.cadence.shared.domain

import kotlinx.datetime.DatePeriod
import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime
import kotlinx.datetime.minus
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

private val PATIENT = UserId("p-1")
private val SEMA = ProtocolItemId("item-sema")
private val BPC = ProtocolItemId("item-bpc")

/** Loggable, but the protocol prescribes no dose band for it. */
private val VITAMIN = ProtocolItemId("item-vitamin")

/** Not loggable at all — the wizard must refuse it. */
private val WEIGH_IN = ProtocolItemId("item-weigh-in")

/** Week 1 runs from the 4th; week 5 opens on 1 June. */
private val START = LocalDate(2026, 5, 4)
private val IN_WEEK_1 = LocalDate(2026, 5, 6)
private val IN_WEEK_5 = LocalDate(2026, 6, 3)

private fun item(
    id: ProtocolItemId,
    kind: ProtocolItemKind = ProtocolItemKind.INJECTION,
) = ProtocolItem(
    id = id,
    protocolId = ProtocolId("pr-1"),
    kind = kind,
    compoundId = CompoundId(id.raw),
    cadence = ProtocolCadence.WEEKLY,
    daysOfWeek = listOf(DayOfWeek.WEDNESDAY),
    times = listOf(LocalTime(7, 0)),
    loggable = kind != ProtocolItemKind.WEIGH_IN,
)

private fun plan(status: ProtocolStatus = ProtocolStatus.ACTIVE) =
    ProtocolPlan(
        protocol =
            Protocol(
                id = ProtocolId("pr-1"),
                patientId = PATIENT,
                startDate = START,
                weeks = 12,
                status = status,
                createdBy = null,
                notes = null,
            ),
        items =
            listOf(
                item(SEMA),
                item(BPC),
                item(VITAMIN, ProtocolItemKind.SUPPLEMENT),
                item(WEIGH_IN, ProtocolItemKind.WEIGH_IN),
            ),
        phases =
            mapOf(
                // The titration band: "the current phase" differs in week 1 vs week 5.
                SEMA to
                    listOf(
                        ProtocolPhase(1, 4, Dose(0.25, DoseUnit.MG)),
                        ProtocolPhase(5, 8, Dose(0.5, DoseUnit.MG)),
                    ),
                BPC to listOf(ProtocolPhase(1, 12, Dose(250.0, DoseUnit.MCG))),
                // VITAMIN and WEIGH_IN have no phases: items with nothing to dose.
            ),
    )

/** A draft mid-wizard, every field filled, so a reset is visible. */
private fun filledDraft() =
    DoseDraft()
        .selectItem(plan(), SEMA, IN_WEEK_1)
        .copy(
            site = InjectionSite.LEFT_THIGH,
            mood = 4,
            sideEffects = listOf(SideEffect.NAUSEA),
            note = "чуть тянет",
            photoAttached = true,
        )

class DoseDraftTest {
    @Test
    fun theWizardIsThePrototypesFiveStepsInItsOrder() {
        // Not a count: a step invented or dropped has to be visible here.
        assertEquals(
            listOf(
                DoseStep.COMPOUND,
                DoseStep.DOSE,
                DoseStep.SITE,
                DoseStep.CONTEXT,
                DoseStep.REVIEW,
            ),
            DoseStep.entries.toList(),
        )
    }

    @Test
    fun anEmptyDraftCannotLeaveTheCompoundStep() {
        assertFalse(DoseDraft().canAdvance(DoseStep.COMPOUND))
    }

    @Test
    fun choosingAnItemOpensTheCompoundStepAndIsTheItemThatGetsLogged() {
        val draft = DoseDraft().selectItem(plan(), SEMA, IN_WEEK_1)

        // The id itself, not merely "something was chosen": the write reads this field.
        assertEquals(SEMA, draft.itemId)
        assertEquals(ProtocolItemKind.INJECTION, draft.kind)
        assertTrue(draft.canAdvance(DoseStep.COMPOUND))
    }

    @Test
    fun anItemTheProtocolDoesNotMarkLoggableIsNotAChoice() {
        val chosen = DoseDraft().selectItem(plan(), SEMA, IN_WEEK_1)

        assertEquals(DoseDraft(), DoseDraft().selectItem(plan(), WEIGH_IN, IN_WEEK_1))
        assertEquals(chosen, chosen.selectItem(plan(), WEIGH_IN, IN_WEEK_1))
    }

    @Test
    fun anItemTheProtocolDoesNotContainIsNotAChoice() {
        assertEquals(DoseDraft(), DoseDraft().selectItem(plan(), ProtocolItemId("nothing"), IN_WEEK_1))
    }

    @Test
    fun aDraftWithNoDoseCannotLeaveTheDoseStep() {
        // A loggable item with no prescribed band: a wizard assuming every item doses would
        // walk straight past.
        val draft = DoseDraft().selectItem(plan(), VITAMIN, IN_WEEK_1)

        assertEquals(VITAMIN, draft.itemId)
        assertNull(draft.dose)
        assertFalse(draft.canAdvance(DoseStep.DOSE))
    }

    @Test
    fun aZeroDoseCannotLeaveTheDoseStep() {
        // The stepper clamps at zero, so this is reachable, not a hypothesis: a null-only
        // guard would let it decrement a vial for an injection of nothing.
        val draft = DoseDraft().selectItem(plan(), SEMA, IN_WEEK_1)

        assertFalse(draft.copy(dose = Dose(0.0, DoseUnit.MG)).canAdvance(DoseStep.DOSE))
        assertTrue(draft.copy(dose = Dose(0.05, DoseUnit.MG)).canAdvance(DoseStep.DOSE))
    }

    @Test
    fun aDoseOpensTheDoseStep() {
        assertTrue(DoseDraft().selectItem(plan(), SEMA, IN_WEEK_1).canAdvance(DoseStep.DOSE))
    }

    @Test
    fun anInjectionWithNoSiteCannotLeaveTheSiteStep() {
        val draft = DoseDraft().selectItem(plan(), SEMA, IN_WEEK_1)

        assertFalse(draft.canAdvance(DoseStep.SITE))
        assertTrue(draft.copy(site = InjectionSite.LEFT_GLUTE).canAdvance(DoseStep.SITE))
    }

    @Test
    fun anItemThatIsNotAnInjectionNeedsNoSite() {
        val draft = DoseDraft().selectItem(plan(), VITAMIN, IN_WEEK_1)

        assertNull(draft.site)
        assertTrue(draft.canAdvance(DoseStep.SITE))
    }

    @Test
    fun theContextStepIsOptionalAndAdvancesWithNothingFilled() {
        val draft = DoseDraft().selectItem(plan(), SEMA, IN_WEEK_1).copy(site = InjectionSite.LEFT_GLUTE)

        assertNull(draft.mood)
        assertEquals(emptyList(), draft.sideEffects)
        assertNull(draft.note)
        assertFalse(draft.photoAttached)
        assertTrue(draft.canAdvance(DoseStep.CONTEXT))
    }

    @Test
    fun theReviewStepHasNoNext() {
        assertFalse(filledDraft().canAdvance(DoseStep.REVIEW))
    }

    @Test
    fun aCompleteDraftCanBeSubmittedEvenThoughItCannotAdvance() {
        // The trap `canSubmit` closes: wiring the last button to `canAdvance(REVIEW)` would
        // render a wizard nobody can finish.
        assertTrue(filledDraft().canSubmit())
        assertFalse(filledDraft().canAdvance(DoseStep.REVIEW))
    }

    @Test
    fun anIncompleteDraftCannotBeSubmittedWhicheverStepIsUnfinished() {
        val complete = filledDraft()

        assertFalse(DoseDraft().canSubmit(), "an empty draft")
        assertFalse(complete.copy(dose = null).canSubmit(), "no dose")
        assertFalse(complete.copy(dose = Dose(0.0, DoseUnit.MG)).canSubmit(), "a zero dose")
        assertFalse(complete.copy(site = null).canSubmit(), "an injection with no site")
    }

    @Test
    fun everyStepAfterTheFirstIsReachableAndTheLastIsTheEnd() {
        assertEquals(DoseStep.DOSE, DoseStep.COMPOUND.next())
        assertEquals(DoseStep.SITE, DoseStep.DOSE.next())
        assertEquals(DoseStep.CONTEXT, DoseStep.SITE.next())
        assertEquals(DoseStep.REVIEW, DoseStep.CONTEXT.next())
        assertNull(DoseStep.REVIEW.next())
    }

    @Test
    fun everyStepBeforeTheLastIsReachableBackwardsAndTheFirstIsTheStart() {
        assertNull(DoseStep.COMPOUND.previous())
        assertEquals(DoseStep.COMPOUND, DoseStep.DOSE.previous())
        assertEquals(DoseStep.DOSE, DoseStep.SITE.previous())
        assertEquals(DoseStep.SITE, DoseStep.CONTEXT.previous())
        assertEquals(DoseStep.CONTEXT, DoseStep.REVIEW.previous())
    }

    @Test
    fun choosingAnItemTakesTheDoseFromThePhaseInForceOnTheDay() {
        // A reset reading a compound's fixed "default" (what the prototype stores) would
        // answer 0,25 in both.
        assertEquals(Dose(0.25, DoseUnit.MG), DoseDraft().selectItem(plan(), SEMA, IN_WEEK_1).dose)
        assertEquals(Dose(0.5, DoseUnit.MG), DoseDraft().selectItem(plan(), SEMA, IN_WEEK_5).dose)
    }

    @Test
    fun changingTheItemReplacesThePreviousItemsDose() {
        // Guards against carrying 0,25 мг into a 250 мкг line when switching compounds.
        val switched = filledDraft().selectItem(plan(), BPC, IN_WEEK_1)

        assertEquals(BPC, switched.itemId)
        assertEquals(Dose(250.0, DoseUnit.MCG), switched.dose)
    }

    @Test
    fun reTappingTheItemAlreadyChosenLeavesAnAdjustedDoseAlone() {
        // The prototype re-applies comp.default on every press, silently restoring 0,25
        // after a step-down to 0,20 — a bug this doesn't port.
        val adjusted = filledDraft().copy(dose = Dose(0.2, DoseUnit.MG))

        assertEquals(adjusted, adjusted.selectItem(plan(), SEMA, IN_WEEK_1))
    }

    @Test
    fun theDoseIsANumberAndAUnitAndNeverARenderedString() {
        val dose = DoseDraft().selectItem(plan(), BPC, IN_WEEK_1).dose

        assertEquals(250.0, dose?.value)
        assertEquals(DoseUnit.MCG, dose?.unit)
    }

    @Test
    fun changingTheItemKeepsTheRestOfTheCheckIn() {
        // Only the dose is item-specific; the rest is about the patient, not the compound.
        val switched = filledDraft().selectItem(plan(), BPC, IN_WEEK_1)

        assertEquals(InjectionSite.LEFT_THIGH, switched.site)
        assertEquals(4, switched.mood)
        assertEquals(listOf(SideEffect.NAUSEA), switched.sideEffects)
        assertEquals("чуть тянет", switched.note)
        assertTrue(switched.photoAttached)
    }

    @Test
    fun aCancelledProtocolDosesNothing() {
        val draft = DoseDraft().selectItem(plan(ProtocolStatus.CANCELLED), SEMA, IN_WEEK_1)

        assertNull(draft.dose)
        assertFalse(draft.canAdvance(DoseStep.DOSE))
    }

    @Test
    fun aDateOutsideTheProtocolDosesNothing() {
        // A day before the course began, and a day after its twelve weeks.
        assertNull(DoseDraft().selectItem(plan(), SEMA, START.minus(DatePeriod(days = 1))).dose)
        assertNull(DoseDraft().selectItem(plan(), SEMA, LocalDate(2027, 1, 1)).dose)
    }
}

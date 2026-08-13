package app.cadence.shared.domain

import kotlinx.datetime.LocalDate

/**
 * The prototype's five steps, in order — named rather than numbered like its
 * `stepDefs[step - 1]`, so a step whose rule is «has a site» says so.
 */
enum class DoseStep { COMPOUND, DOSE, SITE, CONTEXT, REVIEW }

/** The step after this one, or null at the review — which submits instead. */
fun DoseStep.next(): DoseStep? = DoseStep.entries.getOrNull(ordinal + 1)

/** The step before this one, or null at the first. */
fun DoseStep.previous(): DoseStep? = DoseStep.entries.getOrNull(ordinal - 1)

/**
 * In `shared`, not a pile of `remember`s in the screen: moving between, validating and
 * reviewing five steps is logic. Nothing here is a rendered string — the prototype's
 * `LogState` does arithmetic on `parseFloat`, which is the subtask's one explicit
 * prohibition; [Dose] is the pair, and «0,25 мг» is assembled by a formatter at the edge.
 * The photo is a flag, not a path: the upload lands with the storage work.
 */
data class DoseDraft(
    val itemId: ProtocolItemId? = null,
    // Carried so the site step knows whether it applies: `DoseEvent.site` is nullable
    // because a supplement has no zone.
    val kind: ProtocolItemKind? = null,
    val dose: Dose? = null,
    val site: InjectionSite? = null,
    /**
     * Null is not «no vial»: with one open vial per compound the app's own choice is the
     * only one; the picker exists for two open at once.
     */
    val vialId: VialId? = null,
    val mood: Int? = null,
    val sideEffects: List<SideEffect> = emptyList(),
    val note: String? = null,
    val photoAttached: Boolean = false,
) {
    /**
     * The prototype omits a `nextDisabled` for its dose step, so it advances with an empty
     * field. Here every step answers, including the constant ones: `REVIEW` is false because
     * its button saves rather than advances, which is [canSubmit]'s question.
     */
    fun canAdvance(from: DoseStep): Boolean =
        when (from) {
            DoseStep.COMPOUND -> itemId != null

            // Present *and* positive: the stepper clamps at zero, and a zero dose event
            // would decrement a vial for an injection that didn't happen.
            DoseStep.DOSE -> dose != null && dose.value > 0

            DoseStep.SITE -> kind != ProtocolItemKind.INJECTION || site != null

            DoseStep.CONTEXT -> true

            DoseStep.REVIEW -> false
        }

    /**
     * Separate from [canAdvance]: a screen wiring its button to `canAdvance(REVIEW)` would
     * render a wizard that can never finish. Derived, so a new rule can't be forgotten here.
     */
    fun canSubmit(): Boolean = DoseStep.entries.dropLast(1).all { canAdvance(it) }
}

/**
 * The prototype resets to `comp.default`, a compound constant, so week 6 of a titration is
 * offered week 1's number; here the dose comes from the same function the calendar reads.
 * Only the dose is replaced — mood, side effects, note and photo are about the patient, not
 * the compound. Re-tapping the already-chosen item changes nothing: the prototype's row
 * isn't idempotent and silently resets a stepped-down dose, a bug this doesn't port.
 */
fun DoseDraft.selectItem(
    plan: ProtocolPlan,
    itemId: ProtocolItemId,
    on: LocalDate,
): DoseDraft {
    if (itemId == this.itemId) return this
    val item = plan.items.firstOrNull { it.id == itemId && it.loggable } ?: return this

    return copy(itemId = item.id, kind = item.kind, dose = phaseDose(plan, itemId, on))
}

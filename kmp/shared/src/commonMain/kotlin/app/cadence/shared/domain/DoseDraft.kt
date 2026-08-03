package app.cadence.shared.domain

import kotlinx.datetime.LocalDate

/**
 * The prototype's five steps, in its order — `stepDefs` in
 * `log-dose/LogDoseScreen.tsx`.
 *
 * Named rather than numbered: the prototype tracks `step` as an `int` from 1 to
 * `TOTAL` and reads `stepDefs[step - 1]`, so every guard it writes is an
 * arithmetic comparison against a magic number. A step whose rule is «has a
 * site» should say so.
 */
enum class DoseStep { COMPOUND, DOSE, SITE, CONTEXT, REVIEW }

/** The step after this one, or null at the review — which submits instead. */
fun DoseStep.next(): DoseStep? = DoseStep.entries.getOrNull(ordinal + 1)

/** The step before this one, or null at the first. */
fun DoseStep.previous(): DoseStep? = DoseStep.entries.getOrNull(ordinal - 1)

/**
 * Everything the wizard has collected so far, and the rules for leaving a step.
 *
 * In `shared` rather than in a pile of `remember`s in the screen: five steps
 * that can be moved between, validated and reviewed is logic, and logic goes
 * where the gate runs. `composeApp` renders this and reports taps.
 *
 * Nothing here is a rendered string. The prototype's `LogState` holds
 * `dose: string` beside `unit: string` and does arithmetic on `parseFloat`,
 * which is the subtask's one explicit prohibition; [Dose] is the pair, and
 * «0,25 мг» is assembled by a formatter at the edge.
 *
 * The photo is a flag rather than a path: §03 routes photos straight to object
 * storage under a path convention, and that upload lands with the storage work.
 * The slot renders and reports a tap; nothing here pretends to have a file.
 */
data class DoseDraft(
    val itemId: ProtocolItemId? = null,
    // Carried so the site step knows whether it applies: `DoseEvent.site` is
    // nullable because a supplement has no zone, and a wizard that demanded one
    // could not log the item at all.
    val kind: ProtocolItemKind? = null,
    val dose: Dose? = null,
    val site: InjectionSite? = null,
    val mood: Int? = null,
    val sideEffects: List<SideEffect> = emptyList(),
    val note: String? = null,
    val photoAttached: Boolean = false,
) {
    /**
     * Whether «Дальше» is live on [from].
     *
     * The prototype spells two of these as `nextDisabled` on the step
     * definition and simply omits the dose one, so its dose step advances with
     * an empty field. Here every step answers, including the two whose answer
     * is constant — `CONTEXT` because «всё по желанию», `REVIEW` because its
     * button saves rather than advances, which is [canSubmit]'s question.
     */
    fun canAdvance(from: DoseStep): Boolean =
        when (from) {
            DoseStep.COMPOUND -> itemId != null

            // Present *and* positive. The stepper clamps at zero, so «0 мг» is
            // a state a patient can reach by holding «−», and a dose event of
            // zero would decrement a vial for an injection that did not happen.
            DoseStep.DOSE -> dose != null && dose.value > 0

            DoseStep.SITE -> kind != ProtocolItemKind.INJECTION || site != null

            DoseStep.CONTEXT -> true

            DoseStep.REVIEW -> false
        }

    /**
     * Whether «Сохранить дозу» is live.
     *
     * A separate question from [canAdvance], and asking the wrong one is the
     * trap this exists to close: the review step advances nowhere, so a screen
     * wiring its button to `canAdvance(REVIEW)` would render a wizard that
     * cannot be finished.
     *
     * Every step before the review has to be satisfied — derived rather than
     * restated, so a rule added to [canAdvance] cannot be forgotten here.
     */
    fun canSubmit(): Boolean = DoseStep.entries.dropLast(1).all { canAdvance(it) }
}

/**
 * Choose what is being logged, and take the dose from the phase in force.
 *
 * The prototype resets to `comp.default`, a constant on the compound, so a
 * patient in week 6 of a titration is offered week 1's number. The dose in
 * force is a function of the protocol and the day — the same function the
 * calendar reads, so the wizard and the schedule cannot disagree about what
 * today's dose is.
 *
 * Only the dose is replaced. Mood, side effects, the note and the photo are
 * about the patient rather than about the compound, and clearing them because
 * they corrected a wrong first tap would be the wizard losing their work.
 *
 * Re-tapping the item already chosen changes nothing. The prototype's row is
 * not idempotent — it re-applies `comp.default` on every press, so a patient
 * who stepped the dose down to 0,20, went back and tapped the same compound
 * again silently got 0,25. That is one of the prototype's bugs, and the block
 * does not port those.
 *
 * An item the plan does not have, or one the protocol marks unloggable — a
 * weigh-in — is not a choice: the draft comes back unchanged and the selection
 * visibly does not move.
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

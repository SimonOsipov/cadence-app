package app.cadence.screens.dose

import androidx.compose.foundation.ScrollState
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import app.cadence.design.Cadence
import app.cadence.design.CadenceBody
import app.cadence.design.CadenceButton
import app.cadence.design.CadenceButtonKind
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTitle
import app.cadence.shared.domain.DoseDraft
import app.cadence.shared.domain.DoseStep
import app.cadence.shared.domain.InjectionSite
import app.cadence.shared.domain.next
import app.cadence.shared.domain.previous

/** «Шаг 1 · Препарат» and the rest, from the prototype's `stepDefs`. */
fun DoseStep.eyebrowRu(): String =
    "Шаг ${ordinal + 1} · " +
        when (this) {
            DoseStep.COMPOUND -> "Препарат"
            DoseStep.DOSE -> "Доза"
            DoseStep.SITE -> "Место"
            DoseStep.CONTEXT -> "Контекст"
            DoseStep.REVIEW -> "Проверка"
        }

/**
 * The step's question, with the word the prototype sets in italic display.
 *
 * Two runs rather than one string, because the emphasis is where the type
 * changes face — `<Title>Что вы <TitleEm>приняли</TitleEm>?</Title>`.
 */
@Composable
private fun DoseStep.titleRu(): AnnotatedString {
    val emphasis = SpanStyle(fontStyle = FontStyle.Italic, color = Cadence.palette.forestDeep)
    val (before, stressed, after) =
        when (this) {
            DoseStep.COMPOUND -> Triple("Что вы ", "приняли", "?")
            DoseStep.DOSE -> Triple("Сколько ", "взять", "?")
            DoseStep.SITE -> Triple("Куда на ", "теле", "?")
            DoseStep.CONTEXT -> Triple("Как вы ", "себя чувствуете", "?")
            DoseStep.REVIEW -> Triple("Последний ", "взгляд", ".")
        }

    return buildAnnotatedString {
        append(before)
        withStyle(emphasis) { append(stressed) }
        append(after)
    }
}

/** The line under the title, or null where the prototype has none. */
private fun DoseStep.subtitleRu(option: DoseOption?): String? =
    when (this) {
        DoseStep.COMPOUND -> "Сегодняшняя доза уже выбрана."
        DoseStep.DOSE -> option?.let { "По умолчанию для ${it.nameRu}. Нажмите +/− для подстройки." }
        DoseStep.SITE -> "Предложим следующую зону по ротации — выберите любую."
        DoseStep.CONTEXT -> "Короткая сверка — всё по желанию."
        DoseStep.REVIEW -> null
    }

private val PROGRESS_HEIGHT = 3.dp

/**
 * The five-step wizard: chrome, the step's body, and the two buttons.
 *
 * Stateless. The draft and the step come in and every change goes out, so the
 * screen is a function of its arguments — which is what lets a test drive it
 * and what will let the shell hold the state next to the repository.
 */
@Composable
fun DoseWizard(
    draft: DoseDraft,
    step: DoseStep,
    options: List<DoseOption>,
    suggestedSite: InjectionSite,
    modifier: Modifier = Modifier,
    /** Why the last «Сохранить дозу» did not write, if it did not. */
    notice: String? = null,
    lastUsedSites: List<InjectionSite> = emptyList(),
    onDraft: (DoseDraft) -> Unit = { },
    onStep: (DoseStep) -> Unit = { },
    onSubmit: () -> Unit = { },
    onCancel: () -> Unit = { },
) {
    val chosen = options.firstOrNull { it.itemId == draft.itemId }

    Column(
        modifier
            .fillMaxSize()
            .background(Cadence.palette.bg)
            // Like every other full-screen surface here — `TodayScreen`,
            // `ScheduleScreen`, `PlaceholderScreen`. Without it «Отмена» and
            // the counter render under the notch, and no simulator test sees it.
            .windowInsetsPadding(WindowInsets.statusBars),
    ) {
        WizardHeader(step, onCancel)
        WizardProgress(step)
        WizardHeading(step, chosen)

        Column(
            Modifier
                .weight(1f)
                // Keyed on the step: one shared scroll state opens step 4
                // half-way down because step 3's diagram was scrolled.
                .verticalScroll(remember(step) { ScrollState(0) })
                .padding(horizontal = CadenceSpacing.lg),
        ) {
            StepBody(
                step = step,
                draft = draft,
                options = options,
                chosen = chosen,
                suggestedSite = suggestedSite,
                lastUsedSites = lastUsedSites,
                onDraft = onDraft,
            )
        }

        notice?.let {
            CadenceBody(
                it,
                modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
                color = CadenceColors.danger,
            )
        }

        WizardFooter(
            step = step,
            draft = draft,
            onBack = { step.previous()?.let(onStep) ?: onCancel() },
            onNext = { step.next()?.let(onStep) ?: onSubmit() },
        )
    }
}

@Composable
private fun StepBody(
    step: DoseStep,
    draft: DoseDraft,
    options: List<DoseOption>,
    chosen: DoseOption?,
    suggestedSite: InjectionSite,
    lastUsedSites: List<InjectionSite>,
    onDraft: (DoseDraft) -> Unit,
) {
    when (step) {
        DoseStep.COMPOUND -> {
            CompoundStep(options, draft, onDraft)
        }

        DoseStep.DOSE -> {
            DoseAmountStep(draft, chosen, onDraft)
        }

        DoseStep.SITE -> {
            SiteStep(
                draft = draft,
                suggested = suggestedSite,
                lastUsed = lastUsedSites,
                onDraft = onDraft,
            )
        }

        DoseStep.CONTEXT -> {
            ContextStep(draft, onDraft)
        }

        DoseStep.REVIEW -> {
            ReviewStep(draft, chosen)
        }
    }
}

@Composable
private fun WizardHeader(
    step: DoseStep,
    onCancel: () -> Unit,
) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.xs),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CadenceButton(label = "Отмена", onClick = onCancel, kind = CadenceButtonKind.GHOST)
        CadenceMeta("Шаг ${step.ordinal + 1} из ${DoseStep.entries.size}")
        // The prototype balances its header with a fixed spacer so the counter
        // stays centred rather than drifting with the label's width.
        Box(Modifier.width(HEADER_SPACER))
    }
}

private val HEADER_SPACER = 50.dp

@Composable
private fun WizardProgress(step: DoseStep) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs),
    ) {
        DoseStep.entries.forEach { segment ->
            Box(
                Modifier
                    .weight(1f)
                    .height(PROGRESS_HEIGHT)
                    .clip(RoundedCornerShape(CadenceRadius.pill))
                    .background(
                        if (segment.ordinal <= step.ordinal) CadenceColors.forest700 else Cadence.palette.bone,
                    ),
            )
        }
    }
}

@Composable
private fun WizardHeading(
    step: DoseStep,
    chosen: DoseOption?,
) {
    Column(
        Modifier.padding(
            start = CadenceSpacing.xxl,
            end = CadenceSpacing.xxl,
            top = CadenceSpacing.xxl,
            bottom = CadenceSpacing.lg,
        ),
    ) {
        CadenceEyebrow(step.eyebrowRu())
        CadenceTitle(step.titleRu(), modifier = Modifier.padding(top = CadenceSpacing.sm))
        step.subtitleRu(chosen)?.let {
            CadenceBody(it, modifier = Modifier.padding(top = CadenceSpacing.sm), color = Cadence.palette.muted)
        }
    }
}

@Composable
private fun WizardFooter(
    step: DoseStep,
    draft: DoseDraft,
    onBack: () -> Unit,
    onNext: () -> Unit,
) {
    val last = step.next() == null

    Row(
        Modifier
            .fillMaxWidth()
            .windowInsetsPadding(WindowInsets.navigationBars)
            .padding(CadenceSpacing.lg),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        step.previous()?.let {
            CadenceButton(
                label = "Назад",
                onClick = onBack,
                kind = CadenceButtonKind.GHOST,
            )
        }
        CadenceButton(
            label = if (last) "Сохранить дозу" else "Дальше",
            onClick = onNext,
            modifier = Modifier.weight(1f),
            // `canSubmit` on the review and `canAdvance` before it: the review
            // advances nowhere, so asking `canAdvance` there would render a
            // wizard nobody can finish.
            enabled = if (last) draft.canSubmit() else draft.canAdvance(step),
        )
    }
}

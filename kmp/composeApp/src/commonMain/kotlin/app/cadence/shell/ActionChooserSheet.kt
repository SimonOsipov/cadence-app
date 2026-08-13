package app.cadence.shell

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSheet
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTitle
import app.cadence.design.cadenceEmphasisedTitle
import app.cadence.design.pressable
import app.cadence.format.formatInteger
import app.cadence.format.pluralMeals

/**
 * Ported from mobile/src/navigation/ActionChooserSheet.tsx. Data arrives as parameters rather
 * than read off app state, as the prototype does, so a test can render a day other than today.
 */
@Composable
fun ActionChooserSheet(
    open: Boolean,
    doseLogged: Boolean,
    /** «Семаглутид · 0,25 мг ждёт», built from the day's own occurrence. */
    doseDue: String?,
    mealCount: Int,
    mealKcal: Int,
    onDismiss: () -> Unit,
    onPickDose: () -> Unit,
    onPickMeal: () -> Unit,
) {
    CadenceSheet(open = open, onDismiss = onDismiss) {
        Column(Modifier.padding(bottom = HEADER_GAP)) {
            CadenceEyebrow("Что записываем?", Modifier.padding(bottom = CadenceSpacing.xs))
            CadenceTitle(
                cadenceEmphasisedTitle(prefix = "Выберите ", emphasis = "ритм", suffix = "."),
                size = SHEET_TITLE_SIZE,
            )
        }

        Column(
            modifier = Modifier.fillMaxWidth().padding(bottom = OPTIONS_GAP),
            verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm + 2.dp),
        ) {
            ActionOption(
                iconBackground = CadenceColors.forest700,
                iconTint = CadenceColors.cream,
                icon = CadenceIcons.beaker,
                title = "Записать дозу",
                subtitle =
                    if (doseLogged) {
                        "Уже записано сегодня · открыть или поправить"
                    } else {
                        // Must read the live doseDue, not the prototype's fixed copy — this
                        // regressed to the literal for three blocks after repositories landed,
                        // telling a patient on a different compound the wrong drug and dose.
                        doseDue ?: "На сегодня доза не назначена"
                    },
                onClick = onPickDose,
            )
            ActionOption(
                iconBackground = CadenceColors.sand500,
                iconTint = CadenceColors.ink900,
                icon = CadenceIcons.cake,
                title = "Записать приём пищи",
                subtitle =
                    if (mealCount == 0) {
                        "Пока ничего сегодня · начнём ритм"
                    } else {
                        "$mealCount ${pluralMeals(mealCount)} сегодня · ${formatInteger(mealKcal)} ккал"
                    },
                onClick = onPickMeal,
            )
        }

        CancelPill(onDismiss)
    }
}

private val HEADER_GAP = 18.dp
private val OPTIONS_GAP = 14.dp
private val SHEET_TITLE_SIZE = 28.sp
private val OPTION_RADIUS = 18.dp
private val OPTION_PADDING = 14.dp
private val OPTION_ICON_BOX = 52.dp
private val OPTION_ICON_RADIUS = 14.dp
private val OPTION_TITLE_SIZE = 22.sp
private val OPTION_SUBTITLE_SIZE = 12.sp

@Composable
private fun ActionOption(
    iconBackground: Color,
    iconTint: Color,
    icon: List<String>,
    title: String,
    subtitle: String,
    onClick: () -> Unit,
) {
    val palette = Cadence.palette
    val interactionSource = remember { MutableInteractionSource() }
    val shape = RoundedCornerShape(OPTION_RADIUS)

    Row(
        modifier =
            Modifier
                .fillMaxWidth()
                .pressable(onClick, interactionSource)
                .background(palette.paper, shape)
                .border(1.dp, palette.hairline, shape)
                .padding(OPTION_PADDING),
        horizontalArrangement = Arrangement.spacedBy(OPTION_PADDING),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            modifier =
                Modifier
                    .size(OPTION_ICON_BOX)
                    .background(iconBackground, RoundedCornerShape(OPTION_ICON_RADIUS)),
            contentAlignment = Alignment.Center,
        ) {
            CadenceIcon(paths = icon, size = 24.dp, tint = iconTint)
        }

        OptionCopy(title, subtitle)

        CadenceIcon(paths = CadenceIcons.chevronRight, size = 18.dp, tint = palette.subtle)
    }
}

@Composable
private fun RowScope.OptionCopy(
    title: String,
    subtitle: String,
) {
    Column(Modifier.weight(1f)) {
        CadenceTitle(title, size = OPTION_TITLE_SIZE, maxLines = 1)
        BasicText(
            text = subtitle,
            modifier = Modifier.padding(top = 3.dp),
            style =
                Cadence.typography.meta.copy(
                    color = Cadence.palette.muted,
                    fontSize = OPTION_SUBTITLE_SIZE,
                ),
        )
    }
}

/**
 * Kept local rather than added to `CadenceButton`: no existing `CadenceButtonKind` matches
 * it, and one call site is not yet a pattern worth promoting.
 */
@Composable
private fun CancelPill(onDismiss: () -> Unit) {
    val palette = Cadence.palette
    val interactionSource = remember { MutableInteractionSource() }
    val shape = RoundedCornerShape(CadenceRadius.pill)

    Box(
        modifier =
            Modifier
                .fillMaxWidth()
                .pressable(onDismiss, interactionSource)
                .border(1.dp, palette.border, shape)
                .padding(vertical = 13.dp),
        contentAlignment = Alignment.Center,
    ) {
        BasicText(
            text = "Отмена",
            style = Cadence.typography.label.copy(color = palette.muted, fontSize = 13.sp),
        )
    }
}

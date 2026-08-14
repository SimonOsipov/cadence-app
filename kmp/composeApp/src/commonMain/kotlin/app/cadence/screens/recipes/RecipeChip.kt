package app.cadence.screens.recipes

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.design.pressable

private val CHIP_BORDER = 1.dp
private val CHIP_LABEL_SIZE = 13.sp

/**
 * `FilterChip` (`RecipesScreen.tsx:20-39`) and the builder's own type/tag pills
 * (`RecipeBuilderScreen.tsx:527-580`): a hug-width pill, solid when active, an outline in
 * [app.cadence.design.CadencePalette.border] otherwise.
 *
 * Not [app.cadence.design.CadenceChip] — that primitive's active state is `palette.ink`,
 * matching a different prototype control. Shared here rather than in `design/` because the
 * tone pairs below are this context's, read off its own two screens.
 *
 * [activeBackground]/[activeForeground] default to the filter row's forest/cream; the
 * builder's tag row passes sand — the one thing the prototype varies between its own two
 * rows (`RecipeBuilderScreen.tsx:564,573`).
 */
@Composable
internal fun RecipeChip(
    label: String,
    active: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    activeBackground: Color = CadenceColors.forest700,
    activeForeground: Color = CadenceColors.cream,
) {
    val palette = Cadence.palette
    val shape = RoundedCornerShape(CadenceRadius.pill)
    val background = if (active) activeBackground else Color.Transparent
    val outline = if (active) activeBackground else palette.border
    val foreground = if (active) activeForeground else palette.ink2

    Box(
        modifier
            .pressable(onClick, remember { MutableInteractionSource() })
            .background(background, shape)
            .border(CHIP_BORDER, outline, shape)
            // Merged `selected`, the same idiom `CadenceSegmented` uses for its own option
            // boxes — lets a test assert both that this chip is selected and that a sibling
            // is not.
            .semantics(mergeDescendants = true) { selected = active }
            .padding(horizontal = CadenceSpacing.md, vertical = CadenceSpacing.xs),
    ) {
        BasicText(
            text = label,
            style = Cadence.typography.label.copy(color = foreground, fontSize = CHIP_LABEL_SIZE),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

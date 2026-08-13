package app.cadence.screens.nutrition

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import app.cadence.design.Cadence
import app.cadence.design.CadenceBody
import app.cadence.design.CadenceButton
import app.cadence.design.CadenceButtonKind
import app.cadence.design.CadenceChip
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceIconButton
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceNumber
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceStepper
import app.cadence.format.formatDecimal
import app.cadence.format.formatInteger
import app.cadence.format.russianPlural
import app.cadence.shared.domain.Macros
import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.MealItem
import app.cadence.shared.domain.sumTenths
import kotlin.math.roundToInt

// The item list, the totals strip and the «Сохранить» footer for
// `LogMealScreen.kt` (`LogMealScreen.tsx:270-394`) — split into their own
// file once the shell plus these three components would have crossed
// detekt's `LargeClass` threshold, the same way `commonTest/design/` already
// splits one test file per primitive rather than keeping one large file.

private val HAIRLINE = 1.dp
private val MACRO_DOT = 5.dp
private val CARD_SHAPE = RoundedCornerShape(CadenceRadius.lg)
private const val TENTHS_PER_UNIT = 10.0

/** Every position row carries this tag, so a test can count them with `onAllNodesWithTag`. */
const val LOG_MEAL_ITEM_ROW_TAG = "log-meal-item-row"

/** The «{имя приёма} · N позиций» / «{ккал} ккал» line — drawn only while there are items. */
const val LOG_MEAL_ITEMS_HEADER_TAG = "log-meal-items-header"

const val LOG_MEAL_TOTALS_TAG = "log-meal-totals"
const val LOG_MEAL_SAVE_TAG = "log-meal-save"

/**
 * One totals-strip column's value and «/ {цель}» caption, merged onto this
 * tag so a test can read both with one `assertTextEquals(value, caption)` —
 * [id] is stable English, kept separate from the Russian [TotalsColumn.label]
 * for the same reason `macroTrackTag`/`macroFillTag` in `CadenceMacroBar.kt`
 * key on a stable id rather than display copy.
 */
fun logMealTotalTag(id: String) = "log-meal-total-$id"

/** One position's own kcal — `logMealItemKcalTag(0) != logMealItemKcalTag(1)` is the whole point of it existing. */
fun logMealItemKcalTag(index: Int) = "log-meal-item-kcal-$index"

/** The grams pill that opens a position's inline stepper — `ParsedItem.onStartEdit` (`LogMealScreen.tsx:795-816`). */
fun logMealItemGramsTag(index: Int) = "log-meal-item-grams-$index"

/**
 * The sum of what is currently shown, tenths-exact and never turned into a
 * [Macros]. `MacrosTenths.toMacros()` stays the single production
 * conversion to [Macros], at the `TodaySummary` boundary (`Nutrition.kt:62-77`);
 * this file's four labels round straight from tenths through [tenthsLabel]
 * instead of minting a second, competing `Macros` value that nothing else
 * would ever read.
 */
private fun List<MealItem>.foldedMacros(): MacrosTenths = map { it.macros }.sumTenths()

/**
 * A tenths field as a whole unit, for display only — not
 * `MacrosTenths.toMacros()`; see [foldedMacros] for why this file
 * never calls it. [formatDecimal]'s round-half-up is already the tested rule
 * for a `Double`; treating a tenths count as `tenths / 10.0` reuses that rule
 * instead of re-deriving round-half-up a second time in this file.
 */
private fun tenthsLabel(tenths: Int): String = formatDecimal(tenths / TENTHS_PER_UNIT, digits = 0)

/**
 * The list, its header line and the totals strip — `LogMealScreen.tsx:270-318`.
 *
 * Takes the *current* (possibly edited) items, not the parsed originals:
 * [LogMealScreen] is what keeps each position's original values, threading
 * them into [app.cadence.shared.domain.rescaleMealItem] on every edit, so
 * everything below only ever renders the result of that.
 */
@Composable
internal fun LogMealItemsSection(
    mealName: String,
    items: List<MealItem>,
    targets: Macros,
    editingIndex: Int?,
    onToggleEdit: (Int) -> Unit,
    onGramsChange: (Int, Int) -> Unit,
    onDelete: (Int) -> Unit,
) {
    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
        ItemsHeaderLine(mealName, items)

        items.forEachIndexed { index, item ->
            MealItemRow(
                index = index,
                item = item,
                editing = editingIndex == index,
                onToggleEdit = { onToggleEdit(index) },
                onGramsChange = { onGramsChange(index, it) },
                onDelete = { onDelete(index) },
            )
        }

        TotalsStrip(items, targets)
    }
}

@Composable
private fun ItemsHeaderLine(
    mealName: String,
    items: List<MealItem>,
) {
    Row(
        Modifier.fillMaxWidth().testTag(LOG_MEAL_ITEMS_HEADER_TAG),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.Bottom,
    ) {
        val posWord = russianPlural(items.size, "позиция", "позиции", "позиций")
        CadenceEyebrow("$mealName · ${items.size} $posWord")
        CadenceMeta("${tenthsLabel(items.foldedMacros().kcalTenths)} ккал")
    }
}

@Composable
private fun MealItemRow(
    index: Int,
    item: MealItem,
    editing: Boolean,
    onToggleEdit: () -> Unit,
    onGramsChange: (Int) -> Unit,
    onDelete: () -> Unit,
) {
    Column(
        Modifier
            .fillMaxWidth()
            .clip(CARD_SHAPE)
            .background(Cadence.palette.paper)
            .border(HAIRLINE, Cadence.palette.hairline, CARD_SHAPE)
            .padding(CadenceSpacing.md)
            .testTag(LOG_MEAL_ITEM_ROW_TAG),
    ) {
        Row(
            Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs)) {
                CadenceBody(item.name, maxLines = 1)
                Row(horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
                    MacroBadge("Белок", item.macros.proteinGTenths, CadenceColors.forest700)
                    MacroBadge("Углеводы", item.macros.carbsGTenths, CadenceColors.sand600)
                    MacroBadge("Жиры", item.macros.fatGTenths, CadenceColors.sand700)
                }
            }

            CadenceChip(
                label = "${item.grams} г",
                onClick = onToggleEdit,
                active = editing,
                modifier = Modifier.testTag(logMealItemGramsTag(index)),
            )

            Column(
                horizontalAlignment = Alignment.End,
                verticalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs),
            ) {
                CadenceMeta(
                    "${tenthsLabel(item.macros.kcalTenths)} ккал",
                    modifier = Modifier.testTag(logMealItemKcalTag(index)),
                )
                CadenceIconButton(icon = CadenceIcons.trash, contentDescription = "Удалить позицию", onClick = onDelete)
            }
        }

        // Steps of 10, a floor of 5, no ceiling — `LogMealScreen.tsx:855,880`.
        // The floor is this stepper's own: `rescaleMealItem` clamps only at
        // zero (`coerceAtLeast(0)`), so 5 is enforced here, not there.
        if (editing) {
            CadenceStepper(
                value = item.grams.toDouble(),
                onChange = { onGramsChange(it.roundToInt()) },
                min = LOG_MEAL_GRAM_FLOOR,
                max = null,
                step = LOG_MEAL_GRAM_STEP,
                decimals = 0,
                unit = "г",
                modifier = Modifier.padding(top = CadenceSpacing.sm),
            )
        }
    }
}

/** A coloured dot plus «Белок 45 г» — `MacroBadge` (`LogMealScreen.tsx:739-755`). */
@Composable
private fun MacroBadge(
    label: String,
    tenths: Int,
    color: Color,
) {
    Row(
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs),
    ) {
        Box(Modifier.size(MACRO_DOT).clip(CircleShape).background(color))
        CadenceMeta("$label ${tenthsLabel(tenths)} г", color = Cadence.palette.muted)
    }
}

/**
 * Four numeric columns, not a bar: the spec is explicit that this strip is
 * its own code rather than three calls to `CadenceMacroBar` — a bar measures
 * a value against a goal as a fraction, and this reads as two numbers side by
 * side (`LogMealScreen.tsx:903-959`).
 */
@Composable
private fun TotalsStrip(
    items: List<MealItem>,
    targets: Macros,
) {
    val totals = items.foldedMacros()

    Row(
        Modifier
            .fillMaxWidth()
            .clip(CARD_SHAPE)
            .background(Cadence.palette.paper)
            .border(HAIRLINE, Cadence.palette.hairline, CARD_SHAPE)
            .padding(CadenceSpacing.md)
            .testTag(LOG_MEAL_TOTALS_TAG),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
    ) {
        TotalsColumn(
            id = "kcal",
            label = "ккал",
            value = tenthsLabel(totals.kcalTenths),
            goal = formatInteger(targets.kcal),
            modifier = Modifier.weight(1f),
        )
        TotalsColumn(
            id = "protein",
            label = "белок",
            value = tenthsLabel(totals.proteinGTenths),
            goal = "${formatInteger(targets.proteinG)} г",
            modifier = Modifier.weight(1f),
        )
        TotalsColumn(
            id = "carbs",
            label = "углев",
            value = tenthsLabel(totals.carbsGTenths),
            goal = "${formatInteger(targets.carbsG)} г",
            modifier = Modifier.weight(1f),
        )
        TotalsColumn(
            id = "fat",
            label = "жиры",
            value = tenthsLabel(totals.fatGTenths),
            goal = "${formatInteger(targets.fatG)} г",
            modifier = Modifier.weight(1f),
        )
    }
}

@Composable
private fun TotalsColumn(
    id: String,
    label: String,
    value: String,
    goal: String,
    modifier: Modifier,
) {
    // Tagged and merged on this outer Column, not just the value/caption pair
    // below it: `label` is the column's *name* for the user — «белок» versus
    // «жиры» — and a test reading only the value+caption node cannot catch
    // the two labels being swapped between columns. Merging the whole column
    // onto one tag makes `assertTextEquals(label, value, caption)` the single
    // assertion that would catch that transposition.
    Column(
        modifier
            .semantics(mergeDescendants = true) { }
            .testTag(logMealTotalTag(id)),
    ) {
        CadenceEyebrow(label)
        Column {
            CadenceNumber(value = value)
            CadenceMeta("/ $goal", color = Cadence.palette.subtle)
        }
    }
}

/**
 * «Сохранить · {N} ккал», inactive «Добавьте что-нибудь» — always drawn, not
 * only once there are items: the prototype's footer sits outside every stage
 * branch (`LogMealScreen.tsx:344-394`), inactive rather than absent until
 * there is something to save.
 */
@Composable
internal fun LogMealFooter(
    items: List<MealItem>,
    onSave: () -> Unit,
) {
    val canSave = items.isNotEmpty()
    val label =
        if (canSave) {
            "Сохранить · ${tenthsLabel(items.foldedMacros().kcalTenths)} ккал"
        } else {
            "Добавьте что-нибудь"
        }

    CadenceButton(
        label = label,
        onClick = onSave,
        kind = CadenceButtonKind.PRIMARY,
        fillWidth = true,
        enabled = canSave,
        modifier = Modifier.padding(top = CadenceSpacing.sm).testTag(LOG_MEAL_SAVE_TAG),
    )
}

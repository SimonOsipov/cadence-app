package app.cadence.design

/**
 * Half a bar's rounding, in fractions. Shared by [CadenceMacroBarTest],
 * [CadenceSplitBarTest] and [CadenceWeekBarsTest] — the seven-file split of
 * `NutritionPrimitivesTest` (69ed167) left each with its own file-private
 * copy and nothing kept them in step. Hoisted here so a future change is one
 * edit, not three that have to be remembered together.
 */
internal const val BAR_FRACTION_TOLERANCE = 0.03f

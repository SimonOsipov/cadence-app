package app.cadence.design

/**
 * Half a bar's rounding, in fractions. Shared by [CadenceMacroBarTest], [CadenceSplitBarTest]
 * and [CadenceWeekBarsTest] — the seven-file split of `NutritionPrimitivesTest` (69ed167) left
 * each of the three with its own file-private copy of the same value and nothing kept them in
 * step. Hoisted here, package-visible, so a future change to the tolerance is one edit rather
 * than three that have to be remembered together.
 */
internal const val BAR_FRACTION_TOLERANCE = 0.03f

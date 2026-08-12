package app.cadence.shared.parsing

import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.MealItem

private const val TENTHS_PER_UNIT = 10

/**
 * One entry of `SAMPLE_PARSES` (`mobile/src/features/meal/data.ts:75-116`),
 * ported whole: the trigger words [MockMealParser] scores against, the
 * placeholder shown while a text field is empty, the transcript and items a
 * match answers with.
 */
internal data class CannedMealParse(
    val triggerWords: List<String>,
    val placeholder: String,
    val transcript: String,
    val mealName: String,
    val items: List<MealItem>,
)

/**
 * Builds one [MealItem] from the prototype's whole-gram, whole-kcal fields,
 * converting to [MacrosTenths] at the seed — an item of 45 kcal becomes
 * `kcalTenths = 450`, per the step-2 boundary: tenths are computed once, here,
 * never re-derived downstream.
 *
 * Six parameters name the six fields `MealItem` (`mobile/src/features/meal/data.ts:11-19`)
 * carries — the same trade `CadenceShell.kt` already makes three times over
 * (`tabRoutes`, `pushedRoutes`, `modalRoutes`, each under this same
 * `@Suppress`) rather than wrapping a one-off tuple type around a table that
 * exists nowhere else.
 */
@Suppress("LongParameterList")
private fun item(
    name: String,
    grams: Int,
    kcal: Int,
    proteinG: Int,
    carbsG: Int,
    fatG: Int,
) = MealItem(
    name = name,
    grams = grams,
    macros =
        MacrosTenths(
            kcalTenths = kcal * TENTHS_PER_UNIT,
            proteinGTenths = proteinG * TENTHS_PER_UNIT,
            carbsGTenths = carbsG * TENTHS_PER_UNIT,
            fatGTenths = fatG * TENTHS_PER_UNIT,
        ),
)

/**
 * The two facts a text-input hint control needs from a canned parse, without
 * the parse itself.
 *
 * **Visibility decision (step-4's own open question).** [CannedMealParse] and
 * [CANNED_MEAL_PARSES] stay `internal`; this type and [mealSamplePrompts] are
 * the public surface instead of widening them. Making the canned list itself
 * public would hand `composeApp` the mock's whole matching table —
 * `triggerWords`, `mealName`, `items` and all — for the sake of a control
 * that only ever reads two of its five fields. [MealParser] is documented as
 * "a service, not a repository": a widened [CANNED_MEAL_PARSES] would make it
 * one anyway, just via a second, undocumented door next to `parse`. This
 * accessor is that door, narrowed to what «Пример» actually needs, and it
 * costs nothing at the real M9 parser: nothing requires a service's mock to
 * expose its fixtures wholesale.
 */
data class MealSamplePrompt(
    val placeholder: String,
    val transcript: String,
)

/**
 * The canned prompts «Пример» cycles through, in [CANNED_MEAL_PARSES]'s own
 * order — cycling in a different order than [MockMealParser] scores in would
 * make the button's next placeholder disagree with what a matching parse of
 * that same placeholder would return.
 */
fun mealSamplePrompts(): List<MealSamplePrompt> =
    CANNED_MEAL_PARSES.map { MealSamplePrompt(placeholder = it.placeholder, transcript = it.transcript) }

/**
 * The three canned parses, ported whole from `SAMPLE_PARSES`
 * (`mobile/src/features/meal/data.ts:76-115`) in the prototype's own order.
 * Order matters: [MockMealParser]'s tie-break keeps whichever entry is
 * checked first, so breakfast staying first is what makes it the fallback for
 * unmatched text.
 */
internal val CANNED_MEAL_PARSES: List<CannedMealParse> =
    listOf(
        CannedMealParse(
            triggerWords =
                listOf(
                    "yogurt",
                    "eggs",
                    "sourdough",
                    "avocado",
                    "berries",
                    "oats",
                    "oatmeal",
                    "йогурт",
                    "яйц",
                    "хлеб",
                    "авокадо",
                    "ягод",
                ),
            placeholder = "«Греческий йогурт с ягодами, два варёных яйца и тост с авокадо»",
            transcript = "Греческий йогурт с ягодами, два варёных яйца и кусок ржаного хлеба с авокадо.",
            mealName = "Завтрак",
            items =
                listOf(
                    item("Греческий йогурт", grams = 150, kcal = 130, proteinG = 17, carbsG = 6, fatG = 4),
                    item("Ягодная смесь", grams = 80, kcal = 45, proteinG = 1, carbsG = 11, fatG = 0),
                    item("Варёные яйца", grams = 100, kcal = 155, proteinG = 13, carbsG = 1, fatG = 11),
                    item("Ржаной хлеб", grams = 40, kcal = 110, proteinG = 4, carbsG = 22, fatG = 1),
                    item("Авокадо", grams = 60, kcal = 96, proteinG = 1, carbsG = 5, fatG = 9),
                ),
        ),
        CannedMealParse(
            triggerWords =
                listOf(
                    "bowl",
                    "chicken",
                    "rice",
                    "salmon",
                    "quinoa",
                    "salad",
                    "lunch",
                    "обед",
                    "курица",
                    "рис",
                    "батат",
                    "тахини",
                    "капуста",
                ),
            placeholder = "«Куриная боул-чашка с рисом, капустой и тахини»",
            transcript = "Боул с куриной грудкой: бурый рис, кейл, печёный батат и капля тахини.",
            mealName = "Обед",
            items =
                listOf(
                    item("Куриная грудка", grams = 150, kcal = 240, proteinG = 45, carbsG = 0, fatG = 6),
                    item("Бурый рис", grams = 120, kcal = 145, proteinG = 3, carbsG = 30, fatG = 1),
                    item("Кейл с заправкой", grams = 80, kcal = 60, proteinG = 3, carbsG = 6, fatG = 3),
                    item("Батат", grams = 100, kcal = 90, proteinG = 2, carbsG = 21, fatG = 0),
                    item("Тахини", grams = 15, kcal = 90, proteinG = 3, carbsG = 3, fatG = 8),
                ),
        ),
        CannedMealParse(
            triggerWords =
                listOf(
                    "snack",
                    "cottage",
                    "almond",
                    "apple",
                    "peanut",
                    "перекус",
                    "творог",
                    "яблок",
                    "миндаль",
                ),
            placeholder = "«Творог с яблоком»",
            transcript = "Творог с нарезанным яблоком и каплей миндальной пасты.",
            mealName = "Перекус",
            items =
                listOf(
                    item("Творог", grams = 150, kcal = 145, proteinG = 21, carbsG = 6, fatG = 4),
                    item("Яблоко", grams = 180, kcal = 95, proteinG = 0, carbsG = 25, fatG = 0),
                    item("Миндальная паста", grams = 15, kcal = 95, proteinG = 3, carbsG = 3, fatG = 8),
                ),
        ),
    )

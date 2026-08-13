package app.cadence.shared.mock

import app.cadence.shared.domain.Ingredient
import app.cadence.shared.domain.IngredientId
import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.MealType
import app.cadence.shared.domain.Recipe
import app.cadence.shared.domain.RecipeId
import app.cadence.shared.domain.RecipeIngredient
import app.cadence.shared.domain.RecipeTag

// The recipe library's seed: the prototype's 25-ingredient reference table
// and its six starter recipes, ported whole (`mobile/src/features/recipe/data.ts`).
//
// A separate file rather than members of `MockSeed` itself, matching
// `parsing/CannedMealParses.kt`'s precedent for the same reason: `MockSeed.kt`'s
// object body is already close to detekt's `LargeClass` threshold, and this
// table plus the six recipes below (with their steps in full) would push it
// over. `MockSeed.kt` stays untouched by this step; [ingredients] and
// [recipes] are extension properties, not members, so `MockSeed.kt`'s class
// body — the thing `LargeClass` measures — does not grow by a single line.
// `detekt.yml`'s `MagicNumber` excludes this file for the same reason it
// excludes `MockSeed.kt` and `CannedMealParses.kt`: the numbers here (grams,
// tenths of a macro, minutes) are the ported specification, not thresholds a
// name would clarify.

/**
 * §03's `ingredients`: the prototype's 25-entry reference table
 * (`mobile/src/features/recipe/data.ts:57-81`), ported at 0,1 g precision —
 * [MacrosTenths], not whole grams. kcal is tenths too, per the step-2
 * deviation: chicken's `kcalTenths` is 1650, not 165 — the trap that
 * deviation was written to warn this seed away from.
 */
val MockSeed.ingredients: List<Ingredient> get() = SEED_INGREDIENTS

/**
 * §03's `recipes`: the prototype's six starter recipes
 * (`mobile/src/features/recipe/data.ts:105-187`), `ownerId = null` — the
 * clinic's library, not a patient's own. Steps and descriptions (`dek`) are
 * carried in full, word for word.
 */
val MockSeed.recipes: List<Recipe> get() = SEED_RECIPES

/**
 * Six parameters for the six fields `IngredientMeta` (`recipe/data.ts:17-20`)
 * carries — the same trade `CannedMealParses.kt`'s `item()` already makes for
 * the same reason: a one-off tuple type around a table that exists nowhere
 * else would name nothing this helper's own parameter names do not.
 */
@Suppress("LongParameterList")
private fun ingredient(
    id: String,
    nameRu: String,
    kcalTenths: Int,
    proteinGTenths: Int,
    carbsGTenths: Int,
    fatGTenths: Int,
) = Ingredient(
    id = IngredientId(id),
    nameRu = nameRu,
    per100g =
        MacrosTenths(
            kcalTenths = kcalTenths,
            proteinGTenths = proteinGTenths,
            carbsGTenths = carbsGTenths,
            fatGTenths = fatGTenths,
        ),
)

private val SEED_INGREDIENTS: List<Ingredient> =
    listOf(
        ingredient("chicken", "Куриная грудка", 1650, 310, 0, 36),
        ingredient("turkey", "Индейка, филе", 1350, 290, 0, 15),
        ingredient("salmon", "Лосось", 2080, 200, 0, 130),
        ingredient("tuna", "Тунец", 1160, 260, 0, 10),
        ingredient("shrimp", "Креветки", 990, 240, 2, 3),
        ingredient("egg", "Яйцо", 1550, 130, 11, 110),
        ingredient("cottage", "Творог 5%", 1210, 170, 30, 50),
        ingredient("greekyog", "Греческий йогурт", 590, 100, 36, 4),
        ingredient("kefir", "Кефир 1%", 400, 34, 40, 10),
        ingredient("tofu", "Тофу", 1440, 170, 28, 90),
        ingredient("whey", "Протеин, порошок", 3800, 750, 80, 60),
        ingredient("rice", "Бурый рис, варёный", 1230, 27, 250, 10),
        ingredient("oats", "Овсянка, сухая", 3790, 130, 670, 70),
        ingredient("quinoa", "Киноа, варёная", 1200, 44, 210, 19),
        ingredient("lentils", "Чечевица, варёная", 1160, 90, 200, 4),
        ingredient("sweetpot", "Батат", 860, 16, 200, 1),
        ingredient("broccoli", "Брокколи", 340, 28, 70, 4),
        ingredient("spinach", "Шпинат", 230, 29, 36, 4),
        ingredient("avocado", "Авокадо", 1600, 20, 90, 150),
        ingredient("banana", "Банан", 890, 11, 230, 3),
        ingredient("berries", "Ягоды", 570, 7, 140, 3),
        ingredient("almond", "Миндаль", 5790, 210, 220, 500),
        ingredient("tahini", "Тахини", 5950, 170, 210, 540),
        ingredient("oliveoil", "Оливковое масло", 8840, 0, 0, 1000),
        ingredient("peanut", "Арахисовая паста", 5880, 250, 200, 500),
    )

private fun recipeIngredient(
    id: String,
    grams: Int,
) = RecipeIngredient(ingredientId = IngredientId(id), grams = grams)

private val SEED_RECIPES: List<Recipe> =
    listOf(
        Recipe(
            id = RecipeId("chicken-bowl"),
            ownerId = null,
            name = "Боул с курицей и тахини",
            mealType = MealType.LUNCH,
            tags = listOf(RecipeTag.PROTEIN),
            servings = 2,
            prepMin = 15,
            cookMin = 20,
            dek = "Опорный обед курса: много белка, тёплые овощи и капля тахини для вкуса.",
            ingredients =
                listOf(
                    recipeIngredient("chicken", 300),
                    recipeIngredient("rice", 240),
                    recipeIngredient("broccoli", 160),
                    recipeIngredient("sweetpot", 200),
                    recipeIngredient("tahini", 30),
                    recipeIngredient("oliveoil", 10),
                ),
            steps =
                listOf(
                    "Нарежьте куриную грудку полосками, посолите и оставьте на 10 минут.",
                    "Батат кубиками, брокколи соцветиями — запеките при 200 °C около 20 минут с ложкой масла.",
                    "Обжарьте курицу на сухой сковороде до золотистого, 6–8 минут.",
                    "Соберите боул: рис, овощи, курица. Полейте тахини, разведённой ложкой воды.",
                ),
        ),
        Recipe(
            id = RecipeId("cottage-bowl"),
            ownerId = null,
            name = "Творожная чаша с ягодами",
            mealType = MealType.BREAKFAST,
            tags = listOf(RecipeTag.PROTEIN, RecipeTag.GENTLE, RecipeTag.QUICK),
            servings = 1,
            prepMin = 5,
            cookMin = 0,
            dek = "Пять минут, 30+ грамм белка и ничего тяжёлого для утра.",
            ingredients =
                listOf(
                    recipeIngredient("cottage", 200),
                    recipeIngredient("berries", 80),
                    recipeIngredient("banana", 60),
                    recipeIngredient("almond", 15),
                ),
            steps =
                listOf(
                    "Выложите творог в миску, разровняйте.",
                    "Сверху — ягоды и нарезанный банан.",
                    "Присыпьте рубленым миндалём. По желанию — ложка кефира для нежности.",
                ),
        ),
        Recipe(
            id = RecipeId("spinach-omelette"),
            ownerId = null,
            name = "Омлет со шпинатом и творогом",
            mealType = MealType.BREAKFAST,
            tags = listOf(RecipeTag.PROTEIN, RecipeTag.GENTLE, RecipeTag.QUICK),
            servings = 1,
            prepMin = 5,
            cookMin = 8,
            dek = "Мягкий, воздушный, садится легко даже в тошнотные дни.",
            ingredients =
                listOf(
                    recipeIngredient("egg", 150),
                    recipeIngredient("spinach", 50),
                    recipeIngredient("cottage", 50),
                    recipeIngredient("oliveoil", 5),
                ),
            steps =
                listOf(
                    "Взбейте яйца с щепоткой соли.",
                    "Припустите шпинат на капле масла, 1 минуту.",
                    "Влейте яйца, на среднем огне доведите до мягкости.",
                    "В центр — ложку творога, сложите омлет пополам.",
                ),
        ),
        Recipe(
            id = RecipeId("salmon-quinoa"),
            ownerId = null,
            name = "Лосось с киноа и брокколи",
            mealType = MealType.DINNER,
            tags = listOf(RecipeTag.PROTEIN),
            servings = 2,
            prepMin = 10,
            cookMin = 18,
            dek = "Спокойный ужин: жиры из лосося и ровный белок без тяжести.",
            ingredients =
                listOf(
                    recipeIngredient("salmon", 300),
                    recipeIngredient("quinoa", 300),
                    recipeIngredient("broccoli", 200),
                    recipeIngredient("oliveoil", 16),
                ),
            steps =
                listOf(
                    "Лосось посолите, сбрызните маслом и запеките при 190 °C, 14–16 минут.",
                    "Брокколи отварите на пару 5 минут — до яркого зелёного.",
                    "Подавайте на тёплой киноа, рыбу сверху.",
                ),
        ),
        Recipe(
            id = RecipeId("lentil-turkey"),
            ownerId = null,
            name = "Тёплая чечевица с индейкой",
            mealType = MealType.DINNER,
            tags = listOf(RecipeTag.PROTEIN, RecipeTag.GENTLE),
            servings = 2,
            prepMin = 10,
            cookMin = 20,
            dek = "Сытно и мягко: клетчатка чечевицы и постная индейка.",
            ingredients =
                listOf(
                    recipeIngredient("turkey", 240),
                    recipeIngredient("lentils", 300),
                    recipeIngredient("spinach", 60),
                    recipeIngredient("oliveoil", 16),
                ),
            steps =
                listOf(
                    "Индейку нарежьте кубиками, обжарьте на ложке масла до готовности.",
                    "Добавьте варёную чечевицу, прогрейте вместе 3–4 минуты.",
                    "В конце вмешайте шпинат, дайте ему опасть. Посолите.",
                ),
        ),
        Recipe(
            id = RecipeId("protein-smoothie"),
            ownerId = null,
            name = "Протеиновый смузи",
            mealType = MealType.SNACK,
            tags = listOf(RecipeTag.PROTEIN, RecipeTag.GENTLE, RecipeTag.QUICK),
            servings = 1,
            prepMin = 3,
            cookMin = 0,
            dek = "Когда жевать не хочется: белок в стакане, мягко и быстро.",
            ingredients =
                listOf(
                    recipeIngredient("whey", 30),
                    recipeIngredient("kefir", 200),
                    recipeIngredient("banana", 80),
                    recipeIngredient("peanut", 15),
                ),
            steps =
                listOf(
                    "Сложите всё в блендер.",
                    "Взбейте до гладкости, 30 секунд.",
                    "Густо — добавьте воды или льда по вкусу.",
                ),
        ),
    )

package app.cadence.shared.mock

import app.cadence.shared.domain.IngredientId
import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.MealType
import app.cadence.shared.domain.RecipeId
import app.cadence.shared.domain.RecipeIngredient
import app.cadence.shared.domain.RecipeTag
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * Every seeded ingredient and recipe, pinned against
 * `mobile/src/features/recipe/data.ts:57-187` — verified by running that
 * file's own table through a script that multiplies each field by ten and
 * comparing the output to the literals below, not by re-typing 25 rows and
 * trusting the second typing. The set, not the count: a seed that dropped an
 * entry or duplicated one would still report the right size.
 */
class MockSeedRecipesTest {
    @Test
    fun thereAreExactlyTwentyFiveIngredients() {
        assertEquals(25, MockSeed.ingredients.size)
        assertEquals(
            25,
            MockSeed.ingredients
                .map { it.id }
                .distinct()
                .size,
            "an id repeated",
        )
    }

    @Test
    fun everyIngredientCarriesThePrototypesMacrosAtTenthsPrecision() {
        // id -> nameRu, kcalTenths, proteinGTenths, carbsGTenths, fatGTenths.
        // kcalTenths is ten times the prototype's whole-kcal column — chicken's
        // 165 kcal/100g is 1650, not 165, which is the exact trap the step-2
        // deviation records this seed against.
        val expected =
            listOf(
                Triple("chicken", "Куриная грудка", MacrosTenths(1650, 310, 0, 36)),
                Triple("turkey", "Индейка, филе", MacrosTenths(1350, 290, 0, 15)),
                Triple("salmon", "Лосось", MacrosTenths(2080, 200, 0, 130)),
                Triple("tuna", "Тунец", MacrosTenths(1160, 260, 0, 10)),
                Triple("shrimp", "Креветки", MacrosTenths(990, 240, 2, 3)),
                Triple("egg", "Яйцо", MacrosTenths(1550, 130, 11, 110)),
                Triple("cottage", "Творог 5%", MacrosTenths(1210, 170, 30, 50)),
                Triple("greekyog", "Греческий йогурт", MacrosTenths(590, 100, 36, 4)),
                Triple("kefir", "Кефир 1%", MacrosTenths(400, 34, 40, 10)),
                Triple("tofu", "Тофу", MacrosTenths(1440, 170, 28, 90)),
                Triple("whey", "Протеин, порошок", MacrosTenths(3800, 750, 80, 60)),
                Triple("rice", "Бурый рис, варёный", MacrosTenths(1230, 27, 250, 10)),
                Triple("oats", "Овсянка, сухая", MacrosTenths(3790, 130, 670, 70)),
                Triple("quinoa", "Киноа, варёная", MacrosTenths(1200, 44, 210, 19)),
                Triple("lentils", "Чечевица, варёная", MacrosTenths(1160, 90, 200, 4)),
                Triple("sweetpot", "Батат", MacrosTenths(860, 16, 200, 1)),
                Triple("broccoli", "Брокколи", MacrosTenths(340, 28, 70, 4)),
                Triple("spinach", "Шпинат", MacrosTenths(230, 29, 36, 4)),
                Triple("avocado", "Авокадо", MacrosTenths(1600, 20, 90, 150)),
                Triple("banana", "Банан", MacrosTenths(890, 11, 230, 3)),
                Triple("berries", "Ягоды", MacrosTenths(570, 7, 140, 3)),
                Triple("almond", "Миндаль", MacrosTenths(5790, 210, 220, 500)),
                Triple("tahini", "Тахини", MacrosTenths(5950, 170, 210, 540)),
                Triple("oliveoil", "Оливковое масло", MacrosTenths(8840, 0, 0, 1000)),
                Triple("peanut", "Арахисовая паста", MacrosTenths(5880, 250, 200, 500)),
            )
        assertEquals(25, expected.size, "the expectation table itself must list all 25 — a floor, not a sample")

        val byId = MockSeed.ingredients.associateBy { it.id }
        for ((id, nameRu, per100g) in expected) {
            val ingredient = byId[IngredientId(id)] ?: error("no seeded ingredient with id $id")
            assertEquals(nameRu, ingredient.nameRu, "name for $id")
            assertEquals(per100g, ingredient.per100g, "per100g for $id ($nameRu)")
        }
    }

    @Test
    fun thereAreExactlySixLibraryRecipes() {
        assertEquals(6, MockSeed.recipes.size)
        assertEquals(
            6,
            MockSeed.recipes
                .map { it.id }
                .distinct()
                .size,
            "an id repeated",
        )
    }

    @Test
    fun everyLibraryRecipeHasNoOwner() {
        assertEquals(emptyList<String>(), MockSeed.recipes.filter { it.ownerId != null }.map { it.id.raw })
    }

    @Test
    fun chickenBowlCarriesItsFullIngredientsAndSteps() {
        val recipe = MockSeed.recipes.first { it.id == RecipeId("chicken-bowl") }

        assertEquals("Боул с курицей и тахини", recipe.name)
        assertEquals(MealType.LUNCH, recipe.mealType)
        assertEquals(listOf(RecipeTag.PROTEIN), recipe.tags)
        assertEquals(2, recipe.servings)
        assertEquals(15, recipe.prepMin)
        assertEquals(20, recipe.cookMin)
        assertEquals(
            "Опорный обед курса: много белка, тёплые овощи и капля тахини для вкуса.",
            recipe.dek,
        )
        assertEquals(
            listOf(
                RecipeIngredient(IngredientId("chicken"), 300),
                RecipeIngredient(IngredientId("rice"), 240),
                RecipeIngredient(IngredientId("broccoli"), 160),
                RecipeIngredient(IngredientId("sweetpot"), 200),
                RecipeIngredient(IngredientId("tahini"), 30),
                RecipeIngredient(IngredientId("oliveoil"), 10),
            ),
            recipe.ingredients,
        )
        assertEquals(
            listOf(
                "Нарежьте куриную грудку полосками, посолите и оставьте на 10 минут.",
                "Батат кубиками, брокколи соцветиями — запеките при 200 °C около 20 минут с ложкой масла.",
                "Обжарьте курицу на сухой сковороде до золотистого, 6–8 минут.",
                "Соберите боул: рис, овощи, курица. Полейте тахини, разведённой ложкой воды.",
            ),
            recipe.steps,
        )
    }

    @Test
    fun cottageBowlCarriesItsFullIngredientsAndSteps() {
        val recipe = MockSeed.recipes.first { it.id == RecipeId("cottage-bowl") }

        assertEquals("Творожная чаша с ягодами", recipe.name)
        assertEquals(MealType.BREAKFAST, recipe.mealType)
        assertEquals(listOf(RecipeTag.PROTEIN, RecipeTag.GENTLE, RecipeTag.QUICK), recipe.tags)
        assertEquals(1, recipe.servings)
        assertEquals(5, recipe.prepMin)
        assertEquals(0, recipe.cookMin)
        assertEquals("Пять минут, 30+ грамм белка и ничего тяжёлого для утра.", recipe.dek)
        assertEquals(
            listOf(
                RecipeIngredient(IngredientId("cottage"), 200),
                RecipeIngredient(IngredientId("berries"), 80),
                RecipeIngredient(IngredientId("banana"), 60),
                RecipeIngredient(IngredientId("almond"), 15),
            ),
            recipe.ingredients,
        )
        assertEquals(
            listOf(
                "Выложите творог в миску, разровняйте.",
                "Сверху — ягоды и нарезанный банан.",
                "Присыпьте рубленым миндалём. По желанию — ложка кефира для нежности.",
            ),
            recipe.steps,
        )
    }

    @Test
    fun spinachOmeletteCarriesItsFullIngredientsAndSteps() {
        val recipe = MockSeed.recipes.first { it.id == RecipeId("spinach-omelette") }

        assertEquals("Омлет со шпинатом и творогом", recipe.name)
        assertEquals(MealType.BREAKFAST, recipe.mealType)
        assertEquals(listOf(RecipeTag.PROTEIN, RecipeTag.GENTLE, RecipeTag.QUICK), recipe.tags)
        assertEquals(1, recipe.servings)
        assertEquals(5, recipe.prepMin)
        assertEquals(8, recipe.cookMin)
        assertEquals("Мягкий, воздушный, садится легко даже в тошнотные дни.", recipe.dek)
        assertEquals(
            listOf(
                RecipeIngredient(IngredientId("egg"), 150),
                RecipeIngredient(IngredientId("spinach"), 50),
                RecipeIngredient(IngredientId("cottage"), 50),
                RecipeIngredient(IngredientId("oliveoil"), 5),
            ),
            recipe.ingredients,
        )
        assertEquals(
            listOf(
                "Взбейте яйца с щепоткой соли.",
                "Припустите шпинат на капле масла, 1 минуту.",
                "Влейте яйца, на среднем огне доведите до мягкости.",
                "В центр — ложку творога, сложите омлет пополам.",
            ),
            recipe.steps,
        )
    }

    @Test
    fun salmonQuinoaCarriesItsFullIngredientsAndSteps() {
        val recipe = MockSeed.recipes.first { it.id == RecipeId("salmon-quinoa") }

        assertEquals("Лосось с киноа и брокколи", recipe.name)
        assertEquals(MealType.DINNER, recipe.mealType)
        assertEquals(listOf(RecipeTag.PROTEIN), recipe.tags)
        assertEquals(2, recipe.servings)
        assertEquals(10, recipe.prepMin)
        assertEquals(18, recipe.cookMin)
        assertEquals("Спокойный ужин: жиры из лосося и ровный белок без тяжести.", recipe.dek)
        assertEquals(
            listOf(
                RecipeIngredient(IngredientId("salmon"), 300),
                RecipeIngredient(IngredientId("quinoa"), 300),
                RecipeIngredient(IngredientId("broccoli"), 200),
                RecipeIngredient(IngredientId("oliveoil"), 16),
            ),
            recipe.ingredients,
        )
        assertEquals(
            listOf(
                "Лосось посолите, сбрызните маслом и запеките при 190 °C, 14–16 минут.",
                "Брокколи отварите на пару 5 минут — до яркого зелёного.",
                "Подавайте на тёплой киноа, рыбу сверху.",
            ),
            recipe.steps,
        )
    }

    @Test
    fun lentilTurkeyCarriesItsFullIngredientsAndSteps() {
        val recipe = MockSeed.recipes.first { it.id == RecipeId("lentil-turkey") }

        assertEquals("Тёплая чечевица с индейкой", recipe.name)
        assertEquals(MealType.DINNER, recipe.mealType)
        assertEquals(listOf(RecipeTag.PROTEIN, RecipeTag.GENTLE), recipe.tags)
        assertEquals(2, recipe.servings)
        assertEquals(10, recipe.prepMin)
        assertEquals(20, recipe.cookMin)
        assertEquals("Сытно и мягко: клетчатка чечевицы и постная индейка.", recipe.dek)
        assertEquals(
            listOf(
                RecipeIngredient(IngredientId("turkey"), 240),
                RecipeIngredient(IngredientId("lentils"), 300),
                RecipeIngredient(IngredientId("spinach"), 60),
                RecipeIngredient(IngredientId("oliveoil"), 16),
            ),
            recipe.ingredients,
        )
        assertEquals(
            listOf(
                "Индейку нарежьте кубиками, обжарьте на ложке масла до готовности.",
                "Добавьте варёную чечевицу, прогрейте вместе 3–4 минуты.",
                "В конце вмешайте шпинат, дайте ему опасть. Посолите.",
            ),
            recipe.steps,
        )
    }

    @Test
    fun proteinSmoothieCarriesItsFullIngredientsAndSteps() {
        val recipe = MockSeed.recipes.first { it.id == RecipeId("protein-smoothie") }

        assertEquals("Протеиновый смузи", recipe.name)
        assertEquals(MealType.SNACK, recipe.mealType)
        assertEquals(listOf(RecipeTag.PROTEIN, RecipeTag.GENTLE, RecipeTag.QUICK), recipe.tags)
        assertEquals(1, recipe.servings)
        assertEquals(3, recipe.prepMin)
        assertEquals(0, recipe.cookMin)
        assertEquals("Когда жевать не хочется: белок в стакане, мягко и быстро.", recipe.dek)
        assertEquals(
            listOf(
                RecipeIngredient(IngredientId("whey"), 30),
                RecipeIngredient(IngredientId("kefir"), 200),
                RecipeIngredient(IngredientId("banana"), 80),
                RecipeIngredient(IngredientId("peanut"), 15),
            ),
            recipe.ingredients,
        )
        assertEquals(
            listOf(
                "Сложите всё в блендер.",
                "Взбейте до гладкости, 30 секунд.",
                "Густо — добавьте воды или льда по вкусу.",
            ),
            recipe.steps,
        )
    }
}

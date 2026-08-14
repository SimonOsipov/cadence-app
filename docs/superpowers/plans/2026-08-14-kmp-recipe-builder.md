# Recipe Builder (step-12) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port `mobile/src/features/recipe/RecipeBuilderScreen.tsx`'s builder half to
`app.cadence.screens.recipes.RecipeBuilderScreen` — a modal that assembles a `RecipeDraft`
from a name, meal type, tags, servings, time, picked ingredients and cooking steps.

**Architecture:** The screen owns its form state and nothing else: arithmetic comes from
`shared/domain/RecipeMath.kt`, the ingredient search arrives as a `suspend` lambda, and the
draft leaves through `onSave: (RecipeDraft) -> Unit`. Route wiring (`replaceRoute` into the
saved recipe, `reloads`) is step-13's, exactly as step-9/10/11 left theirs.

**Tech Stack:** Kotlin Multiplatform, Compose Multiplatform 1.11, `runComposeUiTest`.

**Spec:** `docs/specs/kmp-nutrition-and-recipes.md` § step-12 (vault master:
`20-Projects/cadence/specs/kmp-nutrition-and-recipes.md`).

## Global Constraints

- RU copy verbatim from the spec: «Новый рецепт», «Название рецепта», «Тип приёма»,
  «Порций», «Время, мин», «Ингредиенты», «Добавить», «Добавьте первый ингредиент»,
  «Приготовление», «Шаг», «Опишите шаг», «На порцию», «Сохранить рецепт».
- Bounds: servings 1…8 default 2; time 5…120 step 5 default 20; per-row grams 5…600 step 10.
- `cookMin = null`, never `0` (spec's named divergence from `RecipeBuilderScreen.tsx:399-400`).
- Screens take DTOs and lambdas, never a repository.
- No raw colour literals — tokens only (`grep "Color(0x"` over `screens/` finds nothing).
- Comments: exception, not habit — one line where a reader would otherwise get it wrong.
- Gate: `./gradlew ktlintCheck detekt testAndroidHostTest` plus
  `./gradlew :composeApp:iosSimulatorArm64Test`.

---

## File Structure

- `kmp/shared/src/commonMain/kotlin/app/cadence/shared/domain/RecipeMath.kt` — **modify**:
  row-level `totalsOf`/`perServingOf`, with `Recipe.totals`/`Recipe.perServing` delegating.
  The builder has rows and a serving count, never a `Recipe`.
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceSplitBar.kt` — **modify**:
  optional track/label/value colours so the dark «На порцию» card is legible. Defaults
  unchanged, so step-10's call site is untouched.
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/screens/recipes/RecipeChip.kt` — **create**:
  the hug-width pill promoted out of `RecipesScreen.kt` (its second call site), with the
  active tone as a parameter (forest for meal type, sand for tags).
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/screens/recipes/RecipesScreen.kt` —
  **modify**: `RecipesFilterChip` deleted, `RecipesFilterRow` calls `RecipeChip`.
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/screens/recipes/RecipeBuilderScreen.kt` —
  **create**: state, header, name, type/tags, the two steppers, the save bar, sheet hosting.
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/screens/recipes/RecipeBuilderParts.kt` —
  **create**: section headings, the ingredients list, the live per-serving card, the steps
  list. Split for the same reason `RecipeRow.kt` was split out of `RecipesScreen.kt`.
- Tests: `kmp/shared/src/commonTest/.../domain/RecipeMathTest.kt` (modify),
  `kmp/composeApp/src/commonTest/.../screens/recipes/RecipeBuilderScreenTest.kt` (create).

## Layout decisions to be pinned by measurement, not assumed

`CadenceStepper` is `fillMaxWidth()` around two 52dp buttons and an `xl`-padded number:
it needs ≈150dp+ or its plus button is squeezed below its tap target (the step-11 defect).
The prototype puts the two steppers side by side (`:585-647`) and the row stepper inline
beside the name and the delete button (`:716-763`); at 343dp neither fits. Both become
stacked/full-width here, and each is pinned by a bounds assertion at `Modifier.width(343.dp)`.

---

### Task 1: Row-level recipe arithmetic

**Files:**
- Modify: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/domain/RecipeMath.kt`
- Test: `kmp/shared/src/commonTest/kotlin/app/cadence/shared/domain/RecipeMathTest.kt`

**Interfaces:**
- Produces: `fun List<RecipeIngredient>.totalsOf(ingredients: List<Ingredient>): MacrosTenths`,
  `fun List<RecipeIngredient>.perServingOf(ingredients: List<Ingredient>, servings: Int): MacrosTenths`.

- [ ] **Step 1: Write the failing tests**

```kotlin
@Test
fun perServingOfDividesRowTotalsByServings() {
    val rows = listOf(RecipeIngredient(CHICKEN.id, 200), RecipeIngredient(RICE.id, 150))
    val whole = rows.totalsOf(TABLE)
    assertEquals(whole.kcalTenths, rows.perServingOf(TABLE, servings = 1).kcalTenths)
    assertEquals(scaleRounded(whole.kcalTenths, 1, 4), rows.perServingOf(TABLE, servings = 4).kcalTenths)
}

@Test
fun recipePerServingAgreesWithItsOwnRows() {
    val recipe = RECIPE_TWO_SERVINGS
    assertEquals(recipe.perServing(TABLE), recipe.ingredients.perServingOf(TABLE, recipe.servings))
}

@Test
fun perServingOfRejectsZeroServings() {
    assertFailsWith<IllegalArgumentException> { emptyList<RecipeIngredient>().perServingOf(TABLE, servings = 0) }
}
```

Fixture note to carry into the test file: the two rows must have **different** grams and
different per-100g macros, or a mutant that reads `rows[0]` twice survives. Servings 4, not
2, so `scaleRounded` is exercised past a halving that a stray `/2` would also satisfy.

- [ ] **Step 2: Run to verify it fails**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests '*RecipeMathTest*'`
Expected: FAIL, unresolved reference `perServingOf`.

- [ ] **Step 3: Implement**

Move the bodies of `Recipe.totals`/`Recipe.perServing` onto the row list; have the two
`Recipe` extensions delegate (`ingredients.totalsOf(table)` / `ingredients.perServingOf(table, servings)`).
Keep `require(servings > 0)` on the row-level function so both entry points carry it.

- [ ] **Step 4: Run to verify it passes**

Run: `./gradlew :shared:testAndroidHostTest --tests '*RecipeMathTest*'` → PASS.

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(kmp): lift recipe totals onto ingredient rows"
```

---

### Task 2: A dark-legible split bar and a shared chip

**Files:**
- Modify: `kmp/composeApp/.../design/CadenceSplitBar.kt`
- Test: `kmp/composeApp/src/commonTest/.../design/CadenceSplitBarTest.kt`
- Create: `kmp/composeApp/.../screens/recipes/RecipeChip.kt`
- Modify: `kmp/composeApp/.../screens/recipes/RecipesScreen.kt`

**Interfaces:**
- Produces: `CadenceSplitBar(..., trackColor: Color = Cadence.palette.sunk, labelColor: Color = Cadence.palette.subtle, valueColor: Color = Cadence.palette.ink2)`;
  `internal fun RecipeChip(label: String, active: Boolean, onClick: () -> Unit, activeBackground: Color = CadenceColors.forest700, activeForeground: Color = CadenceColors.cream)`.

- [ ] **Step 1: Write the failing test**

```kotlin
@Test
fun theLegendTakesTheColoursItIsGiven() =
    runComposeUiTest {
        setContent {
            CadenceTheme {
                CadenceSplitBar(proteinG = 30.0, carbsG = 20.0, fatG = 10.0, labelColor = CadenceColors.cream)
            }
        }
        onNodeWithText("Белок").assertExists()
    }
```

Colour is not readable from semantics, so the honest assertion is the compile-level one plus
the existing shares tests staying green; the parameter exists so the dark card can pass cream.

- [ ] **Step 2: Run to verify it fails** — unresolved parameter `labelColor`.

- [ ] **Step 3: Implement** the three parameters; move `RecipesFilterChip` verbatim into
`RecipeChip.kt` with the two tone parameters defaulted to its current forest/cream values,
and point `RecipesFilterRow` at it.

- [ ] **Step 4: Run** `./gradlew :composeApp:iosSimulatorArm64Test --tests '*CadenceSplitBarTest*' --tests '*RecipesScreenTest*'` → PASS.

- [ ] **Step 5: Commit**

```bash
git commit -am "refactor(kmp): share the recipe chip and let the split bar take dark tones"
```

---

### Task 3: The builder's frame — header, name, type, tags, steppers, save gate

**Files:**
- Create: `kmp/composeApp/.../screens/recipes/RecipeBuilderScreen.kt`
- Test: `kmp/composeApp/src/commonTest/.../screens/recipes/RecipeBuilderScreenTest.kt`

**Interfaces:**
- Consumes: `RecipeDraft`, `RecipeIngredient`, `Ingredient`, `perServingOf` (Task 1), `RecipeChip` (Task 2).
- Produces:
```kotlin
@Composable
fun RecipeBuilderScreen(
    ingredients: List<Ingredient>,
    search: suspend (String) -> List<Ingredient>,
    modifier: Modifier = Modifier,
    onCancel: () -> Unit = { },
    onSave: (RecipeDraft) -> Unit = { },
)
```
plus tags `CADENCE_RECIPE_BUILDER_TAG`, `_CLOSE_TAG`, `_NAME_TAG`, `_SERVINGS_TAG`,
`_TIME_TAG`, `_SAVE_TAG`, and `recipeBuilderMealTypeTag(MealType)` / `recipeBuilderTagTag(RecipeTag)`.

- [ ] **Step 1: Write the failing tests**

```kotlin
@Test
fun savingIsRefusedWithoutAName() = runComposeUiTest {
    var saved: RecipeDraft? = null
    setContent { CadenceTheme { RecipeBuilderScreen(TABLE, fakeSearch(), onSave = { saved = it }) } }
    addIngredient(CHICKEN)                       // name still empty
    onNodeWithTag(CADENCE_RECIPE_BUILDER_SAVE_TAG).performClick()
    waitForIdle()
    assertNull(saved, "an unnamed draft must not save")
}

@Test
fun savingIsRefusedWithoutIngredients() { /* name typed, zero ingredients, saved stays null */ }

@Test
fun servingsStopAt1And8AndTimeAt5And120() { /* 8 clicks down then one more; 10 up then one more */ }

@Test
fun mealTypeDefaultsToLunchAndIsSingleSelect() { /* LUNCH selected; tap DINNER -> LUNCH not selected */ }

@Test
fun tagsAreMultiSelectAndToggleOff() { /* PROTEIN + QUICK selected; re-tap PROTEIN clears only it */ }
```

- [ ] **Step 2: Run to verify they fail** —
`./gradlew :composeApp:iosSimulatorArm64Test --tests '*RecipeBuilderScreenTest*'`, unresolved reference.

- [ ] **Step 3: Implement** the frame: top bar (close + «Новый рецепт» + hairline), the
name `CadenceTextField`, «Тип приёма» chips row (single select, default `MealType.LUNCH`),
the tag chips (multi select, sand tone), the two stacked stepper cards, and the bottom save
bar (`CadenceButton(enabled = draft.canSave())`).

- [ ] **Step 4: Run** the same filter → PASS.

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(kmp): add the recipe builder's frame"
```

---

### Task 4: Ingredients, the live per-serving card, and steps

**Files:**
- Create: `kmp/composeApp/.../screens/recipes/RecipeBuilderParts.kt`
- Modify: `kmp/composeApp/.../screens/recipes/RecipeBuilderScreen.kt`
- Test: `kmp/composeApp/src/commonTest/.../screens/recipes/RecipeBuilderScreenTest.kt`

**Interfaces:**
- Produces tags `CADENCE_RECIPE_BUILDER_ADD_INGREDIENT_TAG`, `_EMPTY_TAG`, `_PER_SERVING_TAG`,
  `_ADD_STEP_TAG`, and `recipeBuilderIngredientTag(Int)`, `recipeBuilderIngredientRemoveTag(Int)`,
  `recipeBuilderStepTag(Int)`, `recipeBuilderStepRemoveTag(Int)`.

- [ ] **Step 1: Write the failing tests**

```kotlin
@Test
fun theSheetStillOpensAfterTheFirstIngredient() { /* empty state -> add -> «Добавить» header action opens the sheet again */ }

@Test
fun theEmptyStateDisappearsOnceARowExists() { /* _EMPTY_TAG exists, then does not */ }

@Test
fun removingARowRemovesThatRowAndNotItsNeighbour() { /* two different ingredients, delete index 0, the other's name and grams remain */ }

@Test
fun thePerServingCardFollowsBothGramsAndServings() {
    // CHICKEN 100 g at 2 servings -> «83 ккал»; servings to 1 -> «165 ккал»;
    // grams +100 -> «330 ккал» — three distinct numbers, so neither input can be ignored.
}

@Test
fun theLastStepCannotBeRemoved() { /* one step: no remove control; add a second: both removable */ }

@Test
fun aRowsGramsStepperStopsAt5And600() { /* both ends, scoped to the row's own container */ }
```

- [ ] **Step 2: Run to verify they fail.**

- [ ] **Step 3: Implement** the sections: heading + action row, dashed empty state that opens
the sheet, per-row name/kcal/protein line with its own grams stepper and delete, the
forest800 «На порцию» card built from `perServingOf` + `CadenceSplitBar` with cream tones,
and the steps list (numbered badge, multiline field, delete when more than one).

- [ ] **Step 4: Run** → PASS.

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(kmp): add the builder's ingredients, live macros and steps"
```

---

### Task 5: The saved draft, and the two width measurements

**Files:**
- Modify: `kmp/composeApp/.../screens/recipes/RecipeBuilderScreen.kt`
- Test: `kmp/composeApp/src/commonTest/.../screens/recipes/RecipeBuilderScreenTest.kt`

- [ ] **Step 1: Write the failing tests**

```kotlin
@Test
fun theSavedDraftCarriesEveryFieldTheFormCollected() {
    // name «  Ужин на двоих » -> trimmed; DINNER (not the LUNCH default);
    // tags [PROTEIN, QUICK]; servings 3 (not the default 2); prepMin 25 (not 20);
    // cookMin null (not 0); two ingredients at their own grams;
    // steps ["Сварить", "Подать"] with a blank third dropped;
    // dek == "«{белок} г белка · {ккал} ккал на порцию.»" computed from perServingOf.
    assertEquals(expectedDraft, saved)
}

@Test
fun thePlusButtonsStay52dpAt343dp() {
    // servings stepper and the first ingredient row's stepper, both measured
    // under Modifier.width(343.dp) — the step-11 squeeze, in two new places.
}
```

- [ ] **Step 2: Run to verify they fail** (`cookMin` and the trim are the likely first reds).

- [ ] **Step 3: Implement** the draft assembly in `onSave`, and stack whatever the
measurement says is squeezed.

- [ ] **Step 4: Run the whole file** → PASS.

- [ ] **Step 5: Full gate**

```bash
./scripts/gate/kmp.sh && cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test
```

- [ ] **Step 6: Commit**

```bash
git commit -m "feat(kmp): save the recipe draft the builder collected"
```

---

## Deviations to record in the spec (step-12) and in step-15's register

1. The two stepper cards are stacked, not side by side (`:585-647`), and each ingredient
   row's grams stepper sits on its own line under the name (`:716-763`) — measured, not
   assumed: `CadenceStepper` needs ≈150dp and gets ~130dp in the prototype's arrangement.
2. The «Сохранённый рецепт открывается сразу…» test named by step-12 is shell-level and
   lands in step-13: `CadenceRoute.RecipeBuilder` is still a `PlaceholderScreen` until then.
3. The save bar has no gradient fade (`:897-928`) — solid `palette.bg`, as
   `RecipeDetailScreen`'s bottom bar already does.

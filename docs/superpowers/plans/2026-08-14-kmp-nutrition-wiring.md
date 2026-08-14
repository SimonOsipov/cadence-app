# Nutrition Wiring (step-13) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the five `PlaceholderScreen`s of the nutrition port with the real screens,
and make the confirmation toast name the day's total **including** the meal it confirms.

**Architecture:** `CadenceApp` keeps every repository read; `CadenceShell` keeps every route.
The screens already take DTOs and lambdas, so this step only carries values in and events out.

**Spec:** `docs/specs/kmp-nutrition-and-recipes.md` § step-13.

## Global Constraints

- `PLACEHOLDER_MEAL_NAME` and every one of the five placeholders are gone by the end.
- The toast reads `MealLogResult.Written.dayTotals`, never the `summary` snapshot: `summary`
  is refreshed by `LaunchedEffect(reloads)` **after** the toast is raised.
- `reloads` is bumped after a meal write and after a recipe save.
- Parse and ingredient search reach the screens as `suspend` lambdas.
- Screens never receive a repository or the `NavHostController`.
- Gate: `./scripts/gate/kmp.sh` plus `./gradlew :shared:iosSimulatorArm64Test :composeApp:iosSimulatorArm64Test`.

## The parameter decision the spec asks for

`CadenceShell` carries 14 parameters today under `@Suppress("LongParameterList")`, and this
step adds eight more. They are grouped into two holders rather than listed:

- `CadenceShellData` — everything read from a repository, plus `zone`. All fields default to
  the "not loaded yet" value, so a test can build the one field it cares about.
- `CadenceShellActions` — every lambda, each defaulting to a no-op.

Chosen over one holder because the two have different lifetimes (data is re-read on every
`reloads` bump; actions are stable) and different test needs. `CadenceNavigationTest`'s four
call sites are the proof: three pass nothing at all, one passes a single lambda.

---

### Task 1: Group the shell's parameters

**Files:**
- Modify: `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/CadenceShell.kt`
- Modify: `kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/CadenceNavigationTest.kt`

**Interfaces:**
- Produces: `data class CadenceShellData(summary, todayMeals, schedule, cabinet, trends, nutritionDay, nutritionWeek, recipes, ingredients, zone)`,
  `data class CadenceShellActions(onOpenActions, onMealSaved, onDoseLogged, onAddVial, onOpenVial, onTrendWindow, onLoadMetric, parseMeal, searchIngredients, onRecipeSaved)`,
  `CadenceShell(navController, modifier, data, actions, trendWindow)`.

- [ ] **Step 1: Refactor under the existing tests** — no behaviour change, so the failing-test
      step belongs to Task 2. Run `--tests '*CadenceNavigationTest*' --tests '*CadenceShellDataTest*'`
      before and after; both must stay green.
- [ ] **Step 2: Commit** `refactor(kmp): group the shell's parameters into data and actions`.

---

### Task 2: The «Питание» tab, «Рецепты» and the recipe card

**Files:**
- Modify: `CadenceShell.kt` (tab route + two pushed routes), `CadenceApp` (two more reads)
- Test: `kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/CadenceShellDataTest.kt`

- [ ] **Step 1: Write the failing tests**

```kotlin
@Test
fun theNutritionTabDrawsTheScreenAndKeepsTheTabBar() = runComposeUiTest {
    setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }
    onNodeWithTag(cadenceTabTag(CadenceDestination.NUTRITION)).performClick()
    waitForIdle()
    onNodeWithTag(CADENCE_NUTRITION_TAG).assertExists()
    onNodeWithTag(CADENCE_TAB_BAR_TAG).assertExists()
}

@Test
fun theRecipesRouteOpensFromNutritionAndListsTheLibrary() { /* «Рецепты» → CADENCE_RECIPES_TAG, a seeded recipe by name */ }

@Test
fun openingARecipeShowsItsOwnDetail() { /* row → CADENCE_RECIPE_DETAIL_TAG with that recipe's name */ }
```

- [ ] **Step 2: Run to verify they fail** — the placeholder draws instead.
- [ ] **Step 3: Implement.** `CadenceApp` reads `nutrition.day`/`nutrition.week`/`recipes.library`/
      `recipes.ingredients("")` inside `LaunchedEffect(reloads)`; the tab composes `NutritionScreen`
      when the day has landed, the two pushed routes compose `RecipesScreen` and
      `RecipeDetailScreen`, the latter resolving its id through `RecipeDetailState`.
- [ ] **Step 4: Run** → PASS. **Step 5: Commit.**

---

### Task 3: «Записать приём» and the toast's ripple

**Files:** `CadenceShell.kt`, `CadenceShellDataTest.kt`

- [ ] **Step 1: Write the failing tests**

```kotlin
@Test
fun theToastNamesTheDayTotalIncludingTheMealJustLogged() {
    // Seeded day is 840 kcal. Log a parsed meal, then read the toast: it must name the
    // sum *after* the write, which is the mutation «toast reads the pre-write snapshot».
}

@Test
fun aLoggedMealJoinsTheDay() { /* the meal count on «Сегодня» goes from 2 to 3 */ }
```

- [ ] **Step 2: Run to verify they fail** (the placeholder logs a constant).
- [ ] **Step 3: Implement.** `modal<CadenceRoute.LogMeal>` composes `LogMealScreen`;
      `onSave` calls `nutrition.log(draft)`, and on `Written` raises the toast from
      `result.dayTotals`, bumps `reloads`, and pops. Delete `PLACEHOLDER_MEAL_NAME`.
- [ ] **Step 4: Run** → PASS. **Step 5: Commit.**

---

### Task 4: The builder's save and «Добавить в день»

**Files:** `CadenceShell.kt`, `CadenceShellDataTest.kt`

- [ ] **Step 1: Write the failing tests**

```kotlin
@Test
fun savingARecipeOpensItAndReturnsToTheLibrary() {
    // «+ Создать» → name + one ingredient → «Сохранить рецепт» → the detail of the saved
    // recipe; pressing back lands on «Рецепты», not on the builder.
}

@Test
fun aSavedRecipeAppearsUnderMyRecipes() { /* «Мои рецепты» section after the save — the reloads bump */ }

@Test
fun addingARecipeToTheDayLogsItAndReturnsToNutrition() { /* meal count grows, toast is up, «Питание» is on screen */ }
```

- [ ] **Step 2: Run to verify they fail.**
- [ ] **Step 3: Implement.** `modal<CadenceRoute.RecipeBuilder>` composes `RecipeBuilderScreen`
      with `searchIngredients`; `onSave` calls `recipes.save(draft)` and on `Saved(id)` bumps
      `reloads` and `replaceRoute(CadenceRoute.RecipeDetail(id.raw))`. `RecipeDetailScreen`'s
      `onAddToDay` logs the draft the same way Task 3 does and returns to the nutrition tab.
- [ ] **Step 4: Run the whole gate.** **Step 5: Commit.**

---

## What this step must not quietly do

- Substitute a canned parse for the photo or voice mode (nutrition invariant 5).
- Raise the toast from `summary` (that is the defect the step exists to prevent).
- Leave `RecipeDetail` unable to say «не найден» for an id no recipe carries.

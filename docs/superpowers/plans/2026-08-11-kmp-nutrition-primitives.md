# Примитивы питания в дизайн-системе — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Завести семь недостающих примитивов дизайн-системы, на которых стоят все пять экранов питания и рецептов, а два из них — ещё и порты дневника/тела и чата/профиля.

**Architecture:** Каждый примитив следует правилу, выведенному в этой базе трижды: **холст ничего не утверждает**. Геометрия живёт в чистой функции рядом с композаблом (`gaugeFraction`, `syringeFillFraction`, `barFraction` — образцы), а сам композабл рисует **разложенные боксы** с `testTag`, ширину которых можно измерить, а не `Canvas`, пиксели которого не проверяет никто. Тест ставится на функцию и на измеренные границы узла.

**Tech Stack:** Kotlin Multiplatform, Compose Multiplatform, `kotlin.test`, `runComposeUiTest`.

## Global Constraints

- Файлы: `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/`, тесты — `kmp/composeApp/src/commonTest/kotlin/app/cadence/design/`.
- Токены только из темы: `CadenceSpacing.{xxs,xs,sm,md,lg,xl,xxl,huge}`, `CadenceRadius.{xs,sm,md,lg,xl,xxl,pill}`, `Cadence.palette.*`, `CadenceColors.*`. Новых цветов и размеров не заводить.
- Вся продуктовая копия — по-русски. Код, комментарии и коммиты — по-английски.
- KDoc объясняет **почему**, а не что: ссылка на прототип или на мутацию, против которой стоит решение. Образец — `CadenceGauge.kt:39-46`.
- `detekt` настроен на `maxIssues: 0`; `MagicNumber` в пакете `design` исключён, поэтому размеры допустимы как приватные `val`, но каждый — с комментарием-источником.
- Максимальная длина строки 120 (`kmp/.editorconfig`).
- Composable-функции именуются с большой буквы (ktlint это разрешает через `ktlint_function_naming_ignore_when_annotated_with = Composable`).
- **Гейт после каждой задачи:** `cd kmp && ./gradlew ktlintCheck detekt` и `./gradlew :composeApp:iosSimulatorArm64Test`. Compose-тесты идут только на iOS-таргете — `testAndroidHostTest` резолвится в `:shared` и этих тестов не увидит.
- Тесты кладутся в **один** файл `NutritionPrimitivesTest.kt`, как `GaugeTest.kt` держит и `CadenceGauge`, и `CadenceUsageRow`.

---

### Task 1: CadenceStepper

Числовой степпер с дробным шагом. Четыре места вызова в этой спеке и пять в `kmp-journal-and-body` (вес и процент жира шагом 0,1) — поэтому `Double`, а не `Int`.

**Files:**
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceNumericStepper.kt`
- Test: `kmp/composeApp/src/commonTest/kotlin/app/cadence/design/NutritionPrimitivesTest.kt`

**Interfaces:**
- Consumes: `CadenceIcons.plus`, `CadenceNumber`, `Cadence.palette`, `CadenceSpacing`, `CadenceRadius`.
- Produces: `fun steppedValue(value: Double, delta: Double, min: Double, max: Double?, decimals: Int): Double`; `@Composable fun CadenceStepper(value: Double, onChange: (Double) -> Unit, min: Double, max: Double?, step: Double, decimals: Int, modifier: Modifier = Modifier, unit: String? = null)`; константы `CADENCE_STEPPER_MINUS_TAG`, `CADENCE_STEPPER_PLUS_TAG`.

  **Порядок параметров исправлен на этапе исполнения:** `modifier` идёт перед необязательными, как требует конвенция Compose и как устроены `CadenceGauge` и `CadenceUsageRow`. Первая редакция плана ставила `unit` перед `modifier`; девять мест вызова в двух спеках зовут его именованными аргументами, так что порядок на них не влияет, но запись приведена к отгруженной подписи.

- [ ] **Step 1: Write the failing test**

```kotlin
    @Test
    fun theStepperClampsAtBothEndsAndStandsStillOnTheBoundary() {
        // Against the mutation «bounded at one end only»: a floor-only stepper
        // passes every assertion about the floor.
        assertEquals(5.0, steppedValue(value = 5.0, delta = -10.0, min = 5.0, max = 600.0, decimals = 0))
        assertEquals(600.0, steppedValue(value = 600.0, delta = 10.0, min = 5.0, max = 600.0, decimals = 0))
        assertEquals(15.0, steppedValue(value = 5.0, delta = 10.0, min = 5.0, max = 600.0, decimals = 0))
    }

    @Test
    fun aStepperWithNoCeilingGrowsWithoutOne() {
        // The grams of a parsed meal item have a floor of 5 and no ceiling
        // (`LogMealScreen.tsx:880`); a shared ceiling would cap it silently.
        assertEquals(10_000.0, steppedValue(value = 9_990.0, delta = 10.0, min = 5.0, max = null, decimals = 0))
    }

    @Test
    fun aTenthOfAStepDoesNotAccumulateBinaryError() {
        // 0.1 + 0.2 is 0.30000000000000004; body fat steps by a tenth and is
        // read as a tenth. Scaled integer arithmetic, like `DOSE_SCALE`.
        var v = 20.0
        repeat(times = 30) { v = steppedValue(v, delta = 0.1, min = 20.0, max = 55.0, decimals = 1) }
        assertEquals(23.0, v)
    }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.design.NutritionPrimitivesTest"`
Expected: FAIL — `Unresolved reference: steppedValue`.

- [ ] **Step 3: Write minimal implementation**

```kotlin
/**
 * The next value, clamped and rounded.
 *
 * A function beside the composable because the clamping is the whole contract
 * and a drawing asserts nothing. Scaled integer arithmetic rather than raw
 * `Double` addition: a tenth of a gram accumulates binary error the way
 * `CadenceDoseStepper` documents at `DOSE_SCALE`, and the number the patient
 * reads must be the number that is stored.
 *
 * `max = null` is «no ceiling», not «no limit worth writing»: the grams of a
 * parsed meal item have a floor and no top (`LogMealScreen.tsx:880`), while the
 * ingredient sheet caps at 600.
 */
fun steppedValue(
    value: Double,
    delta: Double,
    min: Double,
    max: Double?,
    decimals: Int,
): Double {
    val scale = TEN.pow(decimals)
    val next = round((value + delta) * scale) / scale
    val floored = next.coerceAtLeast(min)

    return if (max == null) floored else floored.coerceAtMost(max)
}

private const val TEN = 10.0
```

(`import kotlin.math.pow`, `import kotlin.math.round`.)

- [ ] **Step 4: Run test to verify it passes**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.design.NutritionPrimitivesTest"`
Expected: PASS.

- [ ] **Step 5: Write the composable and its measured test**

```kotlin
    @Test
    fun theStepperReportsTheSteppedValueAndNotTheDelta() =
        runComposeUiTest {
            var reported = 0.0
            setContent {
                CadenceTheme {
                    CadenceStepper(
                        value = 100.0, onChange = { reported = it },
                        min = 5.0, max = 600.0, step = 10.0, decimals = 0, unit = "г",
                    )
                }
            }

            onNodeWithTag(CADENCE_STEPPER_PLUS_TAG).performClick()

            assertEquals(110.0, reported, "the stepper reported a delta rather than a value")
        }
```

```kotlin
const val CADENCE_STEPPER_MINUS_TAG = "cadence-stepper-minus"
const val CADENCE_STEPPER_PLUS_TAG = "cadence-stepper-plus"

/** Reports a value, never a delta: the caller stores what it is shown. */
@Composable
fun CadenceStepper(
    value: Double,
    onChange: (Double) -> Unit,
    min: Double,
    max: Double?,
    step: Double,
    decimals: Int,
    modifier: Modifier = Modifier,
    unit: String? = null,
) {
    Row(
        modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.Center,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        StepperButton("Уменьшить", CADENCE_STEPPER_MINUS_TAG) {
            onChange(steppedValue(value, -step, min, max, decimals))
        }
        CadenceNumber(
            value = formatDecimal(value, decimals),
            unit = unit.orEmpty(),
            modifier = Modifier.padding(horizontal = CadenceSpacing.xl),
        )
        StepperButton("Увеличить", CADENCE_STEPPER_PLUS_TAG, CadenceIcons.plus) {
            onChange(steppedValue(value, step, min, max, decimals))
        }
    }
}
```

`StepperButton` — приватная копия формы из `CadenceStepper.kt:113-138`, но с `testTag` и без привязки к дозе. Минус рисуется полосой, а не иконкой: в `CadenceIcons` нет «минуса», и `CadenceIcon` для неизвестного имени не рисует ничего — эта ошибка в проекте уже была.

- [ ] **Step 6: Run the gate**

Run: `cd kmp && ./gradlew ktlintCheck detekt :composeApp:iosSimulatorArm64Test`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceNumericStepper.kt \
        kmp/composeApp/src/commonTest/kotlin/app/cadence/design/NutritionPrimitivesTest.kt \
        docs/specs/kmp-nutrition-and-recipes.md docs/superpowers/plans/
git commit -m "feat(kmp): a numeric stepper with a fractional step and an optional ceiling"
```

Снапшот спеки и этот план входят в первый коммит шага — они ещё не в репозитории.

---

### Task 2: CadenceMacroBar

Полоса «значение против цели» с подписью. Три места, все на экране «Питание» (`NutritionScreen.tsx:517-519`), и цвета у трёх полос разные — поэтому цвет параметром.

**Files:**
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceMacroBar.kt`
- Modify: `kmp/composeApp/src/commonTest/kotlin/app/cadence/design/NutritionPrimitivesTest.kt`

**Interfaces:**
- Produces: `fun macroFraction(value: Double, goal: Double): Float`; `@Composable fun CadenceMacroBar(label: String, value: Double, goal: Double, unit: String, color: Color, modifier: Modifier)`; `const val CADENCE_MACRO_TRACK_TAG`, `fun macroFillTag(label: String): String`.

- [ ] **Step 1: Write the failing test**

```kotlin
    @Test
    fun theMacroFractionIsClampedAtBothEndsAndSurvivesAZeroGoal() {
        assertEquals(0.5f, macroFraction(value = 70.0, goal = 140.0))
        assertEquals(1f, macroFraction(value = 200.0, goal = 140.0), "a goal beaten is not a bar overflowing")
        assertEquals(0f, macroFraction(value = -1.0, goal = 140.0))
        assertEquals(0f, macroFraction(value = 70.0, goal = 0.0), "a zero goal divides")
    }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.design.NutritionPrimitivesTest"`
Expected: FAIL — `Unresolved reference: macroFraction`.

- [ ] **Step 3: Write minimal implementation**

```kotlin
/**
 * Value against goal, as a fraction.
 *
 * Clamped at the top because the prototype does (`Math.min(1, v / goal)`): a
 * patient over their protein goal reads a full bar, not a bar past its track.
 * A zero goal draws nothing rather than dividing — targets are nullable in the
 * domain and a patient without them is a real state.
 */
fun macroFraction(
    value: Double,
    goal: Double,
): Float = if (goal <= 0.0) 0f else (value / goal).toFloat().coerceIn(0f, 1f)
```

- [ ] **Step 4: Run test to verify it passes**

Run: as above. Expected: PASS.

- [ ] **Step 5: Write the composable and its measured test**

```kotlin
    @Test
    fun theMacroFillIsAsWideAsItsFraction() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceMacroBar("белок", value = 35.0, goal = 140.0, unit = "г", color = CadenceColors.forest700)
                }
            }

            val track = onNodeWithTag(CADENCE_MACRO_TRACK_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
            val fill = onNodeWithTag(macroFillTag("белок"), useUnmergedTree = true).fetchSemanticsNode().boundsInRoot

            assertTrue(abs(fill.width / track.width - 0.25f) < TOLERANCE, "35 of 140 is not a quarter of the track")
        }
```

Композабл: `Row` с подписью слева и «{значение} / {цель} {единица}» справа, под ними трек высотой 5.dp (`NutritionScreen.tsx` рисует `height: 5`), закруглённый `CadenceRadius.pill`, фон `Cadence.palette.sunk`, заливка `fillMaxWidth(fraction)` цветом параметра, оба с тегами.

- [ ] **Step 6: Run the gate** — `cd kmp && ./gradlew ktlintCheck detekt :composeApp:iosSimulatorArm64Test`

- [ ] **Step 7: Commit**

```bash
git add kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceMacroBar.kt \
        kmp/composeApp/src/commonTest/kotlin/app/cadence/design/NutritionPrimitivesTest.kt
git commit -m "feat(kmp): a macro bar that measures a value against its goal"
```

---

### Task 3: CadenceSplitBar

Составная полоса вклада: белок, углеводы и жиры в **калориях**, по правилу 4/4/9 ккал на грамм. Два места — карточка рецепта и живая карточка конструктора.

**Files:**
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceSplitBar.kt`
- Modify: `NutritionPrimitivesTest.kt`

**Interfaces:**
- Produces: `fun splitShares(proteinG: Double, carbsG: Double, fatG: Double): Triple<Float, Float, Float>`; `@Composable fun CadenceSplitBar(proteinG: Double, carbsG: Double, fatG: Double, modifier: Modifier)`; `fun splitSegmentTag(macro: String): String`.

- [ ] **Step 1: Write the failing test**

```kotlin
    @Test
    fun theSplitIsByCaloriesAndNotByGrams() {
        // 10 g of fat is 90 kcal against 10 g of protein's 40: a split drawn by
        // grams would give three equal thirds and look right on a fixture where
        // the grams happen to match. This is the mutation the test exists for.
        val (p, c, f) = splitShares(proteinG = 10.0, carbsG = 10.0, fatG = 10.0)

        assertTrue(abs(p - 40f / 170f) < TOLERANCE, "protein is $p, not 40 of 170 kcal")
        assertTrue(abs(c - 40f / 170f) < TOLERANCE, "carbs are $c")
        assertTrue(abs(f - 90f / 170f) < TOLERANCE, "fat is $f, not 90 of 170 kcal")
    }

    @Test
    fun anEmptyMealSplitsIntoNothingRatherThanDividing() {
        assertEquals(Triple(0f, 0f, 0f), splitShares(0.0, 0.0, 0.0))
    }
```

- [ ] **Step 2: Run test to verify it fails** — `Unresolved reference: splitShares`.

- [ ] **Step 3: Write minimal implementation**

```kotlin
/** §03's rule of thumb, the prototype's `p*4 / c*4 / f*9`. */
private const val KCAL_PER_G_PROTEIN = 4.0
private const val KCAL_PER_G_CARBS = 4.0
private const val KCAL_PER_G_FAT = 9.0

/**
 * The three shares of a meal's energy, by calories rather than by grams.
 *
 * Grams would draw three equal segments for equal grams, which is wrong by a
 * factor of more than two on fat — and right on any fixture whose grams happen
 * to be equal, which is how such a bug survives a suite.
 */
fun splitShares(
    proteinG: Double,
    carbsG: Double,
    fatG: Double,
): Triple<Float, Float, Float> {
    val p = proteinG * KCAL_PER_G_PROTEIN
    val c = carbsG * KCAL_PER_G_CARBS
    val f = fatG * KCAL_PER_G_FAT
    val total = p + c + f

    if (total <= 0.0) return Triple(0f, 0f, 0f)

    return Triple((p / total).toFloat(), (c / total).toFloat(), (f / total).toFloat())
}
```

- [ ] **Step 4: Run test to verify it passes.**

- [ ] **Step 5: Write the composable and its measured test** — `Row` из трёх `Box` с `weight(share)`, каждый со своим тегом и цветом (`forest700`, `#a5773d`-эквивалент из палитры, `sand700`); под полосой легенда «Белок / Углев / Жиры» с граммами. Тест измеряет ширину сегмента жира против ширины полосы и ждёт `90/170`.

- [ ] **Step 6: Run the gate.**

- [ ] **Step 7: Commit** — `feat(kmp): a split bar that divides a meal by calories, not by grams`

---

### Task 4: CadenceRings

Два концентрических кольца: внешнее — калории, внутреннее — белок, с текстом в центре.

**Files:**
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceRings.kt`
- Modify: `NutritionPrimitivesTest.kt`

**Interfaces:**
- Produces: `fun ringSweep(value: Double, goal: Double): Float` (доля 0..1); `@Composable fun CadenceRings(kcal: Double, kcalGoal: Double, proteinG: Double, proteinGoal: Double, modifier: Modifier)`.

- [ ] **Step 1: Write the failing test**

```kotlin
    @Test
    fun theTwoRingsMeasureTwoDifferentThings() {
        // The fixture is deliberately lopsided: 900 of 1800 kcal is a half,
        // 35 of 140 g of protein is a quarter. On a fixture where both are the
        // same, «both rings read calories» passes ([[a-passing-assert-may-pass-for-another-reason]]).
        assertEquals(0.5f, ringSweep(value = 900.0, goal = 1800.0))
        assertEquals(0.25f, ringSweep(value = 35.0, goal = 140.0))
    }

    @Test
    fun aRingIsClampedAndSurvivesAZeroGoal() {
        assertEquals(1f, ringSweep(value = 2400.0, goal = 1800.0))
        assertEquals(0f, ringSweep(value = 900.0, goal = 0.0))
    }
```

- [ ] **Step 2: Run test to verify it fails.**

- [ ] **Step 3: Write minimal implementation** — `ringSweep` идентичен по форме `macroFraction`, но объявлен отдельно: у колец своя семантика и свой потребитель, а склейка двух функций в одну свяжет два экрана одной правкой.

- [ ] **Step 4: Run test to verify it passes.**

- [ ] **Step 5: Write the composable and its measured test.** Дуги рисуются `Canvas` — окружность боксом не выразить, — **но доля берётся из `ringSweep`**, и тест ставится на неё, а не на рисунок. Дополнительно: центральный текст (надзаголовок «ККАЛ» и процент) — обычные узлы, и тест утверждает, что процент считается от калорий, а не от белка:

```kotlin
    @Test
    fun theCentreReadsTheCalorieRingAndNotTheProteinOne() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceRings(kcal = 900.0, kcalGoal = 1800.0, proteinG = 35.0, proteinGoal = 140.0)
                }
            }

            onNodeWithText("50%").assertExists()
        }
```

- [ ] **Step 6: Run the gate.**

- [ ] **Step 7: Commit** — `feat(kmp): two concentric rings, one per metric`

---

### Task 5: CadenceWeekBars

Семь столбцов, пунктир цели с подписью, масштаб `max(значения, цель) × 1,05`, сегодняшний столбец выделен.

**Files:**
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceWeekBars.kt`
- Modify: `NutritionPrimitivesTest.kt`

**Interfaces:**
- Produces: `fun weekScale(values: List<Double>, goal: Double): Double`; `@Composable fun CadenceWeekBars(values: List<Double>, labels: List<String>, goal: Double, goalLabel: String, todayIndex: Int, modifier: Modifier)`; `fun weekBarTag(index: Int): String`, `const val CADENCE_WEEK_ROW_TAG`.

**Note:** расширение `CadenceUsageRow` рассмотрено и отвергнуто — у него нет ни цели, ни пунктира, ни выделенного столбца, а масштаб он берёт по максимуму без запаса. Записать это решение в KDoc.

- [ ] **Step 1: Write the failing test**

```kotlin
    @Test
    fun theScaleGrowsWhenAValueBeatsTheGoal() {
        // Against the mutation «scale by the goal»: a day over the goal would
        // otherwise draw past the top of the chart.
        assertEquals(1890.0, weekScale(values = listOf(1500.0, 1700.0), goal = 1800.0), absoluteTolerance = 0.001)
        assertEquals(2100.0, weekScale(values = listOf(1500.0, 2000.0), goal = 1800.0), absoluteTolerance = 0.001)
    }
```

- [ ] **Step 2: Run test to verify it fails.**

- [ ] **Step 3: Write minimal implementation**

```kotlin
/** The prototype's `Math.max(...values, target) * 1.05`. */
private const val HEADROOM = 1.05

/**
 * The height the tallest thing on the chart is drawn against.
 *
 * The goal participates: a week entirely under it would otherwise scale to its
 * own maximum and draw the dashed goal line off the top.
 */
fun weekScale(
    values: List<Double>,
    goal: Double,
): Double = (values.maxOrNull() ?: 0.0).coerceAtLeast(goal) * HEADROOM
```

- [ ] **Step 4: Run test to verify it passes.**

- [ ] **Step 5: Write the composable and its measured test** — семь `Box` с `fillMaxHeight(value / scale)` и тегами; тест измеряет высоту столбца против высоты ряда и проверяет, что столбец `todayIndex` отличается по цвету от соседа (через `assertExists` на отдельном теге сегодняшнего столбца).

- [ ] **Step 6: Run the gate.**

- [ ] **Step 7: Commit** — `feat(kmp): weekly bars scaled against the goal they are compared to`

---

### Task 6: CadenceSegmented

Сегментированный переключатель. Три режима записи приёма и «На порцию / Всё» в карточке рецепта.

**Files:**
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceSegmented.kt`
- Modify: `NutritionPrimitivesTest.kt`

**Interfaces:**
- Produces: `@Composable fun <T> CadenceSegmented(options: List<T>, selected: T, onSelect: (T) -> Unit, label: (T) -> String, modifier: Modifier)`.

- [ ] **Step 1: Write the failing test**

```kotlin
    @Test
    fun theSegmentedControlReportsTheOptionAndNotItsIndex() =
        runComposeUiTest {
            var picked = "текст"
            setContent {
                CadenceTheme {
                    CadenceSegmented(
                        options = listOf("текст", "фото", "голос"),
                        selected = "текст", onSelect = { picked = it }, label = { it },
                    )
                }
            }

            onNodeWithText("голос").performClick()

            assertEquals("голос", picked, "the control reported a position rather than a value")
        }
```

- [ ] **Step 2: Run test to verify it fails.**

- [ ] **Step 3: Write minimal implementation** — `Row` в пилюле `Cadence.palette.sunk`, каждый сегмент `weight(1f)`, активный получает фон `Cadence.palette.paper` и цвет `ink`, остальные — прозрачный фон и `ink2`. Обобщённый по типу: экраны выбирают перечислениями, а не строками, и `label` отделяет данные от копии.

- [ ] **Step 4: Run test to verify it passes.**

- [ ] **Step 5: Add the disabled case**

```kotlin
    @Test
    fun aDisabledSegmentedControlReportsNothing() =
        runComposeUiTest {
            // «За сколько напоминать» is inert while its parent toggle is off
            // (`SettingsScreen.tsx:358`), and the recipe card's toggle is live —
            // one control, two states.
            var picked = "на порцию"
            setContent {
                CadenceTheme {
                    CadenceSegmented(
                        options = listOf("на порцию", "всё"), selected = "на порцию",
                        onSelect = { picked = it }, label = { it }, enabled = false,
                    )
                }
            }

            onNodeWithText("всё").performClick()

            assertEquals("на порцию", picked, "a disabled control still reported")
        }
```

- [ ] **Step 6: Run the gate.**

- [ ] **Step 7: Commit** — `feat(kmp): a segmented control that reports its option`

---

### Task 7: CadenceTextField

Четыре новых поля: чат разбора, имя рецепта, шаг приготовления, поиск ингредиента. Против двух существующих сырых `BasicTextField` (`DoseSteps.kt:385`, `AddVialScreen.kt:125`).

**Files:**
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceTextField.kt`
- Modify: `NutritionPrimitivesTest.kt`
- Modify: `kmp/composeApp/src/commonMain/kotlin/app/cadence/screens/dose/DoseSteps.kt:385`
- Modify: `kmp/composeApp/src/commonMain/kotlin/app/cadence/screens/inventory/AddVialScreen.kt:125`

**Interfaces:**
- Produces: `@Composable fun CadenceTextField(value: String, onValueChange: (String) -> Unit, placeholder: String, modifier: Modifier, singleLine: Boolean, minLines: Int)`.

- [ ] **Step 1: Write the failing test**

```kotlin
    @Test
    fun thePlaceholderDisappearsOnceThereIsText() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceTextField(value = "", onValueChange = {}, placeholder = "Найти продукт") } }
            onNodeWithText("Найти продукт").assertExists()
        }

    @Test
    fun theFieldShowsWhatItWasGivenRatherThanWhatWasTyped() =
        runComposeUiTest {
            // Against the mutation «the field keeps its own state»: a controlled
            // field whose parent rejects a value must show the rejection.
            setContent { CadenceTheme { CadenceTextField(value = "Творог", onValueChange = {}, placeholder = "Найти продукт") } }

            onNodeWithText("Творог").assertExists()
            onAllNodesWithText("Найти продукт").assertCountEquals(0)
        }
```

- [ ] **Step 2: Run test to verify it fails.**

- [ ] **Step 3: Write minimal implementation** — `Box` с фоном `paper`, рамкой `border` и радиусом `CadenceRadius.md` (форма из `DoseSteps.kt:380-382`), внутри `CadenceMeta(placeholder)` при пустом значении и `BasicTextField` со стилем `Cadence.typography.body`.

- [ ] **Step 4: Run test to verify it passes.**

- [ ] **Step 5: Convert the two existing raw sites** и прогнать их тесты: `DoseWizardTest`, `AddVialScreenTest`. Если конверсия ломает существующее утверждение — это расхождение, и оно записывается, а не чинится молча.

- [ ] **Step 6: Run the gate** — целиком: `cd kmp && ./gradlew ktlintCheck detekt :composeApp:iosSimulatorArm64Test :shared:iosSimulatorArm64Test`

- [ ] **Step 7: Commit** — `feat(kmp): one text field, and the two raw ones folded into it`

---

## Self-Review

**Покрытие шага спеки.** Семь примитивов шага — семь задач. Требование «холст ничего не утверждает» выполнено у всех, кроме `CadenceRings`, где дуга неизбежно рисуется на `Canvas`; там доля вынесена в `ringSweep`, и тест стоит на ней плюс на центральном тексте. Требование «рассмотреть `CadenceUsageRow` прежде, чем заводить `CadenceWeekBars`» выполнено в задаче 5 и записано в KDoc. Требование «два существующих `BasicTextField` переводятся или остаются, и это записывается» выполнено в задаче 7, шаг 5.

**Заглушек нет:** у каждой задачи реальный тест и реальная реализация либо точное описание формы с указанием строки-образца.

**Согласованность типов:** `steppedValue`, `macroFraction`, `splitShares`, `ringSweep`, `weekScale` — пять чистых функций, ни одна не переиспользуется под другим именем. `macroFraction` и `ringSweep` намеренно раздельны, и причина записана.

**Что осталось на исполнителя:** цвет углеводов `#a5773d` из прототипа нужно сопоставить с токеном палитры; если такого нет — это решение (новый токен либо ближайший существующий), и оно записывается расхождением, а не выбирается молча.

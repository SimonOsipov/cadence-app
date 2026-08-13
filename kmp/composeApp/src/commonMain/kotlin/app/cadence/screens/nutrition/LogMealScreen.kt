package app.cadence.screens.nutrition

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceBody
import app.cadence.design.CadenceButton
import app.cadence.design.CadenceButtonKind
import app.cadence.design.CadenceButtonSize
import app.cadence.design.CadenceChip
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIconButton
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSegmented
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTextField
import app.cadence.design.CadenceTitle
import app.cadence.design.cadenceEmphasisedTitle
import app.cadence.format.clockTime
import app.cadence.format.dayAndMonth
import app.cadence.format.weekdayHeadings
import app.cadence.shared.domain.Macros
import app.cadence.shared.domain.MealDraft
import app.cadence.shared.domain.MealItem
import app.cadence.shared.domain.MealSource
import app.cadence.shared.domain.rescaleMealItem
import app.cadence.shared.parsing.MealParseResult
import app.cadence.shared.parsing.mealSamplePrompts
import kotlinx.coroutines.launch
import kotlinx.datetime.LocalDateTime
import kotlinx.datetime.isoDayNumber

// «Что вы ели?» — the meal-log modal, ported from LogMealScreen.tsx.
//
// This file is the screen's shell, its parse states (idle / parsing / parsed
// / failed) and the state that keeps a grams edit rescaling from what was
// actually parsed. The item list, the macro totals strip and the
// «Сохранить» footer draw from [LogMealItemsList.kt] — split out once this
// file's own component (shell + state) approached detekt's `LargeClass`
// threshold, the same way `commonTest/design/` splits one test per primitive.

private const val CLOSE_DESCRIPTION = "Закрыть"
private const val EYEBROW = "Запись приёма"
private const val TITLE_PREFIX = "Что вы "
private const val TITLE_EMPHASIS = "ели?"
private const val EXAMPLE_LABEL = "Пример"
private const val PARSE_LABEL = "Разобрать →"

/** `LogMealScreen.tsx:337` verbatim. */
private const val PARSING_MESSAGE = "Разбираем, что вы ели…"

/**
 * Not in the prototype — `pickSampleForInput`'s `bestScore = -1` start means
 * it never fails, so this copy is new. It names the way out, the same shape
 * the photo/voice refusals below already take, rather than only refusing.
 */
private const val FAILED_MESSAGE = "Не получилось разобрать — можно попробовать ещё раз."

private const val HEARD_LABEL = "Услышали"

/**
 * Not from the prototype: `PhotoInput`/`VoiceInput` (`LogMealScreen.tsx:517-733`)
 * mime a working capture and hand back a canned parse, which nutrition
 * invariant 5 forbids porting. This copy is the step-5 spec's own wording
 * verbatim, `docs/specs/kmp-nutrition-and-recipes.md`'s step-5 section.
 */
private const val PHOTO_UNAVAILABLE = "Распознавание снимка пока не работает — опишите еду текстом"
private const val VOICE_UNAVAILABLE = "Распознавание голоса пока не работает — опишите еду текстом"

/** «Услышали»'s sparkles icon — 16dp beside text, same as `TodayHero.kt:163`'s own inline icon. */
private val SPARKLES_ICON_SIZE = 16.dp
private const val TEXT_FIELD_MIN_LINES = 3

/** The close button's own 40dp box, so the header chip stays centred against it. */
private val HEADER_BALANCE_SIZE = 40.dp

/** The chat field, so a test can reach it under `useUnmergedTree`. */
const val LOG_MEAL_CHAT_FIELD_TAG = "log-meal-chat-field"

/** The gram floor and step a parsed position's inline editor obeys — `LogMealScreen.tsx:855,880`. */
internal const val LOG_MEAL_GRAM_FLOOR = 5.0
internal const val LOG_MEAL_GRAM_STEP = 10.0

/** The three ways of recording a meal — `MODES` in `LogMealScreen.tsx:38-42`. */
enum class LogMealMode(
    val labelRu: String,
) {
    TEXT("Текст"),
    PHOTO("Фото · скоро"),
    VOICE("Голос · скоро"),
}

/**
 * What one parse attempt has settled on.
 *
 * Independent of [LogMealMode]: switching modes replaces this outright with a
 * fresh instance (`LogMealScreen.tsx:62-69`), but a *failed* parse does not —
 * [items], [mealName] and [transcript] are what step-5's own acceptance line
 * means in code: "не получилось… уже разобранные позиции остаются
 * правимыми". [failed] and a previous success coexist on purpose.
 */
private data class MealParseUiState(
    val parsing: Boolean = false,
    val failed: Boolean = false,
    val mealName: String = "",
    val transcript: String = "",
    val items: List<MealItem> = emptyList(),
    /**
     * Bumped on every *successful* parse, never on a failed retry — see
     * [afterParseResult]. [LogMealScreen] keys its editable-position state on
     * this, not on [items]: [MealItem] is a data class, so re-parsing the
     * same text returns a structurally equal list, and a key on content alone
     * would not fire — a second «курица с рисом» after deleting every row
     * would leave the list empty and the footer stuck on «Добавьте
     * что-нибудь» forever, mode-switching the only escape. This field states
     * "a new attempt happened" as a fact independent of what it produced.
     */
    val generation: Int = 0,
) {
    /** Only once a parse has actually settled — not while [parsing] is still true. */
    val showsHeardChip: Boolean get() = transcript.isNotEmpty() && !parsing
}

/**
 * How one parse attempt's outcome updates the state — pulled out of
 * [LogMealScreen] itself so the two-way branch on [MealParseResult] is not
 * also the composable's own cyclomatic complexity. A fresh
 * [MealParseUiState] on success, not [MealParseUiState.copy]: a stale
 * [MealParseUiState.failed] from a *previous* attempt must not survive into a
 * new success. [MealParseUiState.generation] carries forward from `this`
 * (the pre-parse state) precisely because a fresh instance would otherwise
 * reset it to its default and defeat the one thing it exists for.
 */
private fun MealParseUiState.afterParseResult(result: MealParseResult): MealParseUiState =
    when (result) {
        is MealParseResult.Parsed -> {
            MealParseUiState(
                mealName = result.mealName,
                transcript = result.transcript,
                items = result.items,
                generation = generation + 1,
            )
        }

        MealParseResult.Unavailable -> {
            copy(parsing = false, failed = true)
        }
    }

/**
 * One parsed position paired with the values it was parsed at.
 *
 * [original] never changes once a parse lands — every grams edit rescales
 * from it via [rescaleMealItem], not from [current], which is what makes
 * «−10 г, then +10 г» return exactly the numbers the parse produced instead
 * of compounding each edit's own rounding on top of the last. The prototype
 * gets this wrong (`meal/data.ts:140-151` rescales from whatever the item
 * currently holds); this pairing is the port's fix, kept as UI state because
 * [MealParseUiState.items] is overwritten wholesale by the next parse, not
 * edited in place.
 */
private data class LoggedMealItem(
    val original: MealItem,
    val current: MealItem,
)

/**
 * «Что вы ели?» — the meal-log modal (`CadenceRoute.LogMeal`).
 *
 * [parse] arrives as a `suspend` lambda, the same shape `onLoadMetric` takes
 * (`CadenceShell.kt:250`): the screen never reaches into a repository or a
 * parser instance of its own, only into whatever the shell hands it. It is
 * called from here — not wrapped a level up the way `LogDoseModal` wraps
 * `DoseWizard` in `CadenceShell.kt:531-610` — because this screen has no such
 * wrapper: the route names this file directly.
 *
 * [targets] is a DTO, the same shape `MealHero.kt:46-47` takes it in as —
 * this screen reaches into no repository of its own, matching the spec's
 * "экраны берут DTO и лямбды" rule.
 */
@Composable
fun LogMealScreen(
    now: LocalDateTime,
    targets: Macros,
    modifier: Modifier = Modifier,
    parse: suspend (String) -> MealParseResult = { MealParseResult.Unavailable },
    onSave: (MealDraft) -> Unit = { },
    onCancel: () -> Unit = { },
) {
    var mode by remember { mutableStateOf(LogMealMode.TEXT) }
    var chatText by remember { mutableStateOf("") }
    var sampleIndex by remember { mutableStateOf(0) }
    var parseState by remember { mutableStateOf(MealParseUiState()) }
    val samples = remember { mealSamplePrompts() }
    val scope = rememberCoroutineScope()

    // Keyed on `parseState.generation`, not on `parseState.items`: a fresh
    // successful parse (or a mode switch, which resets `parseState` to a
    // fresh instance with `generation = 0`) must reset both the editable
    // positions and which one is mid-edit, and `generation` is what makes
    // that true even when the new parse's items are a content-equal list —
    // `MealItem` is a data class, so re-parsing identical text would
    // otherwise not change the key at all. A failed retry leaves
    // `generation` untouched — see `afterParseResult` — so this key does not
    // fire and the previous parse's edits survive it, which is the same
    // "уже разобранные позиции остаются правимыми" guarantee
    // [MealParseUiState] itself documents.
    var loggedItems by
        remember(parseState.generation) { mutableStateOf(parseState.items.map { LoggedMealItem(it, it) }) }
    var editingIndex by remember(parseState.generation) { mutableStateOf<Int?>(null) }
    val currentItems = loggedItems.map { it.current }

    fun switchMode(next: LogMealMode) {
        if (next == mode) return
        mode = next
        chatText = ""
        parseState = MealParseUiState()
    }

    fun runParse() {
        // Guarded twice: the button below is also disabled on either
        // condition, but a guard here is what makes this function correct on
        // its own rather than correct only alongside one particular caller.
        if (chatText.isBlank() || parseState.parsing) return
        parseState = parseState.copy(parsing = true, failed = false)
        scope.launch { parseState = parseState.afterParseResult(parse(chatText)) }
    }

    fun toggleEdit(index: Int) {
        editingIndex = if (editingIndex == index) null else index
    }

    fun changeGrams(
        index: Int,
        grams: Int,
    ) {
        loggedItems =
            loggedItems.mapIndexed { i, entry ->
                if (i == index) entry.copy(current = rescaleMealItem(entry.original, grams)) else entry
            }
    }

    fun deleteItem(index: Int) {
        loggedItems = loggedItems.filterIndexed { i, _ -> i != index }
        editingIndex = null
    }

    Column(
        modifier
            .fillMaxSize()
            .background(Cadence.palette.bg)
            .windowInsetsPadding(WindowInsets.statusBars)
            .verticalScroll(rememberScrollState()),
    ) {
        LogMealHeader(now, onCancel)
        LogMealTitleBlock()

        LogMealBody(
            mode = mode,
            onSwitchMode = ::switchMode,
            chatText = chatText,
            onChatTextChange = { chatText = it },
            placeholder = samples[sampleIndex].placeholder,
            onCycleSample = {
                sampleIndex = (sampleIndex + 1) % samples.size
                chatText = samples[sampleIndex].transcript
            },
            onParse = ::runParse,
            parseState = parseState,
            targets = targets,
            currentItems = currentItems,
            editingIndex = editingIndex,
            onToggleEdit = ::toggleEdit,
            onGramsChange = ::changeGrams,
            onDelete = ::deleteItem,
            onSave = {
                onSave(MealDraft(name = parseState.mealName, source = MealSource.AI_TEXT, items = currentItems))
            },
        )
    }
}

/**
 * Everything below the title: the mode picker, each mode's own content, the
 * parse-state notices, the item list and the footer. Pulled out of
 * [LogMealScreen] itself so that function's own job — owning the state a
 * grams edit rescales from — is not also its cyclomatic complexity; this
 * composable owns none of that state, only what it takes to render it.
 */
@Suppress("LongParameterList")
@Composable
private fun LogMealBody(
    mode: LogMealMode,
    onSwitchMode: (LogMealMode) -> Unit,
    chatText: String,
    onChatTextChange: (String) -> Unit,
    placeholder: String,
    onCycleSample: () -> Unit,
    onParse: () -> Unit,
    parseState: MealParseUiState,
    targets: Macros,
    currentItems: List<MealItem>,
    editingIndex: Int?,
    onToggleEdit: (Int) -> Unit,
    onGramsChange: (Int, Int) -> Unit,
    onDelete: (Int) -> Unit,
    onSave: () -> Unit,
) {
    Column(
        Modifier.padding(horizontal = CadenceSpacing.lg),
        verticalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
    ) {
        CadenceSegmented(
            options = LogMealMode.entries,
            selected = mode,
            onSelect = onSwitchMode,
            label = { it.labelRu },
        )

        when (mode) {
            LogMealMode.TEXT -> {
                TextModeInput(
                    text = chatText,
                    onTextChange = onChatTextChange,
                    placeholder = placeholder,
                    parsing = parseState.parsing,
                    onCycleSample = onCycleSample,
                    onParse = onParse,
                )
            }

            LogMealMode.PHOTO -> {
                UnavailableModeNotice(PHOTO_UNAVAILABLE)
            }

            LogMealMode.VOICE -> {
                UnavailableModeNotice(VOICE_UNAVAILABLE)
            }
        }

        if (parseState.parsing) ParsingNotice()
        if (parseState.failed) FailedNotice()
        if (parseState.showsHeardChip) HeardChip(parseState.transcript)

        // Hidden rather than drawn empty: an empty parse is unreachable
        // through a canned parser, but deleting every position is not, and
        // «Обед · 0 позиций» over a blank strip is not a state the spec's
        // step-5 section describes.
        if (currentItems.isNotEmpty()) {
            LogMealItemsSection(
                mealName = parseState.mealName,
                items = currentItems,
                targets = targets,
                editingIndex = editingIndex,
                onToggleEdit = onToggleEdit,
                onGramsChange = onGramsChange,
                onDelete = onDelete,
            )
        }

        // Always drawn, even at `idle` — the prototype's footer sits outside
        // every stage branch (`LogMealScreen.tsx:344-394`), inactive until
        // there is something to save rather than absent until there is.
        LogMealFooter(items = currentItems, onSave = onSave)
    }
}

@Composable
private fun LogMealHeader(
    now: LocalDateTime,
    onCancel: () -> Unit,
) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CadenceIconButton(
            icon = CadenceIcons.xMark,
            contentDescription = CLOSE_DESCRIPTION,
            onClick = onCancel,
            background = Cadence.palette.sunk,
        )
        BasicText(
            text = headerChipText(now),
            style = Cadence.typography.number.copy(color = Cadence.palette.subtle, fontSize = 12.sp),
        )
        // Balances the close button's own 40dp box so the chip sits centred,
        // matching the prototype's `<View style={{ width: 40 }} />`.
        Spacer(Modifier.size(HEADER_BALANCE_SIZE))
    }
}

/**
 * «14:05 · Ср 13 августа» — every part read from [now], never a literal. The
 * prototype's «08:42 · вс 24 мая» is fake in both halves
 * (`LogMealScreen.tsx:133`); [weekdayHeadings] already carries the short
 * weekday form this needs, so nothing new was added for that half.
 */
private fun headerChipText(now: LocalDateTime): String {
    val date = now.date
    val weekday = weekdayHeadings()[date.dayOfWeek.isoDayNumber - 1]
    return "${clockTime(now)} · $weekday ${dayAndMonth(date)}"
}

@Composable
private fun LogMealTitleBlock() {
    Column(Modifier.padding(horizontal = CadenceSpacing.xl, vertical = CadenceSpacing.md)) {
        CadenceEyebrow(EYEBROW, Modifier.padding(bottom = CadenceSpacing.xs), color = CadenceColors.sand700)
        CadenceTitle(cadenceEmphasisedTitle(prefix = TITLE_PREFIX, emphasis = TITLE_EMPHASIS))
    }
}

/**
 * The Текст mode's field, «Пример» and «Разобрать →» — `ChatInput` in
 * `LogMealScreen.tsx:404-511`. [CadenceTextField] already draws the bordered
 * card the prototype wraps `ChatInput` in (`paper` background, `hairline`
 * border, `md` radius); nesting a second such box around it here would double
 * that chrome rather than reproduce it.
 */
@Composable
private fun TextModeInput(
    text: String,
    onTextChange: (String) -> Unit,
    placeholder: String,
    parsing: Boolean,
    onCycleSample: () -> Unit,
    onParse: () -> Unit,
) {
    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
        CadenceTextField(
            value = text,
            onValueChange = onTextChange,
            placeholder = placeholder,
            fieldModifier = Modifier.testTag(LOG_MEAL_CHAT_FIELD_TAG),
            minLines = TEXT_FIELD_MIN_LINES,
        )
        Row(
            Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            CadenceChip(label = EXAMPLE_LABEL, onClick = onCycleSample)
            CadenceButton(
                label = PARSE_LABEL,
                onClick = onParse,
                kind = CadenceButtonKind.SECONDARY,
                size = CadenceButtonSize.SMALL,
                // Both halves of "an empty field does not start a parse":
                // disabled dims the same way the prototype's own `hasText ?
                // sand500 : sunk` does, and — unlike the prototype — a
                // disabled CadenceButton also never calls onClick at all.
                enabled = text.isNotBlank() && !parsing,
            )
        }
    }
}

/**
 * «Распознавание снимка / голоса пока не работает — опишите еду текстом» —
 * one line instead of a control, in place of `PhotoInput` /
 * `VoiceInput` (`LogMealScreen.tsx:517-733`), which mime a working capture and
 * hand back a canned parse. Nothing here is pressable: nutrition invariant 5
 * forbids substituting a prepared result for a real one, and the surest way
 * an unimplemented control produces no items is to not be a control.
 */
@Composable
private fun UnavailableModeNotice(text: String) {
    Box(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(CadenceRadius.lg))
            .background(Cadence.palette.sunk)
            .padding(CadenceSpacing.lg),
    ) {
        CadenceBody(text, color = Cadence.palette.muted)
    }
}

@Composable
private fun ParsingNotice() {
    Box(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(CadenceRadius.lg))
            .background(Cadence.palette.sunk)
            .padding(CadenceSpacing.lg),
    ) {
        CadenceMeta(PARSING_MESSAGE, color = Cadence.palette.muted)
    }
}

@Composable
private fun FailedNotice() {
    Box(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(CadenceRadius.lg))
            .background(Cadence.palette.sunk)
            .padding(CadenceSpacing.lg),
    ) {
        CadenceBody(FAILED_MESSAGE, color = Cadence.palette.muted)
    }
}

/** «Услышали» plus the transcript — shown only once a parse has succeeded. */
@Composable
private fun HeardChip(transcript: String) {
    Row(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(CadenceRadius.lg))
            .background(Cadence.palette.sunk)
            .padding(CadenceSpacing.md),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
    ) {
        CadenceIcon(paths = CadenceIcons.sparkles, tint = CadenceColors.sand700, size = SPARKLES_ICON_SIZE)
        Column {
            CadenceMeta(HEARD_LABEL, color = Cadence.palette.subtle)
            CadenceBody(transcript, color = Cadence.palette.ink2)
        }
    }
}

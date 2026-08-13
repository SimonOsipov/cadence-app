package app.cadence.shared.parsing

import app.cadence.shared.domain.MealItem

/**
 * What [MealParser.parse] answers. Two states, not three: the spec's acceptance criterion
 * names three — «разбираем» (parsing), «разобрали» (parsed), «не получилось» (failed) — but
 * the first is the caller's own in-flight state while this suspend function has not yet
 * returned. It never appears here; only the two outcomes a completed call can settle on do.
 */
sealed interface MealParseResult {
    /** A canned meal matched (or was defaulted to) for the given text. */
    data class Parsed(
        val mealName: String,
        val transcript: String,
        val items: List<MealItem>,
    ) : MealParseResult

    /**
     * No parse happened: blank input, or [MockMealParser]'s unavailable mode.
     * Never a substitute for a canned success: nutrition invariant 5 forbids
     * handing back a prepared result in place of a real one, even though the
     * prototype does exactly that for its own photo and voice segments. What
     * M9 replaces is free-text parsing (`POST /me/meals/parse-text`); whether
     * photo and voice ever reach a parser at all is undecided, so this states
     * only what the state is reachable through today.
     */
    data object Unavailable : MealParseResult
}

/**
 * The boundary between free text describing a meal and the [MealItem]s it becomes. A
 * service, not a repository — nothing is stored here, so this lives in its own `parsing`
 * package rather than `shared/repository/`. Until M9's `POST /me/meals/parse-text` exists,
 * [MockMealParser] answers with one of three canned meals, matched by keyword.
 */
interface MealParser {
    suspend fun parse(text: String): MealParseResult
}

package app.cadence.shared.parsing

private const val NO_MATCH_YET = -1

/**
 * Keyword-matched stand-in for M9's `POST /me/meals/parse-text`.
 *
 * Ports `pickSampleForInput` (`mobile/src/features/meal/data.ts:119-127`)
 * verbatim, [NO_MATCH_YET] included: seeding the best score at -1 rather than
 * 0 is why unmatched text still resolves to a canned parse instead of
 * [MealParseResult.Unavailable] — the first entry (breakfast) scores at least
 * zero and that already beats -1, so it always wins in the absence of a
 * better match. Only a strictly higher score replaces the running winner, so
 * ties keep whichever [CANNED_MEAL_PARSES] entry was checked first.
 *
 * [available] is the mode the spec calls for separately: `false` always
 * answers [MealParseResult.Unavailable] — the branch that state is reachable
 * through until M9 supplies a real endpoint, never the keyword matcher, per
 * nutrition invariant 5, which forbids substituting a canned result for
 * either.
 */
class MockMealParser(
    private val available: Boolean = true,
) : MealParser {
    override suspend fun parse(text: String): MealParseResult {
        if (!available || text.isBlank()) return MealParseResult.Unavailable

        val lowered = text.lowercase()
        var best = CANNED_MEAL_PARSES.first()
        var bestScore = NO_MATCH_YET
        for (canned in CANNED_MEAL_PARSES) {
            val score = canned.triggerWords.count { lowered.contains(it) }
            if (score > bestScore) {
                best = canned
                bestScore = score
            }
        }

        return MealParseResult.Parsed(
            mealName = best.mealName,
            transcript = best.transcript,
            items = best.items,
        )
    }
}

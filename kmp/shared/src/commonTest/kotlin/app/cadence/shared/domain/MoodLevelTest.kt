package app.cadence.shared.domain

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

// step-1: the mood scale as data. The prototype carries **two** scales for the same
// `mood smallint 1..5` — «Никак / Слабо / Ровно / Хорошо / Светло» in the dose wizard
// (`log-dose/components.tsx:513`) and «Тяжело / Так себе / Ровно / Хорошо / Светло» in the
// journal (`journal/data.ts:46-53`). One number, two words, depending on which screen the
// patient is looking at. This type is the one scale; the journal's wording wins, because
// the journal is where the number is read back.

class MoodLevelTest {
    /**
     * The whole set, not its size: an invented sixth level, or a renamed one, is exactly what
     * a count check waves through — this codebase has shipped that defect once already.
     */
    @Test
    fun theScaleIsTheFiveNamedLevelsInOrder() {
        assertEquals(
            listOf("Тяжело", "Так себе", "Ровно", "Хорошо", "Светло"),
            MoodLevel.entries.map { it.labelRu },
        )
        assertEquals(listOf(1, 2, 3, 4, 5), MoodLevel.entries.map { it.value })
    }

    @Test
    fun ofResolvesEveryStoredValueAndNothingElse() {
        MoodLevel.entries.forEach { level ->
            assertEquals(level, MoodLevel.of(level.value), "level ${level.value} did not resolve to itself")
        }

        // §03 stores `mood smallint 1..5`; a row outside it is data this app did not write,
        // and «нет отметки» is the honest answer rather than a clamped neighbour.
        assertNull(MoodLevel.of(0))
        assertNull(MoodLevel.of(6))
    }
}

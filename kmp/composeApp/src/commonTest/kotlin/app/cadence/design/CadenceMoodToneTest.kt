package app.cadence.design

import app.cadence.shared.domain.MoodLevel
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull

class CadenceMoodToneTest {
    /**
     * Total against the scale, the same guard `BodyMapTest` keeps over `InjectionSite`: a
     * level added to the domain with no drawing here would render as nothing at all, and a
     * size check would not notice which one went missing.
     */
    @Test
    fun everyLevelOfTheScaleHasItsOwnDrawing() {
        MoodLevel.entries.forEach { level ->
            assertNotNull(CadenceMoodTone.of(level), "level ${level.value} has no tone")
        }
        assertEquals(MoodLevel.entries.toSet(), CadenceMoodTone.entries.map { it.level }.toSet())
        assertEquals(MoodLevel.entries.size, CadenceMoodTone.entries.size, "a level is drawn twice")
    }

    /**
     * The two ends must not collapse into each other: «Тяжело» and «Светло» are the whole
     * point of a coloured scale, and a table that mapped both to the same token would still
     * pass the totality check above.
     */
    @Test
    fun theScaleRunsFromWarningToForestAndTheEndsDiffer() {
        val hard = assertNotNull(CadenceMoodTone.of(MoodLevel.HARD))
        val bright = assertNotNull(CadenceMoodTone.of(MoodLevel.BRIGHT))

        assertEquals(CadenceColors.danger, hard.color)
        assertEquals(CadenceColors.forest700, bright.color)
        assertEquals(
            5,
            CadenceMoodTone.entries
                .map { it.color }
                .toSet()
                .size,
            "two levels share a fill",
        )
        assertEquals(
            5,
            CadenceMoodTone.entries
                .map { it.soft }
                .toSet()
                .size,
            "two levels share a ground",
        )
    }
}

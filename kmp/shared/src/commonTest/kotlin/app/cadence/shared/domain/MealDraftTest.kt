package app.cadence.shared.domain

import kotlin.test.Test
import kotlin.test.assertFalse
import kotlin.test.assertTrue

private fun item() = MealItem(name = "Яблоко", grams = 100, macros = MacrosTenths(500, 3, 140, 2))

class MealDraftTest {
    @Test
    fun aBlankNameRefusesToLogEvenWithASourceAndAnItem() {
        val draft = MealDraft(name = "", source = MealSource.AI_TEXT, items = listOf(item()))

        assertFalse(draft.canLog())
    }

    @Test
    fun noSourceRefusesToLogEvenWithANameAndAnItem() {
        val draft = MealDraft(name = "Обед", source = null, items = listOf(item()))

        assertFalse(draft.canLog())
    }

    @Test
    fun noItemsRefusesToLogEvenWithANameAndASource() {
        val draft = MealDraft(name = "Обед", source = MealSource.AI_TEXT, items = emptyList())

        assertFalse(draft.canLog())
    }

    @Test
    fun aNameASourceAndAnItemCanLog() {
        val draft = MealDraft(name = "Обед", source = MealSource.AI_TEXT, items = listOf(item()))

        assertTrue(draft.canLog())
    }
}

package app.cadence.shared.api

import app.cadence.shared.api.models.DoseBody
import kotlinx.serialization.json.Json
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * What a dose looks like on the wire, which the spec asks to be settled here rather than
 * discovered in M3.
 *
 * The spec expected a trap and there is none: the generator's multiplatform library is known to
 * serialise enums by constant name, and this contract's units are «мг» and «мкг» — but the
 * generated enum carries `@SerialName` per entry, so the value is what travels. Measured rather
 * than reasoned, because both halves of that sentence are the generator's choice and either
 * could change under a version bump.
 */
class DoseOnTheWireTest {
    @Test
    fun aDoseTravelsAsItsUnitAndNotAsAnIdentifier() {
        val json = Json.encodeToString(DoseBody.serializer(), DoseBody(DoseBody.Unit.мг, 0.25))

        assertTrue(json.contains("\"unit\":\"мг\""), "the unit went as $json")
        assertTrue(json.contains("\"value\":0.25"), "the value went as $json")
    }

    // The set and not its size: iterating `entries` cannot see a unit leave the contract, and
    // this project has already paid for that once.
    @Test
    fun theContractAdmitsExactlyTheseTwoUnits() {
        assertEquals(listOf("мг", "мкг"), DoseBody.Unit.entries.map { it.value })
    }

    @Test
    fun everyUnitTheContractAdmitsSurvivesTheRoundTrip() {
        for (unit in DoseBody.Unit.entries) {
            val dose = DoseBody(unit, 1.0)
            val back = Json.decodeFromString(DoseBody.serializer(), Json.encodeToString(DoseBody.serializer(), dose))

            assertEquals(dose, back)
        }
    }

    // The identifier half of the same decision, and it has a cost the Linux gate cannot see:
    // the Cyrillic entry names reach the exported Objective-C header, and clang answers
    // «'swift_name' attribute has invalid identifier for the base name» on every translation
    // unit importing it — measured with xcrun clang -fsyntax-only against the linked framework.
    // Warnings only; Xcode carries no -Werror. `x-enum-varnames` in the schema would buy ASCII
    // names at the same wire values, and that is the schema's decision rather than this file's.
}

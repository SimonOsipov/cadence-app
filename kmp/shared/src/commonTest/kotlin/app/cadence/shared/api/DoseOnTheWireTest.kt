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

    @Test
    fun everyUnitTheContractAdmitsSurvivesTheRoundTrip() {
        for (unit in DoseBody.Unit.entries) {
            val dose = DoseBody(unit, 1.0)
            val back = Json.decodeFromString(DoseBody.serializer(), Json.encodeToString(DoseBody.serializer(), dose))

            assertEquals(dose, back)
        }
    }

    // The number is a Double and that is the generator's mapping of `format: double`, not a
    // choice made here — recorded because §03's own rule is that a dose is {value, unit}, and
    // the day the contract carries a decimal instead, this is where it shows.
    @Test
    fun theValueIsADouble() {
        val dose: DoseBody = DoseBody(DoseBody.Unit.мкг, 250.0)

        assertEquals(250.0, dose.value)
    }
}

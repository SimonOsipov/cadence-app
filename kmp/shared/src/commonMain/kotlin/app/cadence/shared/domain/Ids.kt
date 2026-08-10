package app.cadence.shared.domain

import kotlin.jvm.JvmInline

// Identifiers are typed. §03 hangs eleven contexts off the same handful of
// foreign keys, and a parameter list of Strings is where a patient id ends up
// in a vial id's place with everything still compiling.

@JvmInline
value class UserId(
    val raw: String,
)

@JvmInline
value class CompoundId(
    val raw: String,
)

@JvmInline
value class ProtocolId(
    val raw: String,
)

@JvmInline
value class ProtocolItemId(
    val raw: String,
)

@JvmInline
value class VialId(
    val raw: String,
)

@JvmInline
value class DoseEventId(
    val raw: String,
)

@JvmInline
value class MeasurementId(
    val raw: String,
)

@JvmInline
value class MealId(
    val raw: String,
)

@JvmInline
value class RecipeId(
    val raw: String,
)

@JvmInline
value class IngredientId(
    val raw: String,
)

@JvmInline
value class ThreadId(
    val raw: String,
)

@JvmInline
value class MessageId(
    val raw: Long,
)

@JvmInline
value class ArticleId(
    val raw: String,
)

@JvmInline
value class InviteId(
    val raw: String,
)

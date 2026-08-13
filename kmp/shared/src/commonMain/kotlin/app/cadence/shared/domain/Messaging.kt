package app.cadence.shared.domain

import kotlin.time.Instant

// messaging (§03). The prototypes disagree on sender enums (me/them vs
// doctor/me) and on card nesting; the canonical shape is a real user FK, so
// «who is me» is a client-side comparison and never a stored role.

/** §03's `thread` — `UNIQUE(patient_id, provider_id)`. */
data class MessageThread(
    val id: ThreadId,
    val patientId: UserId,
    val providerId: UserId,
)

enum class MessageKind(
    val code: String,
) {
    TEXT("text"),
    CARD("card"),
    PHOTO("photo"),
}

enum class CardKind(
    val code: String,
) {
    WEIGHT("weight"),
    DOSE("dose"),
    MEAL("meal"),
    PHOTO("photo"),
}

/**
 * Plain strings, not references: keeps the prototype's snapshot semantics — a shared
 * weight card shows the value at share time, not today's number.
 */
data class MessageCard(
    val kind: CardKind,
    val eyebrow: String?,
    val title: String,
    val delta: String?,
    val meta: String?,
    val caption: String?,
)

/** §03's `messages`. The id is a bigserial because ordering rides on it. */
data class Message(
    val id: MessageId,
    val threadId: ThreadId,
    val senderId: UserId,
    val kind: MessageKind,
    val body: String?,
    val card: MessageCard?,
    val photoPath: String?,
    val sentAt: Instant,
    val readAt: Instant?,
)

package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlin.time.Instant

// identity & care — «who exists, who treats whom, who may see what» (§03).

enum class UserRole { PATIENT, DOCTOR, ADMIN }

enum class Sex { FEMALE, MALE }

/**
 * §03's `profiles`. `avatar_bg/fg` don't come over — they're presentation, and the design
 * system already owns colour. Initials do, since the server sends them.
 */
data class Profile(
    val userId: UserId,
    val role: UserRole,
    val fullName: String,
    val initials: String,
    val timeZone: String,
    val locale: String,
    val createdAt: Instant?,
)

/** §03's `patient_profiles`. One `targetWeightKg` — the first correction. */
data class PatientProfile(
    val userId: UserId,
    val dateOfBirth: LocalDate?,
    val sex: Sex?,
    val heightCm: Int?,
    val targetWeightKg: Double?,
    val joinedAt: LocalDate?,
)

/** §03's `provider_profiles`. */
data class ProviderProfile(
    val userId: UserId,
    val titleRu: String,
    val clinicName: String?,
)

/** §03: `care_role endo|dietitian|nurse`. */
enum class CareRole(
    val code: String,
) {
    ENDOCRINOLOGIST("endo"),
    DIETITIAN("dietitian"),
    NURSE("nurse"),
}

/**
 * §03's sixth correction: the mobile prototype has three people, the web one a single
 * doctor, the schema a team. Every doctor's access routes through this, not «role = doctor
 * ⇒ sees all».
 */
data class CareTeamAssignment(
    // §03 gives this row a surrogate id the client never needs: (patient, provider) is
    // UNIQUE there and every read/write is keyed by the pair.
    val patientId: UserId,
    val providerId: UserId,
    val careRole: CareRole,
    val isPrimary: Boolean,
    val since: LocalDate?,
)

enum class InviteStatus { PENDING, ACCEPTED, EXPIRED }

/** §03's `invites`. Onboarding is invite-only; there is no public signup. */
data class Invite(
    val id: InviteId,
    val email: String,
    val role: UserRole,
    val invitedBy: UserId,
    val status: InviteStatus,
    val createdAt: Instant?,
)

enum class MassUnit { KG, LB }

enum class TimeFormat { H24, H12 }

/** §03's `user_preferences`. `lead_time_min 15|30|60` is a closed set there. */
enum class ReminderLeadTime(
    val minutes: Int,
) {
    FIFTEEN(15),
    THIRTY(30),
    SIXTY(60),
}

data class UserPreferences(
    val userId: UserId,
    val doseReminders: Boolean,
    val leadTime: ReminderLeadTime,
    val mealReminders: Boolean,
    val units: MassUnit,
    val timeFormat: TimeFormat,
    val weeklyReport: Boolean,
    val teamMessages: Boolean,
    val reorderAlerts: Boolean,
)

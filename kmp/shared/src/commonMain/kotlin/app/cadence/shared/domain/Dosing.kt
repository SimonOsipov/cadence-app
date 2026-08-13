package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime
import kotlin.time.Instant

// dosing — «the core clinical fact stream» (§03).

/**
 * §03 names only three and writes «(10 zones)»; the rest come from
 * mobile/src/features/log-dose/data.ts. Two invented ones («l-flank», «r-flank») matched no
 * zone the body map can draw and aren't here. Codes are what `dose_events.site_code` stores.
 */
enum class InjectionSite(
    val code: String,
) {
    LEFT_ABDOMEN("l-abdomen"),
    RIGHT_ABDOMEN("r-abdomen"),
    LEFT_DELTOID("l-delt"),
    RIGHT_DELTOID("r-delt"),
    LEFT_GLUTE("l-glute"),
    RIGHT_GLUTE("r-glute"),
    LEFT_THIGH("l-thigh"),
    RIGHT_THIGH("r-thigh"),
    LEFT_LOWER_BACK("l-lback"),
    RIGHT_LOWER_BACK("r-lback"),
}

/** §03's seven side effects, a closed set. */
enum class SideEffect(
    val code: String,
) {
    NAUSEA("nausea"),
    FATIGUE("fatigue"),
    HEADACHE("headache"),
    BLOATING("bloating"),
    INSOMNIA("insomnia"),
    SITE("site"),
    APPETITE("appetite"),
}

/**
 * §03's `dose_events` — the one clinical fact everything else is measured against.
 * `scheduledFor` matches a generated occurrence to a logged event without a schedule table
 * existing. `vialId` makes a vial's remaining count a subtraction, not a column (§03's third
 * correction). The wizard's check-in also writes a `JournalEntry` with `source = DOSE` —
 * one action, two facts.
 */
data class DoseEvent(
    val id: DoseEventId,
    val patientId: UserId,
    val protocolItemId: ProtocolItemId,
    val vialId: VialId?,
    val scheduledForDate: LocalDate,
    val scheduledForTime: LocalTime?,
    val injectedAt: Instant,
    val dose: Dose,
    val site: InjectionSite?,
    val mood: Int?,
    val sideEffects: List<SideEffect>,
    val note: String?,
    val photoPath: String?,
    val createdAt: Instant?,
)

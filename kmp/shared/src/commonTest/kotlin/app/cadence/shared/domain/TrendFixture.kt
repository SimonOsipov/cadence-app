package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalDateTime
import kotlinx.datetime.LocalTime
import kotlinx.datetime.TimeZone
import kotlinx.datetime.toInstant

/**
 * What both trend test classes build their days out of.
 *
 * An object rather than top-level declarations: the sibling tests in this
 * package each keep a file-private `PLAN`, `START` and `reading`, and a second
 * set of those names at package level makes every one of them ambiguous.
 */
internal object TrendFixture {
    /** A real zone rather than UTC, so that «which day is this» is a question. */
    val ZONE = TimeZone.of("Europe/Moscow")

    /** The seed's own start and Sunday, so the numbers here and there are the same numbers. */
    val START = LocalDate(2026, 5, 10)
    val TODAY = LocalDate(2026, 5, 31)

    val PATIENT = UserId("p")

    private val PID = ProtocolId("pr")

    /**
     * A protocol and nothing else.
     *
     * `items` and `phases` are empty because no assertion in either trend test
     * reads them: a window is a function of `startDate` and `weeks`, and a
     * fixture carrying a cadence and a dose would suggest otherwise.
     */
    fun planStartingOn(
        start: LocalDate,
        weeks: Int = 12,
    ): ProtocolPlan =
        ProtocolPlan(
            protocol = Protocol(PID, PATIENT, start, weeks, ProtocolStatus.ACTIVE, null, null),
            items = emptyList(),
            phases = emptyMap(),
        )

    val PLAN = planStartingOn(START)

    fun reading(
        metric: Metric,
        date: LocalDate,
        value: Double,
        at: LocalTime = LocalTime(6, 0),
        id: String = "m-${metric.code}-$date-$at",
    ): Measurement =
        Measurement(
            id = MeasurementId(id),
            patientId = PATIENT,
            metric = metric,
            value = value,
            unit = "kg",
            measuredAt = LocalDateTime(date, at).toInstant(ZONE),
            source = MeasurementSource.MANUAL,
            externalId = null,
            note = null,
        )
}

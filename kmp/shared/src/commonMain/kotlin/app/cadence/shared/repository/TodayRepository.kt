package app.cadence.shared.repository

import app.cadence.shared.domain.Compound
import app.cadence.shared.domain.InjectionSite
import app.cadence.shared.domain.Macros
import app.cadence.shared.domain.Occurrence
import app.cadence.shared.domain.PartOfDay
import app.cadence.shared.domain.ProtocolRow
import app.cadence.shared.domain.ReorderHint
import app.cadence.shared.domain.TitrationStep
import kotlinx.datetime.LocalDate

/**
 * What the Today screen needs, in one object.
 *
 * §11's screen → data map makes this one call — «GET /me/today — next dose +
 * logged state, meals summary & targets, weight glance, reorder hint,
 * greeting/cycle week» — because the server computes the derived parts and
 * sends them ready to render (L10). Splitting it into getters here would be
 * designing a client for an API that does not exist.
 *
 * Everything is a number or a domain type. «0,25 мг» and «1 240 ккал» are
 * assembled by `app.cadence.format` on the UI side; nothing in this object has
 * been through a formatter.
 */
data class TodaySummary(
    /** The day this summary is about, in the patient's zone. */
    val date: LocalDate,
    /** «утро» in «Воскресенье, утро · 4-я неделя» — a fact about the clock. */
    val partOfDay: PartOfDay,
    val cycleWeek: Int?,
    val nextDose: Occurrence?,
    /** What the hero card names above the dose. */
    val nextDoseCompound: Compound?,
    /**
     * «Предложим следующую зону по ротации» — the wizard's site step opens on
     * this one, and the prototype ships it as a frozen constant.
     */
    val suggestedSite: InjectionSite,
    /** «Протокол этой недели», from the same generator the calendar reads. */
    val weekProtocol: List<ProtocolRow>,
    val doseLoggedToday: Boolean,
    val mealCount: Int,
    /** All four numbers, because the meals card draws P/C/F against target. */
    val mealMacros: Macros,
    val targets: Macros,
    val weightKg: Double?,
    /** The same seven points the biomarker glance draws, oldest first. */
    val weightSeries: List<Double>,
    val targetWeightKg: Double?,
    val vialDosesLeft: Int,
    val reorder: ReorderHint?,
    /** «Доза растёт: 0,25 мг → 0,5 мг», derived from the phases. */
    val nextTitration: TitrationStep?,
)

/**
 * The Today screen's data source.
 *
 * Declared here rather than by the screen because `shared` is where the Ktor
 * client will implement it — but it is the screen's shape, not the server's:
 * the mock and the client both answer to this, and swapping one for the other
 * is a constructor argument.
 */
interface TodayRepository {
    /** §11: `GET /me/today`. */
    suspend fun today(): TodaySummary
}

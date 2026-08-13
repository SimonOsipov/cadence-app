package app.cadence.shared.domain

import kotlin.time.Instant

/**
 * The prototype doesn't compute this: `INITIAL_LOG_STATE` carries `suggested`/`lastUsed` as
 * two frozen constants, so its suggestion never moves. Here it's a function of logged
 * events. Rule is least-recently-used: unused zones win, and among used zones the one whose
 * *latest* injection is oldest wins — the tissue doesn't care it was also used in April.
 * Recency is [DoseEvent.injectedAt], never position in [recent]: a repository list has
 * whatever order the query left.
 */
fun suggestNextSite(recent: List<DoseEvent>): InjectionSite {
    val lastUsedAt: Map<InjectionSite, Instant> =
        recent
            // Nullable by design: an oral item has no zone. Also where an unrecognised
            // site_code lands, decoded to null upstream rather than crashing a screen.
            .mapNotNull { event -> event.site?.let { site -> site to event.injectedAt } }
            .groupBy({ it.first }, { it.second })
            .mapValues { (_, times) -> times.max() }

    // Unused zones sort first (no real instant beats Instant.DISTANT_PAST); ties keep the
    // enum's declaration order, a deterministic tie-break with no clinical meaning.
    // `reduce`, not `minByOrNull` + fallback: the enum has ten constants, never empty.
    return InjectionSite.entries.reduce { best, site ->
        val siteAt = lastUsedAt[site] ?: Instant.DISTANT_PAST
        val bestAt = lastUsedAt[best] ?: Instant.DISTANT_PAST
        if (siteAt < bestAt) site else best
    }
}

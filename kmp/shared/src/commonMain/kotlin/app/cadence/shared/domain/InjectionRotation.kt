package app.cadence.shared.domain

import kotlin.time.Instant

/**
 * «Предложим следующую зону по ротации» — the one piece of clinical logic in
 * the dose wizard, and the reason [InjectionSite] has ten members rather than a
 * free-text field.
 *
 * The prototype does not compute this: `INITIAL_LOG_STATE` carries
 * `suggested: 'l-abdomen'` beside `lastUsed: ['r-abdomen']` as two frozen
 * constants, so its suggestion never moves and cannot disagree with what the
 * patient actually did. Here it is a function of the logged events, which is
 * «nothing derived is stored» applied to the one number a patient acts on.
 *
 * The rule is least-recently-used: a zone never injected wins over every zone
 * that has been, and among used zones the one whose *latest* injection is
 * oldest wins. Reading the latest and not the earliest is what makes a zone
 * used twice count as recent — the tissue does not care that it was also used
 * in April.
 *
 * Recency is [DoseEvent.injectedAt] and never the position in [recent]: a list
 * from a repository has whatever order the query left, and a rotation that
 * reads `last()` gives two answers for one history.
 */
fun suggestNextSite(recent: List<DoseEvent>): InjectionSite {
    val lastUsedAt: Map<InjectionSite, Instant> =
        recent
            // Nullable by design: an oral item has no zone, and a patient may
            // skip the step. Such an event says nothing about rotation.
            //
            // It is also where «a zone the current set does not contain» lands.
            // Nothing can hand this function an unknown zone — `site` is a
            // typed `InjectionSite?` — so the obligation belongs to the decoder
            // that will turn `dose_events.site_code` into the enum: an
            // unrecognised code decodes to `null` and is read here as no zone,
            // rather than crashing a screen over a row it cannot name.
            .mapNotNull { event -> event.site?.let { site -> site to event.injectedAt } }
            .groupBy({ it.first }, { it.second })
            .mapValues { (_, times) -> times.max() }

    // A zone with no injection sorts before every real instant, so unused zones
    // come first without a branch; equal keys keep the enum's own declaration
    // order. That order is left/right pairs grouped by body part and means
    // nothing clinically — it is a deterministic tie-break, not a progression
    // across the body, and nobody should «restore» an anatomy to it.
    //
    // `reduce` rather than `minByOrNull` and a fallback: the enum has ten
    // constants, so there is no empty case for which an answer would have to be
    // invented.
    return InjectionSite.entries.reduce { best, site ->
        val siteAt = lastUsedAt[site] ?: Instant.DISTANT_PAST
        val bestAt = lastUsedAt[best] ?: Instant.DISTANT_PAST
        if (siteAt < bestAt) site else best
    }
}

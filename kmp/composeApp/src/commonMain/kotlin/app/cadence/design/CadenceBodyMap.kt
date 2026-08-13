package app.cadence.design

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import app.cadence.shared.domain.InjectionSite

/** Which way round the diagram is showing — the prototype's `state.view`. */
enum class CadenceBodyView(
    val labelRu: String,
) {
    FRONT("Спереди"),
    BACK("Сзади"),
}

/**
 * Where each of §03's ten zones is drawn, and what it is called in Russian.
 * Coordinates and labels are copied from `mobile/src/features/log-dose/data.ts`
 * (`ZONES_FRONT`/`ZONES_BACK`) against its `viewBox="0 0 200 340"` — a spec
 * table, not geometry to reason about. Lives here, not in `shared`, because
 * which side a zone draws on is presentation; `InjectionSite` is the fact.
 */
enum class CadenceBodyZone(
    val site: InjectionSite,
    val labelRu: String,
    val view: CadenceBodyView,
    val x: Float,
    val y: Float,
) {
    RIGHT_DELTOID(InjectionSite.RIGHT_DELTOID, "Правое плечо", CadenceBodyView.FRONT, 54f, 88f),
    LEFT_DELTOID(InjectionSite.LEFT_DELTOID, "Левое плечо", CadenceBodyView.FRONT, 146f, 88f),
    RIGHT_ABDOMEN(InjectionSite.RIGHT_ABDOMEN, "Правый живот", CadenceBodyView.FRONT, 82f, 148f),
    LEFT_ABDOMEN(InjectionSite.LEFT_ABDOMEN, "Левый живот", CadenceBodyView.FRONT, 118f, 148f),
    RIGHT_THIGH(InjectionSite.RIGHT_THIGH, "Правое бедро", CadenceBodyView.FRONT, 82f, 230f),
    LEFT_THIGH(InjectionSite.LEFT_THIGH, "Левое бедро", CadenceBodyView.FRONT, 118f, 230f),
    LEFT_LOWER_BACK(InjectionSite.LEFT_LOWER_BACK, "Левая поясница", CadenceBodyView.BACK, 82f, 175f),
    RIGHT_LOWER_BACK(InjectionSite.RIGHT_LOWER_BACK, "Правая поясница", CadenceBodyView.BACK, 118f, 175f),
    LEFT_GLUTE(InjectionSite.LEFT_GLUTE, "Левая ягодица", CadenceBodyView.BACK, 82f, 210f),
    RIGHT_GLUTE(InjectionSite.RIGHT_GLUTE, "Правая ягодица", CadenceBodyView.BACK, 118f, 210f),
    ;

    companion object {
        private val bySite = entries.associateBy { it.site }

        /** The drawing for a zone. Null only if the set and the table drift. */
        fun of(site: InjectionSite): CadenceBodyZone? = bySite[site]
    }
}

/** The prototype's `viewBox="0 0 200 340"`, which every coordinate above is in. */
private const val VIEW_W = 200f
private const val VIEW_H = 340f

/** The hit target the prototype gives a zone: `r={18}` in the same viewBox. */
private const val TARGET_R = 18f

/** The diagram itself, so a test can measure a zone against its box. */
const val CADENCE_BODY_MAP_TAG = "cadence-body-map"

/** `minHeight: 32` on the prototype's caption area. */
private val CAPTION_MIN_HEIGHT = 32.dp

/**
 * The ten-zone diagram, in the design system because it draws a primitive with
 * no product logic. Each zone is a real node with a Russian `contentDescription`
 * and `selected` state, not a path inside a canvas: a `Canvas` has no text or
 * children, so a diagram drawn as one is unreadable to any test or screen
 * reader; this draws the silhouette and lays the targets over it.
 */
@Composable
fun CadenceBodyMap(
    selected: InjectionSite?,
    suggested: InjectionSite?,
    modifier: Modifier = Modifier,
    view: CadenceBodyView = CadenceBodyView.FRONT,
    lastUsed: List<InjectionSite> = emptyList(),
    onView: (CadenceBodyView) -> Unit = { },
    onSelect: (InjectionSite) -> Unit = { },
) {
    Column(modifier, horizontalAlignment = Alignment.CenterHorizontally) {
        BodyViewToggle(view, onView)

        BoxWithConstraints(
            Modifier
                .padding(top = CadenceSpacing.sm)
                .fillMaxWidth(),
        ) {
            val width = maxWidth
            val height = width * (VIEW_H / VIEW_W)
            val unit = width / VIEW_W

            Box(Modifier.fillMaxWidth().testTag(CADENCE_BODY_MAP_TAG)) {
                CadenceBodySilhouette(view, Modifier.size(width, height))

                CadenceBodyZone.entries.filter { it.view == view }.forEach { zone ->
                    ZoneTarget(
                        zone = zone,
                        size = unit * (TARGET_R * 2),
                        isSelected = selected == zone.site,
                        isSuggested = suggested == zone.site && selected == null,
                        wasUsed = zone.site in lastUsed,
                        onSelect = onSelect,
                        modifier =
                            Modifier.offset(
                                x = unit * (zone.x - TARGET_R),
                                y = unit * (zone.y - TARGET_R),
                            ),
                    )
                }
            }
        }

        ZoneCaption(selected, suggested)
    }
}

@Composable
private fun BodyViewToggle(
    view: CadenceBodyView,
    onView: (CadenceBodyView) -> Unit,
) {
    Row(
        Modifier
            .clip(CircleShape)
            .background(Cadence.palette.sunk)
            .padding(CadenceSpacing.xxs),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs),
    ) {
        CadenceBodyView.entries.forEach { candidate ->
            CadenceChip(
                label = candidate.labelRu,
                onClick = { onView(candidate) },
                active = candidate == view,
            )
        }
    }
}

@Composable
private fun ZoneTarget(
    zone: CadenceBodyZone,
    size: Dp,
    isSelected: Boolean,
    isSuggested: Boolean,
    wasUsed: Boolean,
    onSelect: (InjectionSite) -> Unit,
    modifier: Modifier = Modifier,
) {
    val ring =
        when {
            isSelected -> CadenceColors.forest700
            isSuggested -> CadenceColors.sand500
            wasUsed -> Cadence.palette.muted
            else -> Cadence.palette.border
        }

    Box(
        modifier
            // Scaled, not fixed dp: fixed dp let a target's centre drift by
            // `TARGET_R * (1.dp - unit)` — measured as 14dp off from the
            // shoulder on a 360dp diagram, invisible to any test.
            .size(size)
            .clip(CircleShape)
            .background(if (isSelected) CadenceColors.forest700 else Cadence.palette.paper)
            .clickable { onSelect(zone.site) }
            // clearAndSetSemantics: left to merge, ring + fill become two
            // nodes and onNodeWithContentDescription finds both.
            .clearAndSetSemantics {
                // Prototype appends "· использовано" for `state.lastUsed`;
                // without it a suggestion reads as arbitrary.
                contentDescription = if (wasUsed) "${zone.labelRu} · использовано" else zone.labelRu
                selected = isSelected
            },
        contentAlignment = Alignment.Center,
    ) {
        if (isSelected) {
            CadenceIcon(paths = CadenceIcons.check, size = CadenceSpacing.md, tint = CadenceColors.cream)
        } else {
            Box(
                Modifier
                    .size(CadenceSpacing.xs)
                    .clip(CircleShape)
                    .background(ring),
            )
        }
    }
}

/** Selected zone's name, a rotation-suggestion hint, or nothing. */
@Composable
private fun ZoneCaption(
    selected: InjectionSite?,
    suggested: InjectionSite?,
) {
    val chosen = selected?.let { CadenceBodyZone.of(it) }
    val hint = suggested?.let { CadenceBodyZone.of(it) }

    Column(
        // Reserves the prototype's `minHeight: 32`: choosing a zone turns one
        // caption line into two, and without it everything below jumps.
        Modifier.padding(top = CadenceSpacing.sm).heightIn(min = CAPTION_MIN_HEIGHT),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        if (chosen != null) {
            CadenceTitle(chosen.labelRu)
            CadenceMeta("Нажмите другую зону, чтобы изменить")
        } else if (hint != null) {
            CadenceMeta("Предложено: ${hint.labelRu} — следующее в ротации")
        }
    }
}

/**
 * Torso, head, arms and legs — the prototype's own SVG paths
 * (`mobile/src/features/log-dose/components.tsx`), scaled from its
 * `viewBox="0 0 200 340"`. A path table, not geometry to reason about; back
 * differs from front only in where the torso ends.
 */
@Composable
private fun CadenceBodySilhouette(
    view: CadenceBodyView,
    modifier: Modifier = Modifier,
) {
    val fill = Cadence.palette.sunk
    val stroke = Cadence.palette.border
    val torso =
        if (view == CadenceBodyView.BACK) {
            "M55 78 Q56 70 66 70 L134 70 Q144 70 145 78 L145 180 Q145 192 134 194 L66 194 Q55 192 55 180 Z"
        } else {
            "M55 78 Q56 70 66 70 L134 70 Q144 70 145 78 L145 175 Q145 188 134 190 L66 190 Q55 188 55 175 Z"
        }
    val parts =
        remember(torso) {
            listOf(
                // Head, neck, torso, then the four limbs.
                "M78 38 A22 24 0 1 0 122 38 A22 24 0 1 0 78 38 Z",
                "M92 60 L92 72 Q92 76 96 76 L104 76 Q108 76 108 72 L108 60 Z",
                torso,
                "M40 80 Q34 84 34 96 L34 196 Q34 204 41 206 L48 206 Q55 204 55 196 L55 90 Q55 80 47 78 Z",
                "M160 80 Q166 84 166 96 L166 196 Q166 204 159 206 L152 206 Q145 204 145 196 L145 90 Q145 80 153 78 Z",
                "M68 190 L94 190 Q98 190 98 198 L98 320 Q98 328 91 328 L75 328 Q68 328 68 320 Z",
                "M132 190 L106 190 Q102 190 102 198 L102 320 Q102 328 109 328 L125 328 Q132 328 132 320 Z",
            ).map { PathParser().parsePathString(it).toPath() }
        }

    Canvas(modifier.clearAndSetSemantics { }) {
        val scale = size.width / VIEW_W
        scale(scale, scale, pivot = Offset.Zero) {
            parts.forEach {
                drawPath(it, color = fill)
                drawPath(it, color = stroke, style = Stroke(width = SILHOUETTE_STROKE / scale))
            }
        }
    }
}

/** `strokeWidth={1.2}` in the prototype's viewBox units. */
private const val SILHOUETTE_STROKE = 1.2f

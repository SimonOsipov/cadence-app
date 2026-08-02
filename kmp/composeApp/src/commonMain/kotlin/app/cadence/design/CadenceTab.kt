package app.cadence.design

/**
 * The four destinations of the patient app, ported from `TABS` in
 * mobile/src/components/shared.tsx.
 *
 * Four, not five: the prototype's list carries the centre action alongside them
 * behind a `primary` flag, but an action is not a place. Keeping it in the same
 * set made "which destination is current" answerable with the action — a bar
 * with nothing highlighted, which is the state the prototype expressed as
 * `TabId | null` and which this port had claimed was impossible while leaving
 * it reachable. The showcase reached it on the first tap.
 *
 * Icons are the path data rather than a key into [CadenceIcons.byName]: the set
 * is closed and known at compile time, so a lookup that can miss has no reason
 * to exist here. `byName` is the adapter for the prototype's kebab strings, and
 * a destination is not a string from outside.
 *
 * Labels are product copy and stay Russian.
 */
enum class CadenceDestination(
    val icon: List<String>,
    val label: String,
) {
    TODAY(CadenceIcons.home, "Сегодня"),
    INVENTORY(CadenceIcons.beaker, "Аптечка"),
    TRENDS(CadenceIcons.chartBar, "Тренды"),
    NUTRITION(CadenceIcons.cake, "Питание"),
}

/**
 * The centre action: logging a dose.
 *
 * Not a destination — it pushes a wizard rather than switching tabs — so it
 * carries only what drawing it needs. Its label is never displayed; it exists
 * so a screen reader can name a circle that holds no text.
 */
object CadenceLogAction {
    val icon: List<String> = CadenceIcons.plus

    const val ACCESSIBILITY_LABEL: String = "Записать"
}

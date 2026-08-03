# Divergences from the frozen prototype

`mobile/` is the visual specification for the patient app. A difference between
it and `kmp/` is a decision, not an accident — invariant 1 of the `kmp-app`
architecture note. Every one of them is written down here, with the reason.

Step 11 of the port block reviews this document against a side-by-side run of
the two apps. Entries are added as they are made, not reconstructed at the end.

## Body typeface: Golos Text, not DM Sans

**What the prototype does:** `F.body` in `mobile/src/theme/index.ts` is
`DMSans_400Regular`, with `DMSans_500Medium` and `DMSans_600SemiBold` beside it.

**What we do:** Golos Text at the same three weights.

**Why:** DM Sans publishes `latin` and `latin-ext` only. Every string this
product shows a user is Russian, so DM Sans would hand all of it to a system
fallback — the layout would hold and the type would not be the type. The
prototype had already hit exactly this on the display face (Instrument Serif
has no Cyrillic) and fell back to Cormorant Garamond; nobody re-ran the check
on the body face. Golos Text was designed for Cyrillic rather than extended
into it, and sits close to DM Sans in width and tone, so the prototype's
layouts do not shift.

Decision recorded 2026-07-28 in `specs/dashboard-skeleton.md`; it binds both
surfaces, so the dashboard changes `--font-body` in the same way.

Guarded by `BundledFontTest.everyBundledFaceCoversTheCyrillicAlphabet`, which
parses each bundled file's `cmap`. Swapping a Latin-only subset back in fails
it.

## Card shadows: the step survives, the warm tint does not

**What the prototype does:** `SHADOW` in `theme/index.ts` is a warm umbra —
`shadowColor: '#2e2618'` with an explicit opacity, radius and offset per step.

**What we do:** `CadenceElevation` carries the step only; Compose takes an
elevation and derives the shadow per platform.

**Why:** `Modifier.shadow` has no tint parameter that behaves the same on both
platforms. Reproducing the umbra would mean drawing our own shadow under every
card and sheet. Accepted at the design-system port (BST-05) as a visible but
bounded loss: the elevation *order* is what the layouts depend on, and that
survives intact.

## `border` and `ink700`: the mobile theme wins, and the two prototypes disagree

**What the prototype does:** `mobile/src/theme/index.ts` sets `border` to
`#cdc1a8` and defines `ink700` as `#3f3b35`.

**What we do:** exactly the same. `CadenceColors.kt` matches the mobile theme
value for value.

**Why this is listed at all:** the *web* prototype's `colors_and_type.css` sets
`--border` to `#e4dac6` — our `bone` — and has no `ink-700` at all; `#cdc1a8`
is its `--border-strong`. So the divergence is not between us and the
prototype, it is between the two prototypes, and the mobile one is canon for
the mobile app. Nothing to change here; the web side resolves it in its own
token port (`dashboard-skeleton`, step 2).

Verified name by name on 2026-08-03: `CadenceColors.kt` and `theme/index.ts`
agree on every value, and the icon sets match at 41 each.

## The sparkline is drawn, not vectored

**What the prototype does:** `Spark` in `shared.tsx` is an SVG `<Path>`.
**What we do:** a Compose `Canvas` with the same geometry — the padding, the
2dp stroke, the 3dp trailing dot, the 0.4 fill alpha are the prototype's values.
**Why:** Compose has no SVG. The numbers moved across as `Dp` rather than raw
floats, because a `DrawScope` measures in physical pixels and the prototype's
viewBox units are density-independent; passed straight through, the line would
be a third as thick on a 3x screen and different on each platform.

## A degenerate series is centred, not pinned to the top

**What the prototype does:** `(max - v) / (max - min || 1)` — with no span, the
`|| 1` sends every point to `y = pad`, the top edge. A single point lands at the
top-left corner.
**What we do:** a flat series sits on the centre line; a single measurement sits
in the middle of the box.
**Why:** the prototype never renders either, because all of its data is
hardcoded ascending — its `|| 1` is faithful to a path it never walks. Pinned to
the top with a fill, a flat series fills ~86% of the box and reads as "at
maximum", and «110, 110, 110» draws identically to «0,5 · 0,5 · 0,5»: the
vertical axis stops meaning anything while still looking like it does. A single
point in the top-left reads as an old, high reading, when the trailing dot marks
"now" at the right everywhere else. For adherence and biomarkers this is read
clinically, so "no change" is drawn as no change.

## An empty series draws a baseline, not nothing

**What the prototype does:** nothing — an empty `data` array yields an empty
`<Svg>`.
**What we do:** a hairline rule across the centre.
**Why:** an empty box beside a caption and an axis reads as "failed to load",
and a patient waits and retries instead of going and measuring. A rule says
"there is a scale here and no values on it".

## Non-finite values are dropped

**What the prototype does:** nothing — JavaScript propagates `NaN` silently.
**What we do:** filter them out of the series before any arithmetic.
**Why:** one `NaN` makes `max`, `min` and every coordinate `NaN`, and the whole
chart vanishes with nothing raised. A measurement pipeline that divides — a
ratio with a zero denominator, a unit conversion — produces them.

## The tab bar has four destinations and one action, not five tabs

**What the prototype does:** `TABS` is one list of five, with the centre entry
flagged `primary`, and `active` typed `TabId | null`.
**What we do:** `CadenceDestination` holds the four places; the centre action is
separate, with its own callback.
**Why:** an action is not a place. Keeping them in one set made "which
destination is current" answerable with the action, which draws a bar with
nothing highlighted — the state `TabId | null` expressed and this port had
claimed impossible while leaving it reachable. The showcase reached it on the
first tap of the centre button.

Also renamed: the prototype's `insights` is `TRENDS` here, matching the screen's
own name («Тренды») rather than its route key.

## Russian plurals: the rule, not the prototype's approximation of it

**What the prototype does:** `ActionChooserSheet.tsx` picks the meal noun with
`mealCount === 1 ? 'приём' : mealCount < 5 ? 'приёма' : 'приёмов'`.

**What we do:** `pluralMeals` in `app.cadence.format` applies the actual rule —
11–14 take «приёмов», then the last digit decides.

**Why:** the ternary is right through 20 and wrong from 21 up («21 приёмов»).
No count the sheet can reach today leaves that range, so nothing on screen
changes; what changes is that the next screen counting something copies a rule
that holds instead of one that happens to. Pinned by
`CadenceFormatTest.mealsTakeTheRussianPluralAndNotThePrototypesApproximationOfIt`,
which asserts exactly the counts where the two disagree.

## Number grouping: hand-rolled, because Kotlin/Native has no ICU

**What the prototype does:** `(1240).toLocaleString('ru-RU')` → «1 240», with
U+00A0 between the groups.

**What we do:** `formatInteger` in `app.cadence.format` produces the same
string from the same rule.

**Why:** there is no `toLocaleString` that works on both platforms —
Kotlin/Native carries no ICU, so a locale API would be an expect/actual pair
over two different implementations for a rule that is three lines. The
separator is asserted as the code point, not as «looks spaced»: a plain space
would let a kcal count wrap between its thousands and its hundreds.

## Navigation: one library taken rather than ported

**What the prototype does:** `@react-navigation/native-stack`, with
`animation: 'slide_from_right'`, `animationDuration: 380`, and a
`fullScreenModal` group for the four logging flows.

**What we do:** `org.jetbrains.androidx.navigation:navigation-compose` 2.9.2,
with the same durations and directions spelled out as Compose transitions.

**Why:** unlike the Material widgets this project refused, what a navigation
library carries is not a look. It is back-stack ownership, saved state across
process death, and the platform back gesture — a hand-rolled stack would have
to reimplement all three before the first screen lands. `slide_from_right` has
no Compose preset, so the push is written out: a full-width slide in, with the
outgoing screen trailing at a third of the width, which is what a native iOS
push does under it.

The four modal routes carry their transition by membership in a `modal<>`
builder rather than by repeating two overrides each, the way the prototype's
`Stack.Group` does. One omission in four copies would be a screen sliding the
wrong way, and no test would say so.

## `navigate` is not `navigate`

**What the prototype does:** React Navigation's `navigate` returns to an
existing instance of a route if the stack already holds one; its `push` always
adds a new entry. `AppNavigator.tsx` uses both, deliberately — `push` only for
article-to-article.

**What we do:** `NavHostController.openRoute` reproduces the first, `pushRoute`
the second. Compose's own `navigate` always pushes, and `launchSingleTop`
de-duplicates only against the top of the stack, so neither is the prototype's
behaviour.

**Why it is visible:** «добавить в день» on a recipe hands back to Nutrition
from three screens deep. Faithful, the stack goes
`[Today, Nutrition, Recipes, RecipeDetail]` → `[Today, Nutrition]`; with a plain
navigate it grows to five and back walks a path the user never took. Pinned by
`CadenceNavigationTest.openingARouteAlreadyInTheStackReturnsToItInsteadOfStackingASecond`.

## Screen titles while the screens are placeholders

**What the prototype does:** each screen draws its own composed header — no
screen anywhere renders the bare word «Сегодня» as a title.

**What we do:** the placeholder standing in for each route titles itself
«Экран «Сегодня»».

**Why:** the bare label is also the tab bar's own text, so two nodes would read
the same word and every assertion about which screen is showing would be
ambiguous — which is how the first version of the navigation test failed. This
divergence dies with the placeholders, in steps 3–9.

## `openRoute` re-opens the current screen instead of no-opping

**What the prototype does:** React Navigation's `navigate` to the screen you are
already on, with identical params, does nothing.

**What we do:** rebuild the entry.

**Why:** the cheap comparison — the filled route string — is generated
internally by navigation-compose and not exposed. `NavBackStackEntry.toRoute<T>()`
would rebuild the route object and the routes are data classes, so it is not
impossible, only awkward: `openRoute` would have to be `reified`, and the
`selectDestination` call site passes `CadenceDestination.route`, typed as the
non-serializable `CadenceRoute` supertype. Narrowing that property is what
makes this fixable, if a screen ever holds state worth the trouble.

The alternative —
the type-only guard this code shipped with first — returned early on *any*
same-type navigation, so tapping a neighbouring biomarker from
`TrendDetail("hrv")` did nothing at all and said nothing about why. Given the
choice between a rebuilt entry and a dead tap in a medical app, the rebuild
wins. No screen holds state worth preserving yet; the first one that does is
what forces this line to change. Pinned by
`CadenceNavigationTest.openingTheScreenYouAreOnWithDifferentArgumentsShowsTheNewOnes`
and `…DoesNotStackASecondCopy`.

## `replace` still leaves two copies when the target is already below

**What the prototype does:** `ChatList → ChatThread → replace('ChatList')`
leaves `[…, ChatList, ChatList]`. React Navigation's `replace` swaps the top
entry and does not look further down.

**What we do:** the same — measured on the simulator, `[Today, ChatList,
ChatList]`.

**Why it is listed:** because it is a defect, not a feature — back from the
second `ChatList` lands on a visually identical screen and the button reads as
broken. It is not in the partner's §08 list of prototype bugs, and it is
reachable only from `ChatThreadScreen`'s «к списку», which is ported in step 9.
Left faithful for now and raised there, where the screen that triggers it is in
front of us, rather than fixed blind in the shell.

## Modal transitions live on the NavHost, not on the modal

**What the prototype does:** `Stack.Group` with
`presentation: 'fullScreenModal'` — the four logging flows rise from the bottom
and the screen beneath them does not move.

**What we do:** the same, decided at the `NavHost` by asking whether the
destination on the other side of the transition is a `CadenceRoute.Modal`.

**Why it is written down:** the obvious spelling — overriding `enterTransition`
and `exitTransition` on the four `composable<…>` entries — does not work, and
looks like it does. Compose reads each side of a transition from the
destination it belongs to: a screen's exit comes from the screen being *left*.
Overrides on the modal therefore fired when navigating forward *out* of a
modal, an event that only arrives with step 9, while the underlay kept the
NavHost default and drifted a third of the width sideways on every one of the
four flows. This shipped for a whole review round with a comment asserting the
opposite. Pinned now by
`CadenceNavigationTest.theScreenBeneathAModalDoesNotMove`, its negative
`anOrdinaryPushDoesMoveTheScreenBeneath`, and
`aModalArrivesFromBelowAndAPushFromTheSide`.

## The protocol strip is computed, and it titrates

**What the prototype does:** `TodayScreen.tsx:100-133` is a literal array of
three rows. The semaglutide row reads «Семаглутид · 0,25 мг» for all twelve
weeks, and its state is a hand-wired boolean.

**What we do:** `weekProtocolRows` projects the same occurrences the Schedule
screen renders — dose from the phase covering the current cycle week, today's
state from the day's occurrence.

**Why:** §03 names it directly: «Today-screen protocol strip hardcoded, diverges
from schedule data · both render the same generated occurrences endpoint». The
two cannot disagree now, by construction, which is why both screens are one
subtask.

Rows come from the week's *items*, not the day's occurrences: «Протокол этой
недели» carries the weekly injection on a Wednesday too. `todayStatus` is
nullable so a row on a day it is not due says nothing about «сегодня».

## «Вес · 7 дней» becomes «Вес · 7 недель»

**What the prototype does:** captions the headline glance «Вес · 7 дней» and
draws seven hardcoded points with a literal «↓ 0,6 кг» beneath them.

**What we do:** caption the number of readings the series actually holds, and
compute the delta from its last two points.

**Why:** the prototype's own schedule weighs the patient **weekly**
(«Взвешивание · Воскресенье» in `schedule/data.ts`), so seven readings are seven
weeks. The caption followed a number nobody had rechecked against the data
beside it. This is the same class as the two disconnected vial datasets — two
parts of one prototype disagreeing — and the resolution is the same: the data
wins.

## The meal hero arrives without its suggestion

**What the prototype does:** `MealHero` picks a dish from the recipe library via
`suggestNextMeal(meals, now)` and titles the card with it.

**What we do:** render the half that is arithmetic — «Осталось 960 ккал · 80 г
белка», computed against the day's targets — plus the two ways out.

**Why:** the library and the rule that picks from it belong to the nutrition
section, step 8 of the block. Porting a suggestion with no recipes behind it
would mean seeding a second, parallel set of dishes — which is exactly the
prototype mistake §03 spends its reconciliation table undoing. Completed when
step 8 lands; recorded here so it is not mistaken for an oversight.

## The `:shared`-is-linked proof moves off the placeholder

**What it was:** `AppTest` asserted «заглушка · iOS» on `PlaceholderScreen`'s
footer, which existed so that `:shared` being linked into the UI — rather than
merely into the module graph — had something observable behind it.
`PlaceholderScreen`'s KDoc asked whoever deleted the last placeholder to re-home
that assertion rather than let a real screen grow a platform label.

**What it is now:** the Today route draws the ported screen, whose greeting and
hero are assembled from a `TodaySummary` that comes through `:shared`. Reaching
the screen at all is the proof, and the platform-name test is deleted rather
than moved.

**Note for the remaining seventeen routes:** they still draw placeholders, so
the footer is still there. This resolves the obligation for Today only; the same
question returns for whichever route is last.

## `App()` reads the system clock, so its tests assert what the clock cannot move

`CadenceApp`'s default `CadenceMocks()` uses `SystemCadenceClock`, so `AppTest`
cannot assert a weekday or a cycle week without passing or failing by the day it
runs on — the trap `ConfirmToastTest` already fell into once. Tests that need a
day wind their own clock and pass the mocks in; `AppTest` asserts the patient's
name and the selected tab instead.

## The schedule and Today ports are partial, and here is what is missing

Fidelity is the block's acceptance criterion and invariant 1 of the `kmp-app`
note makes an unrecorded difference an accident rather than a decision. A review
found that the registry covered a fraction of what the two ports changed. The
rest, named:

**Schedule.** No month name or year over the grid; one month rather than the
whole cycle scrolled; no legend; no progress track with the «10 мая · старт /
26 июля · финиш» ends; no «Сегодня» jump or auto-scroll; one injection dot
rather than the prototype's four category dots; the band names the week but not
«Семаглутид · 0,25 мг еженедельно»; the day sheet is not ported and `onOpenDay`
is left unwired, so no day is tappable from the app. Closed by a later pass over
this screen; the day sheet needs the dose wizard (step 5) to have anywhere to go.

**Today.** `TodayMeals` draws totals only — the prototype lists the last three
meals with name, time, item count and kcal, plus an empty state. The macro legs
are relabelled P/C/F → Б/Ж/У and reordered. `MealHero` uses the paler `sand100`
and turns two pills into text links, with «Записать приём пищи» shortened to
«Записать приём» and «Из рецепта» to «Рецепты». The reorder warning sits at the
bottom of the scroll rather than in the prototype's fourth position — a
low-stock warning demoted below the fold is a behaviour change, and it should be
moved back or defended. The hero drops the `06:42` timestamp and the injection
site from «Записано · правый живот, ротация.».

None of these is a decision anyone made; they are what a port leaves behind when
it is measured by tests rather than by a side-by-side run. Step 11 is that run.

## The rotation suggestion is computed, not seeded

**What the prototype does:** `INITIAL_LOG_STATE` in
`mobile/src/features/log-dose/data.ts` carries `suggested: 'l-abdomen'` beside
`lastUsed: ['r-abdomen']` as two independent literals, and `components.tsx`
renders `state.suggested` directly. Nothing recomputes it, so the suggestion
cannot move and cannot disagree with what the patient actually logged — the two
constants are consistent only because someone typed them that way.

**What we do:** `suggestNextSite(recent: List<DoseEvent>)` in
`shared/domain/InjectionRotation.kt` derives it — least-recently-used, an unused
zone ahead of every used one, ties broken by the set's order.

**Why:** «Nothing derived is stored» is a project rule, and a suggestion is the
one number on this screen a patient acts on. A frozen constant is a suggestion
that goes stale the first time it is followed.

**What a side-by-side run will show:** with any mock history in place the
highlighted zone will differ from the prototype's screenshot. That is the point,
not a defect. The prototype's own documented pair — `r-abdomen` used therefore
`l-abdomen` suggested — is reproduced by the rule and asserted in
`InjectionRotationTest.anUnusedZoneWinsOverEveryUsedOne`.

The rule reads `DoseEvent.injectedAt` and not `scheduledForDate`, so a dose
logged after the fact counts from when it went in.

## The dose step has a guard, and the compound row is idempotent

**What the prototype does:** `stepDefs` in `log-dose/LogDoseScreen.tsx` sets
`nextDisabled` on step 1 («Препарат») and step 3 («Место») and on no other. Step
2 advances with whatever is in the field, including nothing, and its stepper
clamps at zero, so «0 мг» is a reachable state that «Дальше» accepts. Its
compound row is also not idempotent: pressing the already-selected compound
re-applies `comp.default`, so a patient who stepped the dose down, went back and
tapped the same compound again silently got the default number returned.

**What we do:** `DoseDraft.canAdvance(DOSE)` requires a dose that exists and is
greater than zero, and `selectItem` returns the draft untouched when the item
tapped is the one already chosen.

**Why:** the zero dose is not cosmetic — the wizard's write decrements a vial,
so an accepted zero records an injection that did not happen and takes stock
with it. The re-tap is listed as a prototype bug rather than a behaviour: «его
баги не к переносу».

**Also deliberately different:** the site step is required only for an item
whose kind is `INJECTION`. `DoseEvent.site` is nullable because a supplement has
no zone, and the prototype never has to answer this because every compound in
its `COMPOUNDS` list is subcutaneous.

## The vial picker is not ported yet

**What the prototype does:** step 2 renders a `VialPicker` under the stepper and
`LogState` carries `vialId`; step 5 names the vial in the review.

**What we do:** `DoseDraft` carries no vial. The write resolves the first vial
of that compound itself, which is what the narrow `logDose` already did — and
it does not read `disposedAt`, because the seed holds one vial per compound
and a disposal branch would be a line no test can reach.

**Why:** the plan's field list for the draft is the patient's choices, and which
vial a dose came out of is a choice the app can make correctly on its own while
there is exactly one active vial per compound. It stops being correct the moment
a patient has two open vials of the same compound — §03 allows it and the
prototype's own `VIALS` seed contains it (`v1` and `v2`, both semaglutide).

**Owed:** the picker, and with it a `vialId` on the draft. Until then a patient
with two open vials cannot say which one they used. Named here rather than in a
comment because step 11 measures this file.

## One tap on the placeholder invents a zone, and the rotation reads it back

**What the prototype does:** its wizard asks. There is no one-tap path.

**What we do, until task 6 of the dose-wizard plan:** the «Записать дозу»
placeholder has one button, and `submit` refuses an injection with no zone, so
`TodaySummary.oneTapDraft()` sends the rotation's own suggestion.

**Why this is a cost and not a detail:** the record is indistinguishable from a
zone the patient chose, and `suggestNextSite` reads events back — so one tap
moves the rotation on evidence nobody gave. Nothing on `DoseEvent` marks a zone
as assumed, so a history built this way cannot be separated afterwards.

**Owed:** it goes away when the wizard replaces the placeholder, because the
wizard asks. Until then, nothing should be read into a mock rotation.

**Also deferred:** `photoAttached` on the draft reaches the write and is
dropped — `photoPath` is always null, and `Written` still reports success. §03
routes photos straight to object storage under a path convention, and the
upload lands with the storage work.

## The mood scale says nothing until the patient says something

**What the prototype does:** `INITIAL_LOG_STATE` seeds `mood: 3`, and
`MoodSlider` renders `labels[value - 1]` unconditionally — so the scale reads
«Ровно» before the patient has touched it, and the third dot is filled in.

**What we do:** `CadenceMoodSlider.value` is `Int?`. Nothing is selected and no
word is shown until one is chosen; the two end captions («Никак», «Светло») stay,
because they label the scale rather than an answer.

**Why:** step 4 of the wizard is «Короткая сверка — всё по желанию», and §03's
`journal_entries.mood` is nullable. A seeded 3 is the wizard putting a word in a
patient's mouth and then writing it to the clinical record as if they had said
it — one of the prototype's bugs rather than one of its behaviours.

Guarded by `StepperAndSliderTest.theMoodSliderShowsNoWordUntilOneIsChosen`.

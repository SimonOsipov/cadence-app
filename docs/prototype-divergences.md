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

## ~~The meal hero arrives without its suggestion~~ — the entry was wrong, 2026-08-14

**What this entry used to claim:** that `MealHero` picks a dish from the recipe
library via `suggestNextMeal(meals, now)`, and that porting the suggestion would
mean seeding a parallel set of dishes.

**What the prototype actually does:** `suggestNextMeal`
(`mobile/src/features/meal/data.ts:244-253`) returns one of four hardcoded
prompts chosen by `meals.length` — «Начнём день / Завтрак? / Целимся в 35 г
белка.» and three more. It reads no recipe, and the `now` it takes is parsed
into an `hour` that the next line discards with `void hour`.

**What we do:** port it as it is — four states by meal count
(`MealHero.kt:63-70`), no dish named.

**What we do differently, and deliberately:** the remaining line is restructured.
The prototype draws one 13sp paragraph, «{ккал} ккал · {белок} г белка осталось.
{meta}» (`TodayScreen.tsx:783-800`); we promote the numbers to a 22sp «Осталось …
ккал · … г белка» and put the suggestion's `meta` on its own body line beneath
(`MealHero.kt:109-118`). The promotion came with the first five-section port
(2026-08-10, `35dfe30`); the nutrition spec's own decision was that it **stays**
as it is, and step 14 added the `meta` line beneath it. `TodayScreenTest.kt:320`
pins the wording («Осталось 960 ккал · 80 г белка»).

**Why this is recorded rather than deleted:** the claim survived two ports and
was cited as the reason a whole card was left unbuilt. The register is read as
evidence about the prototype; an entry that misdescribes it is worse than a
missing one, so the correction stays visible.

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

**Note for the routes still on placeholders** — Learn, Article, Journal, Body,
Recipes, RecipeDetail, Profile, ChatList, ChatThread, LogMeal, RecipeBuilder;
`CadenceShell.kt` is the list that counts, and this one was «seventeen» long
after seven of them had been ported — they still draw placeholders, so
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

**Today.** ~~`TodayMeals` draws totals only — the prototype lists the last three
meals with name, time, item count and kcal, plus an empty state.~~ — paid
2026-08-13 (step 14): `TodayMeals` now draws the last three logged meals with
name, time, item count and kcal, and the empty invitation when there are none
(`MealHero.kt:145-164,235`). The macro legs
are relabelled P/C/F → Б/Ж/У and reordered. `MealHero` uses the paler `sand100`
and turns two pills into text links, with «Записать приём пищи» shortened to
«Записать приём» and «Из рецепта» to «Рецепты». The reorder warning sits at the
bottom of the scroll rather than in the prototype's fourth position — a
low-stock warning demoted below the fold is a behaviour change, and it should be
moved back or defended. The hero drops the `06:42` timestamp and the injection
site from «Записано · правый живот, ротация.».

None of these is a decision anyone made; they are what a port leaves behind when
it is measured by tests rather than by a side-by-side run. Step 11 is that run.

**The dropped timestamp, revisited 2026-08-14.** It stays dropped, and now for a
stated reason rather than by omission. One thing is missing, and it is a
contract, not a label: `TodaySummary` (`TodayRepository.kt:22-51`) carries
`suggestedSite` and `doseLoggedToday` but neither the hour of today's injection
nor the site it went into. No contract the «Сегодня» screen reads returns
either; the hour is exposed nowhere at all — no repository hands back a
`DoseEvent` — while a logged site survives only per vial, on `VialDose`
(`VialDetail.kt:10,42`), which is why the sheet two sentences down has one to
draw. Adding the field belongs to the dosing surface, not to the nutrition port
that walked past it.

The Russian site names are **not** an obstacle, contrary to what an earlier
version of this paragraph claimed: `CadenceBodyZone` (`CadenceBodyMap.kt:49-74`)
carries a `labelRu` for all ten values of `InjectionSite` (`Dosing.kt:14-27`),
`BodyMapTest` pins the mapping as total against `InjectionSite.entries`, and two
screens already render a site through it (`DoseSteps.kt:411`,
`VialDetailSheet.kt:140`).

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

## ~~The vial picker is not ported yet~~ — paid, 2026-08-04

**What the prototype does:** step 2 renders a `VialPicker` under the stepper and
`LogState` carries `vialId`; step 5 names the vial in the review.

**What we did, while this was open:** `DoseDraft` carried no vial, and the write
resolved a vial of that compound itself.

**Corrected, 2026-08-06:** the paragraph here read «the first vial of that
compound», and «One vial set, not two» further down says «the fullest open vial»
and cites the test that asserts it. The code agrees with the second —
`CadenceMocks.vialFor` is `maxByOrNull { remainingDoses(...) }` over vials
filtered on `disposedAt == null && openedAt != null`. The claim that it does not
read `disposedAt` was wrong when written and is wrong now; the seed carries four
BPC vials, one of them disposed with six doses still in it, precisely so that
branch is reachable.

**Why:** the plan's field list for the draft is the patient's choices, and which
vial a dose came out of is a choice the app can make correctly on its own while
there is exactly one active vial per compound. It stops being correct the moment
a patient has two open vials of the same compound — §03 allows it and the
prototype's own `VIALS` seed contains it (`v1` and `v2`, both semaglutide).

**Paid, 2026-08-04.** The picker is in the dose step and `DoseDraft` carries a
`vialId`. It draws only when the compound has more than one open vial — one is
not a choice, and the write makes the same one. Its default goes *into the
draft* rather than only onto the screen, so what the patient is shown and what
is recorded are one value rather than two implementations of one rule.

**Still owed:** the syringe barrel. §03 stores `concentration_label` as a label
(«1 мг/мл»), so even with the vial in hand there is no number to divide by, and
parsing a label to draw an instruction about how far to pull a plunger is not a
thing to do. It needs a schema change, not a parser.

## ~~One tap on the placeholder invents a zone, and the rotation reads it back~~ — gone, with the placeholder

**What the prototype does:** its wizard asks. There is no one-tap path.

**What we did, until task 6 of the dose-wizard plan:** the «Записать дозу»
placeholder had one button, and `submit` refuses an injection with no zone, so
`TodaySummary.oneTapDraft()` sent the rotation's own suggestion.

**Struck 2026-08-06.** `oneTapDraft` has no definition and no caller anywhere in
`kmp/` — the ported wizard replaced it, and the index entry four sections below
already said so while this one still described it in the present tense.

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

## What the wizard's five steps do not draw yet

The chrome, the five bodies and the review are ported. These are missing, and
step 11's side-by-side run is where they get decided rather than discovered:

**Step 2.** The vial picker, and with it the review's «Флакон · N доз» row —
see «The vial picker is not ported yet». The syringe barrel is drawn only when
`DoseOption.syringeUnits` is known, which is nowhere today: the reading depends
on the vial's reconstitution. The prototype computes
`(dose / compound.default) * syringeFill`, which draws the ratio to the
prescription rather than a volume — right for Семаглутид and wrong for the other
three of its own compounds, so it is not ported. The subtitle also drops the
«— 0,25 мг» the prototype appends.

**Step 4.** The «Ничего» chip, which in the prototype clears the others and
makes the review read «Без замечаний» rather than «—». The note box has no
`minHeight: 70`.

**Step 5.** The `{date} · {time}` eyebrow over the hero card; the inner sand
hairline frame; the closing «Нажмите Сохранить дозу…» paragraph. The unit is set
in body face beside the number rather than inside the italic display title.

**Chrome.** `FadeIn` between steps; the footer's gradient scrim; the arrow icon
on «Дальше» and the chevron and `sunk` background on «Назад»; the prototype's
warm confirmation sheet after saving, which auto-dismisses after 2,6 s.

**The disabled button** is alpha only. The prototype also swaps the background
to `sunk` and the label to `subtle`, so its dead button reads grey where ours
reads like a faded primary.

## The dose is `{value, unit}`, and the prototype's string is not ported

**What the prototype does:** `COMPOUNDS` in `log-dose/data.ts` carries
`default: '0.25'` as a **string**, `LogState` holds `dose: string` beside
`unit: string`, and `DoseStepper` does `parseFloat` on every tap and hands back
`n.toFixed(2).replace(/\.?0+$/, '')`. The rendered comma is produced by
`fmtDose`, which is a `String.replace` over the same value.

**What we do:** `Dose(value: Double, unit: DoseUnit)` from `shared`, everywhere.
`DoseDraft` holds it, `DoseEvent` stores it, `CadenceDoseStepper` does the
arithmetic on the number and `formatDose` returns the two runs the screen sets
in two faces. Nothing between the protocol and the screen holds «0,25 мг».

**Why this is a correction and not a divergence:** it is the subtask's one
explicit prohibition — «Доза хранится `{value, unit}`, а не строкой "1,0 мг".
Форматтер один на поверхность.» It is written here because this file is where
the next reader looks, and because the arithmetic changed with it: the stepper
rounds to three places where the prototype's `toFixed(3)` does, and truncating
instead would record 0,399 while the screen read «0,4 мг».

## What the dose wizard still owes

One list, so step 11's side-by-side run has somewhere to start. Each is written
out where it was made; this is the index.

- **The photo upload.** The slot renders and reports a tap, `DoseDraft` carries
  `photoAttached`, and the write drops it — §03 routes photos straight to object
  storage under a path convention, and that lands with the storage work.
- ~~The vial picker~~ — paid 2026-08-04. What remains of it is the review's
  «Флакон · N доз» row and the syringe barrel, which needs a concentration §03
  stores only as a label.
- **The warm confirmation sheet** after «Сохранить дозу», which the prototype
  shows for 2,6 s before returning.
- **The «Ничего» chip** and the review's «Без замечаний».
- **`FadeIn`** between steps, the footer's gradient scrim, and the two button
  icons.
- **A one-tap path that invents a zone** is gone: the placeholder that did it is
  replaced by the wizard, which asks.

## «Добавить флакон» asks for a concentration, not a dose

**What the prototype does:** `AddVialScreen.tsx` asks «Дозировка» with the
placeholder «0,25 мг» and stores the string on the vial, beside a `remaining`
the form also sets.

**What we do:** the form asks «Концентрация» («1 мг/мл») and «Сколько доз», and
the vial arrives sealed with no remaining count at all.

**Why:** the per-dose amount is the protocol phase's answer — one function,
`phaseDose`, read by the calendar, the Today strip and the dose wizard. A vial
carrying its own copy is a derived value stored, and the two go out of step the
first time a doctor titrates. What §03's `vials` holds is `concentration_label`.
And `remaining = total_doses − count(events)` on every read, so a vial nothing
has been drawn from has all of them by arithmetic rather than by assignment.

Guarded by `AddVialScreenTest.theFormAsksForNoDoseBecauseTheProtocolDecidesIt`
and `MockInventoryTest.anAddedVialComesBackFullBecauseNothingHasBeenDrawnFromIt`.

## The vial sheet's two unbuilt actions say so

**What the prototype does:** «Прикрепить фото» and «Перенести в запас» are live
rows that call `onClose`.

**What we do:** both render and are disabled.

**Why:** a row that looks live and does nothing is worse than one that says it
is not ready. The photo goes with the storage work, like the dose wizard's; a
transfer back to sealed stock needs a rule about what happens to the doses
already drawn, which nobody has written yet.

## One vial set, and what that replaces

**What the prototype does:** ships two. `inventory/data.ts` has
`VIAL_INVENTORY` — five vials with `remaining` typed on each and a `recent`
array per vial — and `log-dose/data.ts` has a different `VIALS` with three, its
own ids and its own numbers. Nothing reconciles them, and logging a dose in the
wizard decrements neither. §03 calls this its third correction.

**What we do:** `MockSeed.vials` is the one set, read by the cabinet, the dose
wizard's picker, the Today hero's «доз осталось» and the reorder hint. Every
remaining count is `total_doses − count(dose_events.vial_id)`, computed on read;
`Vial` has no field to store one in and no `status` either.

**Why it is here even though it is a correction:** it is the subtask's
prohibition, and this file is where the next reader looks. It is also the reason
the seed grew a history — «остаток выводится из залогированных доз» has nothing
to derive from until something has been logged.

Guarded by `MockSeedInventoryTest.noSeededVialCarriesARemainingCountThatWasTyped`
and `MockRepositoryTest.aDoseIsDrawnFromTheFullestOpenVialOfItsCompound`.

## What «Аптечка» does not draw yet

**The cabinet.** No per-compound filter chips — the prototype appends one chip
per compound after «Все / Активные / Истекают / Запас», and ours stops at the
four states. No compact row: the prototype has both a `VialCard` and a slimmer
`VialRow`, and the sealed accordion uses the slim one; ours draws cards in both
places. No `WarningCard` band inside the summary — the reorder warning lives on
the Today screen and is not repeated here.

**The sheet.** «Изменить лот, дату или дозу» is dropped rather than disabled:
the prototype's own row wires to `onClose`, so it is a button that does nothing
in the design too, and porting a no-op as a disabled row would claim a feature
that was never specified. «Прикрепить фото» and «Перенести в запас» are present
and disabled — see the entry above.

**The form.** No label photo: same storage exception as the dose wizard's, and
the same deferral. The expiry is typed as a date («2026-12-01») rather than the
prototype's «14 сен» label — a date the app can compare against today is what
the refusal needs, and a formatter renders it back on the card.

Step 11's side-by-side run is where these are decided rather than discovered.

## Тренды: талия прототипа — в дюймах под ярлыком «см»

`mobile/src/features/trends/data.ts` держит `makeWaist()` в диапазоне 37,5 → 35,0
с `unit: 'см'` и `baseline: 37.5`, рядом с бедром `makeThigh()` 64,5 → 60,0 в тех
же «см». Для роста 188 см талия в 37 см невозможна, а бедро в 64,5 — нормально;
дальше это подтверждает собственный копирайт прототипа: у бедра «уже на четыре с
половиной сантиметра» совпадает с падением ряда на 4,5, а у талии «ушёл вниз на
шесть с половиной» совпадает с падением на 2,5 — то есть на 2,5 **дюйма**, что и
есть 6,35 см.

**Что сделано:** ряд не переносится ни как есть, ни пересчётом. Сид описывает
своего пациента (188 см, 101,9 → 98,4 кг): талия 104,0 → 99,0 см, бёдра
108,0 → 105,0 см. Конвенция проекта — «Tape measurements are centimetres».

**Смежное:** у прототипа есть `thigh`, которого нет в §03. `HIP` — не
переименованное бедро: метрики бедра в модели нет вовсе. `CHEST` намеренно
остаётся непромеренным — критерий «метрика без измерений говорит об этом»
требует метрики, у которой их нет.

**Поправка.** Ранняя редакция этой записи утверждала, что `HIP` и `CHEST` у
прототипа отсутствуют. Это неверно: их нет в модуле трендов, но они есть на
экране «Тело» — см. «Тренды: копирайт для `HIP` и `CHEST` у прототипа есть — на
другом экране» ниже.

Guarded by `MeasurementSeedTest.theTapeIsInCentimetresAndOnARealBody` и
`everyMetricButChestHasBeenMeasured`.

## Тренды: у прототипа нет числовых порогов

Подзадача блока 0 описывает «пороги биомаркеров (HRV ≥58, пульс ≤60, сон ≥75)»
как константы клиента, которые переезжают в один модуль. В прототипе их нет:
58, 60 и 75 — это **значения** сидовых рядов (`data.ts:248`, `:75`), а не
границы. Прогресс там считается относительно: `isProgress` сравнивает знак
разницы с полем `direction` ('up' | 'down'), и абсолютной нормы не существует.

**Что сделано:** в один модуль переезжает то, что у прототипа действительно на
метрику — направление прогресса, единица, знаки после запятой, RU-название и
цветовая семья графика (`accent`: 'forest' | 'sand'). Числовые пороги отложены:
§11 назначает их серверному модулю констант, которого ещё нет.

## Тренды: вес в сиде старше приёма в клинику

Восемь литералов веса идут с 12 апреля, тогда как `profile.joinedAt` — 20 апреля.
Пациента взвесили за восемь дней до того, как он появился в клинике. Литералы не
трогаются: на них стоят две мутационные ловушки (несортированный сид и восемь
точек против `DEFAULT_POINTS` = 7), и перегенерация уничтожила бы обе. Все
остальные метрики начинаются с приёма.

Записано, а не исправлено; `MeasurementSeedTest.theHistoryStartsOnTheDayThePatientJoinedTheClinic`
фиксирует расхождение так, что оно не может исчезнуть незамеченным.

## Тренды: проза, коуч и корреляции не переносятся

`TREND_DATA` даёт каждому биомаркеру `narrative`, `coach: { lead, emph, tail }` и
`correlations[]` — «Недели с лучшим сном — заметнее минус на весах», «Идут
вместе — это жир, не вода». Ни одно из этих утверждений не выводится из рядов:
корреляция там — поле `strength: 0.62`, записанное руками, а не посчитанное.

**Что сделано:** не переносится ничего из троих. Причина — не «источника нет», а
**«источник ещё не построен»**: §11 назначает их прямо — «correlations & coach
lines from constants module» (`source/architecture-overview-v1.1.md:449`), и это
тот же серверный модуль, куда §03 отправляет пороги биомаркеров. Формулировка
важна: отложенное с названным владельцем возвращается, отброшенное — нет.

## Тренды: «Заметные сдвиги» выводятся, а не портируются

`TrendsScreen.tsx:81-104` — три захардкоженные карточки, и две из трёх называют
то, чего в §03 нет вовсе: «Глубокий сон +24 мин» (метрики глубокого сна в модели
нет) и «Метаболизм стабилен · 1 720» (это питание, не измерение). Уцелевает
форма первой — «HRV вырос на 17 мс · +31% к старту», то есть метрика и её
дельта.

**Что сделано:** секция считается из рядов — метрики с наибольшим относительным
движением за окно, три штуки, по убыванию. Относительным, а не абсолютным:
восемь миллисекунд HRV на базе пятьдесят — сдвиг больше, чем два килограмма на
базе сто, и сравнение сырых чисел сортировало бы по единице измерения.
Прототипное «+31% к старту» с этим согласно, и та же доля показана в строке.

**Что не переносится:** подписи вроде «С момента старта BPC-157» — проза, см.
выше. Иконка у всех трёх одна: у прототипа они свои (`arrow-trending-up`,
`moon`, `fire`), но это часть той же захардкоженной тройки, а выводить иконку из
метрики пока не из чего — `MetricMeta` их не несёт. Направление стрелки берётся
из знака дельты, чтобы падение не рисовалось стрелкой вверх.

Guarded by `TrendsOverviewTest.theShiftsAreRankedByHowFarTheMetricMovedNotByTheOrderOfTheSet`.

## Тренды: прототип противоречит сам себе про день второй титрации

`schedule/data.ts:113-115` ставит шаги на `addDays(CYCLE_START, 4*7)` и `8*7` —
дни 28 и 56. `journal/data.ts:92-93` независимо пишет `TITRATION_DAY = 28` с
комментарием «week 5 = day 28». А модуль трендов рисует второй шаг на **дне 70**,
и дважды: `trends/data.ts:21` (отметка события) и `:33-34` (граница
`DOSE_SPANS`, которую читает `ScrubChart.tsx:165`).

**Что сделано:** фазы §03 (0,25 w1–4 → 0,5 w5–8 → 1,0 w9–12) дают 28 и 56, то
есть расписание и дневник правы, а график ошибается на две недели. Ничего не
портируется: и полосы, и отметки выводятся из `plan.phases` и
`protocol.startDate`. Расхождение перестаёт быть «сид против прототипа» и
становится «прототип против себя».

Guarded by `ProtocolMarksTest.theSeedTitratesOnTheDaysThePrototypesScheduleDoesAndNotItsChart`.

## Тренды: отметки «добавлен препарат» нет, и причина не та, что казалась

`trends/data.ts:20` рисует `{ day: 49, kind: 'add', '+ BPC-157' }`. Спека
объясняла отсутствие тем, что «`ProtocolItem` не имеет даты начала, поэтому это
не выводится ни из чего». Обоснование неверно: день первого назначения
**выводится** — из `fromWeek` первой фазы, и `protocolMarks` его выводит.

**Что сделано:** отметки нет, потому что у BPC в сиде **одна** фаза на все
двенадцать недель, значит его старт — день 0, а прототипный день 49 — литерал
про курс, которого эти данные не описывают. Элемент, действительно начатый по
ходу курса, получит отметку `START` на своём дне; проверено фикстурой wash-in.

Guarded by `ProtocolMarksTest.anItemDosedFromTheSecondWeekIsMarkedWhereItsFirstBandOpens`.

## Тренды: копирайт для `HIP` и `CHEST` у прототипа есть — на другом экране

Спека утверждала, что прототип их не даёт. Даёт: `body/data.ts:37-41` держит
`{ id: 'hip', label: 'Бёдра', unit: 'см', dec: 0, editable: true }` и
`{ id: 'chest', label: 'Грудь', … }`, `BodyScreen.tsx:723` их рисует, `SEED_HIST`
держит их истории (бёдра 116 → 108, грудь 112 → 105). Ошибка была в том, что
спека смотрела только в `features/trends/`.

**Что портировано оттуда:** `label` и «пациент вводит руками» (`editable: true`).
**Чего у экрана «Тело» нет и что выведено из `WAIST`:** `eyebrow`, направление
прогресса, цветовая семья.

**И третье расхождение — знаки после запятой.** Два модуля спорят: «Тело» даёт
`dec: 0` для талии, бёдер и груди, тренды — `decimals: 1` для талии. Таблица
следует трендам: сид меряет ленту десятыми, и округление до целых сантиметров
уплощает график — семь недельных значений бедра дают шесть шагов при одном знаке
и три при нуле, то есть половина истории становится плато, которого не было.
Талия теряет один шаг из шести.

**Урок шире одного поля:** прототип разложен по экранам, и один экран — не весь
прототип. Прежде чем писать «прототип этого не даёт», грепается весь
`mobile/src/`.

## Тренды: «3 месяца» и «весь цикл» у прототипа — одно и то же окно

`trends/data.ts:144-145` даёт обоим `daysFor(…, 84)` при `TODAY_DAY = 84`: оба
начинаются на дне 0 и кончаются сегодня. Переключатель между ними не переключал
ничего.

**Что сделано:** три окна — длины (7, 28, 84 дня включительно), `CYCLE` — якорь
на `protocol.startDate` и ограничен последним прописанным днём. На сиде это
7 / 28 / 84 / 22 дня, четыре разных ответа. Два окна частичны, и это описано, а
не спрятано: «3 месяца» просит 84 дня истории и получает 42 прожитых с приёма,
«цикл» — 22 дня из 84 прописанных.

Guarded by `TrendWindowTest.theCycleIsAnchoredOnTheProtocolRatherThanCountedBackFromToday`.

## Тренды: hero не помнит, какую метрику открывали последней

`AppState.tsx:76` держит `activeBiomarker` (`useState('weight')`), и
`TrendsScreen.tsx:188` featured-карточкой рисует именно его — то есть последнюю
открытую.

**Что сделано:** hero — первая метрика набора, всегда вес. «Что я смотрел в
прошлый раз» — второй вид памяти приложения, и хранить его негде до блока
профиля; экран остаётся функцией от своих данных.

**Заодно не переносится баг прототипа:** его hero берёт **значение** из
`series.cycle`, игнорируя выбранное окно, а дельту — из выбранного. Переключение
на «7 дней» меняет пилюлю под числом, которое не двигается. У нас оба приходят
из одного ряда.

Guarded by `TrendsScreenTest.theHeroTakesItsNumberFromTheWindowAndNotFromTheWholeCycle`.

## Тренды: пометок к записям нет

`TrendRecentEntry` несёт `note` («Утром · натощак», «День в дороге»), и
`TrendDetailScreen.tsx` рисует её второй строкой. `Measurement.note` в §03 есть,
но в сиде пуст у всех записей.

**Что сделано:** поле читается, и когда оно непустое — рисуется; место под него
не резервируется. Пустая вторая строка в каждой записи выглядела бы как текст,
который не загрузился.

## Тренды: чего три экрана не рисуют

**Список.** Нет спарклайна в цвете семьи для каждой карточки в том виде, в каком
его даёт прототип (`sparkColors` с двумя заливками) — у нас одна заливка на
семью. Нет подписи «НЕД 12 / 12» над «Заметными сдвигами».

**График.** Нет подписей дней под осью (`axisLabel` прототипа: «Вс», «3 нед»,
«нед 9», «сегодня»), нет всплывающей подсказки над скрабом с датой и значением,
нет ярлыков-пилюль у отметок протокола (`AnnLabel` с их разведением по вертикали
при сближении). Нижний отступ под подписи оставлен.

**Экран метрики.** Нет блока корреляций и коуч-строки — см. запись про прозу.
Нет цели (`goal`) на графике: §03 хранит `targetWeightKg` только для веса, а
`MetricMeta` цели не несёт.

**Оболочка.** ~~Вкладка «Тренды» на живых данных остаётся без нижнего таб-бара:
бар живёт внутри экранов, а `TrendsScreen` его не рисует. Тот же долг у аптечки
из предыдущего блока — две вкладки из четырёх на портированных экранах без
выхода. Сторож `onlyTheFourBarDestinationsCarryTheBar` этого не видит: он
запускается на заглушках.~~

**Закрыто.** Долг дожил до первого запуска на симуляторе и оказался не
косметикой: на iOS системного «назад» нет, ни один из двух экранов своей кнопки
не рисует — вкладка была тупиком, из которого выходили перезапуском приложения.
`VialsScreen` и `TrendsScreen` получили `onSelectTab`/`onOpenActions` и рисуют
`CadenceTabBar` сами, как `TodayScreen`; бар остаётся внутри экранов, как у
прототипа. Кнопки «назад» на этих двух вкладках по-прежнему нет — у прототипа
её тоже нет.

Сторож переписан не был — он и не мог поймать: `startShell()` поднимает
оболочку без данных, где все четыре вкладки заглушки, а `PlaceholderScreen`
свой бар рисует. Проверка перенесена на уровень живых данных, в
`CadenceShellDataTest`: три теста, каждый с названной мутацией
(`CadenceTabBar` убран из аптечки, из трендов, `active` подменён константой).

## The demo runs on a fixed day, not on the clock

**What the prototype does:** its fixtures are literals against a frozen «31 May»,
and nothing in it reads a clock at all.

**What we do:** `CadenceApp` builds its mocks on `FixedCadenceClock.at(
MockSeed.DEMO_NOW)` — 31 May 2026, week 4 — instead of `SystemCadenceClock`.

**Why:** the seed is a twelve-week course starting 10 May 2026, and every vial,
measurement and dose event below it is a literal hung off that date. Read the
real clock and the whole fixture ages out at once: `cycleWeek` returns null past
`lastPrescribedDay`, and that null is a hard gate in `occurrencesFor`,
`weekProtocolRows` and `phaseDose` — so the app does not degrade, it goes blank.
It did. The course ended on 1 August 2026 and from the 2nd every screen was
empty, with 537 tests green: every test winds its own clock, and only `App()`
used the real one.

Making the seed follow the clock instead would mean regenerating six weeks of
measurements and a dose history on every launch, and the fixtures the suite
asserts against are the point of the seed. The pin is temporary in the honest
sense: it goes when the Ktor client arrives with a course that is actually
current. `AppTest.theAppOpensInsideTheSeededCourseRatherThanPastItsEnd` is the
one assertion that fails if it is removed.

**Watch for:** any test that reasons about «today» through `App()` rather than
through its own `FixedCadenceClock` is reading 31 May, not the wall clock.

## Мастер дозы открывался, ничего не выбрав, и говорил обратное

**Что делает прототип:** `INITIAL_LOG_STATE` (`mobile/src/features/log-dose/
data.ts:101`) открывается заполненным — `compound: 'sema'`, `dose: '0.25'`,
`vialId: 'v1'`, — и `nextDisabled: !state.compound` делает «Дальше» активной с
первого кадра. Подпись шага 1 «Сегодняшняя доза уже выбрана.» описывает именно
это состояние.

**Что делали мы:** подпись перенесли дословно, предвыбор — нет. `LogDoseModal`
строил `DoseDraft(vialId = openedVial)`, то есть `itemId = null`. Пациент видел
утверждение, что выбор сделан, два пустых радиокружка и «Дальше», которая не
срабатывает, без единого слова о причине. Нашлось на первом же запуске в
эмуляторе; ни один из 294 тестов этого не проверял, а
`finishingTheWizardLogsTheDoseAndReturnsToToday` обходил дефект, кликая
«Семаглутид» первым действием.

**Что теперь:** черновик открывается на `summary.nextDose`, взятой целиком —
`itemId`, `kind` и доза в силе на этот день. Не своё правило: это то самое поле,
из которого герой «Сегодня» и лист действий строят «Семаглутид · 0,25 мг ждёт»,
так что три поверхности не могут разойтись в том, что сейчас ждёт. Дозу берём
из окказии, а не пересчитываем: `phaseDose` уже её посчитал, и второй вызов —
это второй источник правды.

**Чего это не чинит.** `nextDose` — не «доза, которая сейчас должна». В моке
(`CadenceMocks.kt:133`) это `todays.firstOrNull { it.itemId == semaItemId }`,
то есть сегодняшняя окказия недельной инъекции и ничего больше;
`nextDoseCompound` там же — безусловный `MockSeed.semaglutide`. В день, когда
инъекции нет, а BPC-157 положен дважды, предвыбирать по-прежнему нечего, и
подпись шага 1 по-прежнему лжёт. Какое правило должно действовать в такой день,
этот порт не знает — прототип не даёт ответа, потому что его фикстура всегда
воскресенье. Пин на текущее поведение:
`CadenceShellDataTest.onADayTheHeroHasNoDoseTheWizardStillPreselectsNothing`.

**Побочный эффект, который стоит помнить:** `DoseDraft.selectItem` идемпотентен
по замыслу (`DoseDraft.kt:126`), поэтому клик по уже предвыбранному препарату
теперь не делает ничего. Три теста, которые начинались с такого клика, после
правки идут другим путём — и именно они убили мутацию `dose = null`.

## Golos Text и Cormorant не доезжают до Android

**Что происходит:** APK не содержит ни одного `.ttf` и вообще не имеет каталога
`assets/`. Весь текст на Android рисуется системным Roboto — и заголовочный
Cormorant Garamond, и основной Golos Text, который выбран ровно ради кириллицы.
На iOS всё правильно, поэтому дефект и дожил до первого запуска эмулятора.

**Причина, измеренная:** задача `copyAndroidMainComposeResourcesToAndroidAssets`
зарегистрирована, но её `outputDirectory` никогда не конфигурируется — запуск
напрямую падает с «property 'outputDirectory' doesn't have a configured value».
Compose Multiplatform 1.11.1 задаёт его через variant-API AGP для связки
`androidTarget()` + `com.android.library`; `composeApp` собран на новом
`com.android.kotlin.multiplatform.library` (AGP 9.3.1), и на нём хук не
срабатывает. В `build/kotlin-multiplatform-resources/` есть только `iosArm64` и
`iosSimulatorArm64`.

**Что проверено и не подошло:**

- `com.android.library` рядом с KMP-плагином — AGP 9 отвергает прямо: «not
  compatible with the 'org.jetbrains.kotlin.multiplatform' plugin since AGP 9.0».
- Документированный обход AGP (`android.builtInKotlin=false`,
  `android.newDsl=false`) — каскадит: `shared` тоже написан на `kotlin { android
  { } }`, которого в старом DSL нет, и перестаёт компилироваться. Это миграция
  всего репозитория, против рекомендации самого AGP.
- Compose Multiplatform 1.12.0-beta03 (новее стабильной 1.11.1 нет) — требует
  `compileSdk 37`, проект на 36.

**Чего не поймал сьют:** `BundledFontTest` читает байты файлов и проверяет
покрытие кириллицы в `cmap` — он ничего не говорит о том, применилось ли
семейство. И гоняется он только на iOS-таргете: у `composeApp` host-тестов на
Android нет по решению в `build.gradle.kts`.

## Питание: фото и голос показывают заготовку, а порт — нет

**Что делает прототип:** `onCapture` (`LogMealScreen.tsx:525-529`) и удержание
кнопки голоса (`:641-646`) через 1100 мс отдают `SAMPLE_PARSES[1]` и
`SAMPLE_PARSES[2]` — тот же разбор, что и текст, только без текста. Камера и
микрофон не задействованы.

**Что делаем мы:** оба режима видны и помечены «Скоро», а внутри — одна строка,
называющая выход: «Распознавание снимка / голоса пока не работает — опишите еду
текстом».

**Почему:** инвариант nutrition №5 запрещает подставлять заготовленный результат
вместо распознавания. Прототип рисует успех там, где ничего не произошло; порт
этого не повторяет. Решение записано в спеке от 2026-08-11.

## Питание: ручного ввода позиции нет ни у прототипа, ни у порта

**Что делает прототип:** позиции появляются только из разбора. Ни одного
элемента «добавить позицию» в `LogMealScreen.tsx` нет — их можно править
(граммы) и удалять, но не заводить.

**Что делаем мы:** то же самое.

**Почему это записано:** инвариант nutrition №4 говорит про ручной ввод, и до
M9 он **неизмерим** — не потому, что порт что-то упустил, а потому, что
поверхности, на которой это проверяется, не существует ни у одной из сторон.

## Питание: время приёма берётся из часов, а не из трёх литералов

**Что делает прототип:** три литерала, не два. Шапка мастера прибита целиком —
«08:42 · вс 24 мая» (`LogMealScreen.tsx:133`), то есть и время, и дата. Запись
приёма из мастера отдаёт `time: '08:42'` (`:365`). Приём из рецепта получает
`DAY_STATES[DAY_STATE].now`, то есть `'13:14'` для дневного состояния
(`AppState.tsx:134`, `meal/data.ts:165`) — два разных фальшивых времени в одной
сессии.

**Что делаем мы:** приём штампуется часами (`MockNutritionRepository.log`
читает `clock.now()` один раз — и для записи, и для дневных итогов), а мастер
рисует то же чтение в своей шапке.

**Почему:** дата и время — данные о пациенте; литерал здесь не «косметика для
демо», а запись, которая переживёт демо. Часы у приложения закреплены отдельно
(см. «The demo runs on a fixed day»).

## Питание: неделя считается из сида, а не из шести констант

**Что делает прототип:** `weekHistory = [1685, 1742, 1610, 1820, 1455, 1690,
totals.kcal]` и такой же массив по белку (`NutritionScreen.tsx:407-410`) —
шесть литералов и седьмой день из настоящих итогов.

**Что делаем мы:** сеем недельную историю приёмов литералами от `DEMO_NOW` и
считаем колонки из неё (`NutritionRepository.week`).

**Почему:** иначе «средний белок за неделю» — это среднее пяти выдуманных чисел
и одного настоящего, то есть число, которое нельзя ни проверить, ни объяснить
пациенту.

## Питание: коуч вычисляется и не рисуется — ни у прототипа, ни у нас

**Что делает прототип:** `appendMeal` зовёт `pickMealCoach(...)` и кладёт
результат в `mealCoach` на контексте (`AppState.tsx:113`, `:79`, `:185`). Ни
один экран его не читает: во всём `mobile/src` три упоминания, и все три внутри
`AppState.tsx`.

**Что делаем мы:** не портируем — переносить нечего.

**Почему это записано:** `MEAL_COACH_LINES` в `meal/data.ts` выглядит как
готовая фича, и следующий читатель прототипа решит, что порт её потерял.

## Питание: «⋯», «N из 4» и «↑ 6 г к прошлой» — три надписи без источника

**Что делает прототип:**

- `IconBtn name="ellipsis-horizontal"` в шапке «Питания»
  (`NutritionScreen.tsx:435`) — единственная кнопка в этой строке без
  `onPress`; соседний шеврон его имеет.
- `{meals.length} из 4` (`:544`) — четвёрка нигде не задана: ни в целях, ни в
  протоколе, ни в §03.
- `<Pill tone="forest">↑ 6 г к прошлой</Pill>` (`:642`) — литерал; сравнения с
  прошлой неделей у прототипа нет.

**Что делаем мы:** «⋯» не рисуем, «N из 4» не рисуем, пилюлю не рисуем.

**Почему:** каждая из трёх — утверждение о пациенте, за которым нет данных.
Нарисовать их означало бы придумать четвёртую цель, дельту к прошлой неделе и
меню, которого нет.

## Рецепты: у строки конструктора появился потолок граммов

**Что делает прототип:** `setGrams` строки ингредиента зажимает только снизу —
`Math.max(5, g)` (`RecipeBuilderScreen.tsx:383`). Сверху ограничения нет.

**Что делаем мы:** те же 5…600 с шагом 10, что и в листе выбора ингредиента.

**Почему:** лист и строка правят одну и ту же величину; разные границы у двух
входов в одно поле — это две разные модели одного факта. 600 г взяты из листа,
где потолок у прототипа есть.

## Рецепты: `cookMin` пишется null, а не нулём

**Что делает прототип:** при сохранении рецепта `cookMin: 0`
(`RecipeBuilderScreen.tsx:400`), хотя форма спрашивает только одно время.

**Что делаем мы:** `prepMin` = введённое время, `cookMin = null`.

**Почему:** ноль утверждает «готовится ноль минут». Null говорит «не
спрашивали» — и это правда о форме.

## Рецепты: фильтры — прокручиваемые чипы, а не сегмент-контрол

**Что делает прототип:** два горизонтальных `ScrollView` с пилюлями по ширине
содержимого (`RecipesScreen.tsx:279-308`).

**Что делаем мы:** так же — хотя `CadenceSegmented` уже существовал и напрашивался.

**Почему это записано:** замер, а не вкус, и с шириной, при которой он взят
(`RecipesScreen.kt:329-335`). При строке 358dp — это 390pt-телефон минус
по 16dp с каждой стороны — сегмент-контрол отводит «Мягкие для желудка» 72dp при
естественных 132dp; при 343dp (375pt, iPhone SE) обрезаются ещё «Завтрак» и
«Перекус». Три значения из девяти нечитаемы на обычных телефонах. Фильтр, чьи
значения нельзя прочитать, сломан, а не просто некрасив.

## Рецепты: граммы сбрасываются только при выборе **другого** продукта

**Что делает прототип:** `pick` зовёт `setGrams(100)` безусловно
(`RecipeBuilderScreen.tsx:130-132`), поэтому повторное нажатие на уже выбранный
продукт тоже сбрасывает набранные граммы.

**Что делаем мы:** сброс только при смене продукта.

**Почему:** повторное нажатие на выбранную строку — не выбор нового продукта, а
подтверждение прежнего; терять на нём набранные 250 г нечем оправдать. Обе
стороны закреплены тестами, чтобы «улучшение» не уехало обратно.

## Рецепты: переключатель «На порцию / Всё» стоит своей строкой

**Что делает прототип:** переключатель сидит справа от надзаголовка «Макросы»
(`RecipeDetailScreen.tsx:236-272`).

**Что делаем мы:** `CadenceSegmented` во всю ширину под надзаголовком; плитки
меток героя — `palette.paper` при альфе 0,7 вместо прототипных
`rgba(255,255,255,.5)`.

**Почему:** спека называет `CadenceSegmented` прямо, а он по построению делит
ширину поровну; ~52dp по высоте здесь ничего не ломают. Сырой белый не
переносится по правилу «только токены» — по всему `screens/` `grep "Color(0x"`
не находит ничего.

## Рецепты: степперы конструктора стоят один под другим

**Что делает прототип:** «Порций» и «Время, мин» — две карточки в ряд
(`RecipeBuilderScreen.tsx:585-647`), а степпер граммов в строке ингредиента
стоит рядом с названием и удалением (`:716-763`).

**Что делаем мы:** обе карточки одна под другой, степпер граммов — на своей
строке под названием. Панель «Сохранить рецепт» — сплошной `palette.bg` без
градиентной заливки прототипа (`:897-928`).

**Почему:** замер при 343dp. `CadenceStepper` — это две кнопки по 52dp вокруг
числа с отступами 20dp, то есть ≈150dp минимум; половина экрана даёт карточке
~130dp, а строка «имя + степпер + удаление» — ещё меньше. В шаге 11 такая же
раскладка расплющила кнопку «плюс» до 2dp. Градиент не переносится по той же
причине, что и в карточке рецепта: сплошная подложка уже принята для нижних
панелей порта.

## Питание: у позиции появилось число калорий — у прототипа оно пустое

**Что делает прототип:** строка разобранной позиции (`ParsedItem`,
`LogMealScreen.tsx:817-830`) рисует правым столбцом мо́но-текст 16sp, внутри
которого **только пробелы**, и следом подпись «ккал». Три бейджа макросов и
граммы у позиции есть, калорий — нет: привязка не написана. Ошибка не видна в
прототипе, потому что слово «ккал» на месте и колонка выглядит занятой.

**Что делаем мы:** каждая позиция рисует свои калории
(`LogMealItemsList.kt:192`), и у каждой строки свой тег, чтобы тест мог
потребовать два разных числа у двух разных позиций
(`logMealItemKcalTag(0) != logMealItemKcalTag(1)`).

**Почему это записано:** дефект прототипа, а не решение — по §08 такие не
переносятся. Отмечено отдельно, потому что «пустая колонка» легко читается как
намеренная сдержанность вёрстки, и следующий читатель прототипа воспроизведёт
её обратно.

## Рецепты: калории и белок строки — один текст, а не два цветных

**Что делает прототип:** строка рецепта рисует два мо́но-текста подряд —
калории в `pal.ink2` и белок в `C.forest700`, между ними точка 3px
(`RecipesScreen.tsx:107-117`).

**Что делаем мы:** один текст «{ккал} ккал · {белок} г белка» в `palette.ink2`
(`RecipeRow.kt:221-223`).

**Почему:** цвет здесь ничего не кодирует — он не отличает «хватает» от «мало»,
это просто акцент на второй половине составного числового предложения. Тот же
выбор уже сделан на строке остатка `MealHero` и на мета-строке карточки приёма;
третий способ рисовать одно и то же был бы расхождением внутри порта.

## Питание: футер мастера прокручивается, а у прототипа прибит к низу

**Что делает прототип:** «Сохранить · {N} ккал» лежит поверх содержимого —
`position: 'absolute', bottom: 0` внутри `LinearGradient`
(`LogMealScreen.tsx:344-394`), а прокрутка резервирует под него
`paddingBottom: 130` (`:99`). Кнопка видна всегда.

**Что делаем мы:** футер — последний ребёнок той же прокручиваемой колонки
(`LogMealScreen.kt:265-271,381`), поэтому при нескольких разобранных позициях
он уходит ниже сгиба.

**Почему это записано как долг, а не как решение:** решения тут никто не
принимал. Порт держит нужную форму на соседнем экране — у конструктора рецепта
панель зафиксирована, а прокрутка кончается замеренным зазором
(`RecipeBuilderScreen.kt:124-131`, `SAVE_BAR_CLEARANCE`, и тест
`theSaveBarFitsInsideItsOwnClearance` его держит). Та же форма нужна и здесь:
`Box` с прокруткой под фиксированной панелью и зазор в хвосте колонки. Главное
действие экрана записи не должно требовать прокрутки, чтобы его увидеть.

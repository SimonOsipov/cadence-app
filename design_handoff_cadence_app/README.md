# Handoff: Cadence — peptide tracking mobile app

## Overview

**Cadence** is a mobile app for people running peptide protocols (semaglutide, BPC‑157, TB‑500, tesamorelin, etc.) for wellness, recovery, or recomposition goals. It combines dose logging, an injection schedule, 12‑week cycle planning, vial inventory, biomarker tracking, a wellbeing journal, body metrics, a calorie tracker, a recipe creator, an education library, and a care‑team chat — wrapped in a warm, coach‑like voice.

This bundle is the **full interactive prototype**: a single‑device (iPhone) click‑through where the **Today** screen is home and every feature opens as a sliding overlay on top of it. The UI copy is in **Russian (Cyrillic)** by design — the brand voice is coach‑like and warm. Keep the copy as‑is unless instructed otherwise (it is real product copy, not lorem).

> **Tagline:** *Your rhythm, in cadence.*

---

## About the design files

The files in this bundle are **design references created in HTML/React + Babel** — a prototype that demonstrates the intended look, motion, and behavior. They are **not production code to ship directly.** Babel‑in‑the‑browser, global `window.*` component registration, and inline style objects are prototype conveniences, not patterns to carry into production.

Your task is to **recreate these designs in the target codebase's environment** using its established patterns and libraries:
- If there is an existing app (React Native / Expo, SwiftUI, Flutter, Jetpack Compose, etc.), rebuild the screens with that stack's idioms and the team's component library.
- If no codebase exists yet, choose the most appropriate framework for a production mobile app and implement there.

The prototype is the **source of truth for visual design, layout, copy, motion, and interaction logic.** The design tokens in `design-system/colors_and_type.css` are the source of truth for color, type, spacing, radius, shadow, and motion.

---

## Fidelity

**High‑fidelity.** Final colors, typography, spacing, motion, and interactions. Recreate the UI faithfully using the codebase's existing primitives. Pixel measurements, hex values, fonts, and easing curves below are exact.

---

## How to run the prototype

Open `index.html` from a static server (it loads sibling `.jsx`/`.js`/`.css` files, so `file://` will be blocked by CORS):

```bash
cd design_handoff_cadence_app
python3 -m http.server 8000
# open http://localhost:8000
```

The app renders a phone frame centered on a dark stage. A caption under the frame names the current screen.

---

## Architecture (prototype)

Everything mounts into `#root` in `index.html`. Scripts load in dependency order:

1. **React 18.3.1 + ReactDOM + Babel standalone** (pinned, in‑browser JSX transform).
2. **`design-system/heroicons.js`** — vendored Heroicons subset + global `<Icon name=… size=… strokeWidth=… color=…/>` component.
3. **`learn/image-slot.js`** — `<image-slot>` web component (user‑fillable photo placeholder).
4. **`design-system/cadence-components.jsx`** — the brand component library + the two token objects `C` (colors) and `F` (font families). Registers everything on `window`.
5. **`ios-frame.jsx` / `android-frame.jsx`** — device bezels + status bars (`IOSDevice`, `AndroidDevice`).
6. **`tweaks-panel.jsx`** — host‑protocol tweak panel shell (currently wired but exposes no tweaks).
7. **`shared.jsx`** — dashboard atoms: `getPalette(dark)`, `PullToRefresh`, `CadenceTabBar`, `CadenceAndroidNav`, `BiomarkerSheet`, `CycleRing`, `useCoach`, etc.
8. **`variants.jsx`** — the **Today** home screen (`V1Refined`).
9. **Feature folders** — each ships a `*-data.jsx` (mock data + pure helpers) and one or more screen components.
10. **`prototype.jsx`** — the app shell `PrototypeApp`: holds all navigation state and stacks every screen as an absolutely‑positioned layer.

### Navigation model

`PrototypeApp` keeps a single `screen` string in React state (`'today' | 'log-dose' | 'trends' | 'trend-detail' | 'log-meal' | 'nutrition' | 'vials' | 'add-vial' | 'chat-list' | 'chat-thread' | 'profile' | 'schedule' | 'learn' | 'article' | 'journal' | 'body' | 'recipes' | 'recipe-detail' | 'recipe-builder'`) plus a `prevScreen` for back routing. Today is always mounted underneath; every other screen is a sibling `<div>` layered with `position:absolute; inset:0` and a `transform` toggled by whether it is the active screen:

- **Push screens** (Trends, Nutrition, Vials, Chat, Profile, Schedule, Learn, Journal, Body, Recipes) slide in from the right: `translateX(100%) → translateX(0)`.
- **Modal screens** (Log dose, Log meal, Add vial, Recipe builder) slide up from the bottom: `translateY(100%) → translateY(0)`.
- **Bottom sheets** (action chooser, vial detail, biomarker, quick‑feel, confirm toast) render above everything with a forest‑tinted blurred scrim.

Transition is `transform 380ms cubic-bezier(0.22, 1, 0.36, 1)` everywhere. `pointer-events` is toggled off on inactive layers so they don't capture taps. Each layer sits at an explicit `z-index` (Today implicit 0 → recipes/builder at 62‑66 → modals at 60 → sheets at 80). **In production, model this as a navigation stack** (native navigator / router) rather than a pile of always‑mounted divs — the layering is a prototype technique.

`platform` is hard‑coded to `'ios'` in `prototype.jsx`; an Android device frame + Material‑3 bottom nav (`CadenceAndroidNav`) already exist and switch the whole UI when set to `'android'`.

---

## Design tokens

All tokens live in `design-system/colors_and_type.css` as CSS custom properties, and are mirrored as the JS objects `C` and `F` in `cadence-components.jsx`. **Use these — never invent new values.**

### Color

| Role | Token | Hex |
|---|---|---|
| **Primary** (buttons, active tabs, brand) | `--forest-700` | `#2d5f3f` |
| Primary hover | `--forest-800` | `#1f4530` |
| Forest hero card bg | `--forest-800` | `#1f4530` |
| Forest deepest | `--forest-900` | `#142c1f` |
| Forest tints | 600/500/300/200/100/50 | `#3d7a52` / `#4a8161` / `#a6c2af` / `#cadccf` / `#d8e5db` / `#eaf0eb` |
| **Accent** (highlights, warm CTAs) | `--sand-500` | `#d4a574` |
| Sand tints | 900/700/300/100 | `#6b4a25` / `#b8895a` / `#e8d4b8` / `#f3e8d6` |
| Page background | `--cream` | `#f6f1ea` |
| Card on cream | `--paper` | `#fbf8f3` |
| Sunk surface | `--linen` | `#ede5d6` |
| Divider/inset | `--bone` | `#e4dac6` |
| Headline ink | `--ink-900` | `#1a1a1a` |
| Body ink | `--ink-800` | `#2a2a2a` |
| Secondary text | `--ink-600` | `#5c5852` |
| Tertiary / meta | `--ink-500` | `#8a857d` |
| Placeholder | `--ink-400` | `#b3ad9f` |
| Hairline on cream | `--ink-300` | `#d6cfbf` |
| Strong border (inputs) | `--border-strong` | `#cdc1a8` |
| Hairline | `--hairline` | `rgba(26,26,26,0.08)` |
| **Success** | = forest‑700 | `#2d5f3f` (bg `#eaf0eb`) |
| **Warning** (amber) | `--warning` | `#c2780a` (bg `#fbeed1`) |
| **Danger** (terracotta, never red) | `--danger` | `#b8503c` (bg `#f4dfd6`) |
| **Info** (slate) | `--info` | `#4a6b7a` (bg `#dfe6e9`) |

Never pure black or pure white. Semantic colors are warm‑shifted: danger is terracotta, not red; warning is amber. Modal scrim is forest‑tinted `rgba(20,44,31,0.35)` with 4px backdrop blur.

A **dark theme** ("midnight forest") is fully specified in `getPalette(true)` in `shared.jsx` — bg `#0e1e16`, paper `#16291f`, warm‑cream text `#f3ead7`, sand accents. Components receive a `pal` palette object rather than reading CSS vars directly.

### Typography

| Family | Token | Use |
|---|---|---|
| **Instrument Serif** (weight 400; italic for emotion only) | `--font-display` | Display + headlines. Cyrillic falls back to **Cormorant Garamond**. |
| **DM Sans** (400 default, 500 buttons/labels, 600 sub‑heads) | `--font-body` | UI + body. |
| **JetBrains Mono** (`tabular-nums`) | `--font-mono` | All numerics, doses, deltas, stat values. |

Type scale (CSS vars): `--t-display` clamp(56–88px) · `--t-h1` 48 · `--t-h2` 36 · `--t-h3` 26 · `--t-h4` 20 · `--t-lead` 18 · `--t-body` 16 · `--t-small` 14 · `--t-micro` 12. Display/h1/h2 use the serif with tight tracking (`-0.02em` to `-0.01em`); h3/h4 use DM Sans 600. The **eyebrow** microtype is DM Sans 500, 11–12px, `text-transform:uppercase`, `letter-spacing:0.14em`.

Doses are written `0.25 mg` with a thin space; numbers are tabular; no trailing zeros except where dosing precision matters.

### Spacing — 4pt grid (2pt micro)

`--s-1` 2 · `--s-2` 4 · `--s-3` 8 · `--s-4` 12 · `--s-5` 16 · `--s-6` 20 · `--s-7` 24 · `--s-8` 32 · `--s-9` 40 · `--s-10` 48 · `--s-11` 64 · `--s-12` 80 (px). Mobile content gutter is **16px**; cards have **16px** internal pad (hero cards 18–22px).

### Radii

`--r-xs` 4 · `--r-sm` 8 · `--r-md` 12 (inputs, chips) · `--r-lg` **18** (default card — the defining curve) · `--r-xl` 24 · `--r-2xl` 32 (hero panels, modals/sheets 24–28) · `--r-pill` 999 (all buttons, tab pills). Sharp corners (0) only for data tables.

### Shadows (warm umbra — `rgba(46,38,24,…)`, never gray‑black)

- `--shadow-xs` `0 1px 2px /.05`
- `--shadow-sm` `0 2px 6px /.06, 0 1px 2px /.04` — default cards
- `--shadow-md` `0 8px 20px /.08, 0 2px 4px /.04` — menus/popovers
- `--shadow-lg` `0 18px 40px /.10, 0 4px 8px /.05` — sheets/modals

Prefer tone shifts (cream → paper → linen) for hierarchy over heavy shadows.

### Motion

- Entrances: `--ease-out` `cubic-bezier(0.22, 1, 0.36, 1)`
- Transitions: `--ease-in-out` `cubic-bezier(0.65, 0, 0.35, 1)`
- Button press spring: `--ease-spring` `cubic-bezier(0.34, 1.56, 0.64, 1)`
- Durations: `--dur-fast` 140ms (micro) · `--dur` 220ms (default) · `--dur-slow` 420ms (page); screen slides run at **380ms**.
- House move: fade + 4–8px translate. **Press:** scale to `0.98` for ~120ms. **Hover:** primary darkens forest‑700 → forest‑800; cards lift one shadow step. No bouncing, no parallax, no infinite decorative loops.

---

## Shared component library (`design-system/cadence-components.jsx`)

Recreate these as your design system's primitives. Each takes a small prop set:

- **`Eyebrow`** — uppercase microtype label (11px, 0.14em tracking).
- **`Title`** — serif display text; props `size` (default 28), `italic`, `color`.
- **`Body`** / **`Meta`** — DM Sans body (14) / meta (12, ink‑500).
- **`Num`** — `value` + optional `unit`; mono tabular value with serif‑italic unit (unit sized `0.36×`).
- **`Pill`** — small status pill with leading dot; `tone`: `forest` | `sand` | `warning` | `danger` | `neutral` | `dark`.
- **`Chip`** — toggle chip; `active` fills ink‑900/cream.
- **`Btn`** — pill button; `kind`: `primary` (forest fill) | `secondary` (linen) | `ghost` (forest text) | `dark` | `danger` (terracotta outline); `size`: `sm`/`md`/`lg`; `full`.
- **`IconBtn`** — 40×40 round icon button.
- **`Card`** — 18px‑radius surface; `tone`: `paper` | `cream` | `linen` | `forest` (inverted hero) | `sand` | `outline`; `elev`: `none`/`sm`/`md`; `pad`, `radius`.
- **`Section`** — eyebrow title + optional right‑aligned action + children stack.
- **`ListRow`** — `40px icon tile | title+sub | trail+trailSub` grid row; `iconTone`: forest/sand/linen/danger.
- **`TabBar`** (iOS) / **`CadenceTabBar`** / **`CadenceAndroidNav`** — bottom nav with a center primary `+` FAB. Tabs: Today / Vials / **Log (+)** / Trends / Nutrition.
- **`AppHeader`** — meta greeting + large serif name + bell + avatar.
- **`Spark`** — tiny inline trend sparkline (SVG path + end dot).
- **`Icon`** — Heroicons outline, 24/24 viewBox, stroke 1.5, `currentColor`.

**No emoji anywhere in product.** The serif italic carries emotional weight (*lighter, deeper, rhythm, balance, calm*).

---

## Screens / views

Each feature folder pairs a `*-data.jsx` (mock data + pure derive helpers — reusable as your data‑shape spec) with its screen component(s). Routing prop names are listed so you can map the flow graph.

### 1. Today — home (`variants.jsx` → `V1Refined`)
- **Purpose:** the daily hub. Coach line, today's queued dose, cycle progress, biomarker glances, fuel/meal strip, quick entries into every feature.
- **Layout:** `AppHeader` (greeting + name + bell + avatar) → `PullToRefresh` scroll region → stacked cards in a 16px gutter → fixed `CadenceTabBar` at bottom with center `+` FAB.
- **Key pieces:** coach message card (cycles through `COACH_LINES` in `shared.jsx`), today's dose card with a **log dose** CTA (changes to a logged/done state via `doseLogged`), `CycleRing` (week N of 12), biomarker glance cards that open the `BiomarkerSheet`, a fuel strip summarizing meals/kcal.
- **Routes out via props:** `onLogDose`, `onPlusTap` (action chooser), `onOpenTrends`, `onOpenTrend(id)`, `onOpenVials`, `onOpenChat`, `onOpenProfile`, `onOpenSchedule`, `onOpenLearn`, `onOpenJournal`, `onQuickFeel`, `onLogMeal`, `onOpenNutrition`, `onOpenRecipes`.

### 2. Log dose (`log-dose/` → `LogDoseV1`, `wizards.jsx`, `shared-log.jsx`)
- Slides up. Multi‑step wizard to capture a dose: compound, amount (`0.25 mg`), injection site (rotation), time. Confirms and calls `onComplete`, which sets `doseLogged=true` and advances the coach message. `key={logSession}` remounts it fresh each open.

### 3. Action chooser sheet (`prototype.jsx` → `ActionChooserSheet`)
- Bottom sheet from the `+` tab. Two options — **log a dose** / **log a meal** — each an `ActionOption` row (52px icon tile, serif title, contextual sub showing today's status). Cancel button.

### 4. Trends + biomarker detail (`trends/` → `TrendsLanding`, `TrendDetail`, `scrub-chart.jsx`, `trend-data.jsx`)
- Push screen. Landing lists biomarkers (weight, HRV, RHR, sleep, VO₂ max) with sparklines; tapping opens **`TrendDetail`** with a scrubbable line chart, timeframe selector (`1m`/`3m`/`6m`/`1y`), annotations, and a biomarker switcher. `timeframe`/`biomarker` are app state. Links out to Journal and Body.

### 5. Nutrition / calorie day (`meal/` → `NutritionToday`, `meal-today.jsx`, `meal-data.jsx`)
- Push screen. The day's meals, running kcal vs. `MEAL_TARGETS`, macro breakdown, a suggested next meal (`suggestNextMeal`). Entry to log a meal and to recipes. `DAY_STATES` presets seed `baseMeals`; user‑logged meals append to `userMeals`.

### 6. Log meal (`meal/meal-flow.jsx` → `MealLogScreen`)
- Slides up. Chat‑parse mode (`defaultMode='chat'`): type what you ate, it parses into items + totals. `onComplete(meal)` appends the meal, picks a coach line (`pickMealCoach`), and shows the **confirm toast** for 1.7s before returning.

### 7. Recipes (`recipe/` → `RecipeLibrary`, `RecipeDetail`, `RecipeBuilder`, `recipe-data.jsx`)
- Push library of GLP‑1‑friendly, protein‑forward recipes (`RECIPES.STARTERS`) + user recipes. Detail shows ingredients/macros and **add to day** (with portions → `RECIPES.toMeal`). Builder **slides up**, composes a recipe from an ingredient DB, saves via `onSave`.

### 8. Inventory / Vials (`inventory/` → `VialsScreen`, `VialDetailSheet`, `AddVialScreen`, `inventory-data.jsx`)
- Push screen. Active vials with doses‑remaining, expiration, reorder hints (`inventorySummary`). A collapsible **sealed/spare** group (`sealedOpen`). Tapping a vial opens **`VialDetailSheet`** (bottom sheet) with actions: mark opened, log dose from vial, edit, add photo, dispose, move to spare, activate. **Add vial slides up** as a modal wizard.
  - **Note for implementation:** in the prototype several `VialDetailSheet` actions (edit, add photo, dispose, move to spare, activate) currently just close the sheet — they are stubs. Build real destination states / confirm dialogs for these.

### 9. Schedule / 12‑week cycle (`schedule/` → `ScheduleScreen`, `schedule-data.jsx`)
- Push screen. The full protocol calendar — week‑by‑week dosing across a 12‑week cycle with titration ramps, today highlighted, dose/meal status. **Log dose** from here routes back to the wizard.

### 10. Learn / education (`learn/` → `LibraryScreen`, `ArticleReader`, `learn-data.jsx`)
- Push library of peptide profiles, protocol playbooks, recovery stacks (`LEARN`). **`ArticleReader`** is a long‑form reader; articles can link to other articles. Uses `<image-slot>` for hero imagery (user‑fillable placeholder).

### 11. Journal / wellbeing — *Самочувствие* (`journal/` → `JournalScreen`, `QuickFeelSheet`, `journal-data.jsx`)
- Push timeline of wellbeing entries (mood, nausea, fatigue, energy in plain text) with charts. **`QuickFeelSheet`** is a quick‑add bottom sheet; saving replaces today's entry (`JOURNAL.TODAY_DAY`) in `journalEntries`.

### 12. Body / composition — *Тело* (`body/` → `BodyScreen`, `body-data.jsx`)
- Push screen. Body composition, measurements (add via `onAddMeasure`), and progress photos (add via `onAddPhoto`, seeded from `BODY.seed()`). Links into trend detail.

### 13. Chat / care team (`chat/` → `ChatLanding`, `ChatConversation`, `chat-data.jsx`)
- Push list of care‑team threads (`CARE_TEAM`) → conversation view. Default thread `'ksenia'`. Reachable from Today and from Profile.

### 14. Profile + Settings (`profile/` → `ProfileScreen`)
- Push screen. Identity, current cycle, journey stats, care team, membership, and a working **Settings** sub‑screen. Links to chat, trend detail, schedule, journal, body.
  - **Note for implementation:** several Profile rows (Units, Notifications, Privacy & data) all route to the single Settings screen in the prototype — give each its own sub‑page in production.

### Global overlays
- **Confirm toast** — bottom card with an animated check tick after logging a meal.
- **Biomarker sheet** — quick stat + mini chart + "open trend detail" CTA.
- **Pull‑to‑refresh** — custom touch handler in `PullToRefresh` (threshold 70px, spinner) wrapping scroll regions.

---

## Interactions & behavior

- **Navigation:** single `screen` state + `prevScreen` for back. `goBack()` in `prototype.jsx` encodes the back graph (e.g. `trend-detail → trends|today`, `article → learn`, `add-vial → vials`). Reproduce as a real nav stack.
- **Screen transitions:** 380ms `cubic-bezier(0.22,1,0.36,1)`; push = `translateX`, modal = `translateY`. Inactive layers get `pointer-events:none`.
- **Dose logging:** `doseLogged` flips Today's dose card to a done state and advances `coachIndex` (mod `COACH_LINES.length`).
- **Meal logging:** appends to `userMeals`; recomputes day totals (`dayTotals`); selects a coach line (`pickMealCoach`); confirm toast auto‑dismisses after 1700ms. Switching the day‑state preset clears `userMeals`.
- **Modals remount:** Log dose, Log meal, and Add vial use `key={…Session}` so each open starts clean.
- **Press feedback:** `.press` class → `transform: scale(0.98)` on `:active` with spring easing.
- **Pull‑to‑refresh:** only triggers at scrollTop ≤ 0; eases pull at 0.55×, max 110px, fires `onRefresh` after a 900ms spinner.
- **Empty states / photos:** recipe, learn, and body photos use `<image-slot>` placeholders — in production these are real image uploads / empty‑state designs.

---

## State management (prototype → production guidance)

All state currently lives in `PrototypeApp` via `useState`. Group it for production:

| Domain | Prototype state | Notes |
|---|---|---|
| Navigation | `screen`, `prevScreen`, `articleId`, `recipeId`, `chatThreadId`, `vialDetailId` | → router / nav stack |
| Dosing | `doseLogged`, `logSession`, `coachIndex` | dose log entity + coach engine |
| Meals | `userMeals`, `mealCoach`, `confirmSheet`, `mealSession`; derived `meals`, `mealTotals`, `mealHeroSuggestion` | day is `baseMeals (preset) + userMeals` |
| Inventory | `sealedOpen`, `addVialSession`; data in `VIAL_INVENTORY` | |
| Journal | `journalEntries`, `quickFeelOpen` | seeded from `JOURNAL.ENTRIES` |
| Body | `bodyState` (`BODY.seed()`) | measurements + photos histories |
| Recipes | `userRecipes` (+ `RECIPES.STARTERS`) | |

The `*-data.jsx` files define the **data shapes and pure helpers** (`dayTotals`, `suggestNextMeal`, `pickMealCoach`, `inventorySummary`, `RECIPES.toMeal`, `JOURNAL.weekOf`, etc.). Treat them as the spec for your models and selectors; replace the mock arrays with real data fetching.

---

## Content & voice rules (must preserve)

- **Person:** second person (*you*), first‑person plural for coaching (*we'll nudge you*). Never "the user."
- **Casing:** sentence case everywhere except proper nouns (Cadence, Semaglutide). All‑caps only as the 0.14em eyebrow microtype.
- **No emoji, ever.** No exclamation marks — use a confident period. Em‑dashes welcome.
- **Italics** (serif) reserved for emotion: *lighter, deeper, rhythm, balance, calm* — never on a unit, number, or proper noun.
- **Numbers** tabular (JetBrains Mono); doses as `0.25 mg` with a thin space.
- **Tone:** warm, coach‑like, never alarming. *"You skipped two doses. Want to extend by a week?"* not *"You missed 2 doses. This may impact results."*
- The product UI is currently written in **Russian** — keep it unless localization is in scope.

---

## Assets

- **Icons:** Heroicons (MIT, outline 24/24 stroke 1.5) vendored in `design-system/heroicons.js` and exposed as `<Icon>`. Add more by copying outline paths from heroicons.com. In production use the team's icon set (or Heroicons directly).
- **Brand:** `design-system/logo-mark.svg` (forest disc + cadence wave with sand endpoints), `logo-mono.svg`, `wordmark.svg`.
- **Photography (when used):** warm, golden‑hour, slightly‑grainy editorial — hands, food, bodies, light. Never clinical/cool, never lab glassware or syringes. `<image-slot>` placeholders mark where real photos go.
- **Fonts:** Instrument Serif, DM Sans, JetBrains Mono — all Google Fonts (free), imported at the top of `colors_and_type.css`. Cyrillic display falls back to Cormorant Garamond.

---

## Files in this bundle

```
index.html                         # entry — script load order = dependency order
design-system/
  colors_and_type.css              # ALL design tokens (source of truth)
  cadence-components.jsx           # brand component library + C / F token objects
  heroicons.js                     # Icon component + vendored paths
  logo-mark.svg / logo-mono.svg / wordmark.svg
shared.jsx                         # getPalette, PullToRefresh, tab bars, sheets, CycleRing, coach
variants.jsx                       # Today home screen (V1Refined)
prototype.jsx                      # app shell: navigation + layer stacking (READ THIS FIRST)
ios-frame.jsx / android-frame.jsx  # device bezels + status bars
tweaks-panel.jsx                   # tweak-panel host shell (no tweaks exposed)
log-dose/   shared-log.jsx · wizards.jsx · app.jsx
trends/     trend-data.jsx · scrub-chart.jsx · trends.jsx
meal/       meal-data.jsx · meal-flow.jsx · meal-today.jsx · nutrition.jsx
inventory/  inventory-data.jsx · inventory.jsx · vial-detail.jsx · add-vial.jsx
schedule/   schedule-data.jsx · schedule.jsx
learn/      learn-data.jsx · learn.jsx · image-slot.js
journal/    journal-data.jsx · journal.jsx
body/       body-data.jsx · body.jsx
recipe/     recipe-data.jsx · recipe.jsx · recipe-builder.jsx
chat/       chat-data.jsx · chat-landing.jsx · chat-conversation.jsx
profile/    profile.jsx
```

**Start with `prototype.jsx`** (navigation + the full screen graph), then `design-system/colors_and_type.css` and `cadence-components.jsx` (tokens + primitives), then each feature's `*-data.jsx` for data shapes before its screen component.

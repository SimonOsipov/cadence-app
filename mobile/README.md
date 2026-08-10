# Patient app — Expo prototype

> **FROZEN — visual spec, do not extend.**

This directory is the **design specification** for the Cadence patient app: 24 surfaces
(20 screens + 4 bottom sheets), their navigation, the theme and the component set. It is an
Expo / React Native prototype running on hardcoded fixtures — there is no backend, no auth
and no persistence behind it.

It is **not** the product. The shipped patient app is Kotlin Multiplatform + Compose
Multiplatform under `kmp/` — see ADR-001.

## What this is for

- **Read it.** When a screen's behaviour is ambiguous, the prototype is the answer.
- **Port from it**, screen by screen, as each milestone arrives. `src/theme/index.ts` and
  `src/components/` come first (BST-05), the screens follow their own milestones.
- **Run it** when a static read is not enough:
  ```sh
  npm install
  npm run android   # or: npm run ios
  ```

## What this is not for

- Do not add screens, features or dependencies here.
- Do not fix its bugs here. The known ones are enumerated in the partner's build plan §08
  and are deliberately **not** ported.
- Do not ship it, and do not let it grow a build in CI.

## Layout

```
App.tsx                   fonts, providers, navigator mount
src/theme/index.ts        design tokens — colors, type, radii, shadows
src/components/           primitives (button, card, sheet, chip), icons as SVG paths
src/navigation/           stack navigator + the action chooser sheet
src/features/<context>/   one directory per bounded context: screens + local fixtures
```

Feature directories, in roadmap order: `today`, `schedule`, `log-dose`, `inventory`,
`trends`, `journal`, `body`, `meal`, `recipe`, `chat`, `learn`, `profile`.

## Design tokens

`src/theme/index.ts` is a ported copy of `web/prototype/design-system/colors_and_type.css` —
the two surfaces share one palette and one type scale by construction. The Compose theme is
ported from this file, not re-invented from screenshots.

# Doctor dashboard — browser prototype

> **FROZEN — visual spec, do not extend.**

This directory is the **design specification** for the Cadence doctor dashboard: the overview
page, the roster, triage, the patient drawer and the three-pane messages view. `Doctor
Dashboard.html` pulls React and Babel from a CDN and compiles the `dd-*.jsx` files in the
browser — no build step, no backend, fixtures only.

It is **not** the product. The shipped dashboard is Vite + React 18 + TS and lives in `web/`,
one level up — see SKL-09.

## What this is for

- **Read it.** When a dashboard interaction is ambiguous, the prototype is the answer.
- **Port from it.** `design-system/colors_and_type.css` is carried over verbatim into the
  Vite app and a typed token object is generated from it (in the prototype the tokens and
  their inline usages drifted apart — that is fixed during the port, not here).
- **View it** — a plain `file://` open will not work, the Babel loader needs HTTP:
  ```sh
  cd web/prototype && python3 -m http.server 8000
  # http://localhost:8000/Doctor%20Dashboard.html
  ```

## What this is not for

- Do not add pages, components or fixtures here.
- Do not fix its bugs here — they are enumerated in the partner's build plan §08 and are
  deliberately not ported.
- Do not ship it, and do not wire it into CI.

## Layout

```
Doctor Dashboard.html         page shell, CDN script tags, global CSS and grid breakpoints
dd-app.jsx                    sidebar, top bar, patient drawer, page composition
dd-components.jsx             stat cards, roster rows, triage cards, schedule and activity items
dd-chat.jsx                   three-pane messages view
dd-data.jsx, dd-chat-data.jsx fixtures
design-system/
  colors_and_type.css         design tokens — the shared source with the mobile theme
  cadence-components.jsx      typography, Btn, Chip, Pill, Card, Section, ListRow, TabBar, Spark
  heroicons.js                icon set
  logo-mark.svg, logo-mono.svg, wordmark.svg
```

# Living WebGL Sky + Glass Panels — Design

**Date:** 2026-06-07
**Status:** Approved (pending spec review)

## Goal

Give the Tellulf kiosk display real graphical richness now that it runs on an M3
MacBook in Firefox (previously a Raspberry Pi). Replace the flat white background
with a **living, weather-reactive WebGL sky** and turn the existing panels into
**frosted-glass cards** floating over it — without changing the layout or turning
the calm dashboard into a noisy one.

## Concept

A single full-screen WebGL canvas sits behind the existing CSS grid and renders
the sky **procedurally in one fragment shader** (no geometry, no particle system,
no 3D library — raw WebGL, one fullscreen quad). The sky reacts to two axes:

- **Time-of-day phase** — `night` / `dawn` / `day` / `dusk`, derived from the
  sunrise/sunset times the server already computes for lat/lon 59.9508, 10.6847.
- **Current weather condition** — `clear` / `partly` / `cloudy` / `precip`,
  bucketed from the nearest hourly forecast's MET symbol.

Every panel (clock, current weather, calendar, footer boxes) becomes a frosted
card. Text and line colors adapt light↔dark so they stay readable from bright
noon to deep-indigo night.

## Architecture — server is the single source of truth

The server already owns weather data and sunrise/sunset math, so it computes the
entire sky state. The client contains **no weather logic** — it only renders.

### New: `src/lib/sky.ts` (pure, unit-tested)

- `getPhase(now: Date, sunrise: Date, sunset: Date): Phase`
  Returns `'night' | 'dawn' | 'day' | 'dusk'`. Dawn/dusk are windows around the
  sun events (e.g. ±45 min); outside them it's day (sun up) or night (sun down).
- `getCondition(symbol: string): Condition`
  Buckets a MET symbol string into `'clear' | 'partly' | 'cloudy' | 'precip'`.
  Matches on substrings (`rain`, `snow`, `sleet`, `thunder` → precip;
  `clearsky` → clear; `partlycloudy`/`fair` → partly; `cloudy`/`fog` → cloudy).
- `buildSkyState(phase, condition, sunArc): SkyState`
  Returns both render targets:
  - **Shader uniforms** (numbers): `phase` index, `condition` index, `sunArc`
    (0–1 position of sun/moon along its arc), and three `[r,g,b]` gradient colors.
  - **CSS variables** (strings): `--fg`, `--fg-muted`, `--fg-faint`, `--line`,
    `--glass-bg`, `--glass-border`, `--glass-shadow`.
  Implementation: 4 hand-tuned **phase base palettes**, then a **condition
  modifier** desaturates / greys / cools them (clear = base + slight warmth,
  cloudy = greyer + flatter, precip = greyer + cooler). This avoids hand-painting
  all 16 phase×condition combinations.

### `src/viewdata.ts`

- `buildSkyData(hourlyForecasts, now, sunrise, sunset)` — selects the nearest
  hourly forecast symbol, computes phase + condition + sun arc, calls
  `buildSkyState`, returns the data the `sky.eta` partial needs.

### `views/partials/sky.eta` (new)

Renders the hidden driver payload:

- a `<style>` block setting the CSS variables on `:root`, and
- `data-*` attributes carrying the numeric shader uniforms.

### `src/server.ts`

- Render and send a `sky` SSE event on initial connect and inside the existing
  15 s `dataInterval`, via `sendEventIfChanged` (only pushes when the palette
  actually changes — phase/condition transitions, not every tick).

## Delivery: one SSE event, two consumers

`views/layout.eta` gains two elements:

```html
<canvas class="sky" aria-hidden="true"></canvas>
<div id="sky-driver" sse-swap="sky" hx-swap="innerHTML" hidden>
  <%~ it.skyHtml %>   <!-- <style>:root{…}</style> + data-* uniforms -->
</div>
```

- The **CSS variables** drive text + glass colors immediately on swap (CSS
  handles them; no JS).
- The **shader** reads the `data-*` uniforms from `#sky-driver` on each
  `requestAnimationFrame` and **lerps** current→target, so weather/phase changes
  glide over ~2 s with no special transition handling.

## Client renderer — `src/sky.ts` (bundled into `client.js`)

- Creates the WebGL context on the `<canvas class="sky">`, compiles the shader
  program, draws a fullscreen quad each frame.
- Reads target uniforms from `#sky-driver` dataset; keeps a current-uniform
  state; lerps toward target (~2 s time constant) each frame.
- Resizes with the window / devicePixelRatio (kiosk is fixed 1920×1080 but
  handle resize for dev).
- Imported from `src/client.ts` so it ships in the existing `bun build` bundle.

### Fragment shader layers (procedural, all uniform-driven)

1. **Vertical sky gradient** — 3 color stops by phase.
2. **Sun/moon glow** — soft radial light positioned by `sunArc`; warm tint at
   dawn/dusk, pale at night (moon).
3. **Drifting clouds** — fbm noise scrolled by `uTime`; coverage/opacity by
   condition.
4. **Stars** (Phase B) — hash-based twinkle, only at night and when not overcast.
5. **Rain / snow** (Phase B) — procedural streaks (rain) or drifting flakes
   (snow) when condition is `precip`; rain vs snow chosen from the symbol/temp.

Uniforms: `uTime`, `uResolution`, `uPhase`, `uCondition`, `uSunArc`,
`uColor1`, `uColor2`, `uColor3`, `uPrecip` (0 none / 1 rain / 2 snow).

## Glass panels + readability migration

`public/styles.css`:

- A shared glass treatment applied to `clock`, `currentweather`, `calendar`,
  and `.footerBox`: `background: var(--glass-bg)`,
  `backdrop-filter: blur(20px) saturate(1.4)`,
  `border: 1px solid var(--glass-border)`, `border-radius: 18px`, soft shadow
  via `--glass-shadow`, padding.
- `nowcast` stays **card-free** (transparent chart over the sky) but its text and
  gridlines switch to the adaptive vars.
- Remap hardcoded colors (`#000` body text, `#555`, `#999`, `#c0c0c0`, the
  `opacity:0.1` min/max temps) to `--fg` / `--fg-muted` / `--fg-faint` / `--line`.
- Remove `body { background:#fff }` — the canvas provides the background.
- The data-driven blues/greens (power bars, rain) stay as-is; they read fine on
  both light and dark.

The `.sky` canvas is `position: fixed; inset:0; z-index:-1` so the panels'
`backdrop-filter` samples it correctly (verified pattern; confirm early in
Firefox).

## Scope (phased)

- **Phase A:** canvas + shader (gradient + sun/moon glow + clouds) + glass + text
  adaptation + `sky.ts` lib + SSE wiring. The headline effect.
- **Phase B:** stars at night + procedural rain/snow. Included in this project.

## Decisions made

- Nowcast stays glass-free (transparent over the sky looks better than a card).
- ~2 s cross-fades on weather/phase change (via shader lerp + CSS transition on
  text/glass vars).
- Full day→night palette with adapting text (not light-pastels-only).
- Raw WebGL, no 3D library or particle dependency.

## Not doing (YAGNI)

- No three.js / regl / OGL or any 3D/particle dependency.
- No layout or grid changes.
- No changes to the existing nowcast rain animation.

## Verification

- **Unit tests** for `sky.ts`: phase boundaries around sunrise/sunset; symbol→
  condition bucketing; `buildSkyState` returns all required uniforms + CSS vars
  for every phase×condition.
- **Dev override**: a query param or keyboard shortcut to force any
  phase/condition for tuning and screenshots.
- **Visual check** in Firefox via the run skill: confirm glass blur samples the
  canvas, text stays readable across all phases, transitions are smooth.

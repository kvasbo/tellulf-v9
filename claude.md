# Tellulf

## What this is?

A home status display showing weather, calendars, subway departures and power usage. Runs as a kiosk display at 1920x1080.

## Architecture

Go server (`main.go`), rendered HTML pushed to the browser over SSE.

- **net/http** server with SSE (`internal/web`). A single publisher goroutine renders each
  fragment on a schedule (power 1s, weather/calendar 15s, trains 30s, version 60s) and a
  `Hub` fans out only fragments that changed. New connections get a snapshot first.
- **templ** components (`internal/views/*.templ`, generated `*_templ.go` committed), one
  component per partial, typed by the structs in `internal/view`. All formatting happens
  in `internal/view`, so templates stay dumb. templ doesn't interpolate inside `<style>`,
  so the sky's theme variables are written with `templ.Raw` (constants only). `view.Fixed`/`view.Num` mimic
  JavaScript's `toFixed`/`String()` so numbers render exactly as the old TS version did.
- **HTMX** + SSE extension on the client for reactive DOM updates (vendored in `static/vendor/`)
- **Client JS** (`public/client.js`, `public/sky.js`, plain ES modules, no build step) for
  clock, living sky (WebGL), calendar overflow, version check.
- Templates, `static/` and `public/` are embedded in the binary with `go:embed`.
- Every data source is a struct with a `Run(ctx)` loop and state behind a mutex; getters
  return copies. Missing config for calendar/MQTT/Tibber disables that panel with a warning.
- All wall-clock logic uses `tz.Oslo` (tzdata embedded).

## Gotchas

- The kiosk runs **Firefox** (on an M3 MacBook). The client may use APIs Firefox has,
  e.g. `Temporal` (Firefox 139+). Playwright's bundled Chromium lacks `Temporal`, so
  client.js throws there and stops before registering its listeners; stub it
  (`window.Temporal = { Now: { plainDateISO: () => ({ weekOfYear: 1 }) } }`) when
  testing in headless Chromium.
- The htmx SSE extension only listens for event names that some element declares in
  `sse-swap` (or `hx-trigger="sse:..."`). An event no element asks for is silently
  dropped and never reaches `htmx:sseMessage`. That's why `#server-version` has
  `sse-swap="version" hx-swap="none"`.
- Auto-reload: the server's version is its start time. It's in the page and sent
  right after each SSE snapshot, plus every minute; `client.ts` reloads when it
  changes, so any restart reloads open screens within seconds.

## Packages

- `internal/weather` — MET Norway forecasts (hourly + subseasonal), Oslo and the cabin
- `internal/calendar` — Google Calendar (official Go client), kids schedule, fading rules
- `internal/entur` — metro departures
- `internal/smarthouse` — MQTT sensor readings (paho)
- `internal/tibber` — live feed (own graphql-transport-ws client on coder/websocket),
  daily price list, monthly consumption, Norgespris maths
- `internal/sky`, `internal/sun` — living-sky logic and NOAA sunrise/sunset
- `internal/view` — builds template data; `internal/views` — templ components;
  `internal/web` — HTTP, SSE hub, static files

## Running

- `TELLULF_DEV=1 go run .` — development; static files are read from disk
- `go generate ./...` — regenerate templ components after editing a `.templ` file
  (or `go tool templ generate --watch --cmd="go run ."` to regenerate and restart on save)
- `go test ./...` — tests
- `go build -o tellulf .` — production binary (self-contained)
- `docker build .` — runs generate, vet, test and build; distroless image

## APIs

- **Tibber**: Power usage and cost. Reference: https://developer.tibber.com/docs/reference
  - Max two open websockets per token (one per home here). Feeds reconnect with jittered
    exponential backoff, wait 1–60s after "1001 Going away", stop on a rejected token, and
    check `realTimeConsumptionEnabled` before reconnecting.
  - Prices: today+tomorrow fetched once and cached; current price is picked locally.
- **yr.no**: Weather forecasts (MET Norway API)
- **Google Calendar**: Events, birthdays, dinners, barneuker, kindergarten days
- **Entur**: Train departures (RUT Line 1, Slemdal station)
- **MQTT**: Sensor data (temperature, humidity, pressure)

## Norgespris

- Home cap: 5000 kWh/month, cabin cap: 1000 kWh/month; spot price above the cap
- Fixed price (incl. 25 % VAT): 0.50 kr/kWh from 2025-10-01, 0.5625 kr/kWh (45 øre + mva)
  from 2027-01-01. Each kWh is priced at the rate in effect when it was used.
  Schedule lives in `defaultNorgespris` in `internal/tibber/norgespris.go`.

## Environment Variables

CAL_ID_AUDUN, CAL_ID_BARNEUKER, CAL_ID_BURSDAG, CAL_ID_FELLES, CAL_ID_KINDERGARDEN,
CAL_ID_MIDDAG, EXPOSE_PORT, GOOGLE_KEY_B64, MQTT_HOST, MQTT_PASS, MQTT_USER,
TIBBER_ID_CABIN, TIBBER_ID_HOME, TIBBER_KEY, TELLULF_DEV (optional)

Loaded from the environment, plus `.env` in the working directory if present (godotenv;
existing environment variables win).

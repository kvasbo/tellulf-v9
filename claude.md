# Tellulf

## What this is?

A home status display showing weather, calendars, subway departures and power usage. Runs as a
kiosk display at 1920x1080. The layout ("Natt") is dark, still and fixed-size: its design
notes and mockup are in `docs/superpowers/specs/2026-10-26-natt-redesign-design.md`.

## Architecture

Go server (`main.go`), rendered HTML pushed to the browser over SSE.

- **net/http** server with SSE (`internal/web`). A single publisher goroutine renders each
  fragment on a schedule (`power-home`/`power-cabin` 1s, `weather`/`sun`/`calendar` 15s,
  `trains` 30s, version 60s) and a `Hub` fans out only fragments that changed. New
  connections get a snapshot first.
- **templ** components (`internal/views/*.templ`, generated `*_templ.go` committed), one
  component per partial, typed by the structs in `internal/view`. All formatting, rounding
  and wording happens in `internal/view` (the weather sentences and three-hour columns in
  `weather.go`, the calendar lines in `calendar.go`), so templates stay dumb.
- **Client JS**: plain ES modules, no build step. `public/live.js` opens an `EventSource`
  and puts each fragment into the element whose `data-sse` names it (plain `innerHTML`),
  then dispatches a `fragment` event on `document`. `client.js` (clock and date, calendar
  overflow, version check, corner times), `report.js` (logging to the server).
- **Biome** lints and formats `public/` (`biome.json`); CI runs `biome ci`.
- Templates, `static/` and `public/` are embedded in the binary with `go:embed`.
- Every data source is a struct with a `Run(ctx)` loop and state behind a mutex; getters
  return copies. Missing config for calendar/MQTT/Tibber disables that panel with a warning.
- All wall-clock logic uses `tz.Oslo` (tzdata embedded).

## Gotchas

- The kiosk runs the latest **Chrome** on macOS (M3 MacBook). The client may use APIs current Chrome
  has, e.g. `Temporal` (Chrome 144+). Playwright's bundled Chromium is older and lacks `Temporal`, so
  client.js throws there and stops before registering its listeners; stub it
  (`window.Temporal = { Now: { plainDateISO: () => ({ weekOfYear: 1 }) } }`) when
  testing in headless Chromium.
- `EventSource` only delivers named events that have a listener. `live.js` listens for
  every `data-sse` name plus the handlers passed to `connect` (just `version`); a new
  event that is neither is silently dropped.
- Nothing on the page may animate: no `transition`, no `@keyframes`. Missing data is
  shown as absence, never as `–`. Numbers are Norwegian (decimal comma, `−`, a narrow
  no-break space before units); the helpers are in `internal/view/format.go`.
- `public/report.js` POSTs lines to `/log`, which the server writes to its log as
  `msg=client`: page loads, visibility changes and JS errors. Read with `docker logs`.
- In dev mode Chrome caches `public/*.js` heuristically (the files carry a Last-Modified
  header), so a plain reload can keep old JS; fetch with `cache: 'reload'` or hard-reload.
  The embedded production files have no modtime, so the kiosk always revalidates.
- Auto-reload: the server's version is its start time. It's in the page and sent
  right after each SSE snapshot, plus every minute; `client.js` reloads when it
  differs from `data-version` on `<body>`, so any restart reloads open screens within seconds.

## Packages

- `internal/weather` — MET Norway forecasts (hourly + subseasonal), Oslo and the cabin
- `internal/calendar` — Google Calendar (official Go client), kids schedule, fading rules
- `internal/entur` — metro departures
- `internal/smarthouse` — MQTT sensor readings (paho)
- `internal/tibber` — live feed (own graphql-transport-ws client on coder/websocket),
  daily price list, monthly consumption, Norgespris maths
- `internal/sun` — NOAA sunrise/sunset for the SOL cell
- `internal/buildinfo` — build time, set by the Dockerfile via `-ldflags -X` (shown as "dev" otherwise)
- `internal/view` — builds template data; `internal/views` — templ components;
  `internal/web` — HTTP, SSE hub, static files

## Running

- `TELLULF_DEV=1 go run .` — development; static files are read from disk.
  `TELLULF_DEBUG=1` also logs routine fetches.
- `go generate ./...` — regenerate templ components after editing a `.templ` file
  (or `go tool templ generate --watch --cmd="go run ."` to regenerate and restart on save)
- `go test ./...` — tests
- Screenshots must be a true 1920x1080: the in-app browser pane scales down. Use
  headless Chrome: `"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
  --headless=new --use-angle=swiftshader --enable-unsafe-swiftshader --hide-scrollbars
  --window-size=1920,1080 --timeout=12000 --screenshot=out.png http://localhost:3000/`
  (`--timeout`, not `--virtual-time-budget`: the SSE stream never settles).
- `go build -o tellulf .` — production binary (self-contained)
- `docker build .` — runs generate, vet, test and build; distroless image

## APIs

- **Tibber**: Power usage and cost. Reference: https://developer.tibber.com/docs/reference
  - Max two open websockets per token (one per home here). Feeds reconnect with jittered
    exponential backoff, wait 1–60s after "1001 Going away", stop on a rejected token, and
    check `realTimeConsumptionEnabled` before reconnecting.
  - Prices: today+tomorrow fetched once and cached; current price is picked locally.
- **yr.no**: Weather forecasts (MET Norway API). Follows MET's terms: no request before
  `Expires`, and `If-Modified-Since` so an unchanged forecast is a 304. The stored forecast
  can lag, so `HourlyForecasts` drops hours that have passed.
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
TIBBER_ID_CABIN, TIBBER_ID_HOME, TIBBER_KEY, TELLULF_DEV and TELLULF_DEBUG (optional)

Loaded from the environment, plus `.env` in the working directory if present (godotenv;
existing environment variables win).

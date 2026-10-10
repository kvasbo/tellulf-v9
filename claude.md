# Tellulf

## What this is?

A home status display showing weather, calendars, subway departures and power usage. Runs as a kiosk display at 1920x1080.

## Architecture

Go server (`main.go`), rendered HTML pushed to the browser over SSE.

- **net/http** server with SSE (`internal/web`). A single publisher goroutine renders each
  fragment on a schedule (power 1s, weather/calendar 15s, trains 30s, version 60s) and a
  `Hub` fans out only fragments that changed. New connections get a snapshot first.
- **html/template** templates (`views/`), one `{{define}}` per partial. All formatting
  happens in `internal/view`, so templates stay dumb. `view.Fixed`/`view.Num` mimic
  JavaScript's `toFixed`/`String()` so numbers render exactly as the old TS version did.
- **HTMX** + SSE extension on the client for reactive DOM updates (vendored in `static/vendor/`)
- **Client TS** (`client/`) for clock, living sky (WebGL), calendar overflow, version check.
  Bundled to `public/client.js` by esbuild via `go generate` (esbuild runs as a Go tool,
  no Node/Bun needed).
- Templates, `static/` and `public/` are embedded in the binary with `go:embed`.
- Every data source is a struct with a `Run(ctx)` loop and state behind a mutex; getters
  return copies. Missing config for calendar/MQTT/Tibber disables that panel with a warning.
- All wall-clock logic uses `tz.Oslo` (tzdata embedded).

## Packages

- `internal/weather` — MET Norway forecasts (hourly + subseasonal), Oslo and the cabin
- `internal/calendar` — Google Calendar (official Go client), kids schedule, fading rules
- `internal/entur` — metro departures
- `internal/smarthouse` — MQTT sensor readings (paho)
- `internal/tibber` — live feed (own graphql-transport-ws client on coder/websocket),
  daily price list, monthly consumption, Norgespris maths
- `internal/sky`, `internal/sun` — living-sky logic and NOAA sunrise/sunset
- `internal/view` — builds template data; `internal/web` — HTTP, SSE hub, static files

## Running

- `go generate ./...` — bundle the client (needed once, and after editing `client/`)
- `TELLULF_DEV=1 go run .` — development; templates and static files are read from disk
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

## Norgespris (active from 2025-10-01)

- Home cap: 5000 kWh/month @ 0.50 kr/kWh, spot price above
- Cabin cap: 1000 kWh/month @ 0.50 kr/kWh, spot price above

## Environment Variables

CAL_ID_AUDUN, CAL_ID_BARNEUKER, CAL_ID_BURSDAG, CAL_ID_FELLES, CAL_ID_KINDERGARDEN,
CAL_ID_MIDDAG, EXPOSE_PORT, GOOGLE_KEY_B64, MQTT_HOST, MQTT_PASS, MQTT_USER,
TIBBER_ID_CABIN, TIBBER_ID_HOME, TIBBER_KEY, TELLULF_DEV (optional)

Loaded from the environment, plus `.env` in the working directory if present (godotenv;
existing environment variables win).

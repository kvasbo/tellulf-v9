# tellulf-v9

Personal project. Nothing to see here, move along.

A home status display (weather, calendars, metro departures, power usage) that runs as a
1920x1080 kiosk. A Go server renders the HTML and keeps it live over server-sent events;
HTMX swaps the fragments in on the client.

## Requirements

- Go 1.26 or newer. That's all: the browser bundle is built by esbuild, which runs as a
  Go tool, so no Node or Bun is needed.

## Quick start

```sh
go generate ./...          # bundle client/*.ts into public/client.js
TELLULF_DEV=1 go run .     # http://localhost:3000
```

Without any environment variables you get weather and metro departures. Calendar,
sensors and power each need their own variables (see below); when they are missing,
that panel stays empty and the log says why.

Run `go generate ./...` again after editing anything in `client/`.

## Development

- `TELLULF_DEV=1` reads templates (`views/`) and static files (`static/`, `public/`)
  from disk on every request, so edits show up on reload. Run it from the repo root.
  Go code changes still need a restart.
- `go test ./...` runs the tests. Add `-race` to check for data races.
- `go vet ./...` and `gofmt -l .` for checks and formatting.

To try power data without your own account, use Tibber's public demo token and home
(the values in `stack.env`):

```sh
TIBBER_KEY=3A77EECF61BD445F47241A5A36202185C35AF3AF58609E19B53F3A8872AD7BE1-1 \
TIBBER_ID_HOME=96a14971-525a-4420-aae9-e5aedaa129ff \
TIBBER_ID_CABIN=96a14971-525a-4420-aae9-e5aedaa129ff \
TELLULF_DEV=1 go run .
```

## Building and deploying

```sh
go generate ./...
go build -o tellulf .
./tellulf
```

The binary is self-contained: templates, static files, the client bundle and the time
zone database are embedded, so it can run from any directory.

`docker build .` runs generate, vet, test and build, and produces a distroless image.
GitHub Actions builds and pushes an image to `ghcr.io/kvasbo/tellulf-v9` on every push,
tagged with the branch name and commit SHA. `docker-compose.yml` runs the `main` image.

## Environment variables

Three ways to set them:

- **`.env` file** (local development): copy `.env.example` to `.env` in the repo root and
  fill it in. The app reads it at startup, the way Bun did. It's gitignored.
- **Shell**: `TIBBER_KEY=... go run .` or `export`. Shell values win over `.env`.
- **Docker**: `docker-compose.yml` passes the variables into the container. Compose itself
  fills `${...}` from a `.env` next to the compose file (or `stack.env` in Portainer).
  The `.env` in the repo is never copied into the image.

| Variable | Used for |
|---|---|
| `EXPOSE_PORT` | Port to listen on (default 3000) |
| `TELLULF_DEV` | Set to anything to serve templates and static files from disk |
| `MQTT_HOST`, `MQTT_USER`, `MQTT_PASS` | Outdoor sensors, e.g. `mqtt://broker:1883` (port defaults to 1883) |
| `TIBBER_KEY`, `TIBBER_ID_HOME`, `TIBBER_ID_CABIN` | Power usage and price; all three are required |
| `GOOGLE_KEY_B64` | Base64-encoded Google service account key (JSON) |
| `CAL_ID_FELLES`, `CAL_ID_AUDUN` | Event calendars |
| `CAL_ID_BURSDAG` | Birthdays |
| `CAL_ID_MIDDAG` | Dinners |
| `CAL_ID_BARNEUKER` | Kids' weeks |
| `CAL_ID_KINDERGARDEN` | Kindergarten days (the "bhg" count) |

The calendar needs `GOOGLE_KEY_B64` plus at least one `CAL_ID_*`.

## Project layout

```
main.go               wiring: reads config, starts data sources and the web server
client/               browser TypeScript (clock, WebGL sky, calendar overflow)
views/                html/template layout and partials
static/, public/      icons, CSS, vendored htmx; public/client.js is generated
internal/
  weather/            MET Norway forecasts
  calendar/           Google Calendar, kids' schedule, fading rules
  entur/              metro departures
  smarthouse/         MQTT sensor readings
  tibber/             live power feed, prices, monthly totals, Norgespris
  sky/, sun/          living-sky logic, sunrise/sunset
  view/               turns data into what the templates print
  web/                HTTP handlers, SSE hub, static files
```

## Updating dependencies

- Go modules: `go get -u ./... && go mod tidy`
- esbuild: `go get -tool github.com/evanw/esbuild/cmd/esbuild@latest`
- htmx, htmx-ext-sse and idiomorph are vendored in `static/vendor/` (currently htmx
  2.0.11, htmx-ext-sse 2.2.4, idiomorph 0.8.0). To update one, download the new file
  from npm or a CDN and replace it.

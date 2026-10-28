# tellulf-v9

Personal project. Nothing to see here, move along.

A home status display (weather, calendars, metro departures, power usage) that runs as a
1920x1080 kiosk: dark, still, one font. A Go server renders the HTML and keeps it live over
server-sent events; a few lines of JavaScript (`public/live.js`) swap the fragments in on
the client.

## Modes

Besides the overview (`/`), the screen has a page for power (`/strom`), the calendar
(`/kalender`) and the weather (`/vaer`), each in full detail, and one for the week ahead
(`/uke`). Publish the mode on MQTT and every open screen goes there, with a short page
transition:

```sh
mosquitto_pub -h broker -t tellulf/mode -m strom   # or kalender, vaer, uke, oversikt
```

The payload is a mode's name or its path (`strøm` and `vær` work too). After
`TELLULF_MODE_TIMEOUT` (two minutes by default) from the last message the screen goes back to
the overview. Don't publish with `-r`: retained mode messages are ignored, since the broker
would replay them on every reconnect.

Every page can be opened directly: it stays until the mode changes, or goes back to the
overview after the same timeout. For development, `TELLULF_MODE_TIMEOUT=0` keeps a page up
for good.

## Requirements

- Go 1.27.2 or newer. That's all: the browser code is plain JavaScript served as is, so
  there is no Node, Bun or bundling step.

## Quick start

```sh
TELLULF_DEV=1 go run .     # http://localhost:3000
```

Without any environment variables you get weather and metro departures. Calendar,
sensors, power and Claude each need their own variables (see below); when they are
missing, that panel stays empty (without Claude: as it is without it) and the log says why.

## Claude

With `ANTHROPIC_API_KEY` set, Claude writes three things from the calendar and the
forecast: a sentence under the dinner in I DAG, short titles for long events (and a note
from the description on the calendar page), and a few lines about the week on `/uke`.
They are written in the background, a few times an hour at most; a text with a number
that isn't in the data it was written from is thrown away. With `TELLULF_STATE_DIR` they
are saved there (`claude.json`), so a restart shows them at once without asking again. The model is Haiku
(`claude-haiku-5-5`) unless `TELLULF_CLAUDE_MODEL` says otherwise. Calendar titles,
descriptions and places are sent to Anthropic's API. Design notes:
`docs/superpowers/specs/2026-10-28-claude-design.md`.

To try the prompts on a made-up week against the real API (a few øre):

```sh
ANTHROPIC_API_KEY=... go test -tags live -run Live -v ./internal/claude
```

## Development

- `TELLULF_DEV=1` reads static files (`static/`, `public/`) from disk on every request,
  so CSS and JavaScript edits show up on reload. Run it from the repo root.
- Templates are [templ](https://templ.guide) components in `internal/views/*.templ`. After
  editing one, run `go generate ./...` (which runs `go tool templ generate`) and restart.
  The generated `*_templ.go` files are committed, so a plain `go build` always works.
- Or let templ watch: `TELLULF_DEV=1 go tool templ generate --watch --cmd="go run ."`
  regenerates on save, applies text changes to the running server without a restart, and
  restarts it when the Go code in a template changes. Reload the browser to see changes.
- Go code changes outside templates need a restart.
- `go test ./...` runs the tests. Add `-race` to check for data races.
- The browser code is linted and formatted with [Biome](https://biomejs.dev) (config in
  `biome.json`; CI runs `biome ci`). Locally: `npx @biomejs/biome@2.5.15 check --write`.
- `TELLULF_DEBUG=1` also logs the routine fetches (forecasts, departures, calendars).
- `go vet ./...` and `gofmt -l .` for checks and formatting.
- The layout is fixed at 1920x1080, so check it at that size. With the dev server running:

  ```sh
  "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --headless=new \
    --use-angle=swiftshader --enable-unsafe-swiftshader --hide-scrollbars \
    --window-size=1920,1080 --timeout=12000 --screenshot=natt.png http://localhost:3000/
  ```

  The design notes and the mockup it follows are in `docs/superpowers/specs/`.

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
go build -o tellulf .
./tellulf
```

The binary is self-contained: compiled templates, static files, the client JavaScript and the time
zone database are embedded, so it can run from any directory.

`docker build .` regenerates the templates, then runs vet, test and build, and produces a distroless image.
GitHub Actions builds and pushes an image to `ghcr.io/kvasbo/tellulf-v9` on every push,
tagged with the branch name and commit SHA. `docker-compose.yml` runs the `main` image.

## Environment variables

Three ways to set them:

- **`.env` file** (local development): copy `.env.example` to `.env` in the repo root and
  fill it in. The app reads it at startup. It's gitignored.
- **Shell**: `TIBBER_KEY=... go run .` or `export`. Shell values win over `.env`.
- **Docker**: `docker-compose.yml` passes the variables into the container. Compose itself
  fills `${...}` from a `.env` next to the compose file (or `stack.env` in Portainer).
  The `.env` in the repo is never copied into the image.

| Variable | Used for |
|---|---|
| `EXPOSE_PORT` | Port to listen on (default 3000) |
| `TELLULF_DEV` | Set to anything to serve static files from disk |
| `TELLULF_DEBUG` | Set to anything to log routine fetches too |
| `TELLULF_MODE_TIMEOUT` | How long a page other than the overview stays up: `90s`, `5m`, or seconds; `0` for good (default `2m`) |
| `MQTT_HOST`, `MQTT_USER`, `MQTT_PASS` | Outdoor sensors and the mode, e.g. `mqtt://broker:1883` (port defaults to 1883) |
| `TIBBER_KEY`, `TIBBER_ID_HOME`, `TIBBER_ID_CABIN` | Power usage and price; all three are required |
| `ANTHROPIC_API_KEY` | Claude's texts (optional) |
| `TELLULF_CLAUDE_MODEL` | The model for them (default `claude-haiku-5-5`) |
| `TELLULF_STATE_DIR` | A writable directory where Claude's texts are kept between restarts (optional; memory only without it) |
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
static/               favicon
public/               CSS and the browser JavaScript (clock, calendar overflow, reload)
internal/
  weather/            MET Norway forecasts
  calendar/           Google Calendar, kids' schedule, fading rules
  claude/             Claude's texts: today's sentence, short titles, the week
  entur/              metro departures
  smarthouse/         MQTT sensor readings and the mode
  mode/               the pages (modes) and their paths
  tibber/             live power feed, prices, monthly totals, Norgespris
  sun/                sunrise/sunset
  view/               turns data into what the templates print
  views/              templ components (*.templ) and their generated Go code
  web/                HTTP handlers, SSE hub, static files
```

## Updating dependencies

Dependabot (`.github/dependabot.yml`) opens weekly pull requests for Go modules, the
Docker build image and GitHub Actions. To check or update by hand:

- See what's outdated: `go list -m -u all` (newer versions in brackets)
- Security: `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`
- Go modules: `go get -u ./... && go mod tidy`, then `go test ./...`
- templ: `go get -u tool`, then `go generate ./...` to refresh the generated files
- Biome is pinned in `biome.json` (`$schema`) and `.github/workflows/check.yml`; bump both.

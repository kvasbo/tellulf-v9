# Tellulf

## What this is?

A home status display showing weather, calendars, subway departures and power usage. Runs as a kiosk display at 1920x1080.

## Architecture

- **Express** server (`src/server.ts`) with SSE for live updates
- **Eta** templates (`views/`) for server-side HTML rendering
- **HTMX** + SSE extension on the client for reactive DOM updates
- **Client JS** (`public/client.js`) for clock, calendar overflow, version check
- Run directly with `tsx` — no build step

## Running

- `pnpm dev` — development with file watching
- `pnpm start` — production

## APIs

- **Tibber**: Power usage and cost via `tibber-api` package (WebSocket real-time feed)
- **yr.no**: Weather forecasts (MET Norway API)
- **Google Calendar**: Events, birthdays, dinners (only use of `googleapis`)
- **Entur**: Train departures (RUT Line 1, Slemdal station)
- **MQTT**: Sensor data (temperature, humidity, pressure)

## Norgespris (active from 2025-10-01)

- Home cap: 5000 kWh/month @ 0.50 kr/kWh, spot price above
- Cabin cap: 1000 kWh/month @ 0.50 kr/kWh, spot price above

## Environment Variables

CAL_ID_BURSDAG, CAL_ID_FELLES, CAL_ID_MIDDAG, EXPOSE_PORT,
GOOGLE_KEY_B64, MQTT_HOST, MQTT_PASS, MQTT_USER,
TIBBER_ID_CABIN, TIBBER_ID_HOME, TIBBER_KEY

# currentDate
Today's date is 2026-03-07.

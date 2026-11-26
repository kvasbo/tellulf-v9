# Strøm uten Tibber hjemme — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Home power from Elvia, spot prices from hvakosterstrommen.no for both places, the cabin still on Tibber, with the combining logic in a new `internal/power`.

**Architecture:** Three stateless clients (`spotpris`, `elvia`, a slimmed `tibber`) and one stateful `power.Power` that stores raw data (price days, the month's hours per place, live measurements) and derives every number when read (`Data(p, now)`, `Details(p, now)`). `view` and `web` switch from `tibber.` to `power.`.

**Tech Stack:** Go, net/http, templ, existing `schedule`/`tz` helpers.

**Spec:** `docs/superpowers/specs/2026-11-26-strom-uten-tibber-design.md`

## Global Constraints

- Price area NO1 for both places; price = `NOK_per_kWh × 1.25` (no VAT only in NO4).
- Elvia: `GET https://elvia.azure-api.net/customer/metervalues/api/v1/metervalues?startTime=…&endTime=…&includeProductionSteps=false`, `Authorization: Bearer <ELVIA_TOKEN>`, times RFC 3339 with offset, URL-encoded.
- Prices: `GET https://www.hvakosterstrommen.no/api/v1/prices/YYYY/MM-DD_NO1.json`; 404 = not published.
- A price day that has been fetched is never fetched again. Tomorrow from 13:00 Oslo, at most every 15 min.
- Elvia polled every 15 min; cabin's Tibber hours once an hour (`monthlyPace`); failures back off with `schedule.Backoff`.
- Live counts as current when ≤ 10 min old and from today.
- Env: `ELVIA_TOKEN` (home), `TIBBER_KEY` + `TIBBER_ID_CABIN` (cabin); `TIBBER_ID_HOME` removed. Update `docker-compose.yml`, README, CLAUDE.md.
- No animation, absence instead of `–`, Norwegian number helpers from `internal/view/format.go`.
- Match hours by Unix time (sources carry different zones).

## Review Focus

1. Month change: hours/prices from last month must not count in the new month (derive filters by start of month; `MonthKnown` false until the month's first fetch). → test in Task 5.
2. No live data and no hours yet today (just after midnight): "I dag" shows no value, the month row still shows. → test in Task 6.
3. Hour above the cap with no price yet: counts 0 kr, nothing panics. → test in Task 5.
4. DST day (23/25 hours): Elvia/price hours matched by Unix time, quarter-hour averaging groups by hour. → test in Task 2.
5. Only one place configured: the other place is blank, the configured one works, prices still run. → covered by `main.go` wiring in Task 6, checked by running the app.

---

### Task 1: `schedule.Backoff`

**Files:** Create `internal/schedule/backoff.go`, `internal/schedule/backoff_test.go`; modify `internal/tibber/feed.go`, `internal/tibber/tibber.go`, `internal/tibber/tibber_test.go`, `internal/tibber/monthly_test.go`.

**Produces:** `func schedule.Backoff(attempt int) time.Duration` (1-based; half fixed, half random; 10 s base, 30 min ceiling), `schedule.MaxBackoff`.

- [ ] Move `backoff`, `baseBackoff`, `maxBackoff` from `tibber/feed.go` to `schedule` (exported); move `TestBackoffGrowsAndStaysBounded`. Tibber callers use `schedule.Backoff`/`schedule.MaxBackoff`.
- [ ] `go test ./...` passes. Commit `refactor(schedule): share the jittered backoff`.

### Task 2: `internal/spotpris`

**Files:** Create `internal/spotpris/spotpris.go`, `spotpris_test.go`.

**Produces:** `type Price struct{ Start time.Time; Total float64 }`; `var ErrNotPublished`; `func New(area string) *Client`; `func (c *Client) Day(ctx context.Context, day time.Time) ([]Price, error)` — `day` is any time on the Oslo date; result sorted, one per hour.

- [ ] Tests with `httptest` (client's `base` URL field set in-package):
  - `TestDayAddsVAT`: one entry `NOK_per_kWh: 2.0` → `Total == 2.5`, `Start` equal to `time_start`; request path `/2026/11-26_NO1.json`.
  - `TestDayAveragesQuarterHours`: four entries 1,2,3,4 in 00:00–01:00 → one Price `Total == 2.5*1.25`.
  - `TestDayNotPublished`: 404 → `errors.Is(err, ErrNotPublished)`.
  - `TestDayOnDSTDay`: 25 hourly entries on 2026-10-25 (02:00 twice, offsets +02/+01) → 25 Prices, distinct `Start.Unix()`.
- [ ] Implement: group by `time_start` truncated to the hour (Unix), average, × VAT (1.0 for `NO4`). Set a `User-Agent`. 30 s timeout. Non-200/404 → error with status.
- [ ] Tests pass. Commit `feat(spotpris): hourly spot prices from hvakosterstrommen.no`.

### Task 3: `internal/elvia`

**Files:** Create `internal/elvia/elvia.go`, `elvia_test.go`.

**Produces:** `type Hour struct{ Start time.Time; KWh float64 }`; `func New(token string) *Client`; `func (c *Client) Hours(ctx context.Context, from, to time.Time) ([]Hour, error)`.

- [ ] Tests with `httptest`:
  - `TestHoursParsesTimeSeries`: response shaped as in the spec with two hours → two Hours with values; server asserts `Authorization: Bearer tok`, `startTime`/`endTime` parse back to `from`/`to`, `includeProductionSteps=false`.
  - `TestHoursSkipsProductionAndSumsMeters`: two meteringpoints with the same hour (3 and 4) plus a `production: true` entry → one Hour of 7.
  - `TestHoursRejected`: 401 → error.
- [ ] Implement against `/customer/metervalues/api/v1/metervalues`; sort by start.
- [ ] Tests pass. Commit `feat(elvia): hourly consumption from Elvia's meter values API`.

### Task 4: Tibber as a cabin client (alongside the old code)

**Files:** Create `internal/tibber/client.go`; modify `feed.go`, `graphql.go`, tests.

**Produces:** `type Measurement` (exported `liveMeasurement`, same fields); `var ErrUnauthorized`; `type Hour struct{ Start time.Time; KWh float64 }`; `func NewClient(token, homeID string) *Client`; `func (c *Client) RunFeed(ctx context.Context, onMessage func(Measurement))`; `func (c *Client) Hours(ctx context.Context, last int) ([]Hour, error)` (nodes with null consumption skipped).

- [ ] `feed` takes `onMessage func(Measurement)` and a log label instead of `Place`; the old `Tibber` adapts with a closure until Task 6 deletes it. Feed tests keep passing.
- [ ] `TestClientHours`: fake GraphQL returns three nodes, one with `consumption: null` → two Hours.
- [ ] `go test ./...` passes. Commit `refactor(tibber): a client for one home's feed and hours`.

### Task 5: `internal/power`

**Files:** Create `internal/power/power.go` (types, `New`, `Run`, `Data`, `Details`, `Measure`, derive), `norgespris.go` (moved), `history.go` (moved `liveHours`, `mergeHours`, `dayTotals`), `prices.go` (price cache, loop, `priceAt`, `wantedDays`), `hours.go` (Elvia and Tibber loops, `monthlyPace` moved), tests.

**Consumes:** Tasks 1–4.

**Produces:**
- `type Place string`; `Home`, `Cabin`.
- `type PowerData` = old fields + `TodayUntil time.Time` (end of last known hour when not live; zero when live) + `MonthKnown bool`; `func (d PowerData) IsLive(now time.Time) bool`.
- `type Usage struct{ Start time.Time; Consumption float64 }`, `type PricePoint struct{ Start time.Time; Total float64 }`, `type Details` as before (`PowerData`, `Hours`, `Days`, `Prices`, `Norgespris`).
- `type Config struct{ Elvia *elvia.Client; Tibber *tibber.Client; Prices *spotpris.Client }`; `func New(Config) *Power`; `func (p *Power) Run(ctx)`; `func (p *Power) Data(pl Place, now time.Time) PowerData`; `func (p *Power) Details(pl Place, now time.Time) Details`; `func (p *Power) Measure(pl Place, m tibber.Measurement)`.
- `func (n Norgespris) AccumulatedCost(p Place, hours []Usage, price func(time.Time) float64, used float64) float64` — `used` = kWh already used this month before `hours`.

Derive (in `Data`), from raw state `prices map[int64][]PricePoint` (day start Unix → hours), `hours map[Place][]Usage`, `hoursAt map[Place]time.Time`, `live map[Place]PowerData` (feed fields only), `marks map[Place]*liveHours`:

```
month hours = hours[pl] with Start ≥ start of month(now); split into before (< start of today) and today
live := d.IsLive(now)
today kWh   = live ? feed AccumulatedConsumption : Σ today hours; TodayUntil = last today hour + 1h (when !live)
monthly     = Σ before + today kWh
cost before = AccumulatedCost(before, price, 0)
cost today  = live ? split today kWh at the cap (Norgespris / CurrentPrice) as in old onMeasurement
                   : AccumulatedCost(today hours, price, Σ before)
CurrentPrice = last PricePoint with Start ≤ now (any cached day); EffectivePrice as before
MonthKnown  = hoursAt[pl] ≥ start of month(now)
```

Hours without a price count 0 kr above the cap.

- [ ] Move tests from `tibber`: Norgespris (2027 change, cap crossing — new signature), liveHours/mergeHours/dayTotals, cabin negative power, monthlyPace (with `tibber.ErrUnauthorized`).
- [ ] New tests:
  - `TestDataFromHoursWithoutLive`: home, now 26 Nov 13:40, cap 100, Norgespris 0.5; hours 25 Nov 0–23 at 4 kWh (96), 26 Nov 0–12 at 1 kWh, every price 2 → `AccumulatedConsumption 13`, `TodayUntil` 26 Nov 13:00, `MonthlyConsumption 109`, `AccumulatedCost == 4*0.5 + 9*2`, `MonthlyCost == 96*0.5 + 4*0.5 + 9*2`, `CurrentPrice 2`, `EffectivePrice 2`, `MonthKnown`.
  - `TestDataUsesLiveWhenCurrent`: same plus a measurement 2 min old with `AccumulatedConsumption 15` → today 15, `TodayUntil` zero, cost split at the cap with `CurrentPrice`.
  - `TestDataIgnoresLastMonth`: now 1 Dec 00:30, hours only from November, `hoursAt` 30 Nov → monthly 0, `MonthKnown false`.
  - `TestDataHourWithoutPrice`: over the cap with no prices → cost counts Norgespris part only, no panic.
  - `TestCurrentPriceKeepsLastKnown`: prices end at 23:00 yesterday → CurrentPrice is that last price.
  - `TestWantedDays`: 26 Nov 12:59 → 1–26 Nov; 13:00 → 1–27 Nov; 1 Dec 00:10 → 1 Dec only.
  - `TestDetailsMergesHistoryAndLive`: cabin, Tibber hours 0–9, live marks 9–11 → Hours 0–11, Days from hours before today.
- [ ] Implement. Loops: prices every minute (fetch missing wanted days in order, stop at first error, `ErrNotPublished` for tomorrow → recheck in 15 min, drop days before start of month); Elvia every minute checking a due time (15 min after success, `schedule.Backoff` after failure), fetching start of month → next hour; Tibber hours with `monthlyPace`, `last = day*24 + hour + 3`; Tibber feed → `Measure(Cabin, m)`. Missing client → no loop.
- [ ] `go test ./internal/power/` passes. Commit `feat(power): combine Elvia, Tibber and spot prices`.

### Task 6: Switch the app over and delete the old Tibber aggregator

**Files:** Modify `internal/view/view.go`, `strom.go`, their tests, `internal/web/server.go`, `main.go`, `docker-compose.yml`, `README.md`, `CLAUDE.md`; delete aggregator code in `internal/tibber` (`tibber.go` state/loops, `history.go`, `norgespris.go`, price query) and their tests.

- [ ] View: `tibber.` → `power.`; `HasData = !d.Timestamp.IsZero() || d.MonthKnown` in `BuildPower` and `BuildStrom`; `powerIsCurrent` → `d.IsLive(now)`.
- [ ] `BuildStrom` without live: today row `Value = KWh(AccumulatedConsumption)`, `Details = [Kr(AccumulatedCost), "til kl. 13"]` (hour of `TodayUntil`, two digits, Oslo) when `TodayUntil` is set; empty value and no details otherwise. Chart unchanged.
- [ ] Tests: rewrite `TestBuildStromHidesAnOldReading` (stale → power lines gone, today shows `TodayUntil` value with "til kl. 11"); add `TestBuildStromNoHoursYetToday` (not live, `MonthKnown`, `TodayUntil` zero → today empty, month row has value, `HasData`); `TestBuildPowerFromHoursOnly` (`MonthKnown`, no Timestamp → `HasData`, `Power == ""`, Usage set).
- [ ] Web: `Sources.Power *power.Power`; `s.power`/`s.strom` call `Data(p, now)`/`Details(p, now)`.
- [ ] main: `elvia.New(ELVIA_TOKEN)` if set; `tibber.NewClient(TIBBER_KEY, TIBBER_ID_CABIN)` if both set; if neither, warn "power disabled"; warn for a missing place; `spotpris.New("NO1")`.
- [ ] Env/docs: compose adds `ELVIA_TOKEN`, drops `TIBBER_ID_HOME`; README env table and package list; CLAUDE.md APIs/Packages/env list (Tibber section now cabin only; Elvia and hvakosterstrommen notes; `internal/power`).
- [ ] `go generate ./... && go vet ./... && go test ./...` pass. Commit `feat: home power from Elvia, prices from hvakosterstrommen.no`.

### Task 7: Verify

- [ ] Run with `TELLULF_DEV=1` (no MQTT: avoid the real broker), screenshot `/` and `/strom` at 1920×1080 with headless Chrome; home shows month, today "til kl. HH", prices; logs show prices fetched for the month and Elvia hours.
- [ ] Whole-branch review by a fresh agent; fix findings.

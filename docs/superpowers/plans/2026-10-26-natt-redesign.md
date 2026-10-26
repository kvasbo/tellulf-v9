# Natt Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current animated, glass-card display with the calm, dark "Natt" layout from the mockup, on the same Go/templ/SSE server, deleting everything that only served the old look.

**Architecture:** The server keeps rendering templ fragments and pushing them over SSE; only the fragments change (`weather`, `sun`, `calendar`, `power-home`, `power-cabin`, `trains`). All wording, rounding and rules live in `internal/view` with table-driven tests; templates stay dumb. The WebGL sky, animated icons, idiomorph and the hourly chart are removed. One stylesheet, one font (Inter Tight), no motion.

**Tech Stack:** Go 1.27, templ (`go generate ./...` regenerates `*_templ.go`), plain ES modules in `public/`, Biome 2.5.15, headless Chrome for 1080p screenshots.

**Spec:** `docs/superpowers/specs/2026-10-26-natt-redesign-design.md` (mockup: `docs/superpowers/specs/2026-10-26-natt-mockup.webp`, 1280x720, scale 1.5x).

## Global Constraints

- Kiosk is exactly 1920x1080, always dark, no light scheme, no motion of any kind (no `transition`, no `@keyframes`).
- Norwegian number formatting everywhere: decimal comma; `−` (U+2212) for negatives; `–` (U+2013) in ranges; `·` (U+00B7) as separator; a narrow no-break space (U+202F) between number and unit (`0,0 kW`, `0,50 kr`, `27 %`); degrees as whole numbers with `°` attached (`11°`, `−3°`).
- Missing data renders as absence: no `–`, no "ingen". A label with nothing under it is fine.
- Only colours: `--bg #121212`, `--fg #ede7dc`, `--muted #9a948b`, `--faint #5c5852`, `--accent #e3a53a` (I DAG, next departure, kids dot), `--rain-1 #3d4f73`, `--rain-2 #4c6fae`, `--rain-3 #6a9cf0`. Builder may tune within the contrast rules (fg ≥ 7:1, muted ≥ 4.5:1 on bg).
- Font: Inter Tight from Google Fonts, weights 200/300/400, `font-variant-numeric: tabular-nums` on everything with digits. Labels uppercase with ~0.08em letter-spacing.
- Every SSE fragment needs exactly one contiguous element with `data-sse="<name>"`, or `live.js` drops it silently.
- Follow existing style: Go code in English with short comments; templ components one per partial; JS formatted by Biome (tabs, single quotes).
- Commit after every task on branch `natt`. Commit messages end with `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`.

## Review Focus

1. Forecast shorter than 21 hours (MET down, stale cache): columns must show what exists and never slice out of range. Test: Task 2 `TestColumnsWithShortForecast`.
2. Day with no daily forecast (map miss): the day line shows no temperature and no stray `·`. Test: Task 3 `TestDayWithoutForecast`.
3. Entur empty or all departures within two minutes: no rows, no direction, only the label. Test: Task 4 `TestBuildTrainsNoneReachable`.
4. Tibber feed not yet delivered (`Timestamp` zero) or month above the cap: no numbers at all in the first case, `108 %` unclamped in the second. Tests: Task 4 `TestBuildPowerNoData`, `TestBuildPowerAboveCap`.
5. A timed event that started yesterday and ends tomorrow shows today with an empty time cell and its title, not a crash or `–`. Test: Task 3 `TestTodayMultiDayEventHasNoTime`.

---

### Task 1: Branch and Norwegian number formatting

**Files:**
- Modify: `internal/view/format.go`
- Test: `internal/view/format_test.go`

**Interfaces:**
- Produces:
  - `func nb(s string) string` — turns `"-0.4"` into `"−0,4"` (decimal comma, U+2212 minus).
  - `func Deg(v float64) string` — `"11°"`, `"−3°"`, `"0°"` (never `"−0°"`); rounds half away from zero via `tz.Round`.
  - `func DegRange(lo, hi float64) string` — `"5–12°"`, `"−3–2°"`.
  - `func KW(watts float64) string` — `"0,0 kW"`, `"−0,4 kW"` (one decimal, narrow no-break space).
  - `func Kr(v float64) string` — `"0,50 kr"` (two decimals).
  - `func Pct(v float64) string` — `"27 %"` (whole number).
  - `func Capitalize(s string) string` — first rune upper-cased: `"tirsdag"` → `"Tirsdag"`.

- [ ] **Step 1: Create the branch**

```bash
git checkout -b natt
```

- [ ] **Step 2: Write the failing tests** in `internal/view/format_test.go`

```go
func TestDeg(t *testing.T) {
	cases := map[float64]string{11.4: "11°", 11.5: "12°", -3.2: "−3°", -0.4: "0°", 0: "0°"}
	for in, want := range cases {
		if got := Deg(in); got != want {
			t.Errorf("Deg(%v) = %q, want %q", in, got, want)
		}
	}
	if got := DegRange(4.6, 12.2); got != "5–12°" {
		t.Errorf("DegRange = %q", got)
	}
	if got := DegRange(-3.4, 2); got != "−3–2°" {
		t.Errorf("DegRange = %q", got)
	}
}

func TestUnits(t *testing.T) {
	if got := KW(0); got != "0,0 kW" { t.Errorf("KW(0) = %q", got) }
	if got := KW(-400); got != "−0,4 kW" { t.Errorf("KW(-400) = %q", got) }
	if got := KW(-40); got != "0,0 kW" { t.Errorf("KW(-40) = %q", got) }
	if got := KW(12345); got != "12,3 kW" { t.Errorf("KW(12345) = %q", got) }
	if got := Kr(0.5); got != "0,50 kr" { t.Errorf("Kr = %q", got) }
	if got := Pct(27.4); got != "27 %" { t.Errorf("Pct = %q", got) }
	if got := Capitalize("tirsdag"); got != "Tirsdag" { t.Errorf("Capitalize = %q", got) }
}
```

- [ ] **Step 3: Run the tests to see them fail**

Run: `go test ./internal/view/ -run 'TestDeg|TestUnits' -v`
Expected: FAIL, undefined: Deg, DegRange, KW, Kr, Pct, Capitalize.

- [ ] **Step 4: Implement the helpers in `internal/view/format.go`**

`nb` replaces `.` with `,` and a leading `-` with `−`. `Deg` uses `Num(tz.Round(v))` so `-0.4` rounds to `0`; `DegRange` is the lo part without `°`, `–`, then `Deg(hi)`. `KW` uses `Fixed(watts/1000, 1)` (which already never yields `-0.0`). Unit strings use `" "`.

- [ ] **Step 5: Run the tests to see them pass**

Run: `go test ./internal/view/ -run 'TestDeg|TestUnits' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/view/format.go internal/view/format_test.go
git commit -m "Add Norwegian number formatting helpers for the Natt layout"
```

---

### Task 2: Weather view and templates (temperature now, two sentences, six columns, sun)

**Files:**
- Create: `internal/view/weather.go`, `internal/view/weather_test.go`
- Create: `internal/views/weather.templ`, `internal/views/sun.templ`
- Modify: `internal/view/view.go` (delete `CurrentWeather`, `BuildCurrentWeather`, `orDash`; `HourlyForecast`, `ForecastHour`, `weatherIcon`, `rainHeight`, `minMaxTemps`, `deref`, `BuildHourlyForecast`; `windDisplayThreshold`; `Sky`, `cloudCoverage`, `nightFactor`, `rootCSS`, `rgb`, `BuildSky`; the `sky` import). Keep `osloLat`/`osloLon`, `Stamps`, `Power`, `Train`, `Calendar` for now.
- Delete: `internal/view/icons.go`, `internal/views/current_weather.templ`, `internal/views/current_weather_templ.go`, `internal/views/hourly_forecast.templ`, `internal/views/hourly_forecast_templ.go`, `internal/views/sky.templ`, `internal/views/sky_templ.go`, `internal/sky/` (whole package), `TestWeatherIcon` in `format_test.go`.
- Modify (keep compiling): `internal/web/server.go` — remove the `sky`, `current-weather` and `hourly-forecast` fragments, the `Sky`/`CurrentWeather`/`HourlyForecast` fields in `page()`, and `sky()`, `currentWeather()`, `hourlyForecast()`; `internal/views/layout.templ` — remove the `Sky`, `CurrentWeather`, `HourlyForecast` fields from `Page` and their `@` calls plus the `<canvas>` and `#sky-driver`. (Full layout rewrite comes in Task 5; this step only keeps the build green.) `internal/web/web_test.go` — in `TestSSEStartsWithSnapshot` set the expected snapshot length to 4 and the names to `"calendar power-home power-cabin entur version"`; in `TestPageAndStaticFiles` drop the `current_temperature` case.

**Interfaces:**
- Consumes: `Deg` from Task 1; `weather.Hourly` (`Hour int`, `Symbol`, `PrecipitationAmount`, `AirTemperature *float64`; `hours[0]` is the hour `now` falls in); `smarthouse.Readings.TempOut *float64`; `sun.Times(day, lat, lon) (rise, set, ok)`.
- Produces:

```go
type Weather struct {
	Temperature string   // "11°", "" without sensor and forecast
	Line1       string   // "Sol nå, skyet fra kl. 14"
	Line2       string   // "Ned mot 4° i natt · regn fra kl. 02"
	Columns     []Column // up to six
}
type Column struct {
	Hour        string // "15", "00"
	Temperature string // "7°", "" when unknown
	Rain        bool
	RainPx      string // bar thickness at 1080p, "3".."15"; "" when !Rain
	RainLevel   int    // 1 light, 2 moderate, 3 heavy; 0 when !Rain
}
func BuildWeather(r smarthouse.Readings, hours []weather.Hourly, now time.Time) Weather
func symbolWord(symbol string) string
func line1(hours []weather.Hourly) string
func line2(hours []weather.Hourly, now time.Time) string
func columns(hours []weather.Hourly) []Column

type Sun struct{ Rise, Set string } // "07:23", "16:37"; both "" when the sun doesn't rise/set
func BuildSun(now time.Time) Sun
```

Templ: `templ Weather(w view.Weather)` renders the `.now` block (temperature, two lines) and the `.hours` row; `templ Sun(s view.Sun)` renders `<span class="label">Sol</span>` and, when `Rise != ""`, `<div class="value">07:23 · 16:37</div>`.

- [ ] **Step 1: Write the failing tests** in `internal/view/weather_test.go`

Helper: `func hrs(startHour int, temps []float64, rain []float64, symbols []string) []weather.Hourly` building consecutive hours from `startHour` (wrapping at 24; `symbols` may be shorter and defaults to `"cloudy"`, `rain` may be nil).

```go
func TestSymbolWord(t *testing.T) {
	cases := map[string]string{
		"clearsky_day": "Sol", "clearsky_night": "Klart", "clearsky_polartwilight": "Sol",
		"fair_day": "Lettskyet", "partlycloudy_night": "Delvis skyet", "cloudy": "Skyet", "fog": "Tåke",
		"lightrain": "Regn", "rain": "Regn", "heavyrain": "Kraftig regn",
		"rainshowers_day": "Byger", "heavyrainshowers_night": "Byger",
		"sleet": "Sludd", "lightsleetshowers_day": "Sludd",
		"snow": "Snø", "heavysnowshowers_day": "Snø", "lightsnow": "Snø",
		"rainandthunder": "Torden", "heavysnowshowersandthunder_day": "Torden",
		"": "Skyet", "whatever": "Skyet",
	}
	// assert each
}

func TestLine1(t *testing.T) {
	h := hrs(11, nil, nil, []string{"clearsky_day", "clearsky_day", "clearsky_day", "cloudy"})
	if got := line1(h); got != "Sol nå, skyet fra kl. 14" { t.Errorf("got %q", got) }
	same := hrs(11, nil, nil, []string{"cloudy", "cloudy", "cloudy"})
	if got := line1(same); got != "Skyet nå og de neste timene" { t.Errorf("got %q", got) }
	// a change at hour 13 only counts within the next twelve hours
	late := hrs(11, nil, nil, append(repeat("cloudy", 13), "clearsky_day"))
	if got := line1(late); got != "Skyet nå og de neste timene" { t.Errorf("got %q", got) }
	if got := line1(nil); got != "" { t.Errorf("got %q", got) }
}

func TestLine2(t *testing.T) {
	oslo := func(h int) time.Time { return time.Date(2026, 10, 26, h, 30, 0, 0, tz.Oslo) }
	// Morning: highest temperature for the rest of the day, rain from the first wet hour
	temps := []float64{8, 9, 11.6, 12.2, 10, 7, 5, 4, 3.6, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4}
	rain := make([]float64, 24); rain[15] = 0.3
	h := hrs(9, temps, rain, nil)
	if got := line2(h, oslo(9)); got != "Opp mot 12° i dag · regn fra kl. 00" { t.Errorf("got %q", got) }
	// Evening: lowest temperature until 08 tomorrow (index 16); 09 (index 17) is outside the window
	h = hrs(16, []float64{7, 5, 4, 4.4, 4, 4, 4, 4, 5, 5, 5, 5, 5, 5, 5, 5, 9, 1, 1, 1}, nil, nil)
	if got := line2(h, oslo(16)); got != "Ned mot 4° i natt" { t.Errorf("got %q", got) }
	// Raining now: until the first dry hour; snow symbol changes the word
	rain = []float64{1, 1, 0.5, 0, 0}
	h = hrs(16, []float64{2, 2, 2, 2, 2}, rain, []string{"snow", "snow", "snow", "cloudy"})
	if got := line2(h, oslo(16)); got != "Ned mot 2° i natt · snø til kl. 19" { t.Errorf("got %q", got) }
	// A sleet shower at 02
	rain = make([]float64, 12); rain[10] = 0.2
	h = hrs(16, repeatF(1, 12), rain, append(repeat("cloudy", 10), "sleetshowers_night"))
	if got := line2(h, oslo(16)); got != "Ned mot 1° i natt · sludd fra kl. 02" { t.Errorf("got %q", got) }
	if got := line2(nil, oslo(9)); got != "" { t.Errorf("got %q", got) }
}

func TestColumnsStartAtNextMultipleOfThree(t *testing.T) {
	cols := columns(hrs(11, repeatF(5, 24), nil, nil))
	want := []string{"12", "15", "18", "21", "00", "03"}
	// assert len 6 and Hour labels
	cols = columns(hrs(12, repeatF(5, 24), nil, nil))
	// first label "15"
	cols = columns(hrs(23, repeatF(5, 24), nil, nil))
	// first label "00"
}

func TestColumnsSumRainAndColourByWettestHour(t *testing.T) {
	rain := make([]float64, 24)
	rain[1], rain[2], rain[3] = 0.3, 0.3, 0.3 // column "12": sum 0.9, max 0.3 → level 1, 4 px
	rain[5] = 2.5                              // column "15": sum 2.5, max 2.5 → level 3, 6 px
	rain[7], rain[8] = 1, 1                    // column "18": sum 2, max 1 → level 2, 6 px
	rain[10] = 0.05                            // column "21": below threshold → no rain
	cols := columns(hrs(11, repeatF(5, 24), rain, nil))
	// assert cols[0]: Rain, RainLevel 1, RainPx "4"
	// assert cols[1]: Rain, RainLevel 3, RainPx "6"
	// assert cols[2]: Rain, RainLevel 2, RainPx "6"
	// assert cols[3]: !Rain, RainLevel 0, RainPx ""
	// 9 mm in one column → RainPx "15"
}

func TestColumnsWithShortForecast(t *testing.T) {
	cols := columns(hrs(11, repeatF(5, 8), nil, nil)) // hours 11..18 only
	// expect labels "12","15","18" and no panic; a column whose hour is missing is omitted
	if len(cols) != 3 { t.Errorf("got %d columns", len(cols)) }
	if got := columns(nil); len(got) != 0 { t.Errorf("got %d", len(got)) }
}

func TestBuildWeatherTemperature(t *testing.T) {
	now := time.Date(2026, 10, 26, 11, 0, 0, 0, tz.Oslo)
	h := hrs(11, []float64{10.6}, nil, nil)
	sensor := 11.4
	if got := BuildWeather(smarthouse.Readings{TempOut: &sensor}, h, now).Temperature; got != "11°" { t.Errorf("got %q", got) }
	if got := BuildWeather(smarthouse.Readings{}, h, now).Temperature; got != "11°" { t.Errorf("fallback got %q", got) } // 10.6 rounds to 11
	if got := BuildWeather(smarthouse.Readings{}, nil, now); got.Temperature != "" || got.Line1 != "" || got.Line2 != "" || len(got.Columns) != 0 {
		t.Errorf("empty got %+v", got)
	}
}

func TestBuildSun(t *testing.T) {
	s := BuildSun(time.Date(2026, 10, 26, 12, 0, 0, 0, tz.Oslo))
	if s.Rise != "07:23" || s.Set != "16:37" { t.Errorf("got %+v", s) }
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/view/ -run 'TestSymbolWord|TestLine|TestColumns|TestBuildWeather|TestBuildSun' -v`
Expected: FAIL, undefined symbols.

- [ ] **Step 3: Implement `internal/view/weather.go`**

Rules the tests pin, in order:
- `symbolWord`: strip everything from the first `_`; check `thunder` → Torden, then `sleet` → Sludd, `snow` → Snø, `showers` → Byger, `heavyrain` → Kraftig regn, `rain` → Regn; then exact `clearsky` (Sol, or Klart when the original symbol ends in `_night`), `fair`, `partlycloudy`, `cloudy`, `fog`; default Skyet.
- `line1`: word of `hours[0]`; first `i` in `1..12` whose word differs → `"<W> nå, <lower(w)> fra kl. %02d"`, else `"<W> nå og de neste timene"`.
- `line2`: temperature part: `now.Hour() < 15` → max `AirTemperature` over `hours[i]` while `hours[i].Hour >= hours[0].Hour` → `"Opp mot <Deg> i dag"`; otherwise min over indices `0 .. (24-hours[0].Hour)+8` → `"Ned mot <Deg> i natt"`. Rain part: wet means `PrecipitationAmount >= 0.1`; if `hours[0]` is wet, first dry `i` in `1..18` → `"<word> til kl. %02d"` with the word from `hours[0].Symbol`; else first wet `i` in `1..18` → `"<word> fra kl. %02d"` with the word from that hour's symbol; word is `snø` when the symbol contains `snow`, `sludd` for `sleet`, else `regn`. Parts joined with `" · "`.
- `columns`: `k` = smallest `k ≥ 1` with `(hours[0].Hour + k) % 3 == 0`; columns at `k, k+3, …` while the index exists, at most six. Sum = `PrecipitationAmount` over the up-to-three hours from the index; max = wettest of them. `Rain = sum >= 0.1`; `RainLevel` 1 if max < 0.5, 2 if max < 2, else 3; `RainPx = Num(3 + tz.Round(12 * min(sum, 9) / 9))`. `Temperature = Deg(*AirTemperature)` or `""`.
- `BuildWeather`: sensor `TempOut` wins, else `hours[0].AirTemperature`, else `""`.
- `BuildSun`: `sun.Times(now.In(tz.Oslo), osloLat, osloLon)`, `Format("15:04")`.

- [ ] **Step 4: Write the templates** `internal/views/weather.templ` and `internal/views/sun.templ`

Weather markup (classes are what Task 7's CSS targets):

```html
<div class="now">
	<div class="temp">{ w.Temperature }</div>
	<div class="lines"><div class="line1">{ w.Line1 }</div><div class="line2">{ w.Line2 }</div></div>
</div>
<div class="hours">
	for each column: <div class={ "hour", templ.KV("rain", c.Rain) }>
		<div class="h">{ c.Hour }</div>
		<div class="t">{ c.Temperature }</div>
		if c.Rain { <div class={ "bar", "bar-" + strconv.Itoa(c.RainLevel) } style={ "height: " + c.RainPx + "px" }></div> }
	</div>
</div>
```

Then delete the old code and files listed above, trim `server.go`, `layout.templ`, `web_test.go` as described, and run `go generate ./...`.

- [ ] **Step 5: Run everything**

Run: `go generate ./... && go vet ./... && go test ./...`
Expected: PASS across packages, `internal/sky` gone from the tree.

- [ ] **Step 6: Commit**

```bash
git add -A internal/view internal/views internal/web internal/sky
git commit -m "Add the Natt weather view: sentences, three-hour columns and sun times; drop the sky and hourly chart"
```

---

### Task 3: Calendar view and template

**Files:**
- Create: `internal/view/calendar.go`, `internal/view/calendar_test.go` (move the calendar section out of `view.go`, including `TestBuildCalendarGivesHyttaEventsTheCabinForecast` rewritten as `TestHyttaSuffixOnlyToday`)
- Modify: `internal/calendar/enrich.go` (add `Birthday`, use it in `displayTitle`), `internal/calendar/enrich_test.go` or `calendar_test.go` (add `TestBirthday`)
- Modify: `internal/views/calendar.templ` (rewrite), regenerate
- Modify: `internal/view/view.go` (remove the calendar section)

**Interfaces:**
- Consumes: `calendar.Snapshot` (`EventsForDate`, `BirthdaysForDate`, `DinnersForDate`, `KidsStatusForDate`, `BhgDaysLeft`), `calendar.EnrichedEvent` (`DisplayTitle`, `DisplayTime.Start`, `FullDay`, `Faded`, `Title`), `weather.Daily` (`MinTemp`, `MaxTemp`, `LightRainProbability`), `Deg`, `DegRange`, `Capitalize`, `weekdays` from `format.go`, `view.Stamps`.
- Produces:

```go
// internal/calendar/enrich.go
// Birthday parses "Kai Lützenkirchen 1981" into the name and the age reached in thisYear.
func (e Event) Birthday(thisYear int) (name string, age int, ok bool)

// internal/view/calendar.go
type Calendar struct {
	Today Today
	Days  []Day  // tomorrow onwards
	Bhg   string // "bhg: 12", "" until fetched
}
type Today struct {
	Kids   calendar.KidsStatus
	Dinner string       // several joined with " · "
	Notes  []string     // "Kai Lützenkirchen fyller 45", then full-day events
	Events []TodayEvent // timed events in start order
}
type TodayEvent struct {
	Time  string // "12:00"; "" for an event that started on an earlier day
	Title string
	Hytta string // "(5°, 20 % regn)" or ""
	Faded bool
}
type Day struct {
	Kids    calendar.KidsStatus
	Label   string // "Tirsdag" for days 1–6, "Mandag 2." from day 7
	Dinner  string
	Weather string // "5–12° · regn", "5–12°", ""
	Events  []DayEvent // birthdays "Kai 45 år", full-day titles, then "11:00 PT (Lena)"
}
type DayEvent struct {
	Text  string
	Faded bool
}
func BuildCalendar(cal calendar.Snapshot, daily, dailyHytta map[string]weather.Daily, now time.Time, days int) Calendar
```

Templ: `templ Calendar(c view.Calendar, st view.Stamps)` renders `.today`, `.days` (one `.day` per `c.Days`), and `.corner`; `templ kidsDot(k calendar.KidsStatus)` renders `<span class="dot dot-full|dot-away|dot-arriving|dot-leaving"></span>`.

- [ ] **Step 1: Write the failing tests**

In the calendar package:

```go
func TestBirthday(t *testing.T) {
	name, age, ok := Event{Title: "Kai Lützenkirchen 1981"}.Birthday(2026)
	if !ok || name != "Kai Lützenkirchen" || age != 45 { t.Errorf("got %q %d %v", name, age, ok) }
	if _, _, ok := (Event{Title: "Kai"}).Birthday(2026); ok { t.Error("no year should not parse") }
}
```

In `internal/view/calendar_test.go`, with `mon := time.Date(2026, 10, 26, 0, 0, 0, 0, tz.Oslo)` and `now := mon.Add(11 * time.Hour)`:

```go
func TestTodaySplitsDinnerNotesAndEvents(t *testing.T) {
	snap := calendar.Snapshot{
		Dinners:   []calendar.Event{{Title: "Pannekaker", Start: mon, End: mon, FullDay: true}},
		Birthdays: []calendar.Event{{Title: "Kai Lützenkirchen 1981", Start: mon, End: mon, FullDay: true}},
		Events: []calendar.Event{
			{Title: "Turdag E", Start: mon, End: mon, FullDay: true},
			{Title: "Agnes på turn", Start: mon.Add(17 * time.Hour), End: mon.Add(18*time.Hour + 30*time.Minute)},
			{Title: "Passesta: Audun og Tore", Start: mon.Add(12 * time.Hour), End: mon.Add(13 * time.Hour)},
		},
	}
	c := BuildCalendar(snap, nil, nil, now, 2)
	// Today.Dinner == "Pannekaker"
	// Today.Notes == []string{"Kai Lützenkirchen fyller 45", "Turdag E"}
	// Today.Events == [{Time:"12:00", Title:"Passesta: Audun og Tore"}, {Time:"17:00", Title:"Agnes på turn"}]
	// len(c.Days) == 1 (tomorrow)
}

func TestTodayMultiDayEventHasNoTime(t *testing.T) {
	// event from Sunday 20:00 to Tuesday 10:00 → today: Time "" and Title set
}

func TestDayLabelsAddDateFromDaySeven(t *testing.T) {
	c := BuildCalendar(calendar.Snapshot{}, nil, nil, now, 9)
	want := []string{"Tirsdag", "Onsdag", "Torsdag", "Fredag", "Lørdag", "Søndag", "Mandag 2.", "Tirsdag 3."}
	// compare c.Days labels
}

func TestDayWeatherAndRainWord(t *testing.T) {
	daily := map[string]weather.Daily{
		mon.AddDate(0, 0, 1).Format(time.DateOnly): {MinTemp: 4.6, MaxTemp: 12.2, LightRainProbability: 50},
		mon.AddDate(0, 0, 2).Format(time.DateOnly): {MinTemp: 5, MaxTemp: 11, LightRainProbability: 40},
	}
	c := BuildCalendar(calendar.Snapshot{}, daily, nil, now, 4)
	// Days[0].Weather == "5–12° · regn", Days[1].Weather == "5–11°", Days[2].Weather == ""
}

func TestDayWithoutForecast(t *testing.T) {
	c := BuildCalendar(calendar.Snapshot{}, map[string]weather.Daily{}, nil, now, 3)
	// every Days[i].Weather == "" and none contains "·"
}

func TestDayEventsLine(t *testing.T) {
	tue := mon.AddDate(0, 0, 1)
	snap := calendar.Snapshot{
		Dinners:   []calendar.Event{{Title: "Lasagne", Start: tue, End: tue, FullDay: true}, {Title: "Is", Start: tue, End: tue, FullDay: true}},
		Birthdays: []calendar.Event{{Title: "Kai Lützenkirchen 1981", Start: tue, End: tue, FullDay: true}},
		Events: []calendar.Event{
			{Title: "PT (Lena)", Start: tue.Add(11 * time.Hour), End: tue.Add(12 * time.Hour), Source: calendar.Felles},
			{Title: "Turdag E", Start: tue, End: tue, FullDay: true},
		},
	}
	c := BuildCalendar(snap, nil, nil, now, 2)
	// Days[0].Dinner == "Lasagne · Is"
	// Days[0].Events texts == ["Kai 45 år", "Turdag E", "11:00 PT (Lena)"]
	// Days[0].Events[2].Faded == true (Felles event, kids away)
}

func TestHyttaSuffixOnlyToday(t *testing.T) {
	// "Hytta" timed event today and tomorrow, dailyHytta for both days with MaxTemp 5, LightRainProbability 20
	// Today.Events[0].Hytta == "(5°, 20 % regn)"; Days[0].Events[0].Text == "13:00 Hytta" with no suffix
}

func TestBhgCorner(t *testing.T) {
	n := 12
	// BuildCalendar(calendar.Snapshot{BhgDaysLeft: &n}, …).Bhg == "bhg: 12"; nil → ""
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/calendar/ ./internal/view/ -run 'TestBirthday|TestToday|TestDay|TestHytta|TestBhg' -v`
Expected: FAIL (undefined / wrong shapes).

- [ ] **Step 3: Implement**

- `Birthday`: reuse `birthYearPattern`; name is the title minus the last five runes; age `thisYear - year`. `displayTitle`'s birthday branch calls it and keeps producing `"Name (45 år)"` so nothing else changes.
- `BuildCalendar`: today from index 0, `Days` from indices `1..days-1`. Labels: `Capitalize(weekdays[wd])`, plus `" " + day + "."` when `i >= 7`. Weather: `DegRange(min, max)` plus `" · regn"` when `LightRainProbability >= 50`. Notes: birthdays as `name + " fyller " + age` (fallback `DisplayTitle`), then full-day events' `DisplayTitle`. Timed events (today and coming days) are sorted by `Start` with `slices.SortStableFunc`, since `Snapshot.Events` order is not guaranteed. Day events: birthdays as first word of name + `" " + age + " år"`, then full-day, then timed as `DisplayTime.Start + " " + DisplayTitle` (just the title when `Start == ""`). Hytta suffix only on today's timed events: `"(" + Deg(MaxTemp) + ", " + Pct(LightRainProbability) + " regn)"` — note `Pct` already carries the narrow space.
- Template: as in the spec's layout section. `.today-head` holds the label and the dot; `.ev` rows have `.time` and `.title` spans and `.faded` when faded; `.day` has `.row1` (dot, `.weekday`, `.dinner`, `.wx`) and `.row2` with event spans separated by `<span class="sep">·</span>`; `.corner` holds `c.Bhg`, `st.Built`, `st.Started` and the two `id="stamp-loaded"` / `id="stamp-data"` spans `client.js` fills.

- [ ] **Step 4: Run tests and generate**

Run: `go generate ./... && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A internal/calendar internal/view internal/views
git commit -m "Rebuild the calendar column: today, compact coming days, corner line"
```

---

### Task 4: Power and trains views and templates

**Files:**
- Modify: `internal/view/view.go` (rewrite the Power and Entur sections; drop `green`/`orange`/`blue`, bar fields, the Megajoule branch)
- Modify: `internal/view/view_test.go` (replace `TestBuildTrainsSkipsDeparted`; add power tests)
- Modify: `internal/views/power.templ`, `internal/views/entur.templ` (rewrite), regenerate

**Interfaces:**
- Consumes: `KW`, `Kr`, `Pct` from Task 1; `tibber.PowerData` (`Timestamp`, `CurrentPower` W, `EffectivePrice`, `CurrentPrice`, `MonthlyConsumption`, `Cap`); `entur.Train` (`Time`, `Destination`, `Realtime`, `Delayed`).
- Produces:

```go
type Power struct {
	Label   string // "Strøm hjemme", "Hytta"
	HasData bool
	Power   string // "0,0 kW"
	Detail  string // "0,50 kr · 27 %" (just "0,50 kr" when Cap == 0)
}
func BuildPower(d tibber.PowerData, place tibber.Place) Power

type Trains struct {
	Departures []Train // at most two, none under two minutes away
	Direction  string  // "mot Bergkrystallen", "" when empty
}
type Train struct {
	Time string // "11:48"
	Dot  string // "red" delayed, "grey" no realtime, "" otherwise
}
func BuildTrains(trains []entur.Train, now time.Time) Trains
```

Templ: `templ Power(p view.Power)`: `<span class="label">{ p.Label }</span>` then, when `HasData`, `<div class="value">{ p.Power }</div><div class="detail">{ p.Detail }</div>`. `templ Entur(t view.Trains)`: label `T-bane`, then `<div class="value">` with the departures (`<span class="next">11:48</span>`, dot span as today, `<span class="sep">·</span>`, second time), then `<div class="detail">{ t.Direction }</div>`.

- [ ] **Step 1: Write the failing tests**

```go
func TestBuildTrainsSkipsDepartedAndImminent(t *testing.T) {
	now := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC) // 10:00 Oslo
	var trains []entur.Train
	for _, m := range []int{-3, 1, 2, 5, 9} {
		trains = append(trains, entur.Train{Time: now.Add(time.Duration(m) * time.Minute), Destination: "Bergkrystallen", Realtime: true})
	}
	got := BuildTrains(trains, now)
	// Departures == [{"10:02",""},{"10:05",""}], Direction == "mot Bergkrystallen"
}

func TestBuildTrainsDots(t *testing.T) {
	// Delayed → "red"; !Realtime → "grey"
}

func TestBuildTrainsNoneReachable(t *testing.T) {
	// all departures within one minute → no Departures and Direction ""
	// nil input → same
}

func TestBuildPowerHome(t *testing.T) {
	d := tibber.PowerData{Timestamp: time.Now(), CurrentPower: 0, EffectivePrice: 0.5, MonthlyConsumption: 1350, Cap: 5000}
	p := BuildPower(d, tibber.Home)
	// Label "Strøm hjemme", HasData, Power "0,0 kW", Detail "0,50 kr · 27 %"
	d.CurrentPower = -400
	// Power "−0,4 kW"
}

func TestBuildPowerAboveCap(t *testing.T) {
	// MonthlyConsumption 1080, Cap 1000, place Cabin → Label "Hytta", Detail ends with "108 %"
}

func TestBuildPowerWithoutCap(t *testing.T) {
	// Cap 0, EffectivePrice 0, CurrentPrice 0.22 → Detail "0,22 kr"
}

func TestBuildPowerNoData(t *testing.T) {
	// zero PowerData → HasData false, Power "" and Detail ""
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/view/ -run 'TestBuildTrains|TestBuildPower' -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

`BuildPower`: `HasData = !d.Timestamp.IsZero()`; when false return only the label. Price is `EffectivePrice`, or `CurrentPrice` when that is zero. `Detail = Kr(price)` plus `" · " + Pct(d.MonthlyConsumption/d.Cap*100)` when `Cap > 0`. `BuildTrains`: skip `t.Time.Sub(now) < 2*time.Minute`; stop at two; `Direction = "mot " + first.Destination`.

- [ ] **Step 4: Run tests and generate**

Run: `go generate ./... && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A internal/view internal/views
git commit -m "Reduce power and metro panels to one number and one detail line each"
```

---

### Task 5: Page layout and server wiring

**Files:**
- Modify: `internal/views/layout.templ` (full rewrite), regenerate
- Modify: `internal/web/server.go` (`fragments()`, `page()`, `Publish`, helper methods)
- Modify: `internal/web/web_test.go`

**Interfaces:**
- Consumes: `views.Weather`, `views.Sun`, `views.Calendar`, `views.Power`, `views.Entur`; `view.BuildWeather`, `view.BuildSun`, `view.BuildCalendar`, `view.BuildPower`, `view.BuildTrains`, `view.BuildStamps`.
- Produces: `views.Page{Version, Stamps, Weather view.Weather, Sun view.Sun, Calendar view.Calendar, PowerHome, PowerCabin view.Power, Trains view.Trains}`. Fragment names and order: `weather`, `sun`, `calendar`, `power-home`, `power-cabin`, `trains`. Ticks: power 1 s, `weather sun calendar` 15 s, `trains` 30 s, version 1 min.

Layout markup (the ids `now_time`, `now_date`, `now_week`, `stamp-loaded`, `stamp-data` are what `client.js` expects):

```html
<body data-version={ p.Version }>
	<main>
		<section class="clock">
			<div id="now_time"></div>
			<div class="date"><span id="now_date"></span> <span id="now_week"></span></div>
		</section>
		<section class="weather" data-sse="weather">@Weather(p.Weather)</section>
		<section class="bottom">
			<div class="cell" data-sse="trains">@Entur(p.Trains)</div>
			<div class="cell" data-sse="power-home">@Power(p.PowerHome)</div>
			<div class="cell" data-sse="power-cabin">@Power(p.PowerCabin)</div>
			<div class="cell" data-sse="sun">@Sun(p.Sun)</div>
		</section>
	</main>
	<aside class="calendar" data-sse="calendar">@Calendar(p.Calendar, p.Stamps)</aside>
	<script type="module" src="/client.js"></script>
</body>
```

Head: `<html lang="nb">`, charset, viewport, title `Tellulf`, the two `preconnect` links, `https://fonts.googleapis.com/css2?family=Inter+Tight:wght@200;300;400&display=swap`, `/styles.css`.

- [ ] **Step 1: Update the tests**

In `TestPageAndStaticFiles`: keep `{"/", `data-sse="calendar"`, 200}`, add `{"/", `data-sse="weather"`, 200}` and `{"/", "Inter+Tight", 200}`, `{"/favicon.ico", "", 200}`; change `/vendor/idiomorph.esm.js` and `/kids.svg` to expect 404 (they go in Task 6, so these two cases are added in Task 6 instead — leave them out here). In `TestSSEStartsWithSnapshot`: snapshot length 6, loop `len(names) < 7`, `want = "weather sun calendar power-home power-cabin trains version"`.

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/web/ -v`
Expected: FAIL on the fragment names.

- [ ] **Step 3: Rewrite `layout.templ` and wire `server.go`**

Add `weather(now)`, `sun(now)` methods; `calendar(now)` unchanged; `power(p)` drops `now`; `trains(now)` returns `view.Trains`. `Publish`: `data := []string{"weather", "sun", "calendar"}`, `trains := []string{"trains"}`.

- [ ] **Step 4: Generate, vet, test**

Run: `go generate ./... && go vet ./... && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A internal/views internal/web
git commit -m "Lay out the Natt page and publish its fragments"
```

---

### Task 6: Client JavaScript and asset cleanup

**Files:**
- Modify: `public/live.js` (drop idiomorph import and `data-morph`; `swap` is just `innerHTML`), `public/client.js`, `public/report.js` (comment only: "sleep, reloads, JS errors")
- Delete: `public/sky.js`, `static/vendor/` (whole dir), `static/weather-icons-animated/`, `static/weather-icons-static/`, `static/wave1.svg`, `static/wave2.svg`, `static/wave3.svg`, `static/high-temperature.png`, `static/low-temperature.png`, `static/sunrise.png`, `static/sunset.png`, `static/kids.svg`, `static/dinner.svg`, `static/birthday.svg`
- Modify: `internal/web/web_test.go` (add `{"/vendor/idiomorph.esm.js", "", 404}`, `{"/kids.svg", "", 404}`)

**Interfaces:**
- Consumes: element ids from Task 5; `.days` / `.day` / `.day-hidden` classes from Task 3.

`client.js` after the change keeps: error/visibility reporting, clock (`timeFmt`), date with month (`dateFmt` gets `month: 'long'`, output capitalised: `s.charAt(0).toUpperCase() + s.slice(1)`), week (`uke 44`), stamps, overflow check, version reload, midnight reload. It drops `initSky`, the click-to-reload listener. Overflow: `checkCalendarVisibility` measures `document.querySelector('.calendar .days')` and toggles `day-hidden` on its `.day` children; `ResizeObserver` observes `.days`.

- [ ] **Step 1: Make the changes and delete the files**

```bash
git rm -r public/sky.js static/vendor static/weather-icons-animated static/weather-icons-static \
  static/wave1.svg static/wave2.svg static/wave3.svg static/high-temperature.png static/low-temperature.png \
  static/sunrise.png static/sunset.png static/kids.svg static/dinner.svg static/birthday.svg
```

- [ ] **Step 2: Lint and test**

Run: `npx @biomejs/biome@2.5.15 check --write public && go test ./...`
Expected: Biome reports no errors; Go tests PASS including the two new 404 cases.

- [ ] **Step 3: Smoke the page in headless Chrome for JS errors**

Start `TELLULF_DEV=1 go run .` in the background, then:

```bash
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --headless=new --use-angle=swiftshader --enable-unsafe-swiftshader --hide-scrollbars --window-size=1920,1080 --timeout=12000 --enable-logging=stderr --v=0 --screenshot=/dev/null http://localhost:3000/ 2>&1 | grep -i "console" | grep -iv "favicon"
```

Expected: no `Uncaught` lines. The server log shows `msg=client` lines for `page: loaded` and `sse: connected`.

- [ ] **Step 4: Commit**

```bash
git add -A public static internal/web
git commit -m "Trim the client to clock, stamps, overflow and reload; remove sky, icons and idiomorph"
```

---

### Task 7: Stylesheet

**Files:**
- Modify: `public/styles.css` (full rewrite)

**Interfaces:**
- Consumes: the class names from Tasks 2–5: `main`, `.clock`, `#now_time`, `.date`, `#now_week`, `.weather`, `.now`, `.temp`, `.lines`, `.line1`, `.line2`, `.hours`, `.hour`, `.h`, `.t`, `.rain`, `.bar`, `.bar-1/2/3`, `.bottom`, `.cell`, `.label`, `.value`, `.next`, `.detail`, `.train-dot`, `.train-dot-red`, `.train-dot-grey`, `.calendar`, `.today`, `.today-head`, `.dinner`, `.note`, `.ev`, `.time`, `.title`, `.faded`, `.days`, `.day`, `.day-hidden`, `.row1`, `.row2`, `.weekday`, `.wx`, `.sep`, `.dot`, `.dot-full/away/arriving/leaving`, `.corner`, `.hytta`.

Starting values from the mockup × 1.5 (tune by eye against the mockup; keep to an 8 px grid):

- `body`: `background: var(--bg); color: var(--fg); margin: 0; width: 1920px; height: 1080px; overflow: hidden; cursor: none; padding: 72px 96px; display: grid; grid-template-columns: 912px 702px; column-gap: 114px; font: 300 28px/1.3 "Inter Tight", sans-serif; font-variant-numeric: tabular-nums;`
- `#now_time`: 280px, weight 200, `letter-spacing: -0.02em`, `line-height: 1`; `.date`: 40px muted; `#now_week` faint with `::before { content: "· " }`.
- `.now`: flex, `align-items: center`, gap 40px; `.temp` 120px weight 200; `.line1` 28px fg; `.line2` 24px muted.
- `.hours`: `border-top: 1px solid var(--faint)`, padding-top 24px, grid of six equal columns; `.h` 18px muted; `.t` 30px; `.hour.rain .t { color: var(--rain-3) }`; `.bar { width: 60%; margin-top: 14px; border-radius: 2px; }` with `.bar-1/2/3` backgrounds `--rain-1/2/3`.
- `.bottom`: `border-top: 1px solid var(--faint)`, padding-top 24px, grid of four columns; `.label` 18px uppercase muted letter-spacing 0.08em; `.value` 40px; `.next { color: var(--accent) }`; `.detail` 20px muted; `.train-dot` 8px circle inline, red `#d9534f`-ish, grey `var(--faint)`.
- `.calendar`: flex column; `.today` auto; `.days { flex: 1; overflow: hidden; }`; `.corner { height: 24px; font-size: 14px; color: var(--faint); text-align: right; }`.
- `.today-head`: flex, space-between; `.label` in `--accent`; `.dinner` 48px weight 400; `.note` 24px muted; `.ev` grid `80px 1fr`, 28px, `.time` muted, `.hytta` muted; `.faded { color: var(--faint) }`.
- `.days`: `border-top: 1px solid var(--faint)`, padding-top 24px; `.day` margin-bottom 20px; `.row1` grid `20px 150px 1fr auto`, align baseline, `.weekday` 24px muted, `.dinner` 28px fg, `.wx` 22px muted; `.row2` 20px muted, `white-space: nowrap; overflow: hidden; text-overflow: ellipsis; padding-left: 170px`; `.sep` margin `0 8px`.
- `.dot`: 12px circle, `border: 1.5px solid var(--accent)`; `.dot-full` filled; `.dot-arriving` `background: linear-gradient(to right, transparent 50%, var(--accent) 50%)`; `.dot-leaving` the mirror.
- No `transition`, no `animation` anywhere.

- [ ] **Step 1: Write the stylesheet and check it has no motion**

Run: `grep -c -E "transition|animation|@keyframes" public/styles.css`
Expected: `0`.

- [ ] **Step 2: Screenshot at 1920x1080 and compare with the mockup**

With `TELLULF_DEV=1 go run .` running:

```bash
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --headless=new --use-angle=swiftshader --enable-unsafe-swiftshader --hide-scrollbars --window-size=1920,1080 --timeout=12000 --screenshot=/tmp/natt.png http://localhost:3000/
```

Open `/tmp/natt.png` next to `docs/superpowers/specs/2026-10-26-natt-mockup.webp`. Check: hierarchy (clock > date > dinner), one left edge in each column, corner line clear of the last day, no horizontal scroll, labels uppercase, numbers tabular. Adjust sizes until it matches; repeat.

- [ ] **Step 3: Biome and tests**

Run: `npx @biomejs/biome@2.5.15 check public && go test ./...`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add public/styles.css
git commit -m "Style the Natt layout: dark, one font, no motion"
```

---

### Task 8: Documentation

**Files:**
- Modify: `CLAUDE.md`, `README.md`

- [ ] **Step 1: Update `CLAUDE.md`**

Architecture: fragments are `weather sun calendar power-home power-cabin trains`; client JS is `live.js` (plain `innerHTML` swaps), `client.js` (clock, date, calendar overflow, version check, corner times), `report.js`; remove idiomorph, `data-morph`, `sky.js` and the templ `<style>` note. Gotchas: delete the WebGL paragraph; `report.js` logs page loads, visibility changes and JS errors. Packages: remove `internal/sky`; keep `internal/sun` ("NOAA sunrise/sunset for the SOL cell"). Add one line under Gotchas: the layout is fixed at 1920x1080 and all rules for the wording of the weather lines live in `internal/view/weather.go`.

- [ ] **Step 2: Update `README.md`**

Project layout: `static/` is now just the favicon; `public/` "CSS and the browser JavaScript (clock, calendar overflow)". Remove the idiomorph update bullet. Mention the 1080p screenshot command under Development.

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md README.md
git commit -m "Document the Natt layout"
```

---

### Task 9: Final verification

- [ ] **Step 1: Full check**

Run: `go generate ./... && git status --porcelain internal/views && gofmt -l . && go vet ./... && go test -race ./... && npx @biomejs/biome@2.5.15 ci public`
Expected: no generated-file drift, no gofmt output, vet clean, tests PASS, Biome clean.

- [ ] **Step 2: Screenshots in both data states**

With the Tibber demo token from the README and without MQTT (dev `.env`), take the 1080p screenshot; then stop the server, run with `TIBBER_KEY` unset and screenshot again. Expected: in the second, STRØM HJEMME and HYTTA show only their labels; nothing shows `–`.

- [ ] **Step 3: Ask for a fresh code review of the branch** (`superpowers:requesting-code-review`), fix what it finds, re-run Step 1.

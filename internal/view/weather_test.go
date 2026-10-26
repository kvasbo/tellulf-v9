package view

import (
	"slices"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/smarthouse"
	"github.com/kvasbo/tellulf-v9/internal/tz"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

func repeat(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

func repeatF(v float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = v
	}
	return out
}

// hrs builds consecutive forecast hours from startHour. Each slice may be
// shorter than the others (or nil): missing symbols are "cloudy", missing
// rain is 0, missing temperatures are nil.
func hrs(startHour int, temps, rain []float64, symbols []string) []weather.Hourly {
	n := max(len(temps), len(rain), len(symbols))
	var out []weather.Hourly
	for i := range n {
		h := weather.Hourly{Hour: (startHour + i) % 24, Symbol: "cloudy"}
		if i < len(temps) {
			t := temps[i]
			h.AirTemperature = &t
		}
		if i < len(rain) {
			h.PrecipitationAmount = rain[i]
		}
		if i < len(symbols) {
			h.Symbol = symbols[i]
		}
		out = append(out, h)
	}
	return out
}

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
	for symbol, want := range cases {
		if got := symbolWord(symbol); got != want {
			t.Errorf("symbolWord(%q) = %q, want %q", symbol, got, want)
		}
	}
}

func TestLine1(t *testing.T) {
	h := hrs(11, nil, nil, []string{"clearsky_day", "clearsky_day", "clearsky_day", "cloudy"})
	if got := line1(h); got != "Sol nå, skyet fra kl. 14" {
		t.Errorf("got %q", got)
	}
	same := hrs(11, nil, nil, repeat("cloudy", 3))
	if got := line1(same); got != "Skyet nå og de neste timene" {
		t.Errorf("got %q", got)
	}
	// A change thirteen hours out is beyond the twelve-hour look-ahead.
	late := hrs(11, nil, nil, append(repeat("cloudy", 13), "clearsky_day"))
	if got := line1(late); got != "Skyet nå og de neste timene" {
		t.Errorf("got %q", got)
	}
	if got := line1(nil); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestLine2(t *testing.T) {
	oslo := func(h int) time.Time { return time.Date(2026, 10, 26, h, 30, 0, 0, tz.Oslo) }

	// Morning: highest temperature for the rest of the day, rain from the first wet hour.
	temps := []float64{8, 9, 11.6, 12.2, 10, 7, 5, 4, 3.6, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4}
	rain := make([]float64, 24)
	rain[15] = 0.3
	h := hrs(9, temps, rain, nil)
	if got := line2(h, oslo(9)); got != "Opp mot 12° i dag · regn fra kl. 00" {
		t.Errorf("morning: got %q", got)
	}

	// Evening: lowest temperature until 08 tomorrow (index 16); 09 (index 17) is outside.
	h = hrs(16, []float64{7, 5, 4, 4.4, 4, 4, 4, 4, 5, 5, 5, 5, 5, 5, 5, 5, 9, 1, 1, 1}, nil, nil)
	if got := line2(h, oslo(16)); got != "Ned mot 4° i natt" {
		t.Errorf("evening: got %q", got)
	}

	// Raining now: until the first dry hour, worded by the current symbol.
	h = hrs(16, repeatF(2, 5), []float64{1, 1, 0.5, 0, 0}, []string{"snow", "snow", "snow", "cloudy"})
	if got := line2(h, oslo(16)); got != "Ned mot 2° i natt · snø til kl. 19" {
		t.Errorf("snow now: got %q", got)
	}

	// A sleet shower at 02, worded by that hour's symbol.
	rain = make([]float64, 12)
	rain[10] = 0.2
	h = hrs(16, repeatF(1, 12), rain, append(repeat("cloudy", 10), "sleetshowers_night"))
	if got := line2(h, oslo(16)); got != "Ned mot 1° i natt · sludd fra kl. 02" {
		t.Errorf("sleet later: got %q", got)
	}

	if got := line2(nil, oslo(9)); got != "" {
		t.Errorf("empty: got %q", got)
	}
}

func labels(cols []Column) []string {
	var out []string
	for _, c := range cols {
		out = append(out, c.Hour)
	}
	return out
}

func TestColumnsStartAtNextMultipleOfThree(t *testing.T) {
	cases := map[int][]string{
		11: {"12", "15", "18", "21", "00", "03"},
		12: {"15", "18", "21", "00", "03", "06"},
		23: {"00", "03", "06", "09", "12", "15"},
	}
	for start, want := range cases {
		got := labels(columns(hrs(start, repeatF(5, 24), nil, nil)))
		if !slices.Equal(got, want) {
			t.Errorf("start %d: got %v, want %v", start, got, want)
		}
	}
	if got := columns(hrs(11, repeatF(5.4, 24), nil, nil))[0].Temperature; got != "5°" {
		t.Errorf("temperature = %q", got)
	}
}

func TestColumnsSumRainAndColourByWettestHour(t *testing.T) {
	rain := make([]float64, 24)
	rain[1], rain[2], rain[3] = 0.3, 0.3, 0.3 // "12": sum 0.9, max 0.3 → level 1, 4 px
	rain[5] = 2.5                             // "15": sum 2.5, max 2.5 → level 3, 6 px
	rain[7], rain[8] = 1, 1                   // "18": sum 2, max 1 → level 2, 6 px
	rain[10] = 0.05                           // "21": below threshold → no rain
	rain[13], rain[14], rain[15] = 3, 3, 3    // "00": 9 mm → 15 px
	cols := columns(hrs(11, repeatF(5, 24), rain, nil))
	want := []Column{
		{Hour: "12", Temperature: "5°", Rain: true, RainPx: "4", RainLevel: 1},
		{Hour: "15", Temperature: "5°", Rain: true, RainPx: "6", RainLevel: 3},
		{Hour: "18", Temperature: "5°", Rain: true, RainPx: "6", RainLevel: 2},
		{Hour: "21", Temperature: "5°"},
		{Hour: "00", Temperature: "5°", Rain: true, RainPx: "15", RainLevel: 3},
		{Hour: "03", Temperature: "5°"},
	}
	if !slices.Equal(cols, want) {
		t.Errorf("got  %+v\nwant %+v", cols, want)
	}
}

func TestColumnsWithShortForecast(t *testing.T) {
	cols := columns(hrs(11, repeatF(5, 8), nil, nil)) // hours 11..18 only
	if got := labels(cols); !slices.Equal(got, []string{"12", "15", "18"}) {
		t.Errorf("got %v", got)
	}
	if got := columns(nil); len(got) != 0 {
		t.Errorf("got %d columns from nothing", len(got))
	}
}

func TestBuildWeatherTemperature(t *testing.T) {
	now := time.Date(2026, 10, 26, 11, 0, 0, 0, tz.Oslo)
	h := hrs(11, []float64{10.6}, nil, nil)
	sensor := 11.4
	if got := BuildWeather(smarthouse.Readings{TempOut: &sensor}, h, now).Temperature; got != "11°" {
		t.Errorf("sensor: got %q", got)
	}
	if got := BuildWeather(smarthouse.Readings{}, h, now).Temperature; got != "11°" {
		t.Errorf("fallback: got %q", got)
	}
	got := BuildWeather(smarthouse.Readings{}, nil, now)
	if got.Temperature != "" || got.Line1 != "" || got.Line2 != "" || len(got.Columns) != 0 {
		t.Errorf("empty: got %+v", got)
	}
}

func TestBuildSun(t *testing.T) {
	s := BuildSun(time.Date(2026, 10, 26, 12, 0, 0, 0, tz.Oslo))
	if s.Rise != "07:23" || s.Set != "16:37" {
		t.Errorf("got %+v", s)
	}
}

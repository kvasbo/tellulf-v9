package view

import (
	"slices"
	"testing"
	"time"

	"github.com/kvasbo/tellulf-v9/internal/smarthouse"
	"github.com/kvasbo/tellulf-v9/internal/weather"
)

func f(v float64) *float64 { return &v }

// forecast is hourly from 14:00 on 27 October: temperatures from the
// list, rain in hours 10–12 (00–02 the next night) and wind 3 m/s with
// gusts of 6 from the south-west, 8 m/s at 01.
func forecast(n int) []weather.Hourly {
	var out []weather.Hourly
	start := oslo(27, 14, 0)
	for i := range n {
		t := start.Add(time.Duration(i) * time.Hour)
		h := weather.Hourly{Time: t, Hour: t.Hour(), Symbol: "cloudy",
			AirTemperature: f(8 - float64(i%24)/4), WindSpeed: f(3), WindSpeedOfGust: f(6), WindFromDirection: f(225)}
		switch i {
		case 10:
			h.PrecipitationAmount, h.Symbol = 0.3, "rain"
		case 11:
			h.PrecipitationAmount, h.Symbol, h.WindSpeed = 2.6, "rain", f(8)
		case 12:
			h.PrecipitationAmount, h.Symbol = 0.9, "rain"
		}
		out = append(out, h)
	}
	return out
}

func rowText(r Row) []string {
	out := []string{plain(r.Label), plain(r.Value)}
	for _, d := range r.Details {
		out = append(out, plain(d))
	}
	return out
}

func TestBuildVaerRows(t *testing.T) {
	v := BuildVaer(smarthouse.Readings{}, forecast(60), nil, nil, oslo(27, 14, 10))
	want := [][]string{
		{"Temperatur", "2–8°", "Varmest nå", "Kaldest kl. 13"},
		{"Nedbør", "3,8 mm", "Regn fra kl. 00", "Mest 2,6 mm kl. 01"},
		{"Vind", "3 m/s", "Kast 6 m/s", "Fra sørvest", "Mest 8 m/s kl. 01"},
	}
	if len(v.Rows) != 3 {
		t.Fatalf("rows %+v", v.Rows)
	}
	for i, w := range want {
		if got := rowText(v.Rows[i]); !slices.Equal(got, w) {
			t.Errorf("row %d:\n got %q\nwant %q", i, got, w)
		}
	}
	// 48 hours, the shades from the wettest hour, and midnight as the
	// weekday with a line where the day starts.
	rain := v.Rows[1].Chart
	var classes []string
	for _, b := range rain.Bars {
		classes = append(classes, b.Class)
	}
	if !slices.Equal(classes, []string{"rain-1", "rain-3", "rain-2"}) {
		t.Errorf("rain bars %q", classes)
	}
	if got := tickTexts(rain.Ticks); !slices.Equal(got, []string{"18", "ons", "06", "12", "18", "tor", "06", "12"}) {
		t.Errorf("ticks %q", got)
	}
	var midnights int
	for _, l := range rain.Lines {
		if l.X1 == l.X2 {
			midnights++
		}
	}
	if midnights != 2 {
		t.Errorf("got %d day lines, want 2", midnights)
	}
	if c := v.Rows[0].Chart; len(c.Curves) != 1 || c.Curves[0].Kind != "temp" {
		t.Errorf("temperature curve %+v", c.Curves)
	}
	if c := v.Rows[2].Chart; len(c.Bars) != 48 || len(c.Curves) != 1 || c.Curves[0].Kind != "gust" {
		t.Errorf("wind: %d bars, curves %+v", len(c.Bars), c.Curves)
	}
}

func TestBuildVaerDry(t *testing.T) {
	hours := forecast(30)
	for i := range hours {
		hours[i].PrecipitationAmount = 0
	}
	r := BuildVaer(smarthouse.Readings{}, hours, nil, nil, oslo(27, 14, 10)).Rows[1]
	if got := rowText(r); !slices.Equal(got, []string{"Nedbør", "Opphold"}) || len(r.Chart.Bars) != 0 {
		t.Errorf("got %q, %d bars", got, len(r.Chart.Bars))
	}
}

// Without a forecast there are no rows, and without sensors no sensor line.
func TestBuildVaerEmpty(t *testing.T) {
	v := BuildVaer(smarthouse.Readings{}, nil, nil, nil, oslo(27, 14, 10))
	if v.Rows != nil || v.Sensor != "" || v.Now.Temperature != "" {
		t.Errorf("got %+v", v)
	}
	if len(v.Days) != vaerDays || v.Days[0].Label != "Onsdag" || v.Days[0].Oslo != (DayWeather{}) {
		t.Errorf("days %+v", v.Days)
	}
	if v.Days[6].Label != "Tirsdag 3." {
		t.Errorf("a week on the label has the date: %q", v.Days[6].Label)
	}
}

func TestBuildVaerSensorAndDays(t *testing.T) {
	r := smarthouse.Readings{HumOut: f(81.2), Pressure: f(1013.4)}
	daily := map[string]weather.Daily{"2026-10-28": {MinTemp: 4, MaxTemp: 9, LightRainProbability: 60}}
	hytta := map[string]weather.Daily{"2026-10-28": {MinTemp: 5, MaxTemp: 9, LightRainProbability: 20}}
	v := BuildVaer(r, nil, daily, hytta, oslo(27, 14, 10))
	if got := plain(v.Sensor); got != "Fuktighet 81 % · lufttrykk 1013 hPa" {
		t.Errorf("sensor %q", got)
	}
	d := v.Days[0]
	if d.Oslo.Temp != "4–9°" || plain(d.Oslo.Rain) != "60 %" || !d.Oslo.Wet || d.Hytta.Wet || plain(d.Hytta.Rain) != "20 %" {
		t.Errorf("day %+v", d)
	}
}

func TestSunRow(t *testing.T) {
	got := rowText(sunRow(oslo(27, 14, 10)))
	if len(got) != 4 || got[0] != "Sol" || got[3] != "5 min kortere enn i går" {
		t.Errorf("got %q", got)
	}
}

func TestCompass(t *testing.T) {
	cases := map[float64]string{0: "nord", 22: "nord", 23: "nordøst", 180: "sør", 225: "sørvest", 350: "nord", 315: "nordvest"}
	for deg, want := range cases {
		if got := compass(deg); got != want {
			t.Errorf("compass(%v) = %q, want %q", deg, got, want)
		}
	}
}

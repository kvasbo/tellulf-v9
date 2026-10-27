package view

import (
	"strconv"
	"strings"
)

// Row is a text block like a bottom-row cell (label, value, details) with
// a chart beside it. A row without data has only its label.
type Row struct {
	Label   string // "I dag", "Oktober", "Nedbør"
	Value   string // "14,2 kWh"
	Details []string
	Chart   *Chart
}

// Chart is a bar chart, maybe with curves, drawn as SVG at a fixed size in
// px at 1080p. The view does all the arithmetic; the template only places
// what is here.
type Chart struct {
	Width, Height int // the whole SVG, axis row included
	Bars          []Bar
	Lines         []Line
	Curves        []Curve
	Ticks         []Tick
}

// Bar is one slot's bar. Class is "now" for the hour or day now (accent),
// "rain-1".."rain-3" for precipitation, or "" (faint).
type Bar struct {
	X, Y, W, H string
	Class      string
}

// Curve is a polyline through the middle of the slots: "temp" (primary) or
// "gust" (muted).
type Curve struct {
	Points string // "x,y x,y …"
	Kind   string
}

// Line is a 1 px line: "ref" is a reference value (muted), "base" the zero
// line or a limit (faint).
type Line struct {
	X1, Y1, X2, Y2 string
	Kind           string
}

// Tick is an axis label under the slot it starts at; Y is its baseline.
type Tick struct {
	X, Y, Text string
}

// Chart sizes at 1080p: the plot, then a row for the axis labels (18 px).
// The power page's charts are 520 px wide, the weather page's 624.
const (
	chartWidth = 520
	wideChart  = 624
	plotHeight = 112
	axisHeight = 32
	tickY      = plotHeight + 26 // the labels' baseline
	barShare   = 0.72            // of a slot; the rest is the gap
)

// slot is one place on the x axis: a value when known, the bar's class,
// and a label when an axis tick starts there.
type slot struct {
	value float64
	known bool
	class string
	tick  string
}

// barChart lays out slots as bars from a zero line, width px wide and
// scaled so lo..hi fills the plot (lo <= 0 <= hi). Unknown slots get no
// bar; a known value above zero gets at least a hairline, so it reads as
// small rather than missing.
func barChart(slots []slot, lo, hi float64, width int) Chart {
	c := axisChart(slots, width)
	if len(slots) == 0 || hi <= lo {
		return c
	}
	step := float64(width) / float64(len(slots))
	y := func(v float64) float64 { return yFor(v, lo, hi) }
	for i, s := range slots {
		if !s.known {
			continue
		}
		top, bottom := y(max(s.value, 0)), y(min(s.value, 0))
		if s.value > 0 && bottom-top < 1 {
			top = bottom - 1
		}
		c.Bars = append(c.Bars, Bar{
			X: Fixed(float64(i)*step, 1), Y: Fixed(top, 1), W: Fixed(step*barShare, 1), H: Fixed(bottom-top, 1), Class: s.class,
		})
	}
	c.Lines = append(c.Lines, hline(y(0), width, "base"))
	return c
}

// axisChart is an empty chart with the slots' axis labels.
func axisChart(slots []slot, width int) Chart {
	c := Chart{Width: width, Height: plotHeight + axisHeight}
	step := float64(width) / float64(max(len(slots), 1))
	for i, s := range slots {
		if s.tick != "" {
			c.Ticks = append(c.Ticks, Tick{X: Fixed(float64(i)*step, 1), Y: Fixed(tickY, 0), Text: s.tick})
		}
	}
	return c
}

// curve runs through the middle of the bars of the known slots, scaled to
// lo..hi; a slot that isn't known breaks nothing, it is just skipped.
func curve(slots []slot, lo, hi float64, width int, kind string) Curve {
	step := float64(width) / float64(len(slots))
	var pts []string
	for i, s := range slots {
		if s.known {
			pts = append(pts, Fixed((float64(i)+barShare/2)*step, 1)+","+Fixed(yFor(s.value, lo, hi), 1))
		}
	}
	return Curve{Points: strings.Join(pts, " "), Kind: kind}
}

// hline is a line across a plot width px wide at y.
func hline(y float64, width int, kind string) Line {
	return Line{X1: "0", Y1: Fixed(y, 1), X2: strconv.Itoa(width), Y2: Fixed(y, 1), Kind: kind}
}

// yFor is where value v sits in a chart scaled to lo..hi.
func yFor(v, lo, hi float64) float64 { return (hi - v) / (hi - lo) * plotHeight }

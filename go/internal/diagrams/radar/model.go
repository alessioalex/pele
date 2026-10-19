package radar

import (
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

// Axis is a spoke of the chart.
type Axis struct {
	Name  string
	Label string
}

// Curve is a series of values, one per axis.
type Curve struct {
	Name    string
	Label   string
	Entries []float64
}

// Options are the chart's own settings. HasMax is false for a max left to the data.
type Options struct {
	ShowLegend bool
	Ticks      float64
	Max        float64
	HasMax     bool
	Min        float64
	Graticule  string
}

// Model is a parsed radar chart.
type Model struct {
	Title    string
	AccTitle string
	AccDescr string
	Axes     []Axis
	Curves   []Curve
	Options  Options
}

const maxTicks = 32

func invalid(message string) *core.Error {
	e := core.NewError(message, core.CodeSemantic)
	e.Type = "radar"
	return e
}

// collapseIndents is .replace(/\n\s+/g, '\n').
func collapseIndents(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\n' {
			j := i + 1
			for j < len(s) {
				r, size := js.RuneAt(s, j)
				if !js.IsSpace(r) {
					break
				}
				j += size
			}
			b.WriteByte('\n')
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func buildRadar(a ast, title string) *Model {
	axes := make([]Axis, len(a.axes))
	for i, axis := range a.axes {
		axes[i] = Axis{Name: axis.name, Label: axis.name}
		if axis.hasLabel {
			axes[i].Label = axis.label
		}
	}

	curves := make([]Curve, len(a.curves))
	for i, curve := range a.curves {
		var entries []float64
		if !curve.entries[0].hasAxis {
			for _, entry := range curve.entries {
				entries = append(entries, entry.value)
			}
		} else {
			if len(axes) == 0 {
				panic(invalid("Axes must be populated before curves for reference entries"))
			}
			byAxis := map[string]float64{}
			for _, entry := range curve.entries {
				if _, ok := byAxis[entry.axis]; entry.hasAxis && !ok {
					byAxis[entry.axis] = entry.value
				}
			}
			entries = make([]float64, len(axes))
			for j, axis := range axes {
				value, ok := byAxis[axis.Name]
				if !ok {
					panic(invalid("Missing entry for axis " + axis.Label))
				}
				entries[j] = value
			}
		}
		curves[i] = Curve{Name: curve.name, Label: curve.name, Entries: entries}
		if curve.hasLabel {
			curves[i].Label = curve.label
		}
	}

	options := Options{ShowLegend: true, Ticks: 5, Min: 0, Graticule: "circle"}
	for _, option := range a.options {
		switch option.name {
		case "showLegend":
			options.ShowLegend = option.boolean
		case "graticule":
			options.Graticule = option.text
		case "ticks":
			options.Ticks = option.number
		case "max":
			options.Max, options.HasMax = option.number, true
		case "min":
			options.Min = option.number
		}
	}
	if options.Ticks > maxTicks {
		options.Ticks = maxTicks
	}

	m := &Model{Title: a.title, Axes: axes, Curves: curves, Options: options}
	if m.Title == "" {
		m.Title = title
	}
	if a.accTitle != "" {
		m.AccTitle = js.TrimStart(a.accTitle)
	}
	if a.accDescr != "" {
		m.AccDescr = collapseIndents(a.accDescr)
	}
	return m
}

// A variable, so that expressions with it are worked out in float64 as in JavaScript.
var pi = math.Pi

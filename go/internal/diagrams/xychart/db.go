package xychart

import (
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
)

type dataPoint struct {
	value float64
	label string
}

// Axis is a band axis ("band", with categories) or a linear one ("linear", from min to max).
type Axis struct {
	Type       string
	Title      string
	Categories []string
	Min, Max   float64
}

// Datum is a value with its place on the x axis: a category, or a number written as text.
// A category with no value has Has false.
type Datum struct {
	Place string
	Value float64
	Has   bool
}

// Plot is a series.
type Plot struct {
	Type  string
	Title string
	Data  []Datum
	// Nil when no point has a label.
	PointLabels []string
}

// Db is the parsed xychart. Orientation is "" while undefined.
type Db struct {
	Title       string
	AccTitle    string
	AccDescr    string
	Orientation string
	XAxis       Axis
	YAxis       Axis
	hasSetXAxis bool
	// False while the y range comes from the data and not from a `y-axis` statement.
	HasSetYAxis bool
	Plots       []Plot
}

func newDb() *Db {
	return &Db{
		XAxis: Axis{Type: "band", Categories: []string{}},
		YAxis: Axis{Type: "linear", Min: math.Inf(1), Max: math.Inf(-1)},
	}
}

func (db *Db) setOrientation(orientation string) {
	if orientation == "horizontal" {
		db.Orientation = "horizontal"
	} else {
		db.Orientation = "vertical"
	}
}

func (db *Db) setXAxisRangeData(min, max float64) {
	db.XAxis = Axis{Type: "linear", Title: db.XAxis.Title, Min: min, Max: max}
	db.hasSetXAxis = true
}

func (db *Db) setXAxisBand(categories []string) {
	trimmed := make([]string, len(categories))
	for k, c := range categories {
		trimmed[k] = js.Trim(c)
	}
	db.XAxis = Axis{Type: "band", Title: db.XAxis.Title, Categories: trimmed}
	db.hasSetXAxis = true
}

func (db *Db) setYAxisRangeData(min, max float64) {
	db.YAxis = Axis{Type: "linear", Title: db.YAxis.Title, Min: min, Max: max}
	db.HasSetYAxis = true
}

func (db *Db) plotData(values []float64) []Datum {
	if len(values) == 0 {
		return []Datum{}
	}
	// The first series fixes the x range when no x-axis statement did.
	if !db.hasSetXAxis {
		db.setXAxisRangeData(1, float64(len(values)))
	}
	x := db.XAxis
	if x.Type == "band" && len(values) > len(x.Categories) {
		values = values[:len(x.Categories)]
	}
	if !db.HasSetYAxis {
		min, max := db.YAxis.Min, db.YAxis.Max
		for _, value := range values {
			if value < min {
				min = value
			}
			if value > max {
				max = value
			}
		}
		db.YAxis = Axis{Type: "linear", Title: db.YAxis.Title, Min: min, Max: max}
	}
	if x.Type == "band" {
		out := make([]Datum, len(x.Categories))
		for i, category := range x.Categories {
			out[i] = Datum{Place: category}
			if i < len(values) {
				out[i].Value, out[i].Has = values[i], true
			}
		}
		return out
	}
	if len(values) == 1 {
		return []Datum{{Place: js.Num(x.Min), Value: values[0], Has: true}}
	}
	step := (x.Max - x.Min) / float64(len(values)-1)
	out := make([]Datum, len(values))
	for i, value := range values {
		out[i] = Datum{Place: js.Num(x.Min + float64(i)*step), Value: value, Has: true}
	}
	return out
}

func (db *Db) addPlot(typ, title string, data []dataPoint) {
	values := make([]float64, len(data))
	labeled := false
	for k, d := range data {
		values[k] = d.value
		if js.Trim(d.label) != "" {
			labeled = true
		}
	}
	plot := Plot{Type: typ, Title: js.Trim(title), Data: db.plotData(values)}
	// Mermaid accepts labels on the points of a bar series and ignores them.
	if typ == "line" && labeled {
		plot.PointLabels = make([]string, len(data))
		for k, d := range data {
			plot.PointLabels[k] = js.Trim(d.label)
		}
	}
	db.Plots = append(db.Plots, plot)
}

func (db *Db) setAccTitle(text string) {
	db.AccTitle = js.TrimStart(text)
}

// setAccDescription is .replace(/\n\s+/g, '\n').
func (db *Db) setAccDescription(text string) {
	var b strings.Builder
	for i := 0; i < len(text); {
		if text[i] == '\n' {
			j := i + 1
			for j < len(text) {
				r, size := js.RuneAt(text, j)
				if !js.IsSpace(r) {
					break
				}
				j += size
			}
			b.WriteByte('\n')
			i = j
			continue
		}
		b.WriteByte(text[i])
		i++
	}
	db.AccDescr = b.String()
}

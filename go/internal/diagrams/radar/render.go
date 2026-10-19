package radar

import (
	"math"
	"strconv"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
	"github.com/alessioalex/pele/go/internal/text"
)

const (
	swatch = 12
	row    = 26
	gap    = 24
)

type point struct{ x, y float64 }

func option(config *core.Object, key string, fallback, least, most float64) float64 {
	if value, ok := config.Section("radar").Value(key).(float64); ok && value >= least {
		return math.Min(value, most)
	}
	return fallback
}

func relativeRadius(value, least, most, radius float64) float64 {
	clipped := math.Min(math.Max(value, least), most)
	return (radius * (clipped - least)) / (most - least)
}

// closedRoundCurve is a closed Catmull-Rom spline through the points, as cubic Bézier segments.
func closedRoundCurve(points []point, tension float64) string {
	n := len(points)
	var d strings.Builder
	d.WriteString("M" + svg.Num(points[0].x) + "," + svg.Num(points[0].y))
	for i := 0; i < n; i++ {
		p0 := points[(i-1+n)%n]
		p1 := points[i]
		p2 := points[(i+1)%n]
		p3 := points[(i+2)%n]
		d.WriteString(" C" + svg.Num(p1.x+(p2.x-p0.x)*tension) + "," + svg.Num(p1.y+(p2.y-p0.y)*tension) +
			" " + svg.Num(p2.x-(p3.x-p1.x)*tension) + "," + svg.Num(p2.y-(p3.y-p1.y)*tension) + " " + svg.Num(p2.x) + "," + svg.Num(p2.y))
	}
	d.WriteString(" Z")
	return d.String()
}

func polygon(points []point) string {
	var out strings.Builder
	for i, p := range points {
		if i > 0 {
			out.WriteByte(' ')
		}
		out.WriteString(svg.Num(p.x) + "," + svg.Num(p.y))
	}
	return out.String()
}

func renderRadar(model *Model, config *core.Object, options *core.Options) core.Rendered {
	natural := option(config, "width", 320, 1, 4000)
	return common.FitWidth(options, natural, 160, func(width float64, tight bool) core.Rendered {
		return draw(model, config, options, width, (option(config, "height", 320, 1, 4000)*width)/natural, tight)
	})
}

// tight is set when the chart does not fit the width it has with its legend beside it.
func draw(model *Model, config *core.Object, options *core.Options, width, height float64, tight bool) core.Rendered {
	size := options.FontSizeOr(16)
	small := js.Round(size * 0.875)
	measurer := options.Measurer
	pad := options.PaddingOr(8)

	axisScale := option(config, "axisScaleFactor", 1, 0, 10)
	labelFactor := option(config, "axisLabelFactor", 1.05, 0, 10)
	tension := option(config, "curveTension", 0.17, 0, 1)
	radius := math.Min(width, height) / 2

	axes, curves := model.Axes, model.Curves
	ticks, graticule, least := model.Options.Ticks, model.Options.Graticule, model.Options.Min
	count := len(axes)
	unit := make([]point, count)
	for i := range axes {
		angle := (2*pi*float64(i))/float64(count) - pi/2
		unit[i] = point{js.Cos(angle), js.Sin(angle)}
	}

	most := model.Options.Max
	if !model.Options.HasMax {
		most = math.Inf(-1)
		for _, curve := range curves {
			for _, value := range curve.Entries {
				if value > most {
					most = value
				}
			}
		}
	}

	// The chart is drawn around the origin. Margins reserve space; labels and the legend widen it.
	x0 := -width/2 - option(config, "marginLeft", 16, 0, 2000)
	x1 := width/2 + option(config, "marginRight", 16, 0, 2000)
	y0 := -height/2 - option(config, "marginTop", 16, 0, 2000)
	y1 := height/2 + option(config, "marginBottom", 16, 0, 2000)

	var grid strings.Builder
	for i := 0.0; i < ticks; i++ {
		r := math.Min(radius, (radius*(i+1))/ticks)
		if graticule == "circle" {
			grid.WriteString(`<circle r="` + svg.Num(r) + `"/>`)
		} else {
			scaled := make([]point, len(unit))
			for k, u := range unit {
				scaled[k] = point{u.x * r, u.y * r}
			}
			grid.WriteString(`<polygon points="` + polygon(scaled) + `"/>`)
		}
	}

	var spokes, labels strings.Builder
	for i, u := range unit {
		reach := radius * axisScale
		spokes.WriteString("M0,0L" + svg.Num(u.x*reach) + "," + svg.Num(u.y*reach))
		label := text.Layout(axes[i].Label, false, measurer, small, 4000, 0)
		at := radius*labelFactor + 6
		// Each label hangs off its axis on the side away from the center.
		cx := u.x * at
		if u.x > 0.01 {
			cx += label.Width / 2
		} else if u.x < -0.01 {
			cx += -label.Width / 2
		}
		cy := u.y * at
		if u.y > 0.01 {
			cy += label.Height / 2
		} else if u.y < -0.01 {
			cy += -label.Height / 2
		}
		labels.WriteString(`<g class="pele-axis-label" data-id="` + svg.EscText(axes[i].Name) + `">` + svg.LabelSvg(label, cx, cy, "", nil) + "</g>")
		x0 = math.Min(x0, cx-label.Width/2)
		x1 = math.Max(x1, cx+label.Width/2)
		y0 = math.Min(y0, cy-label.Height/2)
		y1 = math.Max(y1, cy+label.Height/2)
	}

	var shapes strings.Builder
	for index, curve := range curves {
		// Mermaid leaves out a curve that does not have a value for every axis.
		if len(curve.Entries) != count {
			continue
		}
		points := make([]point, len(curve.Entries))
		for i, value := range curve.Entries {
			r := relativeRadius(value, least, most, radius)
			safe := 0.0
			if js.IsFinite(r) {
				safe = r
			}
			points[i] = point{unit[i].x * safe, unit[i].y * safe}
		}
		color := svg.SeriesColor(index)
		attrs := ` class="pele-curve" data-id="` + svg.EscText(curve.Name) + `" fill="` + color + `" stroke="` + color + `"`
		if graticule == "circle" {
			shapes.WriteString("<path" + attrs + ` d="` + closedRoundCurve(points, tension) + `"/>`)
		} else {
			shapes.WriteString("<polygon" + attrs + ` points="` + polygon(points) + `"/>`)
		}
	}

	var legend []text.Label
	if model.Options.ShowLegend {
		for _, curve := range curves {
			legend = append(legend, text.Layout(curve.Label, false, measurer, small, 4000, 0))
		}
	}
	legendWidth := 0.0
	for _, label := range legend {
		legendWidth = math.Max(legendWidth, label.Width)
	}
	legendHeight := float64(len(legend) * row)
	legendX := x1 + gap
	legendTop := -legendHeight / 2
	if tight {
		legendX = -(swatch + 8 + legendWidth) / 2
		legendTop = y1 + gap/2
	}
	if len(legend) > 0 && tight {
		x0 = math.Min(x0, legendX)
		x1 = math.Max(x1, -legendX)
		y1 = legendTop + legendHeight
	} else if len(legend) > 0 {
		x1 = legendX + swatch + 8 + legendWidth
		y0 = math.Min(y0, -legendHeight/2)
		y1 = math.Max(y1, legendHeight/2)
	}
	var legendOut strings.Builder
	for index, label := range legend {
		y := legendTop + float64(index*row) + row/2
		color := svg.SeriesColor(index)
		legendOut.WriteString(`<g class="pele-legend-item" data-id="` + svg.EscText(curves[index].Name) + `">` +
			`<rect x="` + svg.Num(legendX) + `" y="` + svg.Num(y-swatch/2) + `" width="` + strconv.Itoa(swatch) + `" height="` + strconv.Itoa(swatch) +
			`" rx="2" fill="` + color + `" fill-opacity="0.4" stroke="` + color + `"/>` +
			svg.LabelSvg(label, legendX+swatch+8+label.Width/2, y, "", nil) +
			"</g>")
	}

	title := text.Layout(model.Title, false, measurer, size, common.TitleRoom(options), text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 8
	}
	bodyWidth := math.Max(x1-x0, title.Width)
	totalWidth := math.Ceil(bodyWidth + 2*pad)
	totalHeight := math.Ceil(y1 - y0 + titleHeight + 2*pad)
	origin := "translate(" + svg.Num(pad+(bodyWidth-(x1-x0))/2-x0) + "," + svg.Num(pad+titleHeight-y0) + ")"

	var content strings.Builder
	content.WriteString(svg.LabelSvg(title, totalWidth/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil))
	content.WriteString(`<g transform="` + origin + `">`)
	if grid.Len() > 0 {
		content.WriteString(`<g class="pele-graticule" fill="none" stroke="var(--_a)">` + grid.String() + "</g>")
	}
	if spokes.Len() > 0 {
		content.WriteString(`<path class="pele-axes" d="` + spokes.String() + `" fill="none" stroke="var(--_b)"/>`)
	}
	if shapes.Len() > 0 {
		content.WriteString(`<g class="pele-curves" fill-opacity="0.2" stroke-width="1.5" stroke-linejoin="round">` + shapes.String() + "</g>")
	}
	if labels.Len() > 0 {
		content.WriteString(`<g class="pele-axis-labels" font-size="` + js.NumberToString(small) + `">` + labels.String() + "</g>")
	}
	if legendOut.Len() > 0 {
		content.WriteString(`<g class="pele-legend" font-size="` + js.NumberToString(small) + `">` + legendOut.String() + "</g>")
	}
	content.WriteString("</g>")

	acc := svg.Accessible{AccTitle: model.AccTitle, AccDescr: model.AccDescr}
	return core.Rendered{
		SVG:    svg.Document("radar", totalWidth, totalHeight, size, options, acc, content.String()),
		Width:  totalWidth,
		Height: totalHeight,
	}
}

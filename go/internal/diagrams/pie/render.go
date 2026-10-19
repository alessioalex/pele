package pie

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
	radiusSize = 120
	swatch     = 12
	row        = 26
	gap        = 32
	outsideGap = 28
)

func option(config *core.Object, key string) (core.Value, bool) {
	return config.Section("pie").Get(key)
}

// A variable, so that expressions with it are worked out in float64 as in JavaScript.
var pi = math.Pi

func point(angle, radius float64) string {
	return svg.Num(js.Sin(angle)*radius) + "," + svg.Num(-js.Cos(angle)*radius)
}

func slice(start, end, outer, inner float64) string {
	if end-start >= pi*2-1e-9 {
		ring := func(r float64, sweep string) string {
			return "M0," + svg.Num(-r) + "A" + svg.Num(r) + "," + svg.Num(r) + " 0 1 " + sweep + " 0," + svg.Num(r) +
				"A" + svg.Num(r) + "," + svg.Num(r) + " 0 1 " + sweep + " 0," + svg.Num(-r) + "Z"
		}
		out := ring(outer, "1")
		if inner > 0 {
			out += ring(inner, "0")
		}
		return out
	}
	large := "0"
	if end-start > pi {
		large = "1"
	}
	if inner <= 0 {
		return "M0,0L" + point(start, outer) + "A" + svg.Num(outer) + "," + svg.Num(outer) + " 0 " + large + " 1 " + point(end, outer) + "Z"
	}
	return "M" + point(start, outer) + "A" + svg.Num(outer) + "," + svg.Num(outer) + " 0 " + large + " 1 " + point(end, outer) +
		"L" + point(end, inner) + "A" + svg.Num(inner) + "," + svg.Num(inner) + " 0 " + large + " 0 " + point(start, inner) + "Z"
}

func renderPie(model *Model, config *core.Object, options *core.Options) core.Rendered {
	return common.FitWidth(options, radiusSize*2, 140, func(diameter float64, tight bool) core.Rendered {
		return draw(model, config, options, diameter/2, tight)
	})
}

type placedLabel struct{ x, y, width float64 }

// tight is set when the chart does not fit the width it has with its legend beside it.
func draw(model *Model, config *core.Object, options *core.Options, radius float64, tight bool) core.Rendered {
	size := options.FontSizeOr(16)
	small := js.Round(size * 0.875)
	measurer := options.Measurer
	pad := options.PaddingOr(8)

	textPosition := 0.75
	if v, ok := option(config, "textPosition"); ok {
		if p, isNum := v.(float64); isNum && p >= 0 && p <= 1 {
			textPosition = p
		}
	}
	inner := 0.0
	if v, ok := option(config, "donutHole"); ok {
		if h, isNum := v.(float64); isNum && h > 0 && h <= 0.9 {
			inner = h * radius
		}
	}
	placed := "right"
	if v, _ := option(config, "legendPosition"); v != nil {
		placed = core.ToString(v)
	}
	legendPosition := placed
	if tight && (placed == "left" || placed == "right") {
		legendPosition = "bottom"
	}

	entries := model.Labels
	sum := 0.0
	for _, label := range entries {
		sum += model.Values[label]
	}

	legend := make([]text.Label, len(entries))
	for i, label := range entries {
		t := label
		if model.ShowData {
			t = label + " [" + js.NumberToString(model.Values[label]) + "]"
		}
		legend[i] = text.Layout(t, false, measurer, small, 4000, 0)
	}
	legendWidth := 0.0
	for _, label := range legend {
		legendWidth = max(legendWidth, label.Width)
	}
	if len(legend) > 0 {
		legendWidth += swatch + 8
	}
	legendHeight := float64(len(legend) * row)

	title := text.Layout(model.Title, false, measurer, size, common.TitleRoom(options), text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 16
	}

	// A slice too thin for its percentage gets the label outside the rim, which needs a margin.
	shares := make([]float64, len(entries))
	for i, label := range entries {
		share := 0.0
		if sum > 0 {
			share = model.Values[label] / sum
		}
		if share*100 >= 1 && js.ToFixed(share*100, 0) != "0" {
			shares[i] = share
		}
	}
	narrow := make([]bool, len(entries))
	anyNarrow := false
	for i, share := range shares {
		if share == 0 || share >= 0.5 {
			continue
		}
		chord := 2 * radius * textPosition * js.Sin(share*pi)
		narrow[i] = chord < measurer.Width(js.ToFixed(share*100, 0)+"%", small, 0)+6
		anyNarrow = anyNarrow || narrow[i]
	}
	margin := 0.0
	if anyNarrow {
		margin = outsideGap
	}

	side := ""
	if legendPosition == "left" || legendPosition == "right" || legendPosition == "center" {
		side = legendPosition
	}
	stacked := legendPosition == "top" || legendPosition == "bottom"
	diameter := (radius + margin) * 2
	width := diameter
	height := diameter
	if side == "left" || side == "right" {
		if legendWidth > 0 {
			width += gap + legendWidth
		}
		height = max(height, legendHeight)
	} else if stacked {
		width = max(width, legendWidth)
		if legendHeight > 0 {
			height += 20 + legendHeight
		}
	}
	width = max(width, title.Width)

	cx := pad
	switch side {
	case "left":
		cx += legendWidth + gap + diameter/2
	case "right":
		cx += diameter / 2
	default:
		cx += width / 2
	}
	bodyTop := pad + titleHeight
	cy := bodyTop
	switch {
	case legendPosition == "top":
		cy += legendHeight + 20 + diameter/2
	case stacked:
		cy += diameter / 2
	default:
		cy += height / 2
	}

	var slices, labels, outside strings.Builder
	angle := 0.0
	rim := radius + outsideGap/2
	var placedOutside []placedLabel
	clear := func(x, y, width float64) bool {
		for _, other := range placedOutside {
			if !(math.Abs(x-other.x) >= (width+other.width)/2+2 || math.Abs(y-other.y) >= small+2) {
				return false
			}
		}
		return true
	}
	for index, label := range entries {
		share := shares[index]
		if share == 0 {
			continue
		}
		end := angle + share*pi*2
		slices.WriteString(`<path class="pele-slice" data-id="` + svg.EscText(label) + `" d="` + slice(angle, end, radius, inner) +
			`" fill="` + svg.SeriesColor(index) + `"/>`)
		mid := (angle + end) / 2
		t := js.ToFixed(share*100, 0) + "%"
		if narrow[index] {
			width := measurer.Width(t, small, 0)
			at := mid
			for step := 0; step < 360 && !clear(js.Sin(at)*rim, -js.Cos(at)*rim, width); step++ {
				at += 2 / rim
			}
			x := js.Sin(at) * rim
			y := -js.Cos(at) * rim
			placedOutside = append(placedOutside, placedLabel{x, y, width})
			outside.WriteString(`<text x="` + svg.Num(x) + `" y="` + svg.Num(y+small*0.35) + `">` + t + "</text>")
		} else {
			r := radius * textPosition
			labels.WriteString(`<text x="` + svg.Num(js.Sin(mid)*r) + `" y="` + svg.Num(-js.Cos(mid)*r+small*0.35) + `">` + t + "</text>")
		}
		angle = end
	}
	slicesOut := slices.String()
	if slicesOut == "" {
		slicesOut = `<circle class="pele-slice" r="` + js.NumberToString(radius) + `" fill="none" stroke="var(--_b)"/>`
	}

	var legendOut strings.Builder
	var lx, ly float64
	switch side {
	case "right":
		lx = cx + diameter/2 + gap
		ly = bodyTop + (height-legendHeight)/2
	case "left":
		lx = pad
		ly = bodyTop + (height-legendHeight)/2
	case "center":
		lx = cx - legendWidth/2
		ly = cy - legendHeight/2
	default:
		lx = pad + (width-legendWidth)/2
		if legendPosition == "top" {
			ly = bodyTop
		} else {
			ly = bodyTop + diameter + 20
		}
	}
	for index, label := range legend {
		y := ly + float64(index*row) + row/2
		legendOut.WriteString(`<g class="pele-legend-item" data-id="` + svg.EscText(entries[index]) + `">` +
			`<rect x="` + svg.Num(lx) + `" y="` + svg.Num(y-swatch/2) + `" width="` + strconv.Itoa(swatch) + `" height="` + strconv.Itoa(swatch) +
			`" rx="2" fill="` + svg.SeriesColor(index) + `"/>` +
			svg.LabelSvg(label, lx+swatch+8+label.Width/2, y, "", nil) +
			"</g>")
	}

	totalWidth := math.Ceil(width + 2*pad)
	totalHeight := math.Ceil(height + titleHeight + 2*pad)
	translate := `transform="translate(` + svg.Num(cx) + "," + svg.Num(cy) + `)"`
	var content strings.Builder
	content.WriteString(svg.LabelSvg(title, totalWidth/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil))
	content.WriteString(`<g class="pele-slices" ` + translate + ` stroke="var(--_bg)" stroke-width="1" stroke-linejoin="round">` + slicesOut + "</g>")
	if labels.Len() > 0 {
		content.WriteString(`<g class="pele-slice-labels" ` + translate + ` font-size="` + js.NumberToString(small) +
			`" text-anchor="middle" fill="var(--_bg)">` + labels.String() + "</g>")
	}
	if outside.Len() > 0 {
		content.WriteString(`<g class="pele-slice-labels" ` + translate + ` font-size="` + js.NumberToString(small) +
			`" text-anchor="middle">` + outside.String() + "</g>")
	}
	if legendOut.Len() > 0 {
		content.WriteString(`<g class="pele-legend" font-size="` + js.NumberToString(small) + `">` + legendOut.String() + "</g>")
	}

	acc := svg.Accessible{AccTitle: model.AccTitle, AccDescr: model.AccDescr}
	return core.Rendered{
		SVG:    svg.Document("pie", totalWidth, totalHeight, size, options, acc, content.String()),
		Width:  totalWidth,
		Height: totalHeight,
	}
}

package xychart

import (
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
	"github.com/alessioalex/pele/go/internal/text"
)

const (
	tick      = 4
	swatch    = 12
	rowHeight = 26
	legendGap = 24
	limit     = 1e300
)

type mark struct {
	at    float64
	label text.Label
}

func on(config *core.Object, key string, fallback bool) bool {
	if value, ok := config.Value(key).(bool); ok {
		return value
	}
	return fallback
}

func dimension(value core.Value, fallback float64) float64 {
	if v, ok := value.(float64); ok && v > 0 {
		return js.Round(math.Min(math.Max(v, 100), 4000))
	}
	return fallback
}

// finite clamps a value into ±limit; ok is false for a missing or non-finite one.
func finite(value float64, has bool) (float64, bool) {
	if has && js.IsFinite(value) {
		return math.Min(math.Max(value, -limit), limit), true
	}
	return 0, false
}

func widest(marks []mark) float64 {
	width := 0.0
	for _, m := range marks {
		width = math.Max(width, m.label.Width)
	}
	return width
}

// aligned writes a label with its left edge, center or right edge at x. A plain one-line label
// keeps its place when the text is wider or narrower than measured.
func aligned(label text.Label, x, y float64, anchor string) string {
	if len(label.Lines) == 1 {
		line := label.Lines[0]
		if len(line) == 1 && line[0].Icon == "" && line[0].Style == 0 {
			return `<text x="` + svg.Num(x) + `" y="` + svg.Num(y+label.Size*0.35) + `" text-anchor="` + anchor + `">` + svg.Esc(line[0].Text) + "</text>"
		}
	}
	shift := 0.0
	if anchor == "start" {
		shift = label.Width / 2
	} else if anchor == "end" {
		shift = -label.Width / 2
	}
	return svg.LabelSvg(label, x+shift, y, "", nil)
}

func render(model *Db, config *core.Object, options *core.Options) core.Rendered {
	if len(model.Plots) == 0 {
		panic(&core.Error{Message: "No Plot to render, please provide a plot with some data", Code: core.CodeSemantic, Type: "xychart"})
	}
	chart := config.Section("xyChart")
	size := options.FontSizeOr(16)
	small := js.Round(size * 0.875)
	tiny := js.Round(size * 0.75)
	measurer := options.Measurer
	pad := options.PaddingOr(8)
	natural := dimension(chart.Value("width"), 700)
	width := common.CapWidth(options, natural, 240, 0)
	// A narrower chart is not as much shorter, so that the plot keeps room for its marks.
	height := js.Round(dimension(chart.Value("height"), 500) * math.Max(0.6, width/natural))
	orientation := model.Orientation
	if orientation == "" {
		orientation, _ = chart.Value("chartOrientation").(string)
	}
	horizontal := orientation == "horizontal"
	// The value of a bar is always written past its end, which is what showDataLabelOutsideBar asks
	// for in Mermaid. Inside, whether it fits changes from bar to bar and with the width of the chart.
	showValues := on(chart, "showDataLabel", false)
	x := model.XAxis
	plots := model.Plots
	label := func(raw string, fontSize, max float64) text.Label {
		return text.Layout(raw, false, measurer, fontSize, max, 0)
	}
	lineHeight := label("0", small, 4000).Height
	tinyHeight := label("0", tiny, 4000).Height

	titleText := ""
	if on(chart, "showTitle", true) {
		titleText = model.Title
	}
	title := text.Layout(titleText, false, measurer, size, width-2*pad, text.Bold)
	titleSpace := 0.0
	if title.Height > 0 {
		titleSpace = title.Height + 12
	}

	type legendItem struct {
		index int
		label text.Label
	}
	var legend []legendItem
	legendWidth := 0.0
	if on(chart, "showLegend", true) {
		for index, plot := range plots {
			if plot.Title == "" {
				continue
			}
			l := label(plot.Title, small, width*0.3)
			legend = append(legend, legendItem{index, l})
			legendWidth = math.Max(legendWidth, l.Width+swatch+8)
		}
	}
	legendSpace := 0.0
	if len(legend) > 0 {
		legendSpace = legendWidth + legendGap
	}

	// The value range: as written, or taken from the data, where bars stand on zero and the
	// range grows to whole steps.
	bars := 0
	for _, plot := range plots {
		if plot.Type == "bar" {
			bars++
		}
	}
	lo := model.YAxis.Min
	hi := model.YAxis.Max
	exact := model.HasSetYAxis && js.IsFinite(lo) && js.IsFinite(hi) && lo != hi
	if exact {
		lo, _ = finite(lo, true)
		hi, _ = finite(hi, true)
	} else {
		lo = math.Inf(1)
		hi = math.Inf(-1)
		for _, plot := range plots {
			for _, d := range plot.Data {
				value, ok := finite(d.Value, d.Has)
				if !ok {
					continue
				}
				if value < lo {
					lo = value
				}
				if value > hi {
					hi = value
				}
			}
		}
		if lo > hi {
			lo, hi = 0, 0
		}
		if bars > 0 {
			lo = math.Min(lo, 0)
			hi = math.Max(hi, 0)
		}
		if lo == hi {
			if lo == 0 {
				hi = 1
			} else {
				lo -= math.Abs(lo) / 2
				hi += math.Abs(hi) / 2
			}
		}
	}

	valueMarks := func(length, spacing float64) []mark {
		step := tickStep(math.Abs(hi-lo), math.Max(2, math.Min(10, math.Floor(length/spacing))))
		if !exact {
			lo = tickValue(step, math.Floor(tickIndex(step, lo)+1e-9))
			hi = tickValue(step, math.Ceil(tickIndex(step, hi)-1e-9))
		}
		var marks []mark
		for _, value := range ticksBetween(math.Min(lo, hi), math.Max(lo, hi), step) {
			marks = append(marks, mark{at: (value - lo) / (hi - lo), label: label(js.Num(value), small, 4000)})
		}
		return marks
	}

	// Places along the category axis are fractions of its length. Categories and bars sit in the
	// middle of equal slots; the points of a line on a numeric axis run from end to end.
	band := x.Type == "band"
	count := 1
	if band {
		count = max(len(x.Categories), 1)
	} else {
		for _, plot := range plots {
			count = max(count, len(plot.Data))
		}
	}
	slot := 1 / float64(count)
	inset := 0.0
	if band || bars > 0 {
		inset = slot / 2
	}
	x0, has0 := finite(x.Min, !band)
	x1, has1 := finite(x.Max, !band)
	ranged := has0 && has1 && x0 != x1
	place := func(i, n int) float64 {
		if band {
			return (float64(i) + 0.5) * slot
		}
		f := 0.5
		if n > 1 {
			f = float64(i) / float64(n-1)
		} else if ranged {
			f = 0
		}
		return inset + f*(1-2*inset)
	}

	categoryConf := chart.Section("xAxis")
	valueConf := chart.Section("yAxis")
	leftConf, bottomConf := valueConf, categoryConf
	if horizontal {
		leftConf, bottomConf = categoryConf, valueConf
	}
	leftTitleText, bottomTitleText := "", ""
	if on(leftConf, "showTitle", true) {
		if horizontal {
			leftTitleText = x.Title
		} else {
			leftTitleText = model.YAxis.Title
		}
	}
	if on(bottomConf, "showTitle", true) {
		if horizontal {
			bottomTitleText = model.YAxis.Title
		} else {
			bottomTitleText = x.Title
		}
	}
	leftTitle := label(leftTitleText, small, height*0.8)
	bottomTitle := label(bottomTitleText, small, width*0.8)
	leftTick := 0.0
	if on(leftConf, "showTick", true) {
		leftTick = tick
	}
	bottomTick := 0.0
	if on(bottomConf, "showTick", true) {
		bottomTick = tick
	}
	leftLabels := on(leftConf, "showLabel", true)
	bottomLabels := on(bottomConf, "showLabel", true)

	wrapAt := width * 0.3
	if !horizontal {
		wrapAt = math.Max(48, (width-2*pad-64-legendSpace)*slot-8)
	}
	var categories []text.Label
	if band && on(categoryConf, "showLabel", true) {
		categories = make([]text.Label, len(x.Categories))
		for k, category := range x.Categories {
			categories[k] = label(category, small, wrapAt)
		}
	}
	categoryWidth := 0.0
	categoryHeight := 0.0
	for _, l := range categories {
		categoryWidth = math.Max(categoryWidth, l.Width)
		categoryHeight = math.Max(categoryHeight, l.Height)
	}

	// The labelled places of the category axis. Labels that would run into each other are thinned out.
	categoryMarks := func(length float64) []mark {
		var marks []mark
		if band {
			need := categoryWidth + 12
			if len(categories) == 0 {
				need = 8
			} else if horizontal {
				need = categoryHeight
			}
			every := math.Max(1, math.Ceil(need/(length*slot)))
			for i := 0.0; i < float64(len(x.Categories)); i += every {
				var l text.Label
				if int(i) < len(categories) {
					l = categories[int(i)]
				}
				marks = append(marks, mark{at: (i + 0.5) * slot, label: l})
			}
		} else if ranged {
			spacing := 80.0
			if horizontal {
				spacing = 32
			}
			step := tickStep(math.Abs(x1-x0), math.Max(2, math.Min(10, math.Floor(length/spacing))))
			for _, value := range ticksBetween(math.Min(x0, x1), math.Max(x0, x1), step) {
				marks = append(marks, mark{at: inset + ((value-x0)/(x1-x0))*(1-2*inset), label: label(js.Num(value), small, 4000)})
			}
		} else if has0 {
			marks = append(marks, mark{at: 0.5, label: label(js.Num(x0), small, 4000)})
		}
		return marks
	}

	// Room past the end of the value axis for what is written beside a point or a bar.
	pointLabelWidth := 0.0
	valueWidth := 0.0
	for _, plot := range plots {
		for _, l := range plot.PointLabels {
			pointLabelWidth = math.Max(pointLabelWidth, measurer.Width(l, tiny, 0))
		}
		if showValues && horizontal && plot.Type == "bar" {
			for _, d := range plot.Data {
				s := ""
				if value, ok := finite(d.Value, d.Has); ok {
					s = js.Num(value)
				}
				valueWidth = math.Max(valueWidth, measurer.Width(s, tiny, 0))
			}
		}
	}

	bottomLabelHeight := 0.0
	if bottomLabels {
		if horizontal || !band {
			bottomLabelHeight = lineHeight
		} else {
			bottomLabelHeight = categoryHeight
		}
	}
	bottomSpace := bottomTick
	if bottomLabelHeight > 0 {
		bottomSpace += bottomLabelHeight + 4
	}
	if bottomTitle.Height > 0 {
		bottomSpace += bottomTitle.Height + 4
	}
	headroom := 0.0
	if !horizontal {
		if pointLabelWidth > 0 || showValues {
			headroom = tinyHeight + 4
		} else if leftLabels {
			headroom = lineHeight / 2
		}
	}
	top := pad + titleSpace + headroom
	plotHeight := math.Max(1, height-pad-bottomSpace-top)
	bottom := top + plotHeight

	var leftMarks []mark
	if horizontal {
		leftMarks = categoryMarks(plotHeight)
	} else {
		leftMarks = valueMarks(plotHeight, 56)
	}
	leftLabelWidth := 0.0
	if leftLabels {
		leftLabelWidth = widest(leftMarks)
	}
	left := pad
	if leftTitle.Height > 0 {
		left += leftTitle.Height + 6
	}
	if leftLabelWidth > 0 {
		left += leftLabelWidth + 6
	}
	left += leftTick
	beside := 0.0
	if pointLabelWidth > 0 {
		beside = pointLabelWidth + 12
	}
	if valueWidth > 0 {
		beside = math.Max(beside, valueWidth+8)
	}
	reach := 0.0
	if horizontal {
		reach = math.Min(width*0.25, beside)
	}
	plotWidth := math.Max(1, width-pad-left-legendSpace-reach)
	var bottomMarks []mark
	if horizontal {
		bottomMarks = valueMarks(plotWidth, 80)
	} else {
		bottomMarks = categoryMarks(plotWidth)
	}
	if bottomLabels && legendSpace+reach == 0 {
		over := 0.0
		for _, m := range bottomMarks {
			over = math.Max(over, m.at*plotWidth+m.label.Width/2-plotWidth)
		}
		plotWidth = math.Max(1, plotWidth-over)
	}
	right := left + plotWidth

	px := func(c, v float64) float64 {
		if horizontal {
			return left + v*plotWidth
		}
		return left + c*plotWidth
	}
	py := func(c, v float64) float64 {
		if horizontal {
			return top + c*plotHeight
		}
		return bottom - v*plotHeight
	}
	scale := func(value float64) float64 { return (value - lo) / (hi - lo) }
	clamp := func(v float64) float64 { return math.Min(math.Max(v, 0), 1) }

	// A line across the plot at a place on the value axis.
	rule := func(v float64) string {
		if horizontal {
			return "M" + svg.Num(px(0, v)) + "," + svg.Num(top) + "V" + svg.Num(bottom)
		}
		return "M" + svg.Num(left) + "," + svg.Num(py(0, v)) + "H" + svg.Num(right)
	}

	var grid strings.Builder
	gridMarks := leftMarks
	if horizontal {
		gridMarks = bottomMarks
	}
	for _, m := range gridMarks {
		grid.WriteString(rule(m.at))
	}

	var axis, labels strings.Builder
	if on(leftConf, "showAxisLine", true) {
		axis.WriteString("M" + svg.Num(left) + "," + svg.Num(top) + "V" + svg.Num(bottom))
	}
	if on(bottomConf, "showAxisLine", true) {
		axis.WriteString("M" + svg.Num(left) + "," + svg.Num(bottom) + "H" + svg.Num(right))
	}
	for _, m := range leftMarks {
		y := bottom - m.at*plotHeight
		if horizontal {
			y = top + m.at*plotHeight
		}
		if leftTick != 0 {
			axis.WriteString("M" + svg.Num(left) + "," + svg.Num(y) + "h" + js.Num(-leftTick))
		}
		if leftLabels {
			labels.WriteString(aligned(m.label, left-leftTick-6, y, "end"))
		}
	}
	for _, m := range bottomMarks {
		at := left + m.at*plotWidth
		if bottomTick != 0 {
			axis.WriteString("M" + svg.Num(at) + "," + svg.Num(bottom) + "v" + js.Num(bottomTick))
		}
		if bottomLabels {
			labels.WriteString(aligned(m.label, at, bottom+bottomTick+4+m.label.Height/2, "middle"))
		}
	}

	span := plotWidth * slot
	if horizontal {
		span = plotHeight * slot
	}
	group := span * 0.7
	thickness := group / float64(max(bars, 1))
	gap := 0.0
	if bars > 1 {
		gap = math.Min(2, thickness*0.2)
	}
	base := clamp(scale(0))
	// Bars that go both ways get a line to stand on.
	if bars > 0 && base > 0 && base < 1 {
		axis.WriteString(rule(base))
	}
	// Lines are drawn over the bars wherever they were declared, as a bar would hide part of one
	// under it. A series keeps the color of its place in the source.
	var series, lines, outside, pointLabels strings.Builder
	bar := 0
	for index, plot := range plots {
		color := svg.SeriesColor(index)
		id := ""
		if plot.Title != "" {
			id = ` data-id="` + svg.EscText(plot.Title) + `"`
		}
		// A series declared before the categories can hold more values than there are categories.
		n := len(plot.Data)
		if band {
			n = min(n, count)
		}
		if plot.Type == "bar" {
			offset := (float64(bar)-float64(bars-1)/2)*thickness - (thickness-gap)/2
			bar++
			var rects strings.Builder
			for i := 0; i < n; i++ {
				value, ok := finite(plot.Data[i].Value, plot.Data[i].Has)
				if !ok {
					continue
				}
				c := place(i, n)
				v := clamp(scale(value))
				from := math.Min(base, v)
				to := math.Max(base, v)
				var bx, by, bw, bh float64
				if horizontal {
					bx = px(c, from)
					by = py(c, 0) + offset
					bw = (to - from) * plotWidth
					bh = thickness - gap
				} else {
					bx = px(c, 0) + offset
					by = py(c, to)
					bw = thickness - gap
					bh = (to - from) * plotHeight
				}
				rects.WriteString(`<rect x="` + svg.Num(bx) + `" y="` + svg.Num(by) + `" width="` + svg.Num(bw) + `" height="` + svg.Num(bh) + `"/>`)
				if !showValues {
					continue
				}
				valueText := js.Num(value)
				w := measurer.Width(valueText, tiny, 0)
				// Towards the end of the bar: 1 when it runs up or right from the baseline, -1 the other way.
				out := -1.0
				if v >= base {
					out = 1
				}
				tx := bx + bw/2
				ty := by + bh/2
				anchor := "middle"
				if horizontal {
					if bh < tiny {
						continue
					}
					if out > 0 {
						tx = bx + bw + out*6
						anchor = "start"
					} else {
						tx = bx + out*6
						anchor = "end"
					}
				} else {
					if w+2 > span/float64(bars) {
						continue
					}
					if out > 0 {
						ty = by - out*(tinyHeight/2+2)
					} else {
						ty = by + bh - out*(tinyHeight/2+2)
					}
				}
				outside.WriteString(`<text x="` + svg.Num(tx) + `" y="` + svg.Num(ty+tiny*0.35) + `" text-anchor="` + anchor + `">` + valueText + "</text>")
			}
			series.WriteString(`<g class="pele-series pele-bars"` + id + ` fill="` + color + `">` + rects.String() + "</g>")
			continue
		}

		// A line is cut where it leaves the value range, so it stays inside the plot.
		var d, dots strings.Builder
		pen := false
		pc := 0.0
		pv := math.NaN()
		for i := 0; i < n; i++ {
			value, ok := finite(plot.Data[i].Value, plot.Data[i].Has)
			if !ok {
				pv = math.NaN()
				continue
			}
			c := place(i, n)
			v := scale(value)
			visible := v >= 0 && v <= 1
			if pv != pv {
				if visible {
					d.WriteString("M" + svg.Num(px(c, v)) + "," + svg.Num(py(c, v)))
				}
				pen = visible
			} else {
				dv := v - pv
				t0 := 0.0
				t1 := 1.0
				if dv != 0 {
					a := -pv / dv
					b := (1 - pv) / dv
					t0 = math.Max(t0, math.Min(a, b))
					t1 = math.Min(t1, math.Max(a, b))
				} else if !visible {
					t0 = 2
				}
				if t0 <= t1 {
					c0 := pc + (c-pc)*t0
					v0 := pv + dv*t0
					c1 := pc + (c-pc)*t1
					v1 := pv + dv*t1
					if !pen || t0 > 0 {
						d.WriteString("M" + svg.Num(px(c0, v0)) + "," + svg.Num(py(c0, v0)))
					}
					d.WriteString("L" + svg.Num(px(c1, v1)) + "," + svg.Num(py(c1, v1)))
				}
				pen = visible
			}
			pc = c
			pv = v
			if !visible {
				continue
			}
			cx := px(c, v)
			cy := py(c, v)
			if span >= 12 {
				dots.WriteString(`<circle cx="` + svg.Num(cx) + `" cy="` + svg.Num(cy) + `" r="3"/>`)
			}
			raw := ""
			if i < len(plot.PointLabels) {
				raw = plot.PointLabels[i]
			}
			l := label(raw, tiny, 4000)
			if l.Height > 0 {
				half := l.Width / 2
				if horizontal {
					pointLabels.WriteString(aligned(l, cx+10, cy, "start"))
				} else {
					pointLabels.WriteString(aligned(l, math.Max(math.Min(cx, right-half), left+half), cy-6-l.Height/2, "middle"))
				}
			}
		}
		lines.WriteString(`<g class="pele-series pele-line"` + id + ` fill="` + color + `">`)
		if d.Len() > 0 {
			lines.WriteString(`<path d="` + d.String() + `" fill="none" stroke="` + color + `" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>`)
		}
		lines.WriteString(dots.String() + "</g>")
	}

	titles := ""
	if leftTitle.Height > 0 {
		titles += `<g transform="translate(` + svg.Num(pad+leftTitle.Height/2) + "," + svg.Num(top+plotHeight/2) + `) rotate(-90)">` +
			svg.LabelSvg(leftTitle, 0, 0, "", nil) + "</g>"
	}
	titles += svg.LabelSvg(bottomTitle, left+plotWidth/2, height-pad-bottomTitle.Height/2, "", nil)

	var legendOut strings.Builder
	legendHeight := 0.0
	for _, item := range legend {
		legendHeight += math.Max(rowHeight, item.label.Height+6)
	}
	ly := top + math.Max(0, (plotHeight-legendHeight)/2)
	lx := right + reach + legendGap
	for _, item := range legend {
		row := math.Max(rowHeight, item.label.Height+6)
		if ly+row > height {
			break
		}
		y := ly + row/2
		color := svg.SeriesColor(item.index)
		legendOut.WriteString(`<g class="pele-legend-item" data-id="` + svg.EscText(plots[item.index].Title) + `">`)
		if plots[item.index].Type == "bar" {
			legendOut.WriteString(`<rect x="` + svg.Num(lx) + `" y="` + svg.Num(y-swatch/2) + `" width="` + js.Num(swatch) + `" height="` + js.Num(swatch) + `" rx="2" fill="` + color + `"/>`)
		} else {
			legendOut.WriteString(`<path d="M` + svg.Num(lx) + "," + svg.Num(y) + "h" + js.Num(swatch) + `" stroke="` + color + `" stroke-width="2" stroke-linecap="round"/>`)
		}
		legendOut.WriteString(svg.LabelSvg(item.label, lx+swatch+8+item.label.Width/2, y, "", nil) + "</g>")
		ly += row
	}

	var content strings.Builder
	content.WriteString(svg.LabelSvg(title, width/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil))
	if grid.Len() > 0 {
		content.WriteString(`<path class="pele-grid" d="` + grid.String() + `" stroke="var(--_a)"/>`)
	}
	content.WriteString(`<g class="pele-plot">` + series.String() + lines.String() + "</g>")
	if axis.Len() > 0 {
		content.WriteString(`<path class="pele-axis" d="` + axis.String() + `" fill="none" stroke="var(--_b)"/>`)
	}
	if labels.Len() > 0 {
		content.WriteString(`<g class="pele-axis-labels" font-size="` + js.Num(small) + `" fill="var(--_m)">` + labels.String() + "</g>")
	}
	if titles != "" {
		content.WriteString(`<g class="pele-axis-titles" font-size="` + js.Num(small) + `">` + titles + "</g>")
	}
	if outside.Len() > 0 {
		content.WriteString(`<g class="pele-data-labels" font-size="` + js.Num(tiny) + `">` + outside.String() + "</g>")
	}
	if pointLabels.Len() > 0 {
		content.WriteString(`<g class="pele-point-labels" font-size="` + js.Num(tiny) + `">` + pointLabels.String() + "</g>")
	}
	if legendOut.Len() > 0 {
		content.WriteString(`<g class="pele-legend" font-size="` + js.Num(small) + `">` + legendOut.String() + "</g>")
	}

	acc := svg.Accessible{AccTitle: model.AccTitle, AccDescr: model.AccDescr}
	return core.Rendered{
		SVG:    svg.Document("xychart", width, height, size, options, acc, content.String()),
		Width:  width,
		Height: height,
	}
}

package quadrant

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
	gap     = 4
	careful = 300
)

type dot struct {
	cx, cy, reach float64
	text          text.Label
}

func setting(config *core.Object, key string) core.Value {
	return config.Section("quadrantChart").Value(key)
}

func length(config *core.Object, key string, fallback, max float64) float64 {
	if value, ok := setting(config, key).(float64); ok && value >= 0 {
		return math.Min(value, max)
	}
	return fallback
}

func hex(color string) string {
	if strings.HasPrefix(color, "#") {
		return color
	}
	return "#" + color
}

func render(model *Db, config *core.Object, options *core.Options) core.Rendered {
	size := options.FontSizeOr(16)
	axisSize := js.Round(size * 0.875)
	pointSize := js.Round(size * 0.75)
	measurer := options.Measurer
	natural := js.Round(math.Max(100, length(config, "chartWidth", 500, 4000)))
	width := js.Round(common.CapWidth(options, natural, 280, 0))
	height := js.Round((math.Max(100, length(config, "chartHeight", 500, 4000)) * width) / natural)
	outer := length(config, "quadrantPadding", options.PaddingOr(8), 200)
	xPad := length(config, "xAxisLabelPadding", 5, 200)
	yPad := length(config, "yAxisLabelPadding", 5, 200)
	hasPoints := len(model.Points) > 0

	label := func(t *Text, fontSize, max float64) text.Label {
		if t == nil {
			return text.Label{}
		}
		return text.Layout(t.Text, t.Type == "markdown", measurer, fontSize, math.Max(max, 40), 0)
	}

	title := text.Layout(model.Title, false, measurer, size, math.Max(width-2*outer, 40), text.Bold)
	titlePad := length(config, "titlePadding", 10, 200)
	titleSpace := 0.0
	if title.Height > 0 {
		titleSpace = title.Height + titlePad
	}

	hasX := (model.XAxisLeft != nil && model.XAxisLeft.Text != "") || (model.XAxisRight != nil && model.XAxisRight.Text != "")
	hasY := (model.YAxisBottom != nil && model.YAxisBottom.Text != "") || (model.YAxisTop != nil && model.YAxisTop.Text != "")
	// Mermaid moves the x axis labels below the chart as soon as there are points.
	xBelow := hasPoints || setting(config, "xAxisPosition") == "bottom"
	yRight := setting(config, "yAxisPosition") == "right"

	side := width - 2*outer
	xLeftMax := side
	if model.XAxisRight != nil {
		xLeftMax = side/2 - 16
	}
	xLeft := label(model.XAxisLeft, axisSize, xLeftMax)
	xRight := label(model.XAxisRight, axisSize, side/2-16)
	xHeight := math.Max(xLeft.Height, xRight.Height)
	xSpace := 0.0
	if hasX {
		xSpace = xHeight + xPad
	}
	tall := height - 2*outer - titleSpace - xSpace
	yBottomMax := tall
	if model.YAxisTop != nil {
		yBottomMax = tall/2 - 16
	}
	yBottom := label(model.YAxisBottom, axisSize, yBottomMax)
	yTop := label(model.YAxisTop, axisSize, tall/2-16)
	yHeight := math.Max(yBottom.Height, yTop.Height)
	ySpace := 0.0
	if hasY {
		ySpace = yHeight + yPad
	}

	left := outer
	if !yRight {
		left += ySpace
	}
	top := outer + titleSpace
	if !xBelow {
		top += xSpace
	}
	w := math.Max(0, side-ySpace)
	h := math.Max(0, tall)
	tileW := math.Max(0, (w-gap)/2)
	tileH := math.Max(0, (h-gap)/2)

	// Boxes a point label keeps clear of: the gaps between quadrants, quadrant labels, points,
	// and the labels placed before it.
	taken := []float64{left + tileW, top, left + w - tileW, top + h, left, top + tileH, left + w, top + h - tileH}
	isClear := func(from int, x0, y0, x1, y1 float64) bool {
		for k := from; k < len(taken); k += 4 {
			if x0 < taken[k+2] && x1 > taken[k] && y0 < taken[k+3] && y1 > taken[k+1] {
				return false
			}
		}
		return true
	}

	textTop := length(config, "quadrantTextTopPadding", 5, 200)
	var quadrants strings.Builder
	for k := 0; k < 4; k++ {
		x := left
		if k == 0 || k == 3 {
			x = left + w - tileW
		}
		y := top
		if k >= 2 {
			y = top + h - tileH
		}
		t := label(model.Quadrants[k], size, tileW-16)
		cx := x + tileW/2
		cy := y + tileH/2
		if hasPoints {
			cy = y + textTop + t.Height/2
		}
		taken = append(taken, cx-t.Width/2, cy-t.Height/2, cx+t.Width/2, cy+t.Height/2)
		quadrants.WriteString(`<g class="pele-quadrant" data-id="` + js.Num(float64(k+1)) + `">` +
			`<rect x="` + svg.Num(x) + `" y="` + svg.Num(y) + `" width="` + svg.Num(tileW) + `" height="` + svg.Num(tileH) + `" rx="` + svg.Radius + `"/>` +
			svg.LabelSvg(t, cx, cy, ` class="pele-label" fill="var(--_fg)" stroke="none"`, nil) +
			"</g>")
	}

	axes := ""
	if hasX {
		y := top - xPad - xHeight/2
		if xBelow {
			y = top + h + xPad + xHeight/2
		}
		lx := left + xLeft.Width/2
		if xRight.Height > 0 {
			lx = left + w/4
		}
		axes += svg.LabelSvg(xLeft, lx, y, "", nil) + svg.LabelSvg(xRight, left+(w*3)/4, y, "", nil)
	}
	if hasY {
		x := left - yPad - yHeight/2
		if yRight {
			x = left + w + yPad + yHeight/2
		}
		turned := func(t text.Label, y float64) string {
			if t.Height > 0 {
				return `<g transform="translate(` + svg.Num(x) + "," + svg.Num(y) + `) rotate(-90)">` + svg.LabelSvg(t, 0, 0, "", nil) + "</g>"
			}
			return ""
		}
		by := top + h - yBottom.Width/2
		if yTop.Height > 0 {
			by = top + (h*3)/4
		}
		axes += turned(yBottom, by) + turned(yTop, top+h/4)
	}

	radius := length(config, "pointRadius", 5, 200)
	textPad := length(config, "pointTextPadding", 5, 200)
	fill := svg.SeriesColor(0)
	var dots []dot
	var circles []string
	for _, point := range model.Points {
		if !(point.X >= 0 && point.X <= 1 && point.Y >= 0 && point.Y <= 1) {
			continue
		}
		shared, hasShared := model.Classes[point.ClassName]
		r := radius
		if point.HasRadius {
			r = point.Radius
		} else if hasShared && shared.HasRadius {
			r = shared.Radius
		}
		r = math.Min(r, 2000)
		color, strokeColor, strokeWidth := point.Color, point.StrokeColor, point.StrokeWidth
		if color == "" {
			color = shared.Color
		}
		if strokeColor == "" {
			strokeColor = shared.StrokeColor
		}
		if strokeWidth == "" {
			strokeWidth = shared.StrokeWidth
		}
		var styles []string
		if color != "" {
			styles = append(styles, "fill:"+hex(color))
		}
		if strokeWidth != "" {
			if strokeColor != "" {
				styles = append(styles, "stroke:"+hex(strokeColor))
			}
			styles = append(styles, "stroke-width:"+strokeWidth)
		}
		cx := left + point.X*w
		cy := top + (1-point.Y)*h
		reach := r
		if strokeWidth != "" {
			reach += math.Min(js.ParseInt(strokeWidth, 0), 400) / 2
		}
		dots = append(dots, dot{cx: cx, cy: cy, reach: reach, text: label(point.Text, pointSize, 200)})
		stroke := ""
		if strokeWidth != "" && strokeColor == "" {
			stroke = ` stroke="` + fill + `"`
		}
		circles = append(circles, `<circle class="pele-point`+svg.ClassNames(point.ClassName)+`" data-id="`+svg.EscText(point.Text.Text)+
			`" cx="`+svg.Num(cx)+`" cy="`+svg.Num(cy)+`" r="`+svg.Num(r)+`"`+stroke+svg.ResolveStyle(styles).Shape+"/>")
	}

	// A label goes below its point, or above, right or left of it when that is the first place
	// that is inside the chart and free. A second round lets it cross the gaps between quadrants.
	// Large charts skip the search for a free place.
	isCareful := len(dots) <= careful
	if isCareful {
		for _, d := range dots {
			taken = append(taken, d.cx-d.reach, d.cy-d.reach, d.cx+d.reach, d.cy+d.reach)
		}
	}
	var labels strings.Builder
	for _, d := range dots {
		t := d.text
		halfW := t.Width / 2
		halfH := t.Size*0.6 + (t.Height-t.LineHeight)/2
		away := d.reach + textPad
		spots := [8]float64{d.cx, d.cy + away + halfH, d.cx, d.cy - away - halfH, d.cx + away + halfW, d.cy, d.cx - away - halfW, d.cy}
		tx := d.cx
		ty := spots[1]
		found := false
		for k := 0; k < 16; k += 2 {
			spot := k % 8
			x := spots[spot]
			if spot < 4 && t.Width < w {
				x = math.Min(math.Max(spots[spot], left+halfW), left+w-halfW)
			}
			y := spots[spot+1]
			if x-halfW < left || x+halfW > left+w || y-halfH < top || y+halfH > top+h {
				continue
			}
			from := 8
			if k < 8 {
				from = 0
			}
			free := !isCareful || isClear(from, x-halfW, y-halfH, x+halfW, y+halfH)
			if !found || free {
				tx = x
				ty = y
				found = true
			}
			if free {
				break
			}
		}
		if isCareful {
			taken = append(taken, tx-halfW, ty-halfH, tx+halfW, ty+halfH)
		}
		labels.WriteString(svg.LabelSvg(t, tx, ty, ` class="pele-label"`, nil))
	}

	// Mermaid puts each new point under the ones before it.
	var content strings.Builder
	content.WriteString(svg.LabelSvg(title, width/2, titlePad/2+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil))
	content.WriteString(`<g class="pele-quadrants" fill="var(--_s)" stroke="var(--_b)">` + quadrants.String() + "</g>")
	if axes != "" {
		content.WriteString(`<g class="pele-axis-labels" font-size="` + js.Num(axisSize) + `" fill="var(--_m)">` + axes + "</g>")
	}
	if len(dots) > 0 {
		content.WriteString(`<g class="pele-points" fill="` + fill + `">`)
		for k := len(circles) - 1; k >= 0; k-- {
			content.WriteString(circles[k])
		}
		content.WriteString("</g>")
		content.WriteString(`<g class="pele-point-labels" font-size="` + js.Num(pointSize) + `">` + labels.String() + "</g>")
	}

	acc := svg.Accessible{AccTitle: model.AccTitle, AccDescr: model.AccDescr}
	return core.Rendered{
		SVG:    svg.Document("quadrantChart", width, height, size, options, acc, content.String()),
		Width:  width,
		Height: height,
	}
}

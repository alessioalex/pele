package wardley

import (
	"math"
	"slices"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
	"github.com/alessioalex/pele/go/internal/text"
)

var defaultStages = []string{"Genesis", "Custom Built", "Product", "Commodity"}

const (
	// Mermaid's canvas has this much room around the plot on every side.
	canvasMargin = 48
	r            = 6
	square       = 10
	labelOffset  = 8
	pipelinePad  = 15
	forceWidth   = 60
	forceHeight  = 30
	forceHead    = 20
	accent       = "var(--_c)"
	lineColor    = "var(--_l)"
)

type placed struct {
	x, y float64
	node *node
}

func setting(config *core.Object, key string, fallback, min, max float64) float64 {
	if v, ok := config.Section("wardley-beta").Value(key).(float64); ok && v >= min && v <= max {
		return v
	}
	return fallback
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// startLabel writes a label with its lines starting at x and its first baseline at y.
func startLabel(label text.Label, x, y float64, attrs string) string {
	shared := svg.SharedStyle(label)
	var body strings.Builder
	baseline := y
	for _, line := range label.Lines {
		first := true
		for _, span := range line {
			if span.Icon != "" {
				continue
			}
			pos := ""
			if first {
				pos = ` x="` + svg.Num(x) + `" y="` + svg.Num(baseline) + `"`
			}
			body.WriteString("<tspan" + pos + svg.SpanStyle(span.Style&^shared) + ">" + svg.Esc(span.Text) + "</tspan>")
			first = false
		}
		baseline += label.LineHeight
	}
	if body.Len() == 0 {
		return ""
	}
	return "<text" + svg.TextStyle(attrs, shared) + ` xml:space="preserve">` + body.String() + "</text>"
}

func dimension(config *core.Object, given float64, has bool, key string, fallback float64) float64 {
	value := setting(config, key, fallback, 1, 1e6)
	if has && js.IsFinite(given) {
		value = given
	}
	return clamp(value, 200, 10000) - 2*canvasMargin
}

func render(m *model, config *core.Object, options *core.Options) core.Rendered {
	natural := dimension(config, m.width, m.hasSize, "width", 900)
	return common.FitWidth(options, natural, 160, func(plotW float64, tight bool) core.Rendered {
		return draw(m, config, options, plotW, natural, tight)
	})
}

func draw(m *model, config *core.Object, options *core.Options, plotW, naturalW float64, tight bool) core.Rendered {
	size := options.FontSizeOr(16)
	small := js.Round(size * 0.8125)
	tiny := js.Round(size * 0.6875)
	measurer := options.Measurer
	pad := options.PaddingOr(8)

	// A narrower map is not as much shorter, so that components keep some room between them.
	plotH := dimension(config, m.height, m.hasSize, "height", 600) * math.Max(0.75, plotW/naturalW)
	showGrid := config.Section("wardley-beta").Value("showGrid") == true

	minX, minY, maxX, maxY := 0.0, 0.0, plotW, plotH
	grow := func(x0, y0, x1, y1 float64) {
		if x0 < minX {
			minX = x0
		}
		if y0 < minY {
			minY = y0
		}
		if x1 > maxX {
			maxX = x1
		}
		if y1 > maxY {
			maxY = y1
		}
	}
	centered := func(label text.Label, cx, cy float64, attrs string) string {
		grow(cx-label.Width/2, cy-label.Height/2, cx+label.Width/2, cy+label.Height/2)
		return svg.LabelSvg(label, cx, cy, attrs, options.Icons)
	}
	starting := func(label text.Label, x, y float64, attrs string) string {
		grow(x, y-label.Size, x+label.Width, y-label.Size+label.Height+4)
		return startLabel(label, x, y, attrs)
	}
	defaultMax := 4000.0
	if tight {
		defaultMax = math.Max(64, math.Min(120, plotW/2))
	}
	textAt := func(raw string, fontSize, maxWidth float64, style int) text.Label {
		return text.Layout(raw, false, measurer, fontSize, maxWidth, style)
	}
	label := func(raw string, fontSize float64) text.Label { return textAt(raw, fontSize, defaultMax, 0) }
	projectX := func(v float64) float64 { return (v / 100) * plotW }
	projectY := func(v float64) float64 { return plotH - (v/100)*plotH }
	smallS, tinyS := js.Num(small), js.Num(tiny)

	stages := defaultStages
	if len(m.stages) > 0 {
		stages = m.stages
	}
	custom := m.hasBoundaries && len(m.boundaries) == len(stages)
	var frame, stageLabels strings.Builder
	stageBottom := plotH
	previous := 0.0
	for index, stage := range stages {
		start := float64(index) / float64(len(stages))
		end := float64(index+1) / float64(len(stages))
		if custom {
			start, end = previous, clamp(m.boundaries[index], 0, 1)
		}
		previous = end
		if index > 0 {
			frame.WriteString(`<path class="pele-wardley-stage" d="M` + svg.Num(start*plotW) + ",0V" + svg.Num(plotH) + `" stroke="var(--_b)" stroke-dasharray="4 4"/>`)
		}
		l := textAt(stage, small, math.Max(48, math.Abs(end-start)*plotW-12), 0)
		stageLabels.WriteString(centered(l, ((start+end)/2)*plotW, plotH+8+l.Height/2, ` class="pele-wardley-stage-label" font-size="`+smallS+`" fill="var(--_m)"`))
		stageBottom = math.Max(stageBottom, plotH+8+l.Height)
	}
	if showGrid {
		for i := 1; i < 4; i++ {
			fi := float64(i)
			frame.WriteString(`<path class="pele-wardley-grid" d="M` + svg.Num((plotW*fi)/4) + ",0V" + svg.Num(plotH) + "M0," + svg.Num((plotH*fi)/4) + "H" + svg.Num(plotW) + `" stroke="var(--_b)" stroke-dasharray="2 6"/>`)
		}
	}
	frame.WriteString(`<path class="pele-wardley-axis" d="M0,0V` + svg.Num(plotH) + "H" + svg.Num(plotW) + `" stroke="` + lineColor + `"/>`)
	xTitle := label("Evolution", small)
	yTitle := label("Visibility", small)
	axisAttrs := ` class="pele-wardley-axis-label" font-size="` + smallS + `" font-weight="var(--_hw)"`
	axisLabels := centered(xTitle, plotW/2, stageBottom+6+xTitle.Height/2, axisAttrs) +
		svg.LabelSvg(yTitle, -16, plotH/2, axisAttrs+` transform="rotate(-90 -16 `+svg.Num(plotH/2)+`)"`, nil)
	grow(-16-yTitle.Height/2, 0, 0, 0)
	legendTop := stageBottom + 6 + xTitle.Height + 10

	var positions []*placed
	byID := map[string]*placed{}
	for _, n := range m.nodes {
		p := &placed{projectX(n.x), projectY(n.y), n}
		if old, ok := byID[n.id]; ok {
			*old = *p
			continue
		}
		byID[n.id] = p
		positions = append(positions, p)
	}

	// Pipelines: a box around the components, with the parent as a square on its top edge.
	var pipelines strings.Builder
	members := map[string]map[string]bool{}
	for _, pl := range m.pipelines {
		set := map[string]bool{}
		for _, id := range pl.componentIDs {
			set[id] = true
		}
		members[pl.nodeID] = set
		var parts []*placed
		for _, id := range pl.componentIDs {
			if part, ok := byID[id]; ok {
				parts = append(parts, part)
			}
		}
		if len(parts) == 0 {
			continue
		}
		slices.SortStableFunc(parts, func(a, b *placed) int { return js.Cmp(a.x - b.x) })
		left, right := parts[0].x, parts[len(parts)-1].x
		y := parts[0].y
		top := y - 2*r
		if parent, ok := byID[pl.nodeID]; ok {
			parent.x = (left + right) / 2
			parent.y = top - square/6.0
		}
		grow(left-pipelinePad, top, right+pipelinePad, top+4*r)
		pipelines.WriteString(`<g class="pele-wardley-pipeline" data-id="` + svg.EscText(pl.nodeID) + `">` +
			`<rect x="` + svg.Num(left-pipelinePad) + `" y="` + svg.Num(top) + `" width="` + svg.Num(right-left+2*pipelinePad) + `" height="` + svg.Num(4*r) +
			`" rx="` + svg.Radius + `" fill="var(--_s)" stroke="var(--_b)"/>`)
		if len(parts) > 1 {
			pipelines.WriteString(`<path d="M` + svg.Num(left) + "," + svg.Num(y) + "H" + svg.Num(right) + `" stroke="` + lineColor + `" stroke-dasharray="4 4"/>`)
		}
		pipelines.WriteString("</g>")
	}

	anchorMax := 4000.0
	if tight {
		anchorMax = math.Max(64, plotW/2)
	}
	// How far from a node's center a line to it stops.
	anchorLabels := map[*node]text.Label{}
	reach := func(at *placed, ux, uy float64) float64 {
		n := at.node
		if n.className == "anchor" {
			l, ok := anchorLabels[n]
			if !ok {
				l = textAt(n.label, small, anchorMax, text.Bold)
				anchorLabels[n] = l
			}
			return 1 / js.Hypot(ux/(l.Width/2+5), uy/(small*0.8))
		}
		if n.isPipelineParent {
			return square / math.Sqrt2
		}
		if n.sourceStrategy != "" {
			return 2 * r
		}
		return r
	}

	// Quadrants around a node that a link runs through: 1 up and right, 2 down and right, 4 up and left, 8 down and left.
	taken := map[*placed]int{}
	occupy := func(at *placed, ux, uy float64) {
		if math.Abs(ux) < 0.25 || math.Abs(uy) < 0.25 {
			return
		}
		bit := 8
		if ux > 0 {
			bit = 2
			if uy < 0 {
				bit = 1
			}
		} else if uy < 0 {
			bit = 4
		}
		taken[at] |= bit
	}

	var links strings.Builder
	for _, l := range m.links {
		from, ok1 := byID[l.source]
		to, ok2 := byID[l.target]
		if !ok1 || !ok2 || from == to {
			continue
		}
		// A pipeline component belongs to its parent already.
		if members[l.target][l.source] {
			continue
		}
		dx, dy := to.x-from.x, to.y-from.y
		distance := js.Hypot(dx, dy)
		if distance < 0.01 {
			continue
		}
		ux, uy := dx/distance, dy/distance
		start, end := reach(from, ux, uy), reach(to, ux, uy)
		if start+end >= distance {
			continue
		}
		occupy(from, ux, uy)
		occupy(to, -ux, -uy)
		x1, y1 := from.x+ux*start, from.y+uy*start
		x2, y2 := to.x-ux*end, to.y-uy*end
		heads := ""
		trim := svg.MarkerTrim("arrow_point")
		if l.flow == "forward" || l.flow == "bidirectional" {
			heads += svg.Marker("arrow_point", x2, y2, ux, uy, lineColor)
			x2 -= ux * trim
			y2 -= uy * trim
		}
		if l.flow == "backward" || l.flow == "bidirectional" {
			heads += svg.Marker("arrow_point", x1, y1, -ux, -uy, lineColor)
			x1 += ux * trim
			y1 += uy * trim
		}
		caption := ""
		if l.label != "" {
			lb := textAt(l.label, tiny+1, 200, 0)
			// Beside the middle of the line, on its upper side.
			side := -1.0
			if ux >= 0 {
				side = 1
			}
			px, py := uy*side, -ux*side
			away := 5 + (math.Abs(px)*lb.Width)/2 + (math.Abs(py)*lb.Height)/2
			caption = centered(lb, (from.x+to.x)/2+px*away, (from.y+to.y)/2+py*away, ` class="pele-edge-label" font-size="`+js.Num(tiny+1)+`" fill="var(--_m)"`)
		}
		dashedClass, dashedAttr := "", ""
		if l.dashed {
			dashedClass, dashedAttr = " pele-wardley-dashed", ` stroke-dasharray="6 6"`
		}
		links.WriteString(`<g class="pele-edge` + dashedClass + `" data-id="` + svg.EscText(l.source) + "-" + svg.EscText(l.target) + `">` +
			`<path d="M` + svg.Num(x1) + "," + svg.Num(y1) + "L" + svg.Num(x2) + "," + svg.Num(y2) + `" stroke="` + lineColor + `"` + dashedAttr + "/>" +
			heads + caption + "</g>")
	}

	// Evolution: a dashed arrow to where the component is heading, drawn as an open circle.
	var trends strings.Builder
	for _, t := range m.trends {
		origin, ok := byID[t.nodeID]
		if !ok {
			continue
		}
		tx, ty := projectX(t.targetX), projectY(t.targetY)
		dx, dy := tx-origin.x, ty-origin.y
		distance := js.Hypot(dx, dy)
		arrow := ""
		start := 0.0
		if distance > 0 {
			start = reach(origin, dx/distance, dy/distance)
		}
		if distance > start+r+4 {
			ux, uy := dx/distance, dy/distance
			ex, ey := tx-ux*(r+2), ty-uy*(r+2)
			trim := svg.MarkerTrim("arrow_point")
			arrow = `<path d="M` + svg.Num(origin.x+ux*start) + "," + svg.Num(origin.y+uy*start) + "L" + svg.Num(ex-ux*trim) + "," + svg.Num(ey-uy*trim) +
				`" stroke="` + accent + `" stroke-dasharray="4 4"/>` + svg.Marker("arrow_point", ex, ey, ux, uy, accent)
		}
		grow(tx-r, ty-r, tx+r, ty+r)
		trends.WriteString(`<g class="pele-wardley-trend" data-id="` + svg.EscText(t.nodeID) + `">` + arrow +
			`<circle cx="` + svg.Num(tx) + `" cy="` + svg.Num(ty) + `" r="6" fill="var(--_bg)" stroke="` + accent + `"/></g>`)
	}

	var nodes strings.Builder
	tri := 1.2 * r
	dot := 0.7 * r
	for _, at := range positions {
		n := at.node
		x, y := at.x, at.y
		anchor := n.className == "anchor"
		strategy := n.sourceStrategy
		shape := ""
		if strategy != "" {
			fill := "var(--_bg)"
			switch strategy {
			case "outsource":
				fill = "var(--_m)"
			case "buy":
				fill = "var(--_a)"
			case "build":
				fill = "var(--_s)"
			}
			shape += `<circle class="pele-wardley-` + strategy + `" cx="` + svg.Num(x) + `" cy="` + svg.Num(y) + `" r="12" fill="` + fill + `" stroke="` + lineColor + `"/>`
		}
		if strategy == "market" {
			bx := tri * js.Cos(math.Pi/6)
			by := y + tri*js.Sin(math.Pi/6)
			shape += `<path d="M` + svg.Num(x) + "," + svg.Num(y-tri) + "L" + svg.Num(x-bx) + "," + svg.Num(by) + "H" + svg.Num(x+bx) + `Z" fill="none" stroke="` + lineColor + `"/>`
			for _, c := range [][2]float64{{x, y - tri}, {x - bx, by}, {x + bx, by}} {
				shape += `<circle cx="` + svg.Num(c[0]) + `" cy="` + svg.Num(c[1]) + `" r="` + svg.Num(dot) + `" fill="var(--_bg)" stroke="` + lineColor + `"/>`
			}
		} else if n.isPipelineParent {
			shape += `<rect x="` + svg.Num(x-square/2) + `" y="` + svg.Num(y-square/2) + `" width="10" height="10" fill="var(--_bg)" stroke="` + lineColor + `"/>`
		} else if !anchor {
			shape += `<circle cx="` + svg.Num(x) + `" cy="` + svg.Num(y) + `" r="6" fill="var(--_bg)" stroke="` + lineColor + `"/>`
		}
		if n.inertia {
			offset := float64(r) + 10
			if n.isPipelineParent {
				offset = square/2 + 10
			}
			if strategy != "" {
				offset += r + 4
			}
			shape += `<rect class="pele-wardley-inertia" x="` + svg.Num(x+offset) + `" y="` + svg.Num(y-r) + `" width="4" height="12" fill="` + lineColor + `"/>`
		}
		var ox, oy *float64
		if n.labelOffsetX != nil {
			v := clamp(*n.labelOffsetX, -5000, 5000)
			ox = &v
		}
		if n.labelOffsetY != nil {
			v := clamp(*n.labelOffsetY, -5000, 5000)
			oy = &v
		}
		or := func(p *float64, def float64) float64 {
			if p != nil {
				return *p
			}
			return def
		}
		var caption string
		if anchor {
			l, ok := anchorLabels[n]
			if !ok {
				l = textAt(n.label, small, anchorMax, text.Bold)
			}
			caption = centered(l, x+or(ox, 0), y+or(oy, -3), ` class="pele-label" font-size="`+smallS+`" font-weight="var(--_hw)"`)
		} else {
			l := label(n.label, small)
			attrs := ` class="pele-label" font-size="` + smallS + `"`
			lift := float64(labelOffset)
			if strategy != "" {
				lift += 10
			}
			if ox != nil || oy != nil {
				caption = starting(l, x+or(ox, lift), y+or(oy, -lift), attrs)
			} else if n.inPipeline {
				caption = centered(l, x, y+2*r+4+l.Height/2, attrs)
			} else {
				// The label goes up and to the right unless a link leaves that way; then the next free corner.
				busy := taken[at]
				corner := 1
				switch {
				case busy&1 == 0:
					corner = 1
				case busy&2 == 0:
					corner = 2
				case busy&4 == 0:
					corner = 4
				case busy&8 == 0:
					corner = 8
				}
				right := corner == 1 || corner == 2
				up := corner == 1 || corner == 4
				lx := x - lift - l.Width
				if right {
					lx = x + lift
				}
				ly := y + lift + small*0.75
				if up {
					ly = y - lift
				}
				caption = starting(l, lx, ly, attrs)
			}
		}
		grow(x-2*r, y-2*r, x+2*r+20, y+2*r)
		kind := "component"
		switch {
		case anchor:
			kind = "anchor"
		case n.isPipelineParent:
			kind = "pipeline-parent"
		case n.inPipeline:
			kind = "pipeline-component"
		}
		nodes.WriteString(`<g class="pele-node pele-wardley-` + kind + `" data-id="` + svg.EscText(n.id) + `">` + shape + caption + "</g>")
	}

	var annotations strings.Builder
	for _, a := range m.annotations {
		num := js.NumberToString(a.number)
		l := textAt(num, tiny, 4000, text.Bold)
		x, y := projectX(a.x), projectY(a.y)
		rad := math.Max(10, l.Width/2+4)
		grow(x-rad, y-rad, x+rad, y+rad)
		annotations.WriteString(`<g class="pele-wardley-annotation" data-id="` + svg.EscText(num) + `">` +
			`<circle cx="` + svg.Num(x) + `" cy="` + svg.Num(y) + `" r="` + svg.Num(rad) + `" fill="var(--_bg)" stroke="` + lineColor + `"/>` +
			svg.LabelSvg(l, x, y, ` font-size="`+tinyS+`" font-weight="var(--_hw)"`, nil) + "</g>")
	}
	var listed []annotation
	for _, a := range m.annotations {
		if a.text != "" {
			listed = append(listed, a)
		}
	}
	slices.SortStableFunc(listed, func(a, b annotation) int { return js.Cmp(a.number - b.number) })
	if len(listed) > 0 {
		listMax := 320.0
		if tight {
			listMax = math.Max(80, plotW-20)
		}
		var lines []text.Label
		width, height := 0.0, 0.0
		for _, a := range listed {
			l := textAt(js.NumberToString(a.number)+". "+a.text, small, listMax, 0)
			lines = append(lines, l)
			width = math.Max(width, l.Width)
			height += l.Height
		}
		width += 20
		height += 14
		// Without a position the list goes under the map, where it covers nothing.
		bx, by := 0.0, legendTop
		if box := m.annotationsBox; box != nil {
			bx = clamp(projectX(box[0]), 0, math.Max(0, plotW-width))
			by = clamp(projectY(box[1]), 0, math.Max(0, plotH-height))
		}
		grow(bx, by, bx+width, by+height)
		var rows strings.Builder
		y := by + 7
		for _, l := range lines {
			rows.WriteString(startLabel(l, bx+10, y+l.LineHeight/2+small*0.35, ` font-size="`+smallS+`"`))
			y += l.Height
		}
		annotations.WriteString(`<g class="pele-wardley-annotations"><rect x="` + svg.Num(bx) + `" y="` + svg.Num(by) + `" width="` + svg.Num(width) + `" height="` + svg.Num(height) +
			`" rx="` + svg.Radius + `" fill="var(--_bg)" stroke="var(--_b)"/>` + rows.String() + "</g>")
	}

	var notes strings.Builder
	for _, n := range m.notes {
		notes.WriteString(starting(textAt(n.text, small, math.Min(360, plotW*0.6), text.Bold), projectX(n.x), projectY(n.y), ` class="pele-wardley-note" font-size="`+smallS+`" font-weight="var(--_hw)"`))
	}

	// Accelerators point right, deaccelerators left.
	var forces strings.Builder
	drawForce := func(name string, px, py float64, forward bool) {
		x, y := projectX(px), projectY(py)
		half := forceHeight / 2.0
		tip, tail, neck := x, x+forceWidth, x+forceHead
		kind := "deaccelerator"
		if forward {
			tip, tail, neck = x+forceWidth, x, x+forceWidth-forceHead
			kind = "accelerator"
		}
		d := "M" + svg.Num(tail) + "," + svg.Num(y-half) + "H" + svg.Num(neck) + "V" + svg.Num(y-half-8) + "L" + svg.Num(tip) + "," + svg.Num(y) +
			"L" + svg.Num(neck) + "," + svg.Num(y+half+8) + "V" + svg.Num(y+half) + "H" + svg.Num(tail) + "Z"
		grow(x, y-half-8, x+forceWidth, y+half+8)
		forces.WriteString(`<g class="pele-wardley-` + kind + `" data-id="` + svg.EscText(name) + `">` +
			`<path d="` + d + `" fill="var(--_s)" stroke="` + lineColor + `"/>` +
			centered(textAt(name, tiny+1, 160, text.Bold), x+forceWidth/2, y+half+8+12, ` font-size="`+js.Num(tiny+1)+`" font-weight="var(--_hw)"`) + "</g>")
	}
	for _, f := range m.accelerators {
		drawForce(f.name, f.x, f.y, true)
	}
	for _, f := range m.deaccelerators {
		drawForce(f.name, f.x, f.y, false)
	}

	titleMax := 4000.0
	if tight {
		titleMax = math.Max(80, plotW)
	}
	title := textAt(m.title, size, titleMax, text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 12
	}
	left := math.Min(minX, plotW/2-title.Width/2)
	right := math.Max(maxX, plotW/2+title.Width/2)
	tx := pad - left
	ty := pad + titleHeight - minY
	totalWidth := math.Ceil(right - left + 2*pad)
	totalHeight := math.Ceil(maxY - minY + titleHeight + 2*pad)
	var content strings.Builder
	content.WriteString(svg.LabelSvg(title, tx+plotW/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil))
	content.WriteString(`<g transform="translate(` + svg.Num(tx) + "," + svg.Num(ty) + `)">`)
	content.WriteString(`<g class="pele-wardley-frame" fill="none">` + frame.String() + "</g>")
	content.WriteString(stageLabels.String() + axisLabels + pipelines.String())
	if links.Len() > 0 {
		content.WriteString(`<g class="pele-edges" fill="none">` + links.String() + "</g>")
	}
	if trends.Len() > 0 {
		content.WriteString(`<g class="pele-wardley-trends" fill="none">` + trends.String() + "</g>")
	}
	content.WriteString(nodes.String() + annotations.String() + notes.String() + forces.String() + "</g>")
	acc := svg.Accessible{AccTitle: m.accTitle, AccDescr: m.accDescr}
	return core.Rendered{SVG: svg.Document("wardley", totalWidth, totalHeight, size, options, acc, content.String()), Width: totalWidth, Height: totalHeight}
}

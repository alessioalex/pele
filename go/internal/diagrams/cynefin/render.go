package cynefin

import (
	"math"
	"strconv"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
	"github.com/alessioalex/pele/go/internal/text"
)

type quadrant struct {
	name     string
	col, row float64
	color    int
}

var quadrants = []quadrant{
	{"complex", 0, 0, 0},
	{"complicated", 1, 0, 1},
	{"chaotic", 0, 1, 3},
	{"clear", 1, 1, 2},
}

const confusionColor = 4

// [name, decision model, kind of practice]
var meta = map[string][3]string{
	"complex":     {"Complex", "Probe → Sense → Respond", "Emergent Practices"},
	"complicated": {"Complicated", "Sense → Analyse → Respond", "Good Practices"},
	"clear":       {"Clear", "Sense → Categorise → Respond", "Best Practices"},
	"chaotic":     {"Chaotic", "Act → Sense → Respond", "Novel Practices"},
	"confusion":   {"Confusion", "", "Disorder"},
}

const (
	maxConfusionItems = 3
	itemMaxWidth      = 200
	itemPadX          = 10
	itemPadY          = 4
	itemGap           = 6
	headGap           = 10
	margin            = 20
	clearance         = 10
)

type badge struct {
	text     string
	label    text.Label
	w, h     float64
	overflow bool
}

type block struct {
	name      string
	title     text.Label
	subtitles []text.Label
	badges    []badge
	w, h      float64
	cx, cy    float64
}

func setting(config *core.Object, key string, fallback, min, max float64) float64 {
	if v, ok := config.Section("cynefin").Value(key).(float64); ok && v >= min && v <= max {
		return v
	}
	return fallback
}

func renderCynefin(model *Model, config *core.Object, options *core.Options) core.Rendered {
	size := options.FontSizeOr(16)
	small := js.Round(size * 0.8125)
	itemSize := js.Round(size * 0.875)
	measurer := options.Measurer
	pad := options.PaddingOr(8)
	icons := options.Icons
	showDescriptions := true
	if v, ok := config.Section("cynefin").Value("showDomainDescriptions").(bool); ok && !v {
		showDescriptions = false
	}
	amplitude := setting(config, "boundaryAmplitude", 8, 0, 50)
	seed := resolveSeed(setting(config, "seed", 0, -1e15, 1e15), options.IDPrefixOr("pele"))
	itemLine := js.Round(itemSize * 1.5)

	newBadge := func(t string, overflow bool) badge {
		label := text.Layout(t, false, measurer, itemSize, itemMaxWidth, 0)
		return badge{text: t, label: label, w: label.Width + 2*itemPadX, h: math.Max(label.Height, itemLine) + 2*itemPadY, overflow: overflow}
	}

	newBlock := func(name string) *block {
		m := meta[name]
		title := text.Layout(m[0], false, measurer, size, 4000, text.Bold)
		var subtitles []text.Label
		if showDescriptions {
			for _, t := range []string{m[1], m[2]} {
				if t != "" {
					subtitles = append(subtitles, text.Layout(t, false, measurer, small, 4000, text.Italic))
				}
			}
		}
		var items []Item
		if d := model.Domains[name]; d != nil {
			items = d.Items
		}
		shown := len(items)
		if name == "confusion" && len(items) > maxConfusionItems {
			shown = maxConfusionItems
		}
		var badges []badge
		for i := 0; i < shown; i++ {
			badges = append(badges, newBadge(items[i].Label, false))
		}
		if shown < len(items) {
			badges = append(badges, newBadge("+"+strconv.Itoa(len(items)-shown)+" more", true))
		}
		w := title.Width
		h := title.Height
		for _, s := range subtitles {
			w = math.Max(w, s.Width)
			h += s.Height
		}
		if len(badges) > 0 {
			h += headGap - itemGap
		}
		for _, b := range badges {
			w = math.Max(w, b.w)
			h += itemGap + b.h
		}
		return &block{name: name, title: title, subtitles: subtitles, badges: badges, w: w, h: h}
	}

	blocksIn := make([]*block, len(quadrants))
	for i, q := range quadrants {
		blocksIn[i] = newBlock(q.name)
	}
	confusion := newBlock("confusion")
	rx := math.Max(88, confusion.w*0.71+8)
	ry := math.Max(64, confusion.h*0.71+6)

	quadW := setting(config, "width", 640, 100, 10000) / 2
	quadH := setting(config, "height", 440, 100, 10000) / 2
	for _, b := range blocksIn {
		quadW = math.Max(quadW, b.w+2*margin)
		quadH = math.Max(quadH, b.h+2*margin)
	}
	// A block sits in the middle of its quadrant, so its inner corner must stay clear of the ellipse.
	for _, b := range blocksIn {
		dx := (quadW - b.w) / 2
		dy := (quadH - b.h) / 2
		if js.Pow(dx/(rx+clearance), 2)+js.Pow(dy/(ry+clearance), 2) >= 1 {
			continue
		}
		wider := b.w + 2*(rx+clearance)
		taller := b.h + 2*(ry+clearance)
		if wider*quadH <= quadW*taller {
			quadW = math.Max(quadW, wider)
		} else {
			quadH = math.Max(quadH, taller)
		}
	}

	title := text.Layout(model.Title, false, measurer, size, 4000, text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 12
	}
	width := 2 * quadW
	height := 2 * quadH
	x0 := pad
	y0 := pad + titleHeight
	cx := x0 + quadW
	cy := y0 + quadH
	right := x0 + width
	bottom := y0 + height

	foldOpts := fold
	foldOpts.segments, foldOpts.pinEnd = 4, true
	westOpts := horizon
	westOpts.segments, westOpts.pinEnd = 4, true
	eastOpts := horizon
	eastOpts.segments, eastOpts.pinStart = 4, true
	foldLine := wave(y0, cy, cx, seed, amplitude, foldOpts)
	west := wave(x0, cx, cy, seed+100, amplitude, westOpts)
	east := wave(cx, right, cy, seed+200, amplitude, eastOpts)
	drop := cliff(cx, cy, bottom, width*0.03)
	at := func(curve []float64, end bool) string {
		if end {
			return svg.Num(curve[len(curve)-2]) + "," + svg.Num(curve[len(curve)-1])
		}
		return svg.Num(curve[0]) + "," + svg.Num(curve[1])
	}

	regions := map[string]string{
		"complex":     "M" + svg.Num(x0) + "," + svg.Num(y0) + "L" + at(foldLine, false) + forward(foldLine) + backward(west) + "Z",
		"complicated": "M" + at(foldLine, false) + "L" + svg.Num(right) + "," + svg.Num(y0) + "L" + at(east, true) + backward(east) + backward(foldLine) + "Z",
		"chaotic":     "M" + at(west, false) + forward(west) + forward(drop) + "L" + svg.Num(x0) + "," + svg.Num(bottom) + "Z",
		"clear":       "M" + at(east, false) + forward(east) + "L" + svg.Num(right) + "," + svg.Num(bottom) + "L" + at(drop, true) + backward(drop) + "Z",
	}

	var fills strings.Builder
	for _, q := range quadrants {
		fills.WriteString(`<path class="pele-cynefin-region" data-id="` + q.name + `" d="` + regions[q.name] + `" fill="` + svg.SeriesColor(q.color) + `" fill-opacity="0.14"/>`)
	}
	boundaries := `<g class="pele-cynefin-boundaries" fill="none" stroke="var(--_b)" stroke-dasharray="6 3">` +
		`<path d="` + curvePath(foldLine) + `"/><path d="` + curvePath(west) + `"/><path d="` + curvePath(east) + `"/>` +
		`<path class="pele-cynefin-cliff" d="` + curvePath(drop) + `" stroke="var(--_l)" stroke-width="2" stroke-dasharray="none" stroke-linecap="round"/></g>`
	oval := ellipsePath(cx, cy, rx, ry)
	ellipse := `<path d="` + oval + `" fill="var(--_bg)"/>` +
		`<path class="pele-cynefin-region" data-id="confusion" d="` + oval + `" fill="` + svg.SeriesColor(confusionColor) +
		`" fill-opacity="0.18" stroke="var(--_b)" stroke-dasharray="4 2"/>`

	drawBlock := func(b *block) string {
		y := b.cy - b.h/2
		var out strings.Builder
		out.WriteString(svg.LabelSvg(b.title, b.cx, y+b.title.Height/2, ` class="pele-label" font-weight="var(--_hw)"`, nil))
		y += b.title.Height
		for _, s := range b.subtitles {
			out.WriteString(svg.LabelSvg(s, b.cx, y+s.Height/2, ` class="pele-cynefin-subtitle" font-size="`+js.Num(small)+`" font-style="italic" fill="var(--_m)"`, nil))
			y += s.Height
		}
		y += headGap - itemGap
		for _, item := range b.badges {
			y += itemGap
			class, id, dash, fill := "", ` data-id="`+svg.EscText(item.text)+`"`, "", ""
			if item.overflow {
				class, id, dash, fill = " pele-cynefin-overflow", "", ` stroke-dasharray="3 2"`, ` fill="var(--_m)"`
			}
			out.WriteString(`<g class="pele-node` + class + `"` + id + `>` +
				`<rect x="` + svg.Num(b.cx-item.w/2) + `" y="` + svg.Num(y) + `" width="` + svg.Num(item.w) + `" height="` + svg.Num(item.h) +
				`" rx="` + svg.Radius + `" fill="var(--_bg)" stroke="var(--_b)"` + dash + `/>` +
				svg.LabelSvg(item.label, b.cx, y+item.h/2, ` class="pele-label" font-size="`+js.Num(itemSize)+`"`+fill, icons) +
				"</g>")
			y += item.h
		}
		return `<g class="pele-cynefin-domain" data-id="` + b.name + `">` + out.String() + "</g>"
	}

	blocks := map[string]*block{}
	var domains strings.Builder
	for index, q := range quadrants {
		b := blocksIn[index]
		b.cx = x0 + quadW*(q.col+0.5)
		b.cy = y0 + quadH*(q.row+0.5)
		blocks[q.name] = b
		domains.WriteString(drawBlock(b))
	}
	confusion.cx = cx
	confusion.cy = cy
	blocks["confusion"] = confusion
	domains.WriteString(drawBlock(confusion))

	inside := func(b *block, x, y float64) bool {
		if b == confusion {
			return js.Pow((x-cx)/(rx+4), 2)+js.Pow((y-cy)/(ry+4), 2) <= 1
		}
		return math.Abs(x-b.cx) <= b.w/2+8 && math.Abs(y-b.cy) <= b.h/2+8
	}

	var edges strings.Builder
	seen := map[string]int{}
	markerTrim := tmpMarkerTrim("arrow_point")
	for _, transition := range model.Transitions {
		from := blocks[transition.From]
		to := blocks[transition.To]
		if from == nil || to == nil || from == to {
			continue
		}
		key := transition.To + " " + transition.From
		if transition.From < transition.To {
			key = transition.From + " " + transition.To
		}
		repeat := seen[key]
		seen[key] = repeat + 1

		dx := to.cx - from.cx
		dy := to.cy - from.cy
		length := js.Hypot(dx, dy)
		if length == 0 || length != length {
			length = 1
		}
		mx := (from.cx + to.cx) / 2
		my := (from.cy + to.cy) / 2
		// Curves bow away from the middle of the diagram, which keeps them off the ellipse.
		outward := -1.0
		if -dy*(mx-cx)+dx*(my-cy) >= 0 {
			outward = 1
		}
		nx := (-dy / length) * outward
		ny := (dx / length) * outward
		// A diagonal runs through the middle, so it bows far enough to pass the ellipse.
		diagonal := from != confusion && to != confusion && math.Abs(dx) > 1 && math.Abs(dy) > 1
		var bow float64
		if diagonal {
			bow = 2 * (js.Hypot(rx*nx, ry*ny) + 16)
		} else {
			bow = length * 0.15
		}
		bow += float64(repeat) * 56
		px := mx + nx*bow
		py := my + ny*bow
		point := func(t float64) (float64, float64) {
			u := 1 - t
			return u*u*from.cx + 2*u*t*px + t*t*to.cx, u*u*from.cy + 2*u*t*py + t*t*to.cy
		}
		insideAt := func(b *block, t float64) bool {
			x, y := point(t)
			return inside(b, x, y)
		}
		crossing := func(b *block, lo, hi float64, leaving bool) float64 {
			for i := 0; i < 16; i++ {
				mid := (lo + hi) / 2
				if insideAt(b, mid) == leaving {
					lo = mid
				} else {
					hi = mid
				}
			}
			return (lo + hi) / 2
		}
		const steps = 48
		i := 1
		for i <= steps && insideAt(from, float64(i)/steps) {
			i++
		}
		if i > steps {
			continue
		}
		t0 := crossing(from, float64(i-1)/steps, float64(i)/steps, true)
		for i <= steps && !insideAt(to, float64(i)/steps) {
			i++
		}
		t1 := 1.0
		if i <= steps {
			t1 = crossing(to, float64(i-1)/steps, float64(i)/steps, false)
		}

		sx, sy := point(t0)
		ex, ey := point(t1)
		blossom := func(a, b, c float64) float64 {
			return (1-t0)*(1-t1)*a + ((1-t0)*t1+t0*(1-t1))*b + t0*t1*c
		}
		qx := blossom(from.cx, px, to.cx)
		qy := blossom(from.cy, py, to.cy)
		el := js.Hypot(ex-qx, ey-qy)
		if el == 0 || el != el {
			el = 1
		}
		ux := (ex - qx) / el
		uy := (ey - qy) / el
		head := tmpMarker("arrow_point", ex, ey, ux, uy, "var(--_l)")
		ex -= ux * markerTrim
		ey -= uy * markerTrim

		label := ""
		if transition.Label != "" {
			l := text.Layout(transition.Label, false, measurer, small, itemMaxWidth, 0)
			lx, ly := point((t0 + t1) / 2)
			away := (math.Abs(nx)*l.Width)/2 + (math.Abs(ny)*l.Height)/2 + 6
			half := l.Width/2 + 4
			tx := math.Min(math.Max(lx+nx*away, x0+half), math.Max(right-half, x0+half))
			label = svg.LabelSvg(l, tx, ly+ny*away, ` class="pele-edge-label" font-size="`+js.Num(small)+`"`, icons)
		}
		edges.WriteString(`<g class="pele-edge" data-id="` + transition.From + "-" + transition.To + `">` +
			`<path d="M` + svg.Num(sx) + "," + svg.Num(sy) + "Q" + svg.Num(qx) + "," + svg.Num(qy) + " " + svg.Num(ex) + "," + svg.Num(ey) +
			`" fill="none" stroke="var(--_l)"/>` + head + label + "</g>")
	}

	totalWidth := math.Ceil(math.Max(width, title.Width) + 2*pad)
	totalHeight := math.Ceil(height + titleHeight + 2*pad)
	content := svg.LabelSvg(title, x0+width/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil) +
		`<g class="pele-cynefin-regions">` + fills.String() + "</g>" +
		boundaries +
		ellipse +
		domains.String()
	if edges.Len() > 0 {
		content += `<g class="pele-edges">` + edges.String() + "</g>"
	}

	acc := svg.Accessible{AccTitle: model.AccTitle, AccDescr: model.AccDescr}
	return core.Rendered{
		SVG:    svg.Document("cynefin", totalWidth, totalHeight, size, options, acc, content),
		Width:  totalWidth,
		Height: totalHeight,
	}
}

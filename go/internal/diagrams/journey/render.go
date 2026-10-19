package journey

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
	padX      = 10
	padY      = 8
	minText   = 124
	maxText   = 200
	gap       = 12
	bandPad   = 6
	gutter    = 24
	level     = 26
	faceR     = 11
	dot       = 5
	dotStep   = 13
	legendGap = 20
)

const eyes = `<circle cx="-3.6" cy="-2.8" r="1.3"/><circle cx="3.6" cy="-2.8" r="1.3"/>`

var mouths = map[string]string{"happy": "M-5,2A6,6 0 0 0 5,2", "neutral": "M-4.5,4.5H4.5", "sad": "M-5,6.5A6,6 0 0 1 5,6.5"}

func face(x, y float64, mood string) string {
	return `<g class="pele-face pele-face-` + mood + `" transform="translate(` + svg.Num(x) + "," + svg.Num(y) + `)">` +
		`<circle r="11" fill="var(--_bg)" stroke="var(--_l)"/>` +
		`<g fill="var(--_l)">` + eyes + `</g>` +
		`<path d="` + mouths[mood] + `" fill="none" stroke="var(--_l)" stroke-linecap="round"/>` +
		"</g>"
}

func moodOf(score float64) string {
	if score > 3 {
		return "happy"
	}
	if score < 3 {
		return "sad"
	}
	return "neutral"
}

func named(names []string) []string {
	var out []string
	for _, n := range names {
		if n != "" {
			out = append(out, n)
		}
	}
	return out
}

func render(m *model, options *core.Options) core.Rendered {
	return common.TurnToFit(options, func(down bool) core.Rendered {
		if down {
			return vertical(m, options)
		}
		return draw(m, options)
	})
}

type legendItem struct {
	name    string
	label   text.Label
	x, y, h float64
}

type run struct {
	start, count int
	label        *text.Label
}

func draw(m *model, options *core.Options) core.Rendered {
	size := options.FontSizeOr(16)
	small := js.Round(size * 0.875)
	tiny := js.Round(size * 0.75)
	measurer := options.Measurer
	pad := options.PaddingOr(8)
	tasks := m.tasks
	n := len(tasks)

	// `Task: 5:` names an actor with no name; Mermaid gives it a dot and an empty legend row.
	names := named(actors(tasks))
	actor := map[string]int{}
	for i, name := range names {
		actor[name] = i
	}

	// Cards widen, up to a point, for a word that would not fit; past that the word is cut.
	textW := float64(minText)
	labels := make([]text.Label, n)
	widest := 0.0
	for i, t := range tasks {
		labels[i] = common.PlainLabel(t.task, measurer, small, textW, 0)
		widest = math.Max(widest, labels[i].Width)
	}
	if widest > textW {
		textW = math.Min(math.Ceil(widest), maxText)
		for i, t := range tasks {
			labels[i] = common.FitLabel(t.task, measurer, small, textW, 0)
		}
	}

	cardW := textW + 2*padX
	pitch := cardW + gap
	plotW := 0.0
	if n > 0 {
		plotW = float64(n)*pitch - gap
	}

	lineH := js.Round(small * 1.5)
	limit := math.Max(plotW, 240)
	var legend []legendItem
	legendW, legendH, lx := 0.0, 0.0, 0.0
	for _, name := range names {
		label := common.FitLabel(name, measurer, small, limit-2*dot-6, 0)
		w := 2*dot + 6 + label.Width
		if lx > 0 && lx+w > limit {
			lx = 0
			legendH += 4
		}
		y := legendH
		if lx > 0 {
			y = legend[len(legend)-1].y
		}
		h := math.Max(label.Height, lineH)
		legend = append(legend, legendItem{name, label, lx, y, h})
		lx += w
		legendW = math.Max(legendW, lx)
		legendH = math.Max(legendH, y+h)
		lx += legendGap
	}

	bodyW := 0.0
	if n > 0 {
		bodyW = gutter + plotW
	}
	title := common.FitLabel(m.title, measurer, size, math.Max(bodyW, 320), text.Bold)
	titleH := 0.0
	if title.Height > 0 {
		titleH = title.Height + 16
	}
	legendSpan := 0.0
	if n > 0 {
		legendSpan = gutter + legendW
	}
	width := math.Max(bodyW, math.Max(title.Width, legendSpan))
	left := pad + (width-bodyW)/2 + gutter
	top := pad + titleH

	// A band covers a run of tasks that share a section, as in Mermaid; a section with no tasks draws nothing.
	var runs []run
	bandH := 0.0
	for i := 0; i < n; {
		end := i + 1
		for end < n && tasks[end].section == tasks[i].section {
			end++
		}
		var label *text.Label
		if tasks[i].section != "" {
			l := common.FitLabel(tasks[i].section, measurer, small, float64(end-i)*pitch-gap-2*padX, text.Bold)
			label = &l
			bandH = math.Max(bandH, math.Max(l.Height, lineH)+2*bandPad)
		}
		runs = append(runs, run{i, end - i, label})
		i = end
	}

	textH := lineH
	for _, l := range labels {
		textH = math.Max(textH, l.Height)
	}
	dotsH := 0.0
	if len(names) > 0 {
		dotsH = 2*dot + 6
	}
	cardH := textH + dotsH + 2*padY
	cardsY := top
	if bandH > 0 {
		cardsY += bandH + bandPad
	}
	chartTop := cardsY + cardH + faceR + 16
	chartBottom := chartTop + 4*level
	right := left + plotW

	var body strings.Builder
	bottom := top
	if n > 0 {
		var grid, scale strings.Builder
		for score := 5; score >= 1; score-- {
			y := chartTop + float64(5-score)*level
			grid.WriteString("M" + svg.Num(left-6) + "," + svg.Num(y) + "H" + svg.Num(right))
			scale.WriteString(`<text x="` + svg.Num(left-12) + `" y="` + svg.Num(y+tiny*0.35) + `">` + strconv.Itoa(score) + "</text>")
		}
		body.WriteString(`<path class="pele-grid" d="` + grid.String() + `" fill="none" stroke="var(--_a)"/>` +
			`<g class="pele-scale" font-size="` + js.Num(tiny) + `" text-anchor="end" fill="var(--_m)">` + scale.String() + "</g>")

		var line, stems, faces strings.Builder
		cards := make([]string, n)
		for i, t := range tasks {
			x := left + float64(i)*pitch
			cx := x + cardW/2
			score := t.score
			lv := 3.0
			if !math.IsNaN(score) {
				lv = math.Min(5, math.Max(1, score))
			}
			y := chartTop + (5-lv)*level
			if i == 0 {
				line.WriteString("M")
			} else {
				line.WriteString("L")
			}
			line.WriteString(svg.Num(cx) + "," + svg.Num(y))
			stems.WriteString("M" + svg.Num(cx) + "," + svg.Num(cardsY+cardH) + "V" + svg.Num(y-faceR))
			faces.WriteString(face(cx, y, moodOf(score)))

			var dots strings.Builder
			people := named(t.people)
			step := math.Min(dotStep, (cardW-2*padX-2*dot)/math.Max(float64(len(people)-1), 1))
			for k, name := range people {
				dots.WriteString(`<circle class="pele-actor" cx="` + svg.Num(cx+(float64(k)-float64(len(people)-1)/2)*step) +
					`" cy="` + svg.Num(cardsY+cardH-padY-dot) + `" r="5" fill="` + svg.SeriesColor(actor[name]) + `" stroke="var(--_s)">` +
					"<title>" + svg.EscText(name) + "</title></circle>")
			}
			cards[i] = `<g class="pele-node pele-task" data-id="` + svg.EscText(js.Trim(t.task)) + `">` +
				`<rect x="` + svg.Num(x) + `" y="` + svg.Num(cardsY) + `" width="` + svg.Num(cardW) + `" height="` + svg.Num(cardH) + `" rx="` + svg.Radius + `" fill="var(--_s)" stroke="var(--_b)"/>` +
				svg.LabelSvg(labels[i], cx, cardsY+padY+textH/2, ` font-size="`+js.Num(small)+`"`, nil) +
				dots.String() + "</g>"
		}
		body.WriteString(`<path class="pele-stems" d="` + stems.String() + `" fill="none" stroke="var(--_b)" stroke-dasharray="2 3"/>`)
		if n > 1 {
			body.WriteString(`<path class="pele-edge" d="` + line.String() + `" fill="none" stroke="var(--_l)" stroke-linejoin="round"/>`)
		}
		body.WriteString(faces.String())

		for _, r := range runs {
			members := strings.Join(cards[r.start:r.start+r.count], "")
			if r.label == nil {
				body.WriteString(members)
				continue
			}
			x := left + float64(r.start)*pitch
			w := float64(r.count)*pitch - gap
			body.WriteString(`<g class="pele-cluster pele-section" data-id="` + svg.EscText(js.Trim(tasks[r.start].section)) + `">` +
				`<rect x="` + svg.Num(x) + `" y="` + svg.Num(top) + `" width="` + svg.Num(w) + `" height="` + svg.Num(bandH) + `" rx="` + svg.Radius + `" fill="var(--_a)"/>` +
				svg.LabelSvg(*r.label, x+w/2, top+bandH/2, ` font-size="`+js.Num(small)+`" font-weight="var(--_hw)"`, nil) +
				members + "</g>")
		}
		bottom = chartBottom + faceR

		if len(legend) > 0 {
			x0 := left + (plotW-legendW)/2
			y0 := bottom + 16
			var items strings.Builder
			for _, it := range legend {
				cy := y0 + it.y + it.h/2
				items.WriteString(`<g class="pele-legend-item" data-id="` + svg.EscText(it.name) + `">` +
					`<circle cx="` + svg.Num(x0+it.x+dot) + `" cy="` + svg.Num(cy) + `" r="5" fill="` + svg.SeriesColor(actor[it.name]) + `"/>` +
					svg.LabelSvg(it.label, x0+it.x+2*dot+6+it.label.Width/2, cy, "", nil) + "</g>")
			}
			body.WriteString(`<g class="pele-legend" font-size="` + js.Num(small) + `">` + items.String() + "</g>")
			bottom = y0 + legendH
		}
	}

	totalWidth := math.Ceil(width + 2*pad)
	totalHeight := math.Ceil(math.Max(bottom, pad) + pad)
	content := svg.LabelSvg(title, pad+width/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil) + body.String()
	return core.Rendered{SVG: svg.Document("journey", totalWidth, totalHeight, size, options, accessible(m), content), Width: totalWidth, Height: totalHeight}
}

func accessible(m *model) svg.Accessible {
	return svg.Accessible{AccTitle: m.accTitle, AccDescr: m.accDescr}
}

func vertical(m *model, options *core.Options) core.Rendered {
	size := options.FontSizeOr(16)
	small := js.Round(size * 0.875)
	measurer := options.Measurer
	pad := options.PaddingOr(8)
	maxWidth := 320.0
	if options.MaxWidth != nil {
		maxWidth = *options.MaxWidth
	}
	room := math.Max(140, maxWidth-2*pad)
	title := common.FitLabel(m.title, measurer, size, room, text.Bold)
	names := named(actors(m.tasks))
	actorOf := map[string]int{}
	for i, name := range names {
		actorOf[name] = i
	}
	y := pad
	if title.Height != 0 {
		y += title.Height + 16
	}
	section, started := "", false
	var body strings.Builder
	width := title.Width
	for index, t := range m.tasks {
		if !started || t.section != section {
			started, section = true, t.section
			head := common.FitLabel(section, measurer, size, room-16, text.Bold)
			if head.Height != 0 {
				body.WriteString(svg.LabelSvg(head, pad+8+head.Width/2, y+head.Height/2, ` class="pele-cluster-label" font-weight="var(--_hw)"`, nil))
				width = math.Max(width, head.Width+16)
				y += head.Height + 8
			}
		}
		label := common.FitLabel(t.task, measurer, small, room-72, 0)
		type person struct {
			name  string
			label text.Label
		}
		var people []person
		for _, name := range named(t.people) {
			people = append(people, person{name, common.FitLabel(name, measurer, small, room-44, 0)})
		}
		textH := math.Max(label.Height, 42)
		h := 2*padY + textH
		for _, p := range people {
			h += math.Max(16, p.label.Height) + 4
		}
		w := math.Max(room, label.Width+72)
		body.WriteString(`<g class="pele-node pele-journey-step" data-id="` + strconv.Itoa(index) + `"><rect x="` + svg.Num(pad) + `" y="` + svg.Num(y) +
			`" width="` + svg.Num(w) + `" height="` + svg.Num(h) + `" rx="` + svg.Radius + `" fill="var(--_s)" stroke="var(--_b)"/>`)
		body.WriteString(svg.LabelSvg(label, pad+padX+label.Width/2, y+padY+label.Height/2, ` class="pele-label"`, nil))
		cx := pad + w - 28
		body.WriteString(face(cx, y+padY+faceR, moodOf(t.score)))
		if !math.IsNaN(t.score) {
			body.WriteString(`<text class="pele-journey-score" x="` + svg.Num(cx) + `" y="` + svg.Num(y+padY+2*faceR+small) + `" font-size="` + js.Num(small) +
				`" text-anchor="middle">` + svg.Num(math.Min(5, math.Max(1, t.score))) + "</text>")
		}
		py := y + padY + textH + 4
		for _, p := range people {
			ph := math.Max(16, p.label.Height)
			body.WriteString(`<circle class="pele-actor" cx="` + svg.Num(pad+16) + `" cy="` + svg.Num(py+ph/2) + `" r="5" fill="` + svg.SeriesColor(actorOf[p.name]) + `"/>` +
				svg.LabelSvg(p.label, pad+28+p.label.Width/2, py+ph/2, ` class="pele-journey-actor" font-size="`+js.Num(small)+`"`, nil))
			py += ph + 4
		}
		body.WriteString("</g>")
		y += h
		if index+1 < len(m.tasks) {
			body.WriteString(`<path class="pele-edge" d="M` + svg.Num(pad+w/2) + "," + svg.Num(y) + `v16" fill="none" stroke="var(--_l)"/>`)
		}
		y += 16
		width = math.Max(width, w)
	}
	totalWidth := math.Ceil(width + 2*pad)
	last := 0.0
	if len(m.tasks) > 0 {
		last = 16
	}
	height := math.Ceil(math.Max(pad+title.Height, y-last) + pad)
	content := svg.LabelSvg(title, totalWidth/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil) + body.String()
	return core.Rendered{SVG: svg.Document("journey", totalWidth, height, size, options, accessible(m), content), Width: totalWidth, Height: height}
}

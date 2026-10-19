package timeline

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
	padX        = 12.0
	padY        = 8.0
	minText     = 152.0
	maxText     = 216.0
	eventTextTD = 264.0
	gap         = 20.0
	axis        = 20.0
	spine       = 14.0
	eventGap    = 8.0
	sectionGap  = 8.0
	tail        = 12.0
	onColor     = ` fill="var(--_bg)"`
	line        = ` fill="none" stroke="var(--_b)"`
)

type group struct {
	section int
	periods []int
}

func box(x, y, w, h float64, attrs string) string {
	return `<rect x="` + svg.Num(x) + `" y="` + svg.Num(y) + `" width="` + svg.Num(w) + `" height="` + svg.Num(h) + `" rx="` + svg.Radius + `"` + attrs + "/>"
}

func renderTimeline(model *Db, config *core.Object, options *core.Options) core.Rendered {
	if model.Direction == "TD" {
		return draw(model, config, options, true)
	}
	return common.TurnToFit(options, func(down bool) core.Rendered { return draw(model, config, options, down) })
}

func draw(model *Db, config *core.Object, options *core.Options, vertical bool) core.Rendered {
	size := options.FontSizeOr(16)
	small := js.Round(size * 0.875)
	measurer := options.Measurer
	pad := options.PaddingOr(8)
	disable, _ := config.Section("timeline").Get("disableMulticolor")
	multicolor := disable != true
	periods := model.Periods
	hasSections := len(model.Sections) > 0

	// Mermaid draws nothing for periods that come before the first section; here they lead, in a neutral color.
	groups := make([]*group, len(model.Sections))
	for section := range model.Sections {
		groups[section] = &group{section: section}
	}
	leading := &group{section: -1}
	for i, period := range periods {
		if period.SectionIndex >= 0 && period.SectionIndex < len(groups) {
			groups[period.SectionIndex].periods = append(groups[period.SectionIndex].periods, i)
		} else {
			leading.periods = append(leading.periods, i)
		}
	}
	if len(leading.periods) > 0 {
		groups = append([]*group{leading}, groups...)
	}

	// Columns widen, up to a point, for a word that would not fit; past that the word is cut.
	// Running down, the two columns share the width the host has.
	room := math.Inf(1)
	if vertical && options.MaxWidth != nil && *options.MaxWidth > 0 {
		room = *options.MaxWidth - 2*pad
	}
	taken := axis + spine*2 + 4*padX
	periodText := math.Min(minText, math.Max(80, js.Round((room-taken)*0.38)))
	eventText := math.Min(eventTextTD, math.Max(110, room-taken-periodText))
	textW := periodText
	eventTextW := func() float64 {
		if vertical {
			return eventText
		}
		return textW
	}
	layoutAll := func(label func(raw string, m text.Measurer, size, max float64, base int) text.Label) ([]text.Label, [][]text.Label) {
		periodLabels := make([]text.Label, len(periods))
		eventLabels := make([][]text.Label, len(periods))
		for i, p := range periods {
			periodLabels[i] = label(p.Text, measurer, size, textW, 0)
		}
		for i, p := range periods {
			labels := make([]text.Label, len(p.Events))
			for k, e := range p.Events {
				labels[k] = label(e, measurer, small, eventTextW(), 0)
			}
			eventLabels[i] = labels
		}
		return periodLabels, eventLabels
	}
	periodLabels, eventLabels := layoutAll(common.PlainLabel)
	widest := 0.0
	for _, label := range periodLabels {
		widest = math.Max(widest, label.Width)
	}
	if !vertical {
		for _, labels := range eventLabels {
			for _, label := range labels {
				widest = math.Max(widest, label.Width)
			}
		}
	}
	overflow := widest > textW
	if vertical {
		for _, labels := range eventLabels {
			for _, label := range labels {
				overflow = overflow || label.Width > eventText
			}
		}
	}
	if overflow {
		limit := periodText
		if math.IsInf(room, 1) {
			limit = maxText
		}
		textW = math.Min(math.Max(math.Ceil(widest), periodText), limit)
		periodLabels, eventLabels = layoutAll(common.FitLabel)
	}

	lineHeight := js.Round(size * 1.5)
	colW := textW + 2*padX
	eventW := eventTextW() + 2*padX
	chipHeight := func(label text.Label) float64 { return math.Max(label.Height, lineHeight) + 2*padY }
	eventHeight := func(label text.Label) float64 { return math.Max(label.Height, js.Round(small*1.5)) + 2*padY }
	colorOf := func(g *group, period int) string {
		if g.section >= 0 {
			return svg.SeriesColor(g.section)
		}
		if hasSections {
			return "var(--_m)"
		}
		if multicolor {
			return svg.SeriesColor(period)
		}
		return svg.SeriesColor(0)
	}

	hasEvents := false
	for _, labels := range eventLabels {
		if len(labels) > 0 {
			hasEvents = true
		}
	}
	slots := 0
	for _, g := range groups {
		slots += max(len(g.periods), 1)
	}

	pitch := colW + gap
	drawn := len(periods) > 0
	bodyW := 0.0
	if vertical {
		bodyW = colW
		if hasEvents {
			bodyW += axis + spine*2 + eventW
		} else if drawn {
			bodyW += axis + tail
		}
	} else if slots > 0 {
		bodyW = float64(slots)*pitch - gap
		if drawn {
			bodyW += tail + arrow
		}
	}
	title := common.FitLabel(model.Title, measurer, size, math.Max(bodyW, 320), text.Bold)
	titleH := 0.0
	if title.Height > 0 {
		titleH = title.Height + 16
	}
	width := math.Max(bodyW, title.Width)
	left := pad + (width-bodyW)/2
	top := pad + titleH

	sectionLabel := func(g *group, w float64) text.Label {
		return common.FitLabel(model.Sections[g.section], measurer, size, w-2*padX, text.Bold)
	}
	header := func(g *group, label text.Label, x, y, w, h float64) string {
		return box(x, y, w, h, ` fill="`+svg.SeriesColor(g.section)+`"`) +
			svg.LabelSvg(label, x+w/2, y+h/2, ` font-weight="var(--_hw)"`+onColor, nil)
	}
	period := func(i int, color string, x, y, h float64) string {
		return box(x, y, colW, h, ` fill="`+color+`"`) + svg.LabelSvg(periodLabels[i], x+colW/2, y+h/2, onColor, nil)
	}
	smallAttr := ` font-size="` + js.Num(small) + `"`
	event := func(label text.Label, color string, x, y, h float64) string {
		return `<g class="pele-node pele-event">` +
			box(x, y, eventW, h, ` fill="`+color+`" fill-opacity="0.16"`) +
			svg.LabelSvg(label, x+eventW/2, y+h/2, smallAttr, nil) +
			"</g>"
	}
	dot := func(x, y float64, color string) string {
		return `<circle class="pele-dot" cx="` + svg.Num(x) + `" cy="` + svg.Num(y) + `" r="4" fill="` + color + `" stroke="var(--_bg)" stroke-width="2"/>`
	}
	open := func(g *group) string {
		if g.section >= 0 {
			return `<g class="pele-cluster pele-section" data-id="` + svg.EscText(js.Trim(model.Sections[g.section])) + `">`
		}
		return ""
	}
	openPeriod := func(i int) string {
		return `<g class="pele-node pele-period" data-id="` + svg.EscText(js.Trim(periods[i].Text)) + `">`
	}

	var body strings.Builder
	var height float64

	if vertical {
		axisX := left + colW + axis
		spineX := axisX + spine
		eventsX := spineX + spine
		y := top
		for _, g := range groups {
			body.WriteString(open(g))
			if g.section >= 0 {
				label := sectionLabel(g, bodyW)
				h := chipHeight(label)
				body.WriteString(header(g, label, left, y, bodyW, h))
				y += h + sectionGap + 4
			}
			for _, i := range g.periods {
				color := colorOf(g, i)
				h := chipHeight(periodLabels[i])
				cy := y + h/2
				d := "M" + svg.Num(left+colW) + "," + svg.Num(cy) + "H" + svg.Num(axisX)
				cards := ""
				ey := y
				last := cy
				for _, label := range eventLabels[i] {
					eh := eventHeight(label)
					last = ey + eh/2
					d += "M" + svg.Num(spineX) + "," + svg.Num(last) + "H" + svg.Num(eventsX)
					cards += event(label, color, eventsX, ey, eh)
					ey += eh + eventGap
				}
				if cards != "" {
					d += "M" + svg.Num(axisX) + "," + svg.Num(cy) + "H" + svg.Num(spineX) + "V" + svg.Num(last)
				}
				body.WriteString(openPeriod(i) +
					period(i, color, left, y, h) +
					`<path class="pele-edge" d="` + d + `"` + line + "/>" +
					dot(axisX, cy, color) +
					cards +
					"</g>")
				y = math.Max(y+h, ey-eventGap) + gap
			}
			if len(g.periods) == 0 {
				y += gap - sectionGap - 4
			}
			if g.section >= 0 {
				body.WriteString("</g>")
			}
		}
		if len(groups) > 0 {
			y -= gap
		}
		content := body.String()
		if drawn {
			tip := y + tail + arrow
			content = `<path class="pele-axis" d="M` + svg.Num(axisX) + "," + svg.Num(top) + "V" + svg.Num(tip-arrow+1) + `" fill="none" stroke="var(--_l)"/>` +
				marker("arrow_point", axisX, tip, 0, 1, "var(--_l)") +
				content
			y = tip
		}
		body.Reset()
		body.WriteString(content)
		height = y + pad
	} else {
		headerH := 0.0
		sectionLabels := make([]*text.Label, len(groups))
		for k, g := range groups {
			if g.section < 0 {
				continue
			}
			label := sectionLabel(g, float64(max(len(g.periods), 1))*pitch-gap)
			headerH = math.Max(headerH, chipHeight(label))
			sectionLabels[k] = &label
		}
		chipH := 0.0
		for _, label := range periodLabels {
			chipH = math.Max(chipH, chipHeight(label))
		}
		periodsY := top
		if hasSections {
			periodsY += headerH + sectionGap
		}
		axisY := periodsY + chipH
		if chipH > 0 {
			axisY += axis
		}
		eventsY := axisY + axis
		bottom := top + headerH
		if drawn {
			bottom = axisY + tail
		}
		slot := 0
		for k, g := range groups {
			span := max(len(g.periods), 1)
			gx := left + float64(slot)*pitch
			body.WriteString(open(g))
			if label := sectionLabels[k]; label != nil {
				body.WriteString(header(g, *label, gx, top, float64(span)*pitch-gap, headerH))
			}
			for j, i := range g.periods {
				color := colorOf(g, i)
				x := gx + float64(j)*pitch
				cx := x + colW/2
				end := axisY
				if len(eventLabels[i]) > 0 {
					end = eventsY
				}
				d := "M" + svg.Num(cx) + "," + svg.Num(periodsY+chipH) + "V" + svg.Num(end)
				cards := ""
				ey := eventsY
				for _, eventLabel := range eventLabels[i] {
					eh := eventHeight(eventLabel)
					if ey > eventsY {
						d += "M" + svg.Num(cx) + "," + svg.Num(ey-eventGap) + "V" + svg.Num(ey)
					}
					cards += event(eventLabel, color, x, ey, eh)
					ey += eh + eventGap
				}
				bottom = math.Max(bottom, ey-eventGap)
				body.WriteString(openPeriod(i) +
					period(i, color, x, periodsY, chipH) +
					`<path class="pele-edge" d="` + d + `"` + line + "/>" +
					dot(cx, axisY, color) +
					cards +
					"</g>")
			}
			if g.section >= 0 {
				body.WriteString("</g>")
			}
			slot += span
		}
		if drawn {
			tip := left + bodyW
			content := `<path class="pele-axis" d="M` + svg.Num(left) + "," + svg.Num(axisY) + "H" + svg.Num(tip-arrow+1) + `" fill="none" stroke="var(--_l)"/>` +
				marker("arrow_point", tip, axisY, 1, 0, "var(--_l)") +
				body.String()
			body.Reset()
			body.WriteString(content)
		}
		height = bottom + pad
	}

	totalWidth := math.Ceil(width + 2*pad)
	totalHeight := math.Ceil(math.Max(height, pad*2))
	content := svg.LabelSvg(title, totalWidth/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil) + body.String()
	acc := svg.Accessible{AccTitle: model.AccTitle, AccDescr: model.AccDescr}
	return core.Rendered{
		SVG:    svg.Document("timeline", totalWidth, totalHeight, size, options, acc, content),
		Width:  totalWidth,
		Height: totalHeight,
		Links:  []core.LinkInfo{},
	}
}

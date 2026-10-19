package eventmodeling

import (
	"math"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
	"github.com/alessioalex/pele/go/internal/text"
)

// Series slot for each kind of entity. A UI has none and is drawn plain.
var seriesOf = map[EntityKind]int{
	"command":   0,
	"event":     1,
	"readmodel": 2,
	"processor": 6,
}

const (
	boxPadX     = 12
	boxPadY     = 10
	boxMinWidth = 96
	boxGap      = 16
	stagger     = 80
	lanePad     = 12
	laneGap     = 8
	dataColumns = 48
	noteColumns = 80
	cardGap     = 8
)

// row is a line of hardWrap. A cut through a surrogate pair leaves JavaScript a lone half,
// which Go strings cannot hold: text has U+FFFD in its place, as the SVG comes out once
// serialized, and measure has what measures the same (the whole character for a high half,
// nothing for a low one).
type row struct {
	text, measure string
}

type placed struct {
	box  *EmBox
	name text.Label
	data []row
	x, y float64
	w, h float64
}

type band struct {
	lane  *EmLane
	label text.Label
	right float64
	y, h  float64
}

// hardWrap splits text into lines of at most `columns` characters, keeping its own line breaks.
func hardWrap(raw string, columns int) []row {
	var out []row
	for _, line := range strings.Split(text.DecodeEntities(raw), "\n") {
		if js.Len(line) <= columns {
			out = append(out, row{line, line})
			continue
		}
		units := js.Units(line)
		for i := 0; i < len(units); i += columns {
			piece := units[i:min(i+columns, len(units))]
			display := js.FromUnits(piece)
			measure := display
			if n := len(piece); utf16.IsSurrogate(rune(piece[0])) || utf16.IsSurrogate(rune(piece[n-1])) {
				var b strings.Builder
				for k := 0; k < n; k++ {
					u := rune(piece[k])
					switch {
					case u >= 0xdc00 && u <= 0xdfff:
						// A low half: either the end of a pair, or nothing to measure.
					case u >= 0xd800 && u <= 0xdbff:
						if k+1 < n && piece[k+1] >= 0xdc00 && piece[k+1] <= 0xdfff {
							b.WriteRune(utf16.DecodeRune(u, rune(piece[k+1])))
							k++
						} else {
							b.WriteRune(0x10000)
						}
					default:
						b.WriteRune(u)
					}
				}
				measure = b.String()
			}
			out = append(out, row{display, measure})
		}
	}
	return out
}

func plainRows(lines []string) []row {
	out := make([]row, len(lines))
	for i, l := range lines {
		out[i] = row{l, l}
	}
	return out
}

func writeLines(rows []row, x, y, step float64, attrs string) string {
	var body strings.Builder
	for i, r := range rows {
		if r.text != "" {
			body.WriteString(`<tspan x="` + svg.Num(x) + `" y="` + svg.Num(y+float64(i)*step) + `">` + svg.Esc(r.text) + "</tspan>")
		}
	}
	if body.Len() == 0 {
		return ""
	}
	return "<text" + attrs + ` xml:space="preserve">` + body.String() + "</text>"
}

func renderEventModel(model *Model, config *core.Object, options *core.Options) core.Rendered {
	return common.TurnToFit(options, func(down bool) core.Rendered { return draw(model, config, options, down) })
}

func n(x float64) string { return js.NumberToString(x) }

func boxClass(box *EmBox) string {
	s := "pele-node pele-em-" + string(box.Kind)
	if box.Reset {
		s += " pele-em-reset"
	}
	return s
}

func draw(model *Model, _ *core.Object, options *core.Options, down bool) core.Rendered {
	if down {
		return verticalEventModel(model, options)
	}
	size := options.FontSizeOr(16)
	nameSize := js.Round(size * 0.875)
	small := js.Round(size * 0.8125)
	mono := js.Round(size * 0.75)
	monoStep := js.Round(mono * 1.4)
	smallStep := js.Round(small * 1.5)
	measurer := options.Measurer
	pad := options.PaddingOr(8)
	widest := func(rows []row, fontSize float64, style int) float64 {
		width := 0.0
		for _, r := range rows {
			width = math.Max(width, measurer.Width(r.measure, fontSize, style))
		}
		return width
	}

	bands := map[*EmLane]*band{}
	var bandOrder []*band
	labelColumn := 0.0
	for _, lane := range model.Lanes {
		label := text.Layout(lane.Label, false, measurer, small, 4000, text.Bold)
		labelColumn = math.Max(labelColumn, label.Width)
		b := &band{lane: lane, label: label, right: math.Inf(-1)}
		bands[lane] = b
		bandOrder = append(bandOrder, b)
	}
	labelColumn += 2 * lanePad

	// Time runs left to right. A frame in a new lane starts under the end of the one before it,
	// so the arrow between them can drop straight down; a frame in the same lane follows on.
	var all []*placed
	byBox := map[*EmBox]*placed{}
	var previous *placed
	for _, box := range model.Boxes {
		b := bands[box.Lane]
		name := text.Layout(box.Name, false, measurer, nameSize, 4000, text.Bold)
		var data []row
		if box.Data != "" {
			data = hardWrap(box.Data, dataColumns)
		}
		w := math.Max(boxMinWidth, math.Max(name.Width, widest(data, mono, text.Mono))+2*boxPadX)
		h := 2*boxPadY + name.Height
		if len(data) > 0 {
			h += 6 + float64(len(data))*monoStep
		}
		x := 0.0
		if previous != nil {
			stag := previous.x + math.Max(previous.w/2, previous.w-stagger)
			if previous.box.Lane == box.Lane {
				x = b.right + boxGap
			} else {
				x = math.Max(stag, b.right+boxGap)
			}
		}
		item := &placed{box: box, name: name, data: data, x: x, w: w, h: h}
		b.right = x + w
		b.h = math.Max(b.h, h)
		all = append(all, item)
		byBox[box] = item
		previous = item
	}

	contentRight := 0.0
	for _, item := range all {
		contentRight = math.Max(contentRight, item.x+item.w)
	}
	laneWidth := labelColumn + contentRight + lanePad

	title := text.Layout(model.Title, false, measurer, size, 4000, text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 12
	}
	left := pad
	top := pad + titleHeight

	y := top
	var lanes strings.Builder
	for _, b := range bandOrder {
		b.y = y
		b.h += 2 * lanePad
		lanes.WriteString(`<g class="pele-cluster pele-em-lane" data-id="` + svg.EscText(b.lane.Label) + `">` +
			`<rect x="` + svg.Num(left) + `" y="` + svg.Num(y) + `" width="` + svg.Num(laneWidth) + `" height="` + svg.Num(b.h) + `" rx="` + svg.Radius + `" fill="var(--_s)"/>` +
			svg.LabelSvg(b.label, left+lanePad+b.label.Width/2, y+b.h/2, ` class="pele-cluster-label" font-size="`+n(small)+`" font-weight="var(--_hw)" fill="var(--_m)"`, nil) +
			"</g>")
		y += b.h + laneGap
	}
	bottom := top
	if len(bandOrder) > 0 {
		bottom = y - laneGap
	}

	var nodes strings.Builder
	for _, item := range all {
		b := bands[item.box.Lane]
		item.x += left + labelColumn
		item.y = b.y + (b.h-item.h)/2
		series, hasSeries := seriesOf[item.box.Kind]
		frame := `x="` + svg.Num(item.x) + `" y="` + svg.Num(item.y) + `" width="` + svg.Num(item.w) + `" height="` + svg.Num(item.h) + `" rx="` + svg.Radius + `"`
		var shape string
		if !hasSeries {
			shape = "<rect " + frame + ` fill="var(--_bg)" stroke="var(--_b)"/>`
		} else {
			shape = "<rect " + frame + ` fill="var(--_bg)"/><rect ` + frame + ` fill="` + svg.SeriesColor(series) + `" fill-opacity="0.22" stroke="` + svg.SeriesColor(series) + `"/>`
		}
		nameY := item.y + boxPadY + item.name.Height/2
		body := svg.LabelSvg(item.name, item.x+item.w/2, nameY, ` class="pele-label" font-size="`+n(nameSize)+`" font-weight="var(--_hw)"`, nil) +
			writeLines(
				item.data,
				item.x+boxPadX,
				item.y+boxPadY+item.name.Height+6+monoStep/2+mono*0.35,
				monoStep,
				` class="pele-em-data" font-family="`+svg.FontMono+`" font-size="`+n(mono)+`"`,
			)
		nodes.WriteString(`<g class="` + boxClass(item.box) + `" data-id="` + svg.EscText(item.box.ID) + `">` + shape + body + "</g>")
	}

	var edges strings.Builder
	trim := markerTrim("arrow_point")
	for _, relation := range model.Relations {
		s := byBox[relation.Source]
		t := byBox[relation.Target]
		if s == nil || t == nil || s == t {
			continue
		}
		var d, head string
		if s.box.Lane == t.box.Lane {
			// Along the lane, from the side of one box to the side of the other.
			forward := t.x >= s.x
			sx, tx := s.x, t.x+t.w
			if forward {
				sx, tx = s.x+s.w, t.x
			}
			sy := s.y + s.h/2
			ty := t.y + t.h/2
			dir := -1.0
			if tx >= sx {
				dir = 1
			}
			d = "M" + svg.Num(sx) + "," + svg.Num(sy) + "L" + svg.Num(tx-dir*trim) + "," + svg.Num(ty)
			head = marker("arrow_point", tx, ty, dir, 0, "var(--_l)")
		} else {
			// Between lanes, straight down or up where the boxes overlap, else in an S.
			goingDown := t.y > s.y
			from := math.Max(s.x, t.x)
			to := math.Min(s.x+s.w, t.x+t.w)
			inset := func(w float64) float64 { return math.Min(24, w/4) }
			var sx, tx float64
			switch {
			case to-from >= 16:
				sx = (from + to) / 2
			case t.x > s.x:
				sx = s.x + s.w - inset(s.w)
			default:
				sx = s.x + inset(s.w)
			}
			switch {
			case to-from >= 16:
				tx = sx
			case t.x > s.x:
				tx = t.x + inset(t.w)
			default:
				tx = t.x + t.w - inset(t.w)
			}
			sy, ty, dir := s.y, t.y+t.h, -1.0
			if goingDown {
				sy, ty, dir = s.y+s.h, t.y, 1
			}
			end := ty - dir*trim
			mid := (sy + end) / 2
			if sx == tx {
				d = "M" + svg.Num(sx) + "," + svg.Num(sy) + "V" + svg.Num(end)
			} else {
				d = "M" + svg.Num(sx) + "," + svg.Num(sy) + "C" + svg.Num(sx) + "," + svg.Num(mid) + " " + svg.Num(tx) + "," + svg.Num(mid) + " " + svg.Num(tx) + "," + svg.Num(end)
			}
			head = marker("arrow_point", tx, ty, 0, dir, "var(--_l)")
		}
		edges.WriteString(`<g class="pele-edge" data-id="` + svg.EscText(relation.Source.ID) + "-" + svg.EscText(relation.Target.ID) + `"><path d="` + d + `" fill="none" stroke="var(--_l)"/>` + head + "</g>")
	}

	// Notes and given/when/then specifications, listed under the lanes with the frame they belong to.
	names := map[string]string{}
	for _, box := range model.Boxes {
		names[box.ID] = box.Name
	}
	var cards strings.Builder
	cardsRight := 0.0
	card := func(kind, heading, frame string, rows []row) {
		headText := heading + " · " + frame
		if name, ok := names[frame]; ok {
			headText += " " + name
		}
		head := text.Layout(headText, false, measurer, small, 4000, text.Bold)
		w := math.Max(head.Width, widest(rows, small, 0)) + 2*boxPadX
		h := 2*boxPadY + head.Height + float64(len(rows))*smallStep
		cy := bottom
		if !(cards.Len() == 0 && len(bandOrder) == 0) {
			cy += cardGap
		}
		cards.WriteString(`<g class="pele-em-` + kind + `" data-id="` + svg.EscText(frame) + `">` +
			`<rect x="` + svg.Num(left) + `" y="` + svg.Num(cy) + `" width="` + svg.Num(w) + `" height="` + svg.Num(h) + `" rx="` + svg.Radius + `" fill="var(--_bg)" stroke="var(--_b)"/>` +
			svg.LabelSvg(head, left+boxPadX+head.Width/2, cy+boxPadY+head.Height/2, ` font-size="`+n(small)+`" font-weight="var(--_hw)"`, nil) +
			writeLines(rows, left+boxPadX, cy+boxPadY+head.Height+smallStep/2+small*0.35, smallStep, ` font-size="`+n(small)+`"`) +
			"</g>")
		bottom = cy + h
		cardsRight = math.Max(cardsRight, w)
	}
	for _, note := range model.Notes {
		card("note", "Note", note.Frame, hardWrap(note.Text, noteColumns))
	}
	for _, spec := range model.Specifications {
		var rows []row
		for _, r := range specRows(spec) {
			rows = append(rows, hardWrap(r, noteColumns)...)
		}
		card("specification", "Given / When / Then", spec.Frame, rows)
	}

	width := math.Max(math.Max(0, cardsRight), title.Width)
	if len(bandOrder) > 0 {
		width = math.Max(math.Max(laneWidth, cardsRight), title.Width)
	}
	totalWidth := math.Ceil(width + 2*pad)
	totalHeight := math.Ceil(bottom + pad)
	content := svg.LabelSvg(title, totalWidth/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil) +
		lanes.String() + edges.String() + nodes.String() + cards.String()

	return core.Rendered{
		SVG:    svg.Document("eventmodeling", totalWidth, totalHeight, size, options, svg.Accessible{AccTitle: model.AccTitle, AccDescr: model.AccDescr}, content),
		Width:  totalWidth,
		Height: totalHeight,
	}
}

func specRows(spec EmSpecification) []string {
	rows := []string{"Given  " + strings.Join(spec.Given, ", ")}
	if len(spec.When) > 0 {
		rows = append(rows, "When  "+strings.Join(spec.When, ", "))
	}
	return append(rows, "Then  "+strings.Join(spec.Then, ", "))
}

const (
	railGap   = 8
	cardMin   = 120
	arrowRoom = 20
)

type vplaced struct {
	y, h, w float64
	index   int
	lane    float64
}

func labelRows(label text.Label) []row {
	out := make([]row, len(label.Lines))
	for i, line := range label.Lines {
		var b strings.Builder
		for _, span := range line {
			b.WriteString(span.Text)
		}
		out[i] = row{b.String(), b.String()}
	}
	return out
}

func verticalEventModel(model *Model, options *core.Options) core.Rendered {
	size := options.FontSizeOr(16)
	small := js.Round(size * 0.875)
	mono := js.Round(size * 0.75)
	pad := options.PaddingOr(8)
	measurer := options.Measurer
	maxWidth := 320.0
	if options.MaxWidth != nil {
		maxWidth = *options.MaxWidth
	}
	room := math.Max(160, maxWidth-2*pad)
	// A relation that skips a card runs down a rail beside the cards. Relations into one card
	// share a rail; others whose runs overlap take the next rail out, so that no two follow the
	// same line.
	order := map[*EmBox]int{}
	for i, box := range model.Boxes {
		order[box] = i
	}
	runs := map[*EmBox]*[2]int{}
	var runOrder []*EmBox
	// A relation between neighbouring cards passes the lower card's lane heading.
	passed := map[*EmBox]bool{}
	for _, relation := range model.Relations {
		s, okS := order[relation.Source]
		t, okT := order[relation.Target]
		if !okS || !okT || s == t {
			continue
		}
		if s-t < 2 && t-s < 2 {
			passed[model.Boxes[max(s, t)]] = true
			continue
		}
		if run := runs[relation.Target]; run != nil {
			run[0] = min(run[0], s)
			run[1] = max(run[1], s)
		} else {
			runs[relation.Target] = &[2]int{min(s, t), max(s, t)}
			runOrder = append(runOrder, relation.Target)
		}
	}
	railOf := map[*EmBox]int{}
	var railEnds []int
	sorted := slices.Clone(runOrder)
	slices.SortStableFunc(sorted, func(a, b *EmBox) int { return runs[a][0] - runs[b][0] })
	for _, target := range sorted {
		from, to := runs[target][0], runs[target][1]
		rail := -1
		for i, end := range railEnds {
			if end < from {
				rail = i
				break
			}
		}
		if rail < 0 {
			rail = len(railEnds)
			railEnds = append(railEnds, to)
		} else {
			railEnds[rail] = to
		}
		railOf[target] = rail
	}
	gutter := 24 + railGap*math.Max(0, float64(len(railEnds)-1))
	// Rails are never shared to save room: past what the width allows, the drawing is wider.
	cardWidth := math.Max(cardMin, room-gutter)
	textRoom := cardWidth - 2*boxPadX
	title := text.Layout(model.Title, false, measurer, size, room, text.Bold)
	y := pad
	if title.Height != 0 {
		y += title.Height + 12
	}
	bottom := pad + title.Height
	width := math.Max(title.Width, cardWidth+gutter)
	var nodes, edges, cards strings.Builder
	positions := map[*EmBox]*vplaced{}
	literal := func(raw string, fontSize float64, style int) text.Label {
		return common.LiteralLabel(text.DecodeEntities(raw), measurer, fontSize, textRoom, style)
	}
	for index, box := range model.Boxes {
		laneRoom := textRoom
		if passed[box] {
			laneRoom = textRoom - arrowRoom
		}
		lane := common.BreakLabel(text.Layout(box.Lane.Label, false, measurer, small, laneRoom, text.Bold), measurer, laneRoom)
		name := text.Layout(box.Name, false, measurer, small, textRoom-36, text.Bold)
		var data *text.Label
		if box.Data != "" {
			l := literal(box.Data, mono, text.Mono)
			data = &l
		}
		dataWidth := 0.0
		if data != nil {
			dataWidth = data.Width
		}
		w := max(cardWidth, lane.Width+2*boxPadX, name.Width+2*boxPadX+36, dataWidth+2*boxPadX)
		headH := lane.Height + 12
		h := 2*boxPadY + name.Height
		if data != nil {
			h += data.Height + 6
		}
		series, hasSeries := seriesOf[box.Kind]
		cy := y + headH
		frame := `x="` + svg.Num(pad) + `" y="` + svg.Num(cy) + `" width="` + svg.Num(w) + `" height="` + svg.Num(h) + `" rx="` + svg.Radius + `"`
		nodes.WriteString(`<g class="pele-cluster pele-em-lane" data-id="` + svg.EscText(box.Lane.Label) + `">` +
			common.LeftLabelSvg(lane, pad+boxPadX, y, ` class="pele-cluster-label" font-size="`+n(small)+`" font-weight="var(--_hw)" fill="var(--_m)"`) +
			`<g class="` + boxClass(box) + `" data-id="` + svg.EscText(box.ID) + `"><rect ` + frame + ` fill="var(--_bg)" stroke="var(--_b)"/>`)
		if hasSeries {
			nodes.WriteString("<rect " + frame + ` fill="` + svg.SeriesColor(series) + `" fill-opacity="0.22" stroke="` + svg.SeriesColor(series) + `"/>`)
		}
		nodes.WriteString(svg.LabelSvg(name, pad+boxPadX+name.Width/2, cy+boxPadY+name.Height/2, ` class="pele-label" font-size="`+n(small)+`" font-weight="var(--_hw)"`, nil) +
			`<text class="pele-em-frame" x="` + svg.Num(pad+w-boxPadX) + `" y="` + svg.Num(cy+boxPadY+small) + `" text-anchor="end" font-size="` + n(mono) + `" fill="var(--_m)">` + svg.Esc(box.ID) + "</text>")
		if data != nil {
			nodes.WriteString(writeLines(labelRows(*data), pad+boxPadX, cy+boxPadY+name.Height+6+data.LineHeight/2+mono*0.35, data.LineHeight, ` class="pele-em-data" font-family="`+svg.FontMono+`" font-size="`+n(mono)+`"`))
		}
		nodes.WriteString("</g></g>")
		positions[box] = &vplaced{y: cy, h: h, w: w, index: index, lane: lane.Width}
		width = math.Max(width, w+gutter)
		bottom = cy + h
		y = bottom + 28
	}
	for _, relation := range model.Relations {
		s := positions[relation.Source]
		t := positions[relation.Target]
		if s == nil || t == nil || s == t {
			continue
		}
		goingDown := t.y > s.y
		dir := -1.0
		sy, ty := s.y, t.y+t.h
		if goingDown {
			dir = 1
			sy, ty = s.y+s.h, t.y
		}
		var d, head string
		if s.index-t.index == 1 || t.index-s.index == 1 {
			// The lower card's lane heading sits in the gap, and wraps to leave the arrow room beside it.
			lower := s
			if goingDown {
				lower = t
			}
			cx := pad + math.Max(math.Min(s.w, t.w)/2, boxPadX+lower.lane+16)
			d = "M" + svg.Num(cx) + "," + svg.Num(sy) + "V" + svg.Num(ty-dir*markerTrim("arrow_point"))
			head = marker("arrow_point", cx, ty, 0, dir, "var(--_l)")
		} else {
			rail := pad + width - gutter + 12 + railGap*float64(railOf[relation.Target])
			// A card can send one relation and receive another: they leave above and arrive below its middle.
			from, to := s.y+s.h/2-5, t.y+t.h/2+5
			d = "M" + svg.Num(pad+s.w) + "," + svg.Num(from) + "H" + svg.Num(rail) + "V" + svg.Num(to) + "H" + svg.Num(pad+t.w+markerTrim("arrow_point"))
			head = marker("arrow_point", pad+t.w, to, -1, 0, "var(--_l)")
		}
		edges.WriteString(`<g class="pele-edge" data-id="` + svg.EscText(relation.Source.ID) + "-" + svg.EscText(relation.Target.ID) + `"><path d="` + d + `" fill="none" stroke="var(--_l)"/>` + head + "</g>")
	}
	names := map[string]string{}
	for _, box := range model.Boxes {
		names[box.ID] = box.Name
	}
	card := func(kind, heading, frame, raw string) {
		headText := heading + " · " + frame
		if name, ok := names[frame]; ok {
			headText += " " + name
		}
		head := text.Layout(headText, false, measurer, small, textRoom, text.Bold)
		body := literal(raw, small, 0)
		w := max(cardWidth, head.Width+2*boxPadX, body.Width+2*boxPadX)
		h := 2*boxPadY + head.Height + body.Height
		cards.WriteString(`<g class="pele-em-` + kind + `" data-id="` + svg.EscText(frame) + `"><rect x="` + svg.Num(pad) + `" y="` + svg.Num(y) + `" width="` + svg.Num(w) + `" height="` + svg.Num(h) + `" rx="` + svg.Radius + `" fill="var(--_bg)" stroke="var(--_b)"/>` +
			svg.LabelSvg(head, pad+boxPadX+head.Width/2, y+boxPadY+head.Height/2, ` font-size="`+n(small)+`" font-weight="var(--_hw)"`, nil) +
			writeLines(labelRows(body), pad+boxPadX, y+boxPadY+head.Height+body.LineHeight/2+small*0.35, body.LineHeight, ` font-size="`+n(small)+`"`) + "</g>")
		bottom = y + h
		y = bottom + cardGap
		width = math.Max(width, w)
	}
	for _, note := range model.Notes {
		card("note", "Note", note.Frame, note.Text)
	}
	for _, spec := range model.Specifications {
		card("specification", "Given / When / Then", spec.Frame, strings.Join(specRows(spec), "\n"))
	}
	totalWidth := math.Ceil(width + 2*pad)
	height := math.Ceil(bottom + pad)
	content := svg.LabelSvg(title, totalWidth/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil) + edges.String() + nodes.String() + cards.String()
	return core.Rendered{
		SVG:    svg.Document("eventmodeling", totalWidth, height, size, options, svg.Accessible{AccTitle: model.AccTitle, AccDescr: model.AccDescr}, content),
		Width:  totalWidth,
		Height: height,
	}
}

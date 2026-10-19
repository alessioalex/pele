package railroad

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
	kTerminal = iota
	kNonTerminal
	kSpecial
	kSequence
	kChoice
	kBypass
	kLoop
	kFolded
)

// A laid-out element. The track enters at the left edge and leaves at the right edge on one
// baseline, with up above it and down below it.
type box struct {
	kind            int
	width, up, down float64
	kids            []*box
	text            string // A leaf's text, or a loop's count.
	id              string // The name a non-terminal refers to.
	reach           float64
}

type metrics struct {
	measurer text.Measurer
	size     float64
	padding  float64
	height   float64
	// Space between elements in a sequence, and between stacked branches.
	gap float64
	sep float64
	arc float64
}

const (
	lead      = 12.0
	headGap   = 6.0
	ruleGap   = 18.0
	countSize = 12.0
	arrow     = 7.0
)

func option(config *core.Object, key string, fallback, max float64) float64 {
	value, ok := config.Section("railroad").Value(key).(float64)
	if ok && value >= 0 {
		return math.Min(value, max)
	}
	return fallback
}

// shown is the text drawn for a value. Text is literal here: a terminal such as "<br>" or "*" is
// itself, not markup. Mermaid's `#35;` codes still stand for characters, and escapes that would
// be invisible are written out.
func shown(s string) string {
	if strings.Contains(s, "¶ß") {
		s = text.DecodeEntities(strings.ReplaceAll(s, "&", "&amp;"))
	}
	if !strings.ContainsAny(s, "\n\r\t") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func reverse[T any](list []T, from int) {
	for i, j := from, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
}

func even(value float64) float64 { return math.Ceil(value/2) * 2 }

func leaf(kind int, s, id string, style int, m *metrics) *box {
	half := m.height / 2
	width := math.Max(even(m.measurer.Width(s, m.size, style)+2*m.padding), m.height)
	return &box{kind: kind, width: width, up: half, down: half, text: s, id: id}
}

func count(min, max float64) string {
	if max == inf && min <= 1 {
		return ""
	}
	str := func(value float64) string {
		if js.IsFinite(value) {
			return js.NumberToString(value)
		}
		return "∞"
	}
	if min == max {
		return str(min) + "×"
	}
	if max == inf {
		return "≥" + str(min) + "×"
	}
	if min == 0 {
		return "≤" + str(max) + "×"
	}
	return str(min) + "–" + str(max) + "×"
}

// below is where the next branch of a choice sits below the baseline, given the one above it.
func below(offset float64, above, branch *box, m *metrics) float64 {
	return math.Max(offset+above.down+m.sep+branch.up, 2*m.arc)
}

func bypass(kid *box, m *metrics) *box {
	reach := math.Max(kid.up+m.sep, 2*m.arc)
	return &box{kind: kBypass, width: kid.width + 4*m.arc, up: reach, down: kid.down, kids: []*box{kid}, reach: reach}
}

func rowOf(row []*box, width float64) *box {
	up, down := 0.0, 0.0
	for _, kid := range row {
		up = math.Max(up, kid.up)
		down = math.Max(down, kid.down)
	}
	return &box{kind: kSequence, width: width, up: up, down: down, kids: row}
}

func measure(root *Node, m *metrics, room float64) *box {
	budgets := map[*Node]float64{}
	type budget struct {
		node  *Node
		width float64
	}
	pending := []budget{{root, room}}
	for i := 0; room != inf && i < len(pending); i++ {
		node, width := pending[i].node, pending[i].width
		budgets[node] = width
		// A sequence that folds puts its rows inside rails, so its parts are sized for those rows.
		around := 0.0
		switch node.Type {
		case Choice, Optional:
			around = 4 * m.arc
		case Repetition:
			if node.Min == 0 {
				around = 6 * m.arc
			} else {
				around = 2 * m.arc
			}
		case Sequence:
			around = 6 * m.arc
		}
		for _, child := range node.Kids {
			pending = append(pending, budget{child, math.Max(40, width-around)})
		}
	}
	return fold(root, parts, func(node *Node, kids []*box) *box {
		room, ok := budgets[node]
		if !ok {
			room = inf
		}
		switch node.Type {
		case Terminal:
			s := shown(node.Text)
			if s != "" && strings.Trim(s, " ") == "" {
				s = strings.Repeat("␣", len(s))
			}
			return leaf(kTerminal, s, "", text.Mono, m)
		case Nonterminal:
			return leaf(kNonTerminal, shown(node.Text), node.Text, 0, m)
		case Special:
			return leaf(kSpecial, "? "+shown(node.Text)+" ?", "", text.Italic, m)
		case Sequence:
			width := math.Max(0, float64(len(kids)-1)) * m.gap
			up, down := 0.0, 0.0
			for _, kid := range kids {
				width += kid.width
				up = math.Max(up, kid.up)
				down = math.Max(down, kid.down)
			}
			seq := &box{kind: kSequence, width: width, up: up, down: down, kids: kids}
			if width <= room || len(kids) < 2 {
				return seq
			}
			var rows, row []*box
			rowW := 0.0
			for _, kid := range kids {
				if len(row) > 0 && rowW+m.gap+kid.width > math.Max(40, room-6*m.arc) {
					rows = append(rows, rowOf(row, rowW))
					row, rowW = nil, 0
				}
				gap := 0.0
				if len(row) > 0 {
					gap = m.gap
				}
				rowW += gap + kid.width
				row = append(row, kid)
			}
			rows = append(rows, rowOf(row, rowW))
			if len(rows) < 2 {
				return seq
			}
			foldedWidth, offset := 0.0, 0.0
			for i := range rows {
				foldedWidth = math.Max(foldedWidth, rows[i].width)
				if i > 0 {
					offset += rows[i-1].down + math.Max(m.sep, 2*m.arc) + rows[i].up
				}
			}
			return &box{kind: kFolded, width: foldedWidth + 6*m.arc, up: rows[0].up, down: offset + rows[len(rows)-1].down, kids: rows}
		case Choice:
			inner, offset := 0.0, 0.0
			for i := range kids {
				inner = math.Max(inner, kids[i].width)
				if i > 0 {
					offset = below(offset, kids[i-1], kids[i], m)
				}
			}
			up, down := 0.0, 0.0
			if len(kids) > 0 {
				up = kids[0].up
				down = offset + kids[len(kids)-1].down
			}
			return &box{kind: kChoice, width: inner + 4*m.arc, up: up, down: down, kids: kids}
		case Optional:
			return bypass(kids[0], m)
		default:
			min, max := node.Min, node.Max
			kid := kids[0]
			if min == 1 && max == 1 {
				return kid
			}
			s := count(min, max)
			reach := math.Max(kid.down+m.sep, 2*m.arc)
			label, extra := 0.0, arrow/2+1
			if s != "" {
				label, extra = even(m.measurer.Width(s, countSize, 0)), countSize+6
			}
			loop := &box{kind: kLoop, width: math.Max(kid.width, label) + 2*m.arc, up: kid.up, down: reach + extra, kids: []*box{kid}, text: s, reach: reach}
			if min == 0 {
				return bypass(loop, m)
			}
			return loop
		}
	})
}

func renderRailroad(model *Model, config *core.Object, options *core.Options) core.Rendered {
	return common.Tighten(options, func(tight bool) core.Rendered { return draw(model, config, options, tight) })
}

func draw(model *Model, config *core.Object, options *core.Options, tight bool) core.Rendered {
	num := svg.Num
	base := options.FontSizeOr(16)
	fontSize := option(config, "fontSize", 0, 200)
	size := fontSize
	headSize := fontSize
	if fontSize == 0 {
		size = js.Round(base * 0.875)
		headSize = base
	}
	padding := option(config, "padding", -1, 200)
	lineHeight := js.Round(size * 1.5)
	m := &metrics{measurer: options.Measurer, size: size, padding: padding, sep: option(config, "verticalSeparation", 10, 500)}
	if padding < 0 {
		m.padding = 12
		m.height = even(lineHeight + 9)
	} else {
		m.height = even(lineHeight + 2*padding)
	}
	m.gap = option(config, "horizontalSeparation", 16, 500)
	m.arc = option(config, "arcRadius", 10, 200)
	if tight {
		m.gap = math.Min(m.gap, 12)
		m.arc = math.Min(m.arc, 6)
	}
	radius := 0.0
	if show, ok := config.Section("railroad").Value("showMarkers").(bool); !ok || show {
		radius = option(config, "markerRadius", 4, 50)
	}
	strokeWidth := option(config, "strokeWidth", 0, 20)
	stroke := ""
	if strokeWidth > 0 {
		stroke = ` stroke-width="` + num(strokeWidth) + `"`
	}
	pad := options.PaddingOr(8)
	arc := m.arc

	A := num(arc)
	turn := func(sweep, dx, dy string) string {
		return "a" + A + "," + A + " 0 0 " + sweep + " " + dx + A + "," + dy + A
	}
	eastToSouth := turn("1", "", "")
	southToEast := turn("0", "", "")
	eastToNorth := turn("0", "", "-")
	northToEast := turn("1", "", "-")
	loopDown := turn("0", "-", "")
	loopUp := turn("0", "-", "-")

	titleWidth := 4000.0
	if tight {
		titleWidth = common.TitleRoom(options)
	}
	title := text.Layout(model.Title, false, m.measurer, base, titleWidth, text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 16
	}
	headHeight := js.Round(headSize * 1.5)
	textShift := size * 0.35
	boxTop := m.height / 2

	var track, arrows, nodes strings.Builder

	line := func(x1, y, x2 float64) {
		if x2 > x1 {
			track.WriteString("M" + num(x1) + "," + num(y) + "H" + num(x2))
		}
	}

	place := func(root *box, left, baseline float64) {
		boxes := []*box{root}
		at := []float64{left, baseline}
		for len(boxes) > 0 {
			bx := boxes[len(boxes)-1]
			boxes = boxes[:len(boxes)-1]
			y := at[len(at)-1]
			x := at[len(at)-2]
			at = at[:len(at)-2]
			kids := bx.kids
			right := x + bx.width
			switch bx.kind {
			case kSequence:
				kx := right
				for i := len(kids) - 1; i >= 0; i-- {
					kx -= kids[i].width
					boxes = append(boxes, kids[i])
					at = append(at, kx, y)
					if i > 0 {
						line(kx-m.gap, y, kx)
					}
					kx -= m.gap
				}
			case kFolded:
				start := x + 2*arc
				from := len(boxes)
				offset := 0.0
				line(x, y, start)
				for i, kid := range kids {
					ky := y + offset
					// Pushed y first and reversed below, which also puts each pair in order.
					boxes = append(boxes, kid)
					at = append(at, ky, start)
					if i+1 < len(kids) {
						next := kids[i+1]
						nextOffset := offset + kid.down + math.Max(m.sep, 2*arc) + next.up
						mid := y + offset + kid.down + math.Max(m.sep, 2*arc)/2
						track.WriteString("M" + num(start+kid.width) + "," + num(ky) + "H" + num(right-2*arc) + "V" + num(mid) +
							"H" + num(x+arc) + "V" + num(y+nextOffset) + "H" + num(start))
						arrows.WriteString("M" + num(start-arrow) + "," + num(y+nextOffset-arrow/2) + "l7,3.5l-7,3.5z")
						offset = nextOffset
					} else {
						track.WriteString("M" + num(start+kid.width) + "," + num(ky) + "H" + num(right-arc) + "V" + num(y) + "H" + num(right))
					}
				}
				reverse(boxes, from)
				reverse(at, 2*from)
			case kChoice:
				inner := bx.width - 4*arc
				railIn := x + arc
				railOut := right - arc
				from := len(boxes)
				offset := 0.0
				for i, kid := range kids {
					if i > 0 {
						offset = below(offset, kids[i-1], kid, m)
					}
					ky := y + offset
					kx := x + 2*arc + (inner-kid.width)/2
					last := i == len(kids)-1
					if i == 0 {
						line(x, y, kx)
						line(kx+kid.width, y, right)
					} else {
						if last {
							track.WriteString("M" + num(x) + "," + num(y) + eastToSouth + "V" + num(ky-arc))
						} else {
							track.WriteString("M" + num(railIn) + "," + num(ky-arc))
						}
						track.WriteString(southToEast)
						if kx > railIn+arc {
							track.WriteString("H" + num(kx))
						}
						track.WriteString("M" + num(kx+kid.width) + "," + num(ky))
						if kx > railIn+arc {
							track.WriteString("H" + num(railOut-arc))
						}
						track.WriteString(eastToNorth)
						if last {
							track.WriteString("V" + num(y+arc) + northToEast)
						}
					}
					boxes = append(boxes, kid)
					at = append(at, ky, kx)
				}
				// Reversed, so that the branches come off the stack top to bottom.
				reverse(boxes, from)
				reverse(at, 2*from)
			case kBypass:
				kid := kids[0]
				kx := x + 2*arc
				line(x, y, kx)
				line(kx+kid.width, y, right)
				track.WriteString("M" + num(x) + "," + num(y) + eastToNorth + "V" + num(y-bx.reach+arc) + northToEast + "H" + num(right-2*arc) +
					eastToSouth + "V" + num(y-arc) + southToEast)
				boxes = append(boxes, kid)
				at = append(at, kx, y)
			case kLoop:
				kid := kids[0]
				kx := x + (bx.width-kid.width)/2
				loopY := y + bx.reach
				middle := x + bx.width/2
				line(x, y, kx)
				line(kx+kid.width, y, right)
				track.WriteString("M" + num(x+arc) + "," + num(y) + loopDown + "V" + num(loopY-arc) + southToEast + "H" + num(right-arc) +
					eastToNorth + "V" + num(y+arc) + loopUp)
				arrows.WriteString("M" + num(middle-arrow/2) + "," + num(loopY) + "l7,-3.5v7z")
				if bx.text != "" {
					nodes.WriteString(`<text class="pele-edge-label pele-count" x="` + num(middle) + `" y="` + num(loopY+countSize+3) +
						`" font-size="12" fill="var(--_m)">` + svg.Esc(bx.text) + "</text>")
				}
				boxes = append(boxes, kid)
				at = append(at, kx, y)
			default:
				s := bx.text
				space := ""
				if strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") || strings.Contains(s, "  ") {
					space = ` xml:space="preserve"`
				}
				shape := `<rect x="` + num(x) + `" y="` + num(y-boxTop) + `" width="` + num(bx.width) + `" height="` + num(m.height) + `"`
				label := `<text x="` + num(x+bx.width/2) + `" y="` + num(y+textShift) + `"` + space
				switch bx.kind {
				case kTerminal:
					nodes.WriteString(`<g class="pele-node pele-terminal">` + shape + ` rx="` + num(boxTop) + `" fill="var(--_s)" stroke="var(--_b)"` + stroke + "/>" +
						label + ` font-family="` + svg.FontMono + `">` + svg.Esc(s) + "</text></g>")
				case kNonTerminal:
					nodes.WriteString(`<g class="pele-node pele-nonterminal" data-id="` + svg.EscText(bx.id) + `">` + shape + ` rx="` + svg.Radius +
						`" fill="var(--_bg)" stroke="var(--_b)"` + stroke + "/>" + label + ">" + svg.Esc(s) + "</text></g>")
				default:
					nodes.WriteString(`<g class="pele-node pele-special">` + shape + ` rx="` + svg.Radius + `" fill="var(--_bg)" stroke="var(--_b)" stroke-dasharray="4 3"` + stroke + "/>" +
						label + ` font-style="italic"><tspan fill="var(--_m)">? </tspan>` + svg.Esc(s[2:len(s)-2]) + `<tspan fill="var(--_m)"> ?</tspan></text></g>`)
				}
			}
		}
	}

	var rules strings.Builder
	width := title.Width
	top := pad + titleHeight
	bottom := pad + title.Height
	for _, rule := range model.Rules {
		room := inf
		if tight {
			maxWidth := 320.0
			if options.MaxWidth != nil {
				maxWidth = *options.MaxWidth
			}
			room = math.Max(80, maxWidth-2*pad-4*radius-2*lead-1)
		}
		bx := measure(rule.Definition, m, room)
		name := shown(rule.Name)
		baseline := top + headHeight + headGap + bx.up
		start := pad + 2*radius
		left := start + lead
		end := left + bx.width + lead
		track.Reset()
		arrows.Reset()
		nodes.Reset()
		line(start, baseline, left)
		line(left+bx.width, baseline, end)
		place(bx, left, baseline)
		rules.WriteString(`<g class="pele-rule" data-id="` + svg.EscText(rule.Name) + `">` +
			`<text class="pele-rule-name" x="` + num(pad) + `" y="` + num(top+headHeight/2+headSize*0.35) + `" font-size="` + num(headSize) +
			`" font-weight="var(--_hw)" text-anchor="start">` + svg.Esc(name) + "</text>" +
			`<path class="pele-edge pele-track" d="` + track.String() + `" fill="none" stroke="var(--_l)"` + stroke + "/>")
		if arrows.Len() > 0 {
			rules.WriteString(`<path class="pele-marker" d="` + arrows.String() + `" fill="var(--_l)"/>`)
		}
		if radius > 0 {
			rules.WriteString(`<circle class="pele-marker pele-start" cx="` + num(pad+radius) + `" cy="` + num(baseline) + `" r="` + num(radius) + `" fill="var(--_l)"/>` +
				`<circle class="pele-marker pele-end" cx="` + num(end+radius) + `" cy="` + num(baseline) + `" r="` + num(radius) + `" fill="var(--_l)"/>`)
		}
		rules.WriteString(nodes.String())
		rules.WriteString("</g>")
		width = max(width, end+2*radius-pad, m.measurer.Width(name, headSize, text.Bold))
		bottom = baseline + bx.down
		top = bottom + m.sep + ruleGap
	}

	totalWidth := math.Ceil(width + 2*pad)
	totalHeight := math.Ceil(bottom + pad)
	content := svg.LabelSvg(title, totalWidth/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil)
	if rules.Len() > 0 {
		content += `<g class="pele-rules" transform="translate(0.5,0.5)" font-size="` + num(size) + `" text-anchor="middle">` + rules.String() + "</g>"
	}
	acc := svg.Accessible{AccTitle: model.AccTitle, AccDescr: model.AccDescr}
	return core.Rendered{
		SVG:    svg.Document("railroad", totalWidth, totalHeight, base, options, acc, content),
		Width:  totalWidth,
		Height: totalHeight,
		Links:  []core.LinkInfo{},
	}
}

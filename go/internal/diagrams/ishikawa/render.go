package ishikawa

import (
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
	"github.com/alessioalex/pele/go/internal/text"
)

// How far a bone moves toward the tail for each unit it rises from the spine.
const slant = 0.5

var length = js.Hypot(slant, 1)

const (
	firstRow = 14.0
	rowGap   = 6.0
	boneEnd  = 8.0
	twigStep = 22.0
	stub     = 18.0
	textGap  = 5.0
	pairGap  = 26.0
	headGap  = 34.0
	tailLen  = 28.0
	boxPadX  = 10.0
	boxPadY  = 5.0
)

// A cause below a category. Bones at even depths run level, from the slanted bone above them
// toward the tail; bones at odd depths slant away from the spine, from the level bone above them.
type bone struct {
	label  text.Label
	depth  int
	parent int
	// Its place among its siblings, counted from the spine outward.
	slot int
	kids int
	// How far its row is from the spine.
	rise float64
	// Where it meets its parent, measured level from the category's bone: 0 on it, negative toward the tail.
	at float64
	// Where its text ends.
	textEnd float64
}

type category struct {
	node  *Node
	label text.Label
	boxW  float64
	boxH  float64
	bones []bone
	// The length of its bone, as distance from the spine.
	rise float64
	// How far its causes reach toward the tail, measured level from its bone.
	reach float64
}

type frame struct {
	node   *Node
	depth  int
	parent int
	slot   int
	// Set when the bone was made and only its row is still to come.
	bone int
}

// arrange gives every cause of a category a row of its own, so that no two texts can meet, and
// measures how far the whole of it reaches. A level bone takes the row nearest the spine and its
// twigs the rows beyond; a slanted twig takes the row beyond everything that hangs off it. The
// outline may be nested as deep as its indentation goes, so the walk keeps a stack of its own.
func arrange(node *Node, up bool, measurer text.Measurer, size, small float64) *category {
	label := text.Layout(node.Text, false, measurer, size, 180, 0)
	lineHeight := js.Round(small * 1.5)
	var bones []bone
	var rows []int
	var stack []frame
	// Above the spine the first cause goes farthest out, so that the causes read from the top down.
	push := func(children []*Node, depth, parent int) {
		n := len(children)
		for slot := n - 1; slot >= 0; slot-- {
			child := children[slot]
			if up {
				child = children[n-1-slot]
			}
			stack = append(stack, frame{node: child, depth: depth, parent: parent, slot: slot, bone: -1})
		}
	}
	push(node.Children, 2, -1)
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if f.bone != -1 {
			rows = append(rows, f.bone)
			continue
		}
		index := len(bones)
		bones = append(bones, bone{
			label:  text.Layout(f.node.Text, false, measurer, small, 200, 0),
			depth:  f.depth,
			parent: f.parent,
			slot:   f.slot,
			kids:   len(f.node.Children),
		})
		if f.depth%2 == 0 {
			rows = append(rows, index)
		} else {
			f.bone = index
			stack = append(stack, f)
		}
		push(f.node.Children, f.depth+1, index)
	}

	rise := firstRow
	for _, index := range rows {
		height := math.Max(bones[index].label.Height, lineHeight) + rowGap
		bones[index].rise = rise + height/2
		rise += height
	}
	boxW := label.Width + 2*boxPadX
	reach := boxW / 2
	for k := range bones {
		b := &bones[k]
		if b.depth%2 == 0 {
			if b.parent != -1 {
				b.at = bones[b.parent].at
			}
			if b.kids > 0 {
				b.textEnd = b.at - float64(b.kids+1)*twigStep - textGap
			} else {
				b.textEnd = b.at - stub - textGap
			}
		} else {
			// Twigs stand along their level bone, the one nearest the spine nearest the tail.
			parent := &bones[b.parent]
			b.at = parent.at - float64(parent.kids-b.slot)*twigStep
			b.textEnd = b.at - textGap
		}
		reach = math.Max(reach, b.label.Width-b.textEnd)
	}
	return &category{node: node, label: label, boxW: boxW, boxH: label.Height + 2*boxPadY, bones: bones, rise: rise + boneEnd, reach: reach}
}

func renderIshikawa(model *Db, config *core.Object, options *core.Options) core.Rendered {
	return common.TurnToFit(options, func(down bool) core.Rendered { return draw(model, config, options, down) })
}

func draw(model *Db, _ *core.Object, options *core.Options, down bool) core.Rendered {
	size := options.FontSizeOr(16)
	small := js.Round(size * 0.875)
	measurer := options.Measurer
	pad := options.PaddingOr(8)
	if down {
		return verticalCauses(model, options)
	}
	root := model.Root
	title := text.Layout(model.Title, false, measurer, size, 4000, text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 12
	}

	body := ""
	minX, maxX, minY, maxY := 0.0, 0.0, 0.0, 0.0
	if root != nil {
		head := text.Layout(root.Text, false, measurer, size, 200, text.Bold)
		headH := math.Max(head.Height+24, 48)
		headW := head.Width + 24 + headH/2
		categories := make([]*category, len(root.Children))
		for i, node := range root.Children {
			categories[i] = arrange(node, i%2 == 0, measurer, size, small)
		}
		// Even a category without causes has a bone long enough to carry its name clear of the head.
		for _, c := range categories {
			c.rise = math.Max(c.rise, headH/2+14)
		}

		// Categories meet the spine in pairs, one above and one below, each pair as far toward the
		// tail from the one before as the wider of the two needs.
		var xs []float64
		x := -headGap
		for i := 0; i < len(categories); i += 2 {
			if i > 0 {
				room := 0.0
				for side := 0; side < 2; side++ {
					before := categories[i-2+side]
					next := 0.0
					if i+side < len(categories) {
						next = categories[i+side].boxW / 2
					}
					room = math.Max(room, before.reach+next)
				}
				x -= room + pairGap
			}
			xs = append(xs, x, x)
		}
		tail := -headGap
		if len(categories) > 0 {
			tail = x - tailLen
		}

		maxX = headW
		minX = tail
		minY = -headH / 2
		maxY = headH / 2
		var groups strings.Builder
		for i, c := range categories {
			sign := 1.0
			if i%2 == 0 {
				sign = -1
			}
			ax := xs[i]
			px := func(at, rise float64) float64 { return ax + at - rise*slant }
			endX := px(0, c.rise)
			endY := sign * c.rise
			// The bone stops short of the spine to leave its arrowhead a clean point.
			trim := arrow - 1
			var d strings.Builder
			d.WriteString("M" + svg.Num(ax-(slant/length)*trim) + "," + svg.Num((sign*trim)/length) + "L" + svg.Num(endX) + "," + svg.Num(endY))
			var texts strings.Builder
			for _, b := range c.bones {
				y := sign * b.rise
				if b.depth%2 == 0 {
					d.WriteString("M" + svg.Num(px(b.at, b.rise)) + "," + svg.Num(y) + "H" + svg.Num(px(b.textEnd+textGap, b.rise)))
				} else {
					from := c.bones[b.parent].rise
					d.WriteString("M" + svg.Num(px(b.at, from)) + "," + svg.Num(sign*from) + "L" + svg.Num(px(b.at, b.rise)) + "," + svg.Num(y))
				}
				right := px(b.textEnd, b.rise)
				attrs := ""
				if b.depth > 2 {
					attrs = ` fill="var(--_m)"`
				}
				texts.WriteString(svg.LabelSvg(b.label, right-b.label.Width/2, y, attrs, nil))
				minX = math.Min(minX, right-b.label.Width)
			}
			boxY := endY + sign*(c.boxH/2)
			minX = math.Min(minX, endX-c.boxW/2)
			maxX = math.Max(maxX, endX+c.boxW/2)
			minY = math.Min(minY, boxY-c.boxH/2)
			maxY = math.Max(maxY, boxY+c.boxH/2)
			groups.WriteString(`<g class="pele-ishikawa-category" data-id="` + svg.EscText(c.node.Text) + `">` +
				`<path class="pele-ishikawa-bones" d="` + d.String() + `" fill="none" stroke="var(--_l)"/>` +
				marker("arrow_point", ax, 0, slant/length, -sign/length, "var(--_l)") +
				`<rect x="` + svg.Num(endX-c.boxW/2) + `" y="` + svg.Num(boxY-c.boxH/2) + `" width="` + svg.Num(c.boxW) + `" height="` + svg.Num(c.boxH) +
				`" rx="` + svg.Radius + `" fill="var(--_s)" stroke="var(--_b)"/>` +
				svg.LabelSvg(c.label, endX, boxY, ` class="pele-label"`, nil))
			if texts.Len() > 0 {
				groups.WriteString(`<g class="pele-ishikawa-causes" font-size="` + js.Num(small) + `">` + texts.String() + "</g>")
			}
			groups.WriteString("</g>")
		}

		r := headH / 2
		body = `<path class="pele-ishikawa-spine" d="M` + svg.Num(tail) + ",0H" + svg.Num(-(arrow - 1)) + `" fill="none" stroke="var(--_l)"/>` +
			marker("arrow_point", 0, 0, 1, 0, "var(--_l)") +
			groups.String() +
			`<g class="pele-ishikawa-head" data-id="` + svg.EscText(root.Text) + `">` +
			`<path d="M0,` + svg.Num(-r) + "H" + svg.Num(headW-r) + "A" + svg.Num(r) + "," + svg.Num(r) + " 0 0 1 " + svg.Num(headW-r) + "," + svg.Num(r) +
			`H0Z" fill="var(--_s)" stroke="var(--_b)" stroke-linejoin="round"/>` +
			svg.LabelSvg(head, 12+head.Width/2, 0, ` class="pele-label" font-weight="var(--_hw)"`, nil) +
			"</g>"
	}

	inner := math.Max(maxX-minX, title.Width)
	width := math.Ceil(inner + 2*pad)
	height := math.Ceil(maxY - minY + titleHeight + 2*pad)
	ox := pad + (inner-(maxX-minX))/2 - minX
	oy := pad + titleHeight - minY
	content := svg.LabelSvg(title, width/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil)
	if body != "" {
		content += `<g transform="translate(` + svg.Num(ox) + "," + svg.Num(oy) + `)">` + body + "</g>"
	}
	return core.Rendered{SVG: svg.Document("ishikawa", width, height, size, options, svg.Accessible{}, content), Width: width, Height: height, Links: []core.LinkInfo{}}
}

type outlineEntry struct {
	node          *Node
	depth, parent int
	x, y, w, h    float64
	label         text.Label
}

func verticalCauses(model *Db, options *core.Options) core.Rendered {
	size := options.FontSizeOr(16)
	small := js.Round(size * 0.875)
	measurer := options.Measurer
	pad := options.PaddingOr(8)
	maxWidth := 320.0
	if options.MaxWidth != nil {
		maxWidth = *options.MaxWidth
	}
	room := math.Max(140, maxWidth-2*pad)
	title := text.Layout(model.Title, false, measurer, size, common.TitleRoom(options), text.Bold)
	var order []*outlineEntry
	type pendingEntry struct {
		node          *Node
		depth, parent int
	}
	var pending []pendingEntry
	if model.Root != nil {
		pending = append(pending, pendingEntry{model.Root, 0, -1})
	}
	depth := 0
	for len(pending) > 0 {
		e := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		index := len(order)
		order = append(order, &outlineEntry{node: e.node, depth: e.depth, parent: e.parent})
		depth = max(depth, e.depth)
		for i := len(e.node.Children) - 1; i >= 0; i-- {
			pending = append(pending, pendingEntry{e.node.Children[i], e.depth + 1, index})
		}
	}
	indent := math.Min(20, math.Max(2, (room-120)/float64(max(1, depth))))
	y := pad
	if title.Height != 0 {
		y += title.Height + 12
	}
	width := title.Width
	var nodes, edges strings.Builder
	for _, e := range order {
		e.x = pad + float64(e.depth)*indent
		labelRoom := math.Max(60, room-float64(e.depth)*indent-20)
		labelSize, style := small, 0
		if e.depth < 2 {
			labelSize, style = size, text.Bold
		}
		e.label = common.BreakLabel(text.Layout(e.node.Text, false, measurer, labelSize, labelRoom, style), measurer, labelRoom)
		e.w = e.label.Width + 20
		e.h = e.label.Height + 12
		e.y = y
		y += e.h + 12
		width = math.Max(width, e.x-pad+e.w)
		class := "pele-ishikawa-cause"
		switch e.depth {
		case 0:
			class = "pele-ishikawa-head"
		case 1:
			class = "pele-ishikawa-category"
		}
		nodes.WriteString(`<g class="pele-node ` + class + `" data-id="` + svg.EscText(e.node.Text) + `">`)
		if e.depth < 2 {
			rx := "4"
			if e.depth == 0 {
				rx = "12"
			}
			nodes.WriteString(`<rect x="` + svg.Num(e.x) + `" y="` + svg.Num(e.y) + `" width="` + svg.Num(e.w) + `" height="` + svg.Num(e.h) + `" rx="` + rx + `" fill="var(--_s)" stroke="var(--_b)"/>`)
		}
		attrs := ` class="pele-label"`
		if e.depth < 2 {
			attrs += ` font-weight="var(--_hw)"`
		} else {
			attrs += ` font-size="` + js.Num(small) + `"`
		}
		nodes.WriteString(svg.LabelSvg(e.label, e.x+e.w/2, e.y+e.h/2, attrs, nil) + "</g>")
		if e.parent >= 0 {
			parent := order[e.parent]
			gx := parent.x + indent/2
			edges.WriteString(`<path class="pele-edge" d="M` + svg.Num(gx) + "," + svg.Num(parent.y+parent.h) + "V" + svg.Num(e.y+e.h/2) + "H" + svg.Num(e.x) + `" fill="none" stroke="var(--_b)"/>`)
		}
	}
	totalWidth := math.Ceil(width + 2*pad)
	height := math.Ceil(math.Max(pad+title.Height, y-12) + pad)
	content := svg.LabelSvg(title, totalWidth/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil) + edges.String() + nodes.String()
	return core.Rendered{SVG: svg.Document("ishikawa", totalWidth, height, size, options, svg.Accessible{}, content), Width: totalWidth, Height: height, Links: []core.LinkInfo{}}
}

package mindmap

import (
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
	"github.com/alessioalex/pele/go/internal/text"
)

// Indexed by Mermaid's node type. A node without delimiters is text on a line, except the root.
var shapeNames = []string{"", "rounded", "rect", "circle", "cloud", "bang", "hex"}

const rootShape = "stadium"

const (
	gapX = 36.0
	gapY = 8.0
	// Between two subtrees, of which at least one has children of its own.
	groupGap   = 18.0
	branchGapX = 52.0
	branchGapY = 28.0
	textPad    = 6.0
	pillPad    = 12.0
	tint       = "0.22"
	iconGap    = 6.0
	// Running down, each level is set in by this much, and links leave a node this far from its left edge.
	indent = 28.0
	stem   = 12.0
	bend   = 8.0
	rowGap = 10.0
)

// leftLabel writes a label with each line starting at left, for an outline, where centered
// lines would look ragged.
func leftLabel(label text.Label, left, cy float64, icons text.IconResolver) string {
	if len(label.Lines) < 2 {
		return svg.LabelSvg(label, left+label.Width/2, cy, ` class="pele-label"`, icons)
	}
	var out strings.Builder
	for k := range label.Lines {
		width := label.Widths[k]
		line := text.Label{Lines: [][]text.Span{label.Lines[k]}, Widths: []float64{width}, Width: width, Height: label.LineHeight, Size: label.Size, LineHeight: label.LineHeight}
		out.WriteString(svg.LabelSvg(line, left+width/2, cy+(float64(k)-float64(len(label.Lines)-1)/2)*label.LineHeight, ` class="pele-label"`, icons))
	}
	return out.String()
}

func renderMindmap(model *Db, config *core.Object, options *core.Options) core.Rendered {
	return common.TurnToFit(options, func(down bool) core.Rendered { return draw(model, config, options, down) })
}

// draw with down set draws the map as an outline: the root at the top, and each level set in
// under its parent. It is far narrower than branches spreading to both sides, and as tall as it
// needs to be.
func draw(model *Db, config *core.Object, options *core.Options, down bool) core.Rendered {
	size := options.FontSizeOr(16)
	measurer := options.Measurer
	pad := options.PaddingOr(8)
	icons := options.Icons
	wrap := 200.0
	if v, ok := config.Section("mindmap").Value("maxNodeWidth").(float64); ok && v > 0 {
		wrap = math.Min(v, 2000)
	}
	glyph := size * 1.25

	nodes := model.Nodes
	n := len(nodes)
	parent := make([]int, n)
	depth := make([]int, n)
	for i := 0; i < n; i++ {
		for _, child := range nodes[i].Children {
			parent[child.ID] = i
			depth[child.ID] = depth[i] + 1
		}
	}
	// An outline has the width the host gives it, less what its level is set in.
	room := math.Inf(1)
	if down && options.MaxWidth != nil && *options.MaxWidth > 0 {
		room = *options.MaxWidth - 2*pad
	}
	labels := make([]text.Label, n)
	shapes := make([]string, n)
	w := make([]float64, n)
	h := make([]float64, n)
	dy := make([]float64, n)
	// Where links meet the node, measured down from its top: its middle.
	anchor := make([]float64, n)
	// Whether the host has the node's icon. One it does not have takes no room.
	drawn := make([]bool, n)

	for i := 0; i < n; i++ {
		node := nodes[i]
		drawn[i] = node.Icon != "" && text.IconMarkup(measurer, node.Icon) != ""
		iconRoom := 0.0
		if drawn[i] {
			iconRoom = glyph + iconGap
		}
		fit := math.Min(wrap, math.Max(72, room-float64(depth[i])*indent-2*textPad-iconRoom))
		label := text.Layout(node.Descr, true, measurer, size, fit, 0)
		labels[i] = label
		tw := label.Width
		if drawn[i] {
			if label.Width > 0 {
				tw += glyph + iconGap
			} else {
				tw += glyph
			}
		}
		th := math.Max(label.Height, js.Round(size*1.5))
		shape := ""
		if node.Type >= 0 && node.Type < len(shapeNames) {
			shape = shapeNames[node.Type]
		}
		if shape == "" && i == 0 {
			shape = rootShape
		}
		// In an outline the root heads a column, and the trunk drops from under its left end. A circle
		// or a cloud would stand wide of that, so the root is a pill there unless it is a box already.
		if down && i == 0 && shape != "rect" && shape != "rounded" {
			shape = rootShape
		}
		shapes[i] = shape
		if shape == "" {
			// A node with no shape of its own is plain text when it ends a branch, and text in a
			// tinted pill when the branch goes on from it, which sets the levels apart.
			if len(node.Children) > 0 {
				w[i] = tw + 2*pillPad
				h[i] = th + 6
			} else {
				w[i] = tw + 2*textPad
				h[i] = th
			}
			anchor[i] = h[i] / 2
		} else {
			s := svg.ShapeSize(shape, tw, th)
			w[i] = s.W
			h[i] = s.H
			dy[i] = s.Dy
			anchor[i] = s.H / 2
		}
	}

	// Each subtree takes a band of height `extent`. `rel` is a subtree's offset among its siblings,
	// `own` the node's offset inside its band, and `kids` the offset of its children's bands.
	extent := make([]float64, n)
	rel := make([]float64, n)
	own := make([]float64, n)
	kids := make([]float64, n)
	stacked := 0.0

	// Stacks subtrees top to bottom and returns the height at which their parent's anchor belongs.
	stack := func(list []*Node, from, to int, gap float64) float64 {
		y := 0.0
		last := 0.0
		for k := from; k < to; k++ {
			c := list[k].ID
			rel[c] = y
			grouped := len(list[k].Children) > 0 || (k+1 < to && len(list[k+1].Children) > 0)
			if grouped {
				last = math.Max(gap, groupGap)
			} else {
				last = gap
			}
			y += extent[c] + last
		}
		stacked = y - last
		a := list[from].ID
		b := list[to-1].ID
		return (rel[a] + own[a] + anchor[a] + rel[b] + own[b] + anchor[b]) / 2
	}

	// Nodes are in depth-first order, so going backwards visits children before their parent.
	for i := n - 1; i > 0; i-- {
		children := nodes[i].Children
		if len(children) == 0 {
			extent[i] = h[i]
			continue
		}
		top := stack(children, 0, len(children), gapY) - anchor[i]
		low := math.Min(0, top)
		extent[i] = math.Max(stacked, top+h[i]) - low
		own[i] = top - low
		kids[i] = -low
	}

	x := make([]float64, n)
	y := make([]float64, n)
	band := make([]float64, n)
	side := make([]float64, n)
	// Running down: how far the trunk under each node has been drawn.
	trunk := make([]float64, n)

	if down {
		// Nodes are in depth-first order, which is the order of an outline's rows.
		row := 0.0
		for i := 0; i < n; i++ {
			if i > 0 && parent[i] == 0 {
				row += rowGap
			}
			x[i] = float64(depth[i])*indent + w[i]/2
			y[i] = row
			row += h[i] + rowGap
		}
	} else if n > 0 {
		// The first branches go right and the rest left, split where the two sides are closest in height.
		branches := nodes[0].Children
		count := len(branches)
		total := 0.0
		for _, branch := range branches {
			total += extent[branch.ID]
		}
		split := count
		best := math.Inf(1)
		right := 0.0
		for k := 1; k < count; k++ {
			right += extent[branches[k-1].ID]
			diff := math.Abs(2*right - total + float64(2*k-count)*branchGapY)
			if diff < best {
				best = diff
				split = k
			}
		}
		for from, to, dir := 0, split, 1.0; from < count; from, to, dir = to, count, -1 {
			middle := stack(branches, from, to, branchGapY)
			for k := from; k < to; k++ {
				c := branches[k].ID
				band[c] = rel[c] - middle
				side[c] = dir
			}
		}
		y[0] = -h[0] / 2
		for i := 0; i < n; i++ {
			if i > 0 {
				y[i] = band[i] + own[i]
			}
			for _, child := range nodes[i].Children {
				c := child.ID
				g := branchGapX
				if i > 0 {
					side[c] = side[i]
					band[c] = band[i] + kids[i] + rel[c]
					g = gapX
				}
				x[c] = x[i] + side[c]*(w[i]/2+g+w[c]/2)
			}
		}
	}

	minX, maxX, minY, maxY := 0.0, 0.0, 0.0, 0.0
	var out, edges, body strings.Builder
	color := ""
	flush := func(section int) {
		if body.Len() == 0 {
			return
		}
		out.WriteString(`<g class="pele-branch pele-section-` + js.Num(float64(section)) + `">` +
			`<g class="pele-edges" fill="none" stroke="` + color + `" stroke-width="1.5">` + edges.String() + "</g>" + body.String() + "</g>")
		edges.Reset()
		body.Reset()
	}

	// The root is drawn last, on top of the links that leave it.
	for k := 1; k <= n; k++ {
		i := k % n
		node := nodes[i]
		shape := shapes[i]
		label := labels[i]
		cx := x[i]
		top := y[i]
		minX = math.Min(minX, cx-w[i]/2)
		maxX = math.Max(maxX, cx+w[i]/2)
		minY = math.Min(minY, top)
		maxY = math.Max(maxY, top+h[i])

		if i == 0 || parent[i] == 0 {
			flush(nodes[k-1].Section)
		}
		if i > 0 && down {
			if parent[i] == 0 {
				color = svg.SeriesColor(node.Section)
			}
			p := parent[i]
			// The link drops from under its parent and turns in to the node's left. It starts where
			// the link to the sibling above turned in, so each stretch of a shared trunk has one color.
			// Under the root's rounded end, the trunk starts where that end has come down to the bottom.
			s := stem
			if p == 0 {
				s = math.Min(math.Max(stem, h[0]/2), indent-bend)
			}
			x1 := x[p] - w[p]/2 + math.Min(s, w[p]/2)
			y1 := trunk[p]
			if y1 != y1 || y1 == 0 {
				y1 = y[p] + h[p]
			}
			left := cx - w[i]/2
			x2 := left
			if shape != "" {
				x2 += svg.ShapeInset(shape, w[i], h[i], 3, 0)
			}
			y2 := top + anchor[i]
			b := math.Max(0, math.Min(bend, math.Min(x2-x1, y2-y1)))
			// The next link carries on from where this one leaves the trunk, so the trunk has no gap.
			trunk[p] = y2 - b
			edges.WriteString(`<path class="pele-edge" d="M` + svg.Num(x1) + "," + svg.Num(y1) + "V" + svg.Num(y2-b) + "Q" + svg.Num(x1) + "," + svg.Num(y2) + " " +
				svg.Num(x1+b) + "," + svg.Num(y2) + "H" + svg.Num(x2) + `"/>`)
		} else if i > 0 {
			if parent[i] == 0 {
				color = svg.SeriesColor(node.Section)
			}
			p := parent[i]
			dir := side[i]
			from := 3
			if dir > 0 {
				from = 1
			}
			inset := 0.0
			if shapes[p] != "" {
				inset = svg.ShapeInset(shapes[p], w[p], h[p], from, 0)
			}
			x1 := x[p] + dir*(w[p]/2-inset)
			y1 := y[p] + anchor[p]
			near := cx - (dir*w[i])/2
			x2 := near
			if shape != "" {
				x2 += dir * svg.ShapeInset(shape, w[i], h[i], 4-from, 0)
			}
			y2 := top + anchor[i]
			mid := svg.Num((x1 + x2) / 2)
			edges.WriteString(`<path class="pele-edge" d="M` + svg.Num(x1) + "," + svg.Num(y1) + "C" + mid + "," + svg.Num(y1) + " " + mid + "," + svg.Num(y2) + " " +
				svg.Num(x2) + "," + svg.Num(y2) + `"/>`)
		}

		pill := shape == "" && len(node.Children) > 0
		// Every node off the root is filled with a tint of its branch's color and outlined in it,
		// whether its shape was given in the source or is the pill of a node that branches.
		// The root is neutral.
		tinted := ` fill="` + color + `" fill-opacity="` + tint + `" stroke="` + color + `"`
		inner := ""
		if shape != "" {
			if i > 0 {
				inner = svg.DrawShape(shape, w[i], h[i], tinted, ` stroke="`+color+`"`)
			} else {
				inner = svg.DrawShape(shape, w[i], h[i], ` fill="var(--_s)" stroke="var(--_b)"`, ` stroke="var(--_b)"`)
			}
		} else if pill {
			inner = `<rect x="` + svg.Num(-w[i]/2) + `" y="` + svg.Num(-h[i]/2) + `" width="` + svg.Num(w[i]) + `" height="` + svg.Num(h[i]) + `" rx="` + svg.Num(h[i]/2) + `"` + tinted + "/>"
		}
		shift := 0.0
		if node.Icon != "" && drawn[i] {
			g := glyph
			if label.Width > 0 {
				g += iconGap
			}
			shift = g / 2
			inner += common.IconSvg(node.Icon, -shift-label.Width/2, dy[i]-glyph/2, glyph, icons)
		}
		kind := shape
		if kind == "" {
			kind = "text"
			if pill {
				kind = "branch"
			}
		}
		rootClass := ""
		if i == 0 {
			rootClass = " pele-root"
		}
		var labelMarkup string
		if down && shape == "" {
			labelMarkup = leftLabel(label, shift-label.Width/2, dy[i], icons)
		} else {
			labelMarkup = svg.LabelSvg(label, shift, dy[i], ` class="pele-label"`, icons)
		}
		group := `<g class="pele-node pele-shape-` + kind + rootClass + svg.ClassNames(node.Class) + `" data-id="` + svg.EscText(node.NodeID) +
			`" transform="translate(` + svg.Num(cx) + "," + svg.Num(top+h[i]/2) + `)">` + inner + labelMarkup + "</g>"
		if i > 0 {
			body.WriteString(group)
		} else {
			out.WriteString(group)
		}
	}

	title := text.Layout(model.Title, false, measurer, size, common.TitleRoom(options), text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 16
	}
	width := math.Max(maxX-minX, title.Width)
	totalWidth := math.Ceil(width + 2*pad)
	totalHeight := math.Ceil(maxY - minY + titleHeight + 2*pad)
	content := svg.LabelSvg(title, totalWidth/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil) +
		`<g transform="translate(` + svg.Num(pad-minX+(width-maxX+minX)/2) + "," + svg.Num(pad+titleHeight-minY) + `)">` + out.String() + "</g>"

	return core.Rendered{
		SVG:    svg.Document("mindmap", totalWidth, totalHeight, size, options, svg.Accessible{}, content),
		Width:  totalWidth,
		Height: totalHeight,
		Links:  []core.LinkInfo{},
	}
}

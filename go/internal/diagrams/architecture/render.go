package architecture

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

const (
	stub     = 12
	corner   = 6
	labelGap = 4
	dotR     = 3
)

var (
	dirX = [4]float64{-1, 1, 0, 0}
	dirY = [4]float64{0, 0, -1, 1}
)

func setting(config *core.Object, key string, fallback, min, max float64) float64 {
	if v, ok := config.Section("architecture").Value(key).(float64); ok && js.IsFinite(v) {
		return math.Min(max, math.Max(min, v))
	}
	return fallback
}

// rank maps the grid lines in use to consecutive indexes, so empty rows and columns take no room.
func rank(values []int) map[int]int {
	slices.Sort(values)
	out := map[int]int{}
	for _, v := range values {
		if _, ok := out[v]; !ok {
			out[v] = len(out)
		}
	}
	return out
}

func iconSlot(name string, x, y, side float64, icons text.IconResolver) string {
	inner, ok := "", false
	if icons != nil {
		inner, ok = icons(name)
	}
	if !ok {
		inner = builtinIcon(name)
	}
	return `<svg class="pele-icon" data-icon="` + svg.EscText(name) + `" x="` + svg.Num(x) + `" y="` + svg.Num(y) + `" width="` + svg.Num(side) + `" height="` + svg.Num(side) + `"` +
		` viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + inner + "</svg>"
}

// fit cuts a label down to a box, ending what is left with an ellipsis.
func fit(label text.Label, maxWidth float64, maxLines int, m text.Measurer) text.Label {
	cut := len(label.Lines) > maxLines
	if !cut && label.Width <= maxWidth {
		return label
	}
	dots := m.Width("…", label.Size, 0)
	var lines [][]text.Span
	var widths []float64
	widest := 0.0
	for i := 0; i < min(maxLines, len(label.Lines)); i++ {
		line := append([]text.Span(nil), label.Lines[i]...)
		width := label.Widths[i]
		if width > maxWidth || (cut && i == maxLines-1) {
			for {
				if len(line) == 0 {
					line = append(line, text.Span{Text: "…", Width: dots})
					width = dots
					break
				}
				span := &line[len(line)-1]
				rest := width - span.Width
				if span.Icon == "" && rest+dots < maxWidth {
					units := js.Units(span.Text)
					lo, hi := 0, len(units)
					for lo < hi {
						mid := (lo + hi + 1) >> 1
						if rest+m.Width(js.FromUnits(units[:mid]), label.Size, span.Style)+dots <= maxWidth {
							lo = mid
						} else {
							hi = mid - 1
						}
					}
					if lo > 0 && units[lo-1] >= 0xd800 && units[lo-1] <= 0xdbff {
						lo--
					}
					span.Text = js.TrimEnd(js.FromUnits(units[:lo])) + "…"
					span.Width = m.Width(span.Text, label.Size, span.Style)
					width = rest + span.Width
					break
				}
				line = line[:len(line)-1]
				width = rest
			}
		}
		lines = append(lines, line)
		widths = append(widths, width)
		widest = math.Max(widest, width)
	}
	out := label
	out.Lines, out.Widths, out.Width, out.Height = lines, widths, widest, float64(len(lines))*label.LineHeight
	return out
}

type port struct {
	x, y, dx, dy float64
	// The box the route has to stay out of when it cannot leave straight.
	x0, y0, x1, y1 float64
	// How far short of the port an arrowhead stops: the radius of a junction's dot.
	gap float64
	// Set where a junction is reached through the border of its group: the line carries on to the junction.
	wire bool
}

// route is an orthogonal route from one port to another, each left along its side's normal.
func route(s, e *port) []float64 {
	sx, sy, sdx, sdy := s.x, s.y, s.dx, s.dy
	ex, ey, edx, edy := e.x, e.y, e.dx, e.dy
	s1x, s1y := sx+sdx*stub, sy+sdy*stub
	e1x, e1y := ex+edx*stub, ey+edy*stub
	if sdx*edx+sdy*edy == 0 {
		cx, cy := sx, ey
		if sdx != 0 {
			cx, cy = ex, sy
		}
		if (cx-sx)*sdx+(cy-sy)*sdy > 0 && (cx-ex)*edx+(cy-ey)*edy > 0 {
			return []float64{sx, sy, cx, cy, ex, ey}
		}
		if sdx != 0 {
			return []float64{sx, sy, s1x, s1y, s1x, e1y, e1x, e1y, ex, ey}
		}
		return []float64{sx, sy, s1x, s1y, e1x, s1y, e1x, e1y, ex, ey}
	}
	sideways := sdx != 0
	offset := ex - sx
	if sideways {
		offset = ey - sy
	}
	if sdx == -edx && sdy == -edy {
		ahead := (ex-sx)*sdx + (ey-sy)*sdy
		if ahead > 0 {
			if math.Abs(offset) < 0.5 {
				return []float64{sx, sy, ex, ey}
			}
			// Turn midway between the two boxes, which is clear of both titles, when there is such a place.
			if sideways {
				lo, hi := e.x1, s.x0
				if sdx > 0 {
					lo, hi = s.x1, e.x0
				}
				mx := (sx + ex) / 2
				if lo < hi {
					mx = (lo + hi) / 2
				}
				return []float64{sx, sy, mx, sy, mx, ey, ex, ey}
			}
			lo, hi := e.y1, s.y0
			if sdy > 0 {
				lo, hi = s.y1, e.y0
			}
			my := (sy + ey) / 2
			if lo < hi {
				my = (lo + hi) / 2
			}
			return []float64{sx, sy, sx, my, ex, my, ex, ey}
		}
	} else if math.Abs(offset) >= 1 {
		if sideways {
			x := math.Min(sx, ex) - stub
			if sdx > 0 {
				x = math.Max(sx, ex) + stub
			}
			return []float64{sx, sy, x, sy, x, ey, ex, ey}
		}
		y := math.Min(sy, ey) - stub
		if sdy > 0 {
			y = math.Max(sy, ey) + stub
		}
		return []float64{sx, sy, sx, y, ex, y, ex, ey}
	}
	// The straight way is blocked by one of the two nodes: go between them, or around both.
	if sideways {
		lane := math.Max(s.y1, e.y1) + stub
		if s.y1+stub <= e.y0 {
			lane = (s.y1 + e.y0) / 2
		} else if e.y1+stub <= s.y0 {
			lane = (e.y1 + s.y0) / 2
		}
		return []float64{sx, sy, s1x, sy, s1x, lane, e1x, lane, e1x, ey, ex, ey}
	}
	lane := math.Max(s.x1, e.x1) + stub
	if s.x1+stub <= e.x0 {
		lane = (s.x1 + e.x0) / 2
	} else if e.x1+stub <= s.x0 {
		lane = (e.x1 + s.x0) / 2
	}
	return []float64{sx, sy, sx, s1y, lane, s1y, lane, e1y, ex, e1y, ex, ey}
}

// simplify drops repeated points and joins runs that continue in a straight line.
func simplify(p []float64) []float64 {
	out := []float64{p[0], p[1]}
	for i := 2; i < len(p); i += 2 {
		x, y := p[i], p[i+1]
		n := len(out)
		sameX := math.Abs(x-out[n-2]) < 0.01
		sameY := math.Abs(y-out[n-1]) < 0.01
		if sameX && sameY {
			continue
		}
		if n >= 4 && ((sameX && math.Abs(out[n-4]-out[n-2]) < 0.01) || (sameY && math.Abs(out[n-3]-out[n-1]) < 0.01)) {
			out[n-2], out[n-1] = x, y
		} else {
			out = append(out, x, y)
		}
	}
	return out
}

func pathData(p []float64, startTrim, endTrim float64) string {
	n := len(p) / 2
	xs := make([]float64, n)
	ys := make([]float64, n)
	for i := 0; i < n; i++ {
		xs[i], ys[i] = p[i*2], p[i*2+1]
	}
	trim := func(at, toward int, amount float64) {
		l := js.Hypot(xs[toward]-xs[at], ys[toward]-ys[at])
		t := math.Min(amount, math.Max(0, l-1))
		if l > 0 && t > 0 {
			xs[at] += ((xs[toward] - xs[at]) / l) * t
			ys[at] += ((ys[toward] - ys[at]) / l) * t
		}
	}
	trim(0, 1, startTrim)
	trim(n-1, n-2, endTrim)
	var d strings.Builder
	d.WriteString("M" + svg.Num(xs[0]) + "," + svg.Num(ys[0]))
	for i := 1; i < n-1; i++ {
		before := js.Hypot(xs[i]-xs[i-1], ys[i]-ys[i-1])
		after := js.Hypot(xs[i+1]-xs[i], ys[i+1]-ys[i])
		r := math.Min(corner, math.Min(before/2, after/2))
		if r < 0.5 {
			d.WriteString("L" + svg.Num(xs[i]) + "," + svg.Num(ys[i]))
			continue
		}
		inX := xs[i] - ((xs[i]-xs[i-1])/before)*r
		inY := ys[i] - ((ys[i]-ys[i-1])/before)*r
		outX := xs[i] + ((xs[i+1]-xs[i])/after)*r
		outY := ys[i] + ((ys[i+1]-ys[i])/after)*r
		d.WriteString("L" + svg.Num(inX) + "," + svg.Num(inY) + "Q" + svg.Num(xs[i]) + "," + svg.Num(ys[i]) + " " + svg.Num(outX) + "," + svg.Num(outY))
	}
	d.WriteString("L" + svg.Num(xs[n-1]) + "," + svg.Num(ys[n-1]))
	return d.String()
}

func renderArchitecture(m *Model, config *core.Object, options *core.Options) core.Rendered {
	return common.Tighten(options, func(tight bool) core.Rendered { return draw(m, config, options, tight) })
}

func draw(m *Model, config *core.Object, options *core.Options, tight bool) core.Rendered {
	iconSize := setting(config, "iconSize", 48, 8, 512)
	size := setting(config, "fontSize", options.FontSizeOr(16), 4, 128)
	groupPad := setting(config, "padding", 20, 0, 256)
	if tight {
		groupPad = math.Min(groupPad, 12)
	}
	small := js.Round(size * 0.875)
	measurer := options.Measurer
	pad := options.PaddingOr(8)
	icons := options.Icons
	half := iconSize / 2

	nodes, groups := m.Nodes, m.Groups
	N, G := len(nodes), len(groups)
	gr := placeOnGrid(nodes, groups, m.Edges, m.LayoutHints)
	index := map[string]int{}
	for i, n := range nodes {
		index[n.ID] = i
	}

	// A service is its icon square with the title beneath; a junction takes the same room.
	labels := make([]text.Label, N)
	boxW := make([]float64, N)
	boxH := make([]float64, N)
	wrap := math.Max(iconSize*2.25, 96)
	if tight {
		wrap = math.Max(iconSize, 64)
	}
	for i, n := range nodes {
		title := ""
		if !n.Junction {
			title = n.Title
		}
		l := text.Layout(title, true, measurer, size, wrap, 0)
		labels[i] = l
		boxW[i] = math.Max(iconSize, 2*math.Ceil(l.Width/2))
		boxH[i] = iconSize
		if l.Height > 0 {
			boxH[i] += labelGap + l.Height
		}
	}

	var colValues, rowValues []int
	for i := 0; i < N; i++ {
		colValues = append(colValues, gr.col[i])
		rowValues = append(rowValues, gr.row[i])
	}
	for g := 0; g < G; g++ {
		if gr.groupEmpty[g] {
			colValues = append(colValues, gr.groupCol[g])
			rowValues = append(rowValues, gr.groupRow[g])
		}
	}
	colOf, rowOf := rank(colValues), rank(rowValues)
	C, R := len(colOf), len(rowOf)
	colW := make([]float64, C)
	rowH := make([]float64, R)
	gap := math.Max(40, js.Round(iconSize*0.75))
	if tight {
		gap = math.Max(24, 2*stub)
	}
	gapX := make([]float64, C)
	gapY := make([]float64, R)
	for i := range gapX {
		gapX[i] = gap
	}
	for i := range gapY {
		gapY[i] = gap
	}
	nodeCol := make([]int, N)
	nodeRow := make([]int, N)
	for i := 0; i < N; i++ {
		c, r := colOf[gr.col[i]], rowOf[gr.row[i]]
		nodeCol[i], nodeRow[i] = c, r
		colW[c] = math.Max(colW[c], boxW[i])
		rowH[r] = math.Max(rowH[r], boxH[i])
	}
	for g := 0; g < G; g++ {
		if !gr.groupEmpty[g] {
			continue
		}
		c, r := colOf[gr.groupCol[g]], rowOf[gr.groupRow[g]]
		colW[c] = math.Max(colW[c], iconSize)
		rowH[r] = math.Max(rowH[r], half)
	}

	// An edge label needs the gap its edge crosses to be wide enough to hold it.
	E := len(m.Edges)
	edgeLabels := make([]text.Label, E)
	ends := make([]int, E*2)
	lookup := func(id string) int {
		if i, ok := index[id]; ok {
			return i
		}
		return -1
	}
	for k, edge := range m.Edges {
		a, b := lookup(edge.LhsID), lookup(edge.RhsID)
		ends[k*2], ends[k*2+1] = a, b
		l := text.Layout(edge.Title, true, measurer, small, 160, 0)
		edgeLabels[k] = l
		if l.Width == 0 || a < 0 || b < 0 {
			continue
		}
		if nodeCol[a] != nodeCol[b] {
			c := min(nodeCol[a], nodeCol[b])
			gapX[c] = math.Max(gapX[c], math.Ceil(l.Width)+28)
		} else {
			colW[nodeCol[a]] = math.Max(colW[nodeCol[a]], 2*math.Ceil(l.Width/2)+8)
			if nodeRow[a] != nodeRow[b] {
				r := min(nodeRow[a], nodeRow[b])
				gapY[r] = math.Max(gapY[r], math.Ceil(l.Height)+24)
			}
		}
	}

	// Room around each group, kept on the rows and columns at its edges. A group that shares an
	// edge with a group inside it stands that much further out. Children come after their
	// parents, so walking backwards settles every group before the one that holds it.
	groupLabels := make([]text.Label, G)
	c0, c1, r0, r1 := make([]int, G), make([]int, G), make([]int, G), make([]int, G)
	insL, insR, insT, insB := make([]float64, G), make([]float64, G), make([]float64, G), make([]float64, G)
	headW, headH := make([]float64, G), make([]float64, G)
	colL, colR := make([]float64, C), make([]float64, C)
	rowT, rowB := make([]float64, R), make([]float64, R)
	badge := js.Round(size * 1.25)
	groupWrap := 320.0
	if tight {
		groupWrap = math.Max(80, common.TitleRoom(options)/2)
	}
	for g := 0; g < G; g++ {
		c0[g] = colOf[gr.groupCol[g]]
		c1[g] = colOf[gr.groupCol[g]+gr.groupCols[g]-1]
		r0[g] = rowOf[gr.groupRow[g]]
		r1[g] = rowOf[gr.groupRow[g]+gr.groupRows[g]-1]
		l := text.Layout(groups[g].Title, true, measurer, size, groupWrap, 0)
		groupLabels[g] = l
		icon := 0.0
		if groups[g].Icon != "" {
			icon = badge
		}
		headW[g] = l.Width + icon
		if l.Width > 0 && icon > 0 {
			headW[g] += 6
		}
		headH[g] = math.Max(l.Height, icon)
	}
	for g := G - 1; g >= 0; g-- {
		left := groupPad + insL[g]
		right := groupPad + insR[g]
		top := groupPad + insT[g]
		if headH[g] > 0 {
			top = math.Max(groupPad, headH[g]+12) + insT[g]
		}
		bottom := groupPad + insB[g]
		if headW[g] > 0 {
			inner := left + right
			for c := c0[g]; c <= c1[g]; c++ {
				inner += colW[c]
				if c < c1[g] {
					inner += colR[c] + gapX[c] + colL[c+1]
				}
			}
			if headW[g]+20 > inner {
				right += math.Ceil(headW[g] + 20 - inner)
			}
		}
		insL[g], insR[g], insT[g], insB[g] = left, right, top, bottom
		colL[c0[g]] = math.Max(colL[c0[g]], left)
		colR[c1[g]] = math.Max(colR[c1[g]], right)
		rowT[r0[g]] = math.Max(rowT[r0[g]], top)
		rowB[r1[g]] = math.Max(rowB[r1[g]], bottom)
		p := gr.groupParent[g]
		if p < 0 {
			continue
		}
		if c0[p] == c0[g] {
			insL[p] = math.Max(insL[p], left)
		}
		if c1[p] == c1[g] {
			insR[p] = math.Max(insR[p], right)
		}
		if r0[p] == r0[g] {
			insT[p] = math.Max(insT[p], top)
		}
		if r1[p] == r1[g] {
			insB[p] = math.Max(insB[p], bottom)
		}
	}

	X := make([]float64, C)
	Y := make([]float64, R)
	innerW := 0.0
	for c := 0; c < C; c++ {
		X[c] = innerW + colL[c]
		innerW = X[c] + colW[c] + colR[c]
		if c < C-1 {
			innerW += gapX[c]
		}
	}
	innerH := 0.0
	for r := 0; r < R; r++ {
		Y[r] = innerH + rowT[r]
		innerH = Y[r] + rowH[r] + rowB[r]
		if r < R-1 {
			innerH += gapY[r]
		}
	}
	// What is drawn, which an edge that has to go around a node can push past the grid.
	minX, minY, maxX, maxY := 0.0, 0.0, innerW, innerH

	gLeft := func(g int) float64 { return X[c0[g]] - insL[g] }
	gRight := func(g int) float64 { return X[c1[g]] + colW[c1[g]] + insR[g] }
	gTop := func(g int) float64 { return Y[r0[g]] - insT[g] }
	gBottom := func(g int) float64 { return Y[r1[g]] + rowH[r1[g]] + insB[g] }
	centerX := func(i int) float64 { return X[nodeCol[i]] + colW[nodeCol[i]]/2 }

	makePort := func(i, side int, viaGroup bool) *port {
		dx, dy := dirX[side], dirY[side]
		cx := centerX(i)
		y := Y[nodeRow[i]]
		p := &port{x: cx, y: y + half, dx: dx, dy: dy, x0: cx - boxW[i]/2, y0: y, x1: cx + boxW[i]/2, y1: y + boxH[i]}
		if nodes[i].Junction {
			p.x0, p.x1 = cx, cx
			p.y0, p.y1 = p.y, p.y
			p.gap = dotR
		} else if dx != 0 {
			p.x = cx + dx*half
		} else if dy < 0 {
			p.y = y
		} else {
			p.y = y + boxH[i]
			if labels[i].Height > 0 {
				p.y += 2
			}
		}
		g := -1
		if viaGroup {
			g = gr.nodeParent[i]
		}
		if g >= 0 {
			p.x0, p.x1, p.y0, p.y1 = gLeft(g), gRight(g), gTop(g), gBottom(g)
			p.gap = 0
			p.wire = nodes[i].Junction
			if dx != 0 {
				p.x = p.x1
				if dx < 0 {
					p.x = p.x0
				}
			} else {
				p.y = p.y1
				if dy < 0 {
					p.y = p.y0
				}
			}
		}
		return p
	}

	var edgesOut, labelsOut strings.Builder
	// Vertical runs of the edges, to keep group titles out from under them.
	var runs []float64
	trimLength := svg.MarkerTrim("arrow_point")
	for k, edge := range m.Edges {
		a, b := ends[k*2], ends[k*2+1]
		if a < 0 || b < 0 {
			continue
		}
		s := makePort(a, sideIndex(edge.LhsDir), edge.LhsGroup)
		e := makePort(b, sideIndex(edge.RhsDir), edge.RhsGroup)
		if s.x == e.x && s.y == e.y && s.dx == e.dx {
			// An edge from a side back to the same side: part the two ends so it shows as a loop.
			s.x -= 8 * math.Abs(s.dy)
			s.y -= 8 * math.Abs(s.dx)
			e.x += 8 * math.Abs(s.dy)
			e.y += 8 * math.Abs(s.dx)
		}
		routed := route(s, e)
		if s.wire {
			routed = append([]float64{centerX(a), Y[nodeRow[a]] + half}, routed...)
		}
		if e.wire {
			routed = append(routed, centerX(b), Y[nodeRow[b]]+half)
		}
		points := simplify(routed)
		if len(points) < 4 {
			continue
		}
		startTrim, endTrim := 0.0, 0.0
		if edge.LhsInto && !s.wire {
			startTrim = trimLength + s.gap
		}
		if edge.RhsInto && !e.wire {
			endTrim = trimLength + e.gap
		}
		edgesOut.WriteString(`<g class="pele-edge" data-id="` + svg.EscText(edge.LhsID+"-"+edge.RhsID) + `">` +
			`<path d="` + pathData(points, startTrim, endTrim) + `"/>`)
		if edge.LhsInto {
			edgesOut.WriteString(svg.Marker("arrow_point", s.x+s.dx*s.gap, s.y+s.dy*s.gap, -s.dx, -s.dy, "var(--_l)"))
		}
		if edge.RhsInto {
			edgesOut.WriteString(svg.Marker("arrow_point", e.x+e.dx*e.gap, e.y+e.dy*e.gap, -e.dx, -e.dy, "var(--_l)"))
		}
		edgesOut.WriteString("</g>")
		best, bestLength, flat, flatLength := -1, -1.0, -1, -1.0
		l := edgeLabels[k]
		for i := 0; i < len(points); i += 2 {
			minX = math.Min(minX, points[i]-4)
			maxX = math.Max(maxX, points[i]+4)
			minY = math.Min(minY, points[i+1]-4)
			maxY = math.Max(maxY, points[i+1]+4)
		}
		for i := 0; i+3 < len(points); i += 2 {
			horizontal := math.Abs(points[i+1]-points[i+3]) < 0.01
			length := math.Abs(points[i]-points[i+2]) + math.Abs(points[i+1]-points[i+3])
			if !horizontal {
				runs = append(runs, points[i], math.Min(points[i+1], points[i+3]), math.Max(points[i+1], points[i+3]))
			}
			if length > bestLength {
				best, bestLength = i, length
			}
			if horizontal && length > flatLength {
				flat, flatLength = i, length
			}
		}
		if l.Width > 0 {
			at := best
			if flatLength >= l.Width+8 {
				at = flat
			}
			x := (points[at] + points[at+2]) / 2
			y := (points[at+1] + points[at+3]) / 2
			w := l.Width + 8
			upright := math.Abs(points[at]-points[at+2]) < 0.01
			// On a short run next to a node the label would land on the node, so it moves off to the side.
			for _, p := range []*port{s, e} {
				if x+w/2 <= p.x0 || x-w/2 >= p.x1 || y+l.Height/2 <= p.y0 || y-l.Height/2 >= p.y1 {
					continue
				}
				if upright {
					if x >= (p.x0+p.x1)/2 {
						x = p.x1 + w/2 + 4
					} else {
						x = p.x0 - w/2 - 4
					}
				} else if y >= (p.y0+p.y1)/2 {
					y = p.y1 + l.Height/2 + 4
				} else {
					y = p.y0 - l.Height/2 - 4
				}
			}
			minX = math.Min(minX, x-w/2)
			maxX = math.Max(maxX, x+w/2)
			minY = math.Min(minY, y-l.Height/2)
			maxY = math.Max(maxY, y+l.Height/2)
			labelsOut.WriteString(svg.EdgeLabelSvg(edge.LhsID+"-"+edge.RhsID, l, x, y, "", icons))
		}
	}

	var clusters strings.Builder
	for g := 0; g < G; g++ {
		group := groups[g]
		x, y := gLeft(g), gTop(g)
		w := gRight(g) - x
		l := groupLabels[g]
		head := ""
		if headW[g] > 0 {
			// The title starts at the left, or after the edges that come down through that corner.
			band := y + headH[g] + 12
			var crossings []float64
			for i := 0; i < len(runs); i += 3 {
				if runs[i+1] < band && runs[i+2] > y && runs[i] > x && runs[i] < x+w {
					crossings = append(crossings, runs[i])
				}
			}
			slices.SortStableFunc(crossings, func(a, b float64) int { return js.Cmp(a - b) })
			crossings = append(crossings, x+w-4)
			start := x + 10
			for _, c := range crossings {
				if c-6-start >= headW[g] {
					break
				}
				start = math.Max(start, c+6)
			}
			if start+headW[g] > x+w-4 {
				start = x + 10
			}
			cy := y + 6 + headH[g]/2
			textX := start
			if group.Icon != "" {
				head += iconSlot(group.Icon, start, cy-badge/2, badge, icons)
				textX += badge + 6
			}
			head += svg.LabelSvg(l, textX+l.Width/2, cy, ` class="pele-cluster-label" fill="var(--_m)"`, icons)
		}
		clusters.WriteString(`<g class="pele-cluster" data-id="` + svg.EscText(group.ID) + `">` +
			`<rect x="` + svg.Num(x) + `" y="` + svg.Num(y) + `" width="` + svg.Num(w) + `" height="` + svg.Num(gBottom(g)-y) + `" rx="` + svg.Radius + `" fill="var(--_a)" fill-opacity="0.5" stroke="var(--_b)"/>` +
			head + "</g>")
	}

	var nodesOut strings.Builder
	glyph := iconSize * 0.5
	textSize := math.Max(6, js.Round(size*0.75))
	for i, n := range nodes {
		at := `data-id="` + svg.EscText(n.ID) + `" transform="translate(` + svg.Num(centerX(i)) + "," + svg.Num(Y[nodeRow[i]]) + `)"`
		if n.Junction {
			nodesOut.WriteString(`<g class="pele-node pele-junction" ` + at + `><circle cy="` + svg.Num(half) + `" r="3" fill="var(--_l)"/></g>`)
			continue
		}
		inner := ""
		if n.Icon != "" {
			inner = iconSlot(n.Icon, -glyph/2, (iconSize-glyph)/2, glyph, icons)
		} else if n.IconText != "" {
			t := text.Layout(n.IconText, false, measurer, textSize, iconSize-8, 0)
			lh := t.LineHeight
			if lh == 0 {
				lh = 1
			}
			lines := int(math.Max(1, math.Floor((iconSize-4)/lh)))
			inner = svg.LabelSvg(fit(t, iconSize-8, lines, measurer), 0, half, ` class="pele-icon-text" font-size="`+js.Num(textSize)+`"`, icons)
		}
		nodesOut.WriteString(`<g class="pele-node pele-service" ` + at + `>` +
			`<rect x="` + svg.Num(-half) + `" width="` + svg.Num(iconSize) + `" height="` + svg.Num(iconSize) + `" rx="` + svg.Radius + `" fill="var(--_s)" stroke="var(--_b)"/>` +
			inner + svg.LabelSvg(labels[i], 0, iconSize+labelGap+labels[i].Height/2, ` class="pele-label"`, icons) + "</g>")
	}

	titleWrap := 4000.0
	if tight {
		titleWrap = common.TitleRoom(options)
	}
	title := text.Layout(m.Title, false, measurer, size, titleWrap, text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 12
	}
	width := math.Ceil(math.Max(maxX-minX, title.Width) + 2*pad)
	height := math.Ceil(maxY - minY + titleHeight + 2*pad)
	ox := js.Round((width-(maxX-minX))/2 - minX)
	oy := js.Round(pad + titleHeight - minY)
	var body strings.Builder
	if clusters.Len() > 0 {
		body.WriteString(`<g class="pele-clusters">` + clusters.String() + "</g>")
	}
	if edgesOut.Len() > 0 {
		body.WriteString(`<g class="pele-edges" fill="none" stroke="var(--_l)" stroke-linecap="round" stroke-linejoin="round">` + edgesOut.String() + "</g>")
	}
	if labelsOut.Len() > 0 {
		body.WriteString(`<g class="pele-edge-labels" font-size="` + js.Num(small) + `">` + labelsOut.String() + "</g>")
	}
	if nodesOut.Len() > 0 {
		body.WriteString(`<g class="pele-nodes">` + nodesOut.String() + "</g>")
	}
	content := svg.LabelSvg(title, width/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil)
	if body.Len() > 0 {
		content += `<g transform="translate(` + js.Num(ox) + "," + js.Num(oy) + `)">` + body.String() + "</g>"
	}
	acc := svg.Accessible{AccTitle: m.AccTitle, AccDescr: m.AccDescr}
	return core.Rendered{SVG: svg.Document("architecture", width, height, size, options, acc, content), Width: width, Height: height}
}

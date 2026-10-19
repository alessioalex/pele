package block

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
	defaultGap = 12
	wrapWidth  = 200
	corner     = 8
	shapeAttrs = ` fill="var(--_s)" stroke="var(--_b)"`
	lineAttrs  = ` stroke="var(--_b)"`
	// Checking every edge against every block is quadratic, so large diagrams route without it.
	obstacleBudget = 2_000_000
	sqrt1_2        = 0.7071067811865476
)

var shapeNames = map[string]string{
	"square":              "rect",
	"round":               "rounded",
	"circle":              "circle",
	"doublecircle":        "dbl-circ",
	"diamond":             "diam",
	"hexagon":             "hex",
	"stadium":             "stadium",
	"subroutine":          "fr-rect",
	"cylinder":            "cyl",
	"rect_left_inv_arrow": "odd",
	"lean_right":          "lean-r",
	"lean_left":           "lean-l",
	"trapezoid":           "trap-b",
	"inv_trapezoid":       "trap-t",
}

// Shapes that keep their own size in a larger cell, where the others stretch to fill it.
var fixedShapes = map[string]bool{"circle": true, "dbl-circ": true, "diam": true}

// Shapes that only widen, because their slants and curves are set by their height.
var wideShapes = map[string]bool{"stadium": true, "hex": true, "odd": true, "lean-r": true, "lean-l": true, "trap-b": true, "trap-t": true}

const (
	kindLeaf = iota
	kindArrow
	kindComposite
	kindSpace
)

type view struct {
	cell
	block *Block
	kind  int
	index int
	shape string
	label text.Label
	style svg.ResolvedStyle
	mask  int
	dx    float64
	dy    float64
	// Size as drawn, which for a fixed shape is smaller than its cell.
	bw float64
	bh float64
}

type route struct {
	edge *Block
	// Two points for a straight line, three for a bend, none for a self-loop.
	points []float64
	loop   int
	label  text.Label
	lx, ly float64
}

func or1(x float64) float64 {
	if x == 0 || math.IsNaN(x) {
		return 1
	}
	return x
}

func isRound(v *view) bool {
	return v.kind == kindLeaf && (v.shape == "circle" || v.shape == "dbl-circ")
}

// edgeY is where a vertical line at x meets the outline, below the center (sign 1) or above it (sign -1).
func edgeY(v *view, x, sign float64) float64 {
	hw := v.bw / 2
	hh := v.bh / 2
	dx := math.Abs(x - v.x)
	reach := hh
	if isRound(v) {
		reach = math.Sqrt(math.Max(hw*hw-dx*dx, 0))
	} else if v.kind == kindLeaf && v.shape == "diam" {
		reach = hh * math.Max(1-dx/hw, 0)
	} else if v.kind == kindLeaf {
		side := 0
		if sign > 0 {
			side = 2
		}
		reach = hh - svg.ShapeInset(v.shape, v.bw, v.bh, side, 0)
	} else if v.kind == kindArrow && arrowAxis(v.mask) == 'x' {
		reach = hh - overhang
	}
	return v.y + sign*reach
}

// edgeX is where a horizontal line at y meets the outline, right of the center (sign 1) or left of it (sign -1).
func edgeX(v *view, y, sign float64) float64 {
	hw := v.bw / 2
	hh := v.bh / 2
	dy := math.Abs(y - v.y)
	reach := hw
	if isRound(v) {
		reach = math.Sqrt(math.Max(hh*hh-dy*dy, 0))
	} else if v.kind == kindLeaf && v.shape == "diam" {
		reach = hw * math.Max(1-dy/hh, 0)
	} else if v.kind == kindLeaf {
		side := 3
		if sign > 0 {
			side = 1
		}
		reach = hw - svg.ShapeInset(v.shape, v.bw, v.bh, side, 0)
	} else if v.kind == kindArrow && arrowAxis(v.mask) == 'y' {
		reach = hw - overhang
	}
	return v.x + sign*reach
}

// border is where the line from the center toward (tx, ty) leaves the outline.
func border(v *view, tx, ty float64, out []float64) []float64 {
	dx := tx - v.x
	dy := ty - v.y
	hw := v.bw / 2
	hh := v.bh / 2
	var k float64
	if isRound(v) {
		k = hw / or1(js.Hypot(dx, dy))
	} else if v.kind == kindLeaf && v.shape == "diam" {
		k = 1 / or1(math.Abs(dx)/hw+math.Abs(dy)/hh)
	} else {
		a := math.Inf(1)
		if dx != 0 {
			a = hw / math.Abs(dx)
		}
		b := math.Inf(1)
		if dy != 0 {
			b = hh / math.Abs(dy)
		}
		k = min(a, b)
	}
	if math.IsInf(k, 1) {
		k = 0
	}
	return append(out, v.x+dx*k, v.y+dy*k)
}

func crossesBox(x1, y1, x2, y2 float64, v *view) bool {
	l := v.x - v.bw/2 + 1
	r := v.x + v.bw/2 - 1
	t := v.y - v.bh/2 + 1
	b := v.y + v.bh/2 - 1
	if (x1 < l && x2 < l) || (x1 > r && x2 > r) || (y1 < t && y2 < t) || (y1 > b && y2 > b) {
		return false
	}
	if x1 == x2 || y1 == y2 {
		return true
	}
	// The segment is diagonal and its bounding box overlaps: the box is hit unless all four corners are on one side.
	dx := x2 - x1
	dy := y2 - y1
	side := func(x, y float64) float64 { return js.Sign(dx*(y-y1) - dy*(x-x1)) }
	first := side(l, t)
	return first == 0 || side(r, t) != first || side(r, b) != first || side(l, b) != first
}

func hits(points []float64, s, t *view, obstacles []*view) int {
	count := 0
	for _, v := range obstacles {
		if v == s || v == t {
			continue
		}
		for i := 0; i+3 < len(points); i += 2 {
			if crossesBox(points[i], points[i+1], points[i+2], points[i+3], v) {
				count++
				break
			}
		}
	}
	return count
}

// routeEdge is a straight line where the two blocks share a row or a column, otherwise one bend
// between facing sides. Obstacles are nil when they are not checked.
func routeEdge(s, t *view, shift float64, obstacles []*view) []float64 {
	sl := s.x - s.bw/2
	sr := s.x + s.bw/2
	st := s.y - s.bh/2
	sb := s.y + s.bh/2
	tl := t.x - t.bw/2
	tr := t.x + t.bw/2
	tt := t.y - t.bh/2
	tb := t.y + t.bh/2
	x1 := math.Max(sl, tl)
	x2 := math.Min(sr, tr)
	y1 := math.Max(st, tt)
	y2 := math.Min(sb, tb)
	overX := x2-x1 > 6
	overY := y2-y1 > 6

	if overX && overY {
		// One block is inside the other: join the inner one to the nearest side of the outer one.
		inner, outer := t, s
		if s.bw*s.bh <= t.bw*t.bh {
			inner, outer = s, t
		}
		up := inner.y - inner.bh/2 - (outer.y - outer.bh/2)
		down := outer.y + outer.bh/2 - (inner.y + inner.bh/2)
		left := inner.x - inner.bw/2 - (outer.x - outer.bw/2)
		right := outer.x + outer.bw/2 - (inner.x + inner.bw/2)
		least := min(up, down, left, right)
		var points []float64
		switch least {
		case down:
			points = []float64{inner.x, edgeY(inner, inner.x, 1), inner.x, outer.y + outer.bh/2}
		case up:
			points = []float64{inner.x, edgeY(inner, inner.x, -1), inner.x, outer.y - outer.bh/2}
		case right:
			points = []float64{edgeX(inner, inner.y, 1), inner.y, outer.x + outer.bw/2, inner.y}
		default:
			points = []float64{edgeX(inner, inner.y, -1), inner.y, outer.x - outer.bw/2, inner.y}
		}
		if inner == s {
			return points
		}
		return []float64{points[2], points[3], points[0], points[1]}
	}
	if overX {
		margin := math.Min(6, (x2-x1)/2)
		x := math.Min(math.Max((x1+x2)/2+shift, x1+margin), x2-margin)
		down := -1.0
		if t.y > s.y {
			down = 1
		}
		return []float64{x, edgeY(s, x, down), x, edgeY(t, x, -down)}
	}
	if overY {
		margin := math.Min(6, (y2-y1)/2)
		y := math.Min(math.Max((y1+y2)/2+shift, y1+margin), y2-margin)
		right := -1.0
		if t.x > s.x {
			right = 1
		}
		return []float64{edgeX(s, y, right), y, edgeX(t, y, -right), y}
	}

	right := -1.0
	if t.x > s.x {
		right = 1
	}
	down := -1.0
	if t.y > s.y {
		down = 1
	}
	across := []float64{edgeX(s, s.y, right), s.y, t.x, s.y, t.x, edgeY(t, t.x, -down)}
	along := []float64{s.x, edgeY(s, s.x, down), s.x, t.y, edgeX(t, t.y, -right), t.y}
	wide := math.Abs(t.x-s.x) >= math.Abs(t.y-s.y)
	best, other := along, across
	if wide {
		best, other = across, along
	}
	if obstacles == nil {
		return best
	}
	least := hits(best, s, t, obstacles)
	if least == 0 {
		return best
	}
	if count := hits(other, s, t, obstacles); count < least {
		best = other
		least = count
	}
	if least == 0 {
		return best
	}
	direct := border(s, t.x, t.y, nil)
	direct = border(t, s.x, s.y, direct)
	if hits(direct, s, t, obstacles) < least {
		return direct
	}
	return best
}

// trace builds the path through two or three points, rounding the bend and leaving room for markers.
func trace(p []float64, startTrim, endTrim float64) svg.EdgePath {
	n := len(p)
	sx := p[0]
	sy := p[1]
	ex := p[n-2]
	ey := p[n-1]
	ax := p[2] - sx
	ay := p[3] - sy
	first := js.Hypot(ax, ay)
	ax /= or1(first)
	ay /= or1(first)
	bx := ex - p[n-4]
	by := ey - p[n-3]
	last := js.Hypot(bx, by)
	bx /= or1(last)
	by /= or1(last)
	if n == 4 && startTrim+endTrim > first {
		scale := first / (startTrim + endTrim)
		startTrim *= scale
		endTrim *= scale
	}
	startTrim = math.Min(startTrim, first)
	endTrim = math.Min(endTrim, last)
	var d strings.Builder
	d.WriteString("M" + svg.Num(sx+ax*startTrim) + "," + svg.Num(sy+ay*startTrim))
	if n == 6 {
		r := math.Max(0, min(corner, first-startTrim, last-endTrim))
		d.WriteString("L" + svg.Num(p[2]-ax*r) + "," + svg.Num(p[3]-ay*r) + "Q" + svg.Num(p[2]) + "," + svg.Num(p[3]) + " " +
			svg.Num(p[2]+bx*r) + "," + svg.Num(p[3]+by*r))
	}
	d.WriteString("L" + svg.Num(ex-bx*endTrim) + "," + svg.Num(ey-by*endTrim))
	return svg.EdgePath{D: d.String(), Sx: sx, Sy: sy, Sdx: -ax, Sdy: -ay, Ex: ex, Ey: ey, Edx: bx, Edy: by}
}

// loopArc: a self-loop is an arc around the top right corner of the block, or around the
// matching point on a round or diamond outline. Gives the arc's center and radius and the angles
// where it leaves and returns.
func loopArc(v *view, k int) (px, py, r, a1, a2 float64) {
	hw := v.bw / 2
	hh := v.bh / 2
	r = math.Max(1, min(10+5*float64(k-1), hw, hh))
	if isRound(v) {
		px = v.x + hw*sqrt1_2
		py = v.y - hw*sqrt1_2
		half := 2 * js.Asin(math.Min(1, r/(2*hw)))
		angle := func(at float64) float64 {
			return js.Atan2(v.y-hw*js.Sin(at)-py, v.x+hw*js.Cos(at)-px)
		}
		return px, py, r, angle(math.Pi/4 + half), angle(math.Pi/4 - half)
	}
	if v.kind == kindLeaf && v.shape == "diam" {
		slope := js.Atan2(hh, hw)
		return v.x + hw/2, v.y - hh/2, r, slope + math.Pi, slope
	}
	return v.x + hw, v.y - hh, r, math.Pi, math.Pi / 2
}

func loopPath(v *view, k int, startTrim, endTrim float64) svg.EdgePath {
	px, py, r, a1, a2 := loopArc(v, k)
	from := a1 + startTrim/r
	to := a2 - endTrim/r
	sweep := math.Mod(to-from, 2*math.Pi)
	if sweep <= 0 {
		sweep += 2 * math.Pi
	}
	x1 := px + r*js.Cos(from)
	y1 := py + r*js.Sin(from)
	x2 := px + r*js.Cos(to)
	y2 := py + r*js.Sin(to)
	sx := px + r*js.Cos(a1)
	sy := py + r*js.Sin(a1)
	ex := px + r*js.Cos(a2)
	ey := py + r*js.Sin(a2)
	// Each marker lies along the chord it covers, so it follows the curve instead of the tangent at its tip.
	sl := js.Hypot(sx-x1, sy-y1)
	el := js.Hypot(ex-x2, ey-y2)
	large := "0"
	if sweep > math.Pi {
		large = "1"
	}
	path := svg.EdgePath{
		D:  "M" + svg.Num(x1) + "," + svg.Num(y1) + "A" + svg.Num(r) + "," + svg.Num(r) + " 0 " + large + " 1 " + svg.Num(x2) + "," + svg.Num(y2),
		Sx: sx, Sy: sy, Ex: ex, Ey: ey,
	}
	if sl > 0 {
		path.Sdx, path.Sdy = (sx-x1)/sl, (sy-y1)/sl
	} else {
		path.Sdx, path.Sdy = js.Sin(a1), -js.Cos(a1)
	}
	if el > 0 {
		path.Edx, path.Edy = (ex-x2)/el, (ey-y2)/el
	} else {
		path.Edx, path.Edy = -js.Sin(a2), js.Cos(a2)
	}
	return path
}

var spots = []float64{0.5, 0.3, 0.7, 0.15, 0.85}

// labelSpot picks the point on a leg where a label of the given size covers no block other than
// the two it joins.
func labelSpot(p []float64, at int, w, h float64, s, t *view, obstacles []*view) float64 {
	if obstacles == nil {
		return 0.5
	}
	for _, spot := range spots {
		x := p[at] + (p[at+2]-p[at])*spot
		y := p[at+1] + (p[at+3]-p[at+1])*spot
		clear := true
		for _, v := range obstacles {
			if v != s && v != t && math.Abs(x-v.x) < (w+v.bw)/2 && math.Abs(y-v.y) < (h+v.bh)/2 {
				clear = false
				break
			}
		}
		if clear {
			return spot
		}
	}
	return 0.5
}

func renderBlock(model *Model, config *core.Object, options *core.Options) core.Rendered {
	return common.Tighten(options, func(tight bool) core.Rendered { return draw(model, config, options, tight) })
}

func draw(model *Model, config *core.Object, options *core.Options, tight bool) core.Rendered {
	size := options.FontSizeOr(16)
	edgeSize := js.Round(size * 0.875)
	measurer := options.Measurer
	pad := options.PaddingOr(8)
	icons := options.Icons
	gap := float64(defaultGap)
	if configured, ok := config.Section("block").Value("padding").(float64); ok && configured >= 0 {
		gap = math.Min(configured, 200)
	}
	if tight {
		gap = math.Min(gap, 6)
	}
	wrap := float64(wrapWidth)
	if tight {
		wrap = 80
	}
	titleRoom := 4000.0
	if tight {
		titleRoom = common.TitleRoom(options)
	}

	styleOf := func(block *Block) svg.ResolvedStyle {
		var declarations []string
		// A style list is split on commas only, so one entry can still hold several declarations.
		add := func(styles []string) {
			for _, style := range styles {
				declarations = append(declarations, strings.Split(style, ";")...)
			}
		}
		names := block.Classes
		if len(names) == 0 {
			names = []string{"default"}
		}
		for _, name := range names {
			if def := model.Classes[name]; def != nil {
				add(def.Styles)
			}
		}
		if block.Styles != nil {
			add(block.Styles)
		}
		return svg.ResolveStyle(declarations)
	}

	newView := func(block *Block, index int) *view {
		typ := block.Type
		var style svg.ResolvedStyle
		if typ == "space" {
			style = svg.ResolveStyle(nil)
		} else {
			style = styleOf(block)
		}
		textStyle := 0
		if style.Bold {
			textStyle |= text.Bold
		}
		if style.Italic {
			textStyle |= text.Italic
		}
		fontSize := size
		if style.FontSize != 0 {
			fontSize = style.FontSize
		}
		v := &view{block: block, index: index, style: style}
		switch typ {
		case "space":
			v.kind = kindSpace
		case "composite":
			v.kind = kindComposite
			v.label = text.Layout(block.Label, false, measurer, fontSize, titleRoom, textStyle)
			if v.label.Height > 0 {
				v.head = v.label.Height + 4
			}
			v.nw = v.label.Width + 2*gap
			v.nh = v.head + 2*gap
		case "block_arrow":
			v.kind = kindArrow
			v.label = text.Layout(block.Label, false, measurer, fontSize, wrap, textStyle)
			blank := true
			for _, line := range v.label.Lines {
				for _, part := range line {
					if part.Icon != "" || js.Trim(part.Text) != "" {
						blank = false
					}
				}
			}
			v.mask = arrowMask(block.Directions)
			var s arrowSizing
			if blank {
				s = arrowSize(v.mask, 0, 0)
			} else {
				s = arrowSize(v.mask, v.label.Width, v.label.Height)
			}
			v.nw, v.nh, v.dx, v.dy = s.w, s.h, s.dx, s.dy
		default:
			v.shape = "rect"
			if name, ok := shapeNames[typ]; ok {
				v.shape = name
			}
			v.label = text.Layout(block.Label, false, measurer, fontSize, wrap, textStyle)
			s := svg.ShapeSize(v.shape, v.label.Width, v.label.Height)
			v.nw = s.W
			v.nh = s.H
			// A cylinder's caps deepen as it widens, so it is sized for the deepest ones.
			if v.shape == "cyl" {
				v.nh = math.Max(s.H, v.label.Height+40)
			}
			v.dy = s.Dy
		}
		v.columns = -1
		if v.kind == kindComposite {
			v.columns = block.Columns
		}
		v.span = block.WidthInColumns
		if v.kind == kindSpace {
			v.span = 1
		}
		v.space = v.kind == kindSpace
		return v
	}

	root := newView(model.Root, 0)
	root.nw = 0
	root.nh = 0
	order := []*view{root}
	byID := map[string]*view{}
	for i := 0; i < len(order); i++ {
		parent := order[i]
		if parent.kind != kindComposite {
			continue
		}
		for _, child := range parent.block.Children {
			v := newView(child, len(order))
			parent.children = append(parent.children, v)
			order = append(order, v)
			if v.kind != kindSpace {
				byID[child.ID] = v
			}
		}
	}

	contentWidth, contentHeight := layout(root, gap)

	obstacles := make([]*view, 0, len(order))
	for _, v := range order {
		switch {
		case v.kind == kindLeaf && fixedShapes[v.shape]:
			v.bw = v.nw
			v.bh = v.nh
		case v.kind == kindArrow:
			v.bw = v.nw
			if v.span > 1 {
				v.bw = v.w
			}
			v.bh = v.nh
		default:
			v.bw = v.w
			v.bh = v.h
			if v.kind == kindLeaf && wideShapes[v.shape] {
				v.bh = v.nh
			}
		}
		if v.kind == kindLeaf && v.shape == "cyl" {
			v.dy = math.Min(12, math.Max(6, v.bw/14)) / 2
		}
		if v.kind == kindLeaf || v.kind == kindArrow {
			obstacles = append(obstacles, v)
		}
	}

	// Edges between the same two blocks are spread apart instead of drawn on top of each other.
	pairs := map[int]int{}
	labelled := map[int]bool{}
	type ends struct{ s, t *view }
	var ending []ends
	var drawn []*Block
	pairKey := func(s, t *view) int {
		return min(s.index, t.index)*len(order) + max(s.index, t.index)
	}
	for _, edge := range model.Edges {
		s := byID[edge.Start]
		t := byID[edge.End]
		if s == nil || t == nil {
			continue
		}
		key := pairKey(s, t)
		pairs[key]++
		if edge.Label != "" {
			labelled[key] = true
		}
		ending = append(ending, ends{s, t})
		drawn = append(drawn, edge)
	}
	var check []*view
	if float64(len(drawn))*float64(len(obstacles)) <= obstacleBudget {
		check = obstacles
	}
	seen := map[int]int{}
	routes := make([]route, 0, len(drawn))
	minX := 0.0
	minY := 0.0
	maxX := contentWidth
	maxY := contentHeight
	grow := func(x, y, w, h float64) {
		if x-w/2 < minX {
			minX = x - w/2
		}
		if x+w/2 > maxX {
			maxX = x + w/2
		}
		if y-h/2 < minY {
			minY = y - h/2
		}
		if y+h/2 > maxY {
			maxY = y + h/2
		}
	}
	for i := range drawn {
		s, t := ending[i].s, ending[i].t
		key := pairKey(s, t)
		total := float64(pairs[key])
		k := seen[key]
		seen[key] = k + 1
		label := text.Layout(drawn[i].Label, false, measurer, edgeSize, wrapWidth, 0)
		var points []float64
		var lx, ly float64
		if s == t {
			px, py, r, _, _ := loopArc(s, k+1)
			points = []float64{}
			grow(px, py, 2*r+2, 2*r+2)
			lx = px + r + 6 + label.Width/2
			ly = py - r/2
		} else {
			step := 14.0
			if labelled[key] {
				step = js.Round(edgeSize*1.5) + 2
			}
			points = routeEdge(s, t, (float64(k)-(total-1)/2)*step, check)
			n := len(points)
			// The label goes on the longer leg.
			firstLeg := math.Abs(points[2]-points[0]) + math.Abs(points[3]-points[1])
			lastLeg := math.Abs(points[n-2]-points[n-4]) + math.Abs(points[n-1]-points[n-3])
			at := 0
			if n == 6 && lastLeg > firstLeg {
				at = 2
			}
			spot := 0.5
			if label.Width > 0 {
				spot = labelSpot(points, at, label.Width+8, label.Height, s, t, check)
			}
			lx = points[at] + (points[at+2]-points[at])*spot
			ly = points[at+1] + (points[at+3]-points[at+1])*spot
		}
		if label.Width > 0 {
			grow(lx, ly, label.Width+8, label.Height)
		}
		loop := 0
		if s == t {
			loop = k + 1
		}
		routes = append(routes, route{edge: drawn[i], points: points, loop: loop, label: label, lx: lx, ly: ly})
	}

	title := text.Layout(model.Title, false, measurer, size, titleRoom, text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 12
	}
	inner := math.Max(maxX-minX, title.Width)
	ox := pad - minX + (inner-(maxX-minX))/2
	oy := pad + titleHeight - minY
	for _, v := range order {
		v.x += ox
		v.y += oy
	}
	width := math.Ceil(inner + 2*pad)
	height := math.Ceil(maxY - minY + titleHeight + 2*pad)

	var clusters, nodes strings.Builder
	for i := 1; i < len(order); i++ {
		v := order[i]
		if v.kind == kindSpace {
			continue
		}
		classes := svg.ClassNames(strings.Join(v.block.Classes, " "))
		id := svg.EscText(v.block.ID)
		if v.kind == kindComposite {
			top := v.y - v.h/2
			clusters.WriteString(`<g class="pele-cluster` + classes + `" data-id="` + id + `">` +
				`<rect x="` + svg.Num(v.x-v.w/2) + `" y="` + svg.Num(top) + `" width="` + svg.Num(v.w) + `" height="` + svg.Num(v.h) +
				`" rx="` + svg.Radius + `" fill="var(--_a)" fill-opacity="0.5" stroke="var(--_b)"` + v.style.Shape + `/>` +
				svg.LabelSvg(v.label, v.x, top+(v.head+gap)/2+2, ` class="pele-cluster-label" fill="var(--_m)"`+v.style.Text, icons) +
				`</g>`)
			continue
		}
		arrow := v.kind == kindArrow
		shapeClass := v.shape
		if arrow {
			shapeClass = "block-arrow"
		}
		nodes.WriteString(`<g class="pele-node pele-shape-` + shapeClass + classes + `" data-id="` + id +
			`" transform="translate(` + svg.Num(v.x) + `,` + svg.Num(v.y) + `)">`)
		if arrow {
			nodes.WriteString(drawArrow(v.mask, v.bw, v.bh, shapeAttrs+v.style.Shape))
		} else {
			nodes.WriteString(svg.DrawShape(v.shape, v.bw, v.bh, shapeAttrs+v.style.Shape, lineAttrs+v.style.Line))
		}
		nodes.WriteString(svg.LabelSvg(v.label, v.dx, v.dy, ` class="pele-label"`+v.style.Text, icons) + `</g>`)
	}

	var edges, labels strings.Builder
	for i := range routes {
		rt := &routes[i]
		edge := rt.edge
		startType := edge.ArrowTypeStart
		endType := edge.ArrowTypeEnd
		var path svg.EdgePath
		if rt.loop > 0 {
			path = loopPath(ending[i].s, rt.loop, svg.MarkerTrim(startType), svg.MarkerTrim(endType))
		} else {
			for k := 0; k < len(rt.points); k += 2 {
				rt.points[k] += ox
				rt.points[k+1] += oy
			}
			path = trace(rt.points, svg.MarkerTrim(startType), svg.MarkerTrim(endType))
		}
		id := svg.EscText(edge.ID)
		edges.WriteString(`<g class="pele-edge" data-id="` + id + `"><path d="` + path.D + `"`)
		if edge.Thickness == "thick" {
			edges.WriteString(` stroke-width="2.5"`)
		}
		if edge.Pattern == "dotted" {
			edges.WriteString(` stroke-dasharray="3 4"`)
		}
		edges.WriteString(`/>` +
			svg.Marker(startType, path.Sx, path.Sy, path.Sdx, path.Sdy, "var(--_l)") +
			svg.Marker(endType, path.Ex, path.Ey, path.Edx, path.Edy, "var(--_l)") +
			`</g>`)
		if rt.label.Width > 0 {
			labels.WriteString(svg.EdgeLabelSvg(edge.ID, rt.label, rt.lx+ox, rt.ly+oy, "", icons))
		}
	}

	var content strings.Builder
	content.WriteString(svg.LabelSvg(title, width/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil))
	if clusters.Len() > 0 {
		content.WriteString(`<g class="pele-clusters">` + clusters.String() + `</g>`)
	}
	if edges.Len() > 0 {
		content.WriteString(`<g class="pele-edges" fill="none" stroke="var(--_l)" stroke-linecap="round">` + edges.String() + `</g>`)
	}
	content.WriteString(`<g class="pele-nodes">` + nodes.String() + `</g>`)
	if labels.Len() > 0 {
		content.WriteString(`<g class="pele-edge-labels" font-size="` + js.NumberToString(edgeSize) + `">` + labels.String() + `</g>`)
	}

	doc := svg.Document("block", width, height, size, options, svg.Accessible{}, content.String())
	return core.Rendered{SVG: doc, Width: width, Height: height, Links: []core.LinkInfo{}}
}

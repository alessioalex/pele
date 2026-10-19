package c4

import (
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
)

// Box is a laid out shape or boundary: top-left corner, size, and the shared shape it is drawn as.
// Boxes are compared by identity.
type Box struct {
	X, Y, W, H float64
	Form       string
}

// Route is a drawn relation.
type Route struct {
	d string
	// Where the markers go: the tip, and the way it points.
	sx, sy, sdx, sdy float64
	ex, ey, edx, edy float64
	// The stretch the label may sit on.
	x0, y0, x1, y1 float64
	// The side of that stretch the label is pushed to, as a unit vector; zero centers it on the stretch.
	nx, ny float64
	// When set, the label stays over the line and moves that way by at most this much.
	lean float64
	// The widest the label should be.
	room float64
	// The corners of the line, as x, y pairs.
	points []float64
}

const (
	loop     = 28
	lane     = 20
	laneStep = 7
	corner   = 8
	// A wide label on an upright lane leans this far out from it, to the middle of the gap between columns.
	lean = 30
	// Label widths that fit the gap between two columns, and where there is room to spare.
	narrow = 88
	wide   = 160
	// The shared person shape: a head of radius 13 centered 14 below the top, over a body that starts 26 below it.
	personHead = 26
)

var inf = math.Inf(1)

// truthy is a number's truthiness: not 0 and not NaN.
func truthy(x float64) bool { return x != 0 && !math.IsNaN(x) }

// The cap radii drawShape uses for the two cylinders.
func cylRy(w float64) float64   { return min(12, max(6, w/14)) }
func queueRx(h float64) float64 { return min(12, h/4) }

// How far along the segment from (px, py) by (dx, dy) it first meets the rectangle; Infinity when it misses.
func hitRect(x0, y0, x1, y1, px, py, dx, dy float64) float64 {
	enter := 0.0
	leave := 1.0
	if dx == 0 {
		if px < x0 || px > x1 {
			return inf
		}
	} else {
		a := (x0 - px) / dx
		b := (x1 - px) / dx
		enter = max(enter, min(a, b))
		leave = min(leave, max(a, b))
	}
	if dy == 0 {
		if py < y0 || py > y1 {
			return inf
		}
	} else {
		a := (y0 - py) / dy
		b := (y1 - py) / dy
		enter = max(enter, min(a, b))
		leave = min(leave, max(a, b))
	}
	if enter <= leave {
		return enter
	}
	return inf
}

func hitEllipse(cx, cy, rx, ry, px, py, dx, dy float64) float64 {
	ux := dx / rx
	uy := dy / ry
	vx := (px - cx) / rx
	vy := (py - cy) / ry
	a := ux*ux + uy*uy
	b := 2 * (ux*vx + uy*vy)
	c := vx*vx + vy*vy - 1
	disc := b*b - 4*a*c
	if a == 0 || disc < 0 {
		return inf
	}
	t := (-b - math.Sqrt(disc)) / (2 * a)
	if t >= 0 && t <= 1 {
		return t
	}
	if c <= 0 {
		return 0
	}
	return inf
}

// Where a segment from outside first meets the outline of a shape, as a fraction of the segment.
func hit(box *Box, px, py, qx, qy float64) float64 {
	x, y, w, h := box.X, box.Y, box.W, box.H
	dx := qx - px
	dy := qy - py
	switch box.Form {
	case "person":
		return min(hitRect(x, y+personHead, x+w, y+h, px, py, dx, dy), hitEllipse(x+w/2, y+14, 13, 13, px, py, dx, dy))
	case "cyl":
		ry := cylRy(w)
		return min(
			hitRect(x, y+ry, x+w, y+h-ry, px, py, dx, dy),
			hitEllipse(x+w/2, y+ry, w/2, ry, px, py, dx, dy),
			hitEllipse(x+w/2, y+h-ry, w/2, ry, px, py, dx, dy),
		)
	case "h-cyl":
		rx := queueRx(h)
		return min(
			hitRect(x+rx, y, x+w-rx, y+h, px, py, dx, dy),
			hitEllipse(x+rx, y+h/2, rx, h/2, px, py, dx, dy),
			hitEllipse(x+w-rx, y+h/2, rx, h/2, px, py, dx, dy),
		)
	}
	return hitRect(x, y, x+w, y+h, px, py, dx, dy)
}

func crosses(points []float64, x0, y0, x1, y1 float64) bool {
	for i := 2; i < len(points); i += 2 {
		px := points[i-2]
		py := points[i-1]
		if hitRect(x0, y0, x1, y1, px, py, points[i]-px, points[i+1]-py) <= 1 {
			return true
		}
	}
	return false
}

func contains(outer, inner *Box) bool {
	return outer.X <= inner.X && outer.Y <= inner.Y && outer.X+outer.W >= inner.X+inner.W && outer.Y+outer.H >= inner.Y+inner.H
}

// orOne is `x || 1`.
func orOne(x float64) float64 {
	if truthy(x) {
		return x
	}
	return 1
}

// The straight line from a to b: x, y of its start, then of its end.
func connect(a, b *Box, shift float64) (float64, float64, float64, float64) {
	ax := a.X + a.W/2
	ay := a.Y + a.H/2
	bx := b.X + b.W/2
	by := b.Y + b.H/2
	down := contains(a, b)
	if down || contains(b, a) {
		// One holds the other: a short line from the nearest side of the outer box.
		outer, inner := b, a
		if down {
			outer, inner = a, b
		}
		ix := inner.X + inner.W/2
		iy := inner.Y + inner.H/2
		left := inner.X - outer.X
		right := outer.X + outer.W - inner.X - inner.W
		top := inner.Y - outer.Y
		bottom := outer.Y + outer.H - inner.Y - inner.H
		least := min(left, right, top, bottom)
		var ox, oy float64
		switch least {
		case left:
			ox = outer.X
		case right:
			ox = outer.X + outer.W
		default:
			ox = ix
		}
		switch {
		case least == left || least == right:
			oy = iy
		case least == bottom:
			oy = outer.Y + outer.H
		default:
			oy = outer.Y
		}
		t := min(hit(inner, ox, oy, ix, iy), 1)
		mx := ox + (ix-ox)*t
		my := oy + (iy-oy)*t
		if down {
			return ox, oy, mx, my
		}
		return mx, my, ox, oy
	}

	px := ax
	py := ay
	qx := bx
	qy := by
	// Shapes that share a column or a row are joined by an upright or level line through the shared span.
	x0 := max(a.X, b.X)
	x1 := min(a.X+a.W, b.X+b.W)
	y0 := max(a.Y, b.Y)
	y1 := min(a.Y+a.H, b.Y+b.H)
	if x0 < x1 {
		px = (x0 + x1) / 2
		qx = px
	} else if y0 < y1 {
		py = (y0 + y1) / 2
		qy = py
	}
	l := orOne(js.Hypot(qx-px, qy-py))
	nx := (-(qy - py) / l) * shift
	ny := ((qx - px) / l) * shift
	ta := hit(a, qx+nx, qy+ny, px+nx, py+ny)
	tb := hit(b, px+nx, py+ny, qx+nx, qy+ny)
	if ta <= 1 && tb <= 1 {
		px += nx
		py += ny
		qx += nx
		qy += ny
	} else {
		px = ax
		py = ay
		qx = bx
		qy = by
		ta = min(hit(a, qx, qy, px, py), 1)
		tb = min(hit(b, px, py, qx, qy), 1)
	}
	return qx + (px-qx)*ta, qy + (py-qy)*ta, px + (qx-px)*tb, py + (qy-py)*tb
}

// Parallel stretches closer than this read as one line, and a line this close to a boundary's frame
// reads as part of it.
const (
	near      = 6
	frameNear = 3
	portStep  = 12
)

// Lanes is what the routes drawn so far take up, so that the next keeps apart from them: upright
// stretches as x, top, bottom; level ones as y, left, right; and where lines leave or reach each
// side of a shape.
type Lanes struct {
	upright []float64
	level   []float64
	frames  [2][]float64
	taken   map[*Box][]float64
	// Routes that found no lane of their own: a line left straight behind a shape, or a lane that
	// runs along another.
	squeezed int
}

func newLanes() *Lanes { return &Lanes{taken: map[*Box][]float64{}} }

func nearList(list []float64, gap, at, lo, hi float64) bool {
	for i := 0; i < len(list); i += 3 {
		if math.Abs(list[i]-at) < gap && list[i+1] < hi && list[i+2] > lo {
			return true
		}
	}
	return false
}

func (l *Lanes) free(upright bool, at, a0, a1 float64) bool {
	lo := min(a0, a1)
	hi := max(a0, a1)
	list, frames := l.level, l.frames[1]
	if upright {
		list, frames = l.upright, l.frames[0]
	}
	return !nearList(list, near, at, lo, hi) && !nearList(frames, frameNear, at, lo, hi)
}

func (l *Lanes) crossings(points []float64) int {
	count := 0
	for i := 2; i < len(points); i += 2 {
		upright := points[i] == points[i-2]
		var at, lo, hi float64
		var list []float64
		if upright {
			at = points[i]
			lo = min(points[i-1], points[i+1])
			hi = max(points[i-1], points[i+1])
			list = l.level
		} else {
			at = points[i+1]
			lo = min(points[i-2], points[i])
			hi = max(points[i-2], points[i])
			list = l.upright
		}
		for j := 0; j < len(list); j += 3 {
			if list[j] > lo && list[j] < hi && list[j+1] < at && list[j+2] > at {
				count++
			}
		}
	}
	return count
}

func (l *Lanes) frame(box *Box) {
	l.frames[0] = append(l.frames[0], box.X, box.Y, box.Y+box.H, box.X+box.W, box.Y, box.Y+box.H)
	l.frames[1] = append(l.frames[1], box.Y, box.X, box.X+box.W, box.Y+box.H, box.X, box.X+box.W)
}

func (l *Lanes) take(points []float64) {
	for i := 2; i < len(points); i += 2 {
		if points[i] == points[i-2] {
			l.upright = append(l.upright, points[i], min(points[i-1], points[i+1]), max(points[i-1], points[i+1]))
		} else if points[i+1] == points[i-1] {
			l.level = append(l.level, points[i+1], min(points[i-2], points[i]), max(points[i-2], points[i]))
		}
	}
}

// Where on one side of a shape (0 left, 1 right, 2 top, 3 bottom) a line can leave or arrive,
// between lo and hi: the places clear of the lines already there, nearest `want` first.
func (l *Lanes) ports(box *Box, side int, want, lo, hi float64) []float64 {
	used := l.taken[box]
	var out []float64
	for k := 0; k < 9 && len(out) < 4; k++ {
		sign := -1.0
		if k&1 != 0 {
			sign = 1
		}
		at := want + sign*math.Ceil(float64(k)/2)*portStep
		if at < lo || at > hi {
			continue
		}
		clear := true
		for i := 0; i < len(used); i += 2 {
			if used[i] == float64(side) && math.Abs(used[i+1]-at) < portStep {
				clear = false
			}
		}
		if clear {
			out = append(out, at)
		}
	}
	if len(out) > 0 {
		return out
	}
	return []float64{want}
}

func (l *Lanes) claim(box *Box, side int, at float64) {
	l.taken[box] = append(l.taken[box], float64(side), at)
}

func path(points []float64, startTrim, endTrim float64) string {
	n := len(points)
	ux := js.Sign(points[2] - points[0])
	uy := js.Sign(points[3] - points[1])
	var d strings.Builder
	d.WriteString("M" + svg.Num(points[0]+ux*startTrim) + "," + svg.Num(points[1]+uy*startTrim))
	for i := 2; i < n-2; i += 2 {
		x := points[i]
		y := points[i+1]
		ix := js.Sign(x - points[i-2])
		iy := js.Sign(y - points[i-1])
		ox := js.Sign(points[i+2] - x)
		oy := js.Sign(points[i+3] - y)
		r := min(corner, math.Abs(x-points[i-2]+y-points[i-1])/2, math.Abs(points[i+2]-x+points[i+3]-y)/2)
		if truthy(ix) {
			d.WriteString("H" + svg.Num(x-ix*r))
		} else {
			d.WriteString("V" + svg.Num(y-iy*r))
		}
		d.WriteString("Q" + svg.Num(x) + "," + svg.Num(y) + " " + svg.Num(x+ox*r) + "," + svg.Num(y+oy*r))
	}
	lx := js.Sign(points[n-2] - points[n-4])
	ly := js.Sign(points[n-1] - points[n-3])
	if truthy(lx) {
		d.WriteString("H" + svg.Num(points[n-2]-lx*endTrim))
	} else {
		d.WriteString("V" + svg.Num(points[n-1]-ly*endTrim))
	}
	return d.String()
}

func clearOf(points []float64, others []*Box, a, b *Box) bool {
	for _, other := range others {
		if other != a && other != b && crosses(points, other.X, other.Y, other.X+other.W, other.Y+other.H) {
			return false
		}
	}
	return true
}

// The route for a line with corners through `points`, from a's outline to b's. `along` is the
// stretch its label sits on.
func bent(points []float64, along int, startTrim, endTrim, nx, lean, room float64) *Route {
	n := len(points)
	return &Route{
		d:      path(points, startTrim, endTrim),
		sx:     points[0],
		sy:     points[1],
		sdx:    -js.Sign(points[2] - points[0]),
		sdy:    -js.Sign(points[3] - points[1]),
		ex:     points[n-2],
		ey:     points[n-1],
		edx:    js.Sign(points[n-2] - points[n-4]),
		edy:    js.Sign(points[n-1] - points[n-3]),
		x0:     points[along],
		y0:     points[along+1],
		x1:     points[along+2],
		y1:     points[along+3],
		nx:     nx,
		lean:   lean,
		room:   room,
		points: points,
	}
}

func across(lo, hi float64) []float64 {
	var out []float64
	mid := (lo + hi) / 2
	step := max(laneStep, (hi-lo)/4)
	for k := 0; k < 9; k++ {
		sign := -1.0
		if k&1 != 0 {
			sign = 1
		}
		at := mid + sign*math.Ceil(float64(k)/2)*step
		if at >= lo+near && at <= hi-near {
			out = append(out, at)
		}
	}
	return out
}

func elbow(a, b *Box, others []*Box, lanes *Lanes, startTrim, endTrim float64) *Route {
	ax := a.X + a.W/2
	ay := a.Y + a.H/2
	bx := b.X + b.W/2
	by := b.Y + b.H/2
	var bestPoints []float64
	var bestCost float64
	var bestUpright bool
	var bestSides [2]int
	found := false
	// Reports whether the route became the best.
	consider := func(points []float64, upright bool, sides [2]int) bool {
		if !clearOf(points, others, a, b) {
			return false
		}
		for i := 2; i < len(points); i += 2 {
			up := points[i] == points[i-2]
			var ok bool
			if up {
				ok = lanes.free(up, points[i], points[i-1], points[i+1])
			} else {
				ok = lanes.free(up, points[i+1], points[i-2], points[i])
			}
			if !ok {
				return false
			}
		}
		cost := float64(lanes.crossings(points)) * 1e6
		for i := 2; i < len(points); i += 2 {
			cost += math.Abs(points[i]-points[i-2]) + math.Abs(points[i+1]-points[i-1])
		}
		if !found || cost < bestCost {
			found = true
			bestPoints, bestCost, bestUpright, bestSides = points, cost, upright, sides
			return true
		}
		return false
	}
	if a.X+a.W <= b.X || b.X+b.W <= a.X {
		right := bx > ax
		down := js.Sign(by - ay)
		sides := [2]int{0, 1}
		lo, hi := b.X+b.W, a.X
		if right {
			sides = [2]int{1, 0}
			lo, hi = a.X+a.W, b.X
		}
		lanesAcross := across(lo, hi)
	search1:
		for _, y0 := range lanes.ports(a, sides[0], ay+(down*a.H)/4, a.Y+8, a.Y+a.H-8) {
			for _, y1 := range lanes.ports(b, sides[1], by-(down*b.H)/4, b.Y+8, b.Y+b.H-8) {
				for _, ln := range lanesAcross {
					x0 := ln + (ax-ln)*min(hit(a, ln, y0, ax, y0), 1)
					x1 := ln + (bx-ln)*min(hit(b, ln, y1, bx, y1), 1)
					if consider([]float64{x0, y0, ln, y0, ln, y1, x1, y1}, true, sides) {
						break search1
					}
				}
			}
		}
	}
	if a.Y+a.H <= b.Y || b.Y+b.H <= a.Y {
		below := by > ay
		sideways := js.Sign(bx - ax)
		sides := [2]int{2, 3}
		lo, hi := b.Y+b.H, a.Y
		if below {
			sides = [2]int{3, 2}
			lo, hi = a.Y+a.H, b.Y
		}
		lanesAcross := across(lo, hi)
	search2:
		for _, x0 := range lanes.ports(a, sides[0], ax+(sideways*a.W)/4, a.X+8, a.X+a.W-8) {
			for _, x1 := range lanes.ports(b, sides[1], bx-(sideways*b.W)/4, b.X+8, b.X+b.W-8) {
				for _, ln := range lanesAcross {
					y0 := ln + (ay-ln)*min(hit(a, x0, ln, x0, ay), 1)
					y1 := ln + (by-ln)*min(hit(b, x1, ln, x1, by), 1)
					if consider([]float64{x0, y0, x0, ln, x1, ln, x1, y1}, false, sides) {
						break search2
					}
				}
			}
		}
	}
	if !found {
		return nil
	}
	points := bestPoints
	lanes.take(points)
	if bestUpright {
		lanes.claim(a, bestSides[0], points[1])
		lanes.claim(b, bestSides[1], points[7])
		// The label sits on the middle stretch, across the gap.
		return bent(points, 2, startTrim, endTrim, 0, 0, narrow)
	}
	lanes.claim(a, bestSides[0], points[0])
	lanes.claim(b, bestSides[1], points[6])
	return bent(points, 2, startTrim, endTrim, 0, 0, wide)
}

type way struct {
	points []float64
	side   int
	p0, p1 float64
	step   int
}

// Routes a relation from a to b. `shift` moves the line sideways when another runs the opposite way.
// `others` are the shapes to steer clear of; steer false skips that check. `lanes` holds what the
// routes before it take up, so that this one keeps apart from them.
func route(a, b *Box, shift float64, others []*Box, steer bool, lanes *Lanes, startTrim, endTrim float64) *Route {
	if a == b {
		spread := min(a.H/2-4, 10)
		l := js.Hypot(loop, spread)
		x := a.X + a.W
		cy := a.Y + a.H/2
		ux := loop / l
		uy := spread / l
		return &Route{
			d: "M" + svg.Num(x+ux*startTrim) + "," + svg.Num(cy-spread-uy*startTrim) + "C" + svg.Num(x+loop) + "," + svg.Num(cy-spread*2) + " " +
				svg.Num(x+loop) + "," + svg.Num(cy+spread*2) + " " + svg.Num(x+ux*endTrim) + "," + svg.Num(cy+spread+uy*endTrim),
			sx:   x,
			sy:   cy - spread,
			sdx:  -ux,
			sdy:  uy,
			ex:   x,
			ey:   cy + spread,
			edx:  -ux,
			edy:  -uy,
			x0:   x + loop + 4,
			y0:   cy,
			x1:   x + loop + 4,
			y1:   cy,
			nx:   1,
			room: 64,
		}
	}

	sx, sy, ex, ey := connect(a, b, shift)
	upright := sx == ex
	blocked := false
	if steer && !contains(a, b) && !contains(b, a) {
		for _, other := range others {
			if other != a && other != b && hitRect(other.X, other.Y, other.X+other.W, other.Y+other.H, sx, sy, ex-sx, ey-sy) <= 1 {
				blocked = true
			}
		}
	}
	if steer && blocked && !upright && sy != ey {
		if around := elbow(a, b, others, lanes, startTrim, endTrim); around != nil {
			return around
		}
		lanes.squeezed++
	}
	if steer && blocked && (upright || sy == ey) {
		laneOn := func(forward bool) float64 {
			ln := inf
			if forward {
				ln = -inf
			}
			widen := func(box *Box) {
				edge := box.Y
				size := box.H
				if upright {
					edge, size = box.X, box.W
				}
				if forward {
					ln = max(ln, edge+size)
				} else {
					ln = min(ln, edge)
				}
			}
			widen(a)
			widen(b)
			for _, other := range others {
				if other != a && other != b && hitRect(other.X, other.Y, other.X+other.W, other.Y+other.H, sx, sy, ex-sx, ey-sy) <= 1 {
					widen(other)
				}
			}
			return ln
		}
		var lo, hi float64
		if upright {
			lo = min(a.Y, b.Y)
			hi = max(a.Y+a.H, b.Y+b.H)
		} else {
			lo = min(a.X, b.X)
			hi = max(a.X+a.W, b.X+b.W)
		}
		outside := func(forward bool, ln float64) bool {
			for _, other := range others {
				if other == a || other == b {
					continue
				}
				var isNear bool
				var start, end float64
				if upright {
					isNear = other.Y < hi && other.Y+other.H > lo
					start = other.X
					end = start + other.W
				} else {
					isNear = other.X < hi && other.X+other.W > lo
					start = other.Y
					end = start + other.H
				}
				if isNear && ((forward && end > ln) || (!forward && start < ln)) {
					return false
				}
			}
			return true
		}
		ax := a.X + a.W/2
		ay := a.Y + a.H/2
		bx := b.X + b.W/2
		by := b.Y + b.H/2
		towards := js.Sign(bx - ax)
		if upright {
			towards = js.Sign(by - ay)
		}
		// The route on one side: each end leaves its shape a quarter of the way from the middle towards
		// the other, clear of the straight lines that meet the middle of that side and of other routes
		// there. The lane runs a little further out for each route already along that gap, and never
		// behind a shape. `step` is how many lanes out it went; 5 when none was free.
		along := func(forward bool) way {
			ln := laneOn(forward)
			sign := -1.0
			if forward {
				sign = 1
			}
			var side int
			var p0, p1 float64
			if upright {
				side = 0
				if forward {
					side = 1
				}
				p0 = lanes.ports(a, side, ay+(towards*a.H)/4, a.Y+8, a.Y+a.H-8)[0]
				p1 = lanes.ports(b, side, by-(towards*b.H)/4, b.Y+8, b.Y+b.H-8)[0]
			} else {
				side = 2
				if forward {
					side = 3
				}
				p0 = lanes.ports(a, side, ax+(towards*a.W)/4, a.X+8, a.X+a.W-8)[0]
				p1 = lanes.ports(b, side, bx-(towards*b.W)/4, b.X+8, b.X+b.W-8)[0]
			}
			pointsAt := func(at float64) []float64 {
				if upright {
					x0 := at + (ax-at)*min(hit(a, at, p0, ax, p0), 1)
					x1 := at + (bx-at)*min(hit(b, at, p1, bx, p1), 1)
					return []float64{x0, p0, at, p0, at, p1, x1, p1}
				}
				y0 := at + (ay-at)*min(hit(a, p0, at, p0, ay), 1)
				y1 := at + (by-at)*min(hit(b, p1, at, p1, by), 1)
				return []float64{p0, y0, p0, at, p1, at, p1, y1}
			}
			points := pointsAt(ln + sign*lane)
			for k := 0; k <= 4; k++ {
				at := ln + sign*(lane+float64(k)*laneStep)
				tried := pointsAt(at)
				if !clearOf(tried, others, a, b) {
					continue
				}
				points = tried
				if lanes.free(upright, at, p0, p1) {
					return way{points, side, p0, p1, k}
				}
			}
			return way{points, side, p0, p1, 5}
		}
		// A detour takes the outside of the drawing when only one side is outside, leaving the gaps
		// between columns and rows to the relations that cross them. Otherwise it takes the side where
		// it crosses fewer lines, then the one where it runs nearer its shapes, so that detours spread
		// over both sides.
		var natural bool
		if upright {
			natural = ey > sy
		} else {
			natural = ex < sx
		}
		out := outside(natural, laneOn(natural))
		back := outside(!natural, laneOn(!natural))
		first := along(natural)
		var other *way
		if out == back {
			o := along(!natural)
			other = &o
		}
		cost := func(w way) int { return lanes.crossings(w.points)*8 + w.step }
		var forward bool
		switch {
		case back && !out:
			forward = !natural
		case other != nil && cost(*other) < cost(first):
			forward = !natural
		default:
			forward = natural
		}
		var chosen way
		switch {
		case forward == natural:
			chosen = first
		case other != nil:
			chosen = *other
		default:
			chosen = along(!natural)
		}
		sign := -1.0
		if forward {
			sign = 1
		}
		if chosen.step > 4 {
			lanes.squeezed++
		}
		lanes.take(chosen.points)
		lanes.claim(a, chosen.side, chosen.p0)
		lanes.claim(b, chosen.side, chosen.p1)
		if upright {
			return bent(chosen.points, 2, startTrim, endTrim, sign, lean, narrow)
		}
		return bent(chosen.points, 2, startTrim, endTrim, 0, 0, wide)
	}

	l := orOne(js.Hypot(ex-sx, ey-sy))
	dx := (ex - sx) / l
	dy := (ey - sy) / l
	if upright || sy == ey {
		lanes.take([]float64{sx, sy, ex, ey})
	}
	r := &Route{
		d:      "M" + svg.Num(sx+dx*startTrim) + "," + svg.Num(sy+dy*startTrim) + "L" + svg.Num(ex-dx*endTrim) + "," + svg.Num(ey-dy*endTrim),
		sx:     sx,
		sy:     sy,
		sdx:    -dx,
		sdy:    -dy,
		ex:     ex,
		ey:     ey,
		edx:    dx,
		edy:    dy,
		x0:     sx,
		y0:     sy,
		x1:     ex,
		y1:     ey,
		room:   wide,
		points: []float64{sx, sy, ex, ey},
	}
	if truthy(shift) {
		r.nx = -dy
		r.ny = dx
	}
	if math.Abs(dx) > math.Abs(dy) {
		r.room = narrow
	}
	return r
}

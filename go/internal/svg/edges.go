package svg

import (
	"math"
	"slices"
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/text"
)

const Arrow = 8

// EdgePath is a drawn edge: its path, and where and which way each end points. The start's
// direction points back out of the path, the end's along it.
type EdgePath struct {
	D   string
	Sx  float64
	Sy  float64
	Sdx float64
	Sdy float64
	Ex  float64
	Ey  float64
	Edx float64
	Edy float64
}

// Marker types are Mermaid's: arrow_point, arrow_circle, arrow_cross; anything else draws nothing.

// The most a corner is rounded by in a stepped route.
const edgeCorner = 12

// How far the handles of a wide turn reach along its two legs. A little more than a circle's,
// which makes the turn fuller. A variable, so that 1 - handle rounds as it does in JS.
var handle = 0.62

// Lead is how far an edge runs straight before a marker at its end, where it has the room.
const Lead = 6

// ease is ease() from src/layout/layered.ts.
func ease(across, along float64) float64 {
	d := math.Abs(across)
	if d == 0 || d != d {
		d = 1
	}
	return max(0.25, min(0.5, (4*math.Abs(along))/d))
}

// Reach is what SideReach returns.
type Reach struct {
	reach map[int]float64
}

// SideReach tracks how far from each side of each node the edges that meet it start to turn: as
// far as the largest marker there needs, for every edge on the side, so that none turns sooner
// than its neighbour and crosses it.
func SideReach() *Reach { return &Reach{reach: map[int]float64{}} }

// Add takes a node, the side of it, and the marker of an end there.
func (r *Reach) Add(node, side int, typ string) {
	trim := MarkerTrim(typ)
	if side >= 0 && trim > 0 && trim+Lead > r.reach[node*4+side] {
		r.reach[node*4+side] = trim + Lead
	}
}

// Of is the reach of a side of a node, 0 for side -1.
func (r *Reach) Of(node, side int) float64 {
	if side < 0 {
		return 0
	}
	return r.reach[node*4+side]
}

func signOr1(v float64) float64 {
	s := js.Sign(v)
	if s == 0 || s != s {
		return 1
	}
	return s
}

func defaultReach(trim float64) float64 {
	if trim > 0 {
		return trim + Lead
	}
	return 0
}

// RoutePath is RoutePathReach with the TS defaults for the reaches: far enough to clear the
// end's own marker (trim + Lead, or 0 for no trim).
func RoutePath(route []float64, curve string, startTrim, endTrim float64) EdgePath {
	return RoutePathReach(route, curve, startTrim, endTrim, defaultReach(startTrim), defaultReach(endTrim))
}

// RoutePathReach builds the path for a route of x, y, axis triples, trimming both ends to leave
// room for markers. `curve` "" is undefined.
// Between two points that are not in line, a route is one S that leaves and arrives along the
// flow, or a step in a stepped route. Where the route itself turns a corner, a stepped route
// rounds it a little, and any other but a linear one turns as widely as it can: the turn takes
// the whole of a leg that ends at a node and half of a leg it shares with the next turn.
// `startReach` and `endReach` say how far from its nodes the edge starts to turn, where the
// other edges on the same side of a node need it to.
func RoutePathReach(route []float64, curve string, startTrim, endTrim, startReach, endReach float64) EdgePath {
	linear := curve == "linear"
	stepped := curve == "step" || curve == "stepBefore" || curve == "stepAfter"
	n := (len(route)+2)/3 + 2
	xs := make([]float64, 0, n)
	ys := make([]float64, 0, n)
	axes := make([]float64, 0, n)
	near := func(ax, ay, bx, by float64) bool { return math.Abs(ax-bx) < 0.01 && math.Abs(ay-by) < 0.01 }
	for i := 0; i < len(route); i += 3 {
		x := route[i]
		y := at(route, i+1)
		axis := at(route, i+2)
		last := len(xs) - 1
		// A stepped route turns at right angles where a smooth one would curve.
		if stepped && last >= 0 && math.Abs(x-xs[last]) >= 0.01 && math.Abs(y-ys[last]) >= 0.01 {
			px := xs[last]
			py := ys[last]
			my, mx := (py+y)/2, (px+x)/2
			if curve == "stepBefore" {
				my, mx = py, px
			} else if curve == "stepAfter" {
				my, mx = y, x
			}
			turns := [4]float64{mx, py, mx, y}
			if axis == 0 {
				turns = [4]float64{px, my, x, my}
			}
			for k := 0; k < 4; k += 2 {
				if near(turns[k], turns[k+1], px, py) || near(turns[k], turns[k+1], x, y) {
					continue
				}
				xs = append(xs, turns[k])
				ys = append(ys, turns[k+1])
				axes = append(axes, axis)
			}
		}
		xs = append(xs, x)
		ys = append(ys, y)
		axes = append(axes, axis)
	}
	last := len(xs) - 1
	X := func(i int) float64 { return at(xs, i) }
	Y := func(i int) float64 { return at(ys, i) }
	aligned := func(i int) bool { return math.Abs(X(i)-X(i-1)) < 0.01 || math.Abs(Y(i)-Y(i-1)) < 0.01 }
	straight := func(i int) bool { return linear || aligned(i) }
	// The way the route heads from point i to point j: along the flow where an S joins them.
	unit := func(i, j int, axis float64) (float64, float64) {
		if !straight(max(i, j)) {
			if axis == 0 {
				return 0, signOr1(Y(j) - Y(i))
			}
			return signOr1(X(j) - X(i)), 0
		}
		dx := X(j) - X(i)
		dy := Y(j) - Y(i)
		l := js.Hypot(dx, dy)
		if l == 0 || l != l {
			l = 1
		}
		return dx / l, dy / l
	}
	sdx, sdy := unit(0, 1, at(axes, 1))
	edx, edy := unit(last-1, last, at(axes, last))
	sx, sy := X(0), Y(0)
	ex, ey := X(last), Y(last)
	if len(xs) == 0 {
		// JS writes xs[0] of an empty route, which gives it one point of NaN.
		xs = append(xs, math.NaN())
		ys = append(ys, math.NaN())
	} else {
		xs[0] += float64(sdx * startTrim)
		ys[0] += float64(sdy * startTrim)
		xs[last] -= float64(edx * endTrim)
		ys[last] -= float64(edy * endTrim)
	}
	length := func(i int) float64 { return math.Abs(X(i)-X(i-1)) + math.Abs(Y(i)-Y(i-1)) }
	// Whether the route turns a corner at point i, from running one way to running the other.
	turns := func(i int) bool {
		return !linear && i > 0 && i < last && aligned(i) && aligned(i+1) && length(i) >= 0.01 && length(i+1) >= 0.01 &&
			(math.Abs(xs[i]-xs[i-1]) < 0.01) != (math.Abs(xs[i+1]-xs[i]) < 0.01)
	}
	// An edge that curves into a marker runs straight for a little way first, where it has the
	// room, so that the marker does not sit on the bend.
	along := func(i int, ux, uy float64) float64 {
		return float64(math.Abs(X(i)-X(i-1))*math.Abs(ux)) + float64(math.Abs(Y(i)-Y(i-1))*math.Abs(uy))
	}
	// Of the way from the end of a run to where it turns, the part that makes up for a larger
	// marker beside this one is kept where it can be, and the rest is at most a quarter of what
	// is left, so that ends on one side of a node turn equally far from it.
	lead := func(reach, trim, run float64) float64 {
		room := min(max(0, reach-Lead-trim), run/4)
		return room + min(reach-trim-room, (run-room)/4)
	}
	arrives := 0.0
	if endReach > endTrim && (!straight(last) || turns(last-1)) {
		arrives = lead(endReach, endTrim, along(last, edx, edy))
	}
	leaves := 0.0
	if startReach > startTrim && (!straight(1) || turns(1)) {
		leaves = lead(startReach, startTrim, along(1, sdx, sdy))
	}
	if arrives >= 1 {
		x := xs[last] - float64(edx*arrives)
		y := ys[last] - float64(edy*arrives)
		axis := axes[last]
		xs = slices.Insert(xs, last, x)
		ys = slices.Insert(ys, last, y)
		axes = slices.Insert(axes, last, axis)
		last++
	}
	if leaves >= 1 {
		x := xs[0] + float64(sdx*leaves)
		y := ys[0] + float64(sdy*leaves)
		axis := axes[1]
		xs = slices.Insert(xs, 1, x)
		ys = slices.Insert(ys, 1, y)
		axes = slices.Insert(axes, 1, axis)
		last++
	}

	var d strings.Builder
	d.WriteString("M" + Num(xs[0]) + "," + Num(ys[0]))
	atX := xs[0]
	atY := ys[0]
	lineTo := func(x, y float64) {
		if near(x, y, atX, atY) {
			return
		}
		d.WriteString("L" + Num(x) + "," + Num(y))
		atX = x
		atY = y
	}
	keep := 1 - handle
	for i := 1; i <= last; i++ {
		x := xs[i]
		y := ys[i]
		if !straight(i) {
			down := axes[i] == 0
			along, across := x-atX, y-atY
			if down {
				along, across = y-atY, x-atX
			}
			k := float64(math.Abs(along) * ease(across, along))
			ux, uy := float64(js.Sign(along)*k), 0.0
			if down {
				ux, uy = 0, float64(js.Sign(along)*k)
			}
			d.WriteString("C" + Num(atX+ux) + "," + Num(atY+uy) + " " + Num(x-ux) + "," + Num(y-uy) + " " + Num(x) + "," + Num(y))
			atX = x
			atY = y
			continue
		}
		if !turns(i) {
			// A point part of the way along one straight run adds nothing to the path.
			if i < last && aligned(i) && aligned(i+1) && float64((x-xs[i-1])*(xs[i+1]-x))+float64((y-ys[i-1])*(ys[i+1]-y)) > 0 {
				continue
			}
			lineTo(x, y)
			continue
		}
		// How much of the leg before the turn and of the leg after it the turn takes: all of one
		// that ends at a node, short of the straight run into a marker, and half of one it shares.
		before := length(i)
		if turns(i - 1) {
			before = length(i) / 2
		}
		after := length(i + 1)
		if turns(i + 1) {
			after = length(i+1) / 2
		}
		if stepped {
			before = min(edgeCorner, before, after)
			after = before
		}
		ux, uy := unit(i-1, i, axes[i])
		vx, vy := unit(i, i+1, axes[i+1])
		lineTo(x-float64(ux*before), y-float64(uy*before))
		atX = x + float64(vx*after)
		atY = y + float64(vy*after)
		if stepped {
			d.WriteString("Q" + Num(x) + "," + Num(y) + " " + Num(atX) + "," + Num(atY))
		} else {
			d.WriteString("C" + Num(x-float64(ux*before*keep)) + "," + Num(y-float64(uy*before*keep)) + " " +
				Num(x+float64(vx*after*keep)) + "," + Num(y+float64(vy*after*keep)) + " " + Num(atX) + "," + Num(atY))
		}
	}
	return EdgePath{D: d.String(), Sx: sx, Sy: sy, Sdx: -sdx, Sdy: -sdy, Ex: ex, Ey: ey, Edx: edx, Edy: edy}
}

// MarkerTrim is how far a marker reaches back from the end of its edge.
func MarkerTrim(typ string) float64 {
	switch typ {
	case "arrow_point":
		return Arrow - 1
	case "arrow_circle":
		return Arrow
	case "arrow_cross":
		return 4
	}
	return 0
}

// Marker draws a marker whose tip is at (x, y), pointing along (dx, dy).
func Marker(typ string, x, y, dx, dy float64, color string) string {
	nx := -dy
	ny := dx
	switch typ {
	case "arrow_point":
		bx := x - float64(dx*Arrow)
		by := y - float64(dy*Arrow)
		const half = Arrow * 0.42
		return `<path class="pele-marker" d="M` + Num(x) + "," + Num(y) + "L" + Num(bx+float64(nx*half)) + "," + Num(by+float64(ny*half)) +
			"L" + Num(bx-float64(nx*half)) + "," + Num(by-float64(ny*half)) + `Z" fill="` + color + `" stroke="none"/>`
	case "arrow_circle":
		return `<circle class="pele-marker" cx="` + Num(x-float64(dx*4)) + `" cy="` + Num(y-float64(dy*4)) + `" r="3.5" fill="` + color + `" stroke="none"/>`
	case "arrow_cross":
		cx := x - float64(dx*4)
		cy := y - float64(dy*4)
		const a = 3.5
		ux := float64((dx + nx) * a)
		uy := float64((dy + ny) * a)
		vx := float64((dx - nx) * a)
		vy := float64((dy - ny) * a)
		return `<path class="pele-marker" d="M` + Num(cx-ux) + "," + Num(cy-uy) + "L" + Num(cx+ux) + "," + Num(cy+uy) +
			"M" + Num(cx-vx) + "," + Num(cy-vy) + "L" + Num(cx+vx) + "," + Num(cy+vy) + `"/>`
	}
	return ""
}

// EdgeLabelSvg draws an edge label on a patch of the background, so the line does not run
// through the text. `icons` may be nil.
func EdgeLabelSvg(id string, label text.Label, x, y float64, attrs string, icons text.IconResolver) string {
	w := label.Width + 8
	h := label.Height
	return `<g class="pele-edge-label" data-id="` + EscText(id) + `">` +
		`<rect x="` + Num(x-w/2) + `" y="` + Num(y-h/2) + `" width="` + Num(w) + `" height="` + Num(h) + `" rx="3" fill="var(--_bg)"/>` +
		LabelSvg(label, x, y, attrs, icons) +
		"</g>"
}

// LoopPath is a self-loop beside a node centered on (c.X, c.Y) of drawn size w by h: out of the
// side that lies across the flow and back in. `k` counts the loops already on the node, so each
// one reaches a little further. It also places the edge's label, labelW by labelH, beyond the
// loop: TS writes labelX and labelY into the edge it is given.
func LoopPath(c Box, w, h, k float64, sideways bool, base, trim, labelW, labelH float64) (path EdgePath, labelX, labelY float64) {
	half := pick(sideways, w, h) / 2
	spread := min(half-4, 8+float64(k*6))
	reach := base + float64(k*8)
	l := js.Hypot(reach, spread)
	if l == 0 || l != l {
		l = 1
	}
	// Local frame: `out` points away from the node, `along` runs along its side.
	pt := func(out, along float64) string {
		if sideways {
			return Num(c.X+along) + "," + Num(c.Y+h/2+out)
		}
		return Num(c.X+w/2+out) + "," + Num(c.Y+along)
	}
	tx := float64((reach / l) * trim)
	ty := float64((spread / l) * trim)
	if sideways {
		labelX = c.X
		labelY = c.Y + h/2 + reach + 4 + labelH/2
	} else {
		labelX = c.X + w/2 + reach + 4 + labelW/2
		labelY = c.Y
	}
	path = EdgePath{
		D:   "M" + pt(0, -spread) + "C" + pt(reach, -spread*2) + " " + pt(reach, spread*2) + " " + pt(tx, spread+ty),
		Sx:  pick(sideways, c.X-spread, c.X+w/2),
		Sy:  pick(sideways, c.Y+h/2, c.Y-spread),
		Sdx: pick(sideways, 0, -1),
		Sdy: pick(sideways, -1, 0),
		Ex:  pick(sideways, c.X+spread, c.X+w/2),
		Ey:  pick(sideways, c.Y+h/2, c.Y+spread),
		Edx: pick(sideways, -spread/l, -reach/l),
		Edy: pick(sideways, -reach/l, -spread/l),
	}
	return path, labelX, labelY
}

package venn

import (
	"math"
	"slices"
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
)

// Circle is the circle of one set.
type Circle struct {
	ID string
	X  float64
	Y  float64
	R  float64
}

// Spot is where the content of a region goes: the point inside all of `inside` and outside every
// other circle that is farthest from their outlines, and how far that is. A margin of 0 or less
// means the circles as placed leave the region empty.
type Spot struct {
	X      float64
	Y      float64
	Margin float64
}

const tau = math.Pi * 2

// setsKey is one key for a sorted list of ids. Ids never hold a quote, so this is one-to-one.
func setsKey(ids []string) string {
	return `["` + strings.Join(ids, `","`) + `"]`
}

// ensurePairwiseSubsets adds, for every overlap of three or more sets, the pairwise overlaps it
// implies and the diagram does not state, a quarter of the smaller set each, so that the circles
// are drawn overlapping. It returns the list it was given when there is nothing to add.
func ensurePairwiseSubsets(subsets []Subset) []Subset {
	present := map[string]bool{}
	sizes := map[string]float64{}
	for _, subset := range subsets {
		sorted := append([]string(nil), subset.Sets...)
		sortUnits(sorted)
		present[setsKey(sorted)] = true
		if len(subset.Sets) == 1 {
			sizes[subset.Sets[0]] = subset.Size
		}
	}
	var added []Subset
	empty := ""
	for _, subset := range subsets {
		if len(subset.Sets) < 3 {
			continue
		}
		members := append([]string(nil), subset.Sets...)
		sortUnits(members)
		for i := 0; i < len(members)-1; i++ {
			for j := i + 1; j < len(members); j++ {
				pair := []string{members[i], members[j]}
				key := setsKey(pair)
				if present[key] {
					continue
				}
				present[key] = true
				a, okA := sizes[pair[0]]
				b, okB := sizes[pair[1]]
				size := 2.5
				if okA && okB {
					size = math.Min(a, b) / 4
				}
				added = append(added, Subset{Sets: pair, Size: size, Label: &empty})
			}
		}
	}
	if len(added) > 0 {
		return append(append([]Subset(nil), subsets...), added...)
	}
	return subsets
}

// lensArea is the area two circles share when their centers are d apart.
func lensArea(r1, r2, d float64) float64 {
	if d >= r1+r2 {
		return 0
	}
	small := math.Min(r1, r2)
	if d <= math.Abs(r1-r2) {
		return math.Pi * small * small
	}
	a1 := r1 * r1 * js.Acos((d*d+r1*r1-r2*r2)/(2*d*r1))
	a2 := r2 * r2 * js.Acos((d*d+r2*r2-r1*r1)/(2*d*r2))
	a3 := 0.5 * math.Sqrt(math.Max((-d+r1+r2)*(d+r1-r2)*(d-r1+r2)*(d+r1+r2), 0))
	return a1 + a2 - a3
}

// distanceForOverlap is how far apart two circles sit for their overlap to have the given area.
// The area falls as the distance grows, so bisection finds it.
func distanceForOverlap(r1, r2, area float64) float64 {
	low := math.Abs(r1 - r2)
	high := r1 + r2
	if !(area > 0) {
		return high
	}
	if area >= math.Pi*js.Pow(math.Min(r1, r2), 2) {
		return low
	}
	for i := 0; i < 48; i++ {
		middle := (low + high) / 2
		if lensArea(r1, r2, middle) > area {
			low = middle
		} else {
			high = middle
		}
	}
	return (low + high) / 2
}

// margin is how far a point is from the nearest outline it must stay within or without. It gives
// up, with some value below floor, as soon as the point cannot reach floor.
func margin(circles []Circle, inside, outside []int, x, y, floor float64) float64 {
	best := math.Inf(1)
	for _, i := range inside {
		c := &circles[i]
		dx := x - c.X
		dy := y - c.Y
		room := c.R - math.Sqrt(dx*dx+dy*dy)
		if room < best {
			if room < floor {
				return room
			}
			best = room
		}
	}
	for _, i := range outside {
		c := &circles[i]
		dx := x - c.X
		dy := y - c.Y
		room := math.Sqrt(dx*dx+dy*dy) - c.R
		if room < best {
			if room < floor {
				return room
			}
			best = room
		}
	}
	return best
}

// findSpot finds the roomiest point of a region by sampling the smallest of its circles on a grid
// and then walking uphill from the best sample. Ties go to the point nearest the middle of the
// region, so a symmetric arrangement gets symmetric labels.
func findSpot(circles []Circle, inside, outside []int) Spot {
	cx := 0.0
	cy := 0.0
	smallest := circles[inside[0]]
	count := float64(len(inside))
	for _, i := range inside {
		c := circles[i]
		cx += c.X / count
		cy += c.Y / count
		if c.R < smallest.R {
			smallest = c
		}
	}
	r := smallest.R
	x := cx
	y := cy
	best := margin(circles, inside, outside, x, y, math.Inf(-1))
	off := 0.0
	consider := func(px, py float64) bool {
		m := margin(circles, inside, outside, px, py, best-r*1e-9)
		if !(m > best-r*1e-9) {
			return false
		}
		distance := js.Hypot(px-cx, py-cy)
		if m > best+r*1e-9 || distance < off-r*1e-9 {
			best = m
			off = distance
			x = px
			y = py
			return true
		}
		return false
	}
	const grid = 10
	for gy := -grid; gy <= grid; gy++ {
		for gx := -grid; gx <= grid; gx++ {
			if gx*gx+gy*gy <= grid*grid {
				consider(smallest.X+(float64(gx)*r)/grid, smallest.Y+(float64(gy)*r)/grid)
			}
		}
	}
	step := r / grid
	for round := 0; round < 40 && step > r*1e-6; round++ {
		moved := false
		for k := 0; k < 8; k++ {
			angle := (float64(k) * tau) / 8
			if consider(x+js.Cos(angle)*step, y+js.Sin(angle)*step) {
				moved = true
			}
		}
		if !moved {
			step /= 2
		}
	}
	return Spot{X: x, Y: y, Margin: best}
}

// layoutCircles places one circle per set, with areas in proportion to the sizes of the sets and
// each pair as far apart as its overlap asks: one, two and three sets exactly, more by relaxing a
// ring of circles toward those distances. Lengths are in units of the square root of a size.
func layoutCircles(subsets []Subset) []Circle {
	var order []string
	sizes := map[string]float64{}
	for _, subset := range subsets {
		for _, id := range subset.Sets {
			if _, ok := sizes[id]; !ok {
				order = append(order, id)
				sizes[id] = 10
			}
		}
	}
	for _, subset := range subsets {
		if len(subset.Sets) == 1 {
			sizes[subset.Sets[0]] = subset.Size
		}
	}

	index := map[string]int{}
	circles := make([]Circle, 0, len(order))
	largest := 0.0
	for _, id := range order {
		if size := sizes[id]; js.IsFinite(size) && size > largest {
			largest = size
		}
	}
	if largest == 0 {
		largest = 10
	}
	widest := math.Sqrt(largest / math.Pi)
	for _, id := range order {
		size := sizes[id]
		index[id] = len(circles)
		// A set too small to hold its name, or with no size at all, still gets a circle that shows.
		r := 0.0
		if js.IsFinite(size) && size > 0 {
			r = math.Sqrt(size / math.Pi)
		}
		circles = append(circles, Circle{ID: id, R: math.Max(r, widest*0.3)})
	}
	n := len(circles)
	if n < 2 {
		return circles
	}

	// The overlap each pair was given, by the indexes of its circles.
	overlaps := map[int]float64{}
	var overlapKeys []int
	for _, subset := range ensurePairwiseSubsets(subsets) {
		if len(subset.Sets) != 2 {
			continue
		}
		a := index[subset.Sets[0]]
		b := index[subset.Sets[1]]
		if a != b {
			key := min(a, b)*n + max(a, b)
			if _, ok := overlaps[key]; !ok {
				overlapKeys = append(overlapKeys, key)
			}
			overlaps[key] = subset.Size
		}
	}
	// How far apart each overlapping pair should be. Pairs that do not overlap only have to keep a gap.
	wanted := make(map[int]float64, len(overlapKeys))
	for _, key := range overlapKeys {
		wanted[key] = distanceForOverlap(circles[key/n].R, circles[key%n].R, overlaps[key])
	}
	gap := widest * 0.15
	distance := func(a, b int) float64 {
		if d, ok := wanted[a*n+b]; ok {
			return d
		}
		return circles[a].R + circles[b].R + gap
	}

	if n == 2 {
		circles[1].X = distance(0, 1)
		return circles
	}

	if n == 3 {
		// Two sets side by side and the third below, the corners of a triangle whose sides are the
		// three distances. An overlap of all three has to exist, so the triangle shrinks until it does.
		ab := distance(0, 1)
		ac := distance(0, 2)
		bc := distance(1, 2)
		triple := false
		for _, subset := range subsets {
			if distinct(subset.Sets) == 3 {
				triple = true
				break
			}
		}
		for attempt := 0; attempt < 40; attempt++ {
			x := 0.0
			if ab > 0 {
				x = (ab*ab + ac*ac - bc*bc) / (2 * ab)
			}
			circles[1].X = ab
			circles[2].X = math.Max(-ac, math.Min(ac, x))
			circles[2].Y = math.Sqrt(math.Max(ac*ac-x*x, 0))
			if !triple || findSpot(circles, []int{0, 1, 2}, nil).Margin > widest*0.12 {
				break
			}
			ab *= 0.94
			ac *= 0.94
			bc *= 0.94
		}
		return circles
	}

	total := 0.0
	for _, c := range circles {
		total += 2*c.R + gap
	}
	rounds := int(math.Min(300, math.Floor(3e6/float64(n*n))))
	if n <= 64 {
		ring := total / tau
		for i := range circles {
			angle := (float64(i)*tau)/float64(n) - math.Pi/2
			circles[i].X = js.Cos(angle) * ring
			circles[i].Y = js.Sin(angle) * ring
		}
	} else {
		columns := int(math.Ceil(math.Sqrt(float64(n))))
		cell := 2*widest + gap
		for i := range circles {
			circles[i].X = float64(i%columns) * cell
			circles[i].Y = float64(i/columns) * cell
		}
	}
	dx := make([]float64, n)
	dy := make([]float64, n)
	pulls := make([]int, n)
	for round := 0; round < rounds; round++ {
		clear(dx)
		clear(dy)
		clear(pulls)
		for a := 0; a < n; a++ {
			for b := a + 1; b < n; b++ {
				overlap, has := wanted[a*n+b]
				apart := circles[a].R + circles[b].R + gap
				ux := circles[b].X - circles[a].X
				uy := circles[b].Y - circles[a].Y
				length := js.Hypot(ux, uy)
				if !has && length >= apart {
					continue
				}
				if length < 1e-9 {
					ux = js.Cos(float64(a + b))
					uy = js.Sin(float64(a + b))
					length = 1
				}
				target := apart
				if has {
					target = overlap
				}
				move := (length - target) / 2 / length
				dx[a] += ux * move
				dy[a] += uy * move
				dx[b] -= ux * move
				dy[b] -= uy * move
				pulls[a]++
				pulls[b]++
			}
		}
		for i := 0; i < n; i++ {
			if pulls[i] == 0 {
				continue
			}
			circles[i].X += dx[i] / float64(pulls[i])
			circles[i].Y += dy[i] / float64(pulls[i])
		}
	}
	return circles
}

func distinct(ids []string) int {
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	return len(seen)
}

// regionPath is the outline of the region all the given circles share, as an SVG path, or "" if
// they share none.
func regionPath(circles []*Circle, format func(float64) string) string {
	whole := func(c *Circle) string {
		return "M" + format(c.X-c.R) + "," + format(c.Y) + "A" + format(c.R) + "," + format(c.R) + " 0 1 1 " + format(c.X+c.R) + "," + format(c.Y) +
			"A" + format(c.R) + "," + format(c.R) + " 0 1 1 " + format(c.X-c.R) + "," + format(c.Y) + "Z"
	}
	if len(circles) == 1 {
		return whole(circles[0])
	}
	within := func(x, y float64, skip, other *Circle) bool {
		for _, c := range circles {
			if c != skip && c != other && js.Hypot(x-c.X, y-c.Y) > c.R*(1+1e-9) {
				return false
			}
		}
		return true
	}
	// The corners of the region: where two outlines cross inside all the other circles.
	type corner struct {
		x, y float64
		a, b *Circle
	}
	var corners []corner
	for i := 0; i < len(circles); i++ {
		for j := i + 1; j < len(circles); j++ {
			a := circles[i]
			b := circles[j]
			d := js.Hypot(b.X-a.X, b.Y-a.Y)
			if d >= a.R+b.R || d <= math.Abs(a.R-b.R) || d == 0 {
				continue
			}
			along := (a.R*a.R - b.R*b.R + d*d) / (2 * d)
			up := math.Sqrt(math.Max(a.R*a.R-along*along, 0))
			mx := a.X + ((b.X-a.X)*along)/d
			my := a.Y + ((b.Y-a.Y)*along)/d
			for _, sign := range [2]float64{1, -1} {
				x := mx + (sign*up*(b.Y-a.Y))/d
				y := my - (sign*up*(b.X-a.X))/d
				if within(x, y, a, b) {
					corners = append(corners, corner{x, y, a, b})
				}
			}
		}
	}
	if len(corners) < 2 {
		// No corners: either one circle lies inside all the others, or there is nothing shared.
		smallest := circles[0]
		for _, c := range circles {
			if c.R < smallest.R {
				smallest = c
			}
		}
		for _, c := range circles {
			if c != smallest && js.Hypot(c.X-smallest.X, c.Y-smallest.Y)+smallest.R > c.R*(1+1e-9) {
				return ""
			}
		}
		return whole(smallest)
	}
	cx := 0.0
	cy := 0.0
	count := float64(len(corners))
	for _, p := range corners {
		cx += p.x / count
		cy += p.y / count
	}
	sortStable(corners, func(p, q corner) float64 {
		return js.Atan2(p.y-cy, p.x-cx) - js.Atan2(q.y-cy, q.x-cx)
	})
	var d strings.Builder
	for i := range corners {
		from := corners[i]
		to := corners[(i+1)%len(corners)]
		if i == 0 {
			d.WriteString("M" + format(from.x) + "," + format(from.y))
		}
		// The side between two corners is an arc of a circle that passes through both; of those,
		// the one whose arc stays inside the others.
		var arc *Circle
		large := 0
		narrowest := math.Inf(1)
		for _, c := range [2]*Circle{from.a, from.b} {
			if c != to.a && c != to.b {
				continue
			}
			start := js.Atan2(from.y-c.Y, from.x-c.X)
			sweep := js.Atan2(to.y-c.Y, to.x-c.X) - start
			if sweep <= 0 {
				sweep += tau
			}
			mx := c.X + c.R*js.Cos(start+sweep/2)
			my := c.Y + c.R*js.Sin(start+sweep/2)
			width := js.Hypot(mx-(from.x+to.x)/2, my-(from.y+to.y)/2)
			if width < narrowest {
				narrowest = width
				arc = c
				large = 0
				if sweep > math.Pi {
					large = 1
				}
			}
		}
		if arc == nil {
			d.WriteString("L" + format(to.x) + "," + format(to.y))
		} else {
			d.WriteString("A" + format(arc.R) + "," + format(arc.R) + " 0 " + string(rune('0'+large)) + " 1 " + format(to.x) + "," + format(to.y))
		}
	}
	return d.String() + "Z"
}

func sortStable[T any](list []T, diff func(a, b T) float64) {
	slices.SortStableFunc(list, func(a, b T) int { return js.Cmp(diff(a, b)) })
}

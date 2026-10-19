package treemap

import (
	"math"
	"slices"

	"github.com/alessioalex/pele/go/internal/js"
)

type box struct {
	node   *node
	parent *box
	// Position among the top-level nodes of the branch this box belongs to, which picks its color.
	series int
	// A leaf's own value, or the sum of the leaves under a section.
	value      float64
	children   []*box
	x, y, w, h float64
}

type insets struct {
	gap float64
	// Space a section keeps for its header, and around its children on the other three sides.
	header, side float64
}

func newBox(n *node, parent *box, series int) *box {
	value := 0.0
	if n.children == nil && n.hasValue && js.IsFinite(n.value) && n.value > 0 {
		value = n.value
	}
	return &box{node: n, parent: parent, series: series, value: value}
}

// squarify lays out boxes in the rectangle as one squarified treemap: rows are added along the
// shorter side, and a row stops growing when another box would make its boxes less square.
func squarify(boxes []*box, total, x, y, w, h, gap float64) {
	rest := total
	from := 0
	for from < len(boxes) {
		scale := (w * h) / rest
		side := math.Min(w, h)
		sum := boxes[from].value * scale
		largest := sum
		worst := math.Max((side*side*largest)/(sum*sum), (sum*sum)/(side*side*largest))
		to := from + 1
		for to < len(boxes) {
			smallest := boxes[to].value * scale
			next := sum + smallest
			ratio := math.Max((side*side*largest)/(next*next), (next*next)/(side*side*smallest))
			if ratio > worst {
				break
			}
			worst = ratio
			sum = next
			to++
		}

		thickness := sum / side
		offset := 0.0
		for i := from; i < to; i++ {
			b := boxes[i]
			length := (b.value * scale) / thickness
			if i == to-1 {
				length = side - offset
			}
			bx, by := x+offset, y
			if w >= h {
				bx, by = x, y+offset
			}
			x0 := js.Round(bx + gap/2)
			y0 := js.Round(by + gap/2)
			b.x, b.y = x0, y0
			if w >= h {
				b.w = js.Round(bx+thickness-gap/2) - x0
				b.h = js.Round(by+length-gap/2) - y0
			} else {
				b.w = js.Round(bx+length-gap/2) - x0
				b.h = js.Round(by+thickness-gap/2) - y0
			}
			offset += length
			rest -= b.value
		}
		if w >= h {
			x += thickness
			w -= thickness
		} else {
			y += thickness
			h -= thickness
		}
		from = to
	}
}

// layoutTreemap returns every box with a size, parents before their children.
func layoutTreemap(roots []*node, width, height float64, in insets) []*box {
	var all []*box
	var top []*box
	for i, n := range roots {
		top = append(top, newBox(n, nil, i))
	}
	all = append(all, top...)
	for i := 0; i < len(all); i++ {
		parent := all[i]
		for _, n := range parent.node.children {
			child := newBox(n, parent, parent.series)
			parent.children = append(parent.children, child)
			all = append(all, child)
		}
	}
	for i := len(all) - 1; i >= 0; i-- {
		if b := all[i]; b.parent != nil {
			b.parent.value += b.value
		}
	}

	var placed []*box
	place := func(children []*box, x, y, w, h float64) {
		if w <= 0 || h <= 0 {
			return
		}
		var shown []*box
		for _, b := range children {
			if b.value > 0 {
				shown = append(shown, b)
			}
		}
		slices.SortStableFunc(shown, func(a, b *box) int { return js.Cmp(b.value - a.value) })
		total := 0.0
		for _, b := range shown {
			total += b.value
		}
		if len(shown) == 0 || !js.IsFinite(total) {
			return
		}
		squarify(shown, total, x-in.gap/2, y-in.gap/2, w+in.gap, h+in.gap, in.gap)
		for _, b := range shown {
			if b.w > 0 && b.h > 0 {
				placed = append(placed, b)
			}
		}
	}
	place(top, 0, 0, width, height)
	for i := 0; i < len(placed); i++ {
		b := placed[i]
		if len(b.children) > 0 {
			place(b.children, b.x+in.side, b.y+in.header, b.w-2*in.side, b.h-in.header-in.side)
		}
	}
	return placed
}

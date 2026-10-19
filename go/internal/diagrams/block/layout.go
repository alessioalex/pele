package block

import (
	"math"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

// cell is a block placed in the grid of its container.
type cell struct {
	children []*view
	// Grid width of this container: a positive count, or -1 to put every child on one row.
	columns float64
	span    float64
	space   bool
	// Height reserved above a container's grid for its title.
	head float64
	// Size the content needs. Set by the caller for leaves, computed here for containers.
	nw, nh float64
	// Placement: grid position, then center and size in pixels.
	col, row float64
	cols     float64
	heights  []float64
	x, y     float64
	w, h     float64
}

const maxSpan = 1000

func calculateBlockPosition(columns, position float64) (px, py float64) {
	if columns == 0 || !js.IsInteger(columns) {
		panic(core.NewError("Columns must be an integer !== 0.", core.CodeSemantic))
	}
	if position < 0 || !js.IsInteger(position) {
		panic(core.NewError("Position must be a non-negative integer."+js.NumberToString(position), core.CodeSemantic))
	}
	if columns < 0 {
		return position, 0
	}
	return math.Mod(position, columns), math.Floor(position / columns)
}

// measure puts each child in its grid cell and sizes the container. Every column is as wide as
// the widest block needs. Every row that holds a plain block is as tall as the tallest block in
// the container, as in Mermaid; a row of nothing but containers is only as tall as they need.
func measure(box *cell, gap, inset float64) {
	columns := box.columns
	var heights []float64
	position := 0.0
	cols := 0.0
	cw := 0.0
	tallest := 0.0
	for _, cv := range box.children {
		child := &cv.cell
		px, py := calculateBlockPosition(columns, position)
		// Mermaid lets a block wider than the rest of its row stick out of the grid. Here it stops at the last column.
		span := 1.0
		if child.span >= 1 {
			span = math.Min(child.span, maxSpan)
		}
		if columns > 0 && span > columns-px {
			span = columns - px
		}
		child.span = span
		child.col = px
		child.row = py
		position += span
		if px+span > cols {
			cols = px + span
		}
		row := int(py)
		if row == len(heights) {
			heights = append(heights, 0)
		}
		if len(child.children) == 0 {
			heights[row] = -1
		} else if heights[row] >= 0 && child.nh > heights[row] {
			heights[row] = child.nh
		}
		if child.space {
			continue
		}
		width := (child.nw - gap*(span-1)) / span
		if width > cw {
			cw = width
		}
		if child.nh > tallest {
			tallest = child.nh
		}
	}
	height := float64(len(heights)-1) * gap
	for row := range heights {
		if heights[row] < 0 {
			heights[row] = tallest
		}
		height += heights[row]
	}
	box.cols = cols
	box.heights = heights
	box.nw = math.Max(box.nw, cols*cw+(cols-1)*gap+2*inset)
	box.nh = height + 2*inset + box.head
}

// arrange gives each child its cell, sharing out any room the container has beyond what it needs.
func arrange(box *cell, gap, inset float64) {
	cols, heights := box.cols, box.heights
	rows := float64(len(heights))
	cw := (box.w - 2*inset - (cols-1)*gap) / cols
	room := box.h - 2*inset - box.head - (rows-1)*gap
	needed := 0.0
	for _, height := range heights {
		needed += height
	}
	scale, share := 0.0, 0.0
	if needed > 0 {
		scale = room / needed
	} else {
		share = room / rows
	}
	left := box.x - box.w/2 + inset
	top := box.y - box.h/2 + inset + box.head
	row := 0
	for _, cv := range box.children {
		child := &cv.cell
		for ; float64(row) < child.row; row++ {
			top += heights[row]*scale + share + gap
		}
		child.w = cw*child.span + gap*(child.span-1)
		child.h = heights[row]*scale + share
		child.x = left + child.col*(cw+gap) + child.w/2
		child.y = top + child.h/2
	}
}

// layout lays out a tree of cells in place. The root has no border, so its grid starts at the origin.
func layout(root *view, gap float64) (width, height float64) {
	order := []*view{root}
	for i := 0; i < len(order); i++ {
		order = append(order, order[i].children...)
	}
	for i := len(order) - 1; i >= 0; i-- {
		box := &order[i].cell
		if len(box.children) > 0 {
			inset := gap
			if i == 0 {
				inset = 0
			}
			measure(box, gap, inset)
		}
	}
	r := &root.cell
	r.w = r.nw
	r.h = r.nh
	r.x = r.w / 2
	r.y = r.h / 2
	for i := 0; i < len(order); i++ {
		box := &order[i].cell
		if len(box.children) > 0 {
			inset := gap
			if i == 0 {
				inset = 0
			}
			arrange(box, gap, inset)
		}
	}
	return r.w, r.h
}

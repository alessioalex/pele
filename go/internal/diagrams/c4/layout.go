package c4

import (
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/text"
)

const (
	shapeW    = 216
	shapeMinH = 64
	padX      = 14
	padY      = 12
	gapX      = 100
	gapY      = 88
	// Room for a relation to run along the inside of a boundary.
	groupPad = lane + 8
	groupGap = 48
	// Where a boundary's heading starts, from its top-left corner.
	headLeft = 16
	headTop  = 10
)

// Shape keywords Mermaid accepts in $shape, $sprite or $tags, as names of the shared shapes.
var forms = map[string]string{
	"person":    "person",
	"box":       "rect",
	"rounded":   "rect",
	"component": "rect",
	"cylinder":  "cyl",
	"database":  "cyl",
	"db":        "cyl",
	"queue":     "h-cyl",
	"pipe":      "h-cyl",
}

// Node is a laid out shape.
type Node struct {
	Box
	shape *Element
	// Type, name, technology, description.
	labels [4]text.Label
	textH  float64
}

// Group is a laid out boundary.
type Group struct {
	Box
	boundary *Element
	kids     []*Group
	nodes    []*Node
	// Name, type, description.
	labels []text.Label
	head   float64
	headW  float64
	headX  float64
}

// Layout is where everything goes.
type Layout struct {
	// The boundaries that are drawn, parents before children. The first is the unnamed top level.
	order []*Group
	boxes map[*Element]*Box
}

func formOf(shape *Element) string {
	if f, ok := forms[js.ToLower(shape.Shape)]; ok {
		return f
	}
	if f, ok := forms[js.ToLower(shape.Sprite)]; ok {
		return f
	}
	if shape.Tags != "" {
		for _, tag := range strings.Split(shape.Tags, ",") {
			if f, ok := forms[js.ToLower(js.Trim(tag))]; ok {
				return f
			}
		}
	}
	kind := shape.Kind
	switch {
	case strings.HasSuffix(kind, "person"):
		return "person"
	case strings.HasSuffix(kind, "_db"):
		return "cyl"
	case strings.HasSuffix(kind, "_queue"):
		return "h-cyl"
	}
	return "rect"
}

func measure(shape *Element, m text.Measurer, size, small, width float64) *Node {
	form := formOf(shape)
	capW := 0.0
	if form == "h-cyl" {
		capW = 36
	}
	capH := 0.0
	if form == "person" {
		capH = personHead
	} else if form == "cyl" {
		capH = 24
	}
	inner := width - 2*padX - capW
	techn := ""
	if shape.Techn != "" {
		techn = "[" + shape.Techn + "]"
	}
	labels := [4]text.Label{
		text.Layout("«"+strings.ReplaceAll(shape.Kind, "_", " ")+"»", false, m, small-1, inner, 0),
		text.Layout(shape.Label, false, m, size, inner, text.Bold),
		text.Layout(techn, false, m, small, inner, 0),
		text.Layout(shape.Descr, false, m, small, inner, 0),
	}
	tw := 0.0
	textH := 0.0
	if labels[3].Height > 0 {
		textH = 4
	}
	for _, label := range labels {
		tw = max(tw, label.Width)
		textH += label.Height
	}
	return &Node{
		Box:    Box{W: max(width, tw+2*padX+capW), H: max(shapeMinH, textH+2*padY+capH), Form: form},
		shape:  shape,
		labels: labels,
		textH:  textH,
	}
}

// Places boxes left to right in rows of `perRow`, from (0, 0), and returns the width and height the rows take.
// Shapes are stretched to the height of their row; boundaries keep their own. perRow is a float, as in
// the TypeScript, where it can be huge or NaN.
func rows(boxes []*Box, perRow, gx, gy float64, stretch bool) (float64, float64) {
	width := 0.0
	y := 0.0
	n := float64(len(boxes))
	for from := 0.0; from < n; from += perRow {
		to := min(n, from+perRow)
		x := 0.0
		rowH := 0.0
		for i := from; i < to; i++ {
			rowH = max(rowH, boxes[int(i)].H)
		}
		for i := from; i < to; i++ {
			box := boxes[int(i)]
			box.X = x
			box.Y = y
			if stretch {
				box.H = rowH
			}
			x += box.W + gx
		}
		width = max(width, x-gx)
		y += rowH + gy
	}
	if len(boxes) > 0 {
		return width, y - gy
	}
	return width, 0
}

// Mermaid's grid: in each boundary, its shapes in rows of c4ShapeInRow, then the boundaries inside it
// in rows of c4BoundaryInRow, each sized around its own content. hasRoom false is room undefined.
func layoutC4(db *Db, m text.Measurer, size, small float64, room float64, hasRoom bool, gap float64) Layout {
	pad := float64(groupPad)
	if hasRoom {
		pad = 12
	}
	groups := make(map[*Element]*Group, len(db.Boundaries))
	list := make([]*Group, 0, len(db.Boundaries))
	for _, boundary := range db.Boundaries {
		labelRoom := 4000.0
		if hasRoom {
			labelRoom = max(80, room-2*headLeft-2*groupPad*2)
		}
		var labels []text.Label
		if boundary.Parent != nil {
			typ := ""
			if boundary.Type != "" {
				typ = "[" + boundary.Type + "]"
			}
			labels = []text.Label{
				text.Layout(boundary.Label, false, m, size, labelRoom, text.Bold),
				text.Layout(typ, false, m, small, labelRoom, 0),
				text.Layout(boundary.Descr, false, m, small, labelRoom, 0),
			}
		}
		h := 0.0
		headW := 0.0
		for _, label := range labels {
			h += label.Height
			headW = max(headW, label.Width)
		}
		g := &Group{boundary: boundary, labels: labels, headW: headW, headX: headLeft, Box: Box{Form: "rect"}}
		if h > 0 {
			g.head = h + headTop + 6 + lane
		} else {
			g.head = pad
		}
		groups[boundary] = g
		list = append(list, g)
	}
	root := groups[db.Boundaries[0]]
	for _, g := range list {
		if parent := g.boundary.Parent; parent != nil {
			if pg := groups[parent]; pg != nil {
				pg.kids = append(pg.kids, g)
			}
		}
	}
	// A boundary that ends up inside itself is not reachable from the top, as in Mermaid.
	order := []*Group{root}
	depths := map[*Element]float64{root.boundary: 0}
	for i := 0; i < len(order); i++ {
		for _, kid := range order[i].kids {
			depths[kid.boundary] = depths[order[i].boundary] + 1
			order = append(order, kid)
		}
	}
	for _, shape := range db.Shapes {
		if shape.Parent != nil {
			depth := depths[shape.Parent]
			nodeWidth := float64(shapeW)
			if hasRoom {
				nodeWidth = max(100, min(shapeW, room-2*depth*pad-32))
			}
			if g := groups[shape.Parent]; g != nil {
				g.nodes = append(g.nodes, measure(shape, m, size, small, nodeWidth))
			}
		}
	}

	perRow := max(1, db.ShapeInRow)
	groupsPerRow := max(1, db.BoundaryInRow)
	kidsTop := make(map[*Group]float64, len(order))
	for i := len(order) - 1; i >= 0; i-- {
		group := order[i]
		gx := float64(gapX)
		if hasRoom {
			gx = gap
		}
		count := func(boxes []*Box, limit, gap float64) float64 {
			if !hasRoom {
				return limit
			}
			widest := 1.0
			for _, box := range boxes {
				widest = max(widest, box.W)
			}
			return max(1, min(limit, math.Floor((room+gap-2*pad)/(widest+gap))))
		}
		nodes := make([]*Box, len(group.nodes))
		for k, node := range group.nodes {
			nodes[k] = &node.Box
		}
		kids := make([]*Box, len(group.kids))
		for k, kid := range group.kids {
			kids[k] = &kid.Box
		}
		gridW, gridH := rows(nodes, count(nodes, perRow, gx), gx, gapY, true)
		kidsW, kidsH := rows(kids, count(kids, groupsPerRow, groupGap), groupGap, groupGap, false)
		between := 0.0
		if gridH > 0 && kidsH > 0 {
			between = groupGap
		}
		frame := pad
		if group == root {
			frame = 0
		}
		kidsTop[group] = gridH + between
		group.W = max(gridW, kidsW, group.headW+2*headLeft-2*frame) + 2*frame
		top := frame
		if frame != 0 {
			top = group.head
		}
		group.H = top + gridH + between + kidsH + frame
	}

	boxes := make(map[*Element]*Box, len(db.Shapes)+len(order))
	for _, group := range order {
		left := group.X
		top := group.Y
		if group != root {
			left += pad
			top += group.head
		}
		for _, node := range group.nodes {
			node.X += left
			node.Y += top
			boxes[node.shape] = &node.Box
		}
		for _, kid := range group.kids {
			kid.X += left
			kid.Y += top + kidsTop[group]
		}
		if group != root {
			boxes[group.boundary] = &group.Box
		}
	}
	return Layout{order: order, boxes: boxes}
}

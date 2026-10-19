package block

import (
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
)

const (
	dirRight = 1
	dirLeft  = 2
	dirUp    = 4
	dirDown  = 8
)

// overhang is how far a head reaches past each side of its shaft.
const (
	overhang = 7
	maxDepth = 20
	nub      = 12
)

type arrowSizing struct {
	w, h float64
	// Shift of the label from the center, away from the heads.
	dx, dy float64
}

// Mermaid compares directions exactly, so `( up )` draws no arrow there. Here spaces are ignored.
func arrowMask(directions []string) int {
	mask := 0
	for _, direction := range directions {
		switch js.Trim(direction) {
		case "right":
			mask |= dirRight
		case "left":
			mask |= dirLeft
		case "up":
			mask |= dirUp
		case "down":
			mask |= dirDown
		case "x":
			mask |= dirRight | dirLeft
		case "y":
			mask |= dirUp | dirDown
		}
	}
	return mask
}

func bit(mask, flag int) float64 {
	if mask&flag != 0 {
		return 1
	}
	return 0
}

// arrowAxis: an arrow along one axis has a shaft and heads wider than it. One that points along
// both axes is a box with a small point on each side it names.
func arrowAxis(mask int) byte {
	if mask != 0 && mask&(dirUp|dirDown) == 0 {
		return 'x'
	}
	if mask != 0 && mask&(dirRight|dirLeft) == 0 {
		return 'y'
	}
	return 0
}

func arrowSize(mask int, tw, th float64) arrowSizing {
	axis := arrowAxis(mask)
	across := bit(mask, dirLeft) - bit(mask, dirRight)
	along := bit(mask, dirUp) - bit(mask, dirDown)
	if axis == 'x' {
		h := math.Max(th+8, 14) + 2*overhang
		depth := math.Min(h/2, maxDepth)
		k := 1.0
		if mask == dirRight|dirLeft {
			k = 2
		}
		return arrowSizing{w: math.Max(tw+12, 16) + depth*k, h: h, dx: (across * depth) / 2}
	}
	if axis == 'y' {
		w := math.Max(tw+16, 14) + 2*overhang
		depth := math.Min(w/2, maxDepth)
		k := 1.0
		if mask == dirUp|dirDown {
			k = 2
		}
		return arrowSizing{w: w, h: math.Max(th+8, 16) + depth*k, dy: (along * depth) / 2}
	}
	return arrowSizing{
		w:  math.Max(tw+16, 24) + nub*(bit(mask, dirLeft)+bit(mask, dirRight)),
		h:  math.Max(th+8, 24) + nub*(bit(mask, dirUp)+bit(mask, dirDown)),
		dx: (across * nub) / 2,
		dy: (along * nub) / 2,
	}
}

func drawArrow(mask int, w, h float64, attrs string) string {
	axis := arrowAxis(mask)
	x := w / 2
	y := h / 2
	var p []float64
	if axis == 'x' {
		depth := math.Min(y, maxDepth)
		s := y - overhang
		x0 := -x
		if mask&dirLeft != 0 {
			x0 = -x + depth
		}
		x1 := x
		if mask&dirRight != 0 {
			x1 = x - depth
		}
		p = []float64{x0, -s, x1, -s}
		if mask&dirRight != 0 {
			p = append(p, x1, -y, x, 0, x1, y)
		}
		p = append(p, x1, s, x0, s)
		if mask&dirLeft != 0 {
			p = append(p, x0, y, -x, 0, x0, -y)
		}
	} else if axis == 'y' {
		depth := math.Min(x, maxDepth)
		s := x - overhang
		y0 := -y
		if mask&dirUp != 0 {
			y0 = -y + depth
		}
		y1 := y
		if mask&dirDown != 0 {
			y1 = y - depth
		}
		p = []float64{s, y0, s, y1}
		if mask&dirDown != 0 {
			p = append(p, x, y1, 0, y, -x, y1)
		}
		p = append(p, -s, y1, -s, y0)
		if mask&dirUp != 0 {
			p = append(p, -x, y0, 0, -y, x, y0)
		}
	} else {
		x0 := -x
		if mask&dirLeft != 0 {
			x0 = -x + nub
		}
		x1 := x
		if mask&dirRight != 0 {
			x1 = x - nub
		}
		y0 := -y
		if mask&dirUp != 0 {
			y0 = -y + nub
		}
		y1 := y
		if mask&dirDown != 0 {
			y1 = y - nub
		}
		cx := (x0 + x1) / 2
		cy := (y0 + y1) / 2
		kx := math.Min(10, (x1-x0)/2)
		ky := math.Min(10, (y1-y0)/2)
		p = []float64{x0, y0}
		if mask&dirUp != 0 {
			p = append(p, cx-kx, y0, cx, -y, cx+kx, y0)
		}
		p = append(p, x1, y0)
		if mask&dirRight != 0 {
			p = append(p, x1, cy-ky, x, cy, x1, cy+ky)
		}
		p = append(p, x1, y1)
		if mask&dirDown != 0 {
			p = append(p, cx+kx, y1, cx, y, cx-kx, y1)
		}
		p = append(p, x0, y1)
		if mask&dirLeft != 0 {
			p = append(p, x0, cy+ky, -x, cy, x0, cy-ky)
		}
	}
	var points strings.Builder
	for i := 0; i < len(p); i += 2 {
		if i > 0 {
			points.WriteByte(' ')
		}
		points.WriteString(svg.Num(p[i]))
		points.WriteByte(',')
		points.WriteString(svg.Num(p[i+1]))
	}
	return `<polygon points="` + points.String() + `" stroke-linejoin="round"` + attrs + `/>`
}

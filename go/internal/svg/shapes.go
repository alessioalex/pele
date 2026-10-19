package svg

import (
	"math"
	"strconv"
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
)

// Products that feed an addition or subtraction are wrapped in float64() so that no
// architecture fuses them into an FMA, which V8 does not do.

// ShapeDims is the size of a node shape drawn around a label (ShapeSize in TS).
type ShapeDims struct {
	W float64
	H float64
	// Dy is the vertical shift of the label from the shape's center.
	Dy float64
}

// Box is what these helpers read of a laid-out node (CNode in src/layout/compound.ts): its
// center, its size and whether it is a group.
type Box struct {
	X       float64
	Y       float64
	W       float64
	H       float64
	IsGroup bool
}

const (
	PadX = 18
	PadY = 12
)

type (
	sizer  func(tw, th float64) ShapeDims
	drawer func(w, h float64, a, line string) string
	// Distance from the bounding box to the outline on a side: 0 top, 1 right, 2 bottom, 3 left.
	// `t` is how far along the side from its middle, to the right or down.
	insetFn func(w, h float64, side int, t float64) float64
	// How much of a side edges may spread along, centered on its middle: the top and bottom, or
	// the left and right when `sideways`. A shape without one keeps its edges at the middle.
	spanFn func(w, h float64, sideways bool) float64
)

type shapeDef struct {
	size    sizer
	draw    drawer
	inset   insetFn
	span    spanFn
	noLabel bool
}

// Kept clear at each end of a straight side, for the corner.
const shapeCorner = 12

func pick(c bool, a, b float64) float64 {
	if c {
		return a
	}
	return b
}

func flat(w, h float64, sideways bool) float64 { return pick(sideways, h, w) - 2*shapeCorner }

// Straight along the top and bottom except for `cut(h)` at each end, and not straight at the sides.
func flatAcross(cut func(h float64) float64) spanFn {
	return func(w, h float64, sideways bool) float64 {
		if sideways {
			return 0
		}
		return w - 2*cut(h) - 2*shapeCorner
	}
}

// How far an ellipse's outline has fallen away from its bounding box, `t` from the middle of a side `half` long.
func bulge(t, half, depth float64) float64 {
	k := min(1, math.Abs(t)/half)
	return float64(depth * (1 - math.Sqrt(1-float64(k*k))))
}

func roundInset(w, h float64, side int, t float64) float64 {
	if side&1 != 0 {
		return bulge(t, h/2, w/2)
	}
	return bulge(t, w/2, h/2)
}

func roundSpan(w, h float64, sideways bool) float64 { return float64(pick(sideways, h, w) * 0.6) }

func capRy(w float64) float64 { return min(12, max(6, w/14)) }

func capInset(w, _ float64, side int, t float64) float64 {
	if side&1 != 0 {
		return 0
	}
	return bulge(t, w/2, capRy(w))
}

func fixed(w, h float64) sizer { return func(_, _ float64) ShapeDims { return ShapeDims{w, h, 0} } }

func triSize(tw, th float64, flip bool) ShapeDims {
	w := max(float64(1.6*tw)+40, 80)
	h := (th + 10) / (1 - (tw+8)/w)
	dy := h/2 - 10 - th/2
	if flip {
		dy = -dy
	}
	return ShapeDims{w, h, dy}
}

func shapePath(d, a string) string { return `<path d="` + d + `"` + a + "/>" }

func leanInset(_, h float64, side int, _ float64) float64 {
	if side&1 != 0 {
		return h / 8
	}
	return 0
}

func boxSize(tw, th float64) ShapeDims {
	return ShapeDims{max(tw+2*PadX, 2*PadX+12), th + 2*PadY, 0}
}

func poly(points []float64, a string) string {
	var b strings.Builder
	b.WriteString(`<polygon points="`)
	for i := 0; i < len(points); i += 2 {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(Num(points[i]))
		b.WriteByte(',')
		b.WriteString(Num(points[i+1]))
	}
	b.WriteByte('"')
	b.WriteString(a)
	b.WriteString("/>")
	return b.String()
}

func shapeRect(w, h float64, a, rx string) string {
	return `<rect x="` + Num(-w/2) + `" y="` + Num(-h/2) + `" width="` + Num(w) + `" height="` + Num(h) + `"` + rx + a + "/>"
}

const roundedRx = ` rx="` + Radius + `"`

// sideInset is an inset of `v` on the sides where `on` holds, and 0 elsewhere.
func sideInset(on func(side int) bool, v func(w, h float64) float64) insetFn {
	return func(w, h float64, side int, _ float64) float64 {
		if on(side) {
			return v(w, h)
		}
		return 0
	}
}

func odd(side int) bool { return side&1 != 0 }

func is(n int) func(side int) bool { return func(side int) bool { return side == n } }

func constant(v float64) func(w, h float64) float64 { return func(_, _ float64) float64 { return v } }

// padded is a shape that grows its label by the padding plus dw across and dh down.
func padded(dw, dh, dy float64) sizer {
	return func(tw, th float64) ShapeDims { return ShapeDims{tw + 2*PadX + dw, th + 2*PadY + dh, dy} }
}

// slanted is a shape as tall as its padded label and wider by a part of that height.
func slanted(k float64) sizer {
	return func(tw, th float64) ShapeDims {
		h := th + 2*PadY
		return ShapeDims{tw + 2*PadX + float64(h*k), h, 0}
	}
}

func circle(r float64, a string) string { return `<circle r="` + Num(r) + `"` + a + "/>" }

func fillNone(d, line string) string { return `<path d="` + d + `" fill="none"` + line + "/>" }

var shapes = map[string]*shapeDef{
	"rect": {size: boxSize, span: flat, draw: func(w, h float64, a, _ string) string { return shapeRect(w, h, a, roundedRx) }},
	"rounded": {
		size: boxSize,
		span: flat,
		draw: func(w, h float64, a, _ string) string { return shapeRect(w, h, a, ` rx="`+Num(min(12, h/2))+`"`) },
	},
	"stadium": {
		inset: func(_, h float64, side int, t float64) float64 {
			if side&1 != 0 {
				return bulge(t, h/2, h/2)
			}
			return 0
		},
		span: func(w, h float64, sideways bool) float64 {
			if sideways {
				return float64(h * 0.6)
			}
			return w - h
		},
		size: func(tw, th float64) ShapeDims {
			h := th + 2*PadY
			return ShapeDims{tw + h*0.5 + 2*PadX - 8, h, 0}
		},
		draw: func(w, h float64, a, _ string) string { return shapeRect(w, h, a, ` rx="`+Num(h/2)+`"`) },
	},
	"fr-rect": {
		span: flat,
		size: padded(16, 0, 0),
		draw: func(w, h float64, a, line string) string {
			x := w/2 - 8
			return shapeRect(w, h, a, roundedRx) +
				fillNone("M"+Num(-x)+","+Num(-h/2)+"V"+Num(h/2)+"M"+Num(x)+","+Num(-h/2)+"V"+Num(h/2), line)
		},
	},
	"cyl": {
		inset: capInset,
		span: func(w, h float64, sideways bool) float64 {
			if sideways {
				return h - 2*shapeCorner - 12
			}
			return float64(w * 0.6)
		},
		size: func(tw, th float64) ShapeDims {
			w := max(tw+2*PadX, 56)
			ry := capRy(w)
			return ShapeDims{w, th + 2*PadY + 2*ry, ry / 2}
		},
		draw: func(w, h float64, a, line string) string {
			rx := w / 2
			ry := capRy(w)
			top := -h/2 + ry
			bottom := h/2 - ry
			return shapePath("M"+Num(-rx)+","+Num(top)+"A"+Num(rx)+","+Num(ry)+" 0 0 1 "+Num(rx)+","+Num(top)+"V"+Num(bottom)+
				"A"+Num(rx)+","+Num(ry)+" 0 0 1 "+Num(-rx)+","+Num(bottom)+"Z", a) +
				fillNone("M"+Num(-rx)+","+Num(top)+"A"+Num(rx)+","+Num(ry)+" 0 0 0 "+Num(rx)+","+Num(top), line)
		},
	},
	"circle": {
		inset: roundInset,
		span:  roundSpan,
		size: func(tw, th float64) ShapeDims {
			d := max(math.Sqrt(float64(tw*tw)+float64(th*th))+16, 44)
			return ShapeDims{d, d, 0}
		},
		draw: func(w, _ float64, a, _ string) string { return circle(w/2, a) },
	},
	"dbl-circ": {
		inset: roundInset,
		span:  roundSpan,
		size: func(tw, th float64) ShapeDims {
			d := max(math.Sqrt(float64(tw*tw)+float64(th*th))+26, 54)
			return ShapeDims{d, d, 0}
		},
		draw: func(w, _ float64, a, line string) string {
			return circle(w/2, a) + circle(w/2-5, ` fill="none"`+line)
		},
	},
	"ellipse": {
		inset: roundInset,
		span:  roundSpan,
		size: func(tw, th float64) ShapeDims {
			return ShapeDims{float64(tw*1.3) + 2*PadX, float64(th*1.2) + 2*PadY, 0}
		},
		draw: func(w, h float64, a, _ string) string {
			return `<ellipse rx="` + Num(w/2) + `" ry="` + Num(h/2) + `"` + a + "/>"
		},
	},
	"diam": {
		inset: func(w, h float64, side int, t float64) float64 {
			if side&1 != 0 {
				return (math.Abs(t) * w) / h
			}
			return (math.Abs(t) * h) / w
		},
		span: roundSpan,
		size: func(tw, th float64) ShapeDims {
			a := tw + 16
			b := th + 8
			return ShapeDims{a + float64(1.6*b), a/1.6 + b, 0}
		},
		draw: func(w, h float64, a, _ string) string {
			return poly([]float64{0, -h / 2, w / 2, 0, 0, h / 2, -w / 2, 0}, a)
		},
	},
	"hex": {
		inset: func(_, _ float64, side int, t float64) float64 {
			if side&1 != 0 {
				return math.Abs(t) / 2
			}
			return 0
		},
		span: func(w, h float64, sideways bool) float64 {
			if sideways {
				return h * 0.5
			}
			return w - h/2 - shapeCorner
		},
		size: slanted(0.5),
		draw: func(w, h float64, a, _ string) string {
			m := h / 4
			return poly([]float64{-w/2 + m, -h / 2, w/2 - m, -h / 2, w / 2, 0, w/2 - m, h / 2, -w/2 + m, h / 2, -w / 2, 0}, a)
		},
	},
	"odd": {
		inset: sideInset(is(3), func(_, h float64) float64 { return h / 4 }),
		span:  flatAcross(func(float64) float64 { return 0 }),
		size:  slanted(0.25),
		draw: func(w, h float64, a, _ string) string {
			notch := h / 4
			return poly([]float64{-w / 2, -h / 2, w / 2, -h / 2, w / 2, h / 2, -w / 2, h / 2, -w/2 + notch, 0}, a)
		},
	},
	"lean-r": {
		inset: leanInset,
		span:  flatAcross(quarter),
		size:  slanted(0.5),
		draw: func(w, h float64, a, _ string) string {
			s := h / 4
			return poly([]float64{-w/2 + s, -h / 2, w / 2, -h / 2, w/2 - s, h / 2, -w / 2, h / 2}, a)
		},
	},
	"lean-l": {
		inset: leanInset,
		span:  flatAcross(quarter),
		size:  slanted(0.5),
		draw: func(w, h float64, a, _ string) string {
			s := h / 4
			return poly([]float64{-w / 2, -h / 2, w/2 - s, -h / 2, w / 2, h / 2, -w/2 + s, h / 2}, a)
		},
	},
	"trap-b": {
		inset: leanInset,
		span:  flatAcross(quarter),
		size:  slanted(0.5),
		draw: func(w, h float64, a, _ string) string {
			s := h / 4
			return poly([]float64{-w/2 + s, -h / 2, w/2 - s, -h / 2, w / 2, h / 2, -w / 2, h / 2}, a)
		},
	},
	"trap-t": {
		inset: leanInset,
		span:  flatAcross(quarter),
		size:  slanted(0.5),
		draw: func(w, h float64, a, _ string) string {
			s := h / 4
			return poly([]float64{-w / 2, -h / 2, w / 2, -h / 2, w/2 - s, h / 2, -w/2 + s, h / 2}, a)
		},
	},
	"text": {
		span: flat,
		size: func(tw, th float64) ShapeDims { return ShapeDims{tw + 8, th + 8, 0} },
		draw: func(w, h float64, _, _ string) string { return shapeRect(w, h, ` fill="none" stroke="none"`, "") },
	},
	"sm-circ": {
		noLabel: true,
		inset:   roundInset,
		span:    roundSpan,
		size:    fixed(14, 14),
		draw:    func(w, _ float64, a, _ string) string { return circle(w/2, a) },
	},
	"f-circ": {
		noLabel: true,
		inset:   roundInset,
		span:    roundSpan,
		size:    fixed(14, 14),
		draw:    func(w, _ float64, _, line string) string { return circle(w/2, ` fill="var(--_l)"`+line) },
	},
	"fr-circ": {
		noLabel: true,
		inset:   roundInset,
		span:    roundSpan,
		size:    fixed(18, 18),
		draw: func(w, _ float64, a, line string) string {
			return circle(w/2, a) + circle(w/2-4, ` fill="var(--_l)"`+line)
		},
	},
	"fork": {
		noLabel: true,
		span:    func(w, h float64, sideways bool) float64 { return pick(sideways, h, w) - 8 },
		size:    fixed(72, 8),
		draw:    func(w, h float64, _, line string) string { return shapeRect(w, h, ` fill="var(--_l)"`+line, ` rx="2"`) },
	},
	"notch-rect": {
		size: boxSize,
		span: flat,
		draw: func(w, h float64, a, _ string) string {
			const c = 10
			return poly([]float64{-w/2 + c, -h / 2, w / 2, -h / 2, w / 2, h / 2, -w / 2, h / 2, -w / 2, -h/2 + c}, a)
		},
	},
	"lin-rect": {
		span: flat,
		size: padded(8, 0, 0),
		draw: func(w, h float64, a, line string) string {
			return shapeRect(w, h, a, roundedRx) + fillNone("M"+Num(-w/2+8)+","+Num(-h/2)+"V"+Num(h/2), line)
		},
	},
	"div-rect": {
		span: flat,
		size: padded(0, 8, 4),
		draw: func(w, h float64, a, line string) string {
			return shapeRect(w, h, a, roundedRx) + fillNone("M"+Num(-w/2)+","+Num(-h/2+8)+"H"+Num(w/2), line)
		},
	},
	"win-pane": {
		span: flat,
		size: padded(8, 8, 4),
		draw: func(w, h float64, a, line string) string {
			return shapeRect(w, h, a, roundedRx) +
				fillNone("M"+Num(-w/2+8)+","+Num(-h/2)+"V"+Num(h/2)+"M"+Num(-w/2)+","+Num(-h/2+8)+"H"+Num(w/2), line)
		},
	},
	"tri": {
		inset: sideInset(odd, func(w, _ float64) float64 { return w / 4 }),
		size:  func(tw, th float64) ShapeDims { return triSize(tw, th, false) },
		draw: func(w, h float64, a, _ string) string {
			return poly([]float64{0, -h / 2, w / 2, h / 2, -w / 2, h / 2}, a)
		},
	},
	"flip-tri": {
		inset: sideInset(odd, func(w, _ float64) float64 { return w / 4 }),
		size:  func(tw, th float64) ShapeDims { return triSize(tw, th, true) },
		draw: func(w, h float64, a, _ string) string {
			return poly([]float64{-w / 2, -h / 2, w / 2, -h / 2, 0, h / 2}, a)
		},
	},
	"sl-rect": {
		inset: sideInset(is(0), constant(5)),
		size:  padded(0, 10, 5),
		draw: func(w, h float64, a, _ string) string {
			return poly([]float64{-w / 2, -h/2 + 10, w / 2, -h / 2, w / 2, h / 2, -w / 2, h / 2}, a)
		},
	},
	"notch-pent": {
		span: flat,
		size: padded(0, 0, 0),
		draw: func(w, h float64, a, _ string) string {
			const c = 10
			return poly([]float64{-w/2 + c, -h / 2, w/2 - c, -h / 2, w / 2, -h/2 + c, w / 2, h / 2, -w / 2, h / 2, -w / 2, -h/2 + c}, a)
		},
	},
	"hourglass": {
		noLabel: true,
		inset:   sideInset(odd, func(w, _ float64) float64 { return w / 2 }),
		size:    fixed(32, 40),
		draw: func(w, h float64, a, _ string) string {
			return poly([]float64{-w / 2, -h / 2, w / 2, -h / 2, -w / 2, h / 2, w / 2, h / 2}, a)
		},
	},
	"delay": {
		size: slanted(0.25),
		draw: func(w, h float64, a, _ string) string {
			r := h / 2
			return shapePath("M"+Num(-w/2)+","+Num(-r)+"H"+Num(w/2-r)+"A"+Num(r)+","+Num(r)+" 0 0 1 "+Num(w/2-r)+","+Num(r)+"H"+Num(-w/2)+"Z", a)
		},
	},
	"h-cyl": {
		size: func(tw, th float64) ShapeDims {
			h := th + 2*PadY
			rx := min(12, h/4)
			return ShapeDims{tw + 2*PadX + 2*rx, h, 0}
		},
		draw: func(w, h float64, a, line string) string {
			ry := h / 2
			rx := min(12, h/4)
			l := -w/2 + rx
			r := w/2 - rx
			return shapePath("M"+Num(l)+","+Num(-ry)+"H"+Num(r)+"A"+Num(rx)+","+Num(ry)+" 0 0 1 "+Num(r)+","+Num(ry)+"H"+Num(l)+
				"A"+Num(rx)+","+Num(ry)+" 0 0 1 "+Num(l)+","+Num(-ry)+"Z", a) +
				fillNone("M"+Num(r)+","+Num(-ry)+"A"+Num(rx)+","+Num(ry)+" 0 0 0 "+Num(r)+","+Num(ry), line)
		},
	},
	"cross-circ": {
		noLabel: true,
		inset:   roundInset,
		span:    roundSpan,
		size:    fixed(44, 44),
		draw: func(w, _ float64, a, line string) string {
			r := w / 2
			k := r * (math.Sqrt2 / 2)
			return circle(r, a) + fillNone("M"+Num(-k)+","+Num(-k)+"L"+Num(k)+","+Num(k)+"M"+Num(k)+","+Num(-k)+"L"+Num(-k)+","+Num(k), line)
		},
	},
	"st-rect": {
		span: func(w, h float64, sideways bool) float64 { return pick(sideways, h, w) - 2*shapeCorner - 8 },
		size: padded(8, 8, 4),
		draw: func(w, h float64, a, _ string) string {
			bw := w - 8
			bh := h - 8
			var out strings.Builder
			for i := 0; i < 3; i++ {
				x := -w/2 + float64((2-i)*4)
				y := -h/2 + float64(i*4)
				out.WriteString(`<rect x="` + Num(x) + `" y="` + Num(y) + `" width="` + Num(bw) + `" height="` + Num(bh) + `"` + roundedRx + a + "/>")
			}
			return out.String()
		},
	},
	"doc": {
		inset: sideInset(is(2), constant(10)),
		size:  padded(0, 10, -5),
		draw:  func(w, h float64, a, _ string) string { return wave(w, h, a) },
	},
	"flag": {
		inset: sideInset(func(side int) bool { return side&1 == 0 }, constant(8)),
		size:  padded(0, 16, 0),
		draw: func(w, h float64, a, _ string) string {
			x := w / 2
			y := h/2 - 8
			q := w / 4
			return shapePath("M"+Num(-x)+","+Num(-y)+"q"+Num(q)+",-10 "+Num(2*q)+",0t"+Num(2*q)+",0V"+Num(y)+"q"+Num(-q)+",10 "+Num(-2*q)+",0t"+Num(-2*q)+",0Z", a)
		},
	},
	"datastore": {
		size: boxSize,
		span: flat,
		draw: func(w, h float64, a, line string) string {
			return shapeRect(w, h, strings.Replace(a, `stroke="var(--_b)"`, `stroke="none"`, 1), "") +
				fillNone("M"+Num(-w/2)+","+Num(-h/2)+"H"+Num(w/2)+"M"+Num(-w/2)+","+Num(h/2)+"H"+Num(w/2), line)
		},
	},
	"folder": {
		size: func(tw, th float64) ShapeDims { return ShapeDims{max(tw+2*PadX, 64), th + 2*PadY + 8, 4} },
		draw: func(w, h float64, a, _ string) string {
			x := w / 2
			y := h / 2
			tab := min(w*0.4, 48)
			return shapePath("M"+Num(-x)+","+Num(-y)+"H"+Num(-x+tab)+"L"+Num(-x+tab+8)+","+Num(-y+8)+"H"+Num(x)+"V"+Num(y)+"H"+Num(-x)+"Z", a)
		},
	},
	"bucket": {
		size:  padded(20, 10, 3),
		inset: sideInset(odd, func(_, h float64) float64 { return h / 12 }),
		draw: func(w, h float64, a, line string) string {
			x := w / 2
			y := h / 2
			const ry = 5
			taper := h / 6
			return shapePath("M"+Num(-x)+","+Num(-y+ry)+"A"+Num(x)+","+Num(ry)+" 0 0 1 "+Num(x)+","+Num(-y+ry)+"L"+Num(x-taper)+","+Num(y-ry)+
				"A"+Num(x-taper)+","+Num(ry)+" 0 0 1 "+Num(-x+taper)+","+Num(y-ry)+"Z", a) +
				fillNone("M"+Num(-x)+","+Num(-y+ry)+"A"+Num(x)+","+Num(ry)+" 0 0 0 "+Num(x)+","+Num(-y+ry), line)
		},
	},
	"console": {
		span: flat,
		size: func(tw, th float64) ShapeDims { return ShapeDims{max(tw+2*PadX, 72), th + 2*PadY + 12, 6} },
		draw: func(w, h float64, a, line string) string {
			return shapeRect(w, h, a, roundedRx) +
				fillNone("M"+Num(-w/2)+","+Num(-h/2+12)+"H"+Num(w/2)+"M"+Num(-w/2+7)+","+Num(-h/2+4)+"l3,2.5l-3,2.5M"+Num(-w/2+13)+","+Num(-h/2+9)+"h4", line)
		},
	},
	"browser": {
		span: flat,
		size: func(tw, th float64) ShapeDims { return ShapeDims{max(tw+2*PadX, 72), th + 2*PadY + 12, 6} },
		draw: func(w, h float64, a, line string) string {
			dots := ""
			for i := 0; i < 3; i++ {
				dots += `<circle cx="` + Num(-w/2+8+float64(i*7)) + `" cy="` + Num(-h/2+6) + `" r="1.5" fill="none"` + line + "/>"
			}
			return shapeRect(w, h, a, roundedRx) + fillNone("M"+Num(-w/2)+","+Num(-h/2+12)+"H"+Num(w/2), line) + dots
		},
	},
	"person": {
		size: func(tw, th float64) ShapeDims { return ShapeDims{max(tw+2*PadX, 56), th + 2*PadY + 26, 13} },
		draw: func(w, h float64, a, _ string) string {
			top := -h / 2
			return `<rect x="` + Num(-w/2) + `" y="` + Num(top+26) + `" width="` + Num(w) + `" height="` + Num(h-26) + `" rx="12"` + a + "/>" +
				`<circle cy="` + Num(top+14) + `" r="13"` + a + "/>"
		},
	},
	"bang": {
		size:  padded(28, 24, 0),
		inset: func(_, _ float64, _ int, _ float64) float64 { return 3 },
		draw: func(w, h float64, a, _ string) string {
			const spikes = 14
			pts := make([]float64, 0, spikes*4)
			for i := 0; i < spikes*2; i++ {
				angle := (math.Pi*float64(i))/spikes - math.Pi/2
				k := 0.82
				if i%2 == 0 {
					k = 1
				}
				pts = append(pts, js.Cos(angle)*(w/2)*k, js.Sin(angle)*(h/2)*k)
			}
			return poly(pts, a)
		},
	},
	"cloud": {
		size: padded(32, 24, 0),
		inset: func(_, _ float64, side int, _ float64) float64 {
			if side&1 != 0 {
				return 2
			}
			return 1
		},
		draw: func(w, h float64, a, _ string) string {
			x := w / 2
			y := h / 2
			r := float64(y * 0.55)
			return shapePath("M"+Num(-x+r)+","+Num(y)+"A"+Num(r)+","+Num(r)+" 0 0 1 "+Num(-x+float64(r*0.6))+","+Num(-y*0.1)+
				"A"+Num(x*0.42)+","+Num(y*0.7)+" 0 0 1 "+Num(-x*0.1)+","+Num(-y*0.72)+
				"A"+Num(x*0.4)+","+Num(y*0.62)+" 0 0 1 "+Num(x*0.62)+","+Num(-y*0.42)+
				"A"+Num(r*1.05)+","+Num(r*1.05)+" 0 0 1 "+Num(x-r)+","+Num(y)+"Z", a)
		},
	},
	"brace": {
		size: padded(12, 0, 0),
		draw: func(w, h float64, _, line string) string {
			return shapeRect(w, h, ` fill="none" stroke="none"`, "") + brace(-w/2+12, h, -1, line)
		},
	},
	"brace-r": {
		size: padded(12, 0, 0),
		draw: func(w, h float64, _, line string) string {
			return shapeRect(w, h, ` fill="none" stroke="none"`, "") + brace(w/2-12, h, 1, line)
		},
	},
	"braces": {
		size: padded(24, 0, 0),
		draw: func(w, h float64, _, line string) string {
			return shapeRect(w, h, ` fill="none" stroke="none"`, "") + brace(-w/2+12, h, -1, line) + brace(w/2-12, h, 1, line)
		},
	},
	"bolt": {
		noLabel: true,
		size:    fixed(36, 56),
		inset:   sideInset(odd, func(w, _ float64) float64 { return w / 4 }),
		draw: func(w, h float64, a, _ string) string {
			return poly([]float64{w * 0.15, -h / 2, -w / 2, h * 0.08, -w * 0.05, h * 0.08, -w * 0.15, h / 2, w / 2, -h * 0.08, w * 0.05, -h * 0.08}, a)
		},
	},
	"lin-cyl": {
		inset: capInset,
		span: func(w, h float64, sideways bool) float64 {
			if sideways {
				return h - 2*shapeCorner - 20
			}
			return float64(w * 0.6)
		},
		size: func(tw, th float64) ShapeDims {
			w := max(tw+2*PadX, 56)
			ry := capRy(w)
			return ShapeDims{w, th + 2*PadY + 2*ry + 8, ry/2 + 4}
		},
		draw: func(w, h float64, a, line string) string {
			rx := w / 2
			ry := capRy(w)
			top := -h/2 + ry
			bottom := h/2 - ry
			arc := func(y float64) string {
				return "M" + Num(-rx) + "," + Num(y) + "A" + Num(rx) + "," + Num(ry) + " 0 0 0 " + Num(rx) + "," + Num(y)
			}
			return shapePath("M"+Num(-rx)+","+Num(top)+"A"+Num(rx)+","+Num(ry)+" 0 0 1 "+Num(rx)+","+Num(top)+"V"+Num(bottom)+
				"A"+Num(rx)+","+Num(ry)+" 0 0 1 "+Num(-rx)+","+Num(bottom)+"Z", a) +
				fillNone(arc(top)+arc(top+8), line)
		},
	},
	"curv-trap": {
		size: slanted(0.6),
		draw: func(w, h float64, a, _ string) string {
			x := w / 2
			y := h / 2
			k := float64(h * 0.3)
			return shapePath("M"+Num(-x+k)+","+Num(-y)+"H"+Num(x-k)+"A"+Num(k)+","+Num(y)+" 0 0 1 "+Num(x-k)+","+Num(y)+"H"+Num(-x+k)+"L"+Num(-x)+",0Z", a)
		},
	},
	"docs": {
		size:  padded(8, 18, -1),
		inset: sideInset(is(2), constant(10)),
		draw: func(w, h float64, a, _ string) string {
			var out strings.Builder
			for i := 0; i < 3; i++ {
				out.WriteString(`<g transform="translate(` + strconv.Itoa((1-i)*4) + "," + strconv.Itoa((i-1)*4) + `)">` + wave(w-8, h-8, a) + "</g>")
			}
			return out.String()
		},
	},
	"bow-rect": {
		size:  slanted(0.4),
		inset: sideInset(is(1), func(_, h float64) float64 { return float64(h * 0.2) }),
		draw: func(w, h float64, a, _ string) string {
			x := w / 2
			y := h / 2
			k := float64(h * 0.2)
			return shapePath("M"+Num(-x+k)+","+Num(-y)+"H"+Num(x)+"A"+Num(k)+","+Num(y)+" 0 0 0 "+Num(x)+","+Num(y)+"H"+Num(-x+k)+
				"A"+Num(k)+","+Num(y)+" 0 0 1 "+Num(-x+k)+","+Num(-y)+"Z", a)
		},
	},
	"tag-rect": {
		size: boxSize,
		span: flat,
		draw: func(w, h float64, a, line string) string {
			return shapeRect(w, h, a, roundedRx) + fillNone("M"+Num(w/2-12)+","+Num(h/2)+"L"+Num(w/2)+","+Num(h/2-12), line)
		},
	},
	"tag-doc": {
		size:  padded(0, 10, -5),
		inset: sideInset(is(2), constant(10)),
		draw: func(w, h float64, a, line string) string {
			return wave(w, h, a) + fillNone("M"+Num(w/2-12)+","+Num(h/2-14)+"L"+Num(w/2)+","+Num(h/2-26), line)
		},
	},
	"lin-doc": {
		size:  padded(8, 10, -5),
		inset: sideInset(is(2), constant(10)),
		draw: func(w, h float64, a, line string) string {
			return wave(w, h, a) + fillNone("M"+Num(-w/2+8)+","+Num(-h/2)+"V"+Num(h/2-6), line)
		},
	},
	"collapsedGroup": {
		size: padded(8, 8, -2),
		draw: func(w, h float64, a, _ string) string {
			return `<rect x="` + Num(-w/2+6) + `" y="` + Num(-h/2+8) + `" width="` + Num(w-12) + `" height="` + Num(h-8) + `"` + roundedRx + a + "/>" +
				`<rect x="` + Num(-w/2) + `" y="` + Num(-h/2) + `" width="` + Num(w) + `" height="` + Num(h-4) + `"` + roundedRx + a + "/>"
		},
	},
}

func quarter(h float64) float64 { return h / 4 }

func brace(x, h, dir float64, line string) string {
	y := h / 2
	k := 6 * dir
	return fillNone("M"+Num(x)+","+Num(-y)+"q"+Num(k)+",0 "+Num(k)+",6V-6q0,6 "+Num(k)+",6q"+Num(-k)+",0 "+Num(-k)+",6V"+Num(y-6)+"q0,6 "+Num(-k)+",6", line)
}

func wave(w, h float64, a string) string {
	x := w / 2
	top := -h / 2
	base := h/2 - 10
	q := w / 4
	return shapePath("M"+Num(-x)+","+Num(top)+"H"+Num(x)+"V"+Num(base)+"q"+Num(-q)+",-12 "+Num(-2*q)+",0t"+Num(-2*q)+",0Z", a)
}

func shapeOf(shape string) *shapeDef {
	if def, ok := shapes[shape]; ok {
		return def
	}
	return shapes["rect"]
}

// ShapeSize is the size of the shape drawn around a label tw by th; an unknown shape is a rect.
func ShapeSize(shape string, tw, th float64) ShapeDims { return shapeOf(shape).size(tw, th) }

// DrawShape draws a shape w by h centered on the origin: `a` holds the attributes of its body,
// `line` those of the lines drawn over it. An unknown shape is a rect.
func DrawShape(shape string, w, h float64, a, line string) string {
	return shapeOf(shape).draw(w, h, a, line)
}

// ShapeInset is how far in from the bounding box the outline lies on a side (0 top, 1 right, 2
// bottom, 3 left), `t` along the side from its middle (TS default 0).
func ShapeInset(shape string, w, h float64, side int, t float64) float64 {
	if def, ok := shapes[shape]; ok && def.inset != nil {
		return def.inset(w, h, side, t)
	}
	return 0
}

// ShapeSpan is how much of a side of the shape edges may spread along, when the flow runs down
// or, if `sideways`, across.
func ShapeSpan(shape string, w, h float64, sideways bool) float64 {
	s := 0.0
	if def, ok := shapes[shape]; ok && def.span != nil {
		s = def.span(w, h, sideways)
	}
	return max(0, s)
}

// ShapeHasLabel reports whether the shape is drawn with its label.
func ShapeHasLabel(shape string) bool {
	def, ok := shapes[shape]
	return !ok || !def.noLabel
}

// at is route[i], or NaN where JS would read undefined.
func at(route []float64, i int) float64 {
	if i < 0 || i >= len(route) {
		return math.NaN()
	}
	return route[i]
}

// EndSide is the side of a node's layout box that a route end lies on: 0 top, 1 right, 2
// bottom, 3 left, or -1 for none.
func EndSide(route []float64, i int, c Box) int {
	dx := at(route, i) - c.X
	dy := at(route, i+1) - c.Y
	if math.Abs(math.Abs(dy)-c.H/2) < 0.5 {
		if dy < 0 {
			return 0
		}
		return 2
	}
	if math.Abs(math.Abs(dx)-c.W/2) < 0.5 {
		if dx < 0 {
			return 3
		}
		return 1
	}
	return -1
}

// InsetRoute moves a route end from a node's layout box c onto the outline of the shape drawn
// in it, w by h.
func InsetRoute(route []float64, i int, shape string, w, h float64, c Box) {
	if c.IsGroup {
		return
	}
	side := EndSide(route, i, c)
	if side == 0 || side == 2 {
		amount := ShapeInset(shape, w, h, side, route[i]-c.X) + (c.H-h)/2
		if side == 0 {
			route[i+1] += amount
		} else {
			route[i+1] -= amount
		}
	} else if side >= 0 {
		amount := ShapeInset(shape, w, h, side, route[i+1]-c.Y) + (c.W-w)/2
		if side == 3 {
			route[i] += amount
		} else {
			route[i] -= amount
		}
	}
}

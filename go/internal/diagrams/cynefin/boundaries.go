package cynefin

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
)

// The geometry of Mermaid's Cynefin boundaries: seeded wavy lines between the domains and the
// S-shaped cliff between Clear and Chaotic. Curves are flat lists of numbers: a start point, then
// six numbers (two control points and an end point) per cubic segment.

// seededRandom is one step of mulberry32.
func seededRandom(seed float64) float64 {
	t := js.ToInt32(seed + 0x6d2b79f5)
	t = int32(uint32(t^int32(uint32(t)>>15)) * uint32(t|1))
	t ^= t + int32(uint32(t^int32(uint32(t)>>7))*uint32(t|61))
	return float64(uint32(t^int32(uint32(t)>>14))) / 4294967296
}

func hashString(str string) float64 {
	var hash int32
	for _, c := range js.Units(str) {
		hash = (hash << 5) - hash + int32(c)
	}
	return float64(hash)
}

// A configured seed other than zero wins; otherwise the seed comes from the diagram's id.
func resolveSeed(configured float64, id string) float64 {
	if js.IsFinite(configured) && configured != 0 {
		return configured
	}
	return hashString(id)
}

type waveOptions struct {
	segments int
	vertical bool
	// Multipliers Mermaid uses to vary the seed per point and per control point.
	pointStep    float64
	controlStep  float64
	controlShift float64
	pinStart     bool
	pinEnd       bool
}

var (
	fold    = waveOptions{segments: 7, vertical: true, pointStep: 17, controlStep: 31, controlShift: 7}
	horizon = waveOptions{segments: 7, vertical: false, pointStep: 23, controlStep: 37, controlShift: 11}
)

// wave is a wavy line along one axis from `from` to `to`, centered on `center` in the other axis.
// Points jitter by up to the amplitude; a pinned end stays on the center line.
func wave(from, to, center, seed, amplitude float64, o waveOptions) []float64 {
	segments := o.segments
	step := (to - from) / float64(segments)
	along := make([]float64, 0, segments+1)
	across := make([]float64, 0, segments+1)
	for i := 0; i <= segments; i++ {
		pinned := (i == 0 && o.pinStart) || (i == segments && o.pinEnd)
		along = append(along, from+float64(i)*step)
		if pinned {
			across = append(across, center)
		} else {
			across = append(across, center+seededRandom(seed+float64(i)*o.pointStep)*amplitude*2-amplitude)
		}
	}
	var out []float64
	if o.vertical {
		out = []float64{across[0], along[0]}
	} else {
		out = []float64{along[0], across[0]}
	}
	for i := 0; i < segments; i++ {
		mid := (along[i] + along[i+1]) / 2
		sign := 1.0
		if i%2 != 0 {
			sign = -1
		}
		offset := amplitude * 1.5 * sign * seededRandom(seed+float64(i)*o.controlStep+o.controlShift)
		if o.vertical {
			out = append(out, across[i]+offset, mid, across[i+1]-offset, mid, across[i+1], along[i+1])
		} else {
			out = append(out, mid, across[i]+offset, mid, across[i+1]-offset, along[i+1], across[i+1])
		}
	}
	return out
}

// cliff is a steep S from (cx, top) down to (cx, bottom).
func cliff(cx, top, bottom, amplitude float64) []float64 {
	h := bottom - top
	return []float64{
		cx, top,
		cx + amplitude, top + h*0.2, cx - amplitude*1.5, top + h*0.55, cx + amplitude*0.5, top + h*0.75,
		cx - amplitude, top + h*0.85, cx + amplitude*0.3, top + h*0.95, cx, bottom,
	}
}

// forward is the path commands that continue from the curve's start point to its end.
func forward(curve []float64) string {
	var d strings.Builder
	for i := 2; i < len(curve); i += 6 {
		d.WriteString("C" + svg.Num(curve[i]) + "," + svg.Num(curve[i+1]) + " " + svg.Num(curve[i+2]) + "," + svg.Num(curve[i+3]) +
			" " + svg.Num(curve[i+4]) + "," + svg.Num(curve[i+5]))
	}
	return d.String()
}

// backward is the path commands that continue from the curve's end point back to its start.
func backward(curve []float64) string {
	var d strings.Builder
	for i := len(curve) - 6; i >= 2; i -= 6 {
		d.WriteString("C" + svg.Num(curve[i+2]) + "," + svg.Num(curve[i+3]) + " " + svg.Num(curve[i]) + "," + svg.Num(curve[i+1]) +
			" " + svg.Num(curve[i-2]) + "," + svg.Num(curve[i-1]))
	}
	return d.String()
}

func curvePath(curve []float64) string {
	return "M" + svg.Num(curve[0]) + "," + svg.Num(curve[1]) + forward(curve)
}

func ellipsePath(cx, cy, rx, ry float64) string {
	r := svg.Num(rx) + "," + svg.Num(ry)
	return "M" + svg.Num(cx-rx) + "," + svg.Num(cy) + "A" + r + " 0 1,1 " + svg.Num(cx+rx) + "," + svg.Num(cy) +
		"A" + r + " 0 1,1 " + svg.Num(cx-rx) + "," + svg.Num(cy) + "Z"
}

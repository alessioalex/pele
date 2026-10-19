package cynefin

// TEMPORARY stand-in until internal/svg/edges.go lands; delete then.

import "github.com/alessioalex/pele/go/internal/svg"

func tmpMarkerTrim(string) float64 { return 7 }

func tmpMarker(_ string, x, y, dx, dy float64, color string) string {
	nx, ny := -dy, dx
	bx, by := x-dx*8, y-dy*8
	half := 8 * 0.42
	return `<path class="pele-marker" d="M` + svg.Num(x) + "," + svg.Num(y) + "L" + svg.Num(bx+nx*half) + "," + svg.Num(by+ny*half) +
		"L" + svg.Num(bx-nx*half) + "," + svg.Num(by-ny*half) + `Z" fill="` + color + `" stroke="none"/>`
}

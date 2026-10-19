package eventmodeling

import "github.com/alessioalex/pele/go/internal/svg"

// TEMPORARY until svg.Marker lands.
func markerTrim(typ string) float64 {
	if typ == "arrow_point" {
		return 7
	}
	return 0
}

func marker(typ string, x, y, dx, dy float64, color string) string {
	nx := -dy
	ny := dx
	bx := x - dx*8
	by := y - dy*8
	half := 8 * 0.42
	return `<path class="pele-marker" d="M` + svg.Num(x) + "," + svg.Num(y) + "L" + svg.Num(bx+nx*half) + "," + svg.Num(by+ny*half) + "L" + svg.Num(bx-nx*half) + "," + svg.Num(by-ny*half) + `Z" fill="` + color + `" stroke="none"/>`
}

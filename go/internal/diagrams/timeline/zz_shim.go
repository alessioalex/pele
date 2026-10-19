package timeline

// TEMPORARY until svg/edges.go lands.
import "github.com/alessioalex/pele/go/internal/svg"

const arrow = 8.0

func marker(typ string, x, y, dx, dy float64, color string) string {
	nx := -dy
	ny := dx
	bx := x - dx*arrow
	by := y - dy*arrow
	half := arrow * 0.42
	return `<path class="pele-marker" d="M` + svg.Num(x) + "," + svg.Num(y) + "L" + svg.Num(bx+nx*half) + "," + svg.Num(by+ny*half) + "L" + svg.Num(bx-nx*half) + "," + svg.Num(by-ny*half) + `Z" fill="` + color + `" stroke="none"/>`
}

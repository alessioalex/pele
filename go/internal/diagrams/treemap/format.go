package treemap

import (
	"math"
	"regexp"
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
)

// The parts of d3-format's specifier that treemap values use: an optional `$`, `,` for thousands
// separators, a precision, and a type of `f`, `%`, `d`, `e` or none. Fill, alignment and width are
// read and ignored. Anything else falls back to the default, `,`.
var reSpec = regexp.MustCompile(`^(?:[^\n\r\x{2028}\x{2029}]?[<>=^])?([-+( ])?(\$)?0?\d*(,)?(?:\.(\d+))?(~)?([a-zA-Z%])?$`)

func group(digits string) string {
	out := ""
	for i := len(digits); i > 0; i -= 3 {
		part := digits[max(0, i-3):i]
		if out != "" {
			out = part + "," + out
		} else {
			out = part
		}
	}
	return out
}

// trimZeros is .replace(/\.?0+$/, ”).
func trimZeros(s string) string {
	t := strings.TrimRight(s, "0")
	if t == s {
		return s
	}
	return strings.TrimSuffix(t, ".")
}

func formatter(spec string) func(float64) string {
	// Mermaid documents `$0,0` for a dollar amount with thousands separators.
	if spec == "$0,0" {
		spec = "$,"
	}
	m := reSpec.FindStringSubmatchIndex(spec)
	if m == nil {
		spec = ","
		m = reSpec.FindStringSubmatchIndex(spec)
	}
	has := func(i int) bool { return m[2*i] >= 0 }
	get := func(i int) string {
		if !has(i) {
			return ""
		}
		return spec[m[2*i]:m[2*i+1]]
	}
	plus := get(1) == "+"
	comma := has(3)
	precision := -1
	if has(4) {
		precision = int(math.Min(js.ToNumber(get(4)), 20))
	}
	typ := get(6)
	prec := func(def int) int {
		if precision < 0 {
			return def
		}
		return precision
	}
	return func(value float64) string {
		abs := math.Abs(value)
		var body, suffix string
		switch typ {
		case "f":
			body = js.ToFixed(abs, prec(6))
		case "%":
			body = js.ToFixed(abs*100, prec(6))
			suffix = "%"
		case "d":
			body = js.ToFixed(js.Round(abs), 0)
		case "e":
			body = js.ToExponential(abs, prec(6))
		default:
			body = js.NumberToString(js.ToNumber(js.ToPrecision(abs, max(1, prec(12)))))
		}
		if has(5) && strings.Contains(body, ".") && !strings.Contains(body, "e") {
			body = trimZeros(body)
		}
		if comma && !strings.Contains(body, "e") {
			if dot := strings.IndexByte(body, '.'); dot == -1 {
				body = group(body)
			} else {
				body = group(body[:dot]) + body[dot:]
			}
		}
		sign := ""
		if value < 0 {
			sign = "-"
		} else if plus {
			sign = "+"
		}
		return sign + get(2) + body + suffix
	}
}

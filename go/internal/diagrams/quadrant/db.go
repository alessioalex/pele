package quadrant

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

// Text is a label and whether it is markdown ("markdown") or plain ("text").
type Text struct {
	Text string
	Type string
}

// PointStyle is a point's own style or its class's. Empty strings are unset.
type PointStyle struct {
	HasRadius   bool
	Radius      float64
	Color       string
	StrokeColor string
	StrokeWidth string
}

// Point is a plotted point.
type Point struct {
	PointStyle
	X, Y      float64
	Text      *Text
	ClassName string
}

// Db is the parsed quadrant chart. Quadrants 1 to 4 are top right, top left, bottom left,
// bottom right; nil is undefined.
type Db struct {
	Title       string
	AccTitle    string
	AccDescr    string
	Quadrants   [4]*Text
	XAxisLeft   *Text
	XAxisRight  *Text
	YAxisBottom *Text
	YAxisTop    *Text
	Points      []Point
	Classes     map[string]PointStyle
}

func newDb() *Db { return &Db{Classes: map[string]PointStyle{}} }

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDigit(int(s[i])) {
			return false
		}
	}
	return true
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')
}

// isHex is /^#?([\dA-Fa-f]{6}|[\dA-Fa-f]{3})$/.
func isHex(s string) bool {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 && len(s) != 3 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isHexDigit(s[i]) {
			return false
		}
	}
	return true
}

// isPixels is /^\d+px$/.
func isPixels(s string) bool {
	return strings.HasSuffix(s, "px") && allDigits(s[:len(s)-2])
}

func invalid(style, value, kind string) *core.Error {
	return &core.Error{Message: "value for " + style + " " + value + " is invalid, please use a valid " + kind,
		Code: core.CodeSemantic, Type: "quadrantChart"}
}

// keyValue is the first two parts of Mermaid's `style.trim().split(/\s*:\s*/)`, found in one pass.
func keyValue(style string) (string, string) {
	s := js.Trim(style)
	colon := strings.IndexByte(s, ':')
	if colon == -1 {
		return s, "undefined"
	}
	rest := js.TrimStart(s[colon+1:])
	next := strings.IndexByte(rest, ':')
	if next == -1 {
		return js.TrimEnd(s[:colon]), rest
	}
	return js.TrimEnd(s[:colon]), js.TrimEnd(rest[:next])
}

func parseStyles(styles []string) PointStyle {
	var out PointStyle
	for _, style := range styles {
		key, value := keyValue(style)
		switch key {
		case "radius":
			if !allDigits(value) {
				panic(invalid(key, value, "number"))
			}
			out.HasRadius = true
			out.Radius = js.ParseInt(value, 0)
		case "color":
			if !isHex(value) {
				panic(invalid(key, value, "hex code"))
			}
			out.Color = value
		case "stroke-color":
			if !isHex(value) {
				panic(invalid(key, value, "hex code"))
			}
			out.StrokeColor = value
		case "stroke-width":
			if !isPixels(value) {
				panic(invalid(key, value, "number of pixels (eg. 10px)"))
			}
			out.StrokeWidth = value
		default:
			panic(&core.Error{Message: "style named " + key + " is not supported.", Code: core.CodeSemantic, Type: "quadrantChart"})
		}
	}
	return out
}

func trimmed(label *Text) *Text {
	return &Text{Text: js.Trim(label.Text), Type: label.Type}
}

func (db *Db) addPoint(label *Text, className, x, y string, styles []string) {
	db.Points = append(db.Points, Point{X: js.ToNumber(x), Y: js.ToNumber(y), Text: trimmed(label), ClassName: className,
		PointStyle: parseStyles(styles)})
}

func (db *Db) addClass(className string, styles []string) {
	db.Classes[className] = parseStyles(styles)
}

// setAccTitle drops leading whitespace.
func (db *Db) setAccTitle(text string) {
	db.AccTitle = js.TrimStart(text)
}

// setAccDescription is .replace(/\n\s+/g, '\n').
func (db *Db) setAccDescription(text string) {
	var b strings.Builder
	for i := 0; i < len(text); {
		if text[i] == '\n' {
			j := i + 1
			for j < len(text) {
				r, size := js.RuneAt(text, j)
				if !js.IsSpace(r) {
					break
				}
				j += size
			}
			b.WriteByte('\n')
			i = j
			continue
		}
		b.WriteByte(text[i])
		i++
	}
	db.AccDescr = b.String()
}

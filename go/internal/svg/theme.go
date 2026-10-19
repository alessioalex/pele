package svg

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
)

// RootStyle holds private aliases set once on the root element, so each use is short and the
// public --pele-* tokens keep their fallbacks. The text color is set as well: an icon is drawn in
// currentColor, which would otherwise be whatever color the page gives the diagram's container.
const RootStyle = "--_bg:var(--pele-bg,#fff);--_fg:var(--pele-fg,#1f1f1f);--_m:var(--pele-muted,#6e6e6e);" +
	"--_l:var(--pele-line,#1f1f1f);--_s:var(--pele-surface,#f3f3f3);--_a:var(--pele-surface-alt,#e6e6e6);" +
	"--_b:var(--pele-border,#8a8a8a);--_c:var(--pele-accent,#5b7bd5);color:var(--_fg)"

const (
	FontMono = "var(--_fm)"
	Radius   = "var(--pele-radius,4px)"
	// The color of what a diagram marks as critical. It is not one of the series colors, as a
	// theme may put those in any order.
	Critical = "var(--pele-critical,#e45756)"
)

// jsSpace is the class of JavaScript's `\s`, which Go's regexp `\s` is narrower than.
const jsSpace = `[\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`

// A list of font names, quoted or not, and nothing that could end the declaration it is put in.
const family = `(?:"[^"'\\\n<>{}();:@]*"|'[^"'\\\n<>{}();:@]*'|[\p{L}\p{N}_-][\p{L}\p{N}_ .-]*)`

var reFamilies = regexp.MustCompile(`^` + jsSpace + `*` + family + `(?:` + jsSpace + `*,` + jsSpace + `*` + family + `)*` + jsSpace + `*$`)

func families(list, generic string) string {
	if list != "" && reFamilies.MatchString(list) {
		return list
	}
	return generic
}

// FontStyle pins what text inherits. The fonts the labels were measured in are the ones they
// are drawn in, unless the page names others. The rest is pinned, as none of it was measured.
func FontStyle(size float64, family, mono string) string {
	return ";--_fm:var(--pele-font-mono," + families(mono, "monospace") + ");font:" + js.Num(size) + "px sans-serif;" +
		"letter-spacing:normal;word-spacing:normal;text-transform:none;direction:ltr;" +
		"font-family:var(--pele-font," + families(family, "sans-serif") + ")"
}

var shapeProps = map[string]bool{
	"fill": true, "fill-opacity": true, "stroke": true, "stroke-width": true, "stroke-dasharray": true,
	"stroke-dashoffset": true, "stroke-linecap": true, "stroke-linejoin": true, "stroke-opacity": true,
	"opacity": true, "rx": true, "ry": true,
}

var textProps = map[string]bool{
	"color": true, "font-size": true, "font-family": true, "font-weight": true, "font-style": true,
	"text-decoration": true, "letter-spacing": true, "word-spacing": true, "opacity": true,
}

// callAfter reports whether lower holds name followed by optional whitespace and "(".
func callAfter(lower, name string) bool {
	for i := strings.Index(lower, name); i >= 0; {
		j := i + len(name)
		for j < len(lower) {
			r, size := js.RuneAt(lower, j)
			if !js.IsSpace(r) {
				break
			}
			j += size
		}
		if j < len(lower) && lower[j] == '(' {
			return true
		}
		next := strings.Index(lower[i+1:], name)
		if next < 0 {
			break
		}
		i += 1 + next
	}
	return false
}

// Unsafe is RE_UNSAFE: /url\s*\(|expression\s*\(|javascript:|[<>{};\\]|@import|image-set|\/\*/i.
func Unsafe(value string) bool {
	lower := js.ASCIILower(value)
	return callAfter(lower, "url") || callAfter(lower, "expression") || strings.Contains(lower, "javascript:") ||
		strings.ContainsAny(lower, "<>{};\\") || strings.Contains(lower, "@import") ||
		strings.Contains(lower, "image-set") || strings.Contains(lower, "/*")
}

// ResolvedStyle is Mermaid style declarations split by what they apply to.
type ResolvedStyle struct {
	Shape     string
	Line      string
	Text      string
	Stroke    string
	HasStroke bool
	Bold      bool
	Italic    bool
	// FontSize is 0 when no usable size was given.
	FontSize float64
}

// orderedMap keeps the first position of each key, as a Map does.
type orderedMap struct {
	keys []string
	vals map[string]string
}

func (m *orderedMap) set(k, v string) {
	if m.vals == nil {
		m.vals = map[string]string{}
	}
	if _, ok := m.vals[k]; !ok {
		m.keys = append(m.keys, k)
	}
	m.vals[k] = v
}

func removeImportant(s string) string {
	lower := js.ASCIILower(s)
	if !strings.Contains(lower, "!important") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if strings.HasPrefix(lower[i:], "!important") {
			i += len("!important")
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// isPxSize is /^[\d.]+(px)?$/.
func isPxSize(s string) bool {
	body := strings.TrimSuffix(s, "px")
	if body == "" {
		return false
	}
	for i := 0; i < len(body); i++ {
		if !(body[i] == '.' || (body[i] >= '0' && body[i] <= '9')) {
			return false
		}
	}
	return true
}

// ResolveStyle splits Mermaid style declarations into what applies to a shape and what applies
// to its text.
func ResolveStyle(declarations []string) ResolvedStyle {
	if len(declarations) == 0 {
		return ResolvedStyle{}
	}
	var shape, txt orderedMap
	for _, decl := range declarations {
		at := strings.IndexByte(decl, ':')
		if at <= 0 {
			continue
		}
		prop := js.ToLower(js.Trim(decl[:at]))
		value := js.Trim(removeImportant(decl[at+1:]))
		if value == "" || Unsafe(value) {
			continue
		}
		if shapeProps[prop] {
			shape.set(prop, value)
		}
		if textProps[prop] {
			if prop == "color" {
				txt.set("fill", value)
			} else {
				txt.set(prop, value)
			}
		}
	}
	if len(shape.keys) == 0 && len(txt.keys) == 0 {
		return ResolvedStyle{}
	}
	var shapeCSS, lineCSS, textCSS strings.Builder
	for _, prop := range shape.keys {
		decl := prop + ":" + shape.vals[prop] + ";"
		shapeCSS.WriteString(decl)
		if prop != "fill" && prop != "fill-opacity" {
			lineCSS.WriteString(decl)
		}
	}
	for _, prop := range txt.keys {
		textCSS.WriteString(prop + ":" + txt.vals[prop] + ";")
	}
	out := ResolvedStyle{}
	if shapeCSS.Len() > 0 {
		out.Shape = ` style="` + Esc(shapeCSS.String()) + `"`
	}
	if lineCSS.Len() > 0 {
		out.Line = ` style="` + Esc(lineCSS.String()) + `"`
	}
	if textCSS.Len() > 0 {
		out.Text = ` style="` + Esc(textCSS.String()) + `"`
	}
	out.Stroke, out.HasStroke = shape.vals["stroke"]
	weight, hasWeight := txt.vals["font-weight"]
	out.Bold = hasWeight && (weight == "bold" || weight == "bolder" || js.ToNumber(weight) >= 600)
	out.Italic = txt.vals["font-style"] == "italic"
	if size, ok := txt.vals["font-size"]; ok && size != "" && isPxSize(size) {
		if px := js.ParseFloat(size); px > 0 && px <= 200 {
			out.FontSize = px
		}
	}
	return out
}

// ClassNames turns a class list from a diagram into class names, with a leading space.
func ClassNames(raw string) string {
	var b strings.Builder
	gap := false
	for _, r := range raw {
		if js.IsWord(r) || r == '-' {
			if gap && b.Len() > 0 {
				b.WriteByte(' ')
			}
			gap = false
			b.WriteRune(r)
		} else {
			gap = true
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return " " + b.String()
}

var series = []string{"#4c78a8", "#f58518", "#54a24b", "#e45756", "#72b7b2", "#eeca3b", "#b279a2", "#9d755d"}

// SeriesColor is the color for the nth data series in a chart, cycling through eight tokens.
func SeriesColor(index int) string {
	k := ((index % 8) + 8) % 8
	return "var(--pele-series-" + strconv.Itoa(k+1) + "," + series[k] + ")"
}

package common

import (
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
	"github.com/alessioalex/pele/go/internal/text"
)

// FitWidth draws a chart whose plot can be any width so that the whole diagram fits the width
// the host has. What surrounds the plot (labels, a legend) is only known once it is drawn, so
// the plot gives up the overflow and is drawn again. `narrow` tells the chart that it did not
// fit at its own width, for a chart that then arranges itself differently.
func FitWidth(options *core.Options, natural, least float64, draw func(plot float64, narrow bool) core.Rendered) core.Rendered {
	result := draw(natural, false)
	available, ok := options.Available()
	if !ok || result.Width <= available {
		return result
	}
	floor := math.Min(least, natural)
	plot := natural
	result = draw(plot, true)
	for round := 0; round < 3 && result.Width > available && plot > floor; round++ {
		plot = math.Max(floor, plot-(result.Width-available))
		result = draw(plot, true)
	}
	return result
}

// TitleRoom is the width a title may take before it wraps.
func TitleRoom(options *core.Options) float64 {
	if available, ok := options.Available(); ok {
		return math.Max(80, available-2*options.PaddingOr(8))
	}
	return 4000
}

// CapWidth is for a chart that is drawn to a set width: its own, or the host's when that is
// smaller. `around` is what the chart adds outside that width.
func CapWidth(options *core.Options, natural, least, around float64) float64 {
	available, ok := options.Available()
	if !ok {
		return natural
	}
	return math.Max(math.Min(least, natural), math.Min(natural, available-around))
}

const breakpoint = 640

// TurnToFit is for a diagram that can run across or down: it draws it across, and down instead
// when the host allows it, the width it has is under the breakpoint, across does not fit, and
// down is narrower.
func TurnToFit(options *core.Options, draw func(down bool) core.Rendered) core.Rendered {
	across := draw(false)
	available, ok := options.Available()
	if (options.AutoDirection != nil && !*options.AutoDirection) || !ok || across.Width <= available {
		return across
	}
	bp := float64(breakpoint)
	if b := options.DirectionBreakpoint; b != nil && *b > 0 {
		bp = *b
	}
	if available >= bp {
		return across
	}
	down := draw(true)
	if down.Width < across.Width {
		return down
	}
	return across
}

// RunsAcross reports whether a direction runs left to right or right to left.
func RunsAcross(direction string) bool { return direction == "LR" || direction == "RL" }

// Tighten draws a diagram with room to spare, and tightly instead when that does not fit the
// width the host has and tight is narrower.
func Tighten(options *core.Options, draw func(tight bool) core.Rendered) core.Rendered {
	loose := draw(false)
	available, ok := options.Available()
	if !ok || loose.Width <= available {
		return loose
	}
	tight := draw(true)
	if tight.Width < loose.Width {
		return tight
	}
	return loose
}

// tagStarts is .replace(/<(?!br\s*\/?>)/gi, '&lt;').
func tagStarts(s string) string {
	var b strings.Builder
	last := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '<' {
			continue
		}
		if isBr(s, i) {
			continue
		}
		b.WriteString(s[last:i])
		b.WriteString("&lt;")
		last = i + 1
	}
	b.WriteString(s[last:])
	return b.String()
}

// isBr matches /<br\s*\/?>/i at i, returning whether it does.
func isBr(s string, i int) bool { return brEnd(s, i) >= 0 }

func brEnd(s string, i int) int {
	if !js.HasPrefixFold(s[i+1:], "br") {
		return -1
	}
	j := skipJSSpace(s, i+3)
	if j < len(s) && s[j] == '/' {
		j++
	}
	if j < len(s) && s[j] == '>' {
		return j + 1
	}
	return -1
}

// PlainLabel lays out text the way Mermaid's journey and timeline draw it: as plain text in
// which only <br> is markup, so other tags and icon names show as written instead of being
// read as formatting.
func PlainLabel(raw string, m text.Measurer, size, max float64, base int) text.Label {
	t := raw
	if strings.Contains(t, "<") {
		t = tagStarts(t)
	}
	if strings.Contains(t, ":fa-") {
		t = strings.ReplaceAll(t, ":fa-", "&colon;fa-")
	}
	return text.Layout(t, false, m, size, max, base)
}

// units splits text by /<br\s*\/?>|ﬂ°°?\w+¶ß|&#?\w+;|[\ud800-\udbff][\udc00-\udfff]|[\s\S]/gi: the
// pieces a word can be cut between.
func units(s string) []string {
	var out []string
	for i := 0; i < len(s); {
		if s[i] == '<' {
			if e := brEnd(s, i); e >= 0 {
				out = append(out, s[i:e])
				i = e
				continue
			}
		}
		if strings.HasPrefix(s[i:], "ﬂ°") {
			j := i + len("ﬂ°")
			if strings.HasPrefix(s[j:], "°") {
				// `°°?` is greedy: try the second degree sign first.
				if e := wordRunThen(s, j+len("°"), "¶ß"); e >= 0 {
					out = append(out, s[i:e])
					i = e
					continue
				}
			}
			if e := wordRunThen(s, j, "¶ß"); e >= 0 {
				out = append(out, s[i:e])
				i = e
				continue
			}
		}
		if s[i] == '&' {
			j := i + 1
			if j < len(s) && s[j] == '#' {
				if e := wordRunThen(s, j+1, ";"); e >= 0 {
					out = append(out, s[i:e])
					i = e
					continue
				}
			}
			if e := wordRunThen(s, j, ";"); e >= 0 {
				out = append(out, s[i:e])
				i = e
				continue
			}
		}
		_, size := js.RuneAt(s, i)
		out = append(out, s[i:i+size])
		i += size
	}
	return out
}

func wordRunThen(s string, i int, end string) int {
	j := i
	for j < len(s) && js.IsWord(rune(s[j])) {
		j++
	}
	if j == i || !strings.HasPrefix(s[j:], end) {
		return -1
	}
	return j + len(end)
}

// FitLabel is PlainLabel that also cuts any word too long to fit a box of fixed width.
// text.Layout wraps at spaces only.
func FitLabel(raw string, m text.Measurer, size, max float64, base int) text.Label {
	label := PlainLabel(raw, m, size, max, base)
	if raw == "" || label.Width <= max {
		return label
	}
	var out strings.Builder
	width := 0.0
	for _, unit := range units(raw) {
		n := js.Len(unit)
		if js.Trim(unit) == "" || (n > 2 && unit[0] == '<') {
			width = 0
		} else {
			measured := unit
			if n > 2 {
				measured = "x"
			}
			w := m.Width(measured, size, base)
			if width > 0 && width+w > max {
				out.WriteByte('\n')
				width = 0
			}
			width += w
		}
		out.WriteString(unit)
	}
	return PlainLabel(out.String(), m, size, max, base)
}

// Fitted is one line of text fitted into a width.
type Fitted struct {
	Text  string
	Size  float64
	Width float64
}

// FitText fits one line of text into a width: first by shrinking the font down to min, then by
// cutting the text and adding an ellipsis. ok is false when not even one character fits.
func FitText(t string, max, size, min float64, m text.Measurer, style int) (Fitted, bool) {
	if t == "" || max <= 0 {
		return Fitted{}, false
	}
	width := m.Width(t, size, style)
	if width <= max {
		return Fitted{t, size, width}, true
	}
	// Width grows with font size, so one division lands on the right size or just above it.
	fitted := math.Max(min, math.Floor(size*max/width))
	for fitted >= min {
		width = m.Width(t, fitted, style)
		if width <= max {
			return Fitted{t, fitted, width}, true
		}
		fitted--
	}
	chars := []rune(t)
	low, high := 0, len(chars)-1
	for low < high {
		mid := (low + high + 1) >> 1
		if m.Width(js.TrimEnd(string(chars[:mid]))+"…", min, style) <= max {
			low = mid
		} else {
			high = mid - 1
		}
	}
	if low == 0 {
		return Fitted{}, false
	}
	cut := js.TrimEnd(string(chars[:low])) + "…"
	width = m.Width(cut, min, style)
	if width <= max {
		return Fitted{cut, min, width}, true
	}
	return Fitted{}, false
}

// IconSvg draws a square for a named icon, in the form labels use. The host's resolver
// supplies the drawing, if any.
func IconSvg(name string, x, y, side float64, icons text.IconResolver) string {
	inner := ""
	if icons != nil {
		inner, _ = icons(name)
	}
	return `<svg class="pele-icon" data-icon="` + svg.EscText(name) + `" x="` + svg.Num(x) + `" y="` + svg.Num(y) +
		`" width="` + svg.Num(side) + `" height="` + svg.Num(side) +
		`" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
		inner + "</svg>"
}

// WordAt reports whether word, written in lower case, is at byte offset at. Only ASCII letters
// match in either case, as with the `i` flag on a regular expression without the `u` flag.
func WordAt(src string, at int, word string) bool {
	if at < 0 || at+len(word) > len(src) {
		return false
	}
	for k := 0; k < len(word); k++ {
		c, w := src[at+k], word[k]
		if c != w && !(w >= 'a' && w <= 'z' && c == w-32) {
			return false
		}
	}
	return true
}

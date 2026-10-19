package svg

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/text"
)

// A rectangle's corner radius comes from a variable. Browsers only lately take a variable in
// the `rx` attribute, and all of them take one in a style, so that is where it is written. It
// goes first there, for a radius the diagram's author set to come after it and win.
const roundedAttr = ` rx="` + Radius + `"`

func rounded(content string) string {
	if !strings.Contains(content, roundedAttr) {
		return content
	}
	var b strings.Builder
	last := 0
	for i := 0; i < len(content); {
		j := strings.Index(content[i:], "<rect")
		if j < 0 {
			break
		}
		start := i + j
		after := start + len("<rect")
		if r, _ := js.RuneAt(content, after); after < len(content) && js.IsWord(r) {
			i = after
			continue
		}
		end := strings.IndexByte(content[after:], '>')
		if end < 0 {
			break
		}
		end += after + 1
		tag := content[start:end]
		b.WriteString(content[last:start])
		b.WriteString(roundTag(tag))
		last = end
		i = end
	}
	b.WriteString(content[last:])
	return b.String()
}

func roundTag(tag string) string {
	at := strings.Index(tag, roundedAttr)
	if at < 0 {
		return tag
	}
	rest := tag[:at] + tag[at+len(roundedAttr):]
	if style := strings.Index(rest, ` style="`); style >= 0 {
		return rest[:style+8] + "rx:var(--_r);" + rest[style+8:]
	}
	end := len(rest) - 1
	if strings.HasSuffix(rest, "/>") {
		end = len(rest) - 2
	}
	return rest[:end] + ` style="rx:var(--_r)"` + rest[end:]
}

// The weights of bold text, each set only in a diagram that has such text.
var weights = [][2]string{{"--_tw", "title"}, {"--_hw", "heading"}, {"--_w", "bold"}}

// Accessible holds a diagram's accessible title and description.
type Accessible struct {
	AccTitle string
	AccDescr string
}

// Document wraps drawn content in the root svg element.
func Document(typ string, width, height, fontSize float64, options *core.Options, acc Accessible, content string) string {
	prefix := Esc(options.IDPrefixOr("pele"))
	head := ""
	aria := ""
	if acc.AccTitle != "" {
		head += `<title id="` + prefix + `-title">` + EscText(acc.AccTitle) + "</title>"
		aria += ` aria-labelledby="` + prefix + `-title"`
	}
	if acc.AccDescr != "" {
		head += `<desc id="` + prefix + `-desc">` + EscText(acc.AccDescr) + "</desc>"
		aria += ` aria-describedby="` + prefix + `-desc"`
	}
	fit := "max-width:100%;height:auto;"
	if options.Responsive != nil && !*options.Responsive {
		fit = ""
	}
	content = rounded(content)
	w := ""
	if strings.Contains(content, "var(--_r)") {
		w = ";--_r:" + Radius
	}
	for _, pair := range weights {
		if strings.Contains(content, "var("+pair[0]+")") {
			w += ";" + pair[0] + ":var(--pele-" + pair[1] + "-weight,bold)"
		}
	}
	return `<svg xmlns="http://www.w3.org/2000/svg" class="pele pele-` + typ + `" viewBox="0 0 ` + js.Num(width) + " " + js.Num(height) +
		`" width="` + js.Num(width) + `" height="` + js.Num(height) + `" style="` + fit + RootStyle + w +
		Esc(FontStyle(fontSize, options.FontFamilyValue(), options.FontFamilyMonoValue())) +
		`" fill="var(--_fg)" role="graphics-document document" aria-roledescription="` + typ + `"` + aria + ">" +
		head + content + "</svg>"
}

// Titled is content with a title above it.
type Titled struct {
	Width   float64
	Height  float64
	Content string
}

// WithTitle puts a diagram title above content that was laid out without one.
func WithTitle(title string, m text.Measurer, size, pad, width, height float64, content string) Titled {
	label := text.Layout(title, false, m, size, 4000, text.Bold)
	if label.Height == 0 {
		return Titled{width, height, content}
	}
	shift := label.Height + 12
	full := max(width, ceil(label.Width+2*pad))
	return Titled{
		Width:  full,
		Height: ceil(height + shift),
		Content: LabelSvg(label, full/2, pad+label.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil) +
			`<g transform="translate(` + Num((full-width)/2) + "," + Num(shift) + `)">` + content + "</g>",
	}
}

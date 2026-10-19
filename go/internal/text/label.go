package text

import (
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
)

// Span is a run of text in one style, or an icon when Icon is set.
type Span struct {
	Text  string
	Style int
	Icon  string
	Width float64
}

// HasIcon reports whether the span is an icon.
func (s *Span) HasIcon() bool { return s.Icon != "" }

// Label is laid-out text: lines of spans and their widths.
type Label struct {
	Lines      [][]Span
	Widths     []float64
	Width      float64
	Height     float64
	Size       float64
	LineHeight float64
}

const LineHeight = 1.5

// isCJK is the range of Chinese and Japanese characters a line can break between. Hangul is
// left out: Korean puts spaces between words.
func isCJK(r rune) bool {
	return (r >= 0x2e80 && r <= 0x9fff) || (r >= 0xf900 && r <= 0xfaff) || (r >= 0xff00 && r <= 0xff60)
}

const (
	closePunct = "、。，．：；！？）］｝」』】〕〉》〙〗ー…‥・ゝゞぁぃぅぇぉっゃゅょゎァィゥェォッャュョヮ,.:;!?)]}"
	openPunct  = "（［｛「『【〔〈《〘〖([{"
)

func isClose(r rune) bool { return strings.ContainsRune(closePunct, r) }
func isOpen(r rune) bool  { return strings.ContainsRune(openPunct, r) }

// HasCJK reports whether s has a character a line can break before or after.
func HasCJK(s string) bool {
	for _, r := range s {
		if isCJK(r) {
			return true
		}
	}
	return false
}

// pieces splits text the way the TypeScript `text.match(RE_PIECE)` does, with RE_PIECE
// `\s*(?:[OPEN]*[CJK][CLOSE]*|[^\sCJK]+[CLOSE]*)\s*|\s+`.
func pieces(s string) []string {
	var out []string
	p := 0
	for p < len(s) {
		q := p
		for q < len(s) {
			r, size := js.RuneAt(s, q)
			if !js.IsSpace(r) {
				break
			}
			q += size
		}
		end := matchWord(s, q)
		if end < 0 {
			// Only the trailing blanks are left, which `\s+` takes.
			if q > p {
				out = append(out, s[p:q])
				p = q
				continue
			}
			_, size := js.RuneAt(s, p)
			p += size
			continue
		}
		for end < len(s) {
			r, size := js.RuneAt(s, end)
			if !js.IsSpace(r) {
				break
			}
			end += size
		}
		out = append(out, s[p:end])
		p = end
	}
	return out
}

// matchWord matches `[OPEN]*[CJK][CLOSE]*|[^\sCJK]+[CLOSE]*` at q and returns its end, or -1.
func matchWord(s string, q int) int {
	if q >= len(s) {
		return -1
	}
	// [OPEN]*[CJK]: the longest run of opening marks that a CJK character follows.
	var opens []int
	i := q
	for i < len(s) {
		r, size := js.RuneAt(s, i)
		if !isOpen(r) {
			break
		}
		opens = append(opens, i)
		i += size
	}
	for k := len(opens); k >= 0; k-- {
		at := i
		if k < len(opens) {
			at = opens[k]
		}
		r, size := js.RuneAt(s, at)
		if at < len(s) && isCJK(r) {
			end := at + size
			for end < len(s) {
				c, n := js.RuneAt(s, end)
				if !isClose(c) {
					break
				}
				end += n
			}
			return end
		}
	}
	end := q
	for end < len(s) {
		r, size := js.RuneAt(s, end)
		if js.IsSpace(r) || isCJK(r) {
			break
		}
		end += size
	}
	if end == q {
		return -1
	}
	for end < len(s) {
		c, n := js.RuneAt(s, end)
		if !isClose(c) {
			break
		}
		end += n
	}
	return end
}

func dedent(text string) string {
	lines := strings.Split(text, "\n")
	min := math.MaxInt
	for _, line := range lines {
		if js.Trim(line) == "" {
			continue
		}
		if indent := leadingSpaces(line); indent < min {
			min = indent
		}
	}
	if min == math.MaxInt || min == 0 {
		return text
	}
	for i, line := range lines {
		n := leadingSpaces(line)
		if min < n {
			n = min
		}
		lines[i] = dropChars(line, n)
	}
	return strings.Join(lines, "\n")
}

// leadingSpaces counts leading whitespace, each of which is one UTF-16 code unit.
func leadingSpaces(s string) int {
	n := 0
	for _, r := range s {
		if !js.IsSpace(r) {
			break
		}
		n++
	}
	return n
}

func dropChars(s string, n int) string {
	for i := range s {
		if n == 0 {
			return s[i:]
		}
		n--
	}
	return ""
}

func pushText(line *[]Span, text string, style int) {
	if text == "" {
		return
	}
	decoded := DecodeEntities(text)
	l := *line
	if len(l) > 0 && l[len(l)-1].Style == style && l[len(l)-1].Icon == "" {
		l[len(l)-1].Text += decoded
		return
	}
	*line = append(l, Span{Text: decoded, Style: style})
}

// nextIcon finds the next match of /(fa[bklrs]?):fa-([\w-]+)/ at or after from.
func nextIcon(text string, from int) (start, end int, icon string) {
	for p := from; p+2 <= len(text); p++ {
		if text[p] != 'f' || text[p+1] != 'a' {
			continue
		}
		q := p + 2
		if q < len(text) && strings.IndexByte("bklrs", text[q]) >= 0 {
			q++
		}
		if !strings.HasPrefix(text[q:], ":fa-") {
			continue
		}
		e := q + 4
		for e < len(text) && (isWordByte(text[e]) || text[e] == '-') {
			e++
		}
		if e == q+4 {
			continue
		}
		return p, e, text[p:q] + ":fa-" + text[q+4:e]
	}
	return -1, -1, ""
}

func pushIcons(m Measurer, line *[]Span, text string, style int) {
	if !strings.Contains(text, ":fa-") {
		pushText(line, text, style)
		return
	}
	last := 0
	for from := 0; ; {
		start, end, icon := nextIcon(text, from)
		if start < 0 {
			break
		}
		pushText(line, text[last:start], style)
		last = end
		from = end
		if IconMarkup(m, icon) != "" {
			*line = append(*line, Span{Style: style, Icon: icon})
		} else if last < len(text) && text[last] == ' ' {
			// An icon the host does not have leaves no gap, nor the space that set it off from the text.
			last++
		} else if last == len(text) {
			l := *line
			if len(l) > 0 && l[len(l)-1].Icon == "" {
				l[len(l)-1].Text = js.TrimEnd(l[len(l)-1].Text)
			}
		}
	}
	pushText(line, text[last:], style)
}

// matchTag matches /<\/?([a-zA-Z][\w-]*)(?:\s[^<>]*)?\/?>/ at s[i] == '<' and returns its end
// and the tag name, or -1.
func matchTag(s string, i int) (int, string) {
	j := i + 1
	if j < len(s) && s[j] == '/' {
		j++
	}
	if j >= len(s) || !((s[j] >= 'a' && s[j] <= 'z') || (s[j] >= 'A' && s[j] <= 'Z')) {
		return -1, ""
	}
	k := j + 1
	for k < len(s) && (isWordByte(s[k]) || s[k] == '-') {
		k++
	}
	name := s[j:k]
	if k < len(s) && s[k] == '>' {
		return k + 1, name
	}
	if k+1 < len(s) && s[k] == '/' && s[k+1] == '>' {
		return k + 2, name
	}
	if r, size := js.RuneAt(s, k); k < len(s) && js.IsSpace(r) {
		e := k + size
		for e < len(s) && s[e] != '<' && s[e] != '>' {
			e++
		}
		if e < len(s) && s[e] == '>' {
			return e + 1, name
		}
	}
	return -1, ""
}

// inline splits inline HTML and, for markdown labels, emphasis markers into styled spans.
func inline(m Measurer, line *[]Span, source string, markdown bool, base int) {
	style := base
	marked := 0
	var text strings.Builder
	flush := func() {
		pushIcons(m, line, text.String(), style)
		text.Reset()
	}
	n := len(source)
	for i := 0; i < n; {
		ch := source[i]
		if ch == '<' {
			if end, name := matchTag(source, i); end >= 0 {
				tag := strings.ToLower(name)
				closing := source[i+1] == '/'
				bit := 0
				if tag == "b" || tag == "strong" {
					bit = Bold
				} else if tag == "i" || tag == "em" {
					bit = Italic
				}
				if bit != 0 {
					flush()
					if closing {
						style = (style &^ bit) | (base & bit)
					} else {
						style |= bit
					}
				}
				i = end
				continue
			}
		} else if markdown && (ch == '*' || ch == '_') {
			double := i+1 < n && source[i+1] == ch
			bit, length := Italic, 1
			if double {
				bit, length = Bold, 2
			}
			open := marked&bit == 0
			prev := ' '
			if i > 0 {
				prev, _ = js.RuneBefore(source, i)
			}
			next := ' '
			if i+length < n {
				next, _ = js.RuneAt(source, i+length)
			}
			marker := string(ch)
			if double {
				marker += marker
			}
			canOpen := open && !js.IsSpace(next) && strings.Contains(source[i+length:], marker)
			canClose := !open && !js.IsSpace(prev)
			wordy := ch == '_' && js.IsWord(prev) && js.IsWord(next)
			if (canOpen || canClose) && !wordy {
				flush()
				if canOpen {
					marked |= bit
					style |= bit
				} else {
					marked &^= bit
					style = (style &^ bit) | (base & bit)
				}
				i += length
				continue
			}
		}
		text.WriteByte(ch)
		i++
	}
	flush()
}

func measureSpans(line []Span, m Measurer, size float64) float64 {
	total := 0.0
	for i := range line {
		if line[i].Icon != "" {
			line[i].Width = size * 1.25
		} else {
			line[i].Width = m.Width(line[i].Text, size, line[i].Style)
		}
		total += line[i].Width
	}
	return total
}

func wrap(line []Span, m Measurer, size, max float64, out *[][]Span, widths *[]float64) {
	var current []Span
	width := 0.0
	commit := func() {
		if len(current) > 0 {
			last := &current[len(current)-1]
			if last.Icon == "" {
				trimmed := js.TrimEnd(last.Text)
				if trimmed != last.Text {
					last.Text = trimmed
					w := m.Width(trimmed, size, last.Style)
					width += w - last.Width
					last.Width = w
				}
			}
		}
		*out = append(*out, current)
		*widths = append(*widths, width)
		current = nil
		width = 0
	}
	for _, span := range line {
		if span.Icon != "" {
			w := size * 1.25
			if width > 0 && width+w > max {
				commit()
			}
			span.Width = w
			current = append(current, span)
			width += w
			continue
		}
		for _, piece := range pieces(span.Text) {
			part := piece
			if width > 0 && width+m.Width(js.TrimEnd(part), size, span.Style) > max {
				commit()
				part = js.TrimStart(part)
			}
			if n := len(current); n > 0 && current[n-1].Style == span.Style && current[n-1].Icon == "" {
				last := &current[n-1]
				last.Text += part
				last.Width = m.Width(last.Text, size, last.Style)
				width = 0
				for _, s := range current {
					width += s.Width
				}
			} else {
				w := m.Width(part, size, span.Style)
				current = append(current, Span{Text: part, Style: span.Style, Width: w})
				width += w
			}
		}
	}
	if len(current) > 0 || len(*out) == 0 {
		commit()
	}
}

// hasMarkup is /[<\\\n*_]|fa[bklrs]?:fa-/.
func hasMarkup(s string) bool {
	if strings.ContainsAny(s, "<\\\n*_") {
		return true
	}
	start, _, _ := nextIconPrefix(s)
	return start >= 0
}

func nextIconPrefix(s string) (int, int, string) {
	for p := 0; p+2 <= len(s); p++ {
		if s[p] != 'f' || s[p+1] != 'a' {
			continue
		}
		q := p + 2
		if q < len(s) && strings.IndexByte("bklrs", s[q]) >= 0 && strings.HasPrefix(s[q+1:], ":fa-") {
			return p, q + 5, ""
		}
		if strings.HasPrefix(s[q:], ":fa-") {
			return p, q + 4, ""
		}
	}
	return -1, -1, ""
}

// splitBreaks splits on /<br\s*\/?>|\\n|\n/gi. The flag makes `\N` a break too.
func splitBreaks(s string) []string {
	var out []string
	last := 0
	for i := 0; i < len(s); {
		switch {
		case s[i] == '\n':
			out = append(out, s[last:i])
			i++
			last = i
			continue
		case s[i] == '\\' && i+1 < len(s) && (s[i+1] == 'n' || s[i+1] == 'N'):
			out = append(out, s[last:i])
			i += 2
			last = i
			continue
		case s[i] == '<' && js.HasPrefixFold(s[i+1:], "br"):
			j := i + 3
			for j < len(s) {
				r, size := js.RuneAt(s, j)
				if !js.IsSpace(r) {
					break
				}
				j += size
			}
			if j < len(s) && s[j] == '/' {
				j++
			}
			if j < len(s) && s[j] == '>' {
				out = append(out, s[last:i])
				i = j + 1
				last = i
				continue
			}
		}
		i++
	}
	return append(out, s[last:])
}

// Layout lays out a label: it reads inline markup, measures, and wraps lines wider than maxWidth.
func Layout(raw string, markdown bool, m Measurer, size, maxWidth float64, base int) Label {
	if raw == "" {
		return Label{}
	}
	lineHeight := js.Round(size * LineHeight)

	if !hasMarkup(raw) && !strings.Contains(raw, "&") && !strings.Contains(raw, "ﬂ") {
		text := js.Trim(raw)
		width := m.Width(text, size, base)
		if width <= maxWidth || !(strings.Contains(text, " ") || HasCJK(text)) {
			return Label{
				Lines:      [][]Span{{{Text: text, Style: base, Width: width}}},
				Widths:     []float64{width},
				Width:      width,
				Height:     lineHeight,
				Size:       size,
				LineHeight: lineHeight,
			}
		}
		var lines [][]Span
		var widths []float64
		wrap([]Span{{Text: text, Style: base, Width: width}}, m, size, maxWidth, &lines, &widths)
		widest := 0.0
		for _, w := range widths {
			if w > widest {
				widest = w
			}
		}
		return Label{Lines: lines, Widths: widths, Width: widest, Height: float64(len(lines)) * lineHeight, Size: size, LineHeight: lineHeight}
	}

	var source string
	if markdown {
		source = js.Trim(dedent(strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")))
	} else {
		source = js.Trim(raw)
	}
	var lines [][]Span
	var widths []float64
	width := 0.0
	for _, part := range splitBreaks(source) {
		var line []Span
		inline(m, &line, js.Trim(part), markdown, base)
		w := measureSpans(line, m, size)
		if w > maxWidth {
			wrap(line, m, size, maxWidth, &lines, &widths)
		} else {
			if line == nil {
				line = []Span{}
			}
			lines = append(lines, line)
			widths = append(widths, w)
		}
	}
	for _, w := range widths {
		if w > width {
			width = w
		}
	}
	return Label{Lines: lines, Widths: widths, Width: width, Height: float64(len(lines)) * lineHeight, Size: size, LineHeight: lineHeight}
}

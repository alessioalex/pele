package common

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
	"github.com/alessioalex/pele/go/internal/text"
)

// LiteralLabel lays out labels that must retain every character but the spaces a line breaks
// at, such as filenames and protocol fields. Long words can break between code points;
// measurement stays bounded by one line of text.
func LiteralLabel(t string, m text.Measurer, size, room float64, style int) text.Label {
	var lines []string
	for _, raw := range strings.Split(t, "\n") {
		chars := []rune(raw)
		if len(chars) == 0 {
			lines = append(lines, "")
		}
		from := 0
		for from < len(chars) {
			fits := func(length int) bool {
				return m.Width(string(chars[from:from+length]), size, style) <= room
			}
			hi := 1
			for hi < len(chars)-from && fits(hi) {
				hi = min(len(chars)-from, hi*2)
			}
			lo := 1
			for lo < hi {
				mid := (lo + hi + 1) / 2
				if fits(mid) {
					lo = mid
				} else {
					hi = mid - 1
				}
			}
			take := lo
			wraps := from+take < len(chars)
			if wraps && !js.IsSpace(chars[from+take]) {
				for i := take - 1; i > 0; i-- {
					if js.IsSpace(chars[from+i]) {
						take = i + 1
						break
					}
				}
			}
			line := string(chars[from : from+take])
			if wraps {
				if trimmed := js.TrimEnd(line); trimmed != "" {
					line = trimmed
				}
			}
			lines = append(lines, line)
			from += take
			for wraps && from < len(chars) && js.IsSpace(chars[from]) {
				from++
			}
		}
	}
	if t == "" {
		lines = nil
	}
	widths := make([]float64, len(lines))
	width := 0.0
	spans := make([][]text.Span, len(lines))
	for i, line := range lines {
		widths[i] = m.Width(line, size, style)
		width = max(width, widths[i])
		spans[i] = []text.Span{{Text: line, Style: style, Width: widths[i]}}
	}
	lineHeight := js.Round(size * 1.5)
	return text.Label{Lines: spans, Widths: widths, Width: width, Height: float64(len(lines)) * lineHeight, Size: size, LineHeight: lineHeight}
}

// BreakLabel cuts the lines of a label that are wider than room between characters.
func BreakLabel(label text.Label, m text.Measurer, room float64) text.Label {
	if label.Width <= room {
		return label
	}
	var lines [][]text.Span
	var widths []float64
	for i, line := range label.Lines {
		if label.Widths[i] <= room {
			lines = append(lines, line)
			widths = append(widths, label.Widths[i])
			continue
		}
		var current []text.Span
		width := 0.0
		commit := func() {
			lines = append(lines, current)
			widths = append(widths, width)
			current = nil
			width = 0
		}
		for _, span := range line {
			if span.Icon != "" {
				if len(current) > 0 && width+span.Width > room {
					commit()
				}
				current = append(current, span)
				width += span.Width
				continue
			}
			for _, ch := range span.Text {
				char := string(ch)
				var last *text.Span
				if len(current) > 0 {
					last = &current[len(current)-1]
				}
				joined := last != nil && last.Icon == "" && last.Style == span.Style
				var next float64
				if joined {
					next = m.Width(last.Text+char, label.Size, span.Style)
				} else {
					next = m.Width(char, label.Size, span.Style)
				}
				added := next
				if joined {
					added = next - last.Width
				}
				if len(current) > 0 && width+added > room {
					commit()
					last = nil
					added = m.Width(char, label.Size, span.Style)
				}
				if last != nil && last.Icon == "" && last.Style == span.Style {
					last.Text += char
					last.Width += added
				} else {
					current = append(current, text.Span{Text: char, Style: span.Style, Width: added})
				}
				width += added
			}
		}
		if len(current) > 0 || len(line) == 0 {
			commit()
		}
	}
	width := 0.0
	for _, w := range widths {
		width = max(width, w)
	}
	out := label
	out.Lines, out.Widths, out.Width, out.Height = lines, widths, width, float64(len(lines))*label.LineHeight
	return out
}

// LeftLabelSvg writes a label with every line starting at x. A line is anchored at its start
// and keeps its spaces, so that where it begins does not depend on how its text was measured.
func LeftLabelSvg(label text.Label, x, top float64, attrs string) string {
	shared := svg.SharedStyle(label)
	open := "<text" + svg.TextStyle(attrs, shared)
	var body strings.Builder
	for i, spans := range label.Lines {
		cy := top + (float64(i)+0.5)*label.LineHeight
		hasIcon := false
		for _, span := range spans {
			if span.Icon != "" {
				hasIcon = true
			}
		}
		if hasIcon {
			w := label.Widths[i]
			one := label
			one.Lines, one.Widths, one.Width, one.Height = [][]text.Span{spans}, []float64{w}, w, label.LineHeight
			body.WriteString(svg.LabelSvg(one, x+w/2, cy, attrs, nil))
			continue
		}
		var t strings.Builder
		for _, span := range spans {
			if own := svg.SpanStyle(span.Style &^ shared); own != "" {
				t.WriteString("<tspan" + own + ">" + svg.Esc(span.Text) + "</tspan>")
			} else {
				t.WriteString(svg.Esc(span.Text))
			}
		}
		if t.Len() > 0 {
			body.WriteString(open + ` x="` + svg.Num(x) + `" y="` + svg.Num(cy+label.Size*0.35) + `" text-anchor="start" xml:space="preserve">` + t.String() + "</text>")
		}
	}
	return body.String()
}

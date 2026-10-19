// Package pre prepares diagram text for the parsers: front matter, directives, comments, entity
// codes, and the detection of the diagram type.
package pre

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

// Preprocessed is the text a parser is given, with what preprocessing read from it.
type Preprocessed struct {
	Text   string
	Title  string
	Config *core.Object
	// The text before its comment lines are removed, and the number of front matter lines before it.
	WithComments string
	LineOffset   int
}

var unsafeKeys = map[string]bool{"__proto__": true, "constructor": true, "prototype": true}

func isLineTerminator(r rune) bool { return js.IsLineEnd(r) }

// matchComment matches /^\s*%%(?!{)[^\n]+\n?/ at p, a line start, and returns its end or -1.
func matchComment(s string, p int) int {
	q := p
	for q < len(s) {
		r, size := js.RuneAt(s, q)
		if !js.IsSpace(r) {
			break
		}
		q += size
	}
	if !strings.HasPrefix(s[q:], "%%") || q+2 >= len(s) || s[q+2] == '{' || s[q+2] == '\n' {
		return -1
	}
	end := q + 2
	nl := strings.IndexByte(s[end:], '\n')
	if nl == -1 {
		return len(s)
	}
	return end + nl + 1
}

// nextLineStart returns the position after the next line terminator at or after p, or -1.
func nextLineStart(s string, p int) int {
	for p < len(s) {
		r, size := js.RuneAt(s, p)
		p += size
		if isLineTerminator(r) {
			return p
		}
	}
	return -1
}

// CleanupComments removes `%%` comment lines, as Mermaid does.
func CleanupComments(text string) string {
	if strings.Contains(text, "%%") {
		var b strings.Builder
		last := 0
		for p := 0; p != -1 && p <= len(text); {
			if end := matchComment(text, p); end != -1 {
				b.WriteString(text[last:p])
				last = end
				if end >= len(text) {
					break
				}
				p = end
				continue
			}
			p = nextLineStart(text, p)
		}
		if last > 0 {
			b.WriteString(text[last:])
			text = b.String()
		}
	}
	return js.TrimStart(text)
}

func isLineBreak(s string, i int) bool { return i < len(s) && (s[i] == '\n' || s[i] == '\r') }

func spaceAt(s string, i int) (bool, int) {
	r, size := js.RuneAt(s, i)
	return i < len(s) && js.IsSpace(r), size
}

type frontMatter struct {
	indent string
	body   string
	length int
}

// matchFrontMatter finds a `---` fenced YAML block at the start of the text the way Mermaid's
// front matter regex does.
func matchFrontMatter(text string) (frontMatter, bool) {
	cursor := 0
	for {
		ok, size := spaceAt(text, cursor)
		if !ok || isLineBreak(text, cursor) {
			break
		}
		cursor += size
	}
	indent := text[:cursor]
	if !strings.HasPrefix(text[cursor:], "---") {
		return frontMatter{}, false
	}
	var openingEnds []int
	for i := cursor + 3; ; {
		ok, size := spaceAt(text, i)
		if !ok {
			break
		}
		if isLineBreak(text, i) {
			openingEnds = append(openingEnds, i)
		}
		i += size
	}
	if len(openingEnds) == 0 {
		return frontMatter{}, false
	}
	fence := indent + "---"
	closingEnd := func(i int) int {
		if !isLineBreak(text, i) || !strings.HasPrefix(text[i+1:], fence) {
			return -1
		}
		last := -1
		for j := i + 1 + len(fence); ; {
			ok, size := spaceAt(text, j)
			if !ok {
				break
			}
			if isLineBreak(text, j) {
				last = j
			}
			j += size
		}
		return last
	}
	needle := "\n" + fence
	lastIndex := func(from int) int {
		end := from + len(needle)
		if end > len(text) {
			end = len(text)
		}
		if end < 0 {
			return -1
		}
		return strings.LastIndex(text[:end], needle)
	}
	lastFence := -1
	for i := strings.LastIndex(text, needle); i > openingEnds[0]; i = lastIndex(i - 1) {
		if closingEnd(i) != -1 {
			lastFence = i
			break
		}
	}
	if lastFence == -1 {
		return frontMatter{}, false
	}
	openingEnd := openingEnds[0]
	for _, candidate := range openingEnds {
		if candidate < lastFence {
			openingEnd = candidate
		}
	}
	bodyStart := openingEnd + 1
	firstAt := 0
	if bodyStart > 0 {
		firstAt = bodyStart - 1
	}
	indexFrom := func(from int) int {
		if from > len(text) {
			return -1
		}
		i := strings.Index(text[from:], needle)
		if i == -1 {
			return -1
		}
		return from + i
	}
	for i := indexFrom(firstAt); i != -1 && i <= lastFence; i = indexFrom(i + 1) {
		if i < bodyStart {
			continue
		}
		if end := closingEnd(i); end != -1 {
			return frontMatter{indent: indent, body: text[bodyStart:i], length: end + 1}, true
		}
	}
	return frontMatter{}, false
}

func merge(target, source *core.Object) *core.Object {
	for _, key := range source.Keys() {
		if unsafeKeys[key] {
			continue
		}
		value := source.Value(key)
		current := target.Value(key)
		if v, ok := value.(*core.Object); ok {
			if c, ok := current.(*core.Object); ok {
				merge(c, v)
			} else {
				target.Set(key, merge(core.NewObject(), v))
			}
		} else {
			target.Set(key, value)
		}
	}
	return target
}

// Merge deep-merges source into target, as Mermaid merges configuration.
func Merge(target, source *core.Object) *core.Object { return merge(target, source) }

type directive struct {
	end     int
	typ     string
	content string
	hasBody bool
}

func wordEnd(s string, i int) int {
	for i < len(s) && js.IsWord(rune(s[i])) {
		i++
	}
	return i
}

func skipSpace(s string, i int) int {
	for i < len(s) {
		r, size := js.RuneAt(s, i)
		if !js.IsSpace(r) {
			break
		}
		i += size
	}
	return i
}

// matchDirective matches /%{2}{\s*(?:(\w+)\s*:|(\w+))\s*(?:(\w+)|((?:(?!}%{2}).|\r?\n)*))?\s*(?:}%{2})?/
// at s[i:], which starts with "%%{".
func matchDirective(s string, i int) (directive, bool) {
	p := skipSpace(s, i+3)
	we := wordEnd(s, p)
	if we == p {
		return directive{}, false
	}
	d := directive{typ: s[p:we]}
	q := skipSpace(s, we)
	if q < len(s) && s[q] == ':' {
		p = q + 1
	} else {
		p = we
	}
	p = skipSpace(s, p)
	if e := wordEnd(s, p); e > p {
		p = e
	} else {
		start := p
		for p < len(s) {
			if strings.HasPrefix(s[p:], "}%%") {
				break
			}
			if s[p] == '\n' {
				p++
				continue
			}
			if s[p] == '\r' && p+1 < len(s) && s[p+1] == '\n' {
				p += 2
				continue
			}
			r, size := js.RuneAt(s, p)
			if js.IsLineEnd(r) {
				break
			}
			p += size
		}
		d.content = s[start:p]
		d.hasBody = true
	}
	p = skipSpace(s, p)
	if strings.HasPrefix(s[p:], "}%%") {
		p += 3
	}
	d.end = p
	return d, true
}

func eachDirective(s string, f func(start int, d directive)) {
	for i := 0; i < len(s); {
		j := strings.Index(s[i:], "%%{")
		if j < 0 {
			return
		}
		start := i + j
		if d, ok := matchDirective(s, start); ok {
			f(start, d)
			i = d.end
			continue
		}
		i = start + 1
	}
}

func directives(text string, config *core.Object) string {
	if !strings.Contains(text, "%%{") {
		return text
	}
	normalized := strings.ReplaceAll(js.Trim(text), "'", "\"")
	eachDirective(normalized, func(_ int, d directive) {
		if d.typ == "wrap" {
			config.Set("wrap", true)
		} else if (d.typ == "init" || d.typ == "initialize") && d.hasBody && d.content != "" {
			// Mermaid ignores an init directive whose body is not valid JSON.
			if args, err := core.ParseJSON(js.Trim(d.content)); err == nil {
				if obj, ok := args.(*core.Object); ok {
					merge(config, obj)
				}
			}
		}
	})
	var b strings.Builder
	last := 0
	eachDirective(text, func(start int, d directive) {
		b.WriteString(text[last:start])
		last = d.end
	})
	b.WriteString(text[last:])
	return b.String()
}

// replaceAttrs is attrs.replace(/="([^"]*)"/g, "='$1'").
func replaceAttrs(attrs string) string {
	if !strings.Contains(attrs, `="`) {
		return attrs
	}
	var b strings.Builder
	last := 0
	for i := 0; i+1 < len(attrs); i++ {
		if attrs[i] != '=' || attrs[i+1] != '"' {
			continue
		}
		close := strings.IndexByte(attrs[i+2:], '"')
		if close < 0 {
			break
		}
		end := i + 2 + close
		b.WriteString(attrs[last:i])
		b.WriteString("='" + attrs[i+2:end] + "'")
		last = end + 1
		i = end
	}
	b.WriteString(attrs[last:])
	return b.String()
}

// htmlTags rewrites /<(\w+)([^>]*)>/g so that attribute values are single-quoted.
func htmlTags(s string) string {
	var b strings.Builder
	last := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '<' {
			continue
		}
		we := wordEnd(s, i+1)
		if we == i+1 {
			continue
		}
		gt := strings.IndexByte(s[we:], '>')
		if gt < 0 {
			continue
		}
		end := we + gt
		b.WriteString(s[last:i])
		b.WriteString("<" + s[i+1:we] + replaceAttrs(s[we:end]) + ">")
		last = end + 1
		i = end
	}
	if last == 0 {
		return s
	}
	b.WriteString(s[last:])
	return b.String()
}

// Preprocess reads front matter and directives and removes comments.
func Preprocess(source string) (*Preprocessed, error) {
	text := source
	if strings.Contains(text, "\r") {
		text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	}
	// A tag needs a closing bracket, so nothing after the last one can match.
	lastClose := -1
	if strings.Contains(text, "<") {
		lastClose = strings.LastIndexByte(text, '>')
	}
	if lastClose > 0 {
		text = htmlTags(text[:lastClose+1]) + text[lastClose+1:]
	}

	config := core.NewObject()
	title := ""
	lineOffset := 0

	if strings.Contains(text, "---") {
		if front, ok := matchFrontMatter(text); ok {
			body := front.body
			if front.indent != "" {
				lines := strings.Split(body, "\n")
				for i, line := range lines {
					if strings.HasPrefix(line, front.indent) {
						lines[i] = line[len(front.indent):]
					}
				}
				body = strings.Join(lines, "\n")
			}
			parsed, err := core.ParseYaml(body)
			if err != nil {
				return nil, core.NewError("Front matter is not valid YAML. "+err.Error(), core.CodeSyntax)
			}
			if obj, ok := parsed.(*core.Object); ok {
				if t := obj.Value("title"); core.Truthy(t) {
					title = core.ToString(t)
				}
				if c, ok := obj.Value("config").(*core.Object); ok {
					merge(config, c)
				}
				if mode := obj.Value("displayMode"); core.Truthy(mode) {
					gantt, ok := config.Value("gantt").(*core.Object)
					if !ok {
						gantt = core.NewObject()
						config.Set("gantt", gantt)
					}
					gantt.Set("displayMode", core.ToString(mode))
				}
			}
			for i := 0; i < front.length && i < len(text); i++ {
				if text[i] == '\n' {
					lineOffset++
				}
			}
			text = text[front.length:]
		}
	}

	text = directives(text, config)
	return &Preprocessed{Text: CleanupComments(text), Title: title, Config: config, WithComments: text, LineOffset: lineOffset}, nil
}

// dropColorSemicolons does what Mermaid's `text.replace(/keyword.*:\S*#.*;/g, drop the last
// character)` does: on a line where the keyword is followed by a colon, a color-like `#`, and
// later a semicolon, the last semicolon goes. Mermaid's regex backtracks without bound; this is
// one pass.
func dropColorSemicolons(text, keyword string) string {
	from := strings.Index(text, keyword)
	if from == -1 {
		return text
	}
	var out strings.Builder
	last := 0
	for from != -1 {
		lineEnd, termSize := len(text), 0
		for j := from; j < len(text); {
			r, size := js.RuneAt(text, j)
			if isLineTerminator(r) {
				lineEnd, termSize = j, size
				break
			}
			j += size
		}
		afterColon := false
		hash := -1
		for j := from + len(keyword); j < lineEnd; {
			r, size := js.RuneAt(text, j)
			if r == ':' {
				afterColon = true
			} else if r == '#' {
				if afterColon {
					hash = j
					break
				}
			} else if js.IsSpace(r) {
				afterColon = false
			}
			j += size
		}
		if hash != -1 {
			semi := strings.LastIndexByte(text[:lineEnd], ';')
			if semi > hash {
				out.WriteString(text[last:semi])
				last = semi + 1
			}
		}
		if lineEnd >= len(text) {
			from = -1
		} else if i := strings.Index(text[lineEnd+termSize:], keyword); i == -1 {
			from = -1
		} else {
			from = lineEnd + termSize + i
		}
	}
	if last == 0 {
		return text
	}
	out.WriteString(text[last:])
	return out.String()
}

// EncodeEntities hides `#name;` entity codes behind placeholder characters, as Mermaid does, so
// that `#` and `;` survive parsing.
func EncodeEntities(text string) string {
	if !strings.Contains(text, "#") {
		return text
	}
	text = dropColorSemicolons(dropColorSemicolons(text, "style"), "classDef")
	var b strings.Builder
	last := 0
	for i := 0; i < len(text); i++ {
		if text[i] != '#' {
			continue
		}
		e := wordEnd(text, i+1)
		if e == i+1 || e >= len(text) || text[e] != ';' {
			continue
		}
		inner := text[i+1 : e]
		numeric := true
		for k := 0; k < len(inner); k++ {
			if inner[k] < '0' || inner[k] > '9' {
				numeric = false
				break
			}
		}
		b.WriteString(text[last:i])
		if numeric {
			b.WriteString("ﬂ°°")
		} else {
			b.WriteString("ﬂ°")
		}
		b.WriteString(inner + "¶ß")
		last = e + 1
		i = e
	}
	if last == 0 {
		return text
	}
	b.WriteString(text[last:])
	return b.String()
}

package core

import (
	"errors"
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
)

// A small YAML reader covering what Mermaid feeds to js-yaml with the JSON schema:
// block and flow collections, quoted and plain scalars, and block scalars.

// incomplete is returned when a quoted scalar or flow collection runs past the text read so far.
type incomplete struct{ message string }

func (e *incomplete) Error() string { return e.message }

const yamlMaxDepth = 64

type yamlReader struct {
	lines []string
	i     int
}

func newYamlReader(src string) *yamlReader {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	return &yamlReader{lines: strings.Split(src, "\n")}
}

func (r *yamlReader) skipBlank() {
	for r.i < len(r.lines) {
		t := js.Trim(r.lines[r.i])
		if t != "" && t[0] != '#' && t != "---" {
			break
		}
		r.i++
	}
}

func (r *yamlReader) indent() (int, error) {
	line := r.lines[r.i]
	k := 0
	for k < len(line) && line[k] == ' ' {
		k++
	}
	if k < len(line) && line[k] == '\t' {
		return 0, errors.New("YAML: tabs are not allowed for indentation")
	}
	return k, nil
}

// leading returns the number of leading whitespace characters of s, which are one code unit each.
func leading(s string) int {
	n := 0
	for _, r := range s {
		if !js.IsSpace(r) {
			break
		}
		n++
	}
	return n
}

// skipChars drops the first n characters of s.
func skipChars(s string, n int) string {
	for i := range s {
		if n == 0 {
			return s[i:]
		}
		n--
	}
	return ""
}

func isRe(s string, re func(string) bool) bool { return re(s) }

func allDigits(s string, ok func(byte) bool) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !ok(s[i]) {
			return false
		}
	}
	return true
}

func dec(c byte) bool { return c >= '0' && c <= '9' }

func stripSign(s string) string {
	if s != "" && (s[0] == '-' || s[0] == '+') {
		return s[1:]
	}
	return s
}

// /^[-+]?(?:0|[1-9][0-9]*)$/
func reInt(s string) bool {
	s = stripSign(s)
	return s == "0" || (s != "" && s[0] >= '1' && s[0] <= '9' && allDigits(s, dec))
}

func prefixed(s, p string, ok func(byte) bool) bool {
	s = stripSign(s)
	return strings.HasPrefix(s, p) && allDigits(s[len(p):], ok)
}

// /^[-+]?(?:[0-9][0-9]*(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][-+]?[0-9]+)?$/
func reFloat(s string) bool {
	s = stripSign(s)
	i := 0
	for i < len(s) && dec(s[i]) {
		i++
	}
	if i > 0 {
		if i < len(s) && s[i] == '.' {
			i++
			for i < len(s) && dec(s[i]) {
				i++
			}
		}
	} else {
		if i >= len(s) || s[i] != '.' {
			return false
		}
		i++
		j := i
		for i < len(s) && dec(s[i]) {
			i++
		}
		if i == j {
			return false
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '-' || s[i] == '+') {
			i++
		}
		j := i
		for i < len(s) && dec(s[i]) {
			i++
		}
		if i == j {
			return false
		}
	}
	return i == len(s)
}

func isYamlBlank(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func at(s string, i int) byte {
	if i < 0 || i >= len(s) {
		return 0
	}
	return s[i]
}

// keyStart is /[\s#:,[\]{}&*!|>'"%@`]/ for the first character of a plain key.
func keyStart(r rune) bool {
	return js.IsSpace(r) || strings.ContainsRune("#:,[]{}&*!|>'\"%@`", r)
}

// splitKey splits `key: rest` at the start of a line. It returns the raw key and the offset
// where the value starts, or ok false.
func splitKey(line string) (key string, length int, ok bool) {
	if line == "" {
		return "", 0, false
	}
	first := line[0]
	var end, colon int
	if first == '"' || first == '\'' {
		i := 1
		for {
			if i >= len(line) {
				return "", 0, false
			}
			c := line[i]
			if first == '"' && c == '\\' {
				i += 2
			} else if c == first {
				if first == '"' || at(line, i+1) != '\'' {
					break
				}
				i += 2
			} else {
				i++
			}
		}
		end = i + 1
		colon = end
		for colon < len(line) && isYamlBlank(line[colon]) {
			colon++
		}
		if at(line, colon) != ':' {
			return "", 0, false
		}
	} else {
		r, _ := js.RuneAt(line, 0)
		if keyStart(r) {
			return "", 0, false
		}
		if first == '-' && (len(line) < 2 || isYamlBlank(line[1]) || line[1] == ':' || line[1] == '#') {
			return "", 0, false
		}
		colon = 1
		for colon < len(line) && line[colon] != ':' && line[colon] != '#' {
			colon++
		}
		if at(line, colon) != ':' {
			return "", 0, false
		}
		end = colon
		for end > 0 && isYamlBlank(line[end-1]) {
			end--
		}
	}
	length = colon + 1
	if length < len(line) && !isYamlBlank(line[length]) {
		return "", 0, false
	}
	for length < len(line) && isYamlBlank(line[length]) {
		length++
	}
	return line[:end], length, true
}

func yamlScalar(raw string) Value {
	s := js.Trim(raw)
	switch s {
	case "", "~", "null", "Null", "NULL":
		return nil
	case "true", "True", "TRUE":
		return true
	case "false", "False", "FALSE":
		return false
	}
	if reInt(s) || reFloat(s) {
		return js.ToNumber(s)
	}
	isHex := func(c byte) bool { return dec(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') }
	if prefixed(s, "0x", isHex) || prefixed(s, "0o", func(c byte) bool { return c >= '0' && c <= '7' }) ||
		prefixed(s, "0b", func(c byte) bool { return c == '0' || c == '1' }) {
		neg := s[0] == '-'
		v := js.ToNumber(stripSign(s))
		if neg {
			return -v
		}
		return v
	}
	switch s {
	case ".inf", ".Inf", ".INF", "+.inf", "+.Inf", "+.INF":
		return math.Inf(1)
	case "-.inf", "-.Inf", "-.INF":
		return math.Inf(-1)
	case ".nan", ".NaN", ".NAN":
		return math.NaN()
	}
	return s
}

var yamlEscapes = map[uint16]string{
	'0': "\x00", 'a': "\x07", 'b': "\b", 't': "\t", 'n': "\n", 'v': "\v", 'f': "\f", 'r': "\r",
	'e': "\x1b", ' ': " ", '"': "\"", '/': "/", '\\': "\\", 'N': "\u0085", '_': " ",
	'L': " ", 'P': " ",
}

// fold folds the line breaks of a scalar: blanks around a break go, one break becomes a space,
// and each further break stays.
func fold(text string) string {
	if !strings.Contains(text, "\n") {
		return text
	}
	lines := strings.Split(text, "\n")
	last := len(lines) - 1
	for i, line := range lines {
		from, to := 0, len(line)
		if i > 0 {
			for from < to && (line[from] == ' ' || line[from] == '\t') {
				from++
			}
		}
		if i < last {
			for to > from && (line[to-1] == ' ' || line[to-1] == '\t') {
				to--
			}
		}
		lines[i] = line[from:to]
	}
	joined := strings.Join(lines, "\n")
	var b strings.Builder
	for i := 0; i < len(joined); {
		if joined[i] != '\n' {
			b.WriteByte(joined[i])
			i++
			continue
		}
		j := i
		for j < len(joined) && joined[j] == '\n' {
			j++
		}
		if j-i == 1 {
			b.WriteByte(' ')
		} else {
			b.WriteString(strings.Repeat("\n", j-i-1))
		}
		i = j
	}
	return b.String()
}

// dropEscapedBreaks is body.replace(/\\\n[ \t]*/g, ”).
func dropEscapedBreaks(s string) string {
	if !strings.Contains(s, "\\\n") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\\' && at(s, i+1) == '\n' {
			i += 2
			for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func unquoteDouble(body string) (string, error) {
	s := js.Units(fold(dropEscapedBreaks(body)))
	var out []uint16
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch != '\\' {
			out = append(out, ch)
			continue
		}
		i++
		var e uint16
		if i < len(s) {
			e = s[i]
		}
		if e == 'x' || e == 'u' || e == 'U' {
			n := 2
			if e == 'u' {
				n = 4
			} else if e == 'U' {
				n = 8
			}
			end := i + 1 + n
			if end > len(s) {
				end = len(s)
			}
			start := i + 1
			if start > len(s) {
				start = len(s)
			}
			cp := js.ParseInt(js.FromUnits(s[start:end]), 16)
			if !(cp >= 0 && cp <= 0x10ffff && cp == math.Trunc(cp)) {
				return "", errors.New("Invalid code point " + js.NumberToString(cp))
			}
			out = append(out, js.Units(string(rune(cp)))...)
			i += n
		} else if rep, ok := yamlEscapes[e]; ok && i < len(s) {
			out = append(out, js.Units(rep)...)
		} else {
			return "", errors.New("YAML: unknown escape sequence")
		}
	}
	return js.FromUnits(out), nil
}

func yamlSkip(s string, pos int, commas bool) int {
	for pos < len(s) {
		c := s[pos]
		if c == ' ' || c == '\n' || c == '\t' || c == '\r' || (commas && c == ',') {
			pos++
		} else if c == '#' && (pos == 0 || spaceBefore(s, pos)) {
			nl := strings.IndexByte(s[pos:], '\n')
			if nl == -1 {
				pos = len(s)
			} else {
				pos += nl
			}
		} else {
			break
		}
	}
	return pos
}

func spaceBefore(s string, pos int) bool {
	r, _ := js.RuneBefore(s, pos)
	return js.IsSpace(r)
}

func entryStart(ch byte, separated bool) error {
	if !separated {
		return errors.New("YAML: missed comma between flow collection entries")
	}
	if ch == ',' {
		return errors.New("YAML: expected the node content, but found ','")
	}
	return nil
}

// flow parses a flow value starting at s[pos]; it returns the value and the index after it.
func flow(s string, pos int, stops string, depth int) (Value, int, error) {
	if depth > yamlMaxDepth {
		return nil, 0, errors.New("YAML: nesting is too deep")
	}
	pos = yamlSkip(s, pos, false)
	ch := at(s, pos)
	if pos < len(s) && ch == '{' {
		out := NewObject()
		pos++
		separated := true
		for {
			pos = yamlSkip(s, pos, false)
			if pos >= len(s) {
				return nil, 0, &incomplete{"YAML: unexpected end of the stream within a flow collection"}
			}
			if s[pos] == '}' {
				return out, pos + 1, nil
			}
			if err := entryStart(s[pos], separated); err != nil {
				return nil, 0, err
			}
			key, next, err := flow(s, pos, ":,}", depth+1)
			if err != nil {
				return nil, 0, err
			}
			pos = yamlSkip(s, next, false)
			var value Value
			if at(s, pos) == ':' && pos < len(s) {
				value, pos, err = flow(s, pos+1, ",}", depth+1)
				if err != nil {
					return nil, 0, err
				}
			}
			out.Set(ToString(key), value)
			pos = yamlSkip(s, pos, false)
			separated = pos < len(s) && s[pos] == ','
			if separated {
				pos++
			}
		}
	}
	if pos < len(s) && ch == '[' {
		out := []Value{}
		pos++
		separated := true
		for {
			pos = yamlSkip(s, pos, false)
			if pos >= len(s) {
				return nil, 0, &incomplete{"YAML: unexpected end of the stream within a flow collection"}
			}
			if s[pos] == ']' {
				return out, pos + 1, nil
			}
			if err := entryStart(s[pos], separated); err != nil {
				return nil, 0, err
			}
			value, next, err := flow(s, pos, ",]", depth+1)
			if err != nil {
				return nil, 0, err
			}
			out = append(out, value)
			pos = yamlSkip(s, next, false)
			separated = pos < len(s) && s[pos] == ','
			if separated {
				pos++
			}
		}
	}
	if pos < len(s) && ch == '"' {
		end := pos + 1
		for end < len(s) && s[end] != '"' {
			if s[end] == '\\' {
				end += 2
			} else {
				end++
			}
		}
		if end >= len(s) {
			return nil, 0, &incomplete{"YAML: unexpected end of the stream within a double quoted scalar"}
		}
		v, err := unquoteDouble(s[pos+1 : end])
		if err != nil {
			return nil, 0, err
		}
		return v, end + 1, nil
	}
	if pos < len(s) && ch == '\'' {
		end := pos + 1
		for end < len(s) {
			if s[end] == '\'' {
				if at(s, end+1) != '\'' || end+1 >= len(s) {
					break
				}
				end++
			}
			end++
		}
		if end >= len(s) {
			return nil, 0, &incomplete{"YAML: unexpected end of the stream within a single quoted scalar"}
		}
		return strings.ReplaceAll(fold(s[pos+1:end]), "''", "'"), end + 1, nil
	}
	end := pos
	for end < len(s) {
		c := s[end]
		if strings.IndexByte(stops, c) >= 0 && (c != ':' || end+1 >= len(s) || flowBreak(s, end+1)) {
			break
		}
		if c == '#' && end > pos && spaceBefore(s, end) {
			nl := strings.IndexByte(s[end:], '\n')
			next := len(s)
			if nl != -1 {
				next = end + nl
			}
			return yamlScalar(fold(js.Trim(s[pos:end]))), next, nil
		}
		end++
	}
	return yamlScalar(fold(js.Trim(s[pos:end]))), end, nil
}

// flowBreak is /[\s,[\]{}]/ for the character at i.
func flowBreak(s string, i int) bool {
	r, _ := js.RuneAt(s, i)
	return js.IsSpace(r) || r == ',' || r == '[' || r == ']' || r == '{' || r == '}'
}

func stripComment(s string) string {
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == '\\' && quote == '"' {
				i++
			} else if c == quote {
				quote = 0
			}
		} else if c == '"' || c == '\'' {
			quote = c
		} else if c == '#' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t') {
			return s[:i]
		}
	}
	return s
}

func blockScalar(r *yamlReader, header string, parentIndent int) string {
	folded := header[0] == '>'
	chomp := ""
	if strings.Contains(header, "-") {
		chomp = "-"
	} else if strings.Contains(header, "+") {
		chomp = "+"
	}
	indent := -1
	if i := strings.IndexAny(header, "123456789"); i >= 0 {
		indent = parentIndent + int(header[i]-'0')
	}
	var out []string
	for r.i < len(r.lines) {
		line := r.lines[r.i]
		if js.Trim(line) == "" {
			out = append(out, "")
			r.i++
			continue
		}
		k := leading(line)
		if indent == -1 {
			if k <= parentIndent {
				break
			}
			indent = k
		}
		if k < indent {
			break
		}
		out = append(out, skipChars(line, indent))
		r.i++
	}
	trailing := 0
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
		trailing++
	}
	var text string
	if folded {
		var b strings.Builder
		for i, cur := range out {
			if i > 0 {
				prev := out[i-1]
				if cur == "" {
					b.WriteByte('\n')
				} else if prev != "" {
					if prev[0] == ' ' || prev[0] == '\t' || cur[0] == ' ' || cur[0] == '\t' {
						b.WriteByte('\n')
					} else {
						b.WriteByte(' ')
					}
				}
			}
			b.WriteString(cur)
		}
		text = b.String()
	} else {
		text = strings.Join(out, "\n")
	}
	if len(out) == 0 {
		if chomp == "+" {
			return strings.Repeat("\n", trailing)
		}
		return ""
	}
	switch chomp {
	case "-":
		return text
	case "+":
		return text + strings.Repeat("\n", trailing+1)
	}
	return text + "\n"
}

// blockHeader is /^[|>][-+0-9]*\s*(?:#.*)?$/.
func blockHeader(rest string) bool {
	i := 1
	for i < len(rest) && (rest[i] == '-' || rest[i] == '+' || dec(rest[i])) {
		i++
	}
	for i < len(rest) {
		r, size := js.RuneAt(rest, i)
		if !js.IsSpace(r) {
			break
		}
		i += size
	}
	if i == len(rest) {
		return true
	}
	if rest[i] != '#' {
		return false
	}
	for _, r := range rest[i+1:] {
		if js.IsLineEnd(r) {
			return false
		}
	}
	return true
}

func inlineValue(r *yamlReader, rest string, indent int) (Value, error) {
	first := at(rest, 0)
	if rest != "" && (first == '|' || first == '>') && blockHeader(rest) {
		return blockScalar(r, rest, indent), nil
	}
	if rest != "" && (first == '{' || first == '[' || first == '"' || first == '\'') {
		// The value may continue on later lines. Read them in growing batches until it closes.
		start := r.i
		text := rest
		taken := 0
		batch := 1
		for {
			value, end, err := flow(text, 0, "", 0)
			if err == nil {
				used := 0
				for k := 0; k < end && k < len(text); k++ {
					if text[k] == '\n' {
						used++
					}
				}
				lineEnd := strings.IndexByte(text[end:], '\n')
				tail := text[end:]
				if lineEnd != -1 {
					tail = text[end : end+lineEnd]
				}
				if js.Trim(stripComment(tail)) != "" {
					return nil, errors.New("YAML: unexpected content after a value")
				}
				r.i = start + used
				return value, nil
			}
			inc, ok := err.(*incomplete)
			if !ok {
				return nil, err
			}
			if start+taken >= len(r.lines) {
				return nil, errors.New(inc.message)
			}
			stop := start + taken + batch
			if stop > len(r.lines) {
				stop = len(r.lines)
			}
			more := r.lines[start+taken : stop]
			text += "\n" + strings.Join(more, "\n")
			taken += len(more)
			batch *= 2
		}
	}
	text := js.Trim(stripComment(rest))
	for r.i < len(r.lines) {
		line := r.lines[r.i]
		if js.Trim(line) == "" {
			j := r.i
			for j < len(r.lines) && js.Trim(r.lines[j]) == "" {
				j++
			}
			if j >= len(r.lines) || leading(r.lines[j]) <= indent {
				break
			}
			text += strings.Repeat("\n", j-r.i)
			r.i = j
			continue
		}
		if leading(line) <= indent {
			break
		}
		more := js.Trim(stripComment(line))
		if !strings.HasSuffix(text, "\n") {
			text += " "
		}
		text += more
		r.i++
	}
	return yamlScalar(text), nil
}

func isSeqLine(line string) bool { return line == "-" || strings.HasPrefix(line, "- ") }

func block(r *yamlReader, minIndent int, depth int) (Value, error) {
	if depth > yamlMaxDepth {
		return nil, errors.New("YAML: nesting is too deep")
	}
	r.skipBlank()
	if r.i >= len(r.lines) {
		return nil, nil
	}
	indent, err := r.indent()
	if err != nil {
		return nil, err
	}
	if indent < minIndent {
		return nil, nil
	}
	line := r.lines[r.i][indent:]

	if isSeqLine(line) {
		out := []Value{}
		for r.i < len(r.lines) {
			r.skipBlank()
			if r.i >= len(r.lines) {
				break
			}
			k, err := r.indent()
			if err != nil {
				return nil, err
			}
			if k != indent {
				break
			}
			cur := r.lines[r.i][indent:]
			if !isSeqLine(cur) {
				break
			}
			rest := cur[1:]
			pad := leading(rest) + 1
			body := js.Trim(stripComment(rest))
			var v Value
			if body == "" {
				r.i++
				v, err = block(r, indent+1, depth+1)
			} else if _, _, ok := splitKey(body); ok {
				r.lines[r.i] = strings.Repeat(" ", indent+pad) + js.TrimStart(rest)
				v, err = block(r, indent+pad, depth+1)
			} else {
				r.i++
				v, err = inlineValue(r, js.Trim(rest), indent)
			}
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}

	if _, _, ok := splitKey(line); !ok {
		r.i++
		return inlineValue(r, js.Trim(line), indent-1)
	}

	out := NewObject()
	for r.i < len(r.lines) {
		r.skipBlank()
		if r.i >= len(r.lines) {
			break
		}
		k, err := r.indent()
		if err != nil {
			return nil, err
		}
		if k < indent {
			break
		}
		if k > indent {
			return nil, errors.New("YAML: bad indentation of a mapping entry")
		}
		cur := r.lines[r.i][indent:]
		rawKey, length, ok := splitKey(cur)
		if !ok {
			return nil, errors.New("YAML: can not read a block mapping entry")
		}
		keyValue, _, err := flow(rawKey, 0, "", 0)
		if err != nil {
			if inc, ok := err.(*incomplete); ok {
				return nil, errors.New(inc.message)
			}
			return nil, err
		}
		key := ToString(keyValue)
		rest := cur[length:]
		r.i++
		if js.Trim(stripComment(rest)) == "" {
			r.skipBlank()
			if r.i < len(r.lines) {
				next, err := r.indent()
				if err != nil {
					return nil, err
				}
				nextLine := r.lines[r.i][next:]
				seq := isSeqLine(nextLine)
				if next > indent || (seq && next == indent) {
					min := indent + 1
					if seq {
						min = indent
					}
					v, err := block(r, min, depth+1)
					if err != nil {
						return nil, err
					}
					out.Set(key, v)
				} else {
					out.Set(key, nil)
				}
			} else {
				out.Set(key, nil)
			}
		} else {
			v, err := inlineValue(r, js.Trim(rest), indent)
			if err != nil {
				return nil, err
			}
			out.Set(key, v)
		}
	}
	return out, nil
}

// ParseYaml reads the YAML of front matter.
func ParseYaml(src string) (Value, error) {
	trimmed := js.Trim(src)
	if trimmed != "" && (trimmed[0] == '{' || trimmed[0] == '[') {
		value, end, err := flow(trimmed, 0, "", 0)
		if err != nil {
			if inc, ok := err.(*incomplete); ok {
				return nil, errors.New(inc.message)
			}
			return nil, err
		}
		if yamlSkip(trimmed, end, false) == len(trimmed) {
			return value, nil
		}
	}
	r := newYamlReader(src)
	value, err := block(r, 0, 0)
	if err != nil {
		if inc, ok := err.(*incomplete); ok {
			return nil, errors.New(inc.message)
		}
		return nil, err
	}
	r.skipBlank()
	if r.i < len(r.lines) && js.Trim(r.lines[r.i]) != "..." {
		return nil, errors.New("YAML: end of the stream or a document separator is expected")
	}
	return value, nil
}

// Package common holds what several diagram types share.
package common

import (
	"regexp"
	"strings"
	"sync"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

// Tokenizer for the diagram types Mermaid defines with Langium grammars. Token types are tried
// in order and the first match wins, as in Chevrotain.

// Scanner returns the end of a match at byte offset at, or -1.
type Scanner func(src string, at int) int

// TokenType is a terminal of a Langium grammar.
type TokenType struct {
	Name string
	// A literal, or else Match, which stands for the sticky regular expression of the TypeScript.
	Literal string
	Match   Scanner
	Hidden  bool
	// Start of a construct that needs a closer. When this matches but the whole pattern does
	// not, no closer exists in the rest of the text, so the pattern is not tried again.
	Opener Scanner
	// Every ASCII character a match can start with, when that is known. The type is then tried
	// only at those characters and at characters outside ASCII. Empty means unknown.
	First string
	// Chevrotain's LONGER_ALT. When this type matches, these are tried at the same place, and
	// the first one that matches more text becomes the token.
	Longer []*TokenType
	// Makes a matcher for a pattern that could rescan the text. It is called once per text, so
	// the matcher may remember what it has scanned.
	NewScanner func() Scanner
}

// Re makes a Scanner from a Go regular expression, matched at the position as a sticky one is.
// Go's `\s` is narrower than JavaScript's, `.` matches `\r`, ` ` and ` `, and a `\b`
// at the start of the pattern does not see the character before the position; write patterns
// with that in mind.
func Re(pattern string) Scanner {
	re := regexp.MustCompile(`^(?:` + pattern + `)`)
	return func(src string, at int) int {
		loc := re.FindStringIndex(src[at:])
		if loc == nil {
			return -1
		}
		return at + loc[1]
	}
}

// Tokens are the tokens of a text: kinds, and byte offsets of their starts and ends. The last
// token is kind -1 at the end of the text.
type Tokens struct {
	Kinds  []int
	Starts []int
	Ends   []int
	Src    string
}

type table struct {
	starting [128][]int
	every    []int
	longer   [][]int
}

type tableKey struct {
	first **TokenType
	n     int
}

var tables sync.Map

func prepare(types []*TokenType) *table {
	key := tableKey{&types[0], len(types)}
	if t, ok := tables.Load(key); ok {
		return t.(*table)
	}
	t := &table{}
	for k := range types {
		t.every = append(t.every, k)
	}
	for c := 0; c < 128; c++ {
		ch := byte(c)
		for k, typ := range types {
			if typ.Literal != "" {
				if typ.Literal[0] == ch {
					t.starting[c] = append(t.starting[c], k)
				}
			} else if typ.First == "" || strings.IndexByte(typ.First, ch) >= 0 {
				t.starting[c] = append(t.starting[c], k)
			}
		}
	}
	t.longer = make([][]int, len(types))
	for k, typ := range types {
		for _, alt := range typ.Longer {
			for j, other := range types {
				if other == alt {
					t.longer[k] = append(t.longer[k], j)
					break
				}
			}
		}
	}
	tables.Store(key, t)
	return t
}

// Tokenize splits src into tokens of the given types. It panics with a lexical error where no
// type matches.
func Tokenize(src string, types []*TokenType, diagram string) Tokens {
	tab := prepare(types)
	var kinds, starts, ends []int
	dead := make([]bool, len(types))
	scanners := make([]Scanner, len(types))
	n := len(src)

	match := func(k, p int) int {
		if dead[k] {
			return -1
		}
		typ := types[k]
		if typ.NewScanner != nil {
			if scanners[k] == nil {
				scanners[k] = typ.NewScanner()
			}
			return scanners[k](src, p)
		}
		if typ.Literal != "" {
			if strings.HasPrefix(src[p:], typ.Literal) {
				return p + len(typ.Literal)
			}
			return -1
		}
		if end := typ.Match(src, p); end >= 0 {
			return end
		}
		if typ.Opener != nil && typ.Opener(src, p) >= 0 {
			dead[k] = true
		}
		return -1
	}

	p := 0
scan:
	for p < n {
		c := src[p]
		candidates := tab.every
		if c < 128 {
			candidates = tab.starting[c]
		}
		for _, k := range candidates {
			end := match(k, p)
			if end <= p {
				continue
			}
			kind := k
			for _, alt := range tab.longer[k] {
				if altEnd := match(alt, p); altEnd > end {
					kind = alt
					end = altEnd
					break
				}
			}
			if !types[kind].Hidden {
				kinds = append(kinds, kind)
				starts = append(starts, p)
				ends = append(ends, end)
			}
			p = end
			continue scan
		}
		panic(core.SyntaxError(diagram, src, p, "", true))
	}
	kinds = append(kinds, -1)
	starts = append(starts, n)
	ends = append(ends, n)
	return Tokens{Kinds: kinds, Starts: starts, Ends: ends, Src: src}
}

func skipBlanks(src string, p int) int {
	for p < len(src) && (src[p] == ' ' || src[p] == '\t') {
		p++
	}
	return p
}

func skipJSSpace(src string, p int) int {
	for p < len(src) {
		r, size := js.RuneAt(src, p)
		if !js.IsSpace(r) {
			break
		}
		p += size
	}
	return p
}

// lineStop returns the first \n or \r at or after p, or len(src).
func lineStop(src string, p int) int {
	if i := strings.IndexAny(src[p:], "\n\r"); i >= 0 {
		return p + i
	}
	return len(src)
}

// restOfLine is `[^\n\r]*?(?=%%)|[^\n\r]*` at p: up to a comment on the line, else to its end.
func restOfLine(src string, p int) int {
	stop := lineStop(src, p)
	if i := strings.Index(src[p:stop], "%%"); i >= 0 {
		return p + i
	}
	return stop
}

func matchAccDescr(src string, at int) int {
	p := skipBlanks(src, at)
	if !strings.HasPrefix(src[p:], "accDescr") {
		return -1
	}
	p += len("accDescr")
	if q := skipBlanks(src, p); q < len(src) && src[q] == ':' {
		return restOfLine(src, q+1)
	}
	q := skipJSSpace(src, p)
	if q >= len(src) || src[q] != '{' {
		return -1
	}
	close := strings.IndexByte(src[q+1:], '}')
	if close < 0 {
		return -1
	}
	return q + 1 + close + 1
}

// ACC_DESCR is /[\t ]*accDescr(?:[\t ]*:([^\n\r]*?(?=%%)|[^\n\r]*)|\s*{([^}]*)})/y.
var ACC_DESCR = &TokenType{Name: "ACC_DESCR", Match: matchAccDescr, First: "\t a"}

// ACC_TITLE is /[\t ]*accTitle[\t ]*:(?:[^\n\r]*?(?=%%)|[^\n\r]*)/y.
var ACC_TITLE = &TokenType{Name: "ACC_TITLE", First: "\t a", Match: func(src string, at int) int {
	p := skipBlanks(src, at)
	if !strings.HasPrefix(src[p:], "accTitle") {
		return -1
	}
	p = skipBlanks(src, p+len("accTitle"))
	if p >= len(src) || src[p] != ':' {
		return -1
	}
	return restOfLine(src, p+1)
}}

// TITLE is /[\t ]*title(?:[\t ][^\n\r]*?(?=%%)|[\t ][^\n\r]*|)/y.
var TITLE = &TokenType{Name: "TITLE", First: "\t t", Match: func(src string, at int) int {
	p := skipBlanks(src, at)
	if !strings.HasPrefix(src[p:], "title") {
		return -1
	}
	p += len("title")
	if p < len(src) && (src[p] == ' ' || src[p] == '\t') {
		return restOfLine(src, p+1)
	}
	return p
}}

// QuotedEnd matches /"([^"\\]|\\.)*"|'([^'\\]|\\.)*'/ at at.
func QuotedEnd(src string, at int) int {
	if at >= len(src) || (src[at] != '"' && src[at] != '\'') {
		return -1
	}
	q := src[at]
	for p := at + 1; p < len(src); {
		c := src[p]
		if c == q {
			return p + 1
		}
		if c == '\\' {
			r, size := js.RuneAt(src, p+1)
			if p+1 >= len(src) || js.IsLineEnd(r) {
				return -1
			}
			p += 1 + size
			continue
		}
		p++
	}
	return -1
}

var STRING = &TokenType{Name: "STRING", Match: QuotedEnd, First: `"'`}

// NewlineEnd matches /\r?\n/ at at.
func NewlineEnd(src string, at int) int {
	if strings.HasPrefix(src[at:], "\r\n") {
		return at + 2
	}
	if at < len(src) && src[at] == '\n' {
		return at + 1
	}
	return -1
}

var NEWLINE = &TokenType{Name: "NEWLINE", Match: NewlineEnd, First: "\r\n"}

var WHITESPACE = &TokenType{Name: "WHITESPACE", Hidden: true, First: "\t ", Match: func(src string, at int) int {
	p := skipBlanks(src, at)
	if p == at {
		return -1
	}
	return p
}}

// notNonSpace is (?!\S) at p.
func notNonSpace(src string, p int) bool {
	r, _ := js.RuneAt(src, p)
	return p >= len(src) || js.IsSpace(r)
}

// fenceTail is (?:\r?\n|(?!\S)) at p, returning its end or -1.
func fenceTail(src string, p int) int {
	if e := NewlineEnd(src, p); e >= 0 {
		return e
	}
	if notNonSpace(src, p) {
		return p
	}
	return -1
}

func yamlOpener(src string, at int) int {
	if !strings.HasPrefix(src[at:], "---") {
		return -1
	}
	return NewlineEnd(src, skipBlanks(src, at+3))
}

// YAML is /---[\t ]*\r?\n(?:[\S\s]*?\r?\n)?---(?:\r?\n|(?!\S))/y.
var YAML = &TokenType{Name: "YAML", Hidden: true, First: "-", Opener: yamlOpener, Match: func(src string, at int) int {
	b := yamlOpener(src, at)
	if b < 0 {
		return -1
	}
	// The optional group comes first: the earliest line break after which a fence closes.
	for q := b; q < len(src); q++ {
		if src[q] != '\n' && src[q] != '\r' {
			continue
		}
		e := NewlineEnd(src, q)
		if e < 0 || !strings.HasPrefix(src[e:], "---") {
			continue
		}
		if end := fenceTail(src, e+3); end >= 0 {
			return end
		}
	}
	if strings.HasPrefix(src[b:], "---") {
		return fenceTail(src, b+3)
	}
	return -1
}}

func directiveOpener(src string, at int) int {
	p := skipBlanks(src, at)
	if !strings.HasPrefix(src[p:], "%%{") {
		return -1
	}
	return p + 3
}

// DIRECTIVE is /[\t ]*%%{[\S\s]*?}%%(?:\r?\n|(?!\S))/y.
var DIRECTIVE = &TokenType{Name: "DIRECTIVE", Hidden: true, First: "\t %", Opener: directiveOpener, Match: func(src string, at int) int {
	p := directiveOpener(src, at)
	if p < 0 {
		return -1
	}
	for {
		i := strings.Index(src[p:], "}%%")
		if i < 0 {
			return -1
		}
		q := p + i + 3
		if end := fenceTail(src, q); end >= 0 {
			return end
		}
		p += i + 1
	}
}}

// SINGLE_LINE_COMMENT is /[\t ]*%%[^\n\r]*/y.
var SINGLE_LINE_COMMENT = &TokenType{Name: "SINGLE_LINE_COMMENT", First: "\t %", Hidden: true, Match: func(src string, at int) int {
	p := skipBlanks(src, at)
	if !strings.HasPrefix(src[p:], "%%") {
		return -1
	}
	return lineStop(src, p+2)
}}

// KeywordEnd matches word followed by a comment, whitespace or the end of input.
func KeywordEnd(src string, at int, word string) int {
	if !strings.HasPrefix(src[at:], word) {
		return -1
	}
	p := at + len(word)
	if strings.HasPrefix(src[p:], "%%") || notNonSpace(src, p) {
		return p
	}
	return -1
}

// Keyword is a token for a word. Mermaid ends a keyword only at whitespace, a comment, or the
// end of input.
func Keyword(word string) *TokenType {
	return &TokenType{Name: word, First: word[:1], Match: func(src string, at int) int { return KeywordEnd(src, at, word) }}
}

// single is text.trim().replace(/[\t ]{2,}/gm, ' ').
func single(text string) string { return CollapseBlanks(js.Trim(text)) }

// CollapseBlanks is .replace(/[\t ]{2,}/g, ' ').
func CollapseBlanks(s string) string {
	if !strings.Contains(s, "  ") && !strings.Contains(s, "\t") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == ' ' || s[i] == '\t' {
			j := i
			for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
				j++
			}
			if j-i >= 2 {
				b.WriteByte(' ')
			} else {
				b.WriteByte(s[i])
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// TitleValue reads the text of a TITLE token: /title([\t ][^\n\r]*|)/.
func TitleValue(text string) string {
	i := strings.Index(text, "title")
	rest := text[i+len("title"):]
	if rest != "" && (rest[0] == ' ' || rest[0] == '\t') {
		return single(rest[:lineStop(rest, 0)])
	}
	return single("")
}

// AccTitleValue reads the text of an ACC_TITLE token: /accTitle[\t ]*:([^\n\r]*)/.
func AccTitleValue(text string) string {
	i := strings.Index(text, "accTitle")
	p := skipBlanks(text, i+len("accTitle"))
	return single(text[p+1 : lineStop(text, p+1)])
}

func isLineTerm(s string, i int) bool {
	r, _ := js.RuneAt(s, i)
	return i >= len(s) || js.IsLineEnd(r)
}

func lineStartAt(s string, i int) bool {
	if i == 0 {
		return true
	}
	r, _ := js.RuneBefore(s, i)
	return js.IsLineEnd(r)
}

// trimLineStarts removes /^\s*/gm.
func trimLineStarts(s string) string {
	var b strings.Builder
	last := 0
	for i := 0; i <= len(s); {
		if lineStartAt(s, i) {
			j := skipJSSpace(s, i)
			if j > i {
				b.WriteString(s[last:i])
				last = j
				i = j
				continue
			}
		}
		if i == len(s) {
			break
		}
		_, size := js.RuneAt(s, i)
		i += size
	}
	b.WriteString(s[last:])
	return b.String()
}

// trimLineEnds removes /\s+$/gm: from each run of whitespace, the part up to the last
// line end in it, as backtracking finds it.
func trimLineEnds(s string) string {
	var b strings.Builder
	last := 0
	for i := 0; i < len(s); {
		r, size := js.RuneAt(s, i)
		if !js.IsSpace(r) {
			i += size
			continue
		}
		j := skipJSSpace(s, i)
		k := -1
		for e := j; e > i; {
			if isLineTerm(s, e) {
				k = e
				break
			}
			_, sz := js.RuneBefore(s, e)
			e -= sz
		}
		if k < 0 {
			i += size
			continue
		}
		b.WriteString(s[last:i])
		last = k
		i = k
	}
	b.WriteString(s[last:])
	return b.String()
}

// collapseBreaks is .replace(/[\n\r]{2,}/g, '\n').
func collapseBreaks(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\n' || s[i] == '\r' {
			j := i
			for j < len(s) && (s[j] == '\n' || s[j] == '\r') {
				j++
			}
			if j-i >= 2 {
				b.WriteByte('\n')
			} else {
				b.WriteByte(s[i])
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// AccDescrValue reads the text of an ACC_DESCR token.
func AccDescrValue(text string) string {
	i := strings.Index(text, "accDescr")
	p := i + len("accDescr")
	if q := skipBlanks(text, p); q < len(text) && text[q] == ':' {
		return single(text[q+1 : lineStop(text, q+1)])
	}
	q := skipJSSpace(text, p)
	body := text[q+1:]
	body = body[:strings.IndexByte(body, '}')]
	return collapseBreaks(CollapseBlanks(trimLineEnds(trimLineStarts(body))))
}

var stringEscapes = map[rune]string{'b': "\b", 'f': "\f", 'n': "\n", 'r': "\r", 't': "\t", 'v': "\v", '0': "\x00"}

// StringValue reads a STRING token.
func StringValue(text string) string {
	if !strings.Contains(text, "\\") {
		return text[1 : len(text)-1]
	}
	var b strings.Builder
	rs := []rune(text)
	for i := 1; i < len(rs)-1; i++ {
		if rs[i] != '\\' {
			b.WriteRune(rs[i])
			continue
		}
		i++
		if e, ok := stringEscapes[rs[i]]; ok {
			b.WriteString(e)
		} else {
			b.WriteRune(rs[i])
		}
	}
	return b.String()
}

// Reader reads tokens for a hand-written parser, with Chevrotain-style messages on failure.
type Reader struct {
	I       int
	Tokens  Tokens
	Types   []*TokenType
	Diagram string
}

func NewReader(tokens Tokens, types []*TokenType, diagram string) *Reader {
	return &Reader{Tokens: tokens, Types: types, Diagram: diagram}
}

// Kind is the kind of the current token.
func (r *Reader) Kind() int { return r.Tokens.Kinds[r.I] }

// KindAt is the kind of token i, or -1 past the end.
func (r *Reader) KindAt(i int) int {
	if i >= len(r.Tokens.Kinds) {
		return -1
	}
	return r.Tokens.Kinds[i]
}

// Text is the text of token i.
func (r *Reader) Text(i int) string { return r.Tokens.Src[r.Tokens.Starts[i]:r.Tokens.Ends[i]] }

// Take returns the text of the current token and moves past it.
func (r *Reader) Take() string {
	t := r.Text(r.I)
	r.I++
	return t
}

func (r *Reader) Accept(kind int) bool {
	if r.Tokens.Kinds[r.I] != kind {
		return false
	}
	r.I++
	return true
}

func (r *Reader) Expect(kind int) string {
	if r.Tokens.Kinds[r.I] != kind {
		r.Fail("token of type '" + r.Types[kind].Name + "'")
	}
	return r.Take()
}

// Fail panics with a syntax error at the current token.
func (r *Reader) Fail(expected string) {
	found := "end of input"
	if r.Kind() != -1 {
		found = "`" + r.Text(r.I) + "`"
	}
	panic(core.SyntaxError(r.Diagram, r.Tokens.Src, r.Tokens.Starts[r.I], "Expecting "+expected+" but found "+found+".", false))
}

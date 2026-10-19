package treemap

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

const (
	tAccDescr = iota
	tAccTitle
	tTitle
	tKeyword
	tClassDef
	tStyleSeparator
	tSeparator
	tComma
	tIndentation
	tWhitespace
	tComment
	tNewline
	tID
	tNumber
	tString
)

func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' }
func isWordByte(c byte) bool {
	return isLetter(c) || (c >= '0' && c <= '9')
}

func skipJSSpace(s string, p int) int {
	for p < len(s) {
		r, size := js.RuneAt(s, p)
		if !js.IsSpace(r) {
			break
		}
		p += size
	}
	return p
}

// classDef matches /classDef\s+([a-zA-Z_][a-zA-Z0-9_]+)(?:\s+([^;\r\n]*))?(?:;)?/ at p and returns
// its end, the class name and the style text.
func classDef(src string, p int) (end int, name, style string) {
	if !strings.HasPrefix(src[p:], "classDef") {
		return -1, "", ""
	}
	q := skipJSSpace(src, p+len("classDef"))
	if q == p+len("classDef") || q >= len(src) || !isLetter(src[q]) {
		return -1, "", ""
	}
	e := q + 1
	for e < len(src) && isWordByte(src[e]) {
		e++
	}
	if e-q < 2 {
		return -1, "", ""
	}
	name = src[q:e]
	if s := skipJSSpace(src, e); s > e {
		t := s
		for t < len(src) && src[t] != ';' && src[t] != '\r' && src[t] != '\n' {
			t++
		}
		style = src[s:t]
		e = t
	}
	if e < len(src) && src[e] == ';' {
		e++
	}
	return e, name, style
}

func run(ok func(byte) bool) common.Scanner {
	return func(src string, at int) int {
		e := at
		for e < len(src) && ok(src[e]) {
			e++
		}
		if e == at {
			return -1
		}
		return e
	}
}

func blank(c byte) bool { return c == ' ' || c == '\t' }

// Mermaid's treemap grammar has its own terminals. Every run of spaces or tabs is an INDENTATION
// token, since that comes before WS, and line breaks are hidden, so rows are told apart by their
// tokens alone and the indentation of a row is whatever whitespace precedes it.
var tokens = []*common.TokenType{
	common.ACC_DESCR,
	common.ACC_TITLE,
	common.TITLE,
	{Name: "TREEMAP_KEYWORD", Match: func(src string, at int) int {
		if strings.HasPrefix(src[at:], "treemap-beta") {
			return at + len("treemap-beta")
		}
		if strings.HasPrefix(src[at:], "treemap") {
			return at + len("treemap")
		}
		return -1
	}},
	{Name: "CLASS_DEF", Match: func(src string, at int) int { e, _, _ := classDef(src, at); return e }},
	{Name: "STYLE_SEPARATOR", Literal: ":::"},
	{Name: "SEPARATOR", Literal: ":"},
	{Name: "COMMA", Literal: ","},
	{Name: "INDENTATION", Match: run(blank)},
	{Name: "WS", Match: run(blank), Hidden: true},
	{Name: "ML_COMMENT", Hidden: true, Match: func(src string, at int) int {
		if !strings.HasPrefix(src[at:], "%%") {
			return -1
		}
		return common.LineEnd(src, at+2)
	}},
	{Name: "NL", Match: common.NewlineEnd, Hidden: true},
	{Name: "ID2", Match: func(src string, at int) int {
		if at >= len(src) || !isLetter(src[at]) {
			return -1
		}
		return run(isWordByte)(src, at)
	}},
	{Name: "NUMBER2", Match: run(func(c byte) bool { return (c >= '0' && c <= '9') || c == '_' || c == '.' || c == ',' })},
	{Name: "STRING2", Match: func(src string, at int) int {
		if at >= len(src) || (src[at] != '"' && src[at] != '\'') {
			return -1
		}
		i := strings.IndexByte(src[at+1:], src[at])
		if i < 0 {
			return -1
		}
		return at + 1 + i + 1
	}},
}

type item struct {
	leaf          bool
	name          string
	value         float64
	classSelector string
}

type row struct {
	classDef  bool
	indent    int
	hasIndent bool
	item      item
	className string
	styleText string
}

type ast struct {
	title, accTitle, accDescr string
	rows                      []row
}

func parseTreemap(src string) ast {
	r := common.NewReader(common.Tokenize(src, tokens, "treemap"), tokens, "treemap")
	var a ast
	r.Expect(tKeyword)
	for r.Kind() != -1 {
		kind := r.Kind()
		switch kind {
		case tAccDescr:
			a.accDescr = common.AccDescrValue(r.Take())
			continue
		case tAccTitle:
			a.accTitle = common.AccTitleValue(r.Take())
			continue
		case tTitle:
			a.title = common.TitleValue(r.Take())
			continue
		}
		if kind != tIndentation && kind != tString && kind != tClassDef {
			r.Fail("a node, a class definition, or a title")
		}
		var rw row
		if kind == tIndentation {
			rw.indent, rw.hasIndent = len(r.Take()), true
		}
		if r.Kind() == tClassDef {
			_, name, style := classDef(r.Take(), 0)
			rw.classDef, rw.className, rw.styleText = true, name, style
			a.rows = append(a.rows, rw)
			continue
		}
		s := r.Expect(tString)
		name := s[1 : len(s)-1]
		// A leaf is a name followed by a colon or comma, with or without whitespace before it.
		after := r.I
		if r.KindAt(r.I) == tIndentation {
			after = r.I + 1
		}
		if k := r.KindAt(after); k == tSeparator || k == tComma {
			r.I = after + 1
			r.Accept(tIndentation)
			value := js.ParseFloat(strings.ReplaceAll(r.Expect(tNumber), ",", ""))
			rw.item = item{leaf: true, name: name, value: value}
		} else {
			rw.item = item{name: name}
		}
		if r.Accept(tStyleSeparator) {
			rw.item.classSelector = r.Expect(tID)
		}
		a.rows = append(a.rows, rw)
	}
	return a
}

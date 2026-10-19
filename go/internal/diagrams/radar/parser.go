package radar

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

const (
	tRadarColon = iota
	tRadar
	tShowLegend
	tGraticule
	tCurve
	tTicks
	tAxis
	tMax
	tMin
	tColon
	tComma
	tOpen
	tClose
	tOpenBrace
	tCloseBrace
	tGraticuleValue
	tBoolean
	tAccDescr
	tAccTitle
	tTitle
	tNumber
	tString
	tID
	tNewline
)

// charAt is src.charCodeAt(i) for ASCII tests, with -1 past the end.
func charAt(src string, i int) int {
	if i < 0 || i >= len(src) {
		return -1
	}
	return int(src[i])
}

func digits(src string, at int) int {
	end := at
	for c := charAt(src, end); c >= '0' && c <= '9'; c = charAt(src, end) {
		end++
	}
	return end
}

// matchNumber is /(?:[0-9]+\.[0-9]+(?!\.))|(?:0|[1-9][0-9]*(?!\.))/y without its backtracking.
// As a regular expression it reads to the end of a run of digits from every position in the
// run, which is quadratic for a long run of zeros.
func newNumberScanner() common.Scanner {
	runFrom, runEnd := 0, 0
	return func(src string, at int) int {
		if at < runFrom || at >= runEnd {
			runFrom = at
			runEnd = digits(src, at)
		}
		whole := runEnd
		if whole == at {
			return -1
		}
		dot := charAt(src, whole) == '.'
		if dot {
			fraction := digits(src, whole+1)
			length := fraction - whole - 1
			if length > 0 && charAt(src, fraction) != '.' {
				return fraction
			}
			if length > 1 {
				return fraction - 1
			}
		}
		if charAt(src, at) == '0' {
			return at + 1
		}
		if !dot {
			return whole
		}
		if whole-1 > at {
			return whole - 1
		}
		return -1
	}
}

// blockOpen is /[\t ]*accDescr\s*{/y, returning the end of the match or -1.
func blockOpen(src string, at int) int {
	p := at
	for p < len(src) && (src[p] == ' ' || src[p] == '\t') {
		p++
	}
	if !strings.HasPrefix(src[p:], "accDescr") {
		return -1
	}
	p += len("accDescr")
	for p < len(src) {
		r, size := js.RuneAt(src, p)
		if !js.IsSpace(r) {
			break
		}
		p += size
	}
	if p < len(src) && src[p] == '{' {
		return p + 1
	}
	return -1
}

// In the other grammars an `accDescr {` that never closes ends the lexing. Here `accDescr` is
// also a name and `{` a token, so each unclosed one would read to the end of the text again.
func newAccDescrScanner() common.Scanner {
	lastBrace := -2
	return func(src string, at int) int {
		if end := blockOpen(src, at); end >= 0 {
			if lastBrace == -2 {
				lastBrace = strings.LastIndexByte(src, '}')
			}
			if end > lastBrace {
				return -1
			}
		}
		return common.ACC_DESCR.Match(src, at)
	}
}

func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

// matchID is /[\w]([-\w]*\w)?/y: a word character, then the longest run of word characters and
// dashes that ends with a word character.
func matchID(src string, at int) int {
	if at >= len(src) || !isWordByte(src[at]) {
		return -1
	}
	end := at + 1
	for p := at + 1; p < len(src) && (isWordByte(src[p]) || src[p] == '-'); p++ {
		if src[p] != '-' {
			end = p + 1
		}
	}
	return end
}

// matchWords is a regular expression of alternative words, tried in order.
func matchWords(words ...string) common.Scanner {
	return func(src string, at int) int {
		for _, w := range words {
			if strings.HasPrefix(src[at:], w) {
				return at + len(w)
			}
		}
		return -1
	}
}

var idType = &common.TokenType{Name: "ID", Match: matchID}

func word(text string) *common.TokenType {
	return &common.TokenType{Name: text, Literal: text, Longer: []*common.TokenType{idType}}
}

func mark(text string) *common.TokenType { return &common.TokenType{Name: text, Literal: text} }

func radarKeyword() *common.TokenType {
	k := common.Keyword("radar-beta")
	k.Longer = []*common.TokenType{idType}
	return k
}

var tokens = []*common.TokenType{
	mark("radar-beta:"),
	radarKeyword(),
	word("showLegend"),
	word("graticule"),
	word("curve"),
	word("ticks"),
	word("axis"),
	word("max"),
	word("min"),
	mark(":"),
	mark(","),
	mark("["),
	mark("]"),
	mark("{"),
	mark("}"),
	{Name: "GRATICULE", Match: matchWords("circle", "polygon")},
	{Name: "BOOLEAN", Match: matchWords("true", "false")},
	{Name: "ACC_DESCR", NewScanner: newAccDescrScanner, First: common.ACC_DESCR.First},
	common.ACC_TITLE,
	common.TITLE,
	{Name: "NUMBER", NewScanner: newNumberScanner},
	common.STRING,
	idType,
	common.NEWLINE,
	common.WHITESPACE,
	common.YAML,
	common.DIRECTIVE,
	common.SINGLE_LINE_COMMENT,
}

type axisAst struct {
	name     string
	label    string
	hasLabel bool
}

type entryAst struct {
	// Langium leaves the reference unresolved; Mermaid reads only its text.
	axis    string
	hasAxis bool
	value   float64
}

type curveAst struct {
	name     string
	label    string
	hasLabel bool
	entries  []entryAst
}

type optionAst struct {
	name    string
	boolean bool
	text    string
	number  float64
}

type ast struct {
	title, accTitle, accDescr string
	axes                      []axisAst
	curves                    []curveAst
	options                   []optionAst
}

func parseRadar(src string) ast {
	r := common.NewReader(common.Tokenize(src, tokens, "radar"), tokens, "radar")
	var a ast

	newlines := func() {
		for r.Kind() == tNewline {
			r.I++
		}
	}

	endOfLine := func() {
		if r.Kind() == -1 {
			return
		}
		if r.Kind() != tNewline {
			r.Fail("a line break or the end of input")
		}
		newlines()
	}

	label := func() (string, bool) {
		if !r.Accept(tOpen) {
			return "", false
		}
		text := common.StringValue(r.Expect(tString))
		r.Expect(tClose)
		return text, true
	}

	entry := func(detailed bool) entryAst {
		if !detailed {
			return entryAst{value: js.ToNumber(r.Expect(tNumber))}
		}
		axis := r.Expect(tID)
		r.Accept(tColon)
		return entryAst{axis: axis, hasAxis: true, value: js.ToNumber(r.Expect(tNumber))}
	}

	curve := func() curveAst {
		node := curveAst{name: r.Expect(tID)}
		node.label, node.hasLabel = label()
		r.Expect(tOpenBrace)
		newlines()
		if r.Kind() != tNumber && r.Kind() != tID {
			r.Fail("a number or an axis name")
		}
		detailed := r.Kind() == tID
		node.entries = append(node.entries, entry(detailed))
		for r.Accept(tComma) {
			newlines()
			node.entries = append(node.entries, entry(detailed))
		}
		newlines()
		r.Expect(tCloseBrace)
		return node
	}

	axis := func() axisAst {
		node := axisAst{name: r.Expect(tID)}
		node.label, node.hasLabel = label()
		return node
	}

	option := func() optionAst {
		kind := r.Kind()
		o := optionAst{name: r.Take()}
		switch kind {
		case tShowLegend:
			o.boolean = r.Expect(tBoolean) == "true"
		case tGraticule:
			o.text = r.Expect(tGraticuleValue)
		default:
			o.number = js.ToNumber(r.Expect(tNumber))
		}
		return o
	}

	isOption := func() bool {
		k := r.Kind()
		return k == tShowLegend || k == tTicks || k == tMax || k == tMin || k == tGraticule
	}

	newlines()
	if !r.Accept(tRadarColon) {
		r.Expect(tRadar)
		r.Accept(tColon)
	}
	newlines()
	for r.Kind() != -1 {
		switch r.Kind() {
		case tNewline:
			r.I++
		case tAccDescr:
			a.accDescr = common.AccDescrValue(r.Take())
			endOfLine()
		case tAccTitle:
			a.accTitle = common.AccTitleValue(r.Take())
			endOfLine()
		case tTitle:
			a.title = common.TitleValue(r.Take())
			endOfLine()
		case tAxis:
			r.I++
			a.axes = append(a.axes, axis())
			for r.Accept(tComma) {
				a.axes = append(a.axes, axis())
			}
		case tCurve:
			r.I++
			a.curves = append(a.curves, curve())
			for r.Accept(tComma) {
				a.curves = append(a.curves, curve())
			}
		default:
			if !isOption() {
				r.Fail("an axis, a curve, an option, a title, or a line break")
			}
			a.options = append(a.options, option())
			for r.Accept(tComma) {
				if !isOption() {
					r.Fail("an option")
				}
				a.options = append(a.options, option())
			}
		}
	}
	return a
}

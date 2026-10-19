package wardley

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

const (
	tLbracket = iota
	tComma
	tRbracket
	tAt
	tSlash
	tLparen
	tRparen
	tMinus
	tLbrace
	tRbrace
	tNumber
	tArrow
	tLinkPort
	tLinkArrow
	tLinkLabel
	tStrategy
	tWardley
	tSize
	tEvolution
	tAnchor
	tComponent
	tLabel
	tInertia
	tEvolve
	tPipeline
	tNote
	tAnnotations
	tAnnotation
	tAccelerator
	tDeaccelerator
	tName
	tWS
	tAccDescr
	tAccTitle
	tTitle
	tInt
	tString
	tID
	tNewline
)

func isDigit(src string, i int) bool { return i < len(src) && src[i] >= '0' && src[i] <= '9' }

func digits(src string, from int) int {
	i := from
	for isDigit(src, i) {
		i++
	}
	return i
}

// numberScanner answers as WARDLEY_NUMBER's pattern does. The pattern reads a whole run of digits
// before it finds there is no fraction, and would do so again from every digit of a long run.
func numberScanner() common.Scanner {
	from, end := 0, 0
	return func(src string, p int) int {
		if p < from || p >= end {
			from = p
			end = digits(src, p)
		}
		if end == p || end >= len(src) || src[end] != '.' {
			return -1
		}
		stop := digits(src, end+1)
		if stop > end+1 {
			return stop
		}
		return -1
	}
}

func literal(s string) common.Scanner {
	return func(src string, at int) int {
		if strings.HasPrefix(src[at:], s) {
			return at + len(s)
		}
		return -1
	}
}

var arrow = &common.TokenType{Name: "ARROW", Match: literal("->"), First: "-"}

// quotedArrow is \+'[^']*'(<>|<|>) at at, tried in that order.
func quotedArrow(src string, at int) int {
	if !strings.HasPrefix(src[at:], "+'") {
		return -1
	}
	close := strings.IndexByte(src[at+2:], '\'')
	if close < 0 {
		return -1
	}
	p := at + 2 + close + 1
	switch {
	case strings.HasPrefix(src[p:], "<>"):
		return p + 2
	case strings.HasPrefix(src[p:], "<"), strings.HasPrefix(src[p:], ">"):
		return p + 1
	}
	return -1
}

// LINK_ARROW is /-->|-\.->|>|\+'[^']*'<>|\+'[^']*'<|\+'[^']*'>/y.
var linkArrow = &common.TokenType{Name: "LINK_ARROW", First: "->+", Match: func(src string, at int) int {
	for _, s := range []string{"-->", "-.->", ">"} {
		if strings.HasPrefix(src[at:], s) {
			return at + len(s)
		}
	}
	return quotedArrow(src, at)
}}

func word(name, text string) *common.TokenType {
	return &common.TokenType{Name: name, Match: literal(text), First: text[:1]}
}

func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

// nameRest is (?:[A-Za-z0-9_()&]|-(?!>))* at p.
func nameRest(src string, p int) int {
	for p < len(src) {
		c := src[p]
		if isLetter(c) || (c >= '0' && c <= '9') || c == '_' || c == '(' || c == ')' || c == '&' {
			p++
		} else if c == '-' && !(p+1 < len(src) && src[p+1] == '>') {
			p++
		} else {
			break
		}
	}
	return p
}

// matchName is NAME_WITH_SPACES:
// /(?!title\s|accTitle|accDescr)[A-Za-z](?:[A-Za-z0-9_()&]|-(?!>))*(?:[ \t]+[A-Za-z(](?:[A-Za-z0-9_()&]|-(?!>))*)*/y
func matchName(src string, at int) int {
	rest := src[at:]
	if strings.HasPrefix(rest, "accTitle") || strings.HasPrefix(rest, "accDescr") {
		return -1
	}
	if strings.HasPrefix(rest, "title") {
		if r, _ := js.RuneAt(src, at+5); js.IsSpace(r) {
			return -1
		}
	}
	if at >= len(src) || !isLetter(src[at]) {
		return -1
	}
	p := nameRest(src, at+1)
	for {
		q := p
		for q < len(src) && (src[q] == ' ' || src[q] == '\t') {
			q++
		}
		if q == p || q >= len(src) || !(isLetter(src[q]) || src[q] == '(') {
			return p
		}
		p = nameRest(src, q+1)
	}
}

func isWordByte(c byte) bool { return js.IsWord(rune(c)) }

// matchInt is /0|[1-9][0-9]*(?!\.)/y.
func matchInt(src string, at int) int {
	if at >= len(src) {
		return -1
	}
	if src[at] == '0' {
		return at + 1
	}
	if src[at] < '1' || src[at] > '9' {
		return -1
	}
	e := digits(src, at+1)
	if e >= len(src) || src[e] != '.' {
		return e
	}
	// Giving back a digit puts a digit, not the point, after the match.
	if e-1 > at {
		return e - 1
	}
	return -1
}

// matchID is /[\w]([-\w]*\w)?/y.
func matchID(src string, at int) int {
	if at >= len(src) || !isWordByte(src[at]) {
		return -1
	}
	end := at + 1
	for p := at + 1; p < len(src) && (src[p] == '-' || isWordByte(src[p])); p++ {
		if src[p] != '-' {
			end = p + 1
		}
	}
	return end
}

var tokens = []*common.TokenType{
	{Name: "[", Literal: "["},
	{Name: ",", Literal: ","},
	{Name: "]", Literal: "]"},
	{Name: "@", Literal: "@"},
	{Name: "/", Literal: "/"},
	{Name: "(", Literal: "("},
	{Name: ")", Literal: ")"},
	{Name: "-", Literal: "-", Longer: []*common.TokenType{arrow, linkArrow, common.YAML}},
	{Name: "{", Literal: "{"},
	{Name: "}", Literal: "}"},
	{Name: "WARDLEY_NUMBER", First: "0123456789", NewScanner: numberScanner},
	arrow,
	{Name: "LINK_PORT", First: "+", Match: func(src string, at int) int {
		for _, s := range []string{"+<>", "+>", "+<"} {
			if strings.HasPrefix(src[at:], s) {
				return at + len(s)
			}
		}
		return -1
	}},
	linkArrow,
	{Name: "LINK_LABEL", First: ";", Match: func(src string, at int) int {
		if at >= len(src) || src[at] != ';' {
			return -1
		}
		p := at + 1
		for p < len(src) && src[p] != '\n' && src[p] != '\r' {
			p++
		}
		if p == at+1 {
			return -1
		}
		return p
	}},
	{Name: "STRATEGY", First: "bom", Match: func(src string, at int) int {
		for _, s := range []string{"build", "buy", "outsource", "market"} {
			if strings.HasPrefix(src[at:], s) {
				return at + len(s)
			}
		}
		return -1
	}},
	word("KW_WARDLEY", "wardley-beta"),
	word("KW_SIZE", "size"),
	word("KW_EVOLUTION", "evolution"),
	word("KW_ANCHOR", "anchor"),
	word("KW_COMPONENT", "component"),
	word("KW_LABEL", "label"),
	word("KW_INERTIA", "inertia"),
	word("KW_EVOLVE", "evolve"),
	word("KW_PIPELINE", "pipeline"),
	word("KW_NOTE", "note"),
	word("KW_ANNOTATIONS", "annotations"),
	word("KW_ANNOTATION", "annotation"),
	word("KW_ACCELERATOR", "accelerator"),
	word("KW_DEACCELERATOR", "deaccelerator"),
	{Name: "NAME_WITH_SPACES", First: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", Match: matchName},
	{Name: "WS", Hidden: true, First: "\t ", Match: func(src string, at int) int {
		p := at
		for p < len(src) && (src[p] == ' ' || src[p] == '\t') {
			p++
		}
		if p == at {
			return -1
		}
		return p
	}},
	common.AccDescr("ACC_DESCR"),
	common.ACC_TITLE,
	common.TITLE,
	{Name: "INT", First: "0123456789", Match: matchInt},
	common.STRING,
	{Name: "ID", First: "_0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", Match: matchID},
	common.NEWLINE,
	common.WHITESPACE,
	common.YAML,
	common.DIRECTIVE,
	common.SINGLE_LINE_COMMENT,
}

type labelAst struct {
	negX, negY       bool
	offsetX, offsetY float64
}

type stageAst struct {
	name        string
	boundary    float64
	hasBoundary bool
	// "" for none: an empty name is left out as none is.
	secondName string
}

type componentAst struct {
	name                  string
	visibility, evolution float64
	label                 *labelAst
	// "" for no decorator.
	strategy string
	inertia  bool
}

type pipelineComponentAst struct {
	name      string
	evolution float64
	label     *labelAst
}

type linkAst struct {
	from, fromPort, arrow, to, toPort string
	linkLabel                         string
	hasLinkLabel                      bool
}

type placedAst struct {
	name string
	x, y float64
}

type sizeAst struct{ width, height float64 }

type anchorAst struct {
	name                  string
	visibility, evolution float64
}

type evolveAst struct {
	component string
	target    float64
}

type pipelineAst struct {
	parent     string
	components []pipelineComponentAst
}

type noteAst struct {
	text                  string
	visibility, evolution float64
}

type annotationAst struct {
	number, x, y float64
	text         string
}

type wardleyAst struct {
	title, accTitle, accDescr          string
	hasTitle, hasAccTitle, hasAccDescr bool
	size                               *sizeAst
	evolution                          []stageAst
	anchors                            []anchorAst
	components                         []componentAst
	links                              []linkAst
	evolves                            []evolveAst
	pipelines                          []pipelineAst
	notes                              []noteAst
	annotations                        [][2]float64
	annotation                         []annotationAst
	accelerators                       []placedAst
	deaccelerators                     []placedAst
}

func parseWardley(src string) *wardleyAst {
	r := common.NewReader(common.Tokenize(src, tokens, "wardley"), tokens, "wardley")
	a := &wardleyAst{}

	at := func(kind int) bool { return r.Kind() == kind }
	isName := func() bool { return at(tString) || at(tID) || at(tName) }
	name := func() string {
		if at(tString) {
			return common.StringValue(r.Take())
		}
		if at(tID) || at(tName) {
			return r.Take()
		}
		r.Fail("a name")
		return ""
	}
	number := func() float64 { return js.ToNumber(r.Expect(tNumber)) }
	integer := func() float64 { return js.ParseInt(r.Expect(tInt), 0) }
	coordinate := func() float64 {
		if at(tNumber) {
			return js.ToNumber(r.Take())
		}
		return js.ParseInt(r.Expect(tInt), 0)
	}
	endOfLine := func() {
		if at(-1) {
			return
		}
		if !at(tNewline) {
			r.Fail("a line break or the end of input")
		}
		for at(tNewline) {
			r.I++
		}
	}
	pair := func(read func() float64) (float64, float64) {
		r.Expect(tLbracket)
		first := read()
		r.Expect(tComma)
		second := read()
		r.Expect(tRbracket)
		return first, second
	}
	label := func() *labelAst {
		r.Expect(tLabel)
		r.Expect(tLbracket)
		l := &labelAst{}
		l.negX = r.Accept(tMinus)
		l.offsetX = integer()
		r.Expect(tComma)
		l.negY = r.Accept(tMinus)
		l.offsetY = integer()
		r.Expect(tRbracket)
		return l
	}
	stage := func() stageAst {
		out := stageAst{name: name()}
		if r.Accept(tAt) {
			out.boundary = number()
			out.hasBoundary = true
		}
		if r.Accept(tSlash) {
			out.secondName = name()
		}
		return out
	}

	for at(tNewline) {
		r.I++
	}
	r.Expect(tWardley)
	for !at(-1) {
		switch r.Kind() {
		case tNewline:
			r.I++
		case tAccDescr:
			a.accDescr, a.hasAccDescr = common.AccDescrValue(r.Take()), true
			endOfLine()
		case tAccTitle:
			a.accTitle, a.hasAccTitle = common.AccTitleValue(r.Take()), true
			endOfLine()
		case tTitle:
			a.title, a.hasTitle = common.TitleValue(r.Take()), true
			endOfLine()
		case tSize:
			r.I++
			width, height := pair(integer)
			endOfLine()
			a.size = &sizeAst{width, height}
		case tEvolution:
			r.I++
			stages := []stageAst{stage()}
			r.Expect(tArrow)
			stages = append(stages, stage())
			for r.Accept(tArrow) {
				stages = append(stages, stage())
			}
			endOfLine()
			a.evolution = stages
		case tAnchor:
			r.I++
			anchor := name()
			visibility, evolution := pair(number)
			endOfLine()
			a.anchors = append(a.anchors, anchorAst{anchor, visibility, evolution})
		case tComponent:
			r.I++
			c := componentAst{name: name()}
			c.visibility, c.evolution = pair(number)
			if at(tLabel) {
				c.label = label()
			}
			if at(tLparen) && r.KindAt(r.I+1) == tStrategy {
				r.I++
				c.strategy = r.Take()
				r.Expect(tRparen)
			}
			if r.Accept(tInertia) {
				c.inertia = true
			} else if r.Accept(tLparen) {
				r.Expect(tInertia)
				r.Expect(tRparen)
				c.inertia = true
			}
			endOfLine()
			a.components = append(a.components, c)
		case tEvolve:
			r.I++
			component := name()
			target := number()
			endOfLine()
			a.evolves = append(a.evolves, evolveAst{component, target})
		case tPipeline:
			r.I++
			parent := name()
			r.Expect(tLbrace)
			r.Expect(tNewline)
			for at(tNewline) {
				r.I++
			}
			var components []pipelineComponentAst
			for {
				r.Expect(tComponent)
				c := pipelineComponentAst{name: name()}
				r.Expect(tLbracket)
				c.evolution = number()
				r.Expect(tRbracket)
				if at(tLabel) {
					c.label = label()
				}
				endOfLine()
				components = append(components, c)
				if !at(tComponent) {
					break
				}
			}
			r.Expect(tRbrace)
			endOfLine()
			a.pipelines = append(a.pipelines, pipelineAst{parent, components})
		case tNote:
			r.I++
			t := common.StringValue(r.Expect(tString))
			visibility, evolution := pair(number)
			endOfLine()
			a.notes = append(a.notes, noteAst{t, visibility, evolution})
		case tAnnotations:
			r.I++
			x, y := pair(coordinate)
			endOfLine()
			a.annotations = append(a.annotations, [2]float64{x, y})
		case tAnnotation:
			r.I++
			n := integer()
			r.Expect(tComma)
			x, y := pair(coordinate)
			t := common.StringValue(r.Expect(tString))
			endOfLine()
			a.annotation = append(a.annotation, annotationAst{n, x, y, t})
		case tAccelerator, tDeaccelerator:
			slow := r.Kind() == tDeaccelerator
			r.I++
			force := name()
			x, y := pair(number)
			endOfLine()
			if slow {
				a.deaccelerators = append(a.deaccelerators, placedAst{force, x, y})
			} else {
				a.accelerators = append(a.accelerators, placedAst{force, x, y})
			}
		default:
			if !isName() {
				r.Fail("a statement")
			}
			link := linkAst{from: name()}
			if at(tLinkPort) {
				link.fromPort = r.Take()
			}
			if at(tLinkArrow) || at(tArrow) {
				link.arrow = r.Take()
			}
			link.to = name()
			if at(tLinkPort) {
				link.toPort = r.Take()
			}
			if at(tLinkLabel) {
				link.linkLabel, link.hasLinkLabel = js.Trim(r.Take()[1:]), true
			}
			endOfLine()
			a.links = append(a.links, link)
		}
	}
	return a
}

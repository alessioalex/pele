package railroad

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

// `railroad-abnf-beta`: ABNF as in RFC 5234, with each rule closed by a semicolon.

const (
	abKeyword = iota
	abEquals
	abSemicolon
	abSlash
	abGroup
	abGroupEnd
	abOption
	abOptionEnd
	abTitle
	abAccTitle
	abAccDescr
	abName
	abString
	abNumber
	abRepeat
	abExact
)

const digits = "0123456789"

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHexDigit(c byte) bool { return isDigit(c) || c >= 'A' && c <= 'F' || c >= 'a' && c <= 'f' }

func isAlnum(c byte) bool { return isDigit(c) || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' }

func skipDigits(src string, p int) int {
	for p < len(src) && isDigit(src[p]) {
		p++
	}
	return p
}

func skipHex(src string, p int) int {
	for p < len(src) && isHexDigit(src[p]) {
		p++
	}
	return p
}

var (
	// /[A-Za-z][A-Za-z0-9-]*/y
	abnfNameToken = &tokenType{Name: "ABNF_RULENAME", First: letters, Match: func(src string, at int) int {
		if at >= len(src) || !isAlnum(src[at]) || isDigit(src[at]) {
			return -1
		}
		p := at + 1
		for p < len(src) && (isAlnum(src[p]) || src[p] == '-') {
			p++
		}
		return p
	}}
	// /;[^\n\r]*/y
	abnfComment = &tokenType{Name: "ABNF_COMMENT", Hidden: true, First: ";", Match: func(src string, at int) int {
		if at >= len(src) || src[at] != ';' {
			return -1
		}
		if i := strings.IndexAny(src[at:], "\n\r"); i >= 0 {
			return at + i
		}
		return len(src)
	}}
)

var abnfTokens = append([]*tokenType{
	opening("railroad-abnf-beta", abnfNameToken),
	literal("="),
	// A semicolon closes a rule only at the end of its line; with anything after it, it starts a comment.
	literal(";", abnfComment),
	literal("/"),
	literal("("),
	literal(")"),
	literal("["),
	literal("]"),
	TITLE,
	ACC_TITLE,
	ACC_DESCR,
	abnfNameToken,
	// /"[^"]*"/y
	{Name: "ABNF_STRING", First: `"`, Match: func(src string, at int) int {
		if at >= len(src) || src[at] != '"' {
			return -1
		}
		if i := strings.IndexByte(src[at+1:], '"'); i >= 0 {
			return at + 1 + i + 1
		}
		return -1
	}},
	// /%[xXdDbB][0-9A-Fa-f]+(?:-[0-9A-Fa-f]+|\.[0-9A-Fa-f]+)*/y
	{Name: "ABNF_NUMVAL", First: "%", Match: func(src string, at int) int {
		if at+1 >= len(src) || src[at] != '%' || !strings.ContainsRune("xXdDbB", rune(src[at+1])) {
			return -1
		}
		p := skipHex(src, at+2)
		if p == at+2 {
			return -1
		}
		for p+1 < len(src) && (src[p] == '-' || src[p] == '.') && isHexDigit(src[p+1]) {
			p = skipHex(src, p+1)
		}
		return p
	}},
	// /[0-9]*\*[0-9]*/y
	{Name: "ABNF_REPEAT", First: digits + "*", Match: func(src string, at int) int {
		p := skipDigits(src, at)
		if p >= len(src) || src[p] != '*' {
			return -1
		}
		return skipDigits(src, p+1)
	}},
	// /[0-9]+/y
	{Name: "ABNF_EXACT_REPEAT", First: digits, Match: func(src string, at int) int {
		if p := skipDigits(src, at); p > at {
			return p
		}
		return -1
	}},
}, append(hiddenTokens("ABNF"), abnfComment)...)

// The $type of a part of the syntax tree.
const (
	abnfAlternation = iota
	abnfConcatenation
	abnfElement
	abnfStringLiteral
	abnfNumVal
	abnfRuleName
	abnfGroup
	abnfOptionalGroup
)

type abnfPart struct {
	typ int
	// A literal's value, a rule name, or an element's repeat ("" when none was written).
	text string
	// An alternation's alternatives, a concatenation's elements, an element's primary, or a
	// group's element.
	kids []*abnfPart
}

type abnfRule = astRule[*abnfPart]

// One open bracket, or the rule itself at the bottom of the stack.
type abnfFrame struct {
	alternation, concatenation *abnfPart
	// The bracket's node, its closing token, and the repeat written before it.
	node   *abnfPart
	close  int
	repeat string
}

func abnfNewFrame(node, alternation *abnfPart, close int, repeat string) *abnfFrame {
	concatenation := &abnfPart{typ: abnfConcatenation}
	alternation.kids = append(alternation.kids, concatenation)
	return &abnfFrame{alternation: alternation, concatenation: concatenation, node: node, close: close, repeat: repeat}
}

func abnfAlternationOf(r *common.Reader) *abnfPart {
	var stack []*abnfFrame
	top := abnfNewFrame(nil, &abnfPart{typ: abnfAlternation}, -2, "")
	for {
		repeat := ""
		if r.Kind() == abRepeat || r.Kind() == abExact {
			repeat = r.Take()
		}
		var primary *abnfPart
		kind := r.Kind()
		switch kind {
		case abString:
			primary = &abnfPart{typ: abnfStringLiteral, text: inner(r.Take())}
		case abNumber:
			primary = &abnfPart{typ: abnfNumVal, text: r.Take()}
		case abName:
			primary = &abnfPart{typ: abnfRuleName, text: r.Take()}
		case abGroup, abOption:
			r.I++
			element := &abnfPart{typ: abnfAlternation}
			stack = append(stack, top)
			typ := abnfOptionalGroup
			if kind == abGroup {
				typ = abnfGroup
			}
			top = abnfNewFrame(&abnfPart{typ: typ, kids: []*abnfPart{element}}, element, kind+1, repeat)
			continue
		default:
			r.Fail("a string, a number, a name, or an opening bracket")
		}
		for {
			top.concatenation.kids = append(top.concatenation.kids, &abnfPart{typ: abnfElement, text: repeat, kids: []*abnfPart{primary}})
			k := r.Kind()
			if k == top.close {
				r.I++
				primary = top.node
				repeat = top.repeat
				top = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				continue
			}
			if k == abSlash {
				r.I++
				top.concatenation = &abnfPart{typ: abnfConcatenation}
				top.alternation.kids = append(top.alternation.kids, top.concatenation)
			} else if k != abString && k != abNumber && k != abName && k != abGroup && k != abOption && k != abRepeat && k != abExact {
				if len(stack) == 0 {
					return top.alternation
				}
				r.Fail("token of type '" + abnfTokens[top.close].Name + "'")
			}
			break
		}
	}
}

func parseAbnf(src string) grammar[abnfRule] {
	return parseGrammar(src, abnfTokens, abTitle, inner, func(r *common.Reader) abnfRule {
		name := r.Expect(abName)
		r.Expect(abEquals)
		definition := abnfAlternationOf(r)
		r.Expect(abSemicolon)
		return abnfRule{name, definition}
	})
}

func abnfNode(part *abnfPart, kids []*Node) *Node {
	switch part.typ {
	case abnfAlternation:
		return choice(kids)
	case abnfConcatenation:
		return sequence(kids)
	case abnfElement:
		repeat := part.text
		if repeat == "" {
			return kids[0]
		}
		star := strings.IndexByte(repeat, '*')
		exact := star == -1
		min := 0.0
		if star > 0 || exact {
			min = js.ParseInt(repeat, 10)
		}
		max := inf
		if exact {
			max = min
		} else if star < len(repeat)-1 {
			max = js.ParseInt(repeat[star+1:], 10)
		}
		if min == 0 && max == 1 {
			return optional(kids[0])
		}
		return repetition(kids[0], min, max)
	case abnfStringLiteral, abnfNumVal:
		return terminal(part.text)
	case abnfRuleName:
		return nonterminal(part.text)
	case abnfGroup:
		return kids[0]
	default:
		return optional(kids[0])
	}
}

func abnfRules(ast grammar[abnfRule]) []Rule {
	return toRules(ast, func(p *abnfPart) []*abnfPart { return p.kids }, abnfNode)
}

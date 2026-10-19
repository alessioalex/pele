package railroad

import "github.com/alessioalex/pele/go/internal/diagrams/common"

// `railroad-beta`: Mermaid's own notation, one constructor call per node.

const (
	irKeyword = iota
	irNonterminal
	irZeroOrMore
	irOneOrMore
	irSequence
	irOptional
	irTerminal
	irSpecial
	irChoice
	irEquals
	irSemicolon
	irOpen
	irComma
	irClose
	irTitle
	irAccTitle
	irAccDescr
	irID
	irString
)

var irIDToken = identifier("RR_ID")

func irWord(name string) *tokenType { return literal(name, irIDToken) }

var irTokens = append([]*tokenType{
	opening("railroad-beta", irIDToken),
	irWord("nonterminal"),
	irWord("zeroOrMore"),
	irWord("oneOrMore"),
	irWord("sequence"),
	irWord("optional"),
	irWord("terminal"),
	irWord("special"),
	irWord("choice"),
	literal("="),
	literal(";"),
	literal("("),
	literal(","),
	literal(")"),
	TITLE,
	ACC_TITLE,
	ACC_DESCR,
	irIDToken,
	quoted("RR_STRING"),
}, append(hiddenTokens("RR"), blockComment("RR_BLOCK_COMMENT", "/*", "*/"))...)

// The $type of an expression of the syntax tree.
const (
	irSequenceExpr = iota
	irChoiceExpr
	irOptionalExpr
	irOneOrMoreExpr
	irZeroOrMoreExpr
	irTerminalExpr
	irNonTerminalExpr
	irSpecialExpr
)

type irExpr struct {
	typ  int
	text string
	// A sequence's elements, a choice's alternatives, or a wrapper's one element.
	kids []*irExpr
}

type irRule = astRule[*irExpr]

func irArgument(r *common.Reader) string {
	r.I++
	r.Expect(irOpen)
	value := unescape(r.Expect(irString))
	r.Expect(irClose)
	return value
}

func irExpression(r *common.Reader) *irExpr {
	var open []*irExpr
	for {
		var value *irExpr
		switch r.Kind() {
		case irTerminal:
			value = &irExpr{typ: irTerminalExpr, text: irArgument(r)}
		case irNonterminal:
			value = &irExpr{typ: irNonTerminalExpr, text: irArgument(r)}
		case irSpecial:
			value = &irExpr{typ: irSpecialExpr, text: irArgument(r)}
		case irSequence:
			value = &irExpr{typ: irSequenceExpr}
		case irChoice:
			value = &irExpr{typ: irChoiceExpr}
		case irOptional:
			value = &irExpr{typ: irOptionalExpr}
		case irOneOrMore:
			value = &irExpr{typ: irOneOrMoreExpr}
		case irZeroOrMore:
			value = &irExpr{typ: irZeroOrMoreExpr}
		default:
			r.Fail("an expression")
		}
		if value.typ <= irZeroOrMoreExpr {
			r.I++
			r.Expect(irOpen)
			open = append(open, value)
			continue
		}
		for {
			if len(open) == 0 {
				return value
			}
			parent := open[len(open)-1]
			if parent.typ >= irOptionalExpr {
				parent.kids = []*irExpr{value}
			} else {
				parent.kids = append(parent.kids, value)
				if r.Accept(irComma) {
					break
				}
			}
			r.Expect(irClose)
			open = open[:len(open)-1]
			value = parent
		}
	}
}

func parseIr(src string) grammar[irRule] {
	return parseGrammar(src, irTokens, irTitle, unescape, func(r *common.Reader) irRule {
		name := r.Expect(irID)
		r.Expect(irEquals)
		definition := irExpression(r)
		r.Expect(irSemicolon)
		return irRule{name, definition}
	})
}

func irNode(e *irExpr, kids []*Node) *Node {
	switch e.typ {
	case irTerminalExpr:
		return terminal(e.text)
	case irNonTerminalExpr:
		return nonterminal(e.text)
	case irSpecialExpr:
		return special(e.text)
	case irSequenceExpr:
		return sequence(kids)
	case irChoiceExpr:
		return choice(kids)
	case irOptionalExpr:
		return optional(kids[0])
	case irOneOrMoreExpr:
		return repetition(kids[0], 1, inf)
	default:
		return repetition(kids[0], 0, inf)
	}
}

func irRules(ast grammar[irRule]) []Rule {
	return toRules(ast, func(e *irExpr) []*irExpr { return e.kids }, irNode)
}

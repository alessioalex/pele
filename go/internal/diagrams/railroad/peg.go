package railroad

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/diagrams/common"
)

// `railroad-peg-beta`: parsing expression grammars.

const (
	pgKeyword = iota
	pgArrow
	pgSemicolon
	pgSlash
	pgAnd
	pgNot
	pgQuestion
	pgStar
	pgPlus
	pgGroup
	pgGroupEnd
	pgDot
	pgTitle
	pgAccTitle
	pgAccDescr
	pgID
	pgString
)

var pegIDToken = identifier("PEG_ID")

var pegTokens = append([]*tokenType{
	opening("railroad-peg-beta", pegIDToken),
	literal("<-"),
	literal(";"),
	literal("/"),
	literal("&"),
	literal("!"),
	literal("?"),
	literal("*"),
	literal("+"),
	literal("("),
	literal(")"),
	literal("."),
	TITLE,
	ACC_TITLE,
	ACC_DESCR,
	pegIDToken,
	quoted("PEG_STRING"),
}, append(hiddenTokens("PEG"),
	// /#[^\n\r]*/y
	&tokenType{Name: "PEG_LINE_COMMENT", Hidden: true, First: "#", Match: func(src string, at int) int {
		if at >= len(src) || src[at] != '#' {
			return -1
		}
		if i := strings.IndexAny(src[at:], "\n\r"); i >= 0 {
			return at + i
		}
		return len(src)
	}})...)

// The $type of a part of the syntax tree.
const (
	pegOrderedChoice = iota
	pegSequence
	pegPrefix
	pegSuffix
	pegLiteral
	pegIdentifier
	pegAny
	pegGroup
)

type pegPart struct {
	typ int
	// A literal's value, a name, the dot, or a prefix's or a suffix's operator ("" when none).
	text string
	// A choice's alternatives, a sequence's elements, a prefix's suffix, a suffix's primary, or
	// a group's element.
	kids []*pegPart
}

type pegRule = astRule[*pegPart]

// One open group, or the rule itself at the bottom of the stack.
type pegFrame struct {
	choice, sequence *pegPart
	// The group's node and the predicate written before it.
	node     *pegPart
	operator string
}

func pegNewFrame(node, choice *pegPart, operator string) *pegFrame {
	sequence := &pegPart{typ: pegSequence}
	choice.kids = append(choice.kids, sequence)
	return &pegFrame{choice: choice, sequence: sequence, node: node, operator: operator}
}

func pegDefinition(r *common.Reader) *pegPart {
	var stack []*pegFrame
	top := pegNewFrame(nil, &pegPart{typ: pegOrderedChoice}, "")
	for {
		operator := ""
		if r.Kind() == pgAnd || r.Kind() == pgNot {
			operator = r.Take()
		}
		var primary *pegPart
		switch r.Kind() {
		case pgString:
			primary = &pegPart{typ: pegLiteral, text: unescape(r.Take())}
		case pgID:
			primary = &pegPart{typ: pegIdentifier, text: r.Take()}
		case pgDot:
			primary = &pegPart{typ: pegAny, text: r.Take()}
		case pgGroup:
			r.I++
			element := &pegPart{typ: pegOrderedChoice}
			stack = append(stack, top)
			top = pegNewFrame(&pegPart{typ: pegGroup, kids: []*pegPart{element}}, element, operator)
			continue
		default:
			r.Fail("a literal, a name, a dot, or an opening parenthesis")
		}
		for {
			suffix := &pegPart{typ: pegSuffix, kids: []*pegPart{primary}}
			if k := r.Kind(); k == pgQuestion || k == pgStar || k == pgPlus {
				suffix.text = r.Take()
			}
			top.sequence.kids = append(top.sequence.kids, &pegPart{typ: pegPrefix, text: operator, kids: []*pegPart{suffix}})
			k := r.Kind()
			if k == pgGroupEnd && len(stack) > 0 {
				r.I++
				primary = top.node
				operator = top.operator
				top = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				continue
			}
			if k == pgSlash {
				r.I++
				top.sequence = &pegPart{typ: pegSequence}
				top.choice.kids = append(top.choice.kids, top.sequence)
			} else if k != pgAnd && k != pgNot && k != pgString && k != pgID && k != pgDot && k != pgGroup {
				if len(stack) == 0 {
					return top.choice
				}
				r.Fail("token of type ')'")
			}
			break
		}
	}
}

func parsePeg(src string) grammar[pegRule] {
	return parseGrammar(src, pegTokens, pgTitle, unescape, func(r *common.Reader) pegRule {
		name := r.Expect(pgID)
		r.Expect(pgArrow)
		rule := pegRule{name, pegDefinition(r)}
		r.Expect(pgSemicolon)
		return rule
	})
}

// pegLabel is what a predicate shows of the expression it looks ahead for.
func pegLabel(node *Node) string {
	switch node.Type {
	case Terminal:
		return `"` + node.Text + `"`
	case Nonterminal, Special:
		return node.Text
	default:
		return "(...)"
	}
}

func pegNode(part *pegPart, kids []*Node) *Node {
	var kid *Node
	if len(kids) > 0 {
		kid = kids[0]
	}
	switch part.typ {
	case pegOrderedChoice:
		return choice(kids)
	case pegSequence:
		return sequence(kids)
	case pegPrefix:
		if part.text != "" {
			return special(part.text + pegLabel(kid))
		}
		return kid
	case pegSuffix:
		switch part.text {
		case "":
			return kid
		case "?":
			return optional(kid)
		case "+":
			return repetition(kid, 1, inf)
		default:
			return repetition(kid, 0, inf)
		}
	case pegLiteral:
		return terminal(part.text)
	case pegIdentifier:
		return nonterminal(part.text)
	case pegAny:
		return special(part.text)
	default:
		return kid
	}
}

func pegRules(ast grammar[pegRule]) []Rule {
	return toRules(ast, func(p *pegPart) []*pegPart { return p.kids }, pegNode)
}

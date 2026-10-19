package railroad

import (
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

// `railroad-ebnf-beta`: EBNF in the W3C and ISO 14977 styles.

const (
	ebKeyword = iota
	ebDefines
	ebEquals
	ebSemicolon
	ebBar
	ebComma
	ebGroup
	ebGroupEnd
	ebOption
	ebOptionEnd
	ebRepeat
	ebRepeatEnd
	ebQuestion
	ebStar
	ebPlus
	ebMinus
	ebTitle
	ebAccTitle
	ebAccDescr
	ebID
	ebString
	ebSpecial
)

// specialSequence is a question mark, text with something other than blanks in it and no `?`
// or `;`, and a question mark: /\?(?=[^?;]*[^?\s;][^?;]*\?)[^?;]*\?/y.
func specialSequence(src string, at int) int {
	if at >= len(src) || src[at] != '?' {
		return -1
	}
	solid := false
	for i := at + 1; i < len(src); {
		c := src[i]
		if c == '?' {
			if solid {
				return i + 1
			}
			return -1
		}
		if c == ';' {
			return -1
		}
		r, size := js.RuneAt(src, i)
		solid = solid || !js.IsSpace(r)
		i += size
	}
	return -1
}

var (
	ebnfIDToken      = identifier("EBNF_ID")
	ebnfSpecialToken = &tokenType{Name: "EBNF_SPECIAL_SEQUENCE", First: "?", Match: specialSequence}
	ebnfHidden       = hiddenTokens("EBNF")
	ebnfISOComment   = blockComment("EBNF_ISO_COMMENT", "(*", "*)")
)

var ebnfTokens = []*tokenType{
	opening("railroad-ebnf-beta", ebnfIDToken),
	literal("::="),
	literal("="),
	literal(";"),
	literal("|"),
	literal(","),
	literal("(", ebnfISOComment),
	literal(")"),
	literal("["),
	literal("]"),
	literal("{"),
	literal("}"),
	literal("?", ebnfSpecialToken),
	literal("*"),
	literal("+"),
	literal("-", ebnfHidden[1]),
	TITLE,
	ACC_TITLE,
	ACC_DESCR,
	ebnfIDToken,
	quoted("EBNF_STRING"),
	ebnfSpecialToken,
	ebnfHidden[0],
	ebnfHidden[1],
	ebnfHidden[2],
	ebnfHidden[3],
	blockComment("EBNF_BLOCK_COMMENT", "/*", "*/"),
	ebnfISOComment,
}

// The $type of a part of the syntax tree.
const (
	ebnfChoice = iota
	ebnfSequence
	ebnfTerm
	ebnfTerminal
	ebnfNonTerminal
	ebnfSpecial
	ebnfGroup
	ebnfOptional
	ebnfRepetition
)

const (
	ebnfOptionalPostfix = iota
	ebnfZeroOrMorePostfix
	ebnfOneOrMorePostfix
	ebnfExceptionPostfix
)

type ebnfPart struct {
	typ  int
	text string
	// A choice's alternatives, a sequence's elements, a term's base, or a bracket's element.
	kids      []*ebnfPart
	postfixes []ebnfPostfix
}

type ebnfPostfix struct {
	typ    int
	except *ebnfPart
}

type ebnfRule = astRule[*ebnfPart]

// One open bracket, or the rule itself at the bottom of the stack.
type ebnfFrame struct {
	choice, sequence, term *ebnfPart
	// The bracket's node, its closing token, and whether it is the right side of an exception.
	node   *ebnfPart
	close  int
	except bool
}

func ebnfNewFrame(node, choice *ebnfPart, close int, except bool) *ebnfFrame {
	sequence := &ebnfPart{typ: ebnfSequence}
	choice.kids = append(choice.kids, sequence)
	return &ebnfFrame{choice: choice, sequence: sequence, node: node, close: close, except: except}
}

var ebnfBrackets = [...]int{ebnfGroup, ebnfOptional, ebnfRepetition}

func ebnfDefinition(r *common.Reader) *ebnfPart {
	var stack []*ebnfFrame
	top := ebnfNewFrame(nil, &ebnfPart{typ: ebnfChoice}, -2, false)
	except := false
	for {
		var primary *ebnfPart
		kind := r.Kind()
		switch kind {
		case ebString:
			primary = &ebnfPart{typ: ebnfTerminal, text: unescape(r.Take())}
		case ebID:
			primary = &ebnfPart{typ: ebnfNonTerminal, text: r.Take()}
		case ebSpecial:
			primary = &ebnfPart{typ: ebnfSpecial, text: js.Trim(inner(r.Take()))}
		case ebGroup, ebOption, ebRepeat:
			r.I++
			element := &ebnfPart{typ: ebnfChoice}
			stack = append(stack, top)
			bracket := &ebnfPart{typ: ebnfBrackets[(kind-ebGroup)>>1], kids: []*ebnfPart{element}}
			top = ebnfNewFrame(bracket, element, kind+1, except)
			except = false
			continue
		default:
			r.Fail("a terminal, a name, or an opening bracket")
		}
		for {
			if except {
				top.term.postfixes = append(top.term.postfixes, ebnfPostfix{ebnfExceptionPostfix, primary})
				except = false
			} else {
				top.term = &ebnfPart{typ: ebnfTerm, kids: []*ebnfPart{primary}}
				top.sequence.kids = append(top.sequence.kids, top.term)
			}
			for {
				k := r.Kind()
				if k == ebQuestion {
					top.term.postfixes = append(top.term.postfixes, ebnfPostfix{typ: ebnfOptionalPostfix})
				} else if k == ebStar {
					top.term.postfixes = append(top.term.postfixes, ebnfPostfix{typ: ebnfZeroOrMorePostfix})
				} else if k == ebPlus {
					top.term.postfixes = append(top.term.postfixes, ebnfPostfix{typ: ebnfOneOrMorePostfix})
				} else {
					break
				}
				r.I++
			}
			k := r.Kind()
			if k == top.close {
				r.I++
				primary = top.node
				except = top.except
				top = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				continue
			}
			if k == ebMinus {
				r.I++
				except = true
			} else if k == ebComma {
				r.I++
			} else if k == ebBar {
				r.I++
				top.sequence = &ebnfPart{typ: ebnfSequence}
				top.choice.kids = append(top.choice.kids, top.sequence)
			} else if k != ebString && k != ebID && k != ebSpecial && k != ebGroup && k != ebOption && k != ebRepeat {
				if len(stack) == 0 {
					return top.choice
				}
				r.Fail("token of type '" + ebnfTokens[top.close].Name + "'")
			}
			break
		}
	}
}

func parseEbnf(src string) grammar[ebnfRule] {
	return parseGrammar(src, ebnfTokens, ebTitle, unescape, func(r *common.Reader) ebnfRule {
		name := r.Expect(ebID)
		if !r.Accept(ebEquals) {
			r.Expect(ebDefines)
		}
		rule := ebnfRule{name, ebnfDefinition(r)}
		r.Expect(ebSemicolon)
		return rule
	})
}

// ebnfParts are a term's base and the right sides of its exceptions, or a part's kids.
func ebnfParts(part *ebnfPart) []*ebnfPart {
	if part.typ != ebnfTerm {
		return part.kids
	}
	list := []*ebnfPart{part.kids[0]}
	for _, postfix := range part.postfixes {
		if postfix.except != nil {
			list = append(list, postfix.except)
		}
	}
	return list
}

func ebnfNode(part *ebnfPart, kids []*Node) *Node {
	switch part.typ {
	case ebnfChoice:
		return choice(kids)
	case ebnfSequence:
		return sequence(kids)
	case ebnfTerm:
		// Each postfix operator applies to everything before it in the term.
		result := kids[0]
		next := 1
		for _, postfix := range part.postfixes {
			switch postfix.typ {
			case ebnfOptionalPostfix:
				result = optional(result)
			case ebnfExceptionPostfix:
				result = &Node{Type: Sequence, Kids: []*Node{result, terminal("-"), kids[next]}}
				next++
			default:
				min := 0.0
				if postfix.typ == ebnfOneOrMorePostfix {
					min = 1
				}
				result = repetition(result, min, inf)
			}
		}
		return result
	case ebnfTerminal:
		return terminal(part.text)
	case ebnfNonTerminal:
		return nonterminal(part.text)
	case ebnfSpecial:
		return special(part.text)
	case ebnfGroup:
		return kids[0]
	case ebnfOptional:
		return optional(kids[0])
	default:
		return repetition(kids[0], 0, inf)
	}
}

func ebnfRules(ast grammar[ebnfRule]) []Rule { return toRules(ast, ebnfParts, ebnfNode) }

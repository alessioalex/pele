package railroad

import (
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

// What the four railroad grammars share: the title and accessibility tokens (which, unlike
// the ones in Mermaid's common grammar, do not take leading blanks), the hidden tokens, the
// statements before the first rule, string values, and the walk from a syntax tree to the rule model.

type tokenType = common.TokenType

func skipSpace(src string, i int) int {
	for i < len(src) {
		r, size := js.RuneAt(src, i)
		if !js.IsSpace(r) {
			break
		}
		i += size
	}
	return i
}

func skipBlanks(src string, i int) int {
	for i < len(src) && (src[i] == ' ' || src[i] == '\t') {
		i++
	}
	return i
}

// restOfLine is `[^\n\r]*?(?=%%)|[^\n\r]*` at p.
func restOfLine(src string, p int) int {
	stop := len(src)
	if i := strings.IndexAny(src[p:], "\n\r"); i >= 0 {
		stop = p + i
	}
	if i := strings.Index(src[p:stop], "%%"); i >= 0 {
		return p + i
	}
	return stop
}

// The braced form scans to the next closing brace. Once none is found, none exists further on
// either, so later braced descriptions fail without scanning again.
func accDescrScanner() common.Scanner {
	unclosed := math.MaxInt
	return func(src string, at int) int {
		if !strings.HasPrefix(src[at:], "accDescr") {
			return -1
		}
		n := len(src)
		i := skipBlanks(src, at+8)
		if i < n && src[i] == ':' {
			for i++; i < n; i++ {
				c := src[i]
				if c == '\n' || c == '\r' || (c == '%' && i+1 < n && src[i+1] == '%') {
					break
				}
			}
			return i
		}
		i = skipSpace(src, i)
		if i >= n || src[i] != '{' || i >= unclosed {
			return -1
		}
		close := strings.IndexByte(src[i+1:], '}')
		if close < 0 {
			unclosed = i
			return -1
		}
		return i + 1 + close + 1
	}
}

var inf = math.Inf(1)

const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// TITLE is /title(?:[\t ][^\n\r]*?(?=%%)|[\t ][^\n\r]*|)/y.
var TITLE = &tokenType{Name: "TITLE", First: "t", Match: func(src string, at int) int {
	if !strings.HasPrefix(src[at:], "title") {
		return -1
	}
	p := at + 5
	if p < len(src) && (src[p] == ' ' || src[p] == '\t') {
		return restOfLine(src, p+1)
	}
	return p
}}

// ACC_TITLE is /accTitle[\t ]*:(?:[^\n\r]*?(?=%%)|[^\n\r]*)/y.
var ACC_TITLE = &tokenType{Name: "ACC_TITLE", First: "a", Match: func(src string, at int) int {
	if !strings.HasPrefix(src[at:], "accTitle") {
		return -1
	}
	p := skipBlanks(src, at+8)
	if p >= len(src) || src[p] != ':' {
		return -1
	}
	return restOfLine(src, p+1)
}}

// ACC_DESCR is /accDescr(?:[\t ]*:([^\n\r]*?(?=%%)|[^\n\r]*)|\s*{([^}]*)})/y.
var ACC_DESCR = &tokenType{Name: "ACC_DESCR", First: "a", NewScanner: accDescrScanner}

func literal(name string, longer ...*tokenType) *tokenType {
	return &tokenType{Name: name, Literal: name, Longer: longer}
}

// opening is the diagram's keyword. A name that starts with it and goes on is a name.
func opening(word string, name *tokenType) *tokenType {
	t := common.Keyword(word)
	t.Longer = []*tokenType{name}
	return t
}

func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

// identifier is /[A-Z_a-z][\w-]*/y.
func identifier(name string) *tokenType {
	return &tokenType{Name: name, First: letters + "_", Match: func(src string, at int) int {
		if at >= len(src) || !(isWordByte(src[at]) && (src[at] < '0' || src[at] > '9')) {
			return -1
		}
		p := at + 1
		for p < len(src) && (isWordByte(src[p]) || src[p] == '-') {
			p++
		}
		return p
	}}
}

// quoted is /"([^"\\]|\\.)*"|'([^'\\]|\\.)*'/y.
func quoted(name string) *tokenType {
	return &tokenType{Name: name, First: `"'`, Match: common.QuotedEnd}
}

// hiddenTokens are whitespace, front matter, directive, and `%%` comment, in the order every
// railroad grammar lists them.
func hiddenTokens(prefix string) []*tokenType {
	yaml := *common.YAML
	yaml.Name, yaml.First = prefix+"_YAML", "-"
	directive := *common.DIRECTIVE
	directive.Name, directive.First = prefix+"_DIRECTIVE", "\t %"
	comment := *common.SINGLE_LINE_COMMENT
	comment.Name, comment.First = prefix+"_SINGLE_LINE_COMMENT", "\t %"
	return []*tokenType{
		{Name: prefix + "_WHITESPACE", Hidden: true, First: "\t \r\n", Match: func(src string, at int) int {
			p := at
			for p < len(src) && (src[p] == '\t' || src[p] == ' ' || src[p] == '\r' || src[p] == '\n') {
				p++
			}
			if p == at {
				return -1
			}
			return p
		}},
		&yaml,
		&directive,
		&comment,
	}
}

// blockComment is a hidden comment from open to close, as /open[\s\S]*?close/y.
func blockComment(name, open, close string) *tokenType {
	opener := func(src string, at int) int {
		if !strings.HasPrefix(src[at:], open) {
			return -1
		}
		return at + len(open)
	}
	return &tokenType{Name: name, Hidden: true, First: open[:1], Opener: opener, Match: func(src string, at int) int {
		p := opener(src, at)
		if p < 0 {
			return -1
		}
		i := strings.Index(src[p:], close)
		if i < 0 {
			return -1
		}
		return p + i + len(close)
	}}
}

// inner is text.slice(1, -1).
func inner(text string) string {
	if len(text) < 2 {
		return ""
	}
	return text[1 : len(text)-1]
}

// Mermaid's railroad value converters know three escapes; any other escaped character stands for itself.
func unescape(text string) string {
	if !strings.Contains(text, `\`) {
		return inner(text)
	}
	end := len(text) - 1
	var b strings.Builder
	for i := 1; i < end; i++ {
		c := text[i]
		if c == '\\' && i+1 < end {
			i++
			switch c = text[i]; c {
			case 'n':
				c = '\n'
			case 'r':
				c = '\r'
			case 't':
				c = '\t'
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

// astRule is a rule of a syntax tree.
type astRule[P any] struct {
	name       string
	definition P
}

type grammar[R any] struct {
	title, accTitle, accDescr string
	rules                     []R
}

// parseGrammar parses the keyword, then any titles and accessibility statements, then rules to
// the end of the text. Every table starts with its keyword and has TITLE, ACC_TITLE, and
// ACC_DESCR in a row from meta.
func parseGrammar[R any](src string, types []*tokenType, meta int, unquote func(string) string, rule func(r *common.Reader) R) grammar[R] {
	r := common.NewReader(common.Tokenize(src, types, "railroad"), types, "railroad")
	var ast grammar[R]
	r.Expect(0)
	for {
		kind := r.Kind()
		if kind == meta {
			title := common.TitleValue(r.Take())
			if title != "" && (title[0] == '"' || title[0] == '\'') && strings.HasSuffix(title, title[:1]) {
				ast.title = unquote(title)
			} else {
				ast.title = title
			}
		} else if kind == meta+1 {
			ast.accTitle = common.AccTitleValue(r.Take())
		} else if kind == meta+2 {
			ast.accDescr = common.AccDescrValue(r.Take())
		} else {
			break
		}
	}
	for r.Kind() != -1 {
		ast.rules = append(ast.rules, rule(r))
	}
	return ast
}

func terminal(value string) *Node   { return &Node{Type: Terminal, Text: value} }
func nonterminal(name string) *Node { return &Node{Type: Nonterminal, Text: name} }
func special(text string) *Node     { return &Node{Type: Special, Text: text} }
func optional(element *Node) *Node  { return &Node{Type: Optional, Kids: []*Node{element}} }
func repetition(element *Node, min, max float64) *Node {
	return &Node{Type: Repetition, Kids: []*Node{element}, Min: min, Max: max}
}

// A sequence or a choice of one is that one, in every notation.
func sequence(elements []*Node) *Node {
	if len(elements) == 1 {
		return elements[0]
	}
	return &Node{Type: Sequence, Kids: elements}
}

func choice(alternatives []*Node) *Node {
	if len(alternatives) == 1 {
		return alternatives[0]
	}
	return &Node{Type: Choice, Kids: alternatives}
}

func parts(node *Node) []*Node { return node.Kids }

// fold rebuilds a tree bottom-up without recursion, so that nesting depth is bounded by memory,
// not by the stack.
func fold[T, R any](root T, children func(T) []T, build func(T, []R) R) R {
	nodes := []T{root}
	lists := [][]T{children(root)}
	done := [][]R{nil}
	for {
		top := len(nodes) - 1
		results := done[top]
		if len(results) < len(lists[top]) {
			child := lists[top][len(results)]
			nodes = append(nodes, child)
			lists = append(lists, children(child))
			done = append(done, nil)
		} else {
			value := build(nodes[top], results)
			if top == 0 {
				return value
			}
			nodes = nodes[:top]
			lists = lists[:top]
			done = done[:top]
			done[top-1] = append(done[top-1], value)
		}
	}
}

// toRules turns each rule's syntax tree into the common model, given how one node is built from its parts.
func toRules[P any](ast grammar[astRule[P]], children func(P) []P, build func(P, []*Node) *Node) []Rule {
	rules := make([]Rule, len(ast.rules))
	for i, rule := range ast.rules {
		rules[i] = Rule{Name: rule.name, Definition: fold(rule.definition, children, build)}
	}
	return rules
}

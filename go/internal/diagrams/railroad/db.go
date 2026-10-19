package railroad

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

func (m *Model) setTitle(text string) { m.Title = text }

func (m *Model) setAccTitle(text string) { m.AccTitle = js.TrimStart(text) }

// setAccDescription is .replace(/\n\s+/g, '\n').
func (m *Model) setAccDescription(text string) {
	var b strings.Builder
	for i := 0; i < len(text); {
		if text[i] == '\n' {
			j := skipSpace(text, i+1)
			if j > i+1 {
				b.WriteByte('\n')
				i = j
				continue
			}
		}
		b.WriteByte(text[i])
		i++
	}
	m.AccDescr = b.String()
}

// A name defined again is drawn again.
func (m *Model) addRule(rule Rule) { m.Rules = append(m.Rules, rule) }

// notationOf matches /^\s*railroad-(ebnf-|abnf-|peg-)?beta/i. Mermaid has a detector per
// keyword, each matching without regard to case.
func notationOf(source string) (Notation, bool) {
	rest := source[skipSpace(source, 0):]
	if !js.HasPrefixFold(rest, "railroad-") {
		return "", false
	}
	rest = rest[len("railroad-"):]
	for _, infix := range [...]Notation{NotationEbnf, NotationAbnf, NotationPeg} {
		if js.HasPrefixFold(rest, string(infix)+"-") && js.HasPrefixFold(rest[len(infix)+1:], "beta") {
			return infix, true
		}
	}
	if js.HasPrefixFold(rest, "beta") {
		return NotationRailroad, true
	}
	return "", false
}

type parsed struct {
	title, accTitle, accDescr string
	rules                     []Rule
}

func parseNotation(source string, notation Notation) parsed {
	switch notation {
	case NotationEbnf:
		ast := parseEbnf(source)
		return parsed{ast.title, ast.accTitle, ast.accDescr, ebnfRules(ast)}
	case NotationAbnf:
		ast := parseAbnf(source)
		return parsed{ast.title, ast.accTitle, ast.accDescr, abnfRules(ast)}
	case NotationPeg:
		ast := parsePeg(source)
		return parsed{ast.title, ast.accTitle, ast.accDescr, pegRules(ast)}
	default:
		ast := parseIr(source)
		return parsed{ast.title, ast.accTitle, ast.accDescr, irRules(ast)}
	}
}

// populate parses the text in one notation and loads the result into the model, as Mermaid's
// four parsers do.
func populate(db *Model, source string, notation Notation) {
	ast := parseNotation(source, notation)
	db.Notation = notation
	if ast.accDescr != "" {
		db.setAccDescription(ast.accDescr)
	}
	if ast.accTitle != "" {
		db.setAccTitle(ast.accTitle)
	}
	if ast.title != "" {
		db.setTitle(ast.title)
	}
	for _, rule := range ast.rules {
		db.addRule(rule)
	}
}

func parseRailroad(source, title string) *Model {
	notation, ok := notationOf(source)
	if !ok {
		e := core.NewError("No railroad diagram keyword found.", core.CodeSyntax)
		e.Type = "railroad"
		panic(e)
	}
	db := &Model{Notation: NotationRailroad, Title: title}
	populate(db, source, notation)
	return db
}

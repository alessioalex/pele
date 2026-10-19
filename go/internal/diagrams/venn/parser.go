package venn

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

const statementNames = "'EOF', 'NEWLINE', 'TITLE', 'SET', 'UNION', 'TEXT', 'INDENT_TEXT', 'STYLE'"
const valueNames = "'NUMERIC', 'IDENTIFIER', 'STRING', 'HEXCOLOR', 'RGBCOLOR', 'RGBACOLOR'"

// parser ports Mermaid's venn grammar. A token is read only when the grammar has to look at one
// to decide, because the lexer asks the model whether an indented `text` continues a set, and
// the statements are what tell the model so.
type parser struct {
	src  string
	db   *DB
	lex  *lexer
	kind int
}

func (p *parser) peek() int {
	if p.kind == -1 {
		p.kind = p.lex.next()
	}
	return p.kind
}

func (p *parser) fail(expected string) {
	if p.peek() == tError {
		panic(core.SyntaxError("venn", p.src, p.lex.start, "", true))
	}
	panic(core.SyntaxError("venn", p.src, p.lex.start, "Expecting "+expected+", got '"+tokenNames[p.kind]+"'", false))
}

func (p *parser) take() string {
	p.kind = -1
	return p.lex.text
}

func (p *parser) expect(wanted int) string {
	if p.peek() != wanted {
		p.fail("'" + tokenNames[wanted] + "'")
	}
	return p.take()
}

func (p *parser) identifier() string {
	if p.peek() != tIdentifier && p.kind != tString {
		p.fail("'IDENTIFIER', 'STRING'")
	}
	return p.take()
}

func (p *parser) identifiers() []string {
	list := []string{p.identifier()}
	for p.peek() == tComma {
		p.take()
		list = append(list, p.identifier())
	}
	return list
}

func (p *parser) subset(ids []string, union bool) {
	var label *string
	if p.peek() == tBracketLabel {
		l := p.take()
		label = &l
	}
	var size *float64
	if p.peek() == tColon {
		p.take()
		s := js.ParseFloat(p.expect(tNumeric))
		size = &s
	}
	if union {
		if len(ids) < 2 {
			panic(core.NewError("union requires multiple identifiers", core.CodeSemantic))
		}
		p.db.validateUnionIdentifiers(ids)
	}
	p.db.addSubsetData(ids, label, size)
	p.db.indent = true
}

// text reads the id of a text, and the label in brackets that an id which is not a number may carry.
func (p *parser) text(sets func() []string) {
	kind := p.peek()
	if kind != tIdentifier && kind != tString && kind != tNumeric {
		p.fail("'NUMERIC', 'IDENTIFIER', 'STRING'")
	}
	id := p.take()
	var label *string
	if kind != tNumeric && p.peek() == tBracketLabel {
		l := p.take()
		label = &l
	}
	p.db.addTextData(sets(), id, label)
}

func (p *parser) style() {
	ids := p.identifiers()
	var styles [][2]string
	for {
		key := p.expect(tIdentifier)
		p.expect(tColon)
		var value string
		if p.peek() == tString {
			value = p.take()
		} else {
			var parts []string
			for p.peek() == tIdentifier || (p.kind >= tNumeric && p.kind <= tRGBAColor) {
				parts = append(parts, p.take())
			}
			if len(parts) == 0 {
				p.fail(valueNames)
			}
			value = strings.Join(parts, " ")
		}
		styles = append(styles, [2]string{key, value})
		if p.peek() != tComma {
			break
		}
		p.take()
	}
	p.db.addStyleData(ids, styles)
}

func parseVenn(src string, db *DB) {
	p := &parser{src: src, db: db, lex: &lexer{src: src, host: db}, kind: -1}
	for p.peek() == tNewline {
		p.take()
	}
	p.expect(tVenn)
	for {
		statement := p.peek()
		if statement == tEOF {
			return
		}
		if statement < tNewline || statement > tStyle || statement == tVenn {
			p.fail(statementNames)
		}
		p.take()
		switch statement {
		case tTitle:
			db.Title = js.SliceFrom(p.lex.text, 6)
		case tSet:
			p.subset([]string{p.identifier()}, false)
		case tUnion:
			p.subset(p.identifiers(), true)
		case tText:
			ids := p.identifiers()
			p.text(func() []string { return ids })
		case tIndentText:
			p.text(func() []string {
				if db.current == nil {
					panic(core.NewError("text requires set", core.CodeSemantic))
				}
				return db.current
			})
		case tStyle:
			p.style()
		}
	}
}

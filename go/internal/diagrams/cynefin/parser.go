package cynefin

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/diagrams/common"
)

const (
	tKeywordColon = iota
	tKeyword
	tArrow
	tColon
	tDomain
	tAccDescr
	tAccTitle
	tTitle
	tString
	tNewline
)

var domainNames = []string{"complex", "complicated", "clear", "chaotic", "confusion"}

// DOMAIN_NAME is /complex|complicated|clear|chaotic|confusion/y.
var domainName = &common.TokenType{Name: "DOMAIN_NAME", First: "c", Match: func(src string, at int) int {
	for _, name := range domainNames {
		if strings.HasPrefix(src[at:], name) {
			return at + len(name)
		}
	}
	return -1
}}

var cynefinTokens = []*common.TokenType{
	{Name: "cynefin-beta:", Literal: "cynefin-beta:"},
	common.Keyword("cynefin-beta"),
	{Name: "-->", Literal: "-->"},
	{Name: ":", Literal: ":"},
	domainName,
	common.ACC_DESCR,
	common.ACC_TITLE,
	common.TITLE,
	common.STRING,
	common.NEWLINE,
	common.WHITESPACE,
	common.YAML,
	common.DIRECTIVE,
	common.SINGLE_LINE_COMMENT,
}

type domainBlockAst struct {
	domain string
	items  []string
}

type transitionAst struct {
	from, to string
	label    string
}

type cynefinAst struct {
	title, accTitle, accDescr string
	domains                   []domainBlockAst
	transitions               []transitionAst
}

func parseCynefin(src string) *cynefinAst {
	r := common.NewReader(common.Tokenize(src, cynefinTokens, "cynefin"), cynefinTokens, "cynefin")
	ast := &cynefinAst{}

	skipLines := func() {
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
		skipLines()
	}

	skipLines()
	if r.Kind() != tKeyword && r.Kind() != tKeywordColon {
		r.Fail("'cynefin-beta'")
	}
	r.I++
	for r.Kind() != -1 {
		switch r.Kind() {
		case tNewline:
			r.I++
		case tAccDescr:
			ast.accDescr = common.AccDescrValue(r.Take())
			endOfLine()
		case tAccTitle:
			ast.accTitle = common.AccTitleValue(r.Take())
			endOfLine()
		case tTitle:
			ast.title = common.TitleValue(r.Take())
			endOfLine()
		case tDomain:
			name := r.Take()
			if r.Accept(tArrow) {
				t := transitionAst{from: name, to: r.Expect(tDomain)}
				if r.Accept(tColon) {
					t.label = common.StringValue(r.Expect(tString))
				}
				endOfLine()
				ast.transitions = append(ast.transitions, t)
				break
			}
			block := domainBlockAst{domain: name}
			skipLines()
			for r.Kind() == tString {
				block.items = append(block.items, common.StringValue(r.Take()))
				skipLines()
			}
			ast.domains = append(ast.domains, block)
		default:
			r.Fail("a domain, a transition, a title, or a line break")
		}
	}
	return ast
}

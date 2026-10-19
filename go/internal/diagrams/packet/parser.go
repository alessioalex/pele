package packet

import (
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

const (
	tPacketBeta = iota
	tPacket
	tDash
	tPlus
	tColon
	tAccDescr
	tAccTitle
	tTitle
	tInt
	tString
	tNewline
)

func isDigit(src string, i int) bool { return i < len(src) && src[i] >= '0' && src[i] <= '9' }

// matchInt is /0|[1-9][0-9]*(?!\.)/y: the lookahead gives back the last digit before a dot.
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
	end := at + 1
	for isDigit(src, end) {
		end++
	}
	if end >= len(src) || src[end] != '.' {
		return end
	}
	if end-1 > at {
		return end - 1
	}
	return -1
}

var tokens = []*common.TokenType{
	{Name: "packet-beta", Literal: "packet-beta"},
	common.Keyword("packet"),
	// A front matter block also starts with dashes, so Langium lets it win over a dash.
	{Name: "-", Literal: "-", Longer: []*common.TokenType{common.YAML}},
	{Name: "+", Literal: "+"},
	{Name: ":", Literal: ":"},
	common.ACC_DESCR,
	common.ACC_TITLE,
	common.TITLE,
	{Name: "INT", Match: matchInt, First: "0123456789"},
	common.STRING,
	common.NEWLINE,
	common.WHITESPACE,
	common.YAML,
	common.DIRECTIVE,
	common.SINGLE_LINE_COMMENT,
}

// blockAst is a field as written. The has flags stand for the numbers that were given.
type blockAst struct {
	start, end, bits          float64
	hasStart, hasEnd, hasBits bool
	label                     string
}

type ast struct {
	title, accTitle, accDescr string
	blocks                    []blockAst
}

func parsePacket(src string) ast {
	r := common.NewReader(common.Tokenize(src, tokens, "packet"), tokens, "packet")
	var a ast

	endOfLine := func() {
		if r.Kind() == -1 {
			return
		}
		if r.Kind() != tNewline {
			r.Fail("a line break or the end of input")
		}
		for r.Kind() == tNewline {
			r.I++
		}
	}

	for r.Kind() == tNewline {
		r.I++
	}
	if !r.Accept(tPacketBeta) {
		r.Expect(tPacket)
	}
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
		case tInt, tPlus:
			var b blockAst
			if r.Accept(tPlus) {
				b.bits, b.hasBits = js.ToNumber(r.Expect(tInt)), true
			} else {
				b.start, b.hasStart = js.ToNumber(r.Take()), true
				if r.Accept(tDash) {
					b.end, b.hasEnd = js.ToNumber(r.Expect(tInt)), true
				}
			}
			r.Expect(tColon)
			b.label = common.StringValue(r.Expect(tString))
			a.blocks = append(a.blocks, b)
			endOfLine()
		default:
			r.Fail("a field, a title, or a line break")
		}
	}
	return a
}

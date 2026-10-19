package pie

import (
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

const (
	tShowData = iota
	tPie
	tColon
	tNumber
	tAccDescr
	tAccTitle
	tTitle
	tString
	tNewline
)

// charAt is src.charCodeAt(i) for ASCII tests, with -1 past the end.
func charAt(src string, i int) int {
	if i < 0 || i >= len(src) {
		return -1
	}
	return int(src[i])
}

func digits(src string, from int) int {
	i := from
	for c := charAt(src, i); c >= '0' && c <= '9'; c = charAt(src, i) {
		i++
	}
	return i
}

// numberScanner answers as NUMBER_PIE's pattern does. The pattern reads a whole run of digits
// before it finds there is no fraction, and would do so again from every digit of a long run of
// zeros.
func numberScanner() common.Scanner {
	from, end, fraction := 0, 0, -1
	return func(src string, p int) int {
		at := p
		if charAt(src, p) == '-' {
			at = p + 1
		}
		if at < from || at >= end {
			from = at
			end = digits(src, at)
			fraction = -1
			if end > at && charAt(src, end) == '.' {
				stop := digits(src, end+1)
				length := stop - end - 1
				if length > 0 && charAt(src, stop) != '.' {
					fraction = stop
				} else if length > 1 {
					fraction = stop - 1
				}
			}
		}
		if end == at {
			return -1
		}
		if fraction >= 0 {
			return fraction
		}
		if charAt(src, at) == '0' {
			if charAt(src, at+1) == '.' {
				return -1
			}
			return at + 1
		}
		if charAt(src, end) != '.' {
			return end
		}
		if end-1 > at {
			return end - 1
		}
		return -1
	}
}

var tokens = []*common.TokenType{
	common.Keyword("showData"),
	common.Keyword("pie"),
	{Name: ":", Literal: ":"},
	{Name: "NUMBER_PIE", NewScanner: numberScanner, First: "-0123456789"},
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

type section struct {
	label string
	value float64
}

type ast struct {
	showData                  bool
	title, accTitle, accDescr string
	sections                  []section
}

func parsePie(src string) ast {
	r := common.NewReader(common.Tokenize(src, tokens, "pie"), tokens, "pie")
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
	r.Expect(tPie)
	a.showData = r.Accept(tShowData)
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
		case tString:
			label := common.StringValue(r.Take())
			r.Expect(tColon)
			value := js.ToNumber(r.Expect(tNumber))
			a.sections = append(a.sections, section{label, value})
			endOfLine()
		default:
			r.Fail("a section, a title, or a line break")
		}
	}
	return a
}

package journey

import (
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

const (
	tEnd = iota
	tEOF
	tInvalid
	tNewline
	tJourney
	tTitle
	tAccTitle
	tAccTitleValue
	tAccDescr
	tAccDescrValue
	tAccDescrMultilineValue
	tSection
	tTaskName
	tTaskData
	tColon
)

var tokenNames = []string{
	"$end", "EOF", "INVALID", "NEWLINE", "journey", "title", "acc_title", "acc_title_value", "acc_descr",
	"acc_descr_value", "acc_descr_multiline_value", "section", "taskName", "taskData", ":",
}

// textEnd: `#`, a line break, and `;` end every kind of text; a colon also ends a task name or a section.
func textEnd(src string, from int, colon bool) int {
	e := from
	for ; e < len(src); e++ {
		c := src[e]
		if c == '#' || c == '\n' || c == ';' || (colon && c == ':') {
			break
		}
	}
	return e
}

// spaceAfter returns the size of the whitespace character at p, or 0.
func spaceAfter(src string, p int) int {
	r, size := js.RuneAt(src, p)
	if p < len(src) && js.IsSpace(r) {
		return size
	}
	return 0
}

func tokenize(src string) common.ScanTokens {
	n := len(src)
	var out common.ScanTokens
	p := 0
	emit := func(typ, e int) {
		out.Types = append(out.Types, typ)
		out.Texts = append(out.Texts, src[p:e])
		out.Starts = append(out.Starts, p)
		p = e
	}
	for {
		p = common.SkipTrivia(src, p)
		if p >= n {
			break
		}
		c := src[p]
		e := p + 1
		if c == '\n' {
			for e < n && src[e] == '\n' {
				e++
			}
			emit(tNewline, e)
			continue
		}
		if common.KeywordAt(src, p, "journey") && !(p+7 < n && js.IsWord(rune(src[p+7]))) {
			emit(tJourney, p+7)
			continue
		}
		// `\s` after a keyword also takes a line break, so `title` alone on a line takes the next line as its text.
		if common.KeywordAt(src, p, "title") {
			if size := spaceAfter(src, p+5); size > 0 {
				if e = textEnd(src, p+5+size, false); e > p+5+size {
					emit(tTitle, e)
					continue
				}
			}
		}
		if e = common.Accessibility(src, p, &out, tAccTitle); e != p {
			p = e
			continue
		}
		if common.KeywordAt(src, p, "section") {
			if size := spaceAfter(src, p+7); size > 0 {
				if e = textEnd(src, p+7+size, true); e > p+7+size {
					emit(tSection, e)
					continue
				}
			}
		}
		switch c {
		case ':':
			if e = textEnd(src, p+1, false); e > p+1 {
				emit(tTaskData, e)
			} else {
				emit(tColon, p+1)
			}
		case ';':
			emit(tInvalid, p+1)
		default:
			emit(tTaskName, textEnd(src, p+1, true))
		}
	}
	// Jison marks its lexer done when a rule runs on empty input, so an accessibility value that
	// matches nothing at the very end of the text is followed by no EOF token.
	if len(out.Starts) == 0 || out.Starts[len(out.Starts)-1] < n {
		emit(tEOF, n)
	}
	emit(tEnd, n)
	return out
}

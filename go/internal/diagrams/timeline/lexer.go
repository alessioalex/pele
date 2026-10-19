package timeline

import (
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

const (
	tEND = iota
	tEOF
	tINVALID
	tNEWLINE
	tTimeline
	tTimelineLR
	tTimelineTD
	tTitle
	tAccTitle
	tAccTitleValue
	tAccDescr
	tAccDescrValue
	tAccDescrMultilineValue
	tSection
	tPeriod
	tEvent
)

var tokenNames = []string{
	"$end",
	"EOF",
	"INVALID",
	"NEWLINE",
	"timeline",
	"timeline_lr",
	"timeline_td",
	"title",
	"acc_title",
	"acc_title_value",
	"acc_descr",
	"acc_descr_value",
	"acc_descr_multiline_value",
	"section",
	"period",
	"event",
}

func byteAt(s string, i int) int {
	if i < 0 || i >= len(s) {
		return -1
	}
	return int(s[i])
}

// spaceAt reports whether a `\s` character is at byte p, and its size in bytes.
func spaceAt(s string, p int) (bool, int) {
	if p >= len(s) {
		return false, 0
	}
	r, size := js.RuneAt(s, p)
	return js.IsSpace(r), size
}

func wordAt(s string, p int) bool {
	c := byteAt(s, p)
	return c >= 0 && js.IsWord(rune(c))
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
			for byteAt(src, e) == '\n' {
				e++
			}
			emit(tNEWLINE, e)
			continue
		}
		if common.KeywordAt(src, p, "timeline") {
			e = p + 8
			for byteAt(src, e) == ' ' || byteAt(src, e) == '\t' {
				e++
			}
			lr := common.KeywordAt(src, e, "lr")
			if e > p+8 && (lr || common.KeywordAt(src, e, "td")) && !wordAt(src, e+2) {
				if lr {
					emit(tTimelineLR, e+2)
				} else {
					emit(tTimelineTD, e+2)
				}
				continue
			}
			if !wordAt(src, p+8) {
				emit(tTimeline, p+8)
				continue
			}
		}
		// `\s` after a keyword also takes a line break, so `title` alone on a line takes the next line as its text.
		if common.KeywordAt(src, p, "title") {
			if ok, size := spaceAt(src, p+5); ok && p+5+size < n && src[p+5+size] != '\n' {
				emit(tTitle, common.LineEnd(src, p+5+size))
				continue
			}
		}
		e = common.Accessibility(src, p, &out, tAccTitle)
		if e != p {
			p = e
			continue
		}
		if common.KeywordAt(src, p, "section") {
			if ok, size := spaceAt(src, p+7); ok {
				start := p + 7 + size
				e = start
				for e < n && src[e] != ':' && src[e] != '\n' {
					e++
				}
				if e > start {
					emit(tSection, e)
					continue
				}
			}
		}
		if c != ':' {
			e = p + 1
			for e < n && src[e] != '#' && src[e] != ':' && src[e] != '\n' {
				e++
			}
			emit(tPeriod, e)
			continue
		}
		// An event runs to the end of the line or to a colon that is followed by whitespace.
		if ok, size := spaceAt(src, p+1); ok {
			start := p + 1 + size
			e = start
			for ; e < n; e++ {
				k := src[e]
				if k == '\n' {
					break
				}
				if k == ':' {
					if sp, _ := spaceAt(src, e+1); sp {
						break
					}
				}
			}
			if e > start {
				emit(tEvent, e)
				continue
			}
		}
		emit(tINVALID, p+1)
	}

	// Jison marks its lexer done when a rule runs on empty input, so an accessibility value that
	// matches nothing at the very end of the text is followed by no EOF token.
	if len(out.Starts) == 0 || out.Starts[len(out.Starts)-1] < n {
		emit(tEOF, n)
	}
	emit(tEND, n)
	return out
}

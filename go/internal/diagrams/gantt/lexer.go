package gantt

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

type tokens struct {
	types  []int
	texts  []string
	starts []int
}

const (
	sInitial = iota
	sAccTitle
	sAccDescr
	sAccDescrMultiline
	sHref
	sCallbackname
	sCallbackargs
	sClick
)

var days = []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}

func spaceAt(src string, p int) int {
	if r, size := js.RuneAt(src, p); p < len(src) && js.IsSpace(r) {
		return size
	}
	return 0
}

func spaces(src string, p int) int {
	for {
		size := spaceAt(src, p)
		if size == 0 {
			return p
		}
		p += size
	}
}

func wordEnd(src string, p int) bool { return p >= len(src) || !js.IsWord(rune(src[p])) }

// headed matches /word\s*:\s*/i, /word\s*\{\s*/i and the like at p: the word, blanks, the mark,
// blanks. A mark of 0 is none.
func headed(src string, p int, word string, mark byte, blanksAfter bool) int {
	if !common.WordAt(src, p, word) {
		return -1
	}
	q := spaces(src, p+len(word))
	if mark != 0 {
		if q >= len(src) || src[q] != mark {
			return -1
		}
		q++
	}
	if blanksAfter {
		q = spaces(src, q)
	}
	return q
}

// statement matches /word\s[^stops]+/i at p.
func statement(src string, p int, word, stops string) int {
	if !common.WordAt(src, p, word) {
		return -1
	}
	size := spaceAt(src, p+len(word))
	if size == 0 {
		return -1
	}
	q := p + len(word) + size
	e := q
	for e < len(src) && strings.IndexByte(stops, src[e]) < 0 {
		e++
	}
	if e == q {
		return -1
	}
	return e
}

// bounded matches /word\b/i at p.
func bounded(src string, p int, word string) int {
	if common.WordAt(src, p, word) && wordEnd(src, p+len(word)) {
		return p + len(word)
	}
	return -1
}

// choice matches /\s+(one|two|…)\b/i after word at p, returning the end and the index of the choice.
func choice(src string, p int, word string, options []string) (int, int) {
	if !common.WordAt(src, p, word) {
		return -1, -1
	}
	q := spaces(src, p+len(word))
	if q == p+len(word) {
		return -1, -1
	}
	for i, o := range options {
		if common.WordAt(src, q, o) && wordEnd(src, q+len(o)) {
			return q + len(o), i
		}
	}
	return -1, -1
}

type rule struct {
	match func(src string, p int) int
	typ   int
}

func stmt(word, stops string, typ int) rule {
	return rule{func(src string, p int) int { return statement(src, p, word, stops) }, typ}
}

func bound(word string, typ int) rule {
	return rule{func(src string, p int) int { return bounded(src, p, word) }, typ}
}

// Statements that start with a keyword, grouped by the keyword's first letter and kept in the
// grammar's rule order.
var keywords = map[byte][]rule{
	'a': {stmt("axisformat", "#\n;", tAxisFormat), stmt("accdescription", "#\n;", tAccDescription)},
	'g': {bound("gantt", tGantt)},
	'd': {stmt("dateformat", "#\n;", tDateFormat)},
	'i': {bound("inclusiveenddates", tInclusiveEndDates), stmt("includes", "#\n;", tIncludes)},
	't': {bound("topaxis", tTopAxis), stmt("tickinterval", "#\n;", tTickInterval), stmt("todaymarker", "\n;", tTodayMarker), stmt("title", "\n", tTitle)},
	'e': {stmt("excludes", "#\n;", tExcludes)},
	's': {stmt("section", "\n", tSection)},
}

// isDate matches /\d\d\d\d-\d\d-\d\d\b/ at p.
func isDate(src string, p int) bool {
	if p+10 > len(src) {
		return false
	}
	for i := 0; i < 10; i++ {
		c := src[p+i]
		if i == 4 || i == 7 {
			if c != '-' {
				return false
			}
		} else if c < '0' || c > '9' {
			return false
		}
	}
	return wordEnd(src, p+10)
}

func tokenize(src string) tokens {
	n := len(src)
	var out tokens
	state := sInitial
	p := 0
	emit := func(typ, s, e int) {
		out.types = append(out.types, typ)
		out.texts = append(out.texts, src[s:e])
		out.starts = append(out.starts, s)
		p = e
	}
	until := func(ch byte, from int) int {
		if from > n {
			return n
		}
		if e := strings.IndexByte(src[from:], ch); e >= 0 {
			return from + e
		}
		return n
	}
	byteAt := func(i int) int {
		if i < n {
			return int(src[i])
		}
		return -1
	}

	keyword := func(c byte) bool {
		e := -1
		switch c | 32 {
		case 'a':
			if e = headed(src, p, "acctitle", ':', true); e >= 0 {
				state = sAccTitle
				emit(tAccTitle, p, e)
				return true
			}
			if e = headed(src, p, "accdescr", ':', true); e >= 0 {
				state = sAccDescr
				emit(tAccDescr, p, e)
				return true
			}
			if e = headed(src, p, "accdescr", '{', true); e >= 0 {
				state = sAccDescrMultiline
				p = e
				return true
			}
		case 'h':
			if e = headed(src, p, "href", 0, false); e > p+4 && e < n && src[e] == '"' {
				state = sHref
				p = e + 1
				return true
			}
		case 'c':
			if e = headed(src, p, "call", 0, false); e > p+4 {
				state = sCallbackname
				p = e
				return true
			}
			if e = headed(src, p, "click", 0, false); e > p+5 {
				state = sClick
				p = e
				return true
			}
		case 'w':
			if end, i := choice(src, p, "weekday", days); end >= 0 {
				emit(tWeekdayMonday+i, p, end)
				return true
			}
			if end, i := choice(src, p, "weekend", []string{"friday", "saturday"}); end >= 0 {
				emit(tWeekendFriday+i, p, end)
				return true
			}
		}
		for _, r := range keywords[c|32] {
			if e = r.match(src, p); e >= 0 {
				emit(r.typ, p, e)
				return true
			}
		}
		return false
	}

	for {
		if p >= n {
			typ := tEOF
			switch state {
			case sAccTitle:
				typ = tAccTitleValue
			case sAccDescr:
				typ = tAccDescrValue
			case sAccDescrMultiline:
				typ = tAccDescrMultilineValue
			case sHref:
				typ = tHref
			case sCallbackname:
				typ = tCallbackname
			case sCallbackargs:
				typ = tCallbackargs
			case sClick:
				typ = tClick
			}
			emit(typ, n, n)
			break
		}
		c := src[p]
		switch state {
		case sInitial:
			_, size := js.RuneAt(src, p)
			d := byteAt(p + 1)
			switch {
			case c == '%':
				if d != '%' {
					e := until('\n', p+1)
					for byteAt(e) == '\n' {
						e++
					}
					p = e
				} else if byteAt(p+2) == '{' {
					// The grammar enters a lexer state it never defines, so nothing can follow.
					emit(tOpenDirective, p, p+3)
					emit(tError, p, p)
					return out
				} else {
					p = until('\n', p+2)
				}
			case c != '}' && byteAt(p+size) == '%':
				// Any character but `}` followed by `%` starts a comment that runs to the end of the line.
				p = until('\n', p+size+1)
			case c == '\n':
				e := p + 1
				for byteAt(e) == '\n' {
					e++
				}
				emit(tNL, p, e)
			case spaceAt(src, p) > 0:
				p = spaces(src, p)
			case keyword(c):
			case c >= '0' && c <= '9' && isDate(src, p):
				emit(tDate, p, p+10)
			case c != ':':
				e := p + 1
				for e < n && src[e] != ':' && src[e] != '\n' {
					e++
				}
				emit(tTaskTxt, p, e)
			default:
				e := p + 1
				for e < n && src[e] != '#' && src[e] != '\n' && src[e] != ';' {
					e++
				}
				if e > p+1 {
					emit(tTaskData, p, e)
				} else {
					emit(tColon, p, e)
				}
			}
		case sAccTitle, sAccDescr:
			typ := tAccDescrValue
			if state == sAccTitle {
				typ = tAccTitleValue
			}
			state = sInitial
			emit(typ, p, until('\n', p))
		case sAccDescrMultiline:
			if c == '}' {
				state = sInitial
				p++
			} else {
				emit(tAccDescrMultilineValue, p, until('}', p))
			}
		case sHref:
			if c == '"' {
				state = sInitial
				p++
			} else {
				emit(tHref, p, until('"', p))
			}
		case sCallbackname:
			if c == '(' {
				if e := spaces(src, p+1); e < n && src[e] == ')' {
					state = sInitial
					p = e + 1
				} else {
					state = sCallbackargs
					p++
				}
			} else {
				emit(tCallbackname, p, until('(', p))
			}
		case sCallbackargs:
			if c == ')' {
				state = sInitial
				p++
			} else {
				emit(tCallbackargs, p, until(')', p))
			}
		case sClick:
			if size := spaceAt(src, p); size > 0 {
				state = sInitial
				p += size
			} else {
				e := p + 1
				for e < n && spaceAt(src, e) == 0 {
					e++
				}
				emit(tClick, p, e)
			}
		}
	}
	out.types = append(out.types, tEnd)
	out.texts = append(out.texts, "")
	out.starts = append(out.starts, n)
	return out
}

package common

import "strings"

// headEnd matches /[\t ]*accDescr(?:[\t ]*:(?:[^\n\r]*?(?=%%)|[^\n\r]*)|\s*({))/y and reports
// whether the braced form matched.
func headEnd(src string, at int) (int, bool) {
	p := skipBlanks(src, at)
	if !strings.HasPrefix(src[p:], "accDescr") {
		return -1, false
	}
	p += len("accDescr")
	if q := skipBlanks(src, p); q < len(src) && src[q] == ':' {
		return restOfLine(src, q+1), false
	}
	q := skipJSSpace(src, p)
	if q < len(src) && src[q] == '{' {
		return q + 1, true
	}
	return -1, false
}

// AccDescr is the common accDescr terminal for grammars where lexing goes on after an
// `accDescr {` that never closes, because `accDescr` is also a name there. Each one would
// otherwise read to the end of the text. The braced form scans for its closing brace; once none
// is found there is none further on.
func AccDescr(name string) *TokenType {
	t := *ACC_DESCR
	t.Name = name
	t.NewScanner = func() Scanner {
		unclosed := false
		return func(src string, at int) int {
			end, braced := headEnd(src, at)
			if end < 0 {
				return -1
			}
			if !braced {
				return end
			}
			if unclosed {
				return -1
			}
			close := strings.IndexByte(src[end:], '}')
			unclosed = close == -1
			if close == -1 {
				return -1
			}
			return end + close + 1
		}
	}
	return &t
}

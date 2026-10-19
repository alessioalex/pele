package xychart

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

type tokenList struct {
	types  []int
	texts  []string
	starts []int
}

const (
	sInitial = iota
	sAxisData
	sAxisBandData
	sData
	sDataInner
	sString
	sAccTitle
	sAccDescr
	sAccDescrMultiline
)

// The token each state gives for the empty rest of the input.
var atEnd = []int{tEOF, tEOF, tEOF, tEOF, tEOF, tStr, tAccTitleValue, tAccDescrValue, tAccDescrMultilineValue}

// Mermaid's grammar means to open a markdown string at `"` followed by a backtick, but its quoting turns
// the rule into one that matches this text and leaves the current state. Markdown strings are never read.
var brokenRule = "`)" + strings.Repeat(" ", 36) + "{ this.pushstate(md_string); }\n<md_string>(?:(?!`\").)+" +
	strings.Repeat(" ", 18) + "{ return md_str; }\n<md_string>(?:`"

var single [128]int

func init() {
	for _, pair := range []struct {
		char byte
		typ  int
	}{
		{':', tColon},
		{'+', tPlus},
		{',', tComma},
		{'=', tEquals},
		{'*', tMult},
		{'#', tBrkt},
		{'_', tUnderscore},
		{'.', tDot},
		{'&', tAmp},
		{'-', tMinus},
		{';', tSemi},
	} {
		single[pair.char] = pair.typ
	}
}

// byteAt is charCodeAt for ASCII, with -1 past the end.
func byteAt(src string, i int) int {
	if i >= 0 && i < len(src) {
		return int(src[i])
	}
	return -1
}

func isDigit(c int) bool  { return c >= '0' && c <= '9' }
func isLetter(c int) bool { return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') }
func isWord(c int) bool   { return isLetter(c) || isDigit(c) || c == '_' }

// spaceAt reports whether the character at byte offset q is `\s`, and its length.
func spaceAt(src string, q int) (bool, int) {
	c := src[q]
	if c < 0x80 {
		return c == ' ' || (c >= '\t' && c <= '\r'), 1
	}
	r, size := js.RuneAt(src, q)
	return js.IsSpace(r), size
}

func tokenize(src string) tokenList {
	n := len(src)
	var out tokenList
	var stack []int
	state := sInitial
	p := 0

	emit := func(typ, s, e int) {
		out.types = append(out.types, typ)
		out.texts = append(out.texts, src[s:e])
		out.starts = append(out.starts, s)
		p = e
	}
	push := func(s int) {
		stack = append(stack, state)
		state = s
	}
	pop := func() {
		if len(stack) > 0 {
			state = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
		}
	}
	indexFrom := func(q int, b byte) int {
		if q >= n {
			return n
		}
		e := strings.IndexByte(src[q:], b)
		if e == -1 {
			return n
		}
		return q + e
	}
	run := func(q int) int {
		for q < n {
			ok, size := spaceAt(src, q)
			if !ok {
				break
			}
			q += size
		}
		return q
	}
	digits := func(q int) int {
		for isDigit(byteAt(src, q)) {
			q++
		}
		return q
	}
	keyword := func(word string) bool {
		return common.WordAt(src, p, word) && !isWord(byteAt(src, p+len(word)))
	}

	for p < n {
		c := int(src[p])

		if state == sString {
			if c == '"' {
				p++
				pop()
			} else {
				emit(tStr, p, indexFrom(p, '"'))
			}
			continue
		}
		if state == sAccTitle || state == sAccDescr {
			emit(atEnd[state], p, indexFrom(p, '\n'))
			pop()
			continue
		}
		if state == sAccDescrMultiline {
			if c == '}' {
				p++
				pop()
			} else {
				emit(tAccDescrMultilineValue, p, indexFrom(p, '}'))
			}
			continue
		}

		next := byteAt(src, p+1)
		if c == '%' && next == '%' {
			p = indexFrom(p+2, '\n')
			continue
		}
		// Mermaid's second comment rule takes any character before `%%` with it, a line break included.
		if c != '}' {
			after := p + 1
			if c >= 0x80 {
				r, size := js.RuneAt(src, p)
				after = p + size
				if r > 0xffff {
					after = -1
				}
			}
			if after != -1 && byteAt(src, after) == '%' && byteAt(src, after+1) == '%' {
				p = indexFrom(after+2, '\n')
				continue
			}
		}
		if c == '\n' || c == '\r' {
			// Only a line break that directly follows an axis or a series ends it; one after spaces is skipped
			// with them, and the state carries on to the next line.
			if (state == sAxisData || state == sData) && (c == '\n' || next == '\n') {
				if c == '\n' {
					emit(tNEWLINE, p, p+1)
				} else {
					emit(tNEWLINE, p, p+2)
				}
				pop()
				continue
			}
			e := p + 1
			for d := byteAt(src, e); d == '\n' || d == '\r'; d = byteAt(src, e) {
				e++
			}
			emit(tNEWLINE, p, e)
			continue
		}

		switch c | 32 {
		case 't':
			if keyword("title") {
				emit(tTitle, p, p+5)
				continue
			}
		case 'a':
			if common.WordAt(src, p, "acc") {
				descr := common.WordAt(src, p+3, "descr")
				if descr || common.WordAt(src, p+3, "title") {
					k := run(p + 8)
					d := byteAt(src, k)
					if d == ':' {
						if descr {
							emit(tAccDescr, p, run(k+1))
							push(sAccDescr)
						} else {
							emit(tAccTitle, p, run(k+1))
							push(sAccTitle)
						}
						continue
					}
					if d == '{' && descr {
						p = run(k + 1)
						push(sAccDescrMultiline)
						continue
					}
				}
			}
		case 'x':
			if keyword("xychart-beta") {
				emit(tXYCHART, p, p+12)
				continue
			}
			if keyword("xychart") {
				emit(tXYCHART, p, p+7)
				continue
			}
			if keyword("x-axis") {
				emit(tXAxis, p, p+6)
				push(sAxisData)
				continue
			}
		case 'y':
			if keyword("y-axis") {
				emit(tYAxis, p, p+6)
				push(sAxisData)
				continue
			}
		case 'v':
			if common.WordAt(src, p, "vertical") {
				emit(tChartOrientation, p, p+8)
				continue
			}
		case 'h':
			if common.WordAt(src, p, "horizontal") {
				emit(tChartOrientation, p, p+10)
				continue
			}
		case 'l':
			if keyword("line") {
				emit(tLine, p, p+4)
				push(sData)
				continue
			}
		case 'b':
			if keyword("bar") {
				emit(tBar, p, p+3)
				push(sData)
				continue
			}
		}

		if c == '[' {
			if state == sAxisData {
				push(sAxisBandData)
			} else if state == sData {
				push(sDataInner)
			}
			emit(tSquareBracesStart, p, p+1)
			continue
		}
		if c == ']' {
			if state == sDataInner || state == sAxisBandData {
				pop()
			}
			emit(tSquareBracesEnd, p, p+1)
			continue
		}
		if state == sAxisData && c == '-' && next == '-' && byteAt(src, p+2) == '>' {
			emit(tArrowDelimiter, p, p+3)
			continue
		}
		if state == sAxisData || state == sDataInner {
			q := p
			if c == '+' || c == '-' {
				q = p + 1
			}
			d := byteAt(src, q)
			e := -1
			if isDigit(d) {
				e = digits(q + 1)
				if byteAt(src, e) == '.' && isDigit(byteAt(src, e+1)) {
					e = digits(e + 2)
				}
			} else if d == '.' && isDigit(byteAt(src, q+1)) {
				e = digits(q + 2)
			}
			if e != -1 {
				emit(tNumberWithDecimal, p, e)
				continue
			}
		}
		if c == '`' && common.WordAt(src, p, brokenRule) {
			p += len(brokenRule)
			pop()
			continue
		}
		if c == '"' {
			p++
			push(sString)
			continue
		}
		if isLetter(c) {
			e := p + 1
			for isLetter(byteAt(src, e)) {
				e++
			}
			emit(tAlpha, p, e)
			continue
		}
		if isDigit(c) {
			emit(tNum, p, digits(p+1))
			continue
		}
		if ok, size := spaceAt(src, p); ok {
			p = run(p + size)
			continue
		}
		if c < 0x80 && single[c] != 0 {
			emit(single[c], p, p+1)
			continue
		}
		break
	}

	if p < n {
		out.types = append(out.types, tERROR)
		out.texts = append(out.texts, "")
		out.starts = append(out.starts, p)
		return out
	}
	// Jison tries the rules of the current state once more against the empty rest of the input.
	out.types = append(out.types, atEnd[state], tEND)
	out.texts = append(out.texts, "", "")
	out.starts = append(out.starts, n, n)
	return out
}

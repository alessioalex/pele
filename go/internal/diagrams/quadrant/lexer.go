package quadrant

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
	sTitle
	sAccTitle
	sAccDescr
	sAccDescrMultiline
	sString
	sMdString
	sClassName
	sPointStart
	sPointX
	sPointY
)

// The token each state gives for the empty rest of the input, where it gives one.
var atEnd = []int{tEOF, tTitleValue, tAccTitleValue, tAccDescrValue, tAccDescrMultilineValue, tStr}

var single [128]int

func init() {
	for _, pair := range []struct {
		chars string
		typ   int
	}{
		{":", tColon},
		{"+", tPlus},
		{",", tComma},
		{"=", tEquals},
		{"*", tMult},
		{"#", tBrkt},
		{"_", tUnderscore},
		{".", tDot},
		{"&", tAmp},
		{"-", tMinus},
		{" \t\v\f", tSpace},
		{";", tSemi},
		{"!$%'`?\\/", tPunctuation},
	} {
		for k := 0; k < len(pair.chars); k++ {
			single[pair.chars[k]] = pair.typ
		}
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

// spaceAt reports whether the character at byte offset q is `\s`.
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
	// The last run scanned of each kind, so that a long run is not scanned again from each of its characters.
	spaceFrom, spaceTo := 0, 0
	dashFrom, dashTo := 0, 0
	wsFrom, wsTo := 0, 0
	gapFrom, gapTo := 0, 0

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
	lineEnd := func(q int) int {
		if q >= n {
			return n
		}
		e := strings.IndexByte(src[q:], '\n')
		if e == -1 {
			return n
		}
		return q + e
	}
	indexFrom := func(q int, b byte) int {
		e := strings.IndexByte(src[q:], b)
		if e == -1 {
			return n
		}
		return q + e
	}
	spaces := func(q int) int {
		for byteAt(src, q) == ' ' {
			q++
		}
		return q
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
	// Mermaid's `(1)|(0(.\d+)?)`, where the dot is any character within a line.
	coordinate := func(q int) int {
		c := byteAt(src, q)
		if c == '1' {
			return q + 1
		}
		if c != '0' {
			return -1
		}
		e := q + 1
		if e < n {
			r, size := js.RuneAt(src, e)
			// An astral character is two code units, the second of which is no digit.
			if !js.IsLineEnd(r) && r <= 0xffff && isDigit(byteAt(src, e+size)) {
				e += size + 1
				for isDigit(byteAt(src, e)) {
					e++
				}
			}
		}
		return e
	}

lexing:
	for p < n {
		c := int(src[p])
		next := byteAt(src, p+1)

		switch state {
		case sTitle, sAccTitle, sAccDescr:
			emit(atEnd[state], p, lineEnd(p))
			pop()
			continue

		case sAccDescrMultiline:
			if c == '}' {
				p++
				pop()
			} else {
				emit(tAccDescrMultilineValue, p, indexFrom(p, '}'))
			}
			continue

		case sString:
			if c == '"' {
				p++
				pop()
			} else {
				emit(tStr, p, indexFrom(p, '"'))
			}
			continue

		case sMdString:
			if c == '`' && next == '"' {
				p += 2
				pop()
				continue
			}
			if c != '`' && c != '"' {
				e := p + 1
				for e < n && src[e] != '`' && src[e] != '"' {
					e++
				}
				emit(tMdStr, p, e)
				continue
			}
			break lexing

		case sClassName:
			if isWord(c) {
				e := p + 1
				for isWord(byteAt(src, e)) {
					e++
				}
				emit(tClassName, p, e)
				pop()
				continue
			}
			break lexing

		case sPointStart:
			if e := coordinate(p); e != -1 {
				emit(tPointX, p, e)
				push(sPointX)
				continue
			}
			k := run(p)
			if byteAt(src, k) == ']' {
				p = spaces(k + 1)
				pop()
				continue
			}
			break lexing

		case sPointX:
			k := run(p)
			if byteAt(src, k) == ',' {
				p = run(k + 1)
				state = sPointY
				continue
			}
			break lexing

		case sPointY:
			if e := coordinate(p); e != -1 {
				emit(tPointY, p, e)
				pop()
				continue
			}
			break lexing
		}

		if c == '%' && next == '%' {
			p = lineEnd(p + 2)
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
				p = lineEnd(after + 2)
				continue
			}
		}
		if c == '\n' || c == '\r' {
			e := p + 1
			for d := byteAt(src, e); d == '\n' || d == '\r'; d = byteAt(src, e) {
				e++
			}
			emit(tNEWLINE, p, e)
			continue
		}

		lower := c | 32
		if lower == 't' && common.WordAt(src, p, "title") && !isWord(byteAt(src, p+5)) {
			emit(tTitle, p, p+5)
			push(sTitle)
			continue
		}
		if lower == 'a' && common.WordAt(src, p, "acc") {
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

		// Axis, quadrant and chart keywords and the axis arrow take the spaces around them.
		j := p
		if c == ' ' {
			if p < spaceFrom || p >= spaceTo {
				spaceFrom = p
				spaceTo = spaces(p)
			}
			j = spaceTo
		}
		d := byteAt(src, j)
		dl := d | 32
		typ := 0
		end := 0
		if dl == 'x' && common.WordAt(src, j, "x-axis") {
			typ = tXAxis
			end = j + 6
		} else if dl == 'y' && common.WordAt(src, j, "y-axis") {
			typ = tYAxis
			end = j + 6
		} else if d == '-' {
			if j < dashFrom || j >= dashTo {
				dashFrom = j
				dashTo = j
				for byteAt(src, dashTo) == '-' {
					dashTo++
				}
			}
			if dashTo-j >= 2 && byteAt(src, dashTo) == '>' {
				typ = tDelimiter
				end = dashTo + 1
			}
		} else if dl == 'q' && common.WordAt(src, j, "quadrant") {
			q := byteAt(src, j+9)
			if byteAt(src, j+8) == '-' && q >= '1' && q <= '4' {
				typ = tQuadrant1 + q - '1'
				end = j + 10
			} else if common.WordAt(src, j+8, "chart") {
				typ = tQuadrant
				end = j + 13
			}
		}
		if typ != 0 {
			emit(typ, p, spaces(end))
			continue
		}

		if lower == 'c' && common.WordAt(src, p, "classdef") && !isWord(byteAt(src, p+8)) {
			emit(tClassdef, p, p+8)
			continue
		}
		if c == '"' {
			if next == '`' {
				p += 2
				push(sMdString)
			} else {
				p++
				push(sString)
			}
			continue
		}
		if c == ':' && next == ':' && byteAt(src, p+2) == ':' {
			p += 3
			push(sClassName)
			continue
		}
		// A point's `: [`, with any whitespace before, between and after, line breaks included.
		if isSpace, _ := spaceAt(src, p); c == ':' || isSpace {
			k := p
			if c != ':' {
				if p < wsFrom || p >= wsTo {
					wsFrom = p
					wsTo = run(p)
				}
				k = wsTo
			}
			if byteAt(src, k) == ':' {
				if k+1 < gapFrom || k+1 >= gapTo {
					gapFrom = k + 1
					gapTo = run(k + 1)
				}
				if byteAt(src, gapTo) == '[' {
					emit(tPointStart, p, run(gapTo+1))
					push(sPointStart)
					continue
				}
			}
		}

		if isLetter(c) {
			e := p + 1
			for isLetter(byteAt(src, e)) {
				e++
			}
			emit(tAlpha, p, e)
			continue
		}
		if c >= 0x80 {
			e := p + 1
			for byteAt(src, e) >= 0x80 {
				e++
			}
			emit(tUnicodeText, p, e)
			continue
		}
		if isDigit(c) {
			e := p + 1
			for isDigit(byteAt(src, e)) {
				e++
			}
			emit(tNum, p, e)
			continue
		}
		if single[c] != 0 {
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
	if state < len(atEnd) {
		out.types = append(out.types, atEnd[state])
		out.texts = append(out.texts, "")
		out.starts = append(out.starts, n)
	}
	out.types = append(out.types, tEND)
	out.texts = append(out.texts, "")
	out.starts = append(out.starts, n)
	return out
}

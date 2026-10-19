//go:build wip

package sequence

import (
	"github.com/alessioalex/pele/go/internal/js"
)

// tokenList holds the tokens. Starts are in UTF-16 code units: the lexer reads the source as
// charCodeAt does, since its rules look one code unit past a character.
type tokenList struct {
	types  []int
	texts  []string
	starts []int
}

const (
	sINITIAL = iota
	sID
	sALIAS
	sLINE
	sCONFIG
	sAccTitle
	sAccDescr
	sAccDescrMultiline
)

const (
	cActorStart = 1
	cActorRest  = 2
	cID         = 4
	cIDWide     = 8
	cLine       = 16
	cMenu       = 32
)

// Character classes of Mermaid's lexer rules, for ASCII. Other characters are tested by isSpace.
var class [128]uint8

func init() {
	in := func(set string, c int) bool {
		for i := 0; i < len(set); i++ {
			if int(set[i]) == c {
				return true
			}
		}
		return false
	}
	for c := 0; c < 128; c++ {
		ws := c == 32 || (c >= 9 && c <= 13)
		var bits uint8
		if !in("/\\+()<->:\n,;", c) {
			bits |= cActorStart
		}
		if !in("+<->:\n,;", c) {
			bits |= cActorRest
		}
		if !in("<>:\n,;@", c) {
			bits |= cIDWide
			if !ws {
				bits |= cID
			}
		}
		if !in("#\n;", c) {
			bits |= cLine
		}
		if !in("/\\+()<>:\n,;-", c) && !ws {
			bits |= cMenu
		}
		class[c] = bits
	}
}

// Tests on code units; -1 stands for the NaN charCodeAt gives past the end.
func isSpaceU(c int) bool { return c >= 0 && js.IsSpace(rune(c)) }
func isDigitU(c int) bool { return c >= 48 && c <= 57 }
func isWordU(c int) bool  { return c >= 0 && c < 128 && js.IsWord(rune(c)) }

type lexer struct {
	u      []uint16
	n      int
	p      int
	state  int
	stack  []int
	nextNl int
	out    tokenList
}

func (l *lexer) at(q int) int {
	if q < 0 || q >= l.n {
		return -1
	}
	return int(l.u[q])
}

func (l *lexer) str(s, e int) string { return js.FromUnits(l.u[s:e]) }

func (l *lexer) emitText(typ, s, e int, text string) {
	l.out.types = append(l.out.types, typ)
	l.out.texts = append(l.out.texts, text)
	l.out.starts = append(l.out.starts, s)
	l.p = e
}

func (l *lexer) emit(typ, s, e int) { l.emitText(typ, s, e, l.str(s, e)) }

func (l *lexer) push(s int) {
	l.stack = append(l.stack, l.state)
	l.state = s
}

func (l *lexer) pop() {
	if len(l.stack) > 0 {
		l.state = l.stack[len(l.stack)-1]
		l.stack = l.stack[:len(l.stack)-1]
	}
}

func (l *lexer) indexOf(c uint16, q int) int {
	for ; q < l.n; q++ {
		if l.u[q] == c {
			return q
		}
	}
	return -1
}

func (l *lexer) eol(q int) int {
	if l.nextNl < q {
		l.nextNl = l.indexOf('\n', q)
		if l.nextNl == -1 {
			l.nextNl = l.n
		}
	}
	return l.nextNl
}

func (l *lexer) has(q int, bit uint8) bool {
	c := l.at(q)
	if c >= 0 && c < 128 {
		return class[c]&bit != 0
	}
	if bit == cID || bit == cMenu {
		return !isSpaceU(c)
	}
	return true
}

// ci matches a lowercase literal without regard to the case of the source.
func (l *lexer) ci(q int, word string) bool {
	for k := 0; k < len(word); k++ {
		w := int(word[k])
		c := l.at(q + k)
		if w >= 97 && w <= 122 {
			if c|32 != w {
				return false
			}
		} else if c != w {
			return false
		}
	}
	return true
}

func (l *lexer) word(lit string) bool { return l.ci(l.p, lit) && !isWordU(l.at(l.p+len(lit))) }

func (l *lexer) skipWs(q int) int {
	for q < l.n && isSpaceU(l.at(q)) {
		q++
	}
	return q
}

func (l *lexer) skipSameLine() bool {
	q := l.p
	for q < l.n {
		c := l.at(q)
		if c == 10 || !isSpaceU(c) {
			break
		}
		q++
	}
	if q == l.p {
		return false
	}
	l.p = q
	return true
}

func (l *lexer) lineRun(q int) int {
	for q < l.n && l.has(q, cLine) {
		q++
	}
	return q
}

func (l *lexer) begin(typ, length, next int) {
	l.emit(typ, l.p, l.p+length)
	l.push(next)
}

func (l *lexer) keyword(lit string, typ, next int) bool {
	if !l.word(lit) {
		return false
	}
	l.emit(typ, l.p, l.p+len(lit))
	if next >= 0 {
		l.push(next)
	}
	return true
}

// menu matches an actor menu keyword, which counts only when spaces and the start of a name follow.
func (l *lexer) menu(lit string, typ int) bool {
	if !l.ci(l.p, lit) {
		return false
	}
	q := l.p + len(lit)
	c := l.at(q)
	if c != 32 && c != 9 {
		return false
	}
	for c == 32 || c == 9 {
		q++
		c = l.at(q)
	}
	if q >= l.n || !l.has(q, cMenu) {
		return false
	}
	l.emit(typ, l.p, l.p+len(lit))
	return true
}

func (l *lexer) arrowAhead(q int) bool {
	c := l.at(q)
	d := l.at(q + 1)
	if c == 45 {
		if d == 120 || d == 88 || d == 41 || d == 92 || d == 47 || d == 45 {
			return true
		}
		e := l.at(q + 2)
		return d == 124 && (e == 92 || e == 47)
	}
	if c == 47 || c == 92 {
		return (d == 124 || d == c) && l.at(q+2) == 45
	}
	return c == 40 && d == 41
}

func (l *lexer) actor() {
	q := l.p + 1
	for q < l.n && l.has(q, cActorStart) {
		q++
	}
	for q < l.n && !l.arrowAhead(q) {
		r := q
		for l.at(r) == 45 {
			r++
		}
		if r >= l.n || !l.has(r, cActorRest) {
			break
		}
		r++
		for r < l.n && l.has(r, cActorRest) {
			r++
		}
		q = r
	}
	l.emitText(tACTOR, l.p, q, js.Trim(l.str(l.p, q)))
}

func (l *lexer) number() bool {
	p := l.p
	q := p
	for isDigitU(l.at(q)) {
		q++
	}
	whole := q > p
	if l.at(q) == 46 && isDigitU(l.at(q+1)) {
		r := q + 2
		if isDigitU(l.at(q + 2)) {
			r = q + 3
		}
		after := l.at(r)
		if after == 32 || after == 10 {
			l.emit(tNUM, p, r)
			return true
		}
	}
	after := l.at(q)
	if whole && (after == 32 || after == 10) {
		l.emit(tNUM, p, q)
		return true
	}
	return false
}

func (l *lexer) dash() {
	p := l.p
	d := l.at(p + 1)
	e := l.at(p + 2)
	switch {
	case d == 62:
		if e == 62 {
			l.emit(tSolidArrow, p, p+3)
		} else {
			l.emit(tSolidOpenArrow, p, p+2)
		}
	case d == 120 || d == 88:
		l.emit(tSolidCross, p, p+2)
	case d == 41:
		l.emit(tSolidPoint, p, p+2)
	case d == 124 && e == 92:
		l.emit(tSolidArrowTop, p, p+3)
	case d == 124 && e == 47:
		l.emit(tSolidArrowBottom, p, p+3)
	case d == 92 && e == 92:
		l.emit(tStickArrowTop, p, p+3)
	case d == 47 && e == 47:
		l.emit(tStickArrowBottom, p, p+3)
	case d == 45:
		f := l.at(p + 3)
		switch {
		case e == 62:
			if f == 62 {
				l.emit(tDottedArrow, p, p+4)
			} else {
				l.emit(tDottedOpenArrow, p, p+3)
			}
		case e == 120 || e == 88:
			l.emit(tDottedCross, p, p+3)
		case e == 41:
			l.emit(tDottedPoint, p, p+3)
		case e == 124 && f == 92:
			l.emit(tSolidArrowTopDotted, p, p+4)
		case e == 124 && f == 47:
			l.emit(tSolidArrowBottomDotted, p, p+4)
		case e == 92 && f == 92:
			l.emit(tStickArrowTopDotted, p, p+4)
		case e == 47 && f == 47:
			l.emit(tStickArrowBottomDotted, p, p+4)
		default:
			l.emit(tMINUS, p, p+1)
		}
	default:
		l.emit(tMINUS, p, p+1)
	}
}

func (l *lexer) reverse(c int) bool {
	p := l.p
	d := l.at(p + 1)
	if (d != 124 && d != c) || l.at(p+2) != 45 {
		return false
	}
	dotted := l.at(p+3) == 45
	top := c == 47
	var typ int
	if d == 124 {
		if dotted {
			typ = pick(top, tSolidArrowTopReverseDotted, tSolidArrowBottomReverseDotted)
		} else {
			typ = pick(top, tSolidArrowTopReverse, tSolidArrowBottomReverse)
		}
	} else if dotted {
		typ = pick(top, tStickArrowTopReverseDotted, tStickArrowBottomReverseDotted)
	} else {
		typ = pick(top, tStickArrowTopReverse, tStickArrowBottomReverse)
	}
	if dotted {
		l.emit(typ, p, p+4)
	} else {
		l.emit(typ, p, p+3)
	}
	return true
}

func pick(c bool, a, b int) int {
	if c {
		return a
	}
	return b
}

func (l *lexer) title() bool {
	if !l.ci(l.p, "title") {
		return false
	}
	q := l.p + 5
	legacy := l.at(q) == 58
	if legacy {
		q++
	}
	if !isSpaceU(l.at(q)) || q+1 >= l.n || !l.has(q+1, cLine) {
		return false
	}
	l.emit(pick(legacy, tLegacyTitle, tTitle), l.p, l.lineRun(q+1))
	return true
}

func (l *lexer) acc() bool {
	descr := l.ci(l.p, "accdescr")
	if !descr && !l.ci(l.p, "acctitle") {
		return false
	}
	q := l.skipWs(l.p + 8)
	c := l.at(q)
	if c == 58 {
		l.emit(pick(descr, tAccDescr, tAccTitle), l.p, l.skipWs(q+1))
		l.push(pick(descr, sAccDescr, sAccTitle))
		return true
	}
	if descr && c == 123 {
		l.p = l.skipWs(q + 1)
		l.push(sAccDescrMultiline)
		return true
	}
	return false
}

func (l *lexer) keywords(c int) bool {
	switch c | 32 {
	case 97:
		return l.keyword("actor", tParticipantActor, sID) ||
			l.keyword("alt", tAlt, sLINE) ||
			l.keyword("and", tAnd, sLINE) ||
			l.keyword("activate", tActivate, sID) ||
			l.acc() ||
			l.keyword("autonumber", tAutonumber, -1)
	case 98:
		return l.keyword("box", tBox, sLINE) || l.keyword("break", tBreak, sLINE)
	case 99:
		return l.keyword("create", tCreate, -1) || l.keyword("critical", tCritical, sLINE)
	case 100:
		return l.keyword("destroy", tDestroy, sID) || l.menu("details", tDetails) || l.keyword("deactivate", tDeactivate, sID)
	case 101:
		return l.keyword("else", tElse, sLINE) || l.keyword("end", tEnd, -1)
	case 108:
		return l.keyword("loop", tLoop, sLINE) || l.keyword("left of", tLeftOf, -1) || l.menu("links", tLinks) || l.menu("link", tLink)
	case 110:
		return l.keyword("note", tNote, -1)
	case 111:
		return l.keyword("opt", tOpt, sLINE) || l.keyword("option", tOption, sLINE) || l.keyword("over", tOver, -1) || l.keyword("off", tOff, -1)
	case 112:
		return l.keyword("participant", tParticipant, sID) ||
			l.keyword("par", tPar, sLINE) ||
			l.keyword("par_over", tParOver, sLINE) ||
			l.menu("properties", tProperties)
	case 114:
		return l.keyword("rect", tRect, sLINE) || l.keyword("right of", tRightOf, -1)
	case 115:
		return l.keyword("sequencediagram", tSD, -1)
	case 116:
		return l.title()
	}
	return false
}

func (l *lexer) startsWith(lit string, q int) bool {
	for k := 0; k < len(lit); k++ {
		if l.at(q+k) != int(lit[k]) {
			return false
		}
	}
	return true
}

func (l *lexer) initial() {
	p := l.p
	c := l.at(p)
	if c == 10 {
		q := p + 1
		for l.at(q) == 10 {
			q++
		}
		l.emit(tNEWLINE, p, q)
		return
	}
	if isSpaceU(c) {
		l.p = l.skipWs(p + 1)
		return
	}
	if c == 35 || (c == 37 && l.at(p+1) != 123) {
		l.p = l.eol(p)
		return
	}
	if c != 125 && l.at(p+1) == 37 && l.at(p+2) == 37 {
		l.p = l.eol(p)
		return
	}
	if (isDigitU(c) || c == 46) && l.number() {
		return
	}
	if c < 128 && (c|32) >= 97 && (c|32) <= 122 && l.keywords(c) {
		return
	}
	switch {
	case c == 44:
		l.emit(tCOMMA, p, p+1)
	case c == 59:
		l.emit(tNEWLINE, p, p+1)
	case l.has(p, cActorStart):
		l.actor()
	case c == 45:
		l.dash()
	case c == 60 && l.startsWith("<<->>", p):
		l.emit(tBidirectionalSolidArrow, p, p+5)
	case c == 60 && l.startsWith("<<-->>", p):
		l.emit(tBidirectionalDottedArrow, p, p+6)
	case (c == 47 || c == 92) && l.reverse(c):
	case c == 58:
		l.emit(tTXT, p, l.lineRun(p+1))
	case c == 43:
		l.emit(tPLUS, p, p+1)
	case c == 40 && l.at(p+1) == 41:
		l.emit(tCENTRAL, p, p+2)
	default:
		l.emit(tINVALID, p, p+1)
	}
}

// id reads a participant name, up to an alias, a config object, or the end of the statement.
func (l *lexer) id() bool {
	if l.skipSameLine() {
		return true
	}
	p := l.p
	c := l.at(p)
	if c == 35 {
		l.p = l.eol(p)
		return true
	}
	if c == 64 && l.at(p+1) == 123 {
		l.begin(tConfigStart, 2, sCONFIG)
		return true
	}
	a := p
	for a < l.n && l.has(a, cID) {
		a++
	}
	if a > p {
		w := l.skipWs(a)
		if l.at(w) == 64 && l.at(w+1) == 123 {
			l.emit(tACTOR, p, a)
			return true
		}
		if w > a && l.ci(w, "as") && isSpaceU(l.at(w+2)) {
			l.emit(tACTOR, p, a)
			l.push(sALIAS)
			return true
		}
	}
	b := a
	for b < l.n && l.has(b, cIDWide) {
		b++
	}
	if b > p {
		end := l.at(b)
		// Before any other delimiter the name must be followed by a comment, so it ends at the last one.
		cut := b
		if b < l.n && end != 10 && end != 59 {
			cut--
			for cut > p && l.at(cut) != 35 {
				cut--
			}
		}
		if cut > p {
			l.emitText(tACTOR, p, cut, js.Trim(l.str(p, cut)))
			l.pop()
			return true
		}
	}
	if l.at(b) == 60 {
		l.emit(tINVALID, p, l.eol(p))
		l.pop()
		return true
	}
	if c != 10 {
		e := l.eol(p)
		l.emitText(tINVALID, p, e, js.Trim(l.str(p, e)))
		l.pop()
		return true
	}
	return false
}

func (l *lexer) step() bool {
	switch l.state {
	case sINITIAL:
		l.initial()
		return true
	case sID:
		return l.id()
	case sALIAS:
		if l.skipSameLine() {
			return true
		}
		if l.at(l.p) == 35 {
			l.p = l.eol(l.p)
			return true
		}
		l.pop()
		l.pop()
		if l.word("as") {
			l.begin(tAS, 2, sLINE)
		} else {
			l.emit(tNEWLINE, l.p, l.p)
		}
		return true
	case sLINE:
		if l.skipSameLine() {
			return true
		}
		if l.at(l.p) == 35 {
			l.p = l.eol(l.p)
			return true
		}
		l.emit(tRestOfLine, l.p, l.lineRun(l.p))
		l.pop()
		return true
	case sCONFIG:
		p := l.p
		if l.at(p) != 125 {
			q := l.indexOf('}', p)
			if q == -1 {
				q = l.n
			}
			l.emit(tConfigContent, p, q)
			return true
		}
		w := l.skipWs(p + 1)
		alias := w > p+1 && l.ci(w, "as") && isSpaceU(l.at(w+2))
		l.emit(tConfigEnd, p, p+1)
		l.pop()
		if alias {
			l.push(sALIAS)
		} else {
			l.pop()
		}
		return true
	case sAccTitle, sAccDescr:
		l.emit(pick(l.state == sAccTitle, tAccTitleValue, tAccDescrValue), l.p, l.eol(l.p))
		l.pop()
		return true
	default:
		if l.at(l.p) == 125 {
			l.p++
			l.pop()
		} else {
			q := l.indexOf('}', l.p)
			if q == -1 {
				q = l.n
			}
			l.emit(tAccDescrMultilineValue, l.p, q)
		}
		return true
	}
}

func tokenize(src string) tokenList {
	u := js.Units(src)
	l := &lexer{u: u, n: len(u), nextNl: -1, state: sINITIAL}
	for l.p < l.n {
		if !l.step() {
			l.out.types = append(l.out.types, tERROR)
			l.out.texts = append(l.out.texts, "")
			l.out.starts = append(l.out.starts, l.p)
			return l.out
		}
	}
	n := l.n
	// Mermaid's lexer tries its rules once more on the empty remainder before reporting the end.
	switch l.state {
	case sINITIAL, sALIAS:
		l.emit(tNEWLINE, n, n)
	case sLINE:
		l.emit(tRestOfLine, n, n)
	case sAccTitle:
		l.emit(tAccTitleValue, n, n)
	case sAccDescr:
		l.emit(tAccDescrValue, n, n)
	case sAccDescrMultiline:
		l.emit(tAccDescrMultilineValue, n, n)
	}
	l.out.types = append(l.out.types, tEND)
	l.out.texts = append(l.out.texts, "")
	l.out.starts = append(l.out.starts, n)
	return l.out
}

package agentflow

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/util"
)

type tokenList struct {
	types  []int
	texts  []string
	starts []int
	// Four numbers per token, counted the way Jison counts them:
	// first line, first column, last line, last column. Lines start at 1, columns at 0, in
	// UTF-16 code units.
	locs []int
}

const (
	sINITIAL = iota
	sString
	sMdString
	sAccTitle
	sAccDescr
	sAccDescrMultiline
	sDir
	sText
	sEllipseText
	sTrapText
	sEdgeText
	sClick
	sCallbackname
	sCallbackargs
	sShapeData
	sShapeDataStr
)

var nodeChar [128]bool

func init() {
	for _, ch := range "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!\"#$%&'*+.`?\\_/" {
		nodeChar[ch] = true
	}
}

var directionWords = [5]string{"TB", "BT", "RL", "LR", "TD"}

type lexer struct {
	src       string
	n         int
	tk        tokenList
	stack     []int
	state     int
	p         int
	line, col int
	firstDir  bool
	dirNext   int
	atNext    int
	lineEnd   int
	dirFailed int
	runEnd    int
	runAt     int
	wsFrom    int
	wsTo      int
	blankFrom int
	blankTo   int
	noLink    int
	noArrow   int
}

// at is charCodeAt for ASCII tests: the byte at q, or -1 past the end.
func (l *lexer) at(q int) int {
	if q < 0 || q >= l.n {
		return -1
	}
	return int(l.src[q])
}

func (l *lexer) spaceAt(q int) (bool, int) {
	if q >= l.n {
		return false, 0
	}
	if c := l.src[q]; c < 0x80 {
		return c == ' ' || (c >= 9 && c <= 13), 1
	}
	r, size := js.RuneAt(l.src, q)
	return js.IsSpace(r), size
}

func (l *lexer) wordAt(q int) bool {
	c := l.at(q)
	return c >= 0 && js.IsWord(rune(c))
}

// advance moves over one match. Jison counts \r\n, \r and \n as one line break each, and after a
// break its column stops at the first U+2028 or U+2029.
func (l *lexer) advance(e int) {
	src := l.src
	brk := -1
	for k := l.p; k < e; k++ {
		ch := src[k]
		if ch == '\n' {
			l.line++
			brk = k
		} else if ch == '\r' {
			if k+1 < e && src[k+1] == '\n' {
				k++
			}
			l.line++
			brk = k
		}
	}
	if brk < 0 {
		l.col += js.Len(src[l.p:e])
	} else {
		k := brk + 1
		for k < e {
			r, size := js.RuneAt(src, k)
			if r == 0x2028 || r == 0x2029 {
				break
			}
			k += size
		}
		l.col = js.Len(src[brk+1 : k])
	}
	l.p = e
}

func (l *lexer) start(typ int, text string) {
	l.tk.types = append(l.tk.types, typ)
	l.tk.texts = append(l.tk.texts, text)
	l.tk.starts = append(l.tk.starts, l.p)
	l.tk.locs = append(l.tk.locs, l.line, l.col)
}

func (l *lexer) emit(typ, e int) {
	l.start(typ, l.src[l.p:e])
	l.advance(e)
	l.tk.locs = append(l.tk.locs, l.line, l.col)
}

// emitFlat is emit for a token that cannot hold a line break.
func (l *lexer) emitFlat(typ, e int) {
	l.start(typ, l.src[l.p:e])
	l.col += js.Len(l.src[l.p:e])
	l.p = e
	l.tk.locs = append(l.tk.locs, l.line, l.col)
}

// emitChars is emit for a run that Jison matches one character at a time, where \r\n counts as
// two line breaks.
func (l *lexer) emitChars(typ, e int) {
	l.start(typ, l.src[l.p:e])
	for _, r := range l.src[l.p:e] {
		if r == '\n' || r == '\r' {
			l.line++
			l.col = 0
		} else if r >= 0x10000 {
			l.col += 2
		} else {
			l.col++
		}
	}
	l.p = e
	l.tk.locs = append(l.tk.locs, l.line, l.col)
}

func (l *lexer) emitText(typ int, text string, e int) {
	l.start(typ, text)
	l.advance(e)
	l.tk.locs = append(l.tk.locs, l.line, l.col)
}

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

func (l *lexer) kw(word string) bool {
	return strings.HasPrefix(l.src[l.p:], word) && !l.wordAt(l.p+len(word))
}

// skipSpace is the end of `\s*` from q.
func (l *lexer) skipSpace(q int) int {
	for q < l.n {
		ok, size := l.spaceAt(q)
		if !ok {
			break
		}
		q += size
	}
	return q
}

// wsEnd is the end of the run of whitespace starting at q. The last run is kept, so asking again
// from inside it costs nothing.
func (l *lexer) wsEnd(q int) int {
	if q >= l.wsFrom && q <= l.wsTo {
		return l.wsTo
	}
	j := l.skipSpace(q)
	l.wsFrom = q
	l.wsTo = j
	return j
}

func (l *lexer) blankEnd(q int) int {
	if q >= l.blankFrom && q <= l.blankTo {
		return l.blankTo
	}
	j := l.blanks(q)
	l.blankFrom = q
	l.blankTo = j
	return j
}

func (l *lexer) blanks(from int) int {
	e := from
	for e < l.n && (l.src[e] == ' ' || l.src[e] == '\t') {
		e++
	}
	return e
}

// arrowEnd is the end of `\s*--+[x>]\s*` starting at q, or -1.
func (l *lexer) arrowEnd(q int) int {
	j := l.wsEnd(q)
	if j == l.noArrow || l.at(j) != '-' || l.at(j+1) != '-' {
		return -1
	}
	k := j + 2
	for l.at(k) == '-' {
		k++
	}
	if t := l.at(k); t == 'x' || t == '>' {
		return l.wsEnd(k + 1)
	}
	l.noArrow = j
	return -1
}

func (l *lexer) link() bool {
	j := l.wsEnd(l.p)
	if j == l.noLink || l.at(j) != '-' {
		return false
	}
	d := l.at(j + 1)
	k := j + 2
	if d == '-' {
		for l.at(k) == '-' {
			k++
		}
		if t := l.at(k); t == 'x' || t == '>' {
			l.emit(tLINK, l.wsEnd(k+1))
		} else {
			l.push(sEdgeText)
			l.emit(tSTART_LINK, l.wsEnd(j+2))
		}
		return true
	}
	if d == '.' {
		for l.at(k) == '.' {
			k++
		}
		if l.at(k) == '-' {
			l.emit(tLINK, l.wsEnd(k+1))
			return true
		}
	}
	l.noLink = j
	return false
}

// commentEnd is the end of `%%` not followed by `{`, through the end of the line, or -1.
func (l *lexer) commentEnd(q int) int {
	if l.at(q) != '%' || l.at(q+1) != '%' || l.at(q+2) == '{' {
		return -1
	}
	if e := strings.IndexByte(l.src[q+2:], '\n'); e >= 0 {
		return q + 2 + e
	}
	return l.n
}

func (l *lexer) indexFrom(q int, b byte) int {
	if e := strings.IndexByte(l.src[q:], b); e >= 0 {
		return q + e
	}
	return l.n
}

func (l *lexer) quote() {
	if l.at(l.p+1) == '`' {
		l.push(sMdString)
		l.advance(l.p + 2)
	} else {
		l.push(sString)
		l.advance(l.p + 1)
	}
}

func (l *lexer) opener(c int) bool {
	p := l.p
	d := l.at(p + 1)
	switch c {
	case '(':
		if d == '-' {
			l.push(sEllipseText)
			l.emitFlat(tELLIPSE_START, p+2)
		} else if d == '[' {
			l.push(sText)
			l.emitFlat(tSTADIUMSTART, p+2)
		} else if d == '(' && l.at(p+2) == '(' {
			l.push(sText)
			l.emitFlat(tDOUBLECIRCLESTART, p+3)
		} else {
			l.push(sText)
			l.emitFlat(tPS, p+1)
		}
		return true
	case '[':
		if d == '[' {
			l.push(sText)
			l.emitFlat(tSUBROUTINESTART, p+2)
		} else if d == '(' {
			l.push(sText)
			l.emitFlat(tCYLINDERSTART, p+2)
		} else if d == '/' {
			l.push(sTrapText)
			l.emitFlat(tTRAPSTART, p+2)
		} else if d == '\\' {
			l.push(sTrapText)
			l.emitFlat(tINVTRAPSTART, p+2)
		} else {
			l.push(sText)
			l.emitFlat(tSQS, p+1)
		}
		return true
	case '|':
		l.push(sText)
		l.emitFlat(tPIPE, p+1)
		return true
	case '{':
		l.push(sText)
		l.emitFlat(tDIAMOND_START, p+1)
		return true
	}
	return false
}

func (l *lexer) nodeString() bool {
	q := l.p
	for q < l.n {
		ch := l.src[q]
		if ch < 128 && nodeChar[ch] {
			q++
		} else if ch == '-' && q+1 < l.n {
			if d := l.src[q+1]; d == '>' || d == '-' || d == '.' {
				break
			}
			q++
		} else {
			break
		}
	}
	if q == l.p {
		return false
	}
	l.emitFlat(tNODE_STRING, q)
	return true
}

// accAt matches `word\s*` + sep + `\s*` at p (sep is ':' or '{').
func (l *lexer) accAt(word string, sep byte) int {
	if !strings.HasPrefix(l.src[l.p:], word) {
		return -1
	}
	j := l.skipSpace(l.p + len(word))
	if l.at(j) != int(sep) {
		return -1
	}
	return l.skipSpace(j + 1)
}

// wordSpaceAt matches `word\s+` at p.
func (l *lexer) wordSpaceAt(word string) int {
	if !strings.HasPrefix(l.src[l.p:], word) {
		return -1
	}
	j := l.skipSpace(l.p + len(word))
	if j == l.p+len(word) {
		return -1
	}
	return j
}

// lineEndFrom is the first of \n, \r, U+2028 and U+2029 at or after q, or n.
func (l *lexer) lineEndFrom(q int) int {
	src := l.src
	for k := q; k < l.n; k++ {
		switch src[k] {
		case '\n', '\r':
			return k
		case 0xe2:
			if k+2 < l.n && src[k+1] == 0x80 && (src[k+2] == 0xa8 || src[k+2] == 0xa9) {
				return k
			}
		}
	}
	return l.n
}

// directionAt matches `.*direction\s+XX[^\n]*` at p. The greedy `.*` takes the last `direction`
// on the line that the rest matches after, and `\s+` may run onto the lines below.
func (l *lexer) directionAt(word string) int {
	src := l.src
	limit := l.lineEnd
	for limit > l.p {
		s := strings.LastIndex(src[l.p:limit], "direction")
		if s < 0 {
			return -1
		}
		s += l.p
		j := s + len("direction")
		k := l.skipSpace(j)
		if k > j && strings.HasPrefix(src[k:], word) {
			return l.indexFrom(k+2, '\n')
		}
		limit = s + len("direction") - 1
	}
	return -1
}

func (l *lexer) initial(c int) bool {
	p := l.p
	src := l.src
	var e int
	switch c {
	case 'a':
		if strings.HasPrefix(src[p:], "acc") {
			if e = l.accAt("accTitle", ':'); e >= 0 {
				l.push(sAccTitle)
				l.emit(tAccTitle, e)
				return true
			}
			if e = l.accAt("accDescr", ':'); e >= 0 {
				l.push(sAccDescr)
				l.emit(tAccDescr, e)
				return true
			}
			if e = l.accAt("accDescr", '{'); e >= 0 {
				l.push(sAccDescrMultiline)
				l.advance(e)
				return true
			}
		}
		if l.kw("agentflow-beta") {
			if l.firstDir {
				l.firstDir = false
				l.push(sDir)
			}
			l.emitFlat(tGRAPH, p+14)
			return true
		}
	case '@':
		if l.at(p+1) == '{' {
			l.push(sShapeData)
			l.emitText(tSHAPE_DATA, "", p+2)
			return true
		}
	case 'c':
		if e = l.wordSpaceAt("call"); e >= 0 {
			l.push(sCallbackname)
			l.advance(e)
			return true
		}
		if l.kw("classDef") {
			l.emitFlat(tCLASSDEF, p+8)
			return true
		}
		if l.kw("class") {
			l.emitFlat(tCLASS, p+5)
			return true
		}
		if e = l.wordSpaceAt("click"); e >= 0 {
			l.push(sClick)
			l.advance(e)
			return true
		}
		if l.kw("connector") {
			l.emitFlat(tConnector, p+9)
			return true
		}
	case '"':
		l.quote()
		return true
	case 's':
		if l.kw("style") {
			l.emitFlat(tSTYLE, p+5)
			return true
		}
	case 'd':
		if l.kw("default") {
			l.emitFlat(tDEFAULT, p+7)
			return true
		}
	case 'l':
		if l.kw("linkStyle") {
			l.emitFlat(tLINKSTYLE, p+9)
			return true
		}
	case 'i':
		if l.kw("interpolate") {
			l.emitFlat(tINTERPOLATE, p+11)
			return true
		}
	case 'h':
		if strings.HasPrefix(src[p:], "href") {
			if ok, size := l.spaceAt(p + 4); ok {
				l.emit(tHREF, p+4+size)
				return true
			}
		}
	case 'f':
		if l.kw("flow") {
			l.emitFlat(tFlow, p+4)
			return true
		}
	case 'g':
		if l.kw("global") {
			l.emitFlat(tGlobal, l.blanks(p+6))
			return true
		}
	case 'e':
		if l.kw("end") {
			l.emitFlat(tEnd, l.blanks(p+3))
			return true
		}
	case '_':
		for _, target := range [...]string{"_self", "_blank", "_parent", "_top"} {
			if l.kw(target) {
				l.emitFlat(tLINK_TARGET, p+len(target))
				return true
			}
		}
	}

	if l.dirNext != -1 && p >= l.dirFailed {
		if l.dirNext < p {
			l.dirNext = strings.Index(src[p:], "direction")
			if l.dirNext >= 0 {
				l.dirNext += p
			}
		}
		if l.dirNext != -1 {
			if l.lineEnd < p {
				l.lineEnd = l.lineEndFrom(p)
			}
			if l.dirNext < l.lineEnd {
				for i, word := range directionWords {
					if e = l.directionAt(word); e >= 0 {
						l.emit(tDirectionTB+i, e)
						return true
					}
				}
				// No direction statement starts anywhere in the rest of this line either.
				l.dirFailed = l.lineEnd
			}
		}
	}

	// A link id is a run of non-space characters up to its last `@` that is not followed by `{` or `"`.
	if space, _ := l.spaceAt(p); l.atNext != -1 && c != '"' && !space {
		if p >= l.runEnd {
			if l.atNext < p {
				l.atNext = strings.IndexByte(src[p:], '@')
				if l.atNext >= 0 {
					l.atNext += p
				}
			}
			q := p
			for q < l.n {
				if src[q] == '"' {
					break
				}
				ok, size := l.spaceAt(q)
				if ok {
					break
				}
				q += size
			}
			l.runEnd = q
			l.runAt = -1
			if l.atNext != -1 && l.atNext < q {
				for k := min(q, l.n-1) - 1; k >= l.atNext; k-- {
					if src[k] != '@' {
						continue
					}
					if after := l.at(k + 1); after != '{' && after != '"' {
						l.runAt = k
						break
					}
				}
			}
		}
		if l.runAt > p {
			l.emitFlat(tLINK_ID, l.runAt+1)
			return true
		}
	}

	if c >= '0' && c <= '9' {
		q := p + 1
		for q < l.n && src[q] >= '0' && src[q] <= '9' {
			q++
		}
		l.emitFlat(tNUM, q)
		return true
	}

	switch c {
	case '#':
		l.emitFlat(tBRKT, p+1)
		return true
	case ':':
		if l.at(p+1) == ':' && l.at(p+2) == ':' {
			l.emitFlat(tSTYLE_SEPARATOR, p+3)
		} else {
			l.emitFlat(tCOLON, p+1)
		}
		return true
	case '&':
		l.emitFlat(tAMP, p+1)
		return true
	case ';':
		l.emitFlat(tSEMI, p+1)
		return true
	case ',':
		l.emitFlat(tCOMMA, p+1)
		return true
	case '*':
		l.emitFlat(tMULT, p+1)
		return true
	case '<':
		l.emitFlat(tTAGSTART, p+1)
		return true
	case '-':
		if l.link() || l.nodeString() {
			return true
		}
		l.emitFlat(tMINUS, p+1)
		return true
	case '(':
		return l.opener(c)
	case '[':
		if l.at(p+1) == '|' {
			l.emitFlat(tVERTEX_WITH_PROPS_START, p+2)
			return true
		}
		return l.opener(c)
	case '|', '{':
		return l.opener(c)
	case '>':
		l.push(sText)
		l.emitFlat(tTAGEND, p+1)
		return true
	case '^':
		l.emitFlat(tUP, p+1)
		return true
	case '\\':
		if l.at(p+1) == '|' {
			l.emitFlat(tSEP, p+2)
			return true
		}
		return l.nodeString()
	case 'v':
		if !l.wordAt(p + 1) {
			l.emitFlat(tDOWN, p+1)
			return true
		}
		return l.nodeString()
	case '%':
		if e = l.commentEnd(p); e >= 0 {
			l.emit(tCOMMENT, e)
			return true
		}
		return l.nodeString()
	}

	if space, size := l.spaceAt(p); space {
		if l.link() {
			return true
		}
		if c == ' ' || c == '\t' {
			if e = l.commentEnd(l.blankEnd(p)); e >= 0 {
				l.emit(tCOMMENT, e)
				return true
			}
		}
		e = p
		for {
			if ch := l.at(e); ch == '\n' {
				e++
			} else if ch == '\r' && l.at(e+1) == '\n' {
				e += 2
			} else {
				break
			}
		}
		if e > p {
			l.emit(tNEWLINE, e)
		} else {
			l.emit(tSPACE, p+size)
		}
		return true
	}

	if c < 128 {
		return l.nodeString()
	}
	// A character outside the BMP is a surrogate pair to Jison, and neither half is a letter.
	if r, size := js.RuneAt(src, p); r < 0x10000 && util.IsUnicodeLetter(r) {
		l.emitFlat(tUNICODE_TEXT, p+size)
		return true
	}
	return false
}

// dirAt matches `\s*(?:LR|RL|TB|BT|TD|BR)\b`, `\s*[<>^]` or `\s*v\b` at p.
func (l *lexer) dirAt() int {
	j := l.skipSpace(l.p)
	if j+2 <= l.n {
		switch l.src[j : j+2] {
		case "LR", "RL", "TB", "BT", "TD", "BR":
			if !l.wordAt(j + 2) {
				return j + 2
			}
		}
	}
	switch l.at(j) {
	case '<', '>', '^':
		return j + 1
	case 'v':
		if !l.wordAt(j + 1) {
			return j + 1
		}
	}
	return -1
}

// nodirAt matches `(?:\r?\n)*\s*\n` at p: the whitespace up to the last line break in it.
func (l *lexer) nodirAt() int {
	end := -1
	for q := l.p; q < l.n; {
		ok, size := l.spaceAt(q)
		if !ok {
			break
		}
		if l.src[q] == '\n' {
			end = q + 1
		}
		q += size
	}
	return end
}

// brs is .replace(/\n\s*/g, '<br/>').
func brs(s string) string {
	if !strings.Contains(s, "\n") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '\n' {
			b.WriteByte(s[i])
			i++
			continue
		}
		i++
		for i < len(s) {
			r, size := js.RuneAt(s, i)
			if !js.IsSpace(r) {
				break
			}
			i += size
		}
		b.WriteString("<br/>")
	}
	return b.String()
}

func tokenize(src string) tokenList {
	l := &lexer{
		src:       src,
		n:         len(src),
		state:     sINITIAL,
		line:      1,
		firstDir:  true,
		lineEnd:   -1,
		runAt:     -1,
		wsFrom:    -1,
		wsTo:      -1,
		blankFrom: -1,
		blankTo:   -1,
		noLink:    -1,
		noArrow:   -1,
	}
	l.dirNext = strings.Index(src, "direction")
	l.atNext = strings.IndexByte(src, '@')
	n := l.n

	for {
		if l.p >= n {
			typ := -1
			switch l.state {
			case sINITIAL:
				typ = tEOF
			case sAccTitle:
				typ = tAccTitleValue
			case sAccDescr:
				typ = tAccDescrValue
			case sAccDescrMultiline:
				typ = tAccDescrMultilineValue
			case sClick:
				typ = tCLICK
			case sCallbackname:
				typ = tCALLBACKNAME
			case sCallbackargs:
				typ = tCALLBACKARGS
			}
			if typ != -1 {
				l.emitFlat(typ, n)
			}
			break
		}

		p := l.p
		c := int(src[p])
		ok := true

		switch l.state {
		case sINITIAL:
			ok = l.initial(c)

		case sString:
			if c == '"' {
				if l.at(p+1) == '`' {
					l.push(sMdString)
					l.advance(p + 2)
				} else {
					l.pop()
					l.advance(p + 1)
				}
			} else {
				l.emit(tSTR, l.indexFrom(p, '"'))
			}

		case sMdString:
			if c == '`' {
				if l.at(p+1) == '"' {
					l.pop()
					l.advance(p + 2)
				} else {
					ok = false
				}
			} else if c == '"' {
				l.quote()
			} else {
				q := p + 1
				for q < n && src[q] != '`' && src[q] != '"' {
					q++
				}
				l.emit(tMD_STR, q)
			}

		case sAccTitle, sAccDescr:
			e := l.indexFrom(p, '\n')
			typ := tAccTitleValue
			if l.state == sAccDescr {
				typ = tAccDescrValue
			}
			l.pop()
			l.emit(typ, e)

		case sAccDescrMultiline:
			if c == '}' {
				l.pop()
				l.advance(p + 1)
			} else {
				l.emit(tAccDescrMultilineValue, l.indexFrom(p, '}'))
			}

		case sDir:
			if c == '"' {
				l.quote()
			} else if e := l.nodirAt(); e >= 0 {
				l.pop()
				l.emit(tNODIR, e)
			} else if e := l.dirAt(); e >= 0 {
				l.pop()
				l.emit(tDIR, e)
			} else {
				ok = l.opener(c)
			}

		case sText:
			d := l.at(p + 1)
			switch c {
			case '"':
				l.quote()
			case '(', '[', '{':
				l.opener(c)
			case ']':
				l.pop()
				if d == ')' {
					l.emitFlat(tSTADIUMEND, p+2)
				} else if d == ']' {
					l.emitFlat(tSUBROUTINEEND, p+2)
				} else {
					l.emitFlat(tSQE, p+1)
				}
			case ')':
				l.pop()
				if d == ']' {
					l.emitFlat(tCYLINDEREND, p+2)
				} else if d == ')' && l.at(p+2) == ')' {
					l.emitFlat(tDOUBLECIRCLEEND, p+3)
				} else {
					l.emitFlat(tPE, p+1)
				}
			case '|':
				l.pop()
				l.emitFlat(tPIPE, p+1)
			case '}':
				l.pop()
				l.emitFlat(tDIAMOND_STOP, p+1)
			case '%':
				// Jison drops the lone `%` from Mermaid's text pattern, so only a comment may start with one.
				if e := l.commentEnd(p); e >= 0 {
					l.advance(e)
				} else {
					ok = false
				}
			default:
				q := p + 1
				for q < n {
					ch := src[q]
					if ch == '[' || ch == ']' || ch == '(' || ch == ')' || ch == '{' || ch == '}' || ch == '|' || ch == '"' || ch == '%' {
						break
					}
					q++
				}
				l.emit(tTEXT, q)
			}

		case sEllipseText:
			d := l.at(p + 1)
			if c == '"' {
				l.quote()
			} else if c == '%' && l.commentEnd(p) >= 0 {
				l.advance(l.commentEnd(p))
			} else if (c == '-' || c == '/' || c == ')') && d == ')' {
				l.pop()
				l.emitFlat(tELLIPSE_END, p+2)
			} else if c == '(' || c == '[' || c == '{' {
				l.opener(c)
			} else if c == ')' || c == ']' || c == '}' {
				ok = false
			} else {
				q := p + 1
				for q < n {
					ch := src[q]
					if ch == '"' || ch == '(' || ch == ')' || ch == '[' || ch == ']' || ch == '{' || ch == '}' {
						break
					}
					if (ch == '-' || ch == '/') && l.at(q+1) == ')' {
						break
					}
					if ch == '%' && l.at(q+1) == '%' && l.at(q+2) != '{' {
						break
					}
					q++
				}
				l.emitChars(tTEXT, q)
			}

		case sTrapText:
			d := l.at(p + 1)
			if c == '"' {
				l.quote()
				break
			}
			switch c {
			case '(':
				if d == '-' || d == '[' || (d == '(' && l.at(p+2) == '(') {
					l.opener(c)
				} else if d == ']' {
					l.pop()
					l.emitFlat(tTRAPEND, p+2)
				} else {
					l.opener(c)
				}
			case '[', '{':
				l.opener(c)
			case '\\':
				if d == ']' {
					l.pop()
					l.emitFlat(tTRAPEND, p+2)
				} else {
					l.emitFlat(tTEXT, p+1)
				}
			case '/':
				if d == ']' {
					l.pop()
					l.emitFlat(tINVTRAPEND, p+2)
				} else {
					l.emitFlat(tTEXT, p+1)
				}
			case ']', ')':
				if d == ']' {
					l.pop()
					l.emitFlat(tTRAPEND, p+2)
				} else {
					ok = false
				}
			case '}':
				ok = false
			case '%':
				if e := l.commentEnd(p); e >= 0 {
					l.advance(e)
				} else {
					ok = false
				}
			default:
				if (c == '?' || c == '=') && d == ']' {
					l.pop()
					l.emitFlat(tTRAPEND, p+2)
					break
				}
				q := p + 1
				for q < n {
					ch := src[q]
					if ch == '\\' || ch == '[' || ch == ']' || ch == '(' || ch == ')' || ch == '{' || ch == '}' || ch == '/' || ch == '%' {
						break
					}
					q++
				}
				l.emit(tTEXT, q)
			}

		case sEdgeText:
			if c == '"' {
				l.quote()
				break
			}
			if e := l.arrowEnd(p); e >= 0 {
				l.pop()
				l.emit(tLINK, e)
				break
			}
			if c == '-' && l.at(p+1) == '-' {
				ok = false
				break
			}
			q := p + 1
			for q < n {
				ch := src[q]
				if ch == '"' {
					break
				}
				if ch == '-' {
					if l.at(q+1) == '-' {
						break
					}
				} else if space, size := l.spaceAt(q); space {
					if l.arrowEnd(q) >= 0 {
						break
					}
					q += size
					continue
				}
				q++
			}
			l.emitChars(tEDGE_TEXT, q)

		case sClick:
			if c == '"' {
				l.quote()
			} else if space, size := l.spaceAt(p); space {
				l.pop()
				l.advance(p + size)
			} else {
				q := p + 1
				for q < n {
					space, size := l.spaceAt(q)
					if space {
						break
					}
					q += size
				}
				l.emitFlat(tCLICK, q)
			}

		case sCallbackname:
			if c == '(' {
				j := l.skipSpace(p + 1)
				l.pop()
				if l.at(j) == ')' {
					l.advance(j + 1)
				} else {
					l.push(sCallbackargs)
					l.advance(p + 1)
				}
			} else {
				l.emit(tCALLBACKNAME, l.indexFrom(p, '('))
			}

		case sCallbackargs:
			if c == ')' {
				l.pop()
				l.advance(p + 1)
			} else {
				l.emit(tCALLBACKARGS, l.indexFrom(p, ')'))
			}

		case sShapeData:
			if c == '"' {
				l.push(sShapeDataStr)
				l.emitFlat(tSHAPE_DATA, p+1)
			} else if c == '}' {
				l.pop()
				l.advance(p + 1)
			} else if c == '^' {
				ok = false
			} else {
				q := p + 1
				for q < n && src[q] != '}' && src[q] != '^' && src[q] != '"' {
					q++
				}
				l.emit(tSHAPE_DATA, q)
			}

		case sShapeDataStr:
			if c == '"' {
				l.pop()
				l.emitFlat(tSHAPE_DATA, p+1)
			} else {
				e := l.indexFrom(p, '"')
				l.emitText(tSHAPE_DATA, brs(src[p:e]), e)
			}
		}

		if !ok {
			l.tk.types = append(l.tk.types, tERROR)
			l.tk.texts = append(l.tk.texts, "")
			l.tk.starts = append(l.tk.starts, l.p)
			l.tk.locs = append(l.tk.locs, l.line, l.col, l.line, l.col)
			return l.tk
		}
	}

	l.tk.types = append(l.tk.types, tEND)
	l.tk.texts = append(l.tk.texts, "")
	l.tk.starts = append(l.tk.starts, n)
	l.tk.locs = append(l.tk.locs, l.line, l.col, l.line, l.col)
	return l.tk
}

package flowchart

import (
	"strings"
	"unicode/utf8"

	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/util"
)

// Tokens are parallel slices of token types, texts and byte offsets.
type Tokens struct {
	Types  []int
	Texts  []string
	Starts []int
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
	sThickEdgeText
	sDottedEdgeText
	sClick
	sCallbackname
	sCallbackargs
	sShapeData
	sShapeDataStr
)

var nodeChar = func() [128]bool {
	var t [128]bool
	for _, ch := range "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!\"#$%&'*+.`?\\_/" {
		t[ch] = true
	}
	return t
}()

var directionCodes = [5]string{"TB", "BT", "RL", "LR", "TD"}

type lexer struct {
	src       string
	n         int
	tok       Tokens
	stack     []int
	state     int
	p         int
	firstGrap bool
	dirNext   int
	atNext    int
	lineEnd   int
	dirFailed int
	runEnd    int
	runAt     int
	wsFrom    int
	wsTo      int
}

// at is charCodeAt for the ASCII range: -1 past the end, and the byte otherwise.
func (l *lexer) at(i int) int {
	if i < 0 || i >= l.n {
		return -1
	}
	return int(l.src[i])
}

// spaceAt reports whether a JS whitespace character starts at i, and its size.
func (l *lexer) spaceAt(i int) (bool, int) {
	if i >= l.n {
		return false, 0
	}
	c := l.src[i]
	if c < utf8.RuneSelf {
		return c == ' ' || (c >= 9 && c <= 13), 1
	}
	r, size := utf8.DecodeRuneInString(l.src[i:])
	return js.IsSpace(r), size
}

func (l *lexer) isSpace(i int) bool {
	ok, _ := l.spaceAt(i)
	return ok
}

// skipSpace is \s* from i.
func (l *lexer) skipSpace(i int) int {
	for i < l.n {
		ok, size := l.spaceAt(i)
		if !ok {
			break
		}
		i += size
	}
	return i
}

func (l *lexer) isWordAt(i int) bool {
	c := l.at(i)
	return c >= 0 && js.IsWord(rune(c))
}

func (l *lexer) emit(typ, s, e int) {
	l.tok.Types = append(l.tok.Types, typ)
	l.tok.Texts = append(l.tok.Texts, l.src[s:e])
	l.tok.Starts = append(l.tok.Starts, s)
	l.p = e
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
	return strings.HasPrefix(l.src[l.p:], word) && !l.isWordAt(l.p+len(word))
}

func isXo(c int) bool { return c == 'x' || c == 'o' || c == '<' }

// reLink is /\s*[xo<]?--+[-xo>]\s*/y, and reThickLink the same with `=`.
func (l *lexer) reLink(q int, ch byte) int {
	j := l.skipSpace(q)
	if isXo(l.at(j)) {
		j++
	}
	r := 0
	for j+r < l.n && l.src[j+r] == ch {
		r++
	}
	if r < 2 {
		return -1
	}
	k := j + r
	switch c := l.at(k); {
	case c == 'x' || c == 'o' || c == '>':
		k++
	case r >= 3:
	default:
		return -1
	}
	return l.skipSpace(k)
}

// reStartLink is /\s*[xo<]?--\s*/y, and reThickStart the same with `=`.
func (l *lexer) reStartLink(q int, ch byte) int {
	j := l.skipSpace(q)
	if isXo(l.at(j)) {
		j++
	}
	if l.at(j) != int(ch) || l.at(j+1) != int(ch) {
		return -1
	}
	return l.skipSpace(j + 2)
}

// reDottedLink is /\s*[xo<]?-?\.+-[xo>]?\s*/y.
func (l *lexer) reDottedLink(q int) int {
	j := l.skipSpace(q)
	if isXo(l.at(j)) {
		j++
	}
	if l.at(j) == '-' {
		j++
	}
	if l.at(j) != '.' {
		return -1
	}
	for l.at(j) == '.' {
		j++
	}
	if l.at(j) != '-' {
		return -1
	}
	j++
	if c := l.at(j); c == 'x' || c == 'o' || c == '>' {
		j++
	}
	return l.skipSpace(j)
}

// reDottedStart is /\s*[xo<]?-\.\s*/y.
func (l *lexer) reDottedStart(q int) int {
	j := l.skipSpace(q)
	if isXo(l.at(j)) {
		j++
	}
	if l.at(j) != '-' || l.at(j+1) != '.' {
		return -1
	}
	return l.skipSpace(j + 2)
}

// reInvisLink is /\s*~~~+\s*/y.
func (l *lexer) reInvisLink(q int) int {
	j := l.skipSpace(q)
	r := 0
	for j+r < l.n && l.src[j+r] == '~' {
		r++
	}
	if r < 3 {
		return -1
	}
	return l.skipSpace(j + r)
}

// reNewline is /(?:\r?\n)+/y.
func (l *lexer) reNewline(q int) int {
	j := q
	for {
		if l.at(j) == '\n' {
			j++
		} else if l.at(j) == '\r' && l.at(j+1) == '\n' {
			j += 2
		} else {
			break
		}
	}
	if j == q {
		return -1
	}
	return j
}

// reSpace is /[^\S\n\r]+/y.
func (l *lexer) reSpace(q int) int {
	j := q
	for j < l.n {
		c := l.src[j]
		if c == '\n' || c == '\r' {
			break
		}
		ok, size := l.spaceAt(j)
		if !ok {
			break
		}
		j += size
	}
	if j == q {
		return -1
	}
	return j
}

// reNodir is /(?:\r?\n)*\s*\n/y: the whitespace from q up to its last line feed.
func (l *lexer) reNodir(q int) int {
	e := -1
	j := q
	for j < l.n {
		ok, size := l.spaceAt(j)
		if !ok {
			break
		}
		if l.src[j] == '\n' {
			e = j + 1
		}
		j += size
	}
	return e
}

// reDir is /\s*(?:LR|RL|TB|BT|TD|BR)\b/y.
func (l *lexer) reDir(q int) int {
	j := l.skipSpace(q)
	if j+2 > l.n {
		return -1
	}
	switch l.src[j : j+2] {
	case "LR", "RL", "TB", "BT", "TD", "BR":
	default:
		return -1
	}
	if l.isWordAt(j + 2) {
		return -1
	}
	return j + 2
}

// reDirSym is /\s*[<>^]/y.
func (l *lexer) reDirSym(q int) int {
	j := l.skipSpace(q)
	if c := l.at(j); c == '<' || c == '>' || c == '^' {
		return j + 1
	}
	return -1
}

// reDirV is /\s*v\b/y.
func (l *lexer) reDirV(q int) int {
	j := l.skipSpace(q)
	if l.at(j) == 'v' && !l.isWordAt(j+1) {
		return j + 1
	}
	return -1
}

// reWordSep is /word\s*<sep>\s*/y.
func (l *lexer) reWordSep(q int, word string, sep byte) int {
	if !strings.HasPrefix(l.src[q:], word) {
		return -1
	}
	j := l.skipSpace(q + len(word))
	if l.at(j) != int(sep) {
		return -1
	}
	return l.skipSpace(j + 1)
}

// reWordSpace is /word\s+/y.
func (l *lexer) reWordSpace(q int, word string) int {
	if !strings.HasPrefix(l.src[q:], word) {
		return -1
	}
	j := l.skipSpace(q + len(word))
	if j == q+len(word) {
		return -1
	}
	return j
}

// reEmptyCall is /\(\s*\)/y.
func (l *lexer) reEmptyCall(q int) int {
	if l.at(q) != '(' {
		return -1
	}
	j := l.skipSpace(q + 1)
	if l.at(j) != ')' {
		return -1
	}
	return j + 1
}

// reDirection is /.*direction\s+<code>[^\n]*/y, where the line from q ends at lineEnd.
func (l *lexer) reDirection(q, lineEnd int, code string) int {
	seg := l.src[q:lineEnd]
	end := len(seg)
	for {
		k := strings.LastIndex(seg[:end], "direction")
		if k < 0 {
			return -1
		}
		j := q + k + 9
		s := l.skipSpace(j)
		if s > j && strings.HasPrefix(l.src[s:], code) {
			e := strings.IndexByte(l.src[s+2:], '\n')
			if e < 0 {
				return l.n
			}
			return s + 2 + e
		}
		end = k + 8
	}
}

// lineEndFrom is the offset of the first line terminator from q, or the length of the source.
func (l *lexer) lineEndFrom(q int) int {
	s := l.src
	for i := q; i < l.n; i++ {
		switch s[i] {
		case '\n', '\r':
			return i
		case 0xe2:
			if i+2 < l.n && s[i+1] == 0x80 && (s[i+2] == 0xa8 || s[i+2] == 0xa9) {
				return i
			}
		}
	}
	return l.n
}

// replaceBreaks is .replace(/\n\s*/g, '<br/>').
func replaceBreaks(s string) string {
	if !strings.Contains(s, "\n") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\n' {
			b.WriteString("<br/>")
			i++
			for i < len(s) {
				r, size := utf8.DecodeRuneInString(s[i:])
				if !js.IsSpace(r) {
					break
				}
				i += size
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// The first character after any run of whitespace starting at q, skipping one optional x, o or <.
func (l *lexer) linkChar(q int) int {
	j := q
	if q >= l.wsFrom && q <= l.wsTo {
		j = l.wsTo
	} else if l.isSpace(q) {
		j = l.skipSpace(q)
		l.wsFrom = q
		l.wsTo = j
	}
	d := l.at(j)
	if isXo(d) {
		return l.at(j + 1)
	}
	return d
}

func (l *lexer) invisAhead(q int) bool {
	j := q
	if q >= l.wsFrom && q <= l.wsTo {
		j = l.wsTo
	} else if l.isSpace(q) {
		j = l.skipSpace(q)
		l.wsFrom = q
		l.wsTo = j
	}
	return l.at(j) == '~'
}

func (l *lexer) quote() {
	if l.at(l.p+1) == '`' {
		l.push(sMdString)
		l.p += 2
	} else {
		l.push(sString)
		l.p++
	}
}

func (l *lexer) invisLink() bool {
	if !l.invisAhead(l.p) {
		return false
	}
	e := l.reInvisLink(l.p)
	if e < 0 {
		return false
	}
	l.emit(tLINK, l.p, e)
	return true
}

func (l *lexer) opener(c int) bool {
	p := l.p
	d := l.at(p + 1)
	switch c {
	case '(':
		if d == '-' {
			l.push(sEllipseText)
			l.emit(tELLIPSE_START, p, p+2)
		} else if d == '[' {
			l.push(sText)
			l.emit(tSTADIUMSTART, p, p+2)
		} else if d == '(' && l.at(p+2) == '(' {
			l.push(sText)
			l.emit(tDOUBLECIRCLESTART, p, p+3)
		} else {
			l.push(sText)
			l.emit(tPS, p, p+1)
		}
		return true
	case '[':
		switch d {
		case '[':
			l.push(sText)
			l.emit(tSUBROUTINESTART, p, p+2)
		case '(':
			l.push(sText)
			l.emit(tCYLINDERSTART, p, p+2)
		case '/':
			l.push(sTrapText)
			l.emit(tTRAPSTART, p, p+2)
		case '\\':
			l.push(sTrapText)
			l.emit(tINVTRAPSTART, p, p+2)
		default:
			l.push(sText)
			l.emit(tSQS, p, p+1)
		}
		return true
	case '|':
		l.push(sText)
		l.emit(tPIPE, p, p+1)
		return true
	case '{':
		l.push(sText)
		l.emit(tDIAMOND_START, p, p+1)
		return true
	}
	return false
}

func (l *lexer) graph(length int) {
	if l.firstGrap {
		l.firstGrap = false
		l.push(sDir)
	}
	l.emit(tGRAPH, l.p, l.p+length)
}

func (l *lexer) nodeString() bool {
	q := l.p
	for q < l.n {
		ch := l.src[q]
		if ch < 128 && nodeChar[ch] {
			q++
		} else if ch == '-' && q+1 < l.n {
			d := l.src[q+1]
			if d == '>' || d == '-' || d == '.' {
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
	l.emit(tNODE_STRING, l.p, q)
	return true
}

func (l *lexer) link(c int) bool {
	p := l.p
	d := l.linkChar(p)
	var e int
	switch d {
	case '-':
		if e = l.reLink(p, '-'); e >= 0 {
			l.emit(tLINK, p, e)
			return true
		}
		if e = l.reStartLink(p, '-'); e >= 0 {
			l.push(sEdgeText)
			l.emit(tSTART_LINK, p, e)
			return true
		}
		if e = l.reDottedLink(p); e >= 0 {
			l.emit(tLINK, p, e)
			return true
		}
		if e = l.reDottedStart(p); e >= 0 {
			l.push(sDottedEdgeText)
			l.emit(tSTART_LINK, p, e)
			return true
		}
	case '=':
		if e = l.reLink(p, '='); e >= 0 {
			l.emit(tLINK, p, e)
			return true
		}
		if e = l.reStartLink(p, '='); e >= 0 {
			l.push(sThickEdgeText)
			l.emit(tSTART_LINK, p, e)
			return true
		}
	case '.':
		if e = l.reDottedLink(p); e >= 0 {
			l.emit(tLINK, p, e)
			return true
		}
	}
	return c != '-' && c != '=' && c != '.' && l.invisLink()
}

func (l *lexer) initial(c int) bool {
	src := l.src
	p := l.p
	var e int
	switch c {
	case 'a':
		if strings.HasPrefix(src[p:], "acc") {
			if e = l.reWordSep(p, "accTitle", ':'); e >= 0 {
				l.push(sAccTitle)
				l.emit(tAccTitle, p, e)
				return true
			}
			if e = l.reWordSep(p, "accDescr", ':'); e >= 0 {
				l.push(sAccDescr)
				l.emit(tAccDescr, p, e)
				return true
			}
			if e = l.reWordSep(p, "accDescr", '{'); e >= 0 {
				l.push(sAccDescrMultiline)
				l.p = e
				return true
			}
		}
	case '@':
		if l.at(p+1) == '{' {
			l.push(sShapeData)
			l.tok.Types = append(l.tok.Types, tSHAPE_DATA)
			l.tok.Texts = append(l.tok.Texts, "")
			l.tok.Starts = append(l.tok.Starts, p)
			l.p += 2
			return true
		}
	case 'c':
		if e = l.reWordSpace(p, "call"); e >= 0 {
			l.push(sCallbackname)
			l.p = e
			return true
		}
		if l.kw("classDef") {
			l.emit(tCLASSDEF, p, p+8)
			return true
		}
		if l.kw("class") {
			l.emit(tCLASS, p, p+5)
			return true
		}
		if e = l.reWordSpace(p, "click"); e >= 0 {
			l.push(sClick)
			l.p = e
			return true
		}
	case '"':
		l.quote()
		return true
	case 's':
		if l.kw("style") {
			l.emit(tSTYLE, p, p+5)
			return true
		}
		if l.kw("swimlane-beta") {
			l.graph(13)
			return true
		}
		if l.kw("subgraph") {
			l.emit(tSubgraph, p, p+8)
			return true
		}
	case 'd':
		if l.kw("default") {
			l.emit(tDEFAULT, p, p+7)
			return true
		}
	case 'l':
		if l.kw("linkStyle") {
			l.emit(tLINKSTYLE, p, p+9)
			return true
		}
	case 'i':
		if l.kw("interpolate") {
			l.emit(tINTERPOLATE, p, p+11)
			return true
		}
	case 'h':
		if strings.HasPrefix(src[p:], "href") {
			if ok, size := l.spaceAt(p + 4); ok {
				l.emit(tHREF, p, p+4+size)
				return true
			}
		}
	case 'f':
		if l.kw("flowchart-elk") {
			l.graph(13)
			return true
		}
		if l.kw("flowchart") {
			l.graph(9)
			return true
		}
	case 'g':
		if l.kw("graph") {
			l.graph(5)
			return true
		}
	case 'e':
		if l.kw("end") {
			l.emit(tEnd, p, l.skipSpace(p+3))
			return true
		}
	case '_':
		for _, w := range [...]string{"_self", "_blank", "_parent", "_top"} {
			if l.kw(w) {
				l.emit(tLINK_TARGET, p, p+len(w))
				return true
			}
		}
	}

	if l.dirNext != -1 && p >= l.dirFailed {
		if l.dirNext < p {
			l.dirNext = indexFrom(src, "direction", p)
		}
		if l.dirNext != -1 {
			if l.lineEnd < p {
				l.lineEnd = l.lineEndFrom(p)
			}
			if l.dirNext < l.lineEnd {
				for i, code := range directionCodes {
					if e = l.reDirection(p, l.lineEnd, code); e >= 0 {
						l.emit(tDirectionTB+i, p, e)
						return true
					}
				}
				// No direction statement starts anywhere in the rest of this line either.
				l.dirFailed = l.lineEnd
			}
		}
	}

	// A link id is a run of non-space characters up to its last `@` that is not followed by `{` or `"`.
	if l.atNext != -1 && c != '"' && !l.isSpace(p) {
		if p >= l.runEnd {
			if l.atNext < p {
				l.atNext = indexFrom(src, "@", p)
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
					after := l.at(k + 1)
					if after != '{' && after != '"' {
						l.runAt = k
						break
					}
				}
			}
		}
		if l.runAt > p {
			l.emit(tLINK_ID, p, l.runAt+1)
			return true
		}
	}

	if c >= '0' && c <= '9' {
		q := p + 1
		for q < l.n && src[q] >= '0' && src[q] <= '9' {
			q++
		}
		l.emit(tNUM, p, q)
		return true
	}

	switch c {
	case '#':
		l.emit(tBRKT, p, p+1)
		return true
	case ':':
		if l.at(p+1) == ':' && l.at(p+2) == ':' {
			l.emit(tSTYLE_SEPARATOR, p, p+3)
		} else {
			l.emit(tCOLON, p, p+1)
		}
		return true
	case '&':
		l.emit(tAMP, p, p+1)
		return true
	case ';':
		l.emit(tSEMI, p, p+1)
		return true
	case ',':
		l.emit(tCOMMA, p, p+1)
		return true
	case '*':
		l.emit(tMULT, p, p+1)
		return true
	case 'x', 'o':
		return l.link(c) || l.nodeString()
	case '<':
		if l.link(c) {
			return true
		}
		l.emit(tTAGSTART, p, p+1)
		return true
	case '-':
		if l.link(c) || l.nodeString() {
			return true
		}
		l.emit(tMINUS, p, p+1)
		return true
	case '=':
		return l.link(c)
	case '.':
		return l.link(c) || l.nodeString()
	case '~':
		return l.invisLink()
	case '(':
		return l.opener(c)
	case '[':
		if l.at(p+1) == '|' {
			l.emit(tVERTEX_WITH_PROPS_START, p, p+2)
			return true
		}
		return l.opener(c)
	case '|', '{':
		return l.opener(c)
	case '>':
		l.push(sText)
		l.emit(tTAGEND, p, p+1)
		return true
	case '^':
		l.emit(tUP, p, p+1)
		return true
	case '\\':
		if l.at(p+1) == '|' {
			l.emit(tSEP, p, p+2)
			return true
		}
		return l.nodeString()
	case 'v':
		if !l.isWordAt(p + 1) {
			l.emit(tDOWN, p, p+1)
			return true
		}
		return l.nodeString()
	}

	if l.isSpace(p) {
		if l.link(c) {
			return true
		}
		if e = l.reNewline(p); e >= 0 {
			l.emit(tNEWLINE, p, e)
			return true
		}
		if e = l.reSpace(p); e >= 0 {
			l.emit(tSPACE, p, e)
			return true
		}
		return false
	}

	if c < 128 {
		return l.nodeString()
	}
	r, size := utf8.DecodeRuneInString(src[p:])
	// An astral character is two code units; its first, a surrogate, is no letter.
	if r < 0x10000 && util.IsUnicodeLetter(r) {
		l.emit(tUNICODE_TEXT, p, p+size)
		return true
	}
	return false
}

func indexFrom(s, sub string, from int) int {
	if from > len(s) {
		return -1
	}
	i := strings.Index(s[from:], sub)
	if i < 0 {
		return -1
	}
	return from + i
}

func (l *lexer) edgeLink(q int, kind int) int {
	switch kind {
	case sEdgeText:
		return l.reLink(q, '-')
	case sThickEdgeText:
		return l.reLink(q, '=')
	}
	return l.reDottedLink(q)
}

func (l *lexer) edgeText(c int, kind int, a, b int) bool {
	if c == '"' {
		l.quote()
		return true
	}
	p := l.p
	d := l.linkChar(p)
	if d == a || d == b {
		if e := l.edgeLink(p, kind); e >= 0 {
			l.pop()
			l.emit(tLINK, p, e)
			return true
		}
	}
	if c == a && (a != '-' || l.at(p+1) == '-') {
		return false
	}
	q := p + l.charSize(p)
	for q < l.n {
		ch := int(l.src[q])
		size := 1
		if ch == '"' {
			break
		}
		if ch == a {
			if a != '-' || l.at(q+1) == '-' {
				break
			}
		} else {
			space := false
			if ch >= utf8.RuneSelf {
				space, size = l.spaceAt(q)
			} else {
				space = l.isSpace(q)
			}
			if ch == 'x' || ch == 'o' || ch == '<' || ch == b || space {
				dd := l.linkChar(q)
				if (dd == a || dd == b) && l.edgeLink(q, kind) >= 0 {
					break
				}
			}
		}
		q += size
	}
	l.emit(tEDGE_TEXT, p, q)
	return true
}

func (l *lexer) charSize(i int) int {
	if i >= l.n || l.src[i] < utf8.RuneSelf {
		return 1
	}
	_, size := utf8.DecodeRuneInString(l.src[i:])
	return size
}

// Tokenize splits flowchart source into tokens, ending with an END token, or an ERROR token
// where no token matches.
func Tokenize(src string) Tokens {
	l := &lexer{
		src:       src,
		n:         len(src),
		firstGrap: true,
		dirNext:   strings.Index(src, "direction"),
		atNext:    strings.Index(src, "@"),
		lineEnd:   -1,
		runAt:     -1,
		wsFrom:    -1,
		wsTo:      -1,
	}
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
				l.emit(typ, n, n)
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
					l.p += 2
				} else {
					l.pop()
					l.p++
				}
			} else {
				e := indexFrom(src, `"`, p)
				if e == -1 {
					e = n
				}
				l.emit(tSTR, p, e)
			}

		case sMdString:
			if c == '`' {
				if l.at(p+1) == '"' {
					l.pop()
					l.p += 2
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
				l.emit(tMD_STR, p, q)
			}

		case sAccTitle, sAccDescr:
			e := indexFrom(src, "\n", p)
			if e == -1 {
				e = n
			}
			typ := tAccDescrValue
			if l.state == sAccTitle {
				typ = tAccTitleValue
			}
			l.pop()
			l.emit(typ, p, e)

		case sAccDescrMultiline:
			if c == '}' {
				l.pop()
				l.p++
			} else {
				e := indexFrom(src, "}", p)
				if e == -1 {
					e = n
				}
				l.emit(tAccDescrMultilineValue, p, e)
			}

		case sDir:
			var e int
			if c == '"' {
				l.quote()
			} else if e = l.reNodir(p); e >= 0 {
				l.pop()
				l.emit(tNODIR, p, e)
			} else if e = l.reDir(p); e >= 0 {
				l.pop()
				l.emit(tDIR, p, e)
			} else if e = l.reDirSym(p); e >= 0 {
				l.pop()
				l.emit(tDIR, p, e)
			} else if e = l.reDirV(p); e >= 0 {
				l.pop()
				l.emit(tDIR, p, e)
			} else {
				ok = l.invisLink() || l.opener(c)
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
					l.emit(tSTADIUMEND, p, p+2)
				} else if d == ']' {
					l.emit(tSUBROUTINEEND, p, p+2)
				} else {
					l.emit(tSQE, p, p+1)
				}
			case ')':
				l.pop()
				if d == ']' {
					l.emit(tCYLINDEREND, p, p+2)
				} else if d == ')' && l.at(p+2) == ')' {
					l.emit(tDOUBLECIRCLEEND, p, p+3)
				} else {
					l.emit(tPE, p, p+1)
				}
			case '|':
				l.pop()
				l.emit(tPIPE, p, p+1)
			case '}':
				l.pop()
				l.emit(tDIAMOND_STOP, p, p+1)
			default:
				if (c == '~' || l.isSpace(p)) && l.invisLink() {
					break
				}
				q := p + 1
				for q < n {
					ch := src[q]
					if ch == '[' || ch == ']' || ch == '(' || ch == ')' || ch == '{' || ch == '}' || ch == '|' || ch == '"' {
						break
					}
					q++
				}
				l.emit(tTEXT, p, q)
			}

		case sEllipseText:
			d := l.at(p + 1)
			if c == '"' {
				l.quote()
			} else if (c == '~' || l.isSpace(p)) && l.invisLink() {
				break
			} else if (c == '-' || c == '/' || c == ')') && d == ')' {
				l.pop()
				l.emit(tELLIPSE_END, p, p+2)
			} else if c == '(' || c == '[' || c == '{' {
				l.opener(c)
			} else if c == ')' || c == ']' || c == '}' {
				ok = false
			} else {
				q := p + l.charSize(p)
				for q < n {
					ch := src[q]
					if ch == '"' || ch == '(' || ch == ')' || ch == '[' || ch == ']' || ch == '{' || ch == '}' {
						break
					}
					if (ch == '-' || ch == '/') && l.at(q+1) == ')' {
						break
					}
					space, size := false, 1
					if ch >= utf8.RuneSelf {
						space, size = l.spaceAt(q)
					} else {
						space = l.isSpace(q)
					}
					if (ch == '~' || space) && l.invisAhead(q) && l.reInvisLink(q) >= 0 {
						break
					}
					q += size
				}
				l.emit(tTEXT, p, q)
			}

		case sTrapText:
			d := l.at(p + 1)
			if c == '"' {
				l.quote()
				break
			}
			if (c == '~' || l.isSpace(p)) && l.invisLink() {
				break
			}
			switch c {
			case '(':
				if d == '-' || d == '[' || (d == '(' && l.at(p+2) == '(') {
					l.opener(c)
				} else if d == ']' {
					l.pop()
					l.emit(tTRAPEND, p, p+2)
				} else {
					l.opener(c)
				}
			case '[', '{':
				l.opener(c)
			case '\\':
				if d == ']' {
					l.pop()
					l.emit(tTRAPEND, p, p+2)
				} else {
					l.emit(tTEXT, p, p+1)
				}
			case '/':
				if d == ']' {
					l.pop()
					l.emit(tINVTRAPEND, p, p+2)
				} else {
					l.emit(tTEXT, p, p+1)
				}
			case ']', ')':
				if d == ']' {
					l.pop()
					l.emit(tTRAPEND, p, p+2)
				} else {
					ok = false
				}
			case '}':
				ok = false
			default:
				if (c == '?' || c == '=') && d == ']' {
					l.pop()
					l.emit(tTRAPEND, p, p+2)
					break
				}
				q := p + 1
				for q < n {
					ch := src[q]
					if ch == '\\' || ch == '[' || ch == ']' || ch == '(' || ch == ')' || ch == '{' || ch == '}' || ch == '/' {
						break
					}
					q++
				}
				l.emit(tTEXT, p, q)
			}

		case sEdgeText:
			ok = l.edgeText(c, sEdgeText, '-', -1)
		case sThickEdgeText:
			ok = l.edgeText(c, sThickEdgeText, '=', -1)
		case sDottedEdgeText:
			ok = l.edgeText(c, sDottedEdgeText, '.', '-')

		case sClick:
			if c == '"' {
				l.quote()
			} else if space, size := l.spaceAt(p); space {
				l.pop()
				l.p += size
			} else {
				q := p + size
				for q < n {
					space, sz := l.spaceAt(q)
					if space {
						break
					}
					q += sz
				}
				l.emit(tCLICK, p, q)
			}

		case sCallbackname:
			if c == '(' {
				e := l.reEmptyCall(p)
				l.pop()
				if e >= 0 {
					l.p = e
				} else {
					l.push(sCallbackargs)
					l.p++
				}
			} else {
				e := indexFrom(src, "(", p)
				if e == -1 {
					e = n
				}
				l.emit(tCALLBACKNAME, p, e)
			}

		case sCallbackargs:
			if c == ')' {
				l.pop()
				l.p++
			} else {
				e := indexFrom(src, ")", p)
				if e == -1 {
					e = n
				}
				l.emit(tCALLBACKARGS, p, e)
			}

		case sShapeData:
			if c == '"' {
				l.push(sShapeDataStr)
				l.emit(tSHAPE_DATA, p, p+1)
			} else if c == '}' {
				l.pop()
				l.p++
			} else if c == '^' {
				ok = false
			} else {
				q := p + 1
				for q < n && src[q] != '}' && src[q] != '^' && src[q] != '"' {
					q++
				}
				l.emit(tSHAPE_DATA, p, q)
			}

		case sShapeDataStr:
			if c == '"' {
				l.pop()
				l.emit(tSHAPE_DATA, p, p+1)
			} else {
				e := indexFrom(src, `"`, p)
				if e == -1 {
					e = n
				}
				l.tok.Types = append(l.tok.Types, tSHAPE_DATA)
				l.tok.Texts = append(l.tok.Texts, replaceBreaks(src[p:e]))
				l.tok.Starts = append(l.tok.Starts, p)
				l.p = e
			}
		}

		if !ok {
			l.tok.Types = append(l.tok.Types, tERROR)
			l.tok.Texts = append(l.tok.Texts, "")
			l.tok.Starts = append(l.tok.Starts, l.p)
			return l.tok
		}
	}

	l.tok.Types = append(l.tok.Types, tEND)
	l.tok.Texts = append(l.tok.Texts, "")
	l.tok.Starts = append(l.tok.Starts, n)
	return l.tok
}

package block

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
)

// lexTokens holds the tokens with their byte offsets in the source.
type lexTokens struct {
	types  []int
	texts  []string
	starts []int
}

const (
	sINITIAL = iota
	sString
	sMdString
	sNODE
	sBLOCK_ARROW
	sARROW_DIR
	sLLABEL
	sCLASSDEF
	sCLASSDEFID
	sCLASS
	sCLASS_STYLE
	sSTYLE_STMNT
	sSTYLE_DEFINITION
	sAccTitle
	sAccDescr
	sAccDescrMultiline
)

var directions = []string{"right", "left", "x", "y", "up", "down"}

// idStop marks the characters that end a node id: ( ) [ { } < > - : = and whitespace.
var idStop = func() (t [128]bool) {
	for _, c := range []int{9, 10, 11, 12, 13, 32, 40, 41, 45, 58, 60, 61, 62, 91, 123, 125} {
		t[c] = true
	}
	return
}()

func isWordByte(c int) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_'
}

func tokenize(src string) lexTokens {
	n := len(src)
	var out lexTokens
	var stack []int
	state := sINITIAL
	p := 0

	at := func(q int) int {
		if q < 0 || q >= n {
			return -1
		}
		return int(src[q])
	}
	put := func(typ int, text string, end int) bool {
		out.types = append(out.types, typ)
		out.texts = append(out.texts, text)
		out.starts = append(out.starts, p)
		p = end
		return true
	}
	emit := func(typ int, end int) bool { return put(typ, src[p:end], end) }
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
	ws := func(q int) int {
		for q < n {
			if c := src[q]; c < 0x80 {
				if c != ' ' && (c < 9 || c > 13) {
					break
				}
				q++
				continue
			}
			r, size := js.RuneAt(src, q)
			if !js.IsSpace(r) {
				break
			}
			q += size
		}
		return q
	}
	digits := func(q int) int {
		for q < n && src[q] >= '0' && src[q] <= '9' {
			q++
		}
		return q
	}
	word := func(q int) int {
		for q < n && isWordByte(int(src[q])) {
			q++
		}
		return q
	}
	startsWith := func(text string, q int) bool { return q <= n && strings.HasPrefix(src[q:], text) }
	keyword := func(text string) bool { return startsWith(text, p) && !isWordByte(at(p+len(text))) }

	quote := func() bool {
		if at(p+1) == '`' {
			push(sMdString)
			p += 2
		} else {
			push(sString)
			p++
		}
		return true
	}

	// End of `.+-[xo>]?` at q, or -1.
	dotted := func(q int) int {
		if at(q) != '.' {
			return -1
		}
		q++
		for at(q) == '.' {
			q++
		}
		if at(q) != '-' {
			return -1
		}
		c := at(q + 1)
		if c == 'x' || c == 'o' || c == '>' {
			return q + 2
		}
		return q + 1
	}

	// End of a complete link at q, before its trailing whitespace, or -1.
	linkEnd := func(q int) int {
		c := at(q)
		if c == 'x' || c == 'o' || c == '<' {
			q++
		}
		line := at(q)
		if line == '.' {
			return dotted(q)
		}
		if line != '-' && line != '=' {
			return -1
		}
		k := q + 1
		for at(k) == line {
			k++
		}
		if k-q == 1 {
			if line == '-' {
				return dotted(k)
			}
			return -1
		}
		head := at(k)
		if head == 'x' || head == 'o' || head == '>' {
			return k + 1
		}
		if k-q >= 3 {
			return k
		}
		return -1
	}

	link := func() bool {
		if e := linkEnd(p); e >= 0 {
			return emit(tLINK, ws(e))
		}
		q := p
		if at(p) == '<' {
			q = p + 1
		}
		a := at(q)
		b := at(q + 1)
		if (a == '-' && (b == '-' || b == '.')) || (a == '=' && b == '=') {
			push(sLLABEL)
			return emit(tSTART_LINK, ws(q+2))
		}
		return false
	}

	lineState := func(typ int) bool {
		e := strings.IndexByte(src[p:], '\n')
		if e == -1 {
			e = n
		} else {
			e += p
		}
		pop()
		return emit(typ, e)
	}

	// `\w+(,\s*\w+)*`
	idList := func(typ, next int) bool {
		k := word(p)
		if k == p {
			return false
		}
		for at(k) == ',' {
			from := ws(k + 1)
			to := word(from)
			if to == from {
				break
			}
			k = to
		}
		state = next
		return emit(typ, k)
	}

	initial := func(c int) bool {
		if c < 0x80 {
			if c == ' ' || (c >= 9 && c <= 13) {
				p = ws(p + 1)
				return true
			}
		} else if r, size := js.RuneAt(src, p); js.IsSpace(r) {
			p = ws(p + size)
			return true
		}
		var q int
		switch c {
		case 'b':
			if startsWith("block", p) {
				if keyword("block-beta") {
					return emit(tKEY, p+10)
				}
				if at(p+5) == ':' {
					return emit(tID_BLOCK, p+6)
				}
				if !isWordByte(at(p + 5)) {
					return emit(tKEY, p+5)
				}
			}
		case 'c':
			if startsWith("columns", p) {
				q = ws(p + 7)
				if q > p+7 {
					if startsWith("auto", q) && !isWordByte(at(q+4)) {
						return put(tCOLUMNS, "-1", q+4)
					}
					if e := digits(q); e > q {
						return put(tCOLUMNS, src[q:e], e)
					}
				}
			} else if startsWith("class", p) {
				def := startsWith("Def", p+5)
				from := p + 5
				if def {
					from = p + 8
				}
				q = ws(from)
				if q > from {
					if def {
						push(sCLASSDEF)
						return emit(tCLASSDEF, q)
					}
					push(sCLASS)
					return emit(tCLASS, q)
				}
			}
		case '"':
			return quote()
		case 's':
			if startsWith("space", p) {
				if at(p+5) == ':' {
					if e := digits(p + 6); e > p+6 {
						return put(tSPACE_BLOCK, src[p+6:e], e)
					}
				}
				if !isWordByte(at(p + 5)) {
					return put(tSPACE_BLOCK, "1", p+5)
				}
			} else if startsWith("style", p) {
				q = ws(p + 5)
				if q > p+5 {
					push(sSTYLE_STMNT)
					return emit(tSTYLE, q)
				}
			}
		case 'd':
			if keyword("default") {
				return emit(tDEFAULT, p+7)
			}
		case 'l':
			if keyword("linkStyle") {
				return emit(tLINKSTYLE, p+9)
			}
		case 'i':
			if keyword("interpolate") {
				return emit(tINTERPOLATE, p+11)
			}
		case 'a':
			if startsWith("accTitle", p) {
				q = ws(p + 8)
				if at(q) == ':' {
					push(sAccTitle)
					return emit(tAccTitle, ws(q+1))
				}
			} else if startsWith("accDescr", p) {
				q = ws(p + 8)
				d := at(q)
				if d == ':' {
					push(sAccDescr)
					return emit(tAccDescr, ws(q+1))
				}
				if d == '{' {
					push(sAccDescrMultiline)
					p = ws(q + 1)
					return true
				}
			}
		case 'e':
			if keyword("end") {
				return emit(tEnd, ws(p+3))
			}
		case '-':
			if at(p+1) == ')' {
				push(sNODE)
				return emit(tNODE_DSTART, p+2)
			}
			return link()
		case '=':
			return link()
		case '<':
			if at(p+1) == '[' {
				push(sBLOCK_ARROW)
				return emit(tBLOCK_ARROW_START, p+2)
			}
			return link()
		case '(':
			d := at(p + 1)
			l := 1
			if d == '-' || d == '[' {
				l = 2
			} else if d == '(' {
				l = 2
				if at(p+2) == '(' {
					l = 3
				}
			}
			push(sNODE)
			return emit(tNODE_DSTART, p+l)
		case ')', '{':
			push(sNODE)
			if at(p+1) == c {
				return emit(tNODE_DSTART, p+2)
			}
			return emit(tNODE_DSTART, p+1)
		case '>':
			push(sNODE)
			return emit(tNODE_DSTART, p+1)
		case '[':
			d := at(p + 1)
			push(sNODE)
			if d == '[' || d == '|' || d == '(' || d == '\\' || d == '/' {
				return emit(tNODE_DSTART, p+2)
			}
			return emit(tNODE_DSTART, p+1)
		case ':':
			if e := digits(p + 1); e > p+1 {
				return put(tSIZE, src[p+1:e], e)
			}
			return false
		}

		q = p
		for q < n {
			ch := src[q]
			if ch < 0x80 {
				if idStop[ch] {
					break
				}
				q++
				continue
			}
			r, size := js.RuneAt(src, q)
			if js.IsSpace(r) {
				break
			}
			q += size
		}
		return q > p && emit(tNODE_ID, q)
	}

	nodeEnd := func(c int) bool {
		d := at(p + 1)
		l := 0
		switch c {
		case '"':
			return quote()
		case '(':
			l = 1
			if d == '-' {
				l = 2
			} else if d == '(' {
				l = 2
				if at(p+2) == '(' {
					l = 3
				}
			}
		case ')':
			l = 1
			if d == ')' {
				l = 2
				if at(p+2) == ')' {
					l = 3
				}
			} else if d == ']' {
				l = 2
			}
		case '}':
			l = 1
			if d == '}' {
				l = 2
			}
		case '-':
			if d == ')' {
				l = 2
			}
		case ']':
			l = 1
			if d == ']' || d == ')' || d == '>' {
				l = 2
			}
		case '\\', '/':
			if d == ']' {
				l = 2
			}
		}
		if l == 0 {
			return false
		}
		pop()
		return emit(tNODE_DEND, p+l)
	}

	arrowDir := func(c int) bool {
		if c == ')' {
			pop()
			pop()
			return put(tBLOCK_ARROW_END, "]>", ws(p+1))
		}
		from := p
		if c == ',' {
			from = p + 1
		}
		from = ws(from)
		for _, dir := range directions {
			if startsWith(dir, from) {
				e := ws(from + len(dir))
				if c == ',' {
					return put(tDIR, src[from:e], e)
				}
				return put(tDIR, src[p:e], e)
			}
		}
		return false
	}

	for {
		if p >= n {
			typ := -1
			switch state {
			case sINITIAL:
				typ = tEOF
			case sString:
				typ = tSTR
			case sCLASSDEFID:
				typ = tCLASSDEF_STYLEOPTS
			case sCLASS_STYLE:
				typ = tSTYLECLASS
			case sSTYLE_DEFINITION:
				typ = tSTYLE_DEFINITION_DATA
			case sAccTitle:
				typ = tAccTitleValue
			case sAccDescr:
				typ = tAccDescrValue
			case sAccDescrMultiline:
				typ = tAccDescrMultilineValue
			}
			p = n
			if typ != -1 {
				put(typ, "", n)
			}
			break
		}

		c := int(src[p])
		ok := true

		switch state {
		case sINITIAL:
			ok = initial(c)

		case sString:
			if c == '"' {
				pop()
				p++
			} else {
				e := strings.IndexByte(src[p:], '"')
				if e == -1 {
					emit(tSTR, n)
				} else {
					emit(tSTR, p+e)
				}
			}

		case sMdString:
			if c == '`' {
				ok = at(p+1) == '"'
				if ok {
					pop()
					p += 2
				}
			} else if c == '"' {
				ok = false
			} else {
				q := p + 1
				for q < n && src[q] != '`' && src[q] != '"' {
					q++
				}
				emit(tMD_STR, q)
			}

		case sNODE:
			ok = nodeEnd(c)

		case sBLOCK_ARROW:
			if c == '"' {
				quote()
			} else if c == ']' && at(p+1) == '>' {
				q := ws(p + 2)
				ok = at(q) == '('
				if ok {
					push(sARROW_DIR)
					p = q + 1
				}
			} else {
				ok = false
			}

		case sARROW_DIR:
			ok = arrowDir(c)

		case sLLABEL:
			if c == '"' {
				if at(p+1) == '`' {
					quote()
				} else {
					emit(tLINK_LABEL, p+1)
					push(sString)
				}
			} else {
				e := linkEnd(ws(p))
				ok = e >= 0
				if ok {
					pop()
					emit(tLINK, ws(e))
				}
			}

		case sCLASSDEF:
			typ := tDEFAULT_CLASSDEF_ID
			k := p
			if startsWith("DEFAULT", p) {
				k = p + 7
			}
			q := ws(k)
			if k == p || q == k {
				typ = tCLASSDEF_ID
				k = word(p)
				q = ws(k)
			}
			ok = k > p && q > k
			if ok {
				state = sCLASSDEFID
				emit(typ, q)
			}

		case sCLASSDEFID:
			lineState(tCLASSDEF_STYLEOPTS)

		case sCLASS:
			ok = idList(tCLASSENTITY_IDS, sCLASS_STYLE)

		case sCLASS_STYLE:
			lineState(tSTYLECLASS)

		case sSTYLE_STMNT:
			ok = idList(tSTYLE_ENTITY_IDS, sSTYLE_DEFINITION)

		case sSTYLE_DEFINITION:
			lineState(tSTYLE_DEFINITION_DATA)

		case sAccTitle:
			lineState(tAccTitleValue)

		case sAccDescr:
			lineState(tAccDescrValue)

		case sAccDescrMultiline:
			if c == '}' {
				pop()
				p++
			} else {
				e := strings.IndexByte(src[p:], '}')
				if e == -1 {
					emit(tAccDescrMultilineValue, n)
				} else {
					emit(tAccDescrMultilineValue, p+e)
				}
			}
		}

		if !ok {
			out.types = append(out.types, tERROR)
			out.texts = append(out.texts, "")
			out.starts = append(out.starts, p)
			return out
		}
	}

	out.types = append(out.types, tEND)
	out.texts = append(out.texts, "")
	out.starts = append(out.starts, n)
	return out
}

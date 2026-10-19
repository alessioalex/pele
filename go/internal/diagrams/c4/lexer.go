package c4

import (
	"strings"
	"unicode/utf8"

	"github.com/alessioalex/pele/go/internal/js"
)

// Token text is src[starts[i]:ends[i]], in byte offsets.
type tokens struct {
	types  []int
	starts []int
	ends   []int
}

const (
	sInitial = iota
	sStruct
	sAttribute
	sString
	sStringKV
	sStringKVKey
	sStringKVValue
	sAcc
	sAccDescrMultiline
)

var keywords = map[string]int{
	"C4Context":           tC4Context,
	"C4Container":         tC4Container,
	"C4Component":         tC4Component,
	"C4Dynamic":           tC4Dynamic,
	"C4Deployment":        tC4Deployment,
	"Person":              tPERSON,
	"Person_Ext":          tPERSON_EXT,
	"System":              tSYSTEM,
	"SystemDb":            tSYSTEM_DB,
	"SystemQueue":         tSYSTEM_QUEUE,
	"System_Ext":          tSYSTEM_EXT,
	"SystemDb_Ext":        tSYSTEM_EXT_DB,
	"SystemQueue_Ext":     tSYSTEM_EXT_QUEUE,
	"Container":           tCONTAINER,
	"ContainerDb":         tCONTAINER_DB,
	"ContainerQueue":      tCONTAINER_QUEUE,
	"Container_Ext":       tCONTAINER_EXT,
	"ContainerDb_Ext":     tCONTAINER_EXT_DB,
	"ContainerQueue_Ext":  tCONTAINER_EXT_QUEUE,
	"Component":           tCOMPONENT,
	"ComponentDb":         tCOMPONENT_DB,
	"ComponentQueue":      tCOMPONENT_QUEUE,
	"Component_Ext":       tCOMPONENT_EXT,
	"ComponentDb_Ext":     tCOMPONENT_EXT_DB,
	"ComponentQueue_Ext":  tCOMPONENT_EXT_QUEUE,
	"Enterprise_Boundary": tENTERPRISE_BOUNDARY,
	"System_Boundary":     tSYSTEM_BOUNDARY,
	"Boundary":            tBOUNDARY,
	"Container_Boundary":  tCONTAINER_BOUNDARY,
	"Deployment_Node":     tNODE,
	"Node":                tNODE,
	"Node_L":              tNODE_L,
	"Node_R":              tNODE_R,
	"Rel":                 tREL,
	"BiRel":               tBIREL,
	"Rel_Up":              tREL_U,
	"Rel_U":               tREL_U,
	"Rel_Down":            tREL_D,
	"Rel_D":               tREL_D,
	"Rel_Left":            tREL_L,
	"Rel_L":               tREL_L,
	"Rel_Right":           tREL_R,
	"Rel_R":               tREL_R,
	"Rel_Back":            tREL_B,
	"RelIndex":            tREL_INDEX,
	"UpdateElementStyle":  tUPDATE_EL_STYLE,
	"UpdateRelStyle":      tUPDATE_REL_STYLE,
	"UpdateLayoutConfig":  tUPDATE_LAYOUT_CONFIG,
}

var directions = [4]string{"TB", "BT", "RL", "LR"}

// spaceAt reports whether the character at byte offset q is `\s`, and its length.
func spaceAt(src string, q int) (bool, int) {
	if q < 0 || q >= len(src) {
		return false, 0
	}
	if c := src[q]; c < utf8.RuneSelf {
		return c == ' ' || (c >= 9 && c <= 13), 1
	}
	r, size := utf8.DecodeRuneInString(src[q:])
	return js.IsSpace(r), size
}

// byteAt is the byte at q, or -1 past the end. Only ever compared with ASCII.
func byteAt(src string, q int) int {
	if q < 0 || q >= len(src) {
		return -1
	}
	return int(src[q])
}

func isWordByte(c int) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_'
}

// lineEndFrom is where /[\n\r  ]/ first matches from p, or len(src).
func lineEndFrom(src string, p int) int {
	for q := p; q < len(src); q++ {
		c := src[q]
		if c == '\n' || c == '\r' {
			return q
		}
		if c == 0xe2 && q+2 < len(src) && src[q+1] == 0x80 && (src[q+2] == 0xa8 || src[q+2] == 0xa9) {
			return q
		}
	}
	return len(src)
}

// matchDirection is the sticky /.*direction\s+XX[^\n]*/y at p, returning the end of the match. The greedy
// `.*` makes it the last `direction` before the line ends; `\s+` may run on over line breaks.
func matchDirection(src string, p, lineEnd int, word string) (int, bool) {
	end := lineEnd
	for end > p {
		d := strings.LastIndex(src[p:end], "direction")
		if d < 0 {
			return 0, false
		}
		d += p
		end = d
		q := d + len("direction")
		spaces := 0
		for {
			ok, size := spaceAt(src, q)
			if !ok {
				break
			}
			q += size
			spaces++
		}
		if spaces == 0 || !strings.HasPrefix(src[q:], word) {
			continue
		}
		q += len(word)
		if nl := strings.IndexByte(src[q:], '\n'); nl >= 0 {
			return q + nl, true
		}
		return len(src), true
	}
	return 0, false
}

// Follows the lexer Jison compiles from c4Diagram.jison: its rule order, its start conditions,
// and what it returns once the input runs out in each of them.
func tokenize(src string) tokens {
	n := len(src)
	var tk tokens
	state := sInitial
	accValue := tAccTitleValue
	p := 0
	done := false
	// Where the next `direction` is; -2 until the first search.
	dirNext := -2
	lineEnd := -1
	dirFailed := 0

	emit := func(typ, s, e int) {
		tk.types = append(tk.types, typ)
		tk.starts = append(tk.starts, s)
		tk.ends = append(tk.ends, e)
		p = e
	}
	skipWs := func(q int) int {
		for {
			ok, size := spaceAt(src, q)
			if !ok {
				return q
			}
			q += size
		}
	}
	upTo := func(ch byte, from int) int {
		if from >= n {
			return n
		}
		q := strings.IndexByte(src[from:], ch)
		if q == -1 {
			return n
		}
		return from + q
	}
	// `word\s[^#\n;]+`
	statement := func(word string, typ int) bool {
		if !strings.HasPrefix(src[p:], word) {
			return false
		}
		q := p + len(word)
		ok, size := spaceAt(src, q)
		if !ok {
			return false
		}
		q += size
		from := q
		for q < n {
			c := src[q]
			if c == '#' || c == '\n' || c == ';' {
				break
			}
			q++
		}
		if q == from {
			return false
		}
		emit(typ, p, q)
		return true
	}

loop:
	for {
		// Jison notes the end of input one call before it reports it, so a rule that matches
		// nothing gets one last turn.
		if done {
			emit(tEND, n, n)
			break
		}
		if p >= n {
			done = true
		}
		c := byteAt(src, p)

		switch state {
		case sInitial:
			if p >= n {
				emit(tEOF, n, n)
				continue
			}
			if dirNext != -1 && p >= dirFailed {
				if dirNext < p {
					dirNext = strings.Index(src[p:], "direction")
					if dirNext != -1 {
						dirNext += p
					}
				}
				if dirNext != -1 {
					if lineEnd < p {
						lineEnd = lineEndFrom(src, p)
					}
					if dirNext < lineEnd {
						matched := false
						for i := 0; i < 4 && !matched; i++ {
							if end, ok := matchDirection(src, p, lineEnd, directions[i]); ok {
								emit(tDirectionTB+i, p, end)
								matched = true
							}
						}
						if matched {
							continue
						}
						// No direction statement starts anywhere in the rest of this line either.
						dirFailed = lineEnd
					}
				}
			}
			if c == 't' && statement("title", tTitle) {
				continue
			}
			if c == 'a' {
				if statement("accDescription", tAccDescription) {
					continue
				}
				title := strings.HasPrefix(src[p:], "accTitle")
				if title || strings.HasPrefix(src[p:], "accDescr") {
					q := skipWs(p + 8)
					d := byteAt(src, q)
					if d == ':' {
						if title {
							emit(tAccTitle, p, skipWs(q+1))
							accValue = tAccTitleValue
						} else {
							emit(tAccDescr, p, skipWs(q+1))
							accValue = tAccDescrValue
						}
						state = sAcc
						continue
					}
					if d == '{' && !title {
						p = skipWs(q + 1)
						state = sAccDescrMultiline
						continue
					}
				}
			}
			if c == '%' && byteAt(src, p+1) == '%' {
				// A comment takes the line breaks after it, so no NEWLINE follows.
				q := upTo('\n', p+2)
				for q < n && (src[q] == '\n' || src[q] == '\r') {
					q++
				}
				p = q
				continue
			}
			if ok, _ := spaceAt(src, p); ok {
				q := p
				last := -1
				for q < n {
					ok, size := spaceAt(src, q)
					if !ok {
						break
					}
					if src[q] == '\n' {
						last = q
					}
					q += size
				}
				if last != -1 {
					emit(tNEWLINE, p, last+1)
				} else {
					p = q
				}
				continue
			}
			if c == '{' {
				emit(tLBRACE, p, p+1)
				continue
			}
			if c == '}' {
				emit(tRBRACE, p, p+1)
				continue
			}
			if isWordByte(c) {
				q := p + 1
				for q < n && isWordByte(int(src[q])) {
					q++
				}
				if q-p <= 19 {
					if typ, ok := keywords[src[p:q]]; ok {
						if typ > tC4Deployment {
							state = sStruct
						}
						emit(typ, p, q)
						continue
					}
				}
			}
			emit(tERROR, p, p)
			break loop

		case sStruct:
			switch {
			case p >= n:
				emit(tEOF_IN_STRUCT, n, n)
			case c == '(':
				q := p + 1
				for byteAt(src, q) == ' ' {
					q++
				}
				if byteAt(src, q) == ',' {
					emit(tATTRIBUTE_EMPTY, p, q+1)
				} else {
					p++
				}
				state = sAttribute
			case c == ')':
				p++
				state = sInitial
			default:
				emit(tERROR, p, p)
				break loop
			}

		case sAttribute:
			if p >= n {
				continue
			}
			switch c {
			case ')':
				p++
				state = sInitial
			case ',':
				if byteAt(src, p+1) == ',' {
					emit(tATTRIBUTE_EMPTY, p, p+2)
				} else {
					p++
				}
			default:
				q := p
				for byteAt(src, q) == ' ' {
					q++
				}
				switch byteAt(src, q) {
				case '"':
					if byteAt(src, q+1) == '"' {
						emit(tATTRIBUTE_EMPTY, p, q+2)
					} else {
						p = q + 1
						state = sString
					}
				case '$':
					p = q + 1
					state = sStringKV
				default:
					// An unquoted value runs to the next comma, through closing parentheses and line breaks.
					emit(tSTR, p, upTo(',', p))
				}
			}

		case sString:
			if c == '"' {
				p++
				state = sAttribute
			} else {
				emit(tSTR, p, upTo('"', p))
			}

		case sStringKV:
			emit(tSTR_KEY, p, upTo('=', p))
			state = sStringKVKey

		case sStringKVKey:
			if p >= n {
				continue
			}
			q := p + 1
			if c == '=' {
				for byteAt(src, q) == ' ' {
					q++
				}
			}
			if c != '=' || byteAt(src, q) != '"' {
				emit(tERROR, p, p)
				break loop
			}
			p = q + 1
			state = sStringKVValue

		case sStringKVValue:
			if p >= n {
				continue
			}
			if c == '"' {
				p++
				state = sAttribute
			} else {
				emit(tSTR_VALUE, p, upTo('"', p))
			}

		case sAcc:
			emit(accValue, p, upTo('\n', p))
			state = sInitial

		default:
			if c == '}' {
				p++
				state = sInitial
			} else {
				emit(tAccDescrMultilineValue, p, upTo('}', p))
			}
		}
	}
	return tk
}

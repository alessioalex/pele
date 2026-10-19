package sankey

import "github.com/alessioalex/pele/go/internal/js"

const (
	tEnd = iota
	// Not a Jison token: the reference lexer would return the same empty token forever from here.
	tStuck
	tSankey
	tNewline
	tComma
	tDquote
	tNonEscapedText
	tEscapedText
	tEOF
)

var tokenNames = []string{
	"$end",
	"STUCK",
	"SANKEY",
	"NEWLINE",
	"COMMA",
	"DQUOTE",
	"NON_ESCAPED_TEXT",
	"ESCAPED_TEXT",
	"EOF",
}

type tokens struct {
	types  []int
	texts  []string
	starts []int
}

const (
	sInitial = iota
	sCSV
	sEscapedText
)

// RFC 4180 TEXTDATA: printable ASCII except the double quote and the comma.
func isTextData(c int) bool { return c >= 32 && c <= 126 && c != 34 && c != 44 }

// at is src.charCodeAt(i) for the ASCII range the lexer branches on; any byte of a character
// outside ASCII stands for that character, and past the end is -1 (NaN).
func at(src string, i int) int {
	if i >= len(src) {
		return -1
	}
	return int(src[i])
}

// The lexer works on bytes: every token is ASCII, and it stops at the first other character.
func tokenize(src string) tokens {
	n := len(src)
	var out tokens
	var stack []int
	state := sInitial
	p := 0

	emit := func(typ, s, e int) {
		out.types = append(out.types, typ)
		out.texts = append(out.texts, src[s:e])
		out.starts = append(out.starts, s)
		p = e
	}
	keyword := func(word string) bool {
		end := min(p+len(word), n)
		return js.ASCIILower(src[p:end]) == word && !js.IsWord(rune(at(src, p+len(word))))
	}

	for p < n {
		c := at(src, p)
		start := p
		q := p
		if state == sEscapedText {
			if c == 34 && at(src, p+1) != 34 {
				if len(stack) > 0 {
					state = stack[len(stack)-1]
					stack = stack[:len(stack)-1]
				}
				emit(tDquote, p, p+1)
				continue
			}
			for q < n {
				d := at(src, q)
				if isTextData(d) || d == 44 || d == 13 || d == 10 {
					q++
				} else if d == 34 && at(src, q+1) == 34 {
					q += 2
				} else {
					break
				}
			}
			emit(tEscapedText, p, q)
		} else {
			if state == sInitial && c|32 == 115 {
				length := 0
				if keyword("sankey-beta") {
					length = 11
				} else if keyword("sankey") {
					length = 6
				}
				if length > 0 {
					stack = append(stack, state)
					state = sCSV
					emit(tSankey, p, p+length)
					continue
				}
			}
			if c == 10 || (c == 13 && at(src, p+1) == 10) {
				if c == 10 {
					emit(tNewline, p, p+1)
				} else {
					emit(tNewline, p, p+2)
				}
				continue
			}
			if c == 44 {
				emit(tComma, p, p+1)
				continue
			}
			if c == 34 {
				stack = append(stack, state)
				state = sEscapedText
				emit(tDquote, p, p+1)
				continue
			}
			for q < n && isTextData(at(src, q)) {
				q++
			}
			emit(tNonEscapedText, p, q)
		}
		// A character outside the CSV alphabet matches as empty text and is never consumed.
		if q == start {
			emit(tStuck, p, p)
			return out
		}
	}

	if state == sEscapedText {
		emit(tEscapedText, n, n)
	} else {
		emit(tEOF, n, n)
	}
	emit(tEnd, n, n)
	return out
}

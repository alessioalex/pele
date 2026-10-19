package common

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
)

// Lexer rules that Mermaid's journey and timeline grammars have in common, ported from the
// compiled Jison rules: comments, whitespace, and the accTitle and accDescr statements.

// ScanTokens are the tokens of a Jison-style lexer.
type ScanTokens struct {
	Types  []int
	Texts  []string
	Starts []int
}

// LineEnd returns the next \n at or after from, or len(src).
func LineEnd(src string, from int) int {
	if from > len(src) {
		return len(src)
	}
	if i := strings.IndexByte(src[from:], '\n'); i >= 0 {
		return from + i
	}
	return len(src)
}

// KeywordAt reports whether word, lowercase ASCII letters, is at p; the grammars are
// case-insensitive.
func KeywordAt(src string, p int, word string) bool {
	if p+len(word) > len(src) {
		return false
	}
	for i := 0; i < len(word); i++ {
		if src[p+i]|32 != word[i] {
			return false
		}
	}
	return true
}

func byteAt(s string, i int) int {
	if i < 0 || i >= len(s) {
		return -1
	}
	return int(s[i])
}

func spaceAtPos(s string, p int) (bool, int) {
	r, size := js.RuneAt(s, p)
	return p < len(s) && js.IsSpace(r), size
}

// SkipTrivia skips comments and whitespace. It stops at a line break that starts a token, which
// is a NEWLINE. Jison compiles `\%%` to a single `%`, so one percent sign starts a comment, and
// the second comment rule takes any character before `%%` with it, a line break included.
func SkipTrivia(src string, from int) int {
	n := len(src)
	p := from
	for p < n {
		c := src[p]
		if c == '%' && byteAt(src, p+1) != '{' {
			p = LineEnd(src, p)
		} else if c != '}' && byteAt(src, p+1+charExtra(src, p)) == '%' && byteAt(src, p+2+charExtra(src, p)) == '%' {
			p = LineEnd(src, p+3+charExtra(src, p))
		} else if c == '\n' {
			return p
		} else if ok, size := spaceAtPos(src, p); ok {
			p += size
			for {
				ok, size := spaceAtPos(src, p)
				if !ok {
					break
				}
				p += size
			}
		} else if c == '#' {
			p = LineEnd(src, p)
		} else {
			return p
		}
	}
	return p
}

// charExtra is how many more bytes than one the character at p takes, so that offsets that
// count characters in the TypeScript count bytes here.
func charExtra(s string, p int) int {
	_, size := js.RuneAt(s, p)
	if size == 0 {
		return 0
	}
	return size - 1
}

func skipSpaces(src string, from int) int {
	p := from
	for {
		ok, size := spaceAtPos(src, p)
		if !ok {
			return p
		}
		p += size
	}
}

// Accessibility lexes an accTitle or accDescr statement at p and returns where it ends, or p if
// there is none. The token types acc_title, acc_title_value, acc_descr, acc_descr_value and
// acc_descr_multiline_value must be numbered consecutively from first.
func Accessibility(src string, p int, out *ScanTokens, first int) int {
	descr := KeywordAt(src, p, "accdescr")
	if !descr && !KeywordAt(src, p, "acctitle") {
		return p
	}
	n := len(src)
	q := skipSpaces(src, p+8)
	c := byteAt(src, q)
	emit := func(typ, s, e int) {
		out.Types = append(out.Types, typ)
		out.Texts = append(out.Texts, src[s:e])
		out.Starts = append(out.Starts, s)
	}
	if c == ':' {
		q = skipSpaces(src, q+1)
		end := LineEnd(src, q)
		if descr {
			emit(first+2, p, q)
			emit(first+3, q, end)
		} else {
			emit(first, p, q)
			emit(first+1, q, end)
		}
		return end
	}
	if !descr || c != '{' {
		return p
	}
	q = skipSpaces(src, q+1)
	for byteAt(src, q) != '}' {
		end := n
		if i := strings.IndexByte(src[q:], '}'); i >= 0 {
			end = q + i
		}
		emit(first+4, q, end)
		if q == n {
			return n
		}
		q = end
	}
	return q + 1
}

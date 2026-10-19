package venn

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
)

const (
	tEnd = iota
	tError
	tEOF
	tNewline
	tVenn
	tTitle
	tSet
	tUnion
	tText
	tIndentText
	tStyle
	tBracketLabel
	tNumeric
	tHexColor
	tRGBColor
	tRGBAColor
	tIdentifier
	tString
	tComma
	tColon
)

var tokenNames = []string{
	"$end",
	"INVALID",
	"EOF",
	"NEWLINE",
	"VENN",
	"TITLE",
	"SET",
	"UNION",
	"TEXT",
	"INDENT_TEXT",
	"STYLE",
	"BRACKET_LABEL",
	"NUMERIC",
	"HEXCOLOR",
	"RGBCOLOR",
	"RGBACOLOR",
	"IDENTIFIER",
	"STRING",
	"COMMA",
	"COLON",
}

var keywords = []struct {
	word string
	kind int
}{
	{"venn-beta", tVenn},
	{"set", tSet},
	{"union", tUnion},
	{"text", tText},
	{"style", tStyle},
}

// lexer reads tokens one at a time, as Mermaid's Jison lexer for venn diagrams does: every rule
// ignores case, the first rule that matches wins, and a keyword must end at a word boundary.
// Positions are byte offsets.
type lexer struct {
	// The text and position of the token next returned last.
	text      string
	start     int
	src       string
	host      *DB
	at        int
	lineStart bool
	ended     bool
}

func (l *lexer) byteAt(i int) int {
	if i < 0 || i >= len(l.src) {
		return -1
	}
	return int(l.src[i])
}

// spaceAt is isSpace(charCodeAt) for the character at byte offset i.
func (l *lexer) spaceAt(i int) (bool, int) {
	r, size := js.RuneAt(l.src, i)
	if r < 0 || r >= 0x10000 {
		return false, size
	}
	return js.IsSpace(r), size
}

func isDigit(c int) bool { return c >= '0' && c <= '9' }

func isHex(c int) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')
}

func isWord(c int) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_'
}

func (l *lexer) word(at int, word string) bool {
	for k := 0; k < len(word); k++ {
		c := l.byteAt(at + k)
		w := int(word[k])
		if c != w && (c+32 != w || c < 65 || c > 90) {
			return false
		}
	}
	return true
}

func (l *lexer) keyword(at int, word string) bool {
	return l.word(at, word) && !isWord(l.byteAt(at+len(word)))
}

func (l *lexer) skipSpaces(i int) int {
	for {
		ok, size := l.spaceAt(i)
		if !ok {
			return i
		}
		i += size
	}
}

// color is the end of `rgb(1, 2, 3)` or `rgba(1, 2, 3, 0.5)` whose parenthesis opens at at, or -1.
func (l *lexer) color(at, parts int) int {
	i := at + 1
	for part := 0; part < parts; part++ {
		i = l.skipSpaces(i)
		from := i
		for isDigit(l.byteAt(i)) || l.byteAt(i) == '.' {
			i++
		}
		if i == from {
			return -1
		}
		i = l.skipSpaces(i)
		want := ','
		if part == parts-1 {
			want = ')'
		}
		if l.byteAt(i) != int(want) {
			return -1
		}
		i++
	}
	return i
}

func (l *lexer) lineEnd(from int) int {
	if from >= len(l.src) {
		return len(l.src)
	}
	end := strings.IndexByte(l.src[from:], '\n')
	if end == -1 {
		return len(l.src)
	}
	return from + end
}

func (l *lexer) next() int {
	src := l.src
	n := len(src)
	for {
		i := l.at
		l.start = i
		l.text = ""
		if i >= n {
			if l.ended {
				return tEnd
			}
			l.ended = true
			return tEOF
		}
		c := int(src[i])
		// The second code unit: after a character of two code units, its low surrogate.
		second := i + 1
		if c >= 0x80 {
			r, size := js.RuneAt(src, i)
			if r >= 0x10000 {
				second = -1
			} else {
				second = i + size
			}
		}
		// A comment runs to the end of the line. It also takes the character before it along,
		// whatever that is: a line break, or the last letter of a word.
		if (c == '%' && l.byteAt(i+1) == '%' && l.byteAt(i+2) != '{') ||
			(c != '}' && second != -1 && l.byteAt(second) == '%' && l.byteAt(second+1) == '%') {
			l.at = l.lineEnd(i + 2)
			continue
		}
		if l.lineStart {
			if c == ' ' || c == '\t' {
				j := i + 1
				for l.byteAt(j) == ' ' || l.byteAt(j) == '\t' {
					j++
				}
				l.at = j
				if l.keyword(j, "text") && l.host.indent {
					// The word `text` itself is dropped; the indentation stands for it.
					l.at = j + 4
					l.lineStart = false
					l.text = src[i:j]
					return tIndentText
				}
				continue
			}
			if c != '\n' && c != '\r' {
				l.host.indent = false
				l.lineStart = false
			}
		}
		if c == '\n' || c == '\r' {
			i++
			for l.byteAt(i) == '\n' || l.byteAt(i) == '\r' {
				i++
			}
			l.lineStart = true
			return l.token(tNewline, i)
		}
		if c == '%' && l.byteAt(i+1) == '%' {
			l.at = l.lineEnd(i)
			continue
		}
		if c == ' ' || c == '\t' {
			i++
			for l.byteAt(i) == ' ' || l.byteAt(i) == '\t' {
				i++
			}
			l.at = i
			continue
		}

		if ((c|32) >= 'a' && (c|32) <= 'z') || c == '_' {
			if l.word(i, "title") {
				if ok, size := l.spaceAt(i + 5); ok {
					from := i + 5 + size
					end := from
					for end < n {
						ch := src[end]
						if ch == '#' || ch == '\n' || ch == ';' {
							break
						}
						end++
					}
					if end > from {
						return l.token(tTitle, end)
					}
				}
			}
			for _, k := range keywords {
				if l.keyword(i, k.word) {
					return l.token(k.kind, i+len(k.word))
				}
			}
			if l.word(i, "rgb") {
				alpha := l.byteAt(i+3)|32 == 'a'
				open := i + 3
				parts := 3
				if alpha {
					open, parts = i+4, 4
				}
				end := -1
				if l.byteAt(open) == '(' {
					end = l.color(open, parts)
				}
				if end != -1 {
					if alpha {
						return l.token(tRGBAColor, end)
					}
					return l.token(tRGBColor, end)
				}
			}
			i++
			for isWord(l.byteAt(i)) || l.byteAt(i) == '-' {
				i++
			}
			return l.token(tIdentifier, i)
		}
		if c == '[' {
			if l.byteAt(i+1) == '"' {
				quote := -1
				if i+2 <= n {
					if q := strings.IndexByte(src[i+2:], '"'); q != -1 {
						quote = i + 2 + q
					}
				}
				if quote == -1 || l.byteAt(quote+1) != ']' {
					return tError
				}
				l.at = quote + 2
				l.text = src[i+2 : quote]
				return tBracketLabel
			}
			end := i + 1
			for end < n && src[end] != ']' && src[end] != '"' {
				end++
			}
			if end == i+1 || l.byteAt(end) != ']' {
				return tError
			}
			l.at = end + 1
			l.text = js.Trim(src[i+1 : end])
			return tBracketLabel
		}
		if isDigit(c) || c == '.' || c == '+' || c == '-' {
			if c == '+' || c == '-' {
				i++
			}
			from := i
			for isDigit(l.byteAt(i)) {
				i++
			}
			if i > from {
				if l.byteAt(i) == '.' && isDigit(l.byteAt(i+1)) {
					i += 2
					for isDigit(l.byteAt(i)) {
						i++
					}
				}
			} else if l.byteAt(i) == '.' && isDigit(l.byteAt(i+1)) {
				i += 2
				for isDigit(l.byteAt(i)) {
					i++
				}
			} else {
				return tError
			}
			return l.token(tNumeric, i)
		}
		if c == '#' {
			end := i + 1
			for end < i+9 && isHex(l.byteAt(end)) {
				end++
			}
			if end >= i+4 {
				return l.token(tHexColor, end)
			}
			return tError
		}
		if c == '"' {
			q := strings.IndexByte(src[i+1:], '"')
			if q == -1 {
				return tError
			}
			return l.token(tString, i+1+q+1)
		}
		if c == ',' {
			return l.token(tComma, i+1)
		}
		if c == ':' {
			return l.token(tColon, i+1)
		}
		return tError
	}
}

func (l *lexer) token(kind, end int) int {
	l.text = l.src[l.at:end]
	l.at = end
	return kind
}

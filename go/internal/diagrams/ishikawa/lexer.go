package ishikawa

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
)

const (
	tEND = iota
	tEOF
	tSPACELINE
	tNL
	tISHIKAWA
	tSPACELIST
	tTEXT
)

var tokenNames = []string{"$end", "EOF", "SPACELINE", "NL", "ISHIKAWA", "SPACELIST", "TEXT"}

type tokens struct {
	types  []int
	starts []int
	ends   []int
}

func byteAt(s string, i int) int {
	if i < 0 || i >= len(s) {
		return -1
	}
	return int(s[i])
}

func keyword(src string, at int) int {
	const word = "ishikawa-beta"
	k := 0
	for k < 13 {
		c := byteAt(src, at+k)
		w := int(word[k])
		if c != w && (c+32 != w || c < 65 || c > 90) {
			break
		}
		k++
	}
	// `ishikawa-beta`, or failing that `ishikawa`, either one ending at a word boundary.
	if k == 13 && !isWordAt(src, at+13) {
		return 13
	}
	if k >= 8 && !isWordAt(src, at+8) {
		return 8
	}
	return 0
}

func isWordAt(s string, i int) bool {
	c := byteAt(s, i)
	return c >= 0 && js.IsWord(rune(c))
}

// tokenize produces the tokens of Mermaid's Jison lexer for Ishikawa diagrams, where the first
// rule that matches wins: white space up to a comment or up to the last line break in it is a
// blank line, a single line break before text is a new line, other white space is indentation,
// and the rest of a line is text.
func tokenize(src string) tokens {
	var out tokens
	n := len(src)
	i := 0
	for i < n {
		j := i
		lastBreak := -1
		for j < n {
			r, size := js.RuneAt(src, j)
			if !js.IsSpace(r) {
				break
			}
			if r == '\n' {
				lastBreak = j
			}
			j += size
		}
		var typ, end int
		if byteAt(src, j) == '%' && byteAt(src, j+1) == '%' {
			typ = tSPACELINE
			end = j + 2
			for end < n {
				r, size := js.RuneAt(src, end)
				if js.IsLineEnd(r) {
					break
				}
				end += size
			}
		} else if j == i {
			if length := keyword(src, i); length > 0 {
				typ = tISHIKAWA
				end = i + length
			} else {
				typ = tTEXT
				end = strings.IndexByte(src[i:], '\n')
				if end == -1 {
					end = n
				} else {
					end += i
				}
			}
		} else if lastBreak > i {
			typ = tSPACELINE
			end = lastBreak + 1
		} else if lastBreak == i {
			typ = tNL
			end = i + 1
		} else {
			typ = tSPACELIST
			end = j
		}
		out.types = append(out.types, typ)
		out.starts = append(out.starts, i)
		out.ends = append(out.ends, end)
		i = end
	}
	out.types = append(out.types, tEOF, tEND)
	out.starts = append(out.starts, n, n)
	out.ends = append(out.ends, n, n)
	return out
}

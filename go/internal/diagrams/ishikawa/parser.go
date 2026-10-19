package ishikawa

import (
	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

const stop = "'SPACELINE', 'NL', 'EOF'"

// parseIshikawa follows the parser Jison builds from Mermaid's grammar: blank lines and comments
// may come first, then the keyword and at most one line break, then statements. A statement is a
// line of text, with or without indentation, or a blank line; each is ended by a blank line, a
// line break or the end of the text, and any number of line breaks after that.
func parseIshikawa(src string, db *Db) {
	t := tokenize(src)
	types, starts, ends := t.types, t.starts, t.ends
	i := 0

	fail := func(expected string) {
		panic(core.SyntaxError("ishikawa", src, starts[i], "Expecting "+expected+", got '"+tokenNames[types[i]]+"'", false))
	}
	ended := func(k int) bool { return types[k] == tSPACELINE || types[k] == tNL || types[k] == tEOF }

	if types[i] == tSPACELINE {
		i++
		for types[i] == tSPACELINE || types[i] == tNL {
			i++
		}
		if types[i] != tISHIKAWA {
			fail("'SPACELINE', 'NL', 'ISHIKAWA'")
		}
	} else if types[i] != tISHIKAWA {
		fail("'SPACELINE', 'ISHIKAWA'")
	}
	i++
	if types[i] == tNL {
		i++
	}

	for {
		typ := types[i]
		if typ == tTEXT || (typ == tSPACELIST && types[i+1] == tTEXT) {
			text := i
			if typ != tTEXT {
				text = i + 1
			}
			i = text + 1
			if !ended(i) {
				fail(stop)
			}
			level := 0
			if typ != tTEXT {
				level = js.Len(src[starts[text-1]:ends[text-1]])
			}
			db.addNode(level, js.Trim(src[starts[text]:ends[text]]))
		} else if typ == tSPACELINE || typ == tSPACELIST {
			i++
			if !ended(i) {
				if typ == tSPACELIST {
					fail(stop + ", 'TEXT'")
				} else {
					fail(stop)
				}
			}
		} else {
			fail("'SPACELINE', 'SPACELIST', 'TEXT'")
		}
		i++
		for types[i] == tNL || types[i] == tEOF {
			i++
		}
		if types[i] == tEND {
			return
		}
	}
}

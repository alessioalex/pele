package quadrant

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

func flags(tokens ...int) []bool {
	out := make([]bool, tEOF+1)
	for _, t := range tokens {
		out[t] = true
	}
	return out
}

var (
	wordTokens = flags(tPunctuation, tAmp, tNum, tAlpha, tComma, tPlus, tEquals, tMult, tDot, tBrkt, tUnderscore, tUnicodeText)
	idTokens   = flags(tAlpha, tNum, tMinus, tComma, tColon, tAmp, tBrkt, tMult, tUnicodeText)
	styleTok   = flags(tAlpha, tNum, tColon, tSpace, tBrkt, tMinus)
	eolTokens  = flags(tNEWLINE, tSemi, tEOF)
)

const textTokens = "'STR', 'MD_STR', 'ALPHA', 'NUM', 'UNICODE_TEXT', 'PUNCTUATION'"

func parseQuadrant(src string, db *Db) {
	toks := tokenize(src)
	types, texts, starts := toks.types, toks.texts, toks.starts
	i := 0

	kind := func(k int) int {
		if k < len(types) {
			return types[k]
		}
		return -1
	}
	is := func(set []bool, t int) bool { return t >= 0 && set[t] }

	fail := func(expected string) {
		t := types[i]
		if t == tERROR {
			panic(core.SyntaxError("quadrantChart", src, starts[i], "", true))
		}
		panic(core.SyntaxError("quadrantChart", src, starts[i], "Expecting "+expected+", got '"+tokenNames[t]+"'", false))
	}

	expect := func(typ int) string {
		if types[i] != typ {
			fail("'" + tokenNames[typ] + "'")
		}
		i++
		return texts[i-1]
	}

	isText := func(t int) bool { return t == tStr || t == tMdStr || is(wordTokens, t) }

	readText := func() *Text {
		first := types[i]
		if !isText(first) {
			fail(textTokens)
		}
		var s strings.Builder
		s.WriteString(texts[i])
		i++
		for t := kind(i); is(wordTokens, t) || t == tSpace || t == tMinus; t = kind(i) {
			s.WriteString(texts[i])
			i++
		}
		typ := "text"
		if first == tMdStr {
			typ = "markdown"
		}
		return &Text{Text: s.String(), Type: typ}
	}

	styles := func() []string {
		var out []string
		for {
			if !is(styleTok, kind(i)) {
				fail("'ALPHA', 'NUM', 'COLON', 'SPACE', 'BRKT', 'MINUS'")
			}
			var s strings.Builder
			s.WriteString(texts[i])
			i++
			for is(styleTok, kind(i)) {
				s.WriteString(texts[i])
				i++
			}
			out = append(out, js.Trim(s.String()))
			if kind(i) != tComma {
				return out
			}
			i++
		}
	}

	for is(eolTokens, kind(i)) || kind(i) == tSpace {
		i++
	}
	expect(tQuadrant)

	for types[i] != tEND {
		for types[i] == tSpace {
			i++
		}
		t := types[i]
		switch t {
		case tClassdef:
			i++
			expect(tSpace)
			if !is(idTokens, kind(i)) {
				fail("'ALPHA', 'NUM', 'UNICODE_TEXT'")
			}
			var id strings.Builder
			id.WriteString(texts[i])
			i++
			for is(idTokens, kind(i)) {
				id.WriteString(texts[i])
				i++
			}
			expect(tSpace)
			db.addClass(id.String(), styles())
		case tXAxis, tYAxis:
			i++
			from := readText()
			var to *Text
			if types[i] == tDelimiter {
				i++
				if isText(types[i]) {
					to = readText()
				} else {
					from.Text += " ⟶ "
				}
			}
			if t == tXAxis {
				db.XAxisLeft = trimmed(from)
				if to != nil {
					db.XAxisRight = trimmed(to)
				}
			} else {
				db.YAxisBottom = trimmed(from)
				if to != nil {
					db.YAxisTop = trimmed(to)
				}
			}
		case tQuadrant1, tQuadrant2, tQuadrant3, tQuadrant4:
			i++
			db.Quadrants[t-tQuadrant1] = trimmed(readText())
		case tTitle:
			i++
			db.Title = js.Trim(expect(tTitleValue))
		case tAccTitle:
			i++
			db.setAccTitle(js.Trim(expect(tAccTitleValue)))
		case tAccDescr:
			i++
			db.setAccDescription(js.Trim(expect(tAccDescrValue)))
		case tAccDescrMultilineValue:
			db.setAccDescription(js.Trim(texts[i]))
			i++
		default:
			if isText(t) {
				label := readText()
				className := ""
				if types[i] == tClassName {
					className = texts[i]
					i++
				}
				expect(tPointStart)
				x := expect(tPointX)
				y := expect(tPointY)
				var st []string
				if is(styleTok, kind(i)) {
					st = styles()
				}
				db.addPoint(label, className, x, y, st)
			}
		}
		if !is(eolTokens, types[i]) {
			fail("'NEWLINE', 'SEMI', 'EOF'")
		}
		i++
	}
}

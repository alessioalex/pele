package sankey

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

func isBlank(r rune) bool { return r != '\n' && r != '\r' && js.IsSpace(r) }

// collapseLineBreaks is .replace(/[\n\r]+/g, '\n').
func collapseLineBreaks(s string) string {
	if !strings.ContainsAny(s, "\n\r") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\n' || s[i] == '\r' {
			for i < len(s) && (s[i] == '\n' || s[i] == '\r') {
				i++
			}
			b.WriteByte('\n')
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// Mermaid's sankeyUtils: strip blanks at both ends of the text, then collapse runs of line
// breaks so that no row is empty.
func prepareTextForParsing(text string) string {
	text = strings.TrimFunc(text, isBlank)
	return js.Trim(collapseLineBreaks(text))
}

func parseSankey(src string, db *Model) {
	tk := tokenize(src)
	types, texts, starts := tk.types, tk.texts, tk.starts
	i := 0

	fail := func(expected string) {
		got := "undefined"
		if t := types[i]; t != tStuck {
			got = tokenNames[t]
		} else if i > 0 {
			got = tokenNames[types[i-1]]
		}
		panic(core.SyntaxError("sankey", src, starts[i], "Expecting "+expected+", got '"+got+"'", false))
	}
	expect := func(typ int, also string) {
		if types[i] != typ {
			fail("'" + tokenNames[typ] + "'" + also)
		}
		i++
	}
	field := func() string {
		if types[i] == tNonEscapedText {
			i++
			return texts[i-1]
		}
		if types[i] != tDquote {
			fail("'DQUOTE', 'NON_ESCAPED_TEXT'")
		}
		i++
		if types[i] != tEscapedText {
			fail("'ESCAPED_TEXT'")
		}
		text := texts[i]
		i++
		expect(tDquote, "")
		return text
	}
	name := func() string { return strings.ReplaceAll(js.Trim(field()), `""`, `"`) }

	expect(tSankey, "")
	expect(tNewline, "")
	for {
		from := name()
		expect(tComma, ", 'NEWLINE', 'EOF'")
		to := name()
		expect(tComma, ", 'NEWLINE', 'EOF'")
		amount := field()
		source := db.findOrCreateNode(from)
		target := db.findOrCreateNode(to)
		db.addLink(source, target, js.ParseFloat(js.Trim(amount)))
		if types[i] != tNewline {
			break
		}
		i++
	}
	if types[i] == tEOF {
		i++
	}
	if types[i] != tEnd {
		fail("'NEWLINE', 'EOF'")
	}
}

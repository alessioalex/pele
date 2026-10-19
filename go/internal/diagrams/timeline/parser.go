package timeline

import (
	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

const statement = "'EOF', 'SPACE', 'NEWLINE', 'title', 'acc_title', 'acc_descr', 'acc_descr_multiline_value', 'section', 'period', 'event'"

// after returns text past its first `at` bytes and the whitespace character that follows them,
// which the TypeScript cuts as one code unit.
func after(text string, at int) string {
	_, size := js.RuneAt(text, at)
	return text[at+size:]
}

func parseTimeline(src string, db *Db) {
	tokens := tokenize(src)
	types, texts, starts := tokens.Types, tokens.Texts, tokens.Starts

	fail := func(i int, expected string) {
		got := tokenNames[types[i]]
		if types[i] == tEND {
			got = "1"
		}
		panic(core.SyntaxError("timeline", src, starts[i], "Expecting "+expected+", got '"+got+"'", false))
	}

	head := types[0]
	if head != tTimeline && head != tTimelineLR && head != tTimelineTD {
		fail(0, "'timeline', 'timeline_lr', 'timeline_td'")
	}
	if head != tTimeline {
		if head == tTimelineLR {
			db.setDirection("LR")
		} else {
			db.setDirection("TD")
		}
	}

	for i := 1; ; i++ {
		text := texts[i]
		switch types[i] {
		case tNEWLINE:
		case tTitle:
			db.setDiagramTitle(after(text, 5))
		case tAccTitle:
			i++
			db.setAccTitle(js.Trim(texts[i]))
		case tAccDescr:
			i++
			db.setAccDescription(js.Trim(texts[i]))
		case tAccDescrMultilineValue:
			db.setAccDescription(js.Trim(text))
		case tSection:
			db.addSection(after(text, 7))
		case tPeriod:
			db.addTask(text, "")
		case tEvent:
			db.addEvent(after(text, 1))
		case tEOF:
			return
		default:
			fail(i, statement)
		}
	}
}

package journey

import (
	"slices"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

type task struct {
	task string
	// As written: any number, or NaN when the score is not one.
	score  float64
	people []string
	// Name of the section in force when the task was declared, "" before the first one.
	section string
}

type model struct {
	title, accTitle, accDescr string
	sections                  []string
	tasks                     []task
	currentSection            string
}

func cmpUnits(a, b string) int { return slices.Compare(js.Units(a), js.Units(b)) }

// actors is everyone named on a task, once each, in code unit order.
func actors(tasks []task) []string {
	seen := map[string]bool{}
	var names []string
	for _, t := range tasks {
		for _, name := range t.people {
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	slices.SortStableFunc(names, cmpUnits)
	return names
}

func (m *model) addTask(name, data string) {
	pieces := strings.Split(data[1:], ":")
	var people []string
	if len(pieces) > 1 {
		for _, p := range strings.Split(pieces[1], ",") {
			people = append(people, js.Trim(p))
		}
	}
	m.tasks = append(m.tasks, task{task: name, score: js.ToNumber(pieces[0]), people: people, section: m.currentSection})
}

// collapseIndents is .replace(/\n\s+/g, '\n').
func collapseIndents(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\n' {
			j := i + 1
			for j < len(s) {
				r, size := js.RuneAt(s, j)
				if !js.IsSpace(r) {
					break
				}
				j += size
			}
			b.WriteByte('\n')
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

const statement = "'EOF', 'SPACE', 'NEWLINE', 'title', 'acc_title', 'acc_descr', 'acc_descr_multiline_value', 'section', 'taskName'"

func parseJourney(src string, m *model) {
	tk := tokenize(src)
	types, texts := tk.Types, tk.Texts
	fail := func(i int, expected string) {
		got := tokenNames[types[i]]
		if types[i] == tEnd {
			got = "1"
		}
		panic(core.SyntaxError("journey", src, tk.Starts[i], "Expecting "+expected+", got '"+got+"'", false))
	}
	if types[0] != tJourney {
		fail(0, "'journey'")
	}
	for i := 1; ; i++ {
		text := texts[i]
		switch types[i] {
		case tNewline:
		case tTitle:
			m.title = text[5+spaceAfter(text, 5):]
		case tAccTitle:
			i++
			m.accTitle = js.TrimStart(js.Trim(texts[i]))
		case tAccDescr:
			i++
			m.accDescr = collapseIndents(js.Trim(texts[i]))
		case tAccDescrMultilineValue:
			m.accDescr = collapseIndents(js.Trim(text))
		case tSection:
			s := text[7+spaceAfter(text, 7):]
			m.currentSection = s
			m.sections = append(m.sections, s)
		case tTaskName:
			i++
			if types[i] != tTaskData {
				fail(i, "'taskData'")
			}
			m.addTask(text, texts[i])
		case tEOF:
			return
		default:
			fail(i, statement)
		}
	}
}

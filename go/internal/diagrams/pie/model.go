package pie

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

// Model is a parsed pie chart. Labels keep the order they were first given in.
type Model struct {
	Title    string
	AccTitle string
	AccDescr string
	ShowData bool
	Labels   []string
	Values   map[string]float64
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

func buildPie(a ast, title string) *Model {
	m := &Model{Title: a.title, ShowData: a.showData, Values: map[string]float64{}}
	for _, s := range a.sections {
		if s.value < 0 {
			e := core.NewError(`"`+s.label+`" has invalid value: `+js.NumberToString(s.value)+
				". Negative values are not allowed in pie charts. All slice values must be >= 0.", core.CodeSemantic)
			e.Type = "pie"
			panic(e)
		}
		if _, ok := m.Values[s.label]; !ok {
			m.Labels = append(m.Labels, s.label)
			m.Values[s.label] = s.value
		}
	}
	if m.Title == "" {
		m.Title = title
	}
	if a.accTitle != "" {
		m.AccTitle = js.TrimStart(a.accTitle)
	}
	if a.accDescr != "" {
		m.AccDescr = collapseIndents(a.accDescr)
	}
	return m
}

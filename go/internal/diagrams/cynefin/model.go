package cynefin

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
)

// Item is an entry in a domain.
type Item struct {
	Label string
}

// Domain is a domain the source declares.
type Domain struct {
	Name  string
	Items []Item
}

// Transition is an arrow between two domains. Label "" is no label.
type Transition struct {
	From, To string
	Label    string
}

// Model is a parsed Cynefin diagram.
type Model struct {
	Title    string
	AccTitle string
	AccDescr string
	// The domains the source declares, by name. All five are drawn either way.
	Domains     map[string]*Domain
	Transitions []Transition
}

// A later block for a domain replaces an earlier one.
func setDomains(m *Model, blocks []domainBlockAst) {
	for _, block := range blocks {
		d := &Domain{Name: block.domain}
		for _, label := range block.items {
			d.Items = append(d.Items, Item{Label: label})
		}
		m.Domains[block.domain] = d
	}
}

func setTransitions(m *Model, transitions []transitionAst) {
	for _, t := range transitions {
		if t.from == t.to {
			continue
		}
		m.Transitions = append(m.Transitions, Transition{From: t.from, To: t.to, Label: t.label})
	}
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

func buildCynefin(ast *cynefinAst, title string) *Model {
	m := &Model{Title: ast.title, Domains: map[string]*Domain{}}
	if m.Title == "" {
		m.Title = title
	}
	if ast.accTitle != "" {
		m.AccTitle = js.TrimStart(ast.accTitle)
	}
	if ast.accDescr != "" {
		m.AccDescr = collapseIndents(ast.accDescr)
	}
	setDomains(m, ast.domains)
	setTransitions(m, ast.transitions)
	return m
}

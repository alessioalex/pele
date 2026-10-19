package treemap

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
)

type node struct {
	name string
	// Sections have a list, which may be empty. Leaves have none.
	children          []*node
	section           bool
	value             float64
	hasValue          bool
	classSelector     string
	cssCompiledStyles []string
}

type model struct {
	title, accTitle, accDescr string
	roots                     []*node
}

// buildHierarchy puts each row in the nearest section above it that is indented less.
func buildHierarchy(rows []row, classes map[string][]string) []*node {
	var roots []*node
	type entry struct {
		node  *node
		level int
	}
	var stack []entry
	for _, rw := range rows {
		if rw.classDef {
			continue
		}
		it := rw.item
		n := &node{name: it.name, section: !it.leaf, classSelector: it.classSelector}
		if !it.leaf {
			n.children = []*node{}
		}
		if it.classSelector != "" {
			if styles := classes[it.classSelector]; len(styles) > 0 {
				n.cssCompiledStyles = styles
			}
		}
		if it.leaf {
			n.value, n.hasValue = it.value, true
		}
		for len(stack) > 0 && stack[len(stack)-1].level >= rw.indent {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			roots = append(roots, n)
		} else {
			parent := stack[len(stack)-1].node
			parent.children = append(parent.children, n)
		}
		if !it.leaf {
			stack = append(stack, entry{n, rw.indent})
		}
	}
	return roots
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

func buildTreemap(a ast, title string) *model {
	classes := map[string][]string{}
	for _, rw := range a.rows {
		if !rw.classDef {
			continue
		}
		// An escaped comma stays in the value; the others separate declarations.
		text := strings.ReplaceAll(rw.styleText, `\,`, "§§§")
		text = strings.ReplaceAll(text, ",", ";")
		text = strings.ReplaceAll(text, "§§§", ",")
		classes[rw.className] = append(classes[rw.className], strings.Split(text, ";")...)
	}
	m := &model{title: a.title, roots: buildHierarchy(a.rows, classes)}
	if m.title == "" {
		m.title = title
	}
	if a.accTitle != "" {
		m.accTitle = js.TrimStart(a.accTitle)
	}
	if a.accDescr != "" {
		m.accDescr = collapseIndents(a.accDescr)
	}
	return m
}
